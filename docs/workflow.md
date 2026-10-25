# Agentic workflow

> **Draft.** A plan, not a description of how things work today. Expect it to change after trying it on a few issues.

## Goals

- Every change starts as a spec, gets reviewed before and after implementation, and lands via a PR.
- Security problems (like issue 07: session folders readable by every sandboxed agent) get caught by review, not by chance.
- No lock-in to one harness or model vendor.

## Flow

1. **Spec.** Write the issue as a GitHub issue (`gh issue create --body-file`). Specs may be drafted with skills (e.g. mattpocock's). Longer design notes go in `docs/` and are linked from the issue.
2. **Issue review.** A fresh-context agent reviews the issue against `AGENTS.md` and the [security checklist](#security-checklist). Findings go as issue comments. Fix the spec before building.
3. **Build.** One branch and one PR per issue, `Closes #n`. `mise run ci` passes locally.
4. **PR review.** Fresh-context reviews, each posted as PR comments:
   - standards and spec (does the diff do what the issue says?), e.g. mattpocock's `code-review` skill;
   - security, against the checklist.
   - Later (phase 2): one of them by a different model than the one that wrote the code.
5. **Fix and merge.** The implementing agent addresses the comments. A human merges.

## Security checklist

Applies to issue reviews and PR reviews.

- For every file or folder secretd creates: who can read or write it? Consider other users **and** the sandboxed agent under nono's default policy (e.g. `$TMPDIR` and `/tmp` are writable from the sandbox, `~/.local/state` is not).
- Can one session reach another session's data?
- Can the agent change what secretd reads (config, binaries, `aws` CLI, session files)?
- Do credential values reach logs, errors, `meta.json` or stdout?
- What happens on failure: partial writes, crashes, expired SSO, a killed child?
- Does a "Done when" item test the boundary from inside the sandbox, not just the happy path?

## GitHub setup

- **Fine-grained PAT, this repo only:**
  - read and write: Contents, Issues, Pull requests;
  - read: Metadata; optionally Actions, so the agent can see CI runs (`gh run view`);
  - expiry e.g. 90 days;
  - not granted: Administration, or Workflows. Without Workflows, agents can't change `.github/workflows/`.
- **Token on the dev machine:** stored in the Keychain via fnox in `fnox.local.toml` (gitignored), exposed as `GH_TOKEN`. Use `GH_TOKEN`, not `GITHUB_TOKEN`: the nono profile hides and replaces `GH_TOKEN` only, so a `GITHUB_TOKEN` would reach the sandbox unmasked.

  ```toml
  # fnox.local.toml
  [providers.keychain]
  type = "keychain"
  service = "<service>"

  [secrets]
  GH_TOKEN = { provider = "keychain", value = "<keychain-name>" }
  ```

  Store it outside the sandbox, from the clipboard:

  ```sh
  pbpaste | tr -d '\r\n' | fnox -c fnox.local.toml set \
    -p keychain -k <keychain-name> \
    GH_TOKEN
  pbcopy < /dev/null                   # clear the clipboard (and any clipboard manager history)
  fnox get GH_TOKEN | cut -c1-11       # → github_pat_
  ```

  Without `-c` and `-k`, `fnox set` writes to `fnox.toml` and uses the variable name as Keychain name. `fnox.local.toml` then overrides it with a name that has no value.
- **Ruleset on `main`:** require a PR, squash merges only, no force pushes. Require one status check: `all-checks`, a gate job in `ci.yaml` (`re-actors/alls-green`). New CI jobs go into its `needs`, so the ruleset never changes.
- **Merging:** the agent uses the owner's token, so it can technically merge. nono's `endpoint_rules` can't distinguish GraphQL mutations (`gh pr merge` uses GraphQL). Accept this, and merge by hand as a convention.
- **Later:** a GitHub App as a separate bot identity for posted reviews.
- **Commits** keep the `Co-Authored-By` trailer for AI-assisted work.

## Harnesses and models

Currently Claude Code. The preference is open, multi-vendor harnesses (Copilot CLI, OpenCode, pi). Keep the workflow portable:

- **Instructions** live in `AGENTS.md`. Claude Code reads `CLAUDE.md`, so add a `CLAUDE.md` that only contains `@AGENTS.md`.
- **Skills:** check per harness whether it supports `SKILL.md`-style skills, and install the same ones in each (mattpocock's, any project skills).
- **Sandbox:** each harness needs its own nono profile: network allowlist, credential routes, config dirs. Do this before switching.
- **Running other models without switching harnesses:**
  - **Not from inside the main agent.** Anything it starts inherits its sandbox, so its nono profile would need every reviewer's hosts, credentials and config dirs. That widens the main agent's sandbox. Copilot CLI, for example, keeps its OAuth token in the Keychain, which a no-keychain profile blocks.
  - **Host-side, one sandbox per reviewer:** e.g. `mise run review <pr>` runs `nono run --profile <reviewer> --read . -- <cli> -p ...`. Each reviewer profile is small: read the repo, post to the PR. The main agent only reads the results (`gh pr view --comments`). A multi-agent manager like herdr could take this over later.
  - **Through GitHub:** reviewers start on PR events (Copilot code review, vendor GitHub integrations, or a custom Action) and post comments. Nothing needs local credentials. For Copilot, this is the simplest option.
- Start with GitHub-native reviewers where available, and host-side scripts for the rest.

### Phases

1. **Now:** one harness (Claude Code). Reviews by fresh-context subagents in the same harness and model, steered by the checklist.
2. **Later:** switch harness, or add herdr. Add reviews by a different model via host-side or GitHub-native reviewers.

## Costs

- Posting comments and reviews via the API is free, and so are Actions on public repos. The costs are model subscriptions and API keys.
- When using an org-paid licence (e.g. Copilot Business), check the org's policy on use for personal projects.

## Open questions

- Keep `issues/*.md` as specs in the repo, or move them fully to GitHub issues?
- Which non-interactive CLI flags and GitHub integrations work today? Verify them; this draft is partly from memory.
- Integration tests that run secretd inside nono: local only, or a macOS CI runner with nono installed?

## First trial

Issue 07 (session storage): small, security-relevant, already specced. Run it through the whole flow.
