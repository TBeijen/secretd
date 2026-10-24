package aws

import (
	"context"
	"strings"
	"time"
)

type Creds struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Expiration      time.Time
}

type Minter interface {
	Mint(ctx context.Context, profile string) (Creds, error)
	Region(ctx context.Context, profile string) (string, error)
}

// CredentialEnv are variables that would let a process bypass the minted
// profiles. They are stripped from both the aws CLI and the child environment.
var CredentialEnv = []string{
	"AWS_ACCESS_KEY_ID",
	"AWS_SECRET_ACCESS_KEY",
	"AWS_SESSION_TOKEN",
	"AWS_SECURITY_TOKEN",
	"AWS_CREDENTIAL_EXPIRATION",
}

func StripEnv(env []string, keys ...string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		drop := false
		for _, s := range keys {
			if k == s {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, kv)
		}
	}
	return out
}
