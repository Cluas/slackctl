package slack

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseUser_Status(t *testing.T) {
	u := parseUser(map[string]any{
		"id":        "U123",
		"name":      "alice",
		"real_name": "Alice Smith",
		"profile": map[string]any{
			"display_name":      "alice",
			"status_text":       "On vacation until Tuesday",
			"status_emoji":      ":palm_tree:",
			"status_expiration": float64(1794931200),
		},
	})
	if u.StatusText != "On vacation until Tuesday" {
		t.Errorf("unexpected status_text %q", u.StatusText)
	}
	if u.StatusEmoji != ":palm_tree:" {
		t.Errorf("unexpected status_emoji %q", u.StatusEmoji)
	}
	if u.StatusExpiration != 1794931200 {
		t.Errorf("unexpected status_expiration %d", u.StatusExpiration)
	}
}

func TestParseUser_NoStatusOmitsFields(t *testing.T) {
	u := parseUser(map[string]any{
		"id": "U123",
		"profile": map[string]any{
			"status_text":       "",
			"status_emoji":      "",
			"status_expiration": float64(0),
		},
	})
	out, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "status_") {
		t.Errorf("empty status should be omitted, got %s", out)
	}
}
