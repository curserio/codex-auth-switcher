# codex-switch

Local account switcher for OpenAI Codex CLI auth profiles.

It keeps Codex chats and local state shared, and switches only `auth.json`.

## Install

```bash
cd codex-auth-switcher
go install ./cmd/codex-switch
```

Or run without installing:

```bash
go run . status
```

## First setup

Log in to the first account with the normal Codex flow, then save it:

```bash
codex-switch init
codex-switch add main
```

Log out and log in to the second account with the normal Codex flow, then save it:

```bash
codex-switch add backup
```

Switch accounts:

```bash
codex-switch use main
codex-switch use backup
```

Already running Codex or VS Code processes may keep the previous token in memory.
After switching, restart/resume Codex or reload the VS Code window if needed.

If you log out/log in manually with the normal Codex flow, `codex-switch`
detects the active `auth.json` by account identity instead of trusting the last
stored profile name. Adding the same account under a second profile name is
rejected to avoid accidentally making two profiles point at the same login.

## Usage

```bash
codex-switch status
codex-switch capture
codex-switch list
codex-switch current
codex-switch rename main personal
codex-switch delete backup
```

`capture` tries `codex app-server` live usage first and falls back to the newest
rate-limit snapshot in `~/.codex/sessions/**/*.jsonl` or
`~/.codex/archived_sessions/*.jsonl`.

## Storage

Profiles are stored in `~/.codex-auth-switcher` by default.

```text
accounts/<name>/auth.json
accounts/<name>/meta.json
accounts/<name>/usage.json
current
switch.log
backups/
```

Auth files are written with `0600` permissions. The tool never prints tokens.
