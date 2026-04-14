package slack

import "testing"

func TestAuthHeaders_Standard(t *testing.T) {
	c := NewClient(Auth{Type: AuthStandard, Token: "xoxb-test-token"}, "https://test.slack.com")
	h := c.AuthHeaders()
	if h["Authorization"] != "Bearer xoxb-test-token" {
		t.Errorf("expected Bearer token, got %q", h["Authorization"])
	}
	if _, ok := h["Cookie"]; ok {
		t.Error("standard auth should not set Cookie header")
	}
}

func TestAuthHeaders_Browser(t *testing.T) {
	c := NewClient(Auth{
		Type:       AuthBrowser,
		XoxcToken:  "xoxc-test",
		XoxdCookie: "xoxd-test",
	}, "https://test.slack.com")
	h := c.AuthHeaders()
	if _, ok := h["Authorization"]; ok {
		t.Error("browser auth should not set Authorization header")
	}
	if h["Cookie"] != "d=xoxd-test" {
		t.Errorf("expected cookie header, got %q", h["Cookie"])
	}
}

func TestPercentEncodeCookie(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple ascii",
			input: "xoxd-abc123",
			want:  "xoxd-abc123",
		},
		{
			name:  "with slashes and plus",
			input: "xoxd-7K1/pOc+ZRH",
			want:  "xoxd-7K1%2FpOc%2BZRH",
		},
		{
			name:  "with equals",
			input: "xoxd-abc==",
			want:  "xoxd-abc%3D%3D",
		},
		{
			name:  "empty",
			input: "",
			want:  "",
		},
		{
			name:  "unreserved chars preserved",
			input: "abc-_.~!*'()",
			want:  "abc-_.~!*'()",
		},
		{
			name:  "space encoded as %20",
			input: "a b",
			want:  "a%20b",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := percentEncodeCookie(tt.input)
			if got != tt.want {
				t.Errorf("percentEncodeCookie(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
