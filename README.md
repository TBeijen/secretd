# secretd

Short-lived AWS credentials for sandboxed AI agents, scoped to profiles you opt into.

> Status: early. Step 1 (mint opted-in SSO profiles) is implemented. See `issues/`.

## Why

With AWS SSO, the SSO token cache can mint credentials for *every* role you have, including admin. Giving an agent `~/.aws` gives it all of that.

`secretd` runs outside the sandbox. It mints STS credentials for an allow-listed set of profiles only, and writes them to a session folder the sandbox can read. The agent switches between those profiles like you would (`--profile`, `AWS_PROFILE`), and never sees the SSO cache.

## How it works

```zsh
secretd -- fnox exec -- nono run --read @SECRETD_DIR@ -- claude
```

1. Picks a context by matching the current directory against `~/.config/secretd/contexts/*.toml`.
2. Mints credentials per profile via `aws configure export-credentials`.
3. Writes `config` + `credentials` to a fresh session folder (0700), and replaces `@SECRETD_DIR@` in the command with its path.
4. Starts the command with `AWS_CONFIG_FILE` / `AWS_SHARED_CREDENTIALS_FILE` pointing there.
5. Refreshes in the background. Removes the folder when the command exits.

## Config

```toml
# ~/.config/secretd/contexts/work.toml
match = ["~/projects/work"]

[aws.profiles.acme-nonprod-ro]
region = "eu-west-1"

[aws.profiles.acme-prod-ro]
source = "acme-prod-ReadOnly"   # name in ~/.aws/config, if different
```

Keep this config outside anything the sandbox can write to.

## Roadmap

1. Mint opted-in SSO profiles.
2. Chain into a dedicated view-only role with SourceIdentity.
3. Pin each session to the current egress IP.
4. CI, vulnerability scanning, releases.

## Security notes

- The agent **can** read the minted credentials. What limits them is the opted-in profiles, short lifetimes, and (later) view-only roles and IP pinning.
- Install `secretd` and `aws` somewhere the sandbox cannot write to.

## License

MIT
