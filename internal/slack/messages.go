package slack

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// MessageSummary is the core message data structure.
type MessageSummary struct {
	ChannelID   string              `json:"channel_id,omitempty"`
	ChannelName string              `json:"channel_name,omitempty"`
	ThreadTS    string              `json:"thread_ts,omitempty"`
	TS          string              `json:"ts"`
	User        string              `json:"user,omitempty"`
	UserName    string              `json:"user_name,omitempty"`
	Text        string              `json:"text,omitempty"`
	Attachments []AttachmentSummary `json:"attachments,omitempty"`
	Reactions   []Reaction          `json:"reactions,omitempty"`
	Files       []FileSummary       `json:"files,omitempty"`
	ReplyCount  int                 `json:"reply_count,omitempty"`
	Permalink   string              `json:"permalink,omitempty"`
}

// AttachmentSummary projects the content-bearing fields of a legacy
// attachment. Color, id, and other presentation fields are dropped.
type AttachmentSummary struct {
	Title string `json:"title,omitempty"`
	Text  string `json:"text,omitempty"`
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
		// Bot messages carry no "user"; they name their author in "username".
		UserName: stringVal(msg, "username"),
		Text:     stringVal(msg, "text"),
		ThreadTS: stringVal(msg, "thread_ts"),
	}

	// Bot/app messages (e.g. monitoring alert cards) often carry no plain
	// "text" and render entirely through Block Kit blocks/attachments.
	if summary.Text == "" {
		summary.Text = renderBlockKitText(msg)
	}

	// Attachment bodies roughly double the size of a channel's output, so they
	// are opt-in via --attachments.
	if atts, ok := msg["attachments"].([]any); ok && includeAttachments() {
		for _, a := range atts {
			am := toRecord(a)
			att := AttachmentSummary{
				Title: stringVal(am, "title"),
				Text:  stringVal(am, "text"),
			}
			// Link unfurls carry their content in "blocks" and would
			// otherwise project as an empty object.
			if att.Title == "" && att.Text == "" {
				continue
			}
			summary.Attachments = append(summary.Attachments, att)
		}
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

// renderBlockKitText reconstructs readable text from a message's Block Kit
// blocks and/or legacy attachments, for messages that carry no plain "text"
// (typically monitoring/alert cards posted by bots and apps).
func renderBlockKitText(msg map[string]any) string {
	var parts []string
	if blocks, ok := msg["blocks"].([]any); ok {
		if s := renderBlocks(blocks); s != "" {
			parts = append(parts, s)
		}
	}
	if attachments, ok := msg["attachments"].([]any); ok {
		for _, a := range attachments {
			if s := renderAttachment(toRecord(a)); s != "" {
				parts = append(parts, s)
			}
		}
	}
	return strings.Join(parts, "\n---\n")
}

// renderAttachment renders a single attachment: modern attachments carry
// their own "blocks", while legacy ones use pretext/title/text/fields.
func renderAttachment(att map[string]any) string {
	if blocks, ok := att["blocks"].([]any); ok {
		if s := renderBlocks(blocks); s != "" {
			return s
		}
	}
	var lines []string
	for _, key := range []string{"pretext", "title", "text"} {
		if v := stringVal(att, key); v != "" {
			lines = append(lines, v)
		}
	}
	if fields, ok := att["fields"].([]any); ok {
		for _, f := range fields {
			fm := toRecord(f)
			title, value := stringVal(fm, "title"), stringVal(fm, "value")
			switch {
			case title != "" && value != "":
				lines = append(lines, fmt.Sprintf("%s: %s", title, value))
			case value != "":
				lines = append(lines, value)
			}
		}
	}
	return strings.Join(lines, "\n")
}

// renderBlocks joins the readable text of section/header/context blocks.
// Structural blocks (divider, actions, input, image, ...) carry no message
// text and are skipped.
func renderBlocks(blocks []any) string {
	var lines []string
	for _, b := range blocks {
		bm := toRecord(b)
		switch stringVal(bm, "type") {
		case "section", "header":
			if v := renderTextObject(bm["text"]); v != "" {
				lines = append(lines, v)
			}
		case "context":
			var elems []string
			for _, e := range getArray(bm, "elements") {
				if v := renderTextObject(e); v != "" {
					elems = append(elems, v)
				}
			}
			if len(elems) > 0 {
				lines = append(lines, strings.Join(elems, "  "))
			}
		}
	}
	return strings.Join(lines, "\n")
}

// renderTextObject extracts text from a Block Kit text object
// ({"type": "mrkdwn"|"plain_text", "text": "..."}).
func renderTextObject(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	switch stringVal(m, "type") {
	case "mrkdwn", "plain_text":
		return stringVal(m, "text")
	}
	return ""
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
