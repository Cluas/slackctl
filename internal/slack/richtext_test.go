package slack

import "testing"

func rtText(text string, style map[string]any) map[string]any {
	m := map[string]any{"type": "text", "text": text}
	if style != nil {
		m["style"] = style
	}
	return m
}

func rtCell(elems ...any) map[string]any {
	return map[string]any{
		"type": "rich_text",
		"elements": []any{
			map[string]any{"type": "rich_text_section", "elements": elems},
		},
	}
}

var code = map[string]any{"code": true}

func TestRenderInlineMergesAdjacentBoldRuns(t *testing.T) {
	got := renderInline([]any{
		rtText("Q1: ", bold),
		rtText("abc123", map[string]any{"bold": true, "code": true}),
		rtText(" where", bold),
		rtText(" plain", nil),
	})
	want := "*Q1: `abc123` where* plain"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseMessageRendersTableBlocksOverTextFallback(t *testing.T) {
	// Shape observed on AI-assistant bots: "text" is a non-empty fallback
	// that silently drops table blocks; the full content lives in blocks.
	msg := map[string]any{
		"ts":     "123.456",
		"bot_id": "B123",
		"text":   "*Summary* intro\n\n*Section 2*\n\n after table",
		"blocks": []any{
			map[string]any{
				"type": "rich_text",
				"elements": []any{
					map[string]any{"type": "rich_text_section", "elements": []any{
						rtText("Summary", bold), rtText(" intro\n\n", nil),
					}},
					map[string]any{"type": "rich_text_list", "style": "bullet", "indent": float64(0), "elements": []any{
						map[string]any{"type": "rich_text_section", "elements": []any{rtText("first ", nil), rtText("abc123", code)}},
						map[string]any{"type": "rich_text_section", "elements": []any{rtText("second", nil)}},
					}},
					map[string]any{"type": "rich_text_section", "elements": []any{rtText("Section 2", bold)}},
				},
			},
			map[string]any{
				"type": "table",
				"rows": []any{
					[]any{rtCell(rtText("Keyword", nil)), rtCell(rtText("Result", nil))},
					[]any{rtCell(rtText("FOO_BAR", code)), rtCell(rtText("a|b\nc", nil))},
					[]any{rtCell(rtText("only one cell", nil))},
				},
			},
			map[string]any{
				"type": "rich_text",
				"elements": []any{
					map[string]any{"type": "rich_text_section", "elements": []any{rtText("after table", nil)}},
				},
			},
			map[string]any{
				"type": "actions",
				"elements": []any{
					map[string]any{"type": "button", "text": map[string]any{"type": "plain_text", "text": "👍"}},
				},
			},
		},
	}

	got := parseMessage(msg, "C123").Text
	want := "*Summary* intro\n" +
		"• first `abc123`\n" +
		"• second\n" +
		"*Section 2*\n" +
		"\n" +
		"| Keyword | Result |\n" +
		"| --- | --- |\n" +
		"| `FOO_BAR` | a\\|b c |\n" +
		"| only one cell |  |\n" +
		"\n" +
		"after table"
	if got != want {
		t.Errorf("Text =\n%s\n\nwant\n%s", got, want)
	}
}

func TestParseMessageKeepsTextWhenNoTable(t *testing.T) {
	// Ordinary user messages carry rich_text blocks that mirror "text";
	// without a table the original "text" must be kept untouched.
	msg := map[string]any{
		"ts":   "123.456",
		"text": "hi <@U1>",
		"blocks": []any{
			map[string]any{"type": "rich_text", "elements": []any{
				map[string]any{"type": "rich_text_section", "elements": []any{
					rtText("hi ", nil), map[string]any{"type": "user", "user_id": "U1"},
				}},
			}},
		},
	}
	if got := parseMessage(msg, "C123").Text; got != "hi <@U1>" {
		t.Errorf("Text = %q, want %q", got, "hi <@U1>")
	}
}

func TestRenderRichTextElements(t *testing.T) {
	block := map[string]any{
		"type": "rich_text",
		"elements": []any{
			map[string]any{"type": "rich_text_section", "elements": []any{
				rtText("see ", nil),
				map[string]any{"type": "link", "url": "https://x.test", "text": "docs"},
				rtText(" ", nil),
				map[string]any{"type": "emoji", "name": "thumbsup", "unicode": "1f44d"},
				rtText(" bold ", bold),
				map[string]any{"type": "channel", "channel_id": "C9"},
			}},
			map[string]any{"type": "rich_text_list", "style": "ordered", "indent": float64(1), "elements": []any{
				map[string]any{"type": "rich_text_section", "elements": []any{rtText("one", nil)}},
				map[string]any{"type": "rich_text_section", "elements": []any{rtText("two", nil)}},
			}},
			map[string]any{"type": "rich_text_preformatted", "elements": []any{rtText("go test ./...", nil)}},
			map[string]any{"type": "rich_text_quote", "elements": []any{rtText("line1\nline2", nil)}},
		},
	}
	want := "see <https://x.test|docs> 👍 *bold* <#C9>\n" +
		"  1. one\n" +
		"  2. two\n" +
		"```\ngo test ./...\n```\n" +
		"> line1\n> line2"
	if got := renderRichText(block); got != want {
		t.Errorf("got\n%s\n\nwant\n%s", got, want)
	}
}

func TestParseMessageRendersRichTextWhenTextEmpty(t *testing.T) {
	msg := map[string]any{
		"ts":   "123.456",
		"text": "",
		"blocks": []any{
			map[string]any{"type": "rich_text", "elements": []any{
				map[string]any{"type": "rich_text_section", "elements": []any{rtText("only blocks", nil)}},
			}},
		},
	}
	if got := parseMessage(msg, "C123").Text; got != "only blocks" {
		t.Errorf("Text = %q, want %q", got, "only blocks")
	}
}
