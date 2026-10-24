package aws

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fakeCLI(t *testing.T) *CLI {
	t.Helper()
	path, err := filepath.Abs("../../testdata/fakeaws/aws")
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewCLI(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNewCLIVersion(t *testing.T) {
	path, _ := filepath.Abs("../../testdata/fakeaws/aws")
	t.Setenv("FAKE_AWS_VERSION", "aws-cli/2.8.1 Python/3.9")
	_, err := NewCLI(context.Background(), path)
	if err == nil || !strings.Contains(err.Error(), "too old") {
		t.Fatalf("err = %v", err)
	}

	t.Setenv("FAKE_AWS_VERSION", "aws-cli/1.33.0 Python/3.9")
	if _, err := NewCLI(context.Background(), path); err == nil {
		t.Fatal("v1 accepted")
	}
}

func TestNewCLIFromPath(t *testing.T) {
	dir, _ := filepath.Abs("../../testdata/fakeaws")
	t.Setenv("PATH", dir)
	c, err := NewCLI(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if c.Path != filepath.Join(dir, "aws") {
		t.Fatalf("path = %s", c.Path)
	}
}

func TestMint(t *testing.T) {
	c := fakeCLI(t)
	t.Setenv("FAKE_AWS_EXPIRATION", "2030-05-01T10:00:00+00:00")

	creds, err := c.Mint(context.Background(), "dev")
	if err != nil {
		t.Fatal(err)
	}
	want := Creds{
		AccessKeyID:     "AKIA-dev",
		SecretAccessKey: "secret-dev",
		SessionToken:    "token-dev",
		Expiration:      time.Date(2030, 5, 1, 10, 0, 0, 0, time.UTC),
	}
	if creds.AccessKeyID != want.AccessKeyID || creds.SecretAccessKey != want.SecretAccessKey ||
		creds.SessionToken != want.SessionToken || !creds.Expiration.Equal(want.Expiration) {
		t.Fatalf("got %+v", creds)
	}
}

func TestMintFailure(t *testing.T) {
	c := fakeCLI(t)
	t.Setenv("FAKE_AWS_FAIL", "prod")
	_, err := c.Mint(context.Background(), "prod")
	if err == nil || !strings.Contains(err.Error(), "Token has expired") {
		t.Fatalf("err = %v", err)
	}
}

func TestMintStripsCredentialEnv(t *testing.T) {
	c := fakeCLI(t)
	log := filepath.Join(t.TempDir(), "env.log")
	t.Setenv("FAKE_AWS_ENV_LOG", log)
	t.Setenv("AWS_ACCESS_KEY_ID", "leaked")
	t.Setenv("AWS_PROFILE", "admin")

	if _, err := c.Mint(context.Background(), "dev"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(log)
	if len(b) != 0 {
		t.Fatalf("aws CLI saw: %s", b)
	}
}

func TestRegion(t *testing.T) {
	c := fakeCLI(t)
	t.Setenv("FAKE_AWS_REGIONS", "dev=eu-west-1")

	if r, err := c.Region(context.Background(), "dev"); err != nil || r != "eu-west-1" {
		t.Fatalf("got %q, %v", r, err)
	}
	if r, err := c.Region(context.Background(), "other"); err != nil || r != "" {
		t.Fatalf("unset region: got %q, %v", r, err)
	}
}

func TestStripEnv(t *testing.T) {
	got := StripEnv([]string{"A=1", "B=2", "AB=3"}, "A")
	if strings.Join(got, ",") != "B=2,AB=3" {
		t.Fatalf("got %v", got)
	}
}
