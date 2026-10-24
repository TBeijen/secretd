package session

import (
	"context"
	"fmt"
	"time"

	"github.com/tbeijen/secretd/internal/aws"
)

const (
	DefaultInterval = 30 * time.Minute
	RetryInterval   = time.Minute
	minInterval     = 30 * time.Second
)

type Profile struct {
	Name   string
	Source string
	Region string
}

// MintAll mints every profile and fails on the first error.
func MintAll(ctx context.Context, m aws.Minter, profiles []Profile) ([]Entry, error) {
	entries := make([]Entry, 0, len(profiles))
	for _, p := range profiles {
		creds, err := m.Mint(ctx, p.Source)
		if err != nil {
			return nil, fmt.Errorf("profile %s: %w", p.Name, err)
		}
		entries = append(entries, Entry{Name: p.Name, Region: p.Region, Creds: creds})
	}
	return entries, nil
}

// Refresh re-mints every profile into current. A profile that fails keeps
// its previous credentials, which may still be valid.
func Refresh(ctx context.Context, m aws.Minter, profiles []Profile, current map[string]Entry) []error {
	var errs []error
	for _, p := range profiles {
		creds, err := m.Mint(ctx, p.Source)
		if err != nil {
			errs = append(errs, fmt.Errorf("profile %s: %w", p.Name, err))
			continue
		}
		current[p.Name] = Entry{Name: p.Name, Region: p.Region, Creds: creds}
	}
	return errs
}

// NextRefresh is halfway to the earliest expiration, clamped to
// [minInterval, DefaultInterval].
func NextRefresh(now time.Time, entries []Entry) time.Duration {
	next := DefaultInterval
	for _, e := range entries {
		if e.Creds.Expiration.IsZero() {
			continue
		}
		if d := e.Creds.Expiration.Sub(now) / 2; d < next {
			next = d
		}
	}
	return max(next, minInterval)
}
