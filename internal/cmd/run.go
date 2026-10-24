package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/tbeijen/secretd/internal/aws"
	"github.com/tbeijen/secretd/internal/config"
	"github.com/tbeijen/secretd/internal/session"
)

const Placeholder = "@SECRETD_DIR@"

type ExitError struct {
	Code int
}

func (e *ExitError) Error() string { return fmt.Sprintf("exit status %d", e.Code) }

func Run(args []string) error {
	contextName, child, err := parseRunArgs(args)
	if err != nil {
		return err
	}

	c, settings, err := loadContext(contextName)
	if err != nil {
		return err
	}

	ctx := context.Background()
	cli, err := aws.NewCLI(ctx, settings.AWSCLI)
	if err != nil {
		return err
	}
	logf("context %s (%s)", c.Name, c.Path)
	logf("aws CLI %s", cli.Path)

	profiles, err := resolveProfiles(ctx, cli, c)
	if err != nil {
		return err
	}
	entries, err := session.MintAll(ctx, cli, profiles)
	if err != nil {
		return err
	}

	s, err := session.Create(filepath.Join(os.TempDir(), "secretd"))
	if err != nil {
		return err
	}
	defer s.Remove()
	if err := s.Write(entries); err != nil {
		return err
	}
	logf("minted %s", strings.Join(c.ProfileNames(), ", "))
	logf("session dir %s", s.Dir)

	refreshCtx, stopRefresh := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		refreshLoop(refreshCtx, s, cli, profiles, entries)
	}()

	err = runChild(child, s, c.ProfileNames())
	stopRefresh()
	<-done
	return err
}

func parseRunArgs(args []string) (contextName string, child []string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			child = args[i+1:]
			if len(child) == 0 {
				return "", nil, errors.New("no command given after --")
			}
			return contextName, child, nil
		case a == "--context":
			if i+1 >= len(args) {
				return "", nil, errors.New("--context requires a value")
			}
			i++
			contextName = args[i]
		case strings.HasPrefix(a, "--context="):
			contextName = strings.TrimPrefix(a, "--context=")
		default:
			return "", nil, fmt.Errorf("unexpected argument %q, usage: secretd [--context X] -- <cmd>...", a)
		}
	}
	return "", nil, errors.New("missing -- <cmd>...")
}

func loadContext(name string) (*config.Context, config.Settings, error) {
	dir, err := config.Dir()
	if err != nil {
		return nil, config.Settings{}, err
	}
	settings, err := config.LoadSettings(dir)
	if err != nil {
		return nil, settings, err
	}
	contexts, err := config.LoadContexts(dir)
	if err != nil {
		return nil, settings, err
	}
	if len(contexts) == 0 {
		return nil, settings, fmt.Errorf("no contexts in %s", filepath.Join(dir, "contexts"))
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, settings, err
	}
	c, err := config.Select(contexts, name, cwd)
	return c, settings, err
}

func resolveProfiles(ctx context.Context, m aws.Minter, c *config.Context) ([]session.Profile, error) {
	var out []session.Profile
	for _, name := range c.ProfileNames() {
		p := c.AWS.Profiles[name]
		sp := session.Profile{Name: name, Source: p.SourceProfile(name), Region: p.Region}
		if sp.Region == "" {
			r, err := m.Region(ctx, sp.Source)
			if err != nil {
				return nil, fmt.Errorf("profile %s: %w", name, err)
			}
			sp.Region = r
		}
		out = append(out, sp)
	}
	return out, nil
}

func refreshLoop(ctx context.Context, s *session.Session, m aws.Minter, profiles []session.Profile, entries []session.Entry) {
	current := make(map[string]session.Entry, len(entries))
	for _, e := range entries {
		current[e.Name] = e
	}

	timer := time.NewTimer(session.NextRefresh(time.Now(), entries))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		errs := session.Refresh(ctx, m, profiles, current)
		for _, err := range errs {
			logf("warning: refresh %s", err)
		}
		all := make([]session.Entry, 0, len(current))
		for _, e := range current {
			all = append(all, e)
		}
		if err := s.Write(all); err != nil {
			logf("warning: write session: %s", err)
			errs = append(errs, err)
		}

		next := session.NextRefresh(time.Now(), all)
		if len(errs) > 0 && next > session.RetryInterval {
			next = session.RetryInterval
		}
		timer.Reset(next)
	}
}

func runChild(args []string, s *session.Session, profiles []string) error {
	for i := range args {
		args[i] = strings.ReplaceAll(args[i], Placeholder, s.Dir)
	}

	c := exec.Command(args[0], args[1:]...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	c.Env = childEnv(os.Environ(), s, profiles)

	// SIGINT/SIGQUIT from the terminal already reach the child via the process
	// group. Forwarding them would deliver them twice.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigs)

	if err := c.Start(); err != nil {
		return err
	}
	go func() {
		for sig := range sigs {
			if sig == syscall.SIGTERM || sig == syscall.SIGHUP {
				c.Process.Signal(sig)
			}
		}
	}()

	err := c.Wait()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return &ExitError{Code: 128 + int(ws.Signal())}
		}
		return &ExitError{Code: exitErr.ExitCode()}
	}
	return err
}

func childEnv(env []string, s *session.Session, profiles []string) []string {
	strip := append(slices.Clone(aws.CredentialEnv), "AWS_CONFIG_FILE", "AWS_SHARED_CREDENTIALS_FILE", "SECRETD_DIR")
	for _, k := range []string{"AWS_PROFILE", "AWS_DEFAULT_PROFILE"} {
		if !slices.Contains(profiles, lookupEnv(env, k)) {
			strip = append(strip, k)
		}
	}
	env = aws.StripEnv(env, strip...)
	return append(env,
		"AWS_CONFIG_FILE="+s.AWSConfigFile(),
		"AWS_SHARED_CREDENTIALS_FILE="+s.AWSCredentialsFile(),
		"SECRETD_DIR="+s.Dir,
	)
}

func lookupEnv(env []string, key string) string {
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == key {
			return v
		}
	}
	return ""
}

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "secretd: "+format+"\n", args...)
}
