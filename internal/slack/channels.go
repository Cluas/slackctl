package slack

import (
	"fmt"
	"strconv"
	"strings"
)

// Channel represents a Slack conversation summary.
type Channel struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	IsDM    bool   `json:"is_im,omitempty"`
	IsMPIM  bool   `json:"is_mpim,omitempty"`
	Topic   string `json:"topic,omitempty"`
	Purpose string `json:"purpose,omitempty"`
	NumMembers int `json:"num_members,omitempty"`
}

// ResolveChannelID resolves a channel name or ID to a channel ID.
func (c *Client) ResolveChannelID(input string) (string, error) {
	input = NormalizeChannelInput(input)
	if IsChannelID(input) {
		return input, nil
	}
	// Try search first (faster for browser auth)
	resp, err := c.API("conversations.list", map[string]string{
		"types":            "public_channel,private_channel",
		"exclude_archived": "true",
		"limit":            "200",
	})
	if err != nil {
		return "", fmt.Errorf("failed to list channels: %w", err)
	}
	channels := getArray(resp, "channels")
	lower := strings.ToLower(input)
	for _, ch := range channels {
		rec := toRecord(ch)
		name := stringVal(rec, "name")
		if strings.ToLower(name) == lower {
			return stringVal(rec, "id"), nil
		}
	}
	return "", fmt.Errorf("channel not found: %s", input)
}

// ResolveChannelName resolves a channel ID to its display name.
func (c *Client) ResolveChannelName(channelID string) (string, error) {
	resp, err := c.API("conversations.info", map[string]string{
		"channel": channelID,
	})
	if err != nil {
		return channelID, nil // fallback to ID
	}
	ch := toRecord(resp["channel"])
	if name := stringVal(ch, "name"); name != "" {
		return name, nil
	}
	return channelID, nil
}

// ListConversations lists user's conversations.
func (c *Client) ListConversations(types string, limit int, excludeArchived bool) ([]Channel, error) {
	params := map[string]string{
		"types":            types,
		"exclude_archived": strconv.FormatBool(excludeArchived),
		"limit":            strconv.Itoa(min(limit, 200)),
	}
	var all []Channel
	for {
		resp, err := c.API("users.conversations", params)
		if err != nil {
			return nil, err
		}
		channels := getArray(resp, "channels")
		for _, ch := range channels {
			rec := toRecord(ch)
			all = append(all, parseChannel(rec))
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

// MarkConversation marks a conversation as read.
func (c *Client) MarkConversation(channelID, ts string) (map[string]any, error) {
	return c.API("conversations.mark", map[string]string{
		"channel": channelID,
		"ts":      ts,
	})
}

// OpenDMChannel opens a DM with a user.
func (c *Client) OpenDMChannel(userID string) (string, error) {
	resp, err := c.API("conversations.open", map[string]string{
		"users": userID,
	})
	if err != nil {
		return "", err
	}
	ch := toRecord(resp["channel"])
	return stringVal(ch, "id"), nil
}

// CreateChannel creates a new channel.
func (c *Client) CreateChannel(name string, isPrivate bool) (string, error) {
	resp, err := c.API("conversations.create", map[string]string{
		"name":       name,
		"is_private": strconv.FormatBool(isPrivate),
	})
	if err != nil {
		return "", err
	}
	ch := toRecord(resp["channel"])
	return stringVal(ch, "id"), nil
}

// InviteToChannel invites users to a channel.
func (c *Client) InviteToChannel(channelID string, userIDs []string) error {
	_, err := c.API("conversations.invite", map[string]string{
		"channel": channelID,
		"users":   strings.Join(userIDs, ","),
	})
	return err
}

func parseChannel(rec map[string]any) Channel {
	ch := Channel{
		ID:   stringVal(rec, "id"),
		Name: stringVal(rec, "name"),
	}
	if v, ok := rec["is_im"].(bool); ok {
		ch.IsDM = v
	}
	if v, ok := rec["is_mpim"].(bool); ok {
		ch.IsMPIM = v
	}
	if topic := toRecord(rec["topic"]); topic != nil {
		ch.Topic = stringVal(topic, "value")
	}
	if purpose := toRecord(rec["purpose"]); purpose != nil {
		ch.Purpose = stringVal(purpose, "value")
	}
	if v, ok := rec["num_members"].(float64); ok {
		ch.NumMembers = int(v)
	}
	return ch
}
