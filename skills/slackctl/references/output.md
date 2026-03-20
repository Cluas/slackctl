# Output shapes (reference)

## Output format

All commands print JSON to stdout.

- `auth whoami` redacts secrets in its output.
- Empty/null values may be omitted via `omitempty`.

## Message shapes

- `message get` returns a single message object:

```json
{
  "channel_id": "C123",
  "ts": "1700000000.000000",
  "user": "U123",
  "text": "hello",
  "reply_count": 3,
  "reactions": [{"name": "eyes", "count": 2}],
  "files": [{"id": "F123", "name": "report.pdf"}]
}
```

- `message list` returns an array of message objects:

```json
[
  {"channel_id": "C123", "ts": "...", "user": "U123", "text": "..."},
  {"channel_id": "C123", "ts": "...", "user": "U456", "text": "..."}
]
```

- `message unread` returns an array of channels with unread info:

```json
[
  {
    "channel_id": "C123",
    "channel_name": "general",
    "unread_count": 3,
    "messages": [
      {"channel_id": "C123", "ts": "...", "user": "U123", "text": "..."}
    ]
  }
]
```

The `messages` field is only present when `--fetch` is used.

## Search shapes

- `search messages` returns:

```json
{
  "messages": [{"channel_id": "C123", "ts": "...", "text": "..."}],
  "total": 42
}
```

- `search files` returns:

```json
{
  "files": [{"id": "F123", "name": "report.pdf", "filetype": "pdf"}],
  "total": 5
}
```

## Channel shapes

- `channel list` returns an array:

```json
[
  {"id": "C123", "name": "general", "num_members": 50},
  {"id": "C456", "name": "random", "num_members": 30}
]
```

## User shapes

- `user get` returns:

```json
{
  "id": "U123",
  "name": "alice",
  "real_name": "Alice Smith",
  "display_name": "alice",
  "email": "alice@example.com",
  "title": "Engineer",
  "tz": "America/New_York"
}
```

- `user list` returns an array of user objects.

## Canvas shapes

- `canvas get` returns:

```json
{
  "canvas": {
    "id": "F123",
    "title": "My Canvas",
    "markdown": "# Heading\n\nContent here..."
  }
}
```

## Auth shapes

- `auth test` returns the raw Slack `auth.test` response:

```json
{
  "ok": true,
  "url": "https://myteam.slack.com/",
  "team": "My Team",
  "user": "alice",
  "user_id": "U123",
  "team_id": "T123"
}
```

- `auth whoami` returns:

```json
{
  "default": "https://myteam.slack.com",
  "workspaces": [
    {
      "workspace_url": "https://myteam.slack.com",
      "workspace_name": "My Team",
      "auth_type": "browser",
      "token": "xoxc-9…abcd"
    }
  ]
}
```
