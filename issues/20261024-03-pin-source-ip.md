# 03 — Pin sessions to the current egress IP

## Goal

Exfiltrated credentials are useless from anywhere but the machine's current egress IP. No static allowlists, nothing to purge, no dependency on a corporate proxy.

Requires 02 (session policies only apply to `AssumeRole`).

## Config

```toml
[aws.profiles.acme-prod-view]
# ...
pin_source_ip = true
extra_cidrs   = []        # optional static additions
```

## Behaviour

- Before each mint, get the egress IP via `https://checkip.amazonaws.com`. It takes the same path as AWS API traffic, including any corporate-proxy bypass (e.g. Zscaler).
- Pass a session policy: `Allow *` plus `Deny *` when `NotIpAddress aws:SourceIp [<ip>/32, extra_cidrs...]` and `aws:ViaAWSService = false`.
  - A deny-only session policy allows nothing, so the `Allow` is required.
- Re-check the IP every ~60s. On change, mint again immediately (network switch, VPN or corporate proxy toggling).
- IPv6: pin the `/64`. Check which address family the CLI endpoints actually use (default IPv4 unless dual-stack is enabled).

## Out of scope

- EKS API calls. The cluster validates the token itself, so `aws:SourceIp` doesn't apply. Use `publicAccessCidrs` / a private endpoint instead.
- Enforcing that every caller of the role passes such a policy. AWS has no condition key for "a session policy was passed". This is a self-imposed control.

## Done when

- Keys copied off the machine (e.g. used via a phone hotspot) get `AccessDenied`.
- Switching networks mid-session recovers within ~1 min.
