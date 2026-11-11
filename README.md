# slackctl

A fast, single-binary Slack CLI for AI agents and humans. Written in Go.

## Features

- **Single binary** — no Node.js, no runtime dependencies; macOS, Linux & Windows (amd64/arm64)
- **Auto-auth** — extracts tokens from Slack Desktop, Chrome, Brave, Firefox (cross-platform: macOS Keychain, Linux Secret Service, Windows DPAPI)
- **Browser auth** — works with `xoxc` + `xoxd` cookie (no bot token required)
- **AI-friendly** — JSON output, designed for agent tool use
- **Full Slack API** — messages, channels, users, search, canvas, reactions

## Install

### Homebrew (macOS / Linux)

```bash
brew tap cluas/tap
brew install slackctl
```

### Scoop (Windows)

```powershell
scoop bucket add cluas https://github.com/cluas/scoop-bucket
scoop install slackctl
```

### Prebuilt binaries (macOS / Linux / Windows)

Download an archive for your OS/arch from the [releases page](https://github.com/cluas/slackctl/releases),
extract it, and put `slackctl` (or `slackctl.exe` on Windows) on your `$PATH`.

### Go install

```bash
go install github.com/cluas/slackctl/cmd/slackctl@latest
```

### From source

```bash
git clone https://github.com/cluas/slackctl.git
cd slackctl
make build            # binary for the current platform
make cross            # cross-compile for all platforms into dist/
```

Manual single-target cross-compile (no Make):

```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o slackctl.exe ./cmd/slackctl/
```

## Quick start

```bash
# Import credentials from Slack Desktop
slackctl auth import-desktop

# Or from browser
slackctl auth import-chrome
slackctl auth import-firefox
slackctl auth import-brave

# Or add manually
slackctl auth add --workspace-url https://myteam.slack.com --xoxc xoxc-... --xoxd xoxd-...

# Verify
slackctl auth test
```

## Usage

```
slackctl
├── auth
│   ├── whoami              # Show configured workspaces
│   ├── test                # Verify credentials (auth.test)
│   ├── add                 # Add token or xoxc/xoxd manually
│   ├── import-desktop      # Extract from Slack Desktop app
│   ├── import-chrome       # Extract from Chrome (macOS)
│   ├── import-brave        # Extract from Brave (macOS)
│   ├── import-firefox      # Extract from Firefox
│   ├── parse-curl          # Extract from cURL command (stdin)
│   ├── set-default         # Set default workspace
│   └── remove              # Remove a workspace
├── message
│   ├── get <target> [ts]   # Fetch a single message
│   ├── list <target>       # List channel history or thread
│   ├── send <target> <text># Post a message
│   ├── edit <target> <ts>  # Edit a message
│   ├── delete <target> [ts]# Delete a message
│   ├── react add|remove    # Manage reactions
│   └── saved               # List saved-for-later messages (Later)
├── canvas
│   └── get <url-or-id>     # Fetch canvas as Markdown
├── search
│   ├── messages <query>    # Search messages
│   └── files <query>       # Search files
├── user
│   ├── list                # List workspace users
│   └── get <id-or-handle>  # Get user info
├── channel
│   ├── list                # List conversations
│   ├── new <name>          # Create a channel
│   ├── invite <ch> <users> # Invite users
│   └── mark <ch> <ts>      # Mark as read
└── listen                  # Stream real-time events as JSON Lines
```

### listen: real-time event stream

`slackctl listen` opens a real-time websocket via `rtm.connect` (works with
browser `xoxc` + `d` cookie auth, including Enterprise Grid) and prints one
JSON object per line on stdout. Diagnostics go to stderr. It is read-only:
it never sends messages, marks anything read, or reacts.

How it works:

1. Calls `rtm.connect` to get a one-time `wss://` URL and your own identity.
2. Dials the websocket with the same TLS fingerprint, `d` cookie and
   User-Agent used for API calls, so browser-session credentials work.
3. Prints `_connected` on the first `hello`, then one line per event that
   passes the filters below, until you press Ctrl-C (exit 0).

```bash
# Every new message in conversations you are in
slackctl listen

# Pick a workspace when you have several
slackctl listen --workspace myteam

# Only DMs, group DMs, and messages that @-mention you
slackctl listen --mentions-or-dms | jq -r --unbuffered \
  'select(.type=="message") | "\(.channel) \(.user): \(.text)"'

# Several event types; edits/deletes and bot messages filtered as you like
slackctl listen --types message,reaction_added --all-subtypes --include-bots=false

# Everything Slack sends (empty --types), saved for later analysis
slackctl listen --types "" > events.jsonl

# Extract button / link URLs from bot messages (attachments + blocks)
slackctl listen | jq -r --unbuffered \
  '.attachments[]?.actions[]?.url // empty, .blocks[]?.elements[]?.url // empty'
```

| Flag | Default | Description |
|---|---|---|
| `--types` | `message` | Comma-separated event types to emit; empty string = all |
| `--mentions-or-dms` | `false` | Only DMs (`D…` / `channel_type: im`), group DMs (`is_mpim`, resolved via `conversations.info` and cached), or text that mentions you exactly (`<@your-id>` or `<@your-id\|label>`) |
| `--include-bots` | `true` | Include bot messages (`--include-bots=false` to drop). A message counts as a bot message if it has a `bot_id` or subtype `bot_message`, so app messages with an empty subtype are dropped too |
| `--all-subtypes` | `false` | Also emit `message_changed`, `message_deleted`, joins, etc. |

All filtering happens locally: Slack streams every event your account can
see over the websocket, and `slackctl` decides what to print. RTM has no
per-channel subscription, so filters reduce output, not network traffic; in a
large workspace with many joined channels, expect the full event volume to be
received even with `--mentions-or-dms`.

By default only message subtypes that carry new content are emitted: plain
messages, `bot_message`, `thread_broadcast`, `file_share`, `me_message`.
Non-message events (e.g. `reaction_added`) are emitted whenever their type
is in `--types`, as the **original Slack payload, unchanged** (the other
filters and the fields below apply to `message` events only).

`message` line fields: `type, subtype, channel, ts, thread_ts, user, bot_id,
text` (max 2000 chars), `from_self, is_mention, is_dm, event_ts, deleted_ts`, plus
`attachments` and `blocks` passed through unchanged when present (bot messages
often keep their button URLs there). For `message_changed`, content fields come
from the nested edited message; for `message_deleted`, they come from
`previous_message`, and `ts` / `deleted_ts` are the removed message's timestamp
(`event_ts` is when it was deleted). Your own messages are kept and flagged
`"from_self": true`.

```json
{"type":"message","channel":"D0123ABCD","ts":"1700000000.000100","user":"U0123ABCD","text":"hi <@U0SELF>","from_self":false,"is_mention":true,"is_dm":true}
```

Control lines:

- `{"type":"_connected","self":{...},"via":"rtm.connect",...}` on first `hello`
- `{"type":"_reconnected",...}` after any later reconnect (events sent while
  disconnected are not replayed; use `message list` to backfill)
- `{"type":"_auth_error","error":"..."}` when credentials are rejected
  (`invalid_auth` / `not_authed` / `token_revoked` / `token_expired` /
  `account_inactive`); the process then exits non-zero. Re-run
  `slackctl auth import-desktop` (or your usual auth command) and restart.

The connection is kept alive with `ping`/`pong` every 20s; a missing pong,
`goodbye`, or network error triggers reconnect with exponential backoff
(1s, 2s, 4s … capped at 60s), reusing `reconnect_url` when Slack provides one.
Group-DM lookups (`conversations.info`) run off the websocket loop, so a slow or
rate-limited lookup delays output but never the keepalive.

If writing to stdout fails (e.g. disk full), the process logs the error and
exits non-zero instead of silently dropping events.

Running it as a long-lived process: restart only on non-zero exit, and fix
credentials first when the last line was `_auth_error`, e.g.

```bash
until slackctl listen >> events.jsonl; do
  tail -n1 events.jsonl | grep -q '"_auth_error"' && break
  sleep 5
done
```

## Environment variables

| Variable | Description |
|---|---|
| `SLACK_TOKEN` | Slack token (xoxb-/xoxp-/xoxc-) |
| `SLACK_COOKIE_D` | Browser cookie `d` value (required for xoxc tokens) |
| `SLACK_WORKSPACE_URL` | Workspace URL (required for browser auth) |

Environment variables take precedence over stored credentials.

## Agent skill

This repo ships an agent skill at `skills/slackctl/` compatible with Claude Code, Codex, Gemini CLI, etc.

```bash
# Recommended (via skills.sh)
npx skills add cluas/slackctl -g
```

<details>
<summary>Manual installation</summary>

```bash
git clone https://github.com/cluas/slackctl.git
bash slackctl/scripts/install-skill.sh
```

</details>

This installs the skill to `~/.claude/skills/` and `~/.agents/skills/`. Restart your agent session to activate.

## Credentials storage

Credentials are stored in `~/.config/agent-slack/credentials.json` (compatible with [agent-slack](https://github.com/stablyai/agent-slack)).

## Acknowledgements

This project is a Go rewrite of [agent-slack](https://github.com/stablyai/agent-slack) by [stablyai](https://github.com/stablyai). Thanks to the original authors for the excellent design and architecture that made this port straightforward.

## License

[MIT](LICENSE)
