# secretd

## What this is

A Go CLI that mints short-lived AWS credentials for opted-in profiles and starts a child command with them. It runs **outside** the sandbox, wrapping e.g. `fnox exec -- nono run ...`.

Specs are GitHub issues (older ones in `issues/`). Read the relevant issue before implementing.

Workflow (draft) and the security review checklist: `docs/workflow.md`.

## Building and testing

Tooling (Go, hk) via mise: `mise install`. Tasks:

```sh
mise run setup   # once per clone: HTTPS remote, gh credentials, push mode, hk hooks
mise run build   # bin/secretd
mise run test    # go test -race ./...
mise run lint    # hk check --all: gofmt, go vet, whitespace, newlines
mise run fix     # hk fix --all
mise run ci      # lint + test + build
```

Run `mise run ci` before handing back work. CI runs the same task.

Releases: push a `v*` tag, goreleaser builds via `.github/workflows/release.yaml`. Actions are pinned to commit SHAs.

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

## Commits and PRs

Conventional Commits: `<type>: <summary>`, lowercase, imperative, no trailing period. Optionally a scope: `fix(session): ...`.

| Type | For | In release notes |
|---|---|---|
| `feat` | New behaviour | yes |
| `fix` | Bug fixes | yes |
| `refactor`, `perf`, `chore`, `build` | Internal changes | yes |
| `chore(deps)`, `fix(deps)` | Dependency updates (Renovate's semantic commit format) | yes |
| `docs`, `test`, `ci` | Docs, tests, CI | no |

- One PR per issue. The PR title follows the same format: it becomes the squash commit on `main`, and goreleaser builds the changelog from it. CI enforces this (`pr-title` job in `ci.yaml`; keep its type list in sync with this table).
- Branch commits are squashed away, so their messages matter less. Use the same format anyway.
- Reference the issue in the PR body: `Closes #n`.

## Agent skills

### Issue tracker

Issues and specs are GitHub issues on `TBeijen/secretd`, via `gh`. See `docs/agents/issue-tracker.md`.

### Triage labels

Default vocabulary: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `GLOSSARY.md` and `docs/adr/` at the repo root, created when needed. See `docs/agents/domain.md`.

## Never

- Print, log or include credential values in errors. Use profile names and expiry times only.
- Write credentials anywhere but the session folder.
- Run `aws sso login` or other interactive flows.
