package slack

import (
	"fmt"
	"net/http"
)

// RTMInfo is the result of rtm.connect: a one-time websocket URL plus the
// identity of the connected user.
type RTMInfo struct {
	URL      string `json:"-"`
	SelfID   string `json:"id"`
	SelfName string `json:"name"`
	TeamID   string `json:"team_id,omitempty"`
	TeamName string `json:"team_name,omitempty"`
}

// RTMConnect calls rtm.connect and returns the websocket URL and self identity.
// Works with both standard tokens and browser (xoxc + d cookie) auth. Slack
// refuses it with enterprise_is_restricted for Enterprise Grid org-level
// sessions.
func (c *Client) RTMConnect() (RTMInfo, error) {
	resp, err := c.API("rtm.connect", map[string]string{"batch_presence_aware": "1"})
	if err != nil {
		return RTMInfo{}, err
	}
	self := toRecord(resp["self"])
	team := toRecord(resp["team"])
	info := RTMInfo{
		URL:      stringVal(resp, "url"),
		SelfID:   stringVal(self, "id"),
		SelfName: stringVal(self, "name"),
		TeamID:   stringVal(team, "id"),
		TeamName: stringVal(team, "name"),
	}
	if info.URL == "" {
		return RTMInfo{}, fmt.Errorf("rtm.connect returned no websocket url")
	}
	return info, nil
}

// WebsocketHTTPClient returns an HTTP client (same uTLS fingerprint as API
// calls, but without an overall timeout) suitable for a websocket handshake.
func (c *Client) WebsocketHTTPClient() *http.Client {
	return newStreamingHTTPClient(c.auth.Source)
}

// WebsocketHeaders returns the handshake headers: auth cookie / bearer token,
// User-Agent matching the auth source, and the Slack web client Origin.
func (c *Client) WebsocketHeaders() http.Header {
	h := http.Header{}
	for k, v := range c.AuthHeaders() {
		h.Set(k, v)
	}
	h.Set("User-Agent", c.profile().UserAgent)
	h.Set("Origin", "https://app.slack.com")
	return h
}

// ConversationIsMPIM reports whether a conversation is a multi-person DM,
// using conversations.info (read-only).
func (c *Client) ConversationIsMPIM(channelID string) (bool, error) {
	resp, err := c.API("conversations.info", map[string]string{"channel": channelID})
	if err != nil {
		return false, err
	}
	ch := toRecord(resp["channel"])
	isMPIM, _ := ch["is_mpim"].(bool)
	return isMPIM, nil
}
