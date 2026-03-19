package slack

import "testing"

func TestParseCanvasURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantID   string
		wantHost string
		wantErr  bool
	}{
		{
			name:     "standard canvas URL",
			input:    "https://myteam.slack.com/docs/T123/F0ABCDEF123",
			wantID:   "F0ABCDEF123",
			wantHost: "https://myteam.slack.com",
		},
		{
			name:    "not a slack URL",
			input:   "https://example.com/docs/F123",
			wantErr: true,
		},
		{
			name:    "no docs path",
			input:   "https://myteam.slack.com/archives/C123",
			wantErr: true,
		},
		{
			name:    "no canvas ID",
			input:   "https://myteam.slack.com/docs/T123",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParseCanvasURL(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ref.CanvasID != tt.wantID {
				t.Errorf("CanvasID = %q, want %q", ref.CanvasID, tt.wantID)
			}
			if ref.WorkspaceURL != tt.wantHost {
				t.Errorf("WorkspaceURL = %q, want %q", ref.WorkspaceURL, tt.wantHost)
			}
		})
	}
}

func TestHtmlToMarkdown(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple paragraph",
			input: "<p>Hello world</p>",
			want:  "Hello world",
		},
		{
			name:  "heading",
			input: "<h1>Title</h1><p>Body</p>",
		},
		{
			name:  "extracts from main tag",
			input: "<html><body><main><p>Content</p></main></body></html>",
			want:  "Content",
		},
		{
			name:  "extracts from article tag",
			input: "<html><body><article><p>Article content</p></article></body></html>",
			want:  "Article content",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := htmlToMarkdown(tt.input)
			if err != nil {
				t.Fatalf("htmlToMarkdown: %v", err)
			}
			if tt.want != "" && got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if got == "" {
				t.Error("empty result")
			}
		})
	}
}

func TestExtractTag(t *testing.T) {
	html := `<html><body><main class="content">inner text</main></body></html>`
	got := extractTag(html, "main")
	if got != "inner text" {
		t.Errorf("got %q, want %q", got, "inner text")
	}
	got = extractTag(html, "article")
	if got != "" {
		t.Errorf("expected empty for missing tag, got %q", got)
	}
}
