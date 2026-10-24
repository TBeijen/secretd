# 06 — Agent awareness

## Goal

An agent inside the sandbox knows which AWS profiles it has, and what to do when they fail. It doesn't go hunting in `~/.aws` or try `aws sso login`.

## Session README

secretd writes `$SECRETD_DIR/README.md` next to `aws/`, rewritten on every refresh:

- Opted-in profiles with region, plus their expiry times.
- Credentials refresh automatically. No action needed while they're valid.
- Don't run `aws sso login`, don't read `~/.aws`. Neither works from the sandbox.
- Expired or failing credentials: tell the user to run `aws sso login` on the host. secretd picks it up within a minute.
- Use `--profile <name>` or `AWS_PROFILE=<name> <cmd>`. Each tool call is a fresh shell, so exported vars don't carry over.

Agent-agnostic, and nothing to install. The sandbox can only read the session folder, so the agent can't edit its own instructions. It never contains credential values.

## Getting it into the agent

Ship snippets in the main README, not in code:

- Claude Code: a `SessionStart` hook that prints `$SECRETD_DIR/README.md` when `SECRETD_DIR` is set, and prints nothing otherwise.
- Other agents: one line in `AGENTS.md` / `CLAUDE.md`: "If `SECRETD_DIR` is set, read `$SECRETD_DIR/README.md` before using AWS."

## Later, maybe

- A Claude plugin in the repo (skill + hooks), installable via `/plugin marketplace add TBeijen/secretd`. It could include a hook that recognises `ExpiredToken` or "profile could not be found" and points to the fix, similar to nono's plugin. Only worth it once the README approach shows what agents actually get wrong.
- A kubeconfig section, once EKS kubeconfigs are exposed.

## Out of scope

Personal workflow, such as workspace switchers or which profile maps to which cluster. That belongs in the user's own `~/.claude/CLAUDE.md` or skills.

## Done when

- A Claude session started via `secretd exec` lists the available profiles without exploring.
- With expired credentials, it tells the user to log in on the host, instead of attempting a fix itself.
