package slack

import "testing"

func TestParseMessageURL(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantHost  string
		wantCh    string
		wantTS    string
		wantThTS  string
		wantErr   bool
	}{
		{
			name:     "standard message URL",
			input:    "https://myteam.slack.com/archives/C0123ABC/p1700000000123456",
			wantHost: "https://myteam.slack.com",
			wantCh:   "C0123ABC",
			wantTS:   "1700000000.123456",
		},
		{
			name:     "with thread_ts",
			input:    "https://myteam.slack.com/archives/C0123ABC/p1700000000123456?thread_ts=1699999999.000000",
			wantHost: "https://myteam.slack.com",
			wantCh:   "C0123ABC",
			wantTS:   "1700000000.123456",
			wantThTS: "1699999999.000000",
		},
		{
			name:     "enterprise URL",
			input:    "https://company.enterprise.slack.com/archives/G0123DEF/p1600000000654321",
			wantHost: "https://company.enterprise.slack.com",
			wantCh:   "G0123DEF",
			wantTS:   "1600000000.654321",
		},
		{
			name:    "not a slack URL",
			input:   "https://example.com/foo",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParseMessageURL(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ref.WorkspaceURL != tt.wantHost {
				t.Errorf("WorkspaceURL = %q, want %q", ref.WorkspaceURL, tt.wantHost)
			}
			if ref.ChannelID != tt.wantCh {
				t.Errorf("ChannelID = %q, want %q", ref.ChannelID, tt.wantCh)
			}
			if ref.Timestamp != tt.wantTS {
				t.Errorf("Timestamp = %q, want %q", ref.Timestamp, tt.wantTS)
			}
			if ref.ThreadTS != tt.wantThTS {
				t.Errorf("ThreadTS = %q, want %q", ref.ThreadTS, tt.wantThTS)
			}
		})
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"https://myteam.slack.com/foo/bar", "https://myteam.slack.com", false},
		{"https://myteam.slack.com", "https://myteam.slack.com", false},
		{"https://myteam.slack.com/", "https://myteam.slack.com", false},
		{"not-a-url", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := NormalizeURL(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsChannelID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"C0123ABC", true},
		{"G0123DEF", true},
		{"D0123GHI", true},
		{"C1", true},
		{"general", false},
		{"U0123ABC", false},
		{"", false},
		{"C", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := IsChannelID(tt.input); got != tt.want {
				t.Errorf("IsChannelID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsUserID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"U0123ABC", true},
		{"W0123DEF", true},
		{"C0123ABC", false},
		{"alice", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := IsUserID(tt.input); got != tt.want {
				t.Errorf("IsUserID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeChannelInput(t *testing.T) {
	if got := NormalizeChannelInput("#general"); got != "general" {
		t.Errorf("got %q, want %q", got, "general")
	}
	if got := NormalizeChannelInput("general"); got != "general" {
		t.Errorf("got %q, want %q", got, "general")
	}
}
