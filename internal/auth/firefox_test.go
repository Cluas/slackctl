package auth

import "testing"

func TestCleanURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "clean URL unchanged",
			input: "https://myteam.slack.com/",
			want:  "https://myteam.slack.com/",
		},
		{
			name:  "control chars stripped",
			input: "https://l\x15\x1b-group.slack.com/",
			want:  "https://l-group.slack.com/",
		},
		{
			name:  "replacement char stripped",
			input: "https://l\uFFFD-group.slack.com/",
			want:  "https://l-group.slack.com/",
		},
		{
			name:  "mixed junk",
			input: "https://te\x00\x1f\x7f\x80\uFFFDam.slack.com/",
			want:  "https://team.slack.com/",
		},
		{
			name:  "empty",
			input: "",
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanURL(tt.input)
			if got != tt.want {
				t.Errorf("cleanURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractTeamsFromRawText(t *testing.T) {
	raw := `{"teams":{"T1":{"name":"My Team","url":"https://myteam.slack.com/","token":"xoxc-111-222-333-abc"}}}`
	teams := extractTeamsFromRawText(raw)
	if len(teams) != 1 {
		t.Fatalf("expected 1 team, got %d", len(teams))
	}
	if teams[0].Name != "My Team" {
		t.Errorf("name = %q", teams[0].Name)
	}
	if teams[0].URL != "https://myteam.slack.com/" {
		t.Errorf("url = %q", teams[0].URL)
	}
	if teams[0].Token != "xoxc-111-222-333-abc" {
		t.Errorf("token = %q", teams[0].Token)
	}
}

func TestExtractTeamsFromRawText_NoMatch(t *testing.T) {
	teams := extractTeamsFromRawText("no json here")
	if len(teams) != 0 {
		t.Errorf("expected 0 teams, got %d", len(teams))
	}
}

func TestDecodeFirefoxCookie(t *testing.T) {
	// Double-encoded
	got := decodeFirefoxCookie("xoxd-abc%252Fdef")
	if got != "xoxd-abc/def" {
		t.Errorf("got %q, want %q", got, "xoxd-abc/def")
	}
	// Single-encoded
	got = decodeFirefoxCookie("xoxd-abc%2Fdef")
	if got != "xoxd-abc/def" {
		t.Errorf("got %q, want %q", got, "xoxd-abc/def")
	}
	// Not encoded
	got = decodeFirefoxCookie("xoxd-plain")
	if got != "xoxd-plain" {
		t.Errorf("got %q, want %q", got, "xoxd-plain")
	}
}
