# 07 — Move session storage out of `$TMPDIR`

## Problem

Session folders live in `$TMPDIR/secretd/<pid>-<rand>/`. nono's built-in `system_write_macos` group grants read+write on `$TMPDIR` and `/private/var/folders`, and `system_write_linux` grants `/tmp` and `$TMPDIR`. So every sandboxed agent:

- can read **all** running sessions' credentials, e.g. a personal-context session reading a work context's;
- can delete, rename or plant files in any session folder;
- gains nothing from `--read @SECRETD_DIR@`, since the folder is already readable.

The 0700/0400 modes only keep out other users. The agent runs as the same user.

## Approach

Follow nono: state lives under `$XDG_STATE_HOME/nono` (default `~/.local/state/nono`, on every platform, not `~/Library/Application Support`). nono's default policy doesn't grant `~/.local/state`, and it gives the sandbox individual files (e.g. the intercept CA) via explicit grants.

```
$XDG_STATE_HOME/secretd/          # default ~/.local/state/secretd, 0700, owner-checked
  sessions/<id>/                  # 0700
    meta.json, secretd.log        # issue 05, never granted to the sandbox
    share/                        # SECRETD_DIR, the only path to grant
      aws/config, aws/credentials # 0400
      README.md                   # issue 06
```

- `SECRETD_DIR` and `@SECRETD_DIR@` point to `share/`, not the session root.
- Only `share/`'s layout is a public interface (users grant it, agents read its README, AWS tools read `aws/`). The rest of the session folder is internal and read only through `secretd` commands, so it can change freely.
- Invocation stays the same: `nono run --read @SECRETD_DIR@ ...`. The flag exists (`--read DIR`, recursive read-only). It now actually grants something.
- Base folder: `$XDG_STATE_HOME/secretd`, default `~/.local/state/secretd`. A relative `XDG_STATE_HOME` is ignored, as nono does. Override via `SECRETD_STATE_DIR` for tests.
- Keep the existing checks: base owned by the user, 0700, not a symlink. Sweep dead sessions at startup.
- Exclude the folder from Time Machine on first creation (`tmutil addexclusion`, no admin needed). Ignore failures, and skip on Linux.

## Trade-offs

- Survives reboots, unlike `$TMPDIR`. A crashed session leaves expired credentials behind until the next sweep. The credentials are short-lived. nono's OAuth-capture `providers.json` with real tokens is in the same place.
- Not in memory. On Linux, `$XDG_RUNTIME_DIR` (tmpfs, removed at logout) would be better on paper. Not now: nono doesn't use it either, and one location is easier to reason about.

## Docs

- README: "Install" / security notes say the sandbox must not be able to read `$XDG_STATE_HOME/secretd`, only `@SECRETD_DIR@`. Point out that a profile granting `~/.local/state` breaks the isolation.
- Issue 05 uses this layout instead of its own.

## Done when

- Inside `nono run --read @SECRETD_DIR@`, the own session's `share/` is readable. `~/.local/state/secretd`, other sessions and the own `meta.json` / log are not.
- Two parallel sessions with different contexts can't see each other's credentials.
- `$TMPDIR/secretd` is no longer created.
