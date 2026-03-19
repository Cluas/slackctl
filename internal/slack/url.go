package slack

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// MessageRef represents a parsed Slack message URL.
type MessageRef struct {
	WorkspaceURL string
	ChannelID    string
	Timestamp    string // e.g. "1234567890.123456"
	ThreadTS     string // optional thread parent ts
}

var slackMsgURLRe = regexp.MustCompile(
	`(?i)^https?://([^/]+)/archives/([A-Z0-9]+)/p(\d{10})(\d{6})(?:\?.*thread_ts=([\d.]+))?`,
)

// ParseMessageURL extracts channel, timestamp, and optional thread_ts from a Slack message URL.
func ParseMessageURL(raw string) (*MessageRef, error) {
	m := slackMsgURLRe.FindStringSubmatch(raw)
	if m == nil {
		return nil, fmt.Errorf("not a valid Slack message URL: %s", raw)
	}
	host := m[1]
	channelID := m[2]
	ts := m[3] + "." + m[4]

	workspaceURL := "https://" + host
	ref := &MessageRef{
		WorkspaceURL: workspaceURL,
		ChannelID:    channelID,
		Timestamp:    ts,
	}
	if m[5] != "" {
		ref.ThreadTS = m[5]
	}
	return ref, nil
}

// NormalizeURL returns the canonical form of a Slack workspace URL (scheme + host).
func NormalizeURL(u string) (string, error) {
	parsed, err := url.Parse(u)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %s", u)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("invalid URL (no host): %s", u)
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

// IsChannelID returns true if s looks like a Slack channel ID (C/G/D prefix).
func IsChannelID(s string) bool {
	if len(s) < 2 {
		return false
	}
	prefix := s[0]
	return (prefix == 'C' || prefix == 'G' || prefix == 'D') && isAlphanumUpper(s[1:])
}

// IsUserID returns true if s looks like a Slack user ID (U/W prefix).
func IsUserID(s string) bool {
	if len(s) < 2 {
		return false
	}
	prefix := s[0]
	return (prefix == 'U' || prefix == 'W') && isAlphanumUpper(s[1:])
}

func isAlphanumUpper(s string) bool {
	for _, c := range s {
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

// NormalizeChannelInput strips leading # from channel names.
func NormalizeChannelInput(s string) string {
	return strings.TrimPrefix(s, "#")
}
