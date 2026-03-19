# Targets: URL vs channel (reference)

`slackctl` accepts either a **Slack message URL** (preferred) or a **channel reference**.

## Preferred: Slack message URL

Use the message permalink whenever you have it:

```text
https://<workspace>.slack.com/archives/<channel_id>/p<digits>[?thread_ts=...]
```

Examples:

- `slackctl message get "<url>"`
- `slackctl message list "<url>"`
- `slackctl message send "<url>" "reply text"`
- `slackctl message edit "<url>" <ts> "updated text"`
- `slackctl message delete "<url>"`
- `slackctl message react add "<url>" <ts> "eyes"`

## Channel targets (when you don't have a URL)

Channel references can be:

- channel name: `general` (bare name, without `#` prefix)
- channel id: `C...` (or `G...`/`D...`)
- user id: `U...` (opens DM automatically for `message send`)

### `message get` by channel + timestamp

```bash
slackctl message get "general" "1770165109.628379"
```

### `message list` by channel (history)

```bash
slackctl message list "general" --limit 20
slackctl message list "general" --thread-ts "1770165109.000001"
```

### Send to channel or DM

```bash
slackctl message send "general" "hello everyone"
slackctl message send U01AAAA "hello via DM"
slackctl message send "general" "threaded reply" --thread-ts "1770165109.000001"
```

### Edit/delete by channel + timestamp

```bash
slackctl message edit "general" "1770165109.628379" "updated text"
slackctl message delete "general" "1770165109.628379"
```

### Reactions by channel + timestamp

```bash
slackctl message react add "general" "1770165109.628379" "eyes"
slackctl message react remove "general" "1770165109.628379" "eyes"
```

### Mark as read

```bash
slackctl channel mark "general" "1770165109.628379"
```

## Multi-workspace ambiguity (channel names only)

If you have multiple workspaces configured and your target is a channel **name**, disambiguate with:

```bash
slackctl message list "general" --workspace "myteam" --limit 10
```

Channel IDs (`C...`/`G...`/`D...`) do not require `--workspace`.
