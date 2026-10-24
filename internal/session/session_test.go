package session

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/tbeijen/secretd/internal/aws"
)

func TestCreateAndWrite(t *testing.T) {
	base := filepath.Join(t.TempDir(), "secretd")
	s, err := Create(base)
	if err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(s.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode %o", fi.Mode().Perm())
	}

	entries := []Entry{
		{Name: "b", Creds: aws.Creds{AccessKeyID: "AK-b", SecretAccessKey: "s-b"}},
		{Name: "a", Region: "eu-west-1", Creds: aws.Creds{AccessKeyID: "AK-a", SecretAccessKey: "s-a", SessionToken: "t-a"}},
	}
	if err := s.Write(entries); err != nil {
		t.Fatal(err)
	}
	// Second write must replace the read-only files.
	if err := s.Write(entries); err != nil {
		t.Fatal(err)
	}

	assertFile(t, s.AWSConfigFile(), "[profile a]\nregion = eu-west-1\n\n[profile b]\n\n")
	assertFile(t, s.AWSCredentialsFile(),
		"[a]\naws_access_key_id = AK-a\naws_secret_access_key = s-a\naws_session_token = t-a\n\n"+
			"[b]\naws_access_key_id = AK-b\naws_secret_access_key = s-b\n\n")

	leftovers, _ := filepath.Glob(filepath.Join(s.Dir, "aws", ".tmp-*"))
	if len(leftovers) != 0 {
		t.Fatalf("temp files left: %v", leftovers)
	}

	if err := s.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.Dir); !os.IsNotExist(err) {
		t.Fatal("session dir not removed")
	}
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != want {
		t.Fatalf("%s:\n%s\nwant:\n%s", path, b, want)
	}
	fi, _ := os.Stat(path)
	if fi.Mode().Perm() != 0o400 {
		t.Fatalf("%s mode %o", path, fi.Mode().Perm())
	}
}

func TestCreateRefusesOpenBase(t *testing.T) {
	base := filepath.Join(t.TempDir(), "secretd")
	if err := os.Mkdir(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(base); err == nil {
		t.Fatal("accepted a 0755 base dir")
	}
}

func TestSweep(t *testing.T) {
	base := filepath.Join(t.TempDir(), "secretd")
	if err := os.Mkdir(base, 0o700); err != nil {
		t.Fatal(err)
	}

	dead := exec.Command("true")
	if err := dead.Run(); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(base, strconv.Itoa(dead.Process.Pid)+"-x")
	live := filepath.Join(base, strconv.Itoa(os.Getppid())+"-x")
	other := filepath.Join(base, "not-a-session")
	for _, d := range []string{stale, live, other} {
		if err := os.Mkdir(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := Create(base); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("stale dir not swept")
	}
	for _, d := range []string{live, other} {
		if _, err := os.Stat(d); err != nil {
			t.Errorf("%s removed", d)
		}
	}
}

func TestNextRefresh(t *testing.T) {
	now := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	entry := func(d time.Duration) Entry {
		return Entry{Creds: aws.Creds{Expiration: now.Add(d)}}
	}

	tests := []struct {
		name    string
		entries []Entry
		want    time.Duration
	}{
		{"no expiration", []Entry{{}}, DefaultInterval},
		{"one hour", []Entry{entry(time.Hour)}, DefaultInterval},
		{"earliest wins", []Entry{entry(time.Hour), entry(20 * time.Minute)}, 10 * time.Minute},
		{"clamped", []Entry{entry(10 * time.Second)}, minInterval},
		{"expired", []Entry{entry(-time.Minute)}, minInterval},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NextRefresh(now, tt.entries); got != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}

type fakeMinter struct {
	fail map[string]bool
}

func (f fakeMinter) Mint(_ context.Context, profile string) (aws.Creds, error) {
	if f.fail[profile] {
		return aws.Creds{}, errors.New("expired")
	}
	return aws.Creds{AccessKeyID: "new-" + profile, SecretAccessKey: "s"}, nil
}

func (f fakeMinter) Region(context.Context, string) (string, error) { return "", nil }

func TestMintAllFailsOnError(t *testing.T) {
	profiles := []Profile{{Name: "a", Source: "src-a"}, {Name: "b", Source: "src-b"}}
	_, err := MintAll(context.Background(), fakeMinter{fail: map[string]bool{"src-b": true}}, profiles)
	if err == nil {
		t.Fatal("expected error")
	}

	entries, err := MintAll(context.Background(), fakeMinter{}, profiles)
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].Name != "a" || entries[0].Creds.AccessKeyID != "new-src-a" {
		t.Fatalf("got %+v", entries[0])
	}
}

func TestRefreshKeepsPreviousOnFailure(t *testing.T) {
	profiles := []Profile{{Name: "a", Source: "a"}, {Name: "b", Source: "b"}}
	current := map[string]Entry{
		"a": {Name: "a", Creds: aws.Creds{AccessKeyID: "old-a"}},
		"b": {Name: "b", Creds: aws.Creds{AccessKeyID: "old-b"}},
	}

	errs := Refresh(context.Background(), fakeMinter{fail: map[string]bool{"b": true}}, profiles, current)
	if len(errs) != 1 {
		t.Fatalf("errs = %v", errs)
	}
	if current["a"].Creds.AccessKeyID != "new-a" || current["b"].Creds.AccessKeyID != "old-b" {
		t.Fatalf("got %+v", current)
	}
}
