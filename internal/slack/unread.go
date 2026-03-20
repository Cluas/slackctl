package slack

import (
	"strconv"
)

// UnreadChannel represents a channel with unread messages.
type UnreadChannel struct {
	ChannelID   string           `json:"channel_id"`
	ChannelName string           `json:"channel_name,omitempty"`
	UnreadCount int              `json:"unread_count"`
	Messages    []MessageSummary `json:"messages,omitempty"`
}

// FetchUnreadChannels returns channels that have unread messages.
// Uses client.counts API for accurate unread counts across all channel types.
func (c *Client) FetchUnreadChannels(limit int) ([]UnreadChannel, error) {
	resp, err := c.API("client.counts", map[string]string{
		"thread_count_by_last_read": "true",
		"org_wide_aware":            "true",
	})
	if err != nil {
		return nil, err
	}

	var unreads []UnreadChannel

	// Collect from channels, mpims, ims
	for _, key := range []string{"channels", "mpims", "ims"} {
		for _, ch := range getArray(resp, key) {
			rec := toRecord(ch)
			count := intVal(rec, "mention_count")
			if count == 0 {
				if hasUnreads, ok := rec["has_unreads"].(bool); ok && hasUnreads {
					count = 1
				}
			}
			if count > 0 {
				unreads = append(unreads, UnreadChannel{
					ChannelID:   stringVal(rec, "id"),
					UnreadCount: count,
				})
			}
		}
	}

	// Resolve channel names (batch, skip on error)
	for i := range unreads {
		name, _ := c.ResolveChannelName(unreads[i].ChannelID)
		unreads[i].ChannelName = name
	}

	if len(unreads) > limit {
		unreads = unreads[:limit]
	}
	return unreads, nil
}

// FetchUnreadMessages fetches the actual unread messages for a channel.
// Gets channel info to find the last_read timestamp, then fetches messages after it.
func (c *Client) FetchUnreadMessages(channelID string, maxMessages int) ([]MessageSummary, error) {
	resp, err := c.API("conversations.info", map[string]string{
		"channel": channelID,
	})
	if err != nil {
		return nil, err
	}
	ch := toRecord(resp["channel"])
	lastRead := stringVal(ch, "last_read")
	if lastRead == "" {
		lastRead = "0"
	}

	params := map[string]string{
		"channel": channelID,
		"oldest":  lastRead,
		"limit":   strconv.Itoa(min(maxMessages, 200)),
	}
	var messages []MessageSummary
	for {
		resp, err := c.API("conversations.history", params)
		if err != nil {
			return nil, err
		}
		msgs := getArray(resp, "messages")
		for _, m := range msgs {
			rec := toRecord(m)
			messages = append(messages, *parseMessage(rec, channelID))
		}
		cursor := getString(resp, "response_metadata", "next_cursor")
		if cursor == "" || len(messages) >= maxMessages {
			break
		}
		params["cursor"] = cursor
	}
	if len(messages) > maxMessages {
		messages = messages[:maxMessages]
	}
	return messages, nil
}
