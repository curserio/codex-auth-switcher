# Security Policy

## Reporting a Vulnerability

Please report security issues privately to the repository owner before opening a
public issue.

Do not attach or paste:

- `auth.json`
- `usage.json`
- `.codex` directories
- Codex session logs
- JWTs, OAuth tokens, access tokens, refresh tokens, or bearer tokens
- Screenshots or terminal output that include account identifiers or secrets

If a report needs examples, redact tokens and account-specific values first.

## Local Auth Data

`codex-switch` manages local Codex CLI auth profiles. Profile directories can
contain usable local auth material. Treat `~/.codex-auth-switcher`,
`~/.codex/auth.json`, and Codex backups as sensitive local data.
