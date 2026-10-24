# secretd

## What this is

A Go CLI that mints short-lived AWS credentials for opted-in profiles and starts a child command with them. It runs **outside** the sandbox, wrapping e.g. `fnox exec -- nono run ...`.

Design and scope live in `issues/`. Read the relevant issue before implementing.

## Building

```sh
mkdir -p bin
go build -o bin/secretd .
```

## Testing

```sh
go test ./...
```

Tests must never call AWS. Put a fake `aws` script on `PATH` (under `testdata/`) that returns fixed JSON.

## Project layout (planned)

```
main.go              # CLI entry, os.Args dispatch
internal/
  cmd/               # run (wrap child), ls
  config/            # contexts, matching, TOML parsing
  aws/               # Minter interface + CLI-based implementation
  session/           # session folder, rendering, atomic writes, refresh
testdata/
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
