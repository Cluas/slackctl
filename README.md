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

```bash
# Every new message in conversations you are in
slackctl listen

# Only DMs, group DMs, and messages that @-mention you
slackctl listen --mentions-or-dms | while read -r line; do
  echo "$line" | jq -r 'select(.type=="message") | "\(.channel) \(.user): \(.text)"'
done
```

| Flag | Default | Description |
|---|---|---|
| `--types` | `message` | Comma-separated event types to emit; empty string = all |
| `--mentions-or-dms` | `false` | Only DMs (`D…`), group DMs (`G…` / `is_mpim`), or text containing `<@your-id>` |
| `--include-bots` | `true` | Include bot messages (`--include-bots=false` to drop) |
| `--all-subtypes` | `false` | Also emit `message_changed`, `message_deleted`, joins, etc. |

Message line fields: `type, subtype, channel, ts, thread_ts, user, bot_id,
text` (max 2000 chars), `from_self, is_mention, is_dm, event_ts`. Your own
messages are kept and flagged `"from_self": true`.

Control lines:

- `{"type":"_connected","self":{...},"via":"rtm.connect",...}` on first `hello`
- `{"type":"_reconnected",...}` after any later reconnect
- `{"type":"_auth_error","error":"..."}` when credentials are rejected
  (`invalid_auth` / `not_authed`); the process then exits non-zero

The connection is kept alive with `ping`/`pong` every 20s; a missing pong,
`goodbye`, or network error triggers reconnect with exponential backoff
(1s, 2s, 4s … capped at 60s), reusing `reconnect_url` when Slack provides one.

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
