# 01 — Mint credentials for opted-in SSO profiles

## Goal

`secretd exec -- <cmd>` mints short-lived STS credentials for an allow-listed set of AWS profiles and exposes only those to `<cmd>` (typically `fnox exec -- nono run ... -- claude`).

Port of `nono-pocs/fnox-socket-poc/secretd-poc.sh` to Go.

## Config

Central, outside sandbox-writable paths (chezmoi-managed):

```toml
# ~/.config/secretd/contexts/work.toml
match = ["~/projects/work"]

[aws.profiles.acme-nonprod-ro]
region = "eu-west-1"          # optional, else taken from source profile

[aws.profiles.acme-prod-ro]
source = "acme-prod-ReadOnly"   # optional alias; defaults to key
```

- Context = longest `match` prefix of cwd. `--context <name>` overrides.
- No match → error, no credentials minted.
- Never read config from cwd or its parents.

## Behaviour

- Per profile: `aws configure export-credentials --profile <source> --format process`.
- Session dir is owned by secretd:
  - Created with `os.MkdirTemp` under `$TMPDIR/secretd/` (dir 0700; files 0400, written to a temp file then renamed).
  - Before exec, secretd replaces `@SECRETD_DIR@` in the child's argv with the real path. This gets it into `nono run --read ...`, which the shell has already expanded by then.
  - Also exported as `SECRETD_DIR` for scripts and debugging.
- Rendered `config` holds only `[profile <name>]` + `region`. No `sso_*` keys.
- Refresh at ~50% of the remaining TTL (from `Expiration`).
- Child env: set `AWS_CONFIG_FILE`, `AWS_SHARED_CREDENTIALS_FILE`, `SECRETD_DIR`. Strip `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`. Keep `AWS_PROFILE` only if it names an exported profile.
- Clean up on child exit. On start, sweep stale session dirs.
- Failed refresh (expired SSO session): warn on stderr and keep the child running. Retry every ~60s until it succeeds, so an `aws sso login` outside the sandbox takes effect within a minute. Never start `aws sso login` from secretd.

## CLI

```
secretd exec [--context X] -- <cmd>...
secretd context show      # resolved context + profiles for cwd
secretd context list      # all contexts
```

Wrapper sketch:

```zsh
secretd exec -- fnox exec -- nono run --profile <nono-profile> --read @SECRETD_DIR@ --allow-cwd -- claude "$@"
```

## Dependencies

Stdlib + `BurntSushi/toml` only. Talk to AWS by shelling out to the `aws` CLI, not via aws-sdk-go-v2.

- Hide this behind a small interface (`Minter.Mint(ctx, profile) (Creds, error)`), so an SDK-based implementation can be added later, e.g. for remote agent environments without the CLI.
- Resolve `aws` to an absolute path once at startup (config `aws_cli = ...`, default a PATH lookup), and log it. Installing it somewhere the sandbox can't write to is up to the user.
- Requires AWS CLI v2 ≥ 2.9 (`export-credentials`). Check the version at startup with a clear error.

## Done when

- A wrapper (e.g. `secretd exec -- nono run ... -- claude`) works, and inside the sandbox `aws sts get-caller-identity --profile acme-nonprod-ro` works.
- A profile not in the context is not visible (`profile not found`).
- Tests use a fake `aws` binary, not real AWS.
