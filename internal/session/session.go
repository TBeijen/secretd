package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/tbeijen/secretd/internal/aws"
)

type Session struct {
	Dir string
}

type Entry struct {
	Name   string
	Region string
	Creds  aws.Creds
}

// Create makes a fresh session dir under base. Stale dirs of dead secretd
// processes are removed first.
func Create(base string) (*Session, error) {
	if err := ensureBase(base); err != nil {
		return nil, err
	}
	sweep(base)

	dir, err := os.MkdirTemp(base, strconv.Itoa(os.Getpid())+"-")
	if err != nil {
		return nil, err
	}
	if err := os.Mkdir(filepath.Join(dir, "aws"), 0o700); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	return &Session{Dir: dir}, nil
}

func (s *Session) AWSConfigFile() string      { return filepath.Join(s.Dir, "aws", "config") }
func (s *Session) AWSCredentialsFile() string { return filepath.Join(s.Dir, "aws", "credentials") }

func (s *Session) Remove() error { return os.RemoveAll(s.Dir) }

// Write renders config and credentials, replacing both files atomically.
func (s *Session) Write(entries []Entry) error {
	entries = append([]Entry(nil), entries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })

	var cfg, creds strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&cfg, "[profile %s]\n", e.Name)
		if e.Region != "" {
			fmt.Fprintf(&cfg, "region = %s\n", e.Region)
		}
		cfg.WriteString("\n")

		fmt.Fprintf(&creds, "[%s]\n", e.Name)
		fmt.Fprintf(&creds, "aws_access_key_id = %s\n", e.Creds.AccessKeyID)
		fmt.Fprintf(&creds, "aws_secret_access_key = %s\n", e.Creds.SecretAccessKey)
		if e.Creds.SessionToken != "" {
			fmt.Fprintf(&creds, "aws_session_token = %s\n", e.Creds.SessionToken)
		}
		creds.WriteString("\n")
	}

	if err := writeFile(s.AWSConfigFile(), cfg.String()); err != nil {
		return err
	}
	return writeFile(s.AWSCredentialsFile(), creds.String())
}

func writeFile(path, content string) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)

	if _, err := f.WriteString(content); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(0o400); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ensureBase creates base if needed and refuses one that others can access.
func ensureBase(base string) error {
	if err := os.MkdirAll(base, 0o700); err != nil {
		return err
	}
	fi, err := os.Lstat(base)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s: not a directory", base)
	}
	if fi.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%s: permissions %o, want 0700", base, fi.Mode().Perm())
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
		return fmt.Errorf("%s: not owned by current user", base)
	}
	return nil
}

func sweep(base string) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return
	}
	for _, e := range entries {
		pidStr, _, ok := strings.Cut(e.Name(), "-")
		if !ok || !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(pidStr)
		if err != nil || pid == os.Getpid() || alive(pid) {
			continue
		}
		os.RemoveAll(filepath.Join(base, e.Name()))
	}
}

func alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
