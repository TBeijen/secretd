# 05 — Session logs and `session` / `logs` commands

## Goal

With several sessions running, see what each one is doing, and look back afterwards, without relying on stderr scrolling past inside a TUI.

## Layout

State lives in `$XDG_STATE_HOME/secretd` (default `~/.local/state/secretd`), kept after exit:

```
sessions/<id>/
  meta.json     # id, pid, cwd, context, profiles, session dir, started, ended
  secretd.log   # log/slog text format
```

- `<id>` is the random suffix of the credentials folder. The child gets it as `SECRETD_SESSION`.
- Keep this folder out of the sandbox's writable paths, so an agent can't edit its own trail. README says so, same as for the config.
- Logs never contain credential values (same rule as now).

## Output

- Everything goes to the log file.
- stderr: startup lines, and warnings/errors after the child starts. Routine refresh lines go to the file only.

## Commands

```sh
secretd session list            # id, state (running/ended), started, context, cwd
secretd logs                    # most recent session
secretd logs --session <id>     # id prefix is enough if unambiguous
secretd logs --follow           # tail -f
secretd logs --all              # all sessions interleaved, prefixed with id
```

- Running vs ended: the pid is alive and `ended` is unset. A crashed session (pid dead, no `ended`) shows as `ended (crashed)`.

## Retention

- At startup, remove sessions that ended more than 14 days ago. Configurable via `log_retention_days` in `config.toml`.
- No daemon, no background pruning.

## Maybe later

- `secretd session refresh <id>` (or `SIGUSR1`) to refresh immediately after `aws sso login`, instead of waiting for the 60s retry.

## Out of scope

- Notifications (macOS, ntfy). stderr is enough for now.
- JSON log output.

## Done when

- Two parallel sessions show up in `session list`, each with its own log.
- `logs --follow` shows a refresh as it happens.
- A refresh failure appears on stderr. A successful refresh doesn't.
- Sessions older than the retention period are removed on the next run.
