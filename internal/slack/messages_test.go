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

func TestParseMessageProjectsBotUsername(t *testing.T) {
	// Bot messages set "username" and no "user".
	msg := map[string]any{
		"ts":       "123.456",
		"subtype":  "bot_message",
		"username": "deploy-bot",
		"text":     "deploy finished",
	}

	got := parseMessage(msg, "C123")
	if got.UserName != "deploy-bot" {
		t.Errorf("UserName = %q, want %q", got.UserName, "deploy-bot")
	}
	if got.User != "" {
		t.Errorf("User = %q, want empty", got.User)
	}
}

func TestParseMessageOmitsAttachmentsByDefault(t *testing.T) {
	t.Setenv("SLACKCTL_ATTACHMENTS", "")

	msg := map[string]any{
		"ts":   "123.456",
		"text": "deploy finished",
		"attachments": []any{
			map[string]any{
				"id":    1,
				"color": "CEF1F3",
				"title": "Status",
				"text":  "all checks passed",
			},
		},
	}

	got := parseMessage(msg, "C123")
	if got.Attachments != nil {
		t.Errorf("Attachments = %v, want nil without --attachments", got.Attachments)
	}
	if got.Text != "deploy finished" {
		t.Errorf("Text = %q", got.Text)
	}
}

func TestParseMessageProjectsAttachmentTitleAndText(t *testing.T) {
	t.Setenv("SLACKCTL_ATTACHMENTS", "1")

	msg := map[string]any{
		"ts":   "123.456",
		"text": "deploy finished",
		"attachments": []any{
			map[string]any{
				"id":       1,
				"color":    "CEF1F3",
				"title":    "Status",
				"text":     "all checks passed",
				"fallback": "Status\nall checks passed",
			},
			map[string]any{
				"id":    2,
				"title": "Next",
				"text":  "promote to staging",
			},
		},
	}

	got := parseMessage(msg, "C123")
	if len(got.Attachments) != 2 {
		t.Fatalf("len(Attachments) = %d, want 2", len(got.Attachments))
	}
	if got.Attachments[0].Title != "Status" {
		t.Errorf("Attachments[0].Title = %q", got.Attachments[0].Title)
	}
	if got.Attachments[0].Text != "all checks passed" {
		t.Errorf("Attachments[0].Text = %q", got.Attachments[0].Text)
	}
	if got.Attachments[1].Title != "Next" {
		t.Errorf("Attachments[1].Title = %q", got.Attachments[1].Title)
	}
	if got.Text != "deploy finished" {
		t.Errorf("Text = %q, want the plain text", got.Text)
	}
}

func TestParseMessageSkipsContentlessAttachments(t *testing.T) {
	t.Setenv("SLACKCTL_ATTACHMENTS", "1")

	// Link unfurls carry their content in attachments[].blocks and have no
	// title/text, so they must not project as empty objects.
	msg := map[string]any{
		"ts":   "123.456",
		"text": "fyi, see this link",
		"attachments": []any{
			map[string]any{
				"id": 1,
				"blocks": []any{
					map[string]any{
						"type": "section",
						"text": map[string]any{"type": "mrkdwn", "text": "unfurled"},
					},
				},
			},
		},
	}

	got := parseMessage(msg, "C123")
	if len(got.Attachments) != 0 {
		t.Errorf("len(Attachments) = %d, want 0", len(got.Attachments))
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
