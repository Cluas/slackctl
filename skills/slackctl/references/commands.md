# `slackctl` command map (reference)

Run `slackctl --help` (or `slackctl <command> --help`) for the full option list.

## Auth

- `slackctl auth whoami` — show configured workspaces + token sources (secrets redacted)
- `slackctl auth test [--workspace <url-or-unique-substring>]` — verify credentials (`auth.test`)
- `slackctl auth import-desktop` — import browser-style creds from Slack Desktop (macOS)
- `slackctl auth import-chrome` — import creds from Chrome (macOS)
- `slackctl auth import-brave` — import creds from Brave (macOS)
- `slackctl auth import-firefox` — import creds from Firefox profile storage (macOS/Linux)
- `slackctl auth parse-curl` — read a copied Slack cURL command from stdin and save creds
- `slackctl auth add --workspace-url <url> [--token <xoxb/xoxp> | --xoxc <xoxc> --xoxd <xoxd>]`
- `slackctl auth set-default <workspace-url>`
- `slackctl auth remove <workspace-url>`

## Messages / threads

- `slackctl message get <url-or-channel> [timestamp]`
  - Fetch a single message by URL or channel + ts.

- `slackctl message list <url-or-channel> [timestamp]`
  - Lists recent channel messages (channel history), or fetches thread replies.
  - Options:
    - `--thread-ts <seconds>.<micros>` (switches to thread mode; fetches replies)
    - `--limit <n>` (default `20`)

- `slackctl message unread`
  - Lists channels with unread messages using `client.counts` API.
  - Options:
    - `--limit <n>` (default `50`)
    - `--fetch` (also retrieve unread message content)
    - `--max-per-channel <n>` (default `10`, with `--fetch`)

- `slackctl message send <channel-or-url> <text>`
  - Posts a message. If target is a user ID (`U...`), opens a DM automatically.
  - Options:
    - `--thread-ts <seconds>.<micros>` (reply in thread)

- `slackctl message edit <url-or-channel> <timestamp> <text>`
  - Edit a message's text.

- `slackctl message delete <url-or-channel> [timestamp]`
  - Delete a message.

- `slackctl message react add <url-or-channel> <timestamp> <emoji>`
- `slackctl message react remove <url-or-channel> <timestamp> <emoji>`

## Channels

- `slackctl channel list [--limit <n>] [--types <types>]`
  - Default types: `public_channel,private_channel`
- `slackctl channel new <name> [--private]`
- `slackctl channel invite <channel> <user1,user2,...>`
- `slackctl channel mark <channel> <timestamp>` — mark as read

## Search

- `slackctl search messages <query> [--limit <n>]`
- `slackctl search files <query> [--limit <n>]`

## Canvas

- `slackctl canvas get <canvas-url-or-id> [--max-chars <n>]`
  - Fetches a Slack canvas and converts HTML to Markdown.
  - Default `--max-chars 20000`, use `-1` for unlimited.

## Users

- `slackctl user list [--limit <n>] [--include-bots]`
- `slackctl user get <U...|@handle|handle>`

## Global flags

- `--workspace <url-or-unique-substring>` — select workspace (needed when multiple workspaces configured and using channel names)
