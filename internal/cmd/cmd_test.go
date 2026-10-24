package cmd

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setup creates a config dir with one context matching a fresh project dir,
// puts the fake aws CLI on PATH and chdirs into the project.
func setup(t *testing.T) (project string) {
	t.Helper()
	fakeDir, err := filepath.Abs("../../testdata/fakeaws")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	project = t.TempDir()
	cfg := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cfg, "contexts"), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx := `match = ["` + project + `"]

[aws.profiles.dev-ro]
region = "eu-west-1"

[aws.profiles.prod-ro]
source = "acme-prod-ReadOnly"
`
	if err := os.WriteFile(filepath.Join(cfg, "contexts", "work.toml"), []byte(ctx), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SECRETD_CONFIG_DIR", cfg)
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("FAKE_AWS_REGIONS", "acme-prod-ReadOnly=us-east-1")
	t.Chdir(project)
	return project
}

const childScript = `
{
  echo "arg=$1"
  echo "dir=$SECRETD_DIR"
  echo "profile=$AWS_PROFILE"
  echo "key=$AWS_ACCESS_KEY_ID"
  cat "$AWS_CONFIG_FILE" "$AWS_SHARED_CREDENTIALS_FILE"
} > "$OUT"
exit "${CODE:-0}"
`

func TestExec(t *testing.T) {
	setup(t)
	out := filepath.Join(t.TempDir(), "out")
	t.Setenv("OUT", out)
	t.Setenv("AWS_ACCESS_KEY_ID", "leaked")
	t.Setenv("AWS_PROFILE", "admin")

	err := Exec([]string{"--", "sh", "-c", childScript, "sh", Placeholder + "/aws"})
	if err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)

	dir := lineValue(got, "dir=")
	if !strings.HasPrefix(dir, filepath.Join(os.Getenv("TMPDIR"), "secretd")) {
		t.Fatalf("dir = %q", dir)
	}
	for _, want := range []string{
		"arg=" + dir + "/aws\n",
		"profile=\n",
		"key=\n",
		"[profile dev-ro]\nregion = eu-west-1\n",
		"[profile prod-ro]\nregion = us-east-1\n",
		"[prod-ro]\naws_access_key_id = AKIA-acme-prod-ReadOnly\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("child output missing %q:\n%s", want, got)
		}
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("session dir not removed after exit")
	}
}

func TestExecKeepsAllowedProfile(t *testing.T) {
	setup(t)
	out := filepath.Join(t.TempDir(), "out")
	t.Setenv("OUT", out)
	t.Setenv("AWS_PROFILE", "dev-ro")

	if err := Exec([]string{"--", "sh", "-c", childScript}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(out)
	if !strings.Contains(string(b), "profile=dev-ro\n") {
		t.Fatalf("AWS_PROFILE not kept:\n%s", b)
	}
}

func TestExecExitCode(t *testing.T) {
	setup(t)
	t.Setenv("OUT", filepath.Join(t.TempDir(), "out"))
	t.Setenv("CODE", "3")

	err := Exec([]string{"--", "sh", "-c", childScript})
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 3 {
		t.Fatalf("err = %v", err)
	}
}

func TestExecMintFailureStartsNothing(t *testing.T) {
	setup(t)
	marker := filepath.Join(t.TempDir(), "ran")
	t.Setenv("FAKE_AWS_FAIL", "acme-prod-ReadOnly")

	err := Exec([]string{"--", "touch", marker})
	if err == nil || !strings.Contains(err.Error(), "prod-ro") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("child ran despite mint failure")
	}
}

func TestExecMissingProfile(t *testing.T) {
	setup(t)
	marker := filepath.Join(t.TempDir(), "ran")

	// dev-ro has a region in the context, so only the mint call hits the
	// missing profile; prod-ro fails earlier, at the region lookup.
	for _, missing := range []string{"dev-ro", "acme-prod-ReadOnly"} {
		t.Run(missing, func(t *testing.T) {
			t.Setenv("FAKE_AWS_MISSING", missing)
			err := Exec([]string{"--", "touch", marker})
			want := "(" + missing + ") could not be found"
			if err == nil || !strings.Contains(err.Error(), want) || strings.Contains(err.Error(), "[ERROR]") {
				t.Fatalf("err = %v", err)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("child ran despite missing profile")
			}
		})
	}
}

func TestExecNoContext(t *testing.T) {
	setup(t)
	t.Chdir(t.TempDir())
	err := Exec([]string{"--", "true"})
	if err == nil || !strings.Contains(err.Error(), "no context matches") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseExecArgs(t *testing.T) {
	tests := []struct {
		args        []string
		wantContext string
		wantChild   string
		wantErr     bool
	}{
		{args: []string{"--", "a", "b"}, wantChild: "a b"},
		{args: []string{"--context", "work", "--", "a"}, wantContext: "work", wantChild: "a"},
		{args: []string{"--context=work", "--", "a", "--context"}, wantContext: "work", wantChild: "a --context"},
		{args: []string{"--"}, wantErr: true},
		{args: []string{"a"}, wantErr: true},
		{args: []string{"--context"}, wantErr: true},
		{args: []string{}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			ctx, child, err := parseExecArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if ctx != tt.wantContext || strings.Join(child, " ") != tt.wantChild {
				t.Fatalf("got %q %v", ctx, child)
			}
		})
	}
}

func lineValue(s, prefix string) string {
	for _, l := range strings.Split(s, "\n") {
		if v, ok := strings.CutPrefix(l, prefix); ok {
			return v
		}
	}
	return ""
}

func TestContextShow(t *testing.T) {
	setup(t)
	var b strings.Builder
	if err := ContextCmd([]string{"show"}, &b); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"context: work\n", "  dev-ro  (region: eu-west-1)\n", "  prod-ro  (source: acme-prod-ReadOnly)\n"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("missing %q:\n%s", want, b.String())
		}
	}
}

func TestContextList(t *testing.T) {
	project := setup(t)
	t.Chdir(t.TempDir())
	var b strings.Builder
	if err := ContextCmd([]string{"list"}, &b); err != nil {
		t.Fatal(err)
	}
	want := "CONTEXT  MATCH" + strings.Repeat(" ", len(project)-3) + "PROFILES\n" +
		"work     " + project + "  dev-ro, prod-ro\n"
	if b.String() != want {
		t.Fatalf("got:\n%s\nwant:\n%s", b.String(), want)
	}
}

func TestContextUsage(t *testing.T) {
	setup(t)
	for _, args := range [][]string{{}, {"ls"}, {"list", "x"}, {"show", "x"}} {
		if err := ContextCmd(args, io.Discard); err == nil {
			t.Errorf("%v: expected error", args)
		}
	}
}
