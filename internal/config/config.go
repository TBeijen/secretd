package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

type Settings struct {
	AWSCLI string `toml:"aws_cli"`
}

type Profile struct {
	Source string `toml:"source"`
	Region string `toml:"region"`
}

type AWS struct {
	Profiles map[string]Profile `toml:"profiles"`
}

type Context struct {
	Name  string   `toml:"-"`
	Path  string   `toml:"-"`
	Match []string `toml:"match"`
	AWS   AWS      `toml:"aws"`
}

// SourceProfile returns the ~/.aws/config profile a sandbox profile is minted from.
func (p Profile) SourceProfile(name string) string {
	if p.Source != "" {
		return p.Source
	}
	return name
}

// ProfileNames returns the sandbox-visible profile names, sorted.
func (c *Context) ProfileNames() []string {
	names := make([]string, 0, len(c.AWS.Profiles))
	for n := range c.AWS.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Profile names end up as INI section headers, so keep them strict.
var (
	profileNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	regionRe      = regexp.MustCompile(`^[a-z0-9-]+$`)
)

func Dir() (string, error) {
	if d := os.Getenv("SECRETD_CONFIG_DIR"); d != "" {
		return d, nil
	}
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "secretd"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "secretd"), nil
}

func LoadSettings(dir string) (Settings, error) {
	var s Settings
	path := filepath.Join(dir, "config.toml")
	md, err := toml.DecodeFile(path, &s)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("%s: %w", path, err)
	}
	if err := checkUndecoded(path, md); err != nil {
		return s, err
	}
	return s, nil
}

func LoadContexts(dir string) ([]Context, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "contexts", "*.toml"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)

	contexts := make([]Context, 0, len(paths))
	for _, p := range paths {
		var c Context
		md, err := toml.DecodeFile(p, &c)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if err := checkUndecoded(p, md); err != nil {
			return nil, err
		}
		c.Name = strings.TrimSuffix(filepath.Base(p), ".toml")
		c.Path = p
		if err := c.validate(); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		contexts = append(contexts, c)
	}
	return contexts, nil
}

// Select returns the named context, or else the one whose match path is the
// longest prefix of cwd.
func Select(contexts []Context, name, cwd string) (*Context, error) {
	if name != "" {
		for i := range contexts {
			if contexts[i].Name == name {
				return &contexts[i], nil
			}
		}
		return nil, fmt.Errorf("unknown context %q", name)
	}

	cwd = resolve(cwd)
	var best *Context
	bestLen := -1
	ambiguous := false
	for i := range contexts {
		for _, m := range contexts[i].Match {
			m = resolve(expandHome(m))
			if !within(cwd, m) {
				continue
			}
			switch {
			case len(m) > bestLen:
				best, bestLen, ambiguous = &contexts[i], len(m), false
			case len(m) == bestLen && best != &contexts[i]:
				ambiguous = true
			}
		}
	}
	if ambiguous {
		return nil, fmt.Errorf("multiple contexts match %s, use --context", cwd)
	}
	if best == nil {
		return nil, fmt.Errorf("no context matches %s", cwd)
	}
	return best, nil
}

func (c *Context) validate() error {
	if len(c.AWS.Profiles) == 0 {
		return errors.New("no [aws.profiles.*] defined")
	}
	for _, m := range c.Match {
		if !filepath.IsAbs(expandHome(m)) {
			return fmt.Errorf("match %q: must be absolute or start with ~/", m)
		}
	}
	for name, p := range c.AWS.Profiles {
		if !profileNameRe.MatchString(name) {
			return fmt.Errorf("profile %q: invalid name", name)
		}
		if p.Source != "" && !profileNameRe.MatchString(p.Source) {
			return fmt.Errorf("profile %q: invalid source %q", name, p.Source)
		}
		if p.Region != "" && !regionRe.MatchString(p.Region) {
			return fmt.Errorf("profile %q: invalid region %q", name, p.Region)
		}
	}
	return nil
}

func checkUndecoded(path string, md toml.MetaData) error {
	if keys := md.Undecoded(); len(keys) > 0 {
		return fmt.Errorf("%s: unknown key %q", path, keys[0].String())
	}
	return nil
}

func expandHome(p string) string {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~"))
}

// resolve follows symlinks where possible, so /tmp and /private/tmp compare equal.
func resolve(p string) string {
	p = filepath.Clean(p)
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

func within(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
