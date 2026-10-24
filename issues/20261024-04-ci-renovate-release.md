# 04 — CI, Renovate, vulnerability scanning, releases

## Goal

Tested, scanned, released binaries that mise can install. Few dependencies, few releases.

## CI

- Reuse the go-workspace setup: `ci.yaml` (`go vet`, `go test`), `release.yaml` (goreleaser on `v*` tags).
- Tests never touch AWS: a fake `aws` on `PATH` returns fixed `export-credentials` / `assume-role` JSON.
- Pin actions to commit SHAs. This binary mints cloud credentials, so its supply chain matters more than usual.

## Vulnerability scanning

- `govulncheck ./...` on every PR, plus a weekly scheduled run on `main`.
  - It only reports vulnerabilities in code that's actually called, stdlib included.
  - No findings means no reason to release.
- Go toolchain updates count: stdlib CVEs only reach users through a rebuild and release.

## Renovate

- Go modules, GitHub Actions, `.mise.toml` tools.
- Cooldown: `minimumReleaseAge` 7 days. Security updates (`osvVulnerabilityAlerts`) bypass it.
- Automerge patch/minor/digest updates once CI is green. Majors are manual.

## Release

- Manual tagging. Release when there's a feature, or when the weekly `govulncheck` reports a finding.
- Install via mise, `github:TBeijen/secretd`. Never from the repo's `./bin`: `~/projects` is writable from the sandbox.

## Dependencies

- Keep to stdlib + `BurntSushi/toml`.
- AWS goes through the `aws` CLI (`export-credentials`, `sts assume-role`), so there are no SDK modules.
- The egress IP check uses `net/http`.

## Open questions

- Build provenance (`actions/attest-build-provenance`) so installs can be verified?
