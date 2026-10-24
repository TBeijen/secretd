# secretd

## What this is

A Go CLI that mints short-lived AWS credentials for opted-in profiles and starts a child command with them. It runs **outside** the sandbox, wrapping e.g. `fnox exec -- nono run ...`.

Design and scope live in `issues/`. Read the relevant issue before implementing.

## Building and testing

Tooling (Go, hk) via mise: `mise install`. Tasks:

```sh
mise run build   # bin/secretd
mise run test    # go test -race ./...
mise run lint    # hk check --all: gofmt, go vet, whitespace, newlines
mise run fix     # hk fix --all
mise run ci      # lint + test + build
```

Run `mise run ci` before handing back work.

Tests must never call AWS. `testdata/fakeaws/aws` stands in for the CLI; its behaviour is set via `FAKE_AWS_*` env vars (see the script header).

## Project layout

```
main.go              # CLI entry, os.Args dispatch
internal/
  cmd/               # run (mint, wrap child, signals, env), ls
  config/            # contexts, matching, TOML parsing, validation
  aws/               # Minter interface + CLI implementation, env stripping
  session/           # session dir, rendering, atomic writes, refresh timing
testdata/
  fakeaws/aws        # fake AWS CLI for tests
examples/config/     # example config dir; loaded by a test, keep it valid
```

## Design principles

- Config is central (`~/.config/secretd/contexts/*.toml`). Never read config from the cwd or its parents.
- Least privilege by default: no matching context means nothing is minted.
- The child gets only the opted-in profiles. Strip inherited `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`.
- AWS access goes through the `aws` CLI behind the `Minter` interface. No AWS SDK for now.
- Fail loudly at startup, degrade gracefully at refresh (warn, retry, keep the child running).

## Code style

- Standard Go: `gofmt`, `go vet`.
- No CLI framework. Subcommand dispatch via `os.Args`.
- Dependencies: stdlib + `github.com/BurntSushi/toml`. Ask before adding any.
- No comments unless the why is non-obvious.
- Errors and logs go to stderr. Never mix them into stdout.

## Never

- Print, log or include credential values in errors. Use profile names and expiry times only.
- Write credentials anywhere but the session folder.
- Run `aws sso login` or other interactive flows.
