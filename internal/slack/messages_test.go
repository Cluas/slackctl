package slack

import "testing"

func TestParseMessageRendersAttachmentBlocks(t *testing.T) {
	// Shape observed on monitoring-alert bots (e.g. member-alert channels):
	// "text" is empty and the whole message renders through
	// attachments[].blocks.
	msg := map[string]any{
		"ts":   "123.456",
		"user": "U123",
		"text": "",
		"attachments": []any{
			map[string]any{
				"id":       float64(1),
				"color":    "#FFC300",
				"fallback": "[no preview available]",
				"blocks": []any{
					map[string]any{
						"type": "section",
						"text": map[string]any{
							"type": "mrkdwn",
							"text": "*:warning: P2 Warning - prod task alert*",
						},
					},
					map[string]any{
						"type": "context",
						"elements": []any{
							map[string]any{
								"type": "mrkdwn",
								"text": ":clock2: *time:* 2026-08-19 11:43:45",
							},
						},
					},
					map[string]any{
						"type": "actions",
						"elements": []any{
							map[string]any{"type": "static_select"},
						},
					},
				},
			},
		},
	}

	got := parseMessage(msg, "C123")
	want := "*:warning: P2 Warning - prod task alert*\n:clock2: *time:* 2026-08-19 11:43:45"
	if got.Text != want {
		t.Errorf("Text = %q, want %q", got.Text, want)
	}
}

func TestParseMessageRendersLegacyAttachmentFields(t *testing.T) {
	msg := map[string]any{
		"ts": "123.456",
		"attachments": []any{
			map[string]any{
				"pretext": "Heads up",
				"title":   "Build failed",
				"text":    "see logs for details",
				"fields": []any{
					map[string]any{"title": "branch", "value": "main"},
					map[string]any{"value": "no-title-value"},
				},
			},
		},
	}

	got := parseMessage(msg, "C123")
	want := "Heads up\nBuild failed\nsee logs for details\nbranch: main\nno-title-value"
	if got.Text != want {
		t.Errorf("Text = %q, want %q", got.Text, want)
	}
}

func TestParseMessagePrefersPlainText(t *testing.T) {
	msg := map[string]any{
		"ts":   "123.456",
		"text": "hello there",
		"attachments": []any{
			map[string]any{"fallback": "ignored"},
		},
	}

	got := parseMessage(msg, "C123")
	if got.Text != "hello there" {
		t.Errorf("Text = %q, want %q", got.Text, "hello there")
	}
}

func TestParseMessageTopLevelBlocks(t *testing.T) {
	msg := map[string]any{
		"ts": "123.456",
		"blocks": []any{
			map[string]any{
				"type": "header",
				"text": map[string]any{
					"type": "plain_text",
					"text": "Deploy finished",
				},
			},
		},
	}

	got := parseMessage(msg, "C123")
	if got.Text != "Deploy finished" {
		t.Errorf("Text = %q, want %q", got.Text, "Deploy finished")
	}
}
