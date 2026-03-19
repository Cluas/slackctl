package slack

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// MessageSummary is the core message data structure.
type MessageSummary struct {
	ChannelID   string         `json:"channel_id,omitempty"`
	ChannelName string         `json:"channel_name,omitempty"`
	ThreadTS    string         `json:"thread_ts,omitempty"`
	TS          string         `json:"ts"`
	User        string         `json:"user,omitempty"`
	UserName    string         `json:"user_name,omitempty"`
	Text        string         `json:"text,omitempty"`
	Reactions   []Reaction     `json:"reactions,omitempty"`
	Files       []FileSummary  `json:"files,omitempty"`
	ReplyCount  int            `json:"reply_count,omitempty"`
	Permalink   string         `json:"permalink,omitempty"`
}

type Reaction struct {
	Name  string   `json:"name"`
	Count int      `json:"count"`
	Users []string `json:"users,omitempty"`
}

type FileSummary struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	Filetype string `json:"filetype,omitempty"`
	Mimetype string `json:"mimetype,omitempty"`
	Size     int    `json:"size,omitempty"`
	URL      string `json:"url_private,omitempty"`
}

// FetchMessage fetches a single message by channel + timestamp.
func (c *Client) FetchMessage(channelID, ts string) (*MessageSummary, error) {
	// Try conversations.history with latest=ts, limit=1, inclusive=true
	resp, err := c.API("conversations.history", map[string]string{
		"channel":   channelID,
		"latest":    ts,
		"limit":     "1",
		"inclusive":  "true",
	})
	if err != nil {
		return nil, err
	}
	messages := getArray(resp, "messages")
	if len(messages) == 0 {
		return nil, fmt.Errorf("message not found: %s in %s", ts, channelID)
	}
	msg := toRecord(messages[0])
	return parseMessage(msg, channelID), nil
}

// FetchThread fetches all replies in a thread.
func (c *Client) FetchThread(channelID, threadTS string, limit int) ([]MessageSummary, error) {
	params := map[string]string{
		"channel": channelID,
		"ts":      threadTS,
		"limit":   strconv.Itoa(limit),
	}
	var all []MessageSummary
	for {
		resp, err := c.API("conversations.replies", params)
		if err != nil {
			return nil, err
		}
		messages := getArray(resp, "messages")
		for _, m := range messages {
			rec := toRecord(m)
			all = append(all, *parseMessage(rec, channelID))
		}
		cursor := getString(resp, "response_metadata", "next_cursor")
		if cursor == "" || len(all) >= limit {
			break
		}
		params["cursor"] = cursor
	}
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// FetchChannelHistory fetches recent messages from a channel.
func (c *Client) FetchChannelHistory(channelID string, limit int) ([]MessageSummary, error) {
	params := map[string]string{
		"channel": channelID,
		"limit":   strconv.Itoa(min(limit, 200)),
	}
	var all []MessageSummary
	for {
		resp, err := c.API("conversations.history", params)
		if err != nil {
			return nil, err
		}
		messages := getArray(resp, "messages")
		for _, m := range messages {
			rec := toRecord(m)
			all = append(all, *parseMessage(rec, channelID))
		}
		cursor := getString(resp, "response_metadata", "next_cursor")
		if cursor == "" || len(all) >= limit {
			break
		}
		params["cursor"] = cursor
	}
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

func parseMessage(msg map[string]any, channelID string) *MessageSummary {
	summary := &MessageSummary{
		ChannelID: channelID,
		TS:        stringVal(msg, "ts"),
		User:      stringVal(msg, "user"),
		Text:      stringVal(msg, "text"),
		ThreadTS:  stringVal(msg, "thread_ts"),
	}

	if rc, ok := msg["reply_count"].(float64); ok {
		summary.ReplyCount = int(rc)
	}
	if perma, ok := msg["permalink"].(string); ok {
		summary.Permalink = perma
	}

	// Parse reactions
	if reactions, ok := msg["reactions"].([]any); ok {
		for _, r := range reactions {
			rm := toRecord(r)
			reaction := Reaction{
				Name:  stringVal(rm, "name"),
				Count: intVal(rm, "count"),
			}
			if users, ok := rm["users"].([]any); ok {
				for _, u := range users {
					if s, ok := u.(string); ok {
						reaction.Users = append(reaction.Users, s)
					}
				}
			}
			summary.Reactions = append(summary.Reactions, reaction)
		}
	}

	// Parse files
	if files, ok := msg["files"].([]any); ok {
		for _, f := range files {
			fm := toRecord(f)
			summary.Files = append(summary.Files, FileSummary{
				ID:       stringVal(fm, "id"),
				Name:     stringVal(fm, "name"),
				Filetype: stringVal(fm, "filetype"),
				Mimetype: stringVal(fm, "mimetype"),
				Size:     intVal(fm, "size"),
				URL:      stringVal(fm, "url_private"),
			})
		}
	}

	return summary
}

// SendMessage posts a message to a channel or thread.
func (c *Client) SendMessage(channelID, text, threadTS string) (map[string]any, error) {
	params := map[string]string{
		"channel": channelID,
		"text":    text,
	}
	if threadTS != "" {
		params["thread_ts"] = threadTS
	}
	return c.API("chat.postMessage", params)
}

// EditMessage updates a message's text.
func (c *Client) EditMessage(channelID, ts, text string) (map[string]any, error) {
	return c.API("chat.update", map[string]string{
		"channel": channelID,
		"ts":      ts,
		"text":    text,
	})
}

// DeleteMessage removes a message.
func (c *Client) DeleteMessage(channelID, ts string) (map[string]any, error) {
	return c.API("chat.delete", map[string]string{
		"channel": channelID,
		"ts":      ts,
	})
}

// AddReaction adds a reaction to a message.
func (c *Client) AddReaction(channelID, ts, name string) (map[string]any, error) {
	return c.API("reactions.add", map[string]string{
		"channel":   channelID,
		"timestamp": ts,
		"name":      name,
	})
}

// RemoveReaction removes a reaction from a message.
func (c *Client) RemoveReaction(channelID, ts, name string) (map[string]any, error) {
	return c.API("reactions.remove", map[string]string{
		"channel":   channelID,
		"timestamp": ts,
		"name":      name,
	})
}

// --- helpers ---

func toRecord(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func stringVal(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func intVal(m map[string]any, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}

func getArray(m map[string]any, key string) []any {
	if v, ok := m[key].([]any); ok {
		return v
	}
	return nil
}

func getString(m map[string]any, keys ...string) string {
	current := m
	for i, k := range keys {
		if i == len(keys)-1 {
			if v, ok := current[k].(string); ok {
				return v
			}
			return ""
		}
		if next, ok := current[k].(map[string]any); ok {
			current = next
		} else {
			return ""
		}
	}
	return ""
}

// JSONString marshals v to a JSON string, used for API params that need JSON values.
func JSONString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
