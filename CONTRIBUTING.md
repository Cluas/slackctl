# Contributing to slackctl

## Development

### Prerequisites

- Go 1.21+
- CGO enabled (required for SQLite)
- macOS, Linux, or Windows

### Build

```bash
make build
```

### Test

```bash
make test
```

### Project structure

```
cmd/slackctl/       # CLI entry point
internal/
├── slack/          # Slack API client, messages, channels, users, search, canvas
├── auth/           # Credential extraction (Desktop, Chrome, Brave, Firefox, SQLite, crypto)
├── cli/            # Cobra commands
└── lib/            # Shared utilities
```

## Guidelines

- Keep functions short — one function, one job
- No unnecessary abstractions — three similar lines > premature helper
- Test with a real Slack workspace before submitting
- JSON output must be stable — don't break existing field names

## Pull requests

1. Fork and create a feature branch
2. Make your changes
3. Run `make test`
4. Submit a PR with a clear description

## Reporting issues

Open an issue at https://github.com/cluas/slackctl/issues
