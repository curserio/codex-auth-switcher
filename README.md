# codex-switch

Local account switcher for OpenAI Codex CLI auth profiles.

It keeps Codex chats and local state shared, and switches Codex auth files.

This is an unofficial tool and is not affiliated with OpenAI.

## Requirements

- Go matching the version in `go.mod`
- OpenAI Codex CLI installed
- At least one Codex CLI login completed through the normal Codex flow

## Install

```bash
go install github.com/curserio/codex-auth-switcher/cmd/codex-switch@latest
```

For local development:

```bash
cd codex-auth-switcher
go install ./cmd/codex-switch
```

Or use the project target:

```bash
make install
```

Or run without installing:

```bash
go run ./cmd/codex-switch status
```

## First setup

Log in to the first account with the normal Codex flow, then save it:

```bash
codex-switch init
codex-switch add main
```

`add` also tries a live app-server usage read and then stores the refreshed
auth files again. If that warning fails, run `codex-switch capture` once after
Codex starts working for that account; `capture` also refreshes the saved auth
files for the active profile.

Do not use Codex or VS Code plugin logout before adding the next account. Logout
can revoke the OAuth token on the server, which makes the saved profile unusable.
Instead, prepare a clean local login state:

```bash
codex-switch prepare-login
```

Then log in to the second account with the normal Codex flow and save it:

```bash
codex-switch add backup
```

Switch accounts:

```bash
codex-switch use main
codex-switch use backup
```

`use` validates the target with a live app-server usage read after switching.
If the target token has been revoked, it restores the previous profile and
reports the failure.

Already running Codex or VS Code processes may keep the previous token in memory.
After switching, restart/resume Codex or reload the VS Code window if needed.

If you log out/log in manually with the normal Codex flow, `codex-switch`
detects the active `auth.json` by account identity instead of trusting the last
stored profile name. Adding the same account under a second profile name is
rejected to avoid accidentally making two profiles point at the same login.

## Usage

```bash
codex-switch status
codex-switch status --json
codex-switch capture
codex-switch doctor
codex-switch list
codex-switch current
codex-switch rename main personal
codex-switch delete backup
```

`capture` reads live usage from `codex app-server`. It does not use stale
session jsonl snapshots as a fallback.

`status --json` prints machine-readable profile and usage state without tokens
or raw auth data.

`doctor` checks local profile files, duplicate identities, active profile
matching, and live app-server usage for the active account. It exits non-zero
when a failure is found.

If `codex` is not on `PATH`, set `CODEX_SWITCH_CODEX_BIN` to the Codex CLI
binary path before running commands that read live usage.

## Development

```bash
make test
make check
make status-demo
```

## Storage

Profiles are stored in `~/.codex-auth-switcher` by default.

```text
accounts/<name>/auth.json
accounts/<name>/installation_id
accounts/<name>/meta.json
accounts/<name>/usage.json
current
switch.log
backups/
```

`installation_id` is saved with each profile when present; `prepare-login`
rotates it before the next login. `.credentials.json` is not managed by this
tool; MCP/app credentials stay shared across profiles.

Auth files are written with `0600` permissions. The tool never prints tokens.

## Safety and Privacy

`codex-switch` modifies local Codex CLI auth state:

- `~/.codex/auth.json`
- `~/.codex/installation_id`
- `~/.codex-auth-switcher`

The profile store can contain usable local auth material. Do not publish,
share, or attach profile directories, `auth.json`, Codex session logs, or
terminal output containing tokens. Use `codex-switch doctor` to inspect local
state without printing raw auth data.

## Maintainer Notes

Recommended GitHub settings:

- Repository topics: `codex`, `openai`, `cli`, `go`, `auth`, `developer-tools`
- Enable secret scanning
- Protect the default branch
- Create the first release tag as `v0.1.0`
