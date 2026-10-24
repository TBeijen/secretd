package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeContext(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, "contexts", name+".toml")
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSelect(t *testing.T) {
	root := t.TempDir()
	projects := filepath.Join(root, "projects")
	for _, d := range []string{"work/app", "workshop", "work/special/x", "elsewhere"} {
		if err := os.MkdirAll(filepath.Join(projects, d), 0o700); err != nil {
			t.Fatal(err)
		}
	}

	cfg := t.TempDir()
	writeContext(t, cfg, "work", `match = ["`+filepath.Join(projects, "work")+`"]
[aws.profiles.a]
`)
	writeContext(t, cfg, "special", `match = ["`+filepath.Join(projects, "work", "special")+`"]
[aws.profiles.b]
`)
	contexts, err := LoadContexts(cfg)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		cwd, name, want, wantErr string
	}{
		{cwd: "work", want: "work"},
		{cwd: "work/app", want: "work"},
		{cwd: "work/special/x", want: "special"},
		{cwd: "workshop", wantErr: "no context matches"},
		{cwd: "elsewhere", wantErr: "no context matches"},
		{cwd: "elsewhere", name: "work", want: "work"},
		{cwd: "work", name: "nope", wantErr: "unknown context"},
	}
	for _, tt := range tests {
		t.Run(tt.cwd+"/"+tt.name, func(t *testing.T) {
			c, err := Select(contexts, tt.name, filepath.Join(projects, tt.cwd))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if c.Name != tt.want {
				t.Fatalf("got %s, want %s", c.Name, tt.want)
			}
		})
	}
}

func TestSelectAmbiguous(t *testing.T) {
	dir := t.TempDir()
	cfg := t.TempDir()
	writeContext(t, cfg, "a", `match = ["`+dir+`"]
[aws.profiles.x]
`)
	writeContext(t, cfg, "b", `match = ["`+dir+`"]
[aws.profiles.y]
`)
	contexts, err := LoadContexts(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Select(contexts, "", dir); err == nil || !strings.Contains(err.Error(), "multiple contexts") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadContextsValidation(t *testing.T) {
	tests := []struct {
		name, content, wantErr string
	}{
		{"unknown key", "match = [\"/x\"]\nprofile = \"a\"\n[aws.profiles.a]\n", "unknown key"},
		{"no profiles", "match = [\"/x\"]\n", "no [aws.profiles.*]"},
		{"relative match", "match = [\"projects/x\"]\n[aws.profiles.a]\n", "must be absolute"},
		{"bad name", "[aws.profiles.\"a]b\"]\n", "invalid name"},
		{"bad source", "[aws.profiles.a]\nsource = \"x\\ny\"\n", "invalid source"},
		{"bad region", "[aws.profiles.a]\nregion = \"EU WEST\"\n", "invalid region"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := t.TempDir()
			writeContext(t, cfg, "c", tt.content)
			_, err := LoadContexts(cfg)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestSourceProfile(t *testing.T) {
	if got := (Profile{}).SourceProfile("a"); got != "a" {
		t.Fatalf("got %s", got)
	}
	if got := (Profile{Source: "b"}).SourceProfile("a"); got != "b" {
		t.Fatalf("got %s", got)
	}
}

func TestLoadSettings(t *testing.T) {
	cfg := t.TempDir()
	s, err := LoadSettings(cfg)
	if err != nil || s.AWSCLI != "" {
		t.Fatalf("missing file: %+v, %v", s, err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "config.toml"), []byte(`aws_cli = "/opt/aws"`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err = LoadSettings(cfg)
	if err != nil || s.AWSCLI != "/opt/aws" {
		t.Fatalf("got %+v, %v", s, err)
	}
}
