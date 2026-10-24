package aws

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const callTimeout = 60 * time.Second

// CLI mints credentials by shelling out to AWS CLI v2.
type CLI struct {
	Path string
}

var versionRe = regexp.MustCompile(`aws-cli/(\d+)\.(\d+)\.(\d+)`)

// NewCLI resolves the aws binary once, to an absolute path, and checks its version.
func NewCLI(ctx context.Context, path string) (*CLI, error) {
	if path == "" {
		p, err := exec.LookPath("aws")
		if err != nil {
			return nil, fmt.Errorf("aws CLI not found: %w", err)
		}
		path = p
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	c := &CLI{Path: abs}

	out, err := c.run(ctx, "--version")
	if err != nil {
		return nil, err
	}
	if err := checkVersion(out); err != nil {
		return nil, fmt.Errorf("%s: %w", abs, err)
	}
	return c, nil
}

func checkVersion(out string) error {
	m := versionRe.FindStringSubmatch(out)
	if m == nil {
		return fmt.Errorf("unrecognised version output %q", strings.TrimSpace(out))
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	if major < 2 || (major == 2 && minor < 9) {
		return fmt.Errorf("aws CLI %s.%s.%s is too old, need >= 2.9 for export-credentials", m[1], m[2], m[3])
	}
	return nil
}

type processCreds struct {
	Version         int    `json:"Version"`
	AccessKeyID     string `json:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey"`
	SessionToken    string `json:"SessionToken"`
	Expiration      string `json:"Expiration"`
}

func (c *CLI) Mint(ctx context.Context, profile string) (Creds, error) {
	out, err := c.run(ctx, "configure", "export-credentials", "--profile", profile, "--format", "process")
	if err != nil {
		return Creds{}, err
	}

	var pc processCreds
	if err := json.Unmarshal([]byte(out), &pc); err != nil {
		return Creds{}, errors.New("export-credentials: unparseable output")
	}
	creds := Creds{
		AccessKeyID:     pc.AccessKeyID,
		SecretAccessKey: pc.SecretAccessKey,
		SessionToken:    pc.SessionToken,
	}
	if creds.AccessKeyID == "" || creds.SecretAccessKey == "" {
		return Creds{}, errors.New("export-credentials: incomplete credentials")
	}
	for _, v := range []string{creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken} {
		if strings.ContainsAny(v, "\r\n") {
			return Creds{}, errors.New("export-credentials: unexpected characters in credentials")
		}
	}
	if pc.Expiration != "" {
		exp, err := time.Parse(time.RFC3339, pc.Expiration)
		if err != nil {
			return Creds{}, fmt.Errorf("export-credentials: bad expiration %q", pc.Expiration)
		}
		creds.Expiration = exp
	}
	return creds, nil
}

// Region returns the profile's configured region, or "" if none is set.
func (c *CLI) Region(ctx context.Context, profile string) (string, error) {
	out, err := c.run(ctx, "configure", "get", "region", "--profile", profile)
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && strings.TrimSpace(out) == "" {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (c *CLI) run(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.Path, args...)
	cmd.Env = StripEnv(os.Environ(), append(CredentialEnv, "AWS_PROFILE", "AWS_DEFAULT_PROFILE")...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		msg := firstLine(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), &cliError{args: args, msg: msg, err: err}
	}
	return stdout.String(), nil
}

type cliError struct {
	args []string
	msg  string
	err  error
}

func (e *cliError) Error() string {
	n := min(len(e.args), 2)
	return fmt.Sprintf("aws %s: %s", strings.Join(e.args[:n], " "), e.msg)
}

func (e *cliError) Unwrap() error { return e.err }

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}
