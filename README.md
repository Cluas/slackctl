# slackctl

A fast, single-binary Slack CLI for AI agents and humans. Written in Go.

## Features

- **Single binary** — no Node.js, no runtime dependencies
- **Auto-auth** — extracts tokens from Slack Desktop, Chrome, Brave, Firefox
- **Browser auth** — works with `xoxc` + `xoxd` cookie (no bot token required)
- **AI-friendly** — JSON output, designed for agent tool use
- **Full Slack API** — messages, channels, users, search, canvas, reactions

## Install

### Homebrew (macOS / Linux)

```bash
brew tap cluas/tap
brew install slackctl
```

### Go install

```bash
go install github.com/cluas/slackctl/cmd/slackctl@latest
```

### From source

```bash
git clone https://github.com/cluas/slackctl.git
cd slackctl
make build
# binary at ./slackctl
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
│   └── react add|remove    # Manage reactions
├── canvas
│   └── get <url-or-id>     # Fetch canvas as Markdown
├── search
│   ├── messages <query>    # Search messages
│   └── files <query>       # Search files
├── user
│   ├── list                # List workspace users
│   └── get <id-or-handle>  # Get user info
└── channel
    ├── list                # List conversations
    ├── new <name>          # Create a channel
    ├── invite <ch> <users> # Invite users
    └── mark <ch> <ts>      # Mark as read
```

## Environment variables

| Variable | Description |
|---|---|
| `SLACK_TOKEN` | Slack token (xoxb-/xoxp-/xoxc-) |
| `SLACK_COOKIE_D` | Browser cookie `d` value (required for xoxc tokens) |
| `SLACK_WORKSPACE_URL` | Workspace URL (required for browser auth) |

Environment variables take precedence over stored credentials.

## Credentials storage

Credentials are stored in `~/.config/agent-slack/credentials.json` (compatible with [agent-slack](https://github.com/stablyai/agent-slack)).

## Acknowledgements

This project is a Go rewrite of [agent-slack](https://github.com/stablyai/agent-slack) by [stablyai](https://github.com/stablyai). Thanks to the original authors for the excellent design and architecture that made this port straightforward.

## License

[MIT](LICENSE)
