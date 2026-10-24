# 02 — Chain into a dedicated LLM role, with SourceIdentity

## Goal

Agent sessions use a purpose-built, view-only role instead of the human's SSO permission set. CloudTrail attributes every call to the human.

Chain: `me → SSO permission set → LLM role`.

## Config

```toml
[aws.profiles.acme-prod-view]
source          = "acme-prod-ReadOnly"       # SSO profile used to assume the role
assume_role     = "arn:aws:iam::123456789012:role/llm-view-only"
source_identity = "auto"                    # = SSO session user; or a literal value
duration        = "15m"                     # role chaining caps this at 1h
```

secretd calls `sts:AssumeRole` itself, with the source profile's credentials, `--source-identity` and `--role-session-name llm-<user>`.

## Sample IAM (`examples/iam/`)

- `llm-view-only.permissions.json`: `ViewOnlyAccess` plus an explicit deny on data reads (`s3:GetObject*`, `secretsmanager:GetSecretValue`, `ssm:GetParameter*`, `kms:Decrypt`, `dynamodb:GetItem/Query/Scan/BatchGetItem`, `logs:GetLogEvents`?, `athena:GetQueryResults`).
- `llm-view-only.trust.json`:
  - Principal: account root, with condition `aws:PrincipalArn` like `.../aws-reserved/sso.amazonaws.com/*/AWSReservedSSO_<PermSet>_*`.
  - Actions: `sts:AssumeRole`, `sts:SetSourceIdentity`.
  - Requires that `sts:SourceIdentity` is present.
- EKS: access entry mapping the role to the `view` cluster policy (no Secrets).

## Open questions

- Can the trust policy bind `sts:SourceIdentity` to the caller's SSO session name, or only require that it is present?
- Which data-read actions should be denied, and which allowed (e.g. CloudWatch Logs on non-prod)?
- Is a dedicated Identity Center permission set an option instead? It would make this chaining unnecessary.

## Done when

- CloudTrail events from the agent show `sourceIdentity = <me>`.
- `aws s3 cp s3://<bucket>/<key> -` is denied; `aws s3api get-bucket-policy` works.
