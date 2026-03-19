package auth

import "testing"

func TestParseSlackCurlCommand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantURL string
		wantErr bool
	}{
		{
			name: "standard curl with cookie header",
			input: `curl 'https://myteam.slack.com/api/conversations.history' \
  -H 'Cookie: d=xoxd-abc123' \
  --data-raw 'token=xoxc-111-222-333-aaa'`,
			wantURL: "https://myteam.slack.com",
		},
		{
			name: "curl with -b flag",
			input: `curl 'https://myteam.slack.com/api/chat.postMessage' \
  -b 'd=xoxd-cookie456' \
  -d 'token=xoxc-444-555-666-bbb'`,
			wantURL: "https://myteam.slack.com",
		},
		{
			name: "token in JSON body",
			input: `curl 'https://team.slack.com/api/users.info' \
  -H 'Cookie: d=xoxd-test789' \
  --data-raw '{"token":"xoxc-777-888-999-ccc"}'`,
			wantURL: "https://team.slack.com",
		},
		{
			name:    "no slack URL",
			input:   `curl 'https://example.com/api'`,
			wantErr: true,
		},
		{
			name: "no xoxd cookie",
			input: `curl 'https://myteam.slack.com/api/test' \
  -H 'Cookie: session=abc' \
  -d 'token=xoxc-111-222-333-aaa'`,
			wantErr: true,
		},
		{
			name: "no xoxc token",
			input: `curl 'https://myteam.slack.com/api/test' \
  -H 'Cookie: d=xoxd-abc123'`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseSlackCurlCommand(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if parsed.WorkspaceURL != tt.wantURL {
				t.Errorf("WorkspaceURL = %q, want %q", parsed.WorkspaceURL, tt.wantURL)
			}
			if parsed.XoxcToken == "" {
				t.Error("XoxcToken is empty")
			}
			if parsed.XoxdCookie == "" {
				t.Error("XoxdCookie is empty")
			}
		})
	}
}
