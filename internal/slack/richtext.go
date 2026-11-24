package slack

import (
	"strconv"
	"strings"
)

// hasBlockType reports whether any top-level block has the given type.
func hasBlockType(blocks []any, blockType string) bool {
	for _, b := range blocks {
		if stringVal(toRecord(b), "type") == blockType {
			return true
		}
	}
	return false
}

// renderRichText renders a "rich_text" block as Slack mrkdwn-style text.
func renderRichText(block map[string]any) string {
	var parts []string
	for _, e := range getArray(block, "elements") {
		if s := renderRichTextElement(toRecord(e)); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// renderRichTextElement renders one block-level element of a rich_text block.
func renderRichTextElement(el map[string]any) string {
	switch stringVal(el, "type") {
	case "rich_text_section":
		return strings.TrimRight(renderInline(getArray(el, "elements")), "\n")
	case "rich_text_list":
		indent := strings.Repeat("  ", intVal(el, "indent"))
		ordered := stringVal(el, "style") == "ordered"
		n := intVal(el, "offset")
		var lines []string
		for _, item := range getArray(el, "elements") {
			n++
			marker := "• "
			if ordered {
				marker = strconv.Itoa(n) + ". "
			}
			text := strings.TrimRight(renderInline(getArray(toRecord(item), "elements")), "\n")
			lines = append(lines, indent+marker+text)
		}
		return strings.Join(lines, "\n")
	case "rich_text_preformatted":
		return "```\n" + strings.Trim(renderInline(getArray(el, "elements")), "\n") + "\n```"
	case "rich_text_quote":
		text := strings.TrimRight(renderInline(getArray(el, "elements")), "\n")
		return "> " + strings.ReplaceAll(text, "\n", "\n> ")
	}
	return ""
}

// renderInline renders the inline elements of a section, list item, etc.
func renderInline(elements []any) string {
	var b strings.Builder
	for i := 0; i < len(elements); i++ {
		el := toRecord(elements[i])
		switch stringVal(el, "type") {
		case "text":
			// Consecutive bold runs (e.g. bold, then bold+code) share one
			// pair of "*" so they don't render as "*a**`b`*".
			if isBold(el) {
				var inner strings.Builder
				for ; i < len(elements) && isBold(toRecord(elements[i])); i++ {
					run := toRecord(elements[i])
					inner.WriteString(applyStyle(stringVal(run, "text"), withoutBold(toRecord(run["style"]))))
				}
				i--
				b.WriteString(applyStyle(inner.String(), bold))
				continue
			}
			b.WriteString(applyStyle(stringVal(el, "text"), toRecord(el["style"])))
		case "link":
			url, text := stringVal(el, "url"), stringVal(el, "text")
			if text == "" || text == url {
				b.WriteString(applyStyle("<"+url+">", toRecord(el["style"])))
			} else {
				b.WriteString(applyStyle("<"+url+"|"+text+">", toRecord(el["style"])))
			}
		case "user":
			b.WriteString("<@" + stringVal(el, "user_id") + ">")
		case "usergroup":
			b.WriteString("<!subteam^" + stringVal(el, "usergroup_id") + ">")
		case "channel":
			b.WriteString("<#" + stringVal(el, "channel_id") + ">")
		case "broadcast":
			b.WriteString("<!" + stringVal(el, "range") + ">")
		case "emoji":
			if u := stringVal(el, "unicode"); u != "" {
				if s := unicodeFromHex(u); s != "" {
					b.WriteString(s)
					continue
				}
			}
			b.WriteString(":" + stringVal(el, "name") + ":")
		case "date":
			b.WriteString(stringVal(el, "fallback"))
		}
	}
	return b.String()
}

var bold = map[string]any{"bold": true}

func isBold(el map[string]any) bool {
	return stringVal(el, "type") == "text" && toRecord(el["style"])["bold"] == true
}

func withoutBold(style map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range style {
		if k != "bold" {
			out[k] = v
		}
	}
	return out
}

// applyStyle wraps text in mrkdwn markers. Leading/trailing whitespace stays
// outside the markers so "*foo *" does not break bold rendering.
func applyStyle(text string, style map[string]any) string {
	if len(style) == 0 || strings.TrimSpace(text) == "" {
		return text
	}
	core := strings.TrimSpace(text)
	lead := text[:strings.Index(text, core)]
	trail := text[len(lead)+len(core):]
	if style["code"] == true {
		core = "`" + core + "`"
	}
	if style["strike"] == true {
		core = "~" + core + "~"
	}
	if style["italic"] == true {
		core = "_" + core + "_"
	}
	if style["bold"] == true {
		core = "*" + core + "*"
	}
	return lead + core + trail
}

// unicodeFromHex turns an emoji "unicode" field like "1f44d" or
// "1f3f3-fe0f" into the actual characters.
func unicodeFromHex(hex string) string {
	var b strings.Builder
	for _, part := range strings.Split(hex, "-") {
		cp, err := strconv.ParseUint(part, 16, 32)
		if err != nil {
			return ""
		}
		b.WriteRune(rune(cp))
	}
	return b.String()
}

// renderTable renders a "table" block as a Markdown table. The first row is
// treated as the header, matching how Slack displays it.
func renderTable(block map[string]any) string {
	var rows [][]string
	width := 0
	for _, r := range getArray(block, "rows") {
		cells, _ := r.([]any)
		var row []string
		for _, c := range cells {
			row = append(row, renderTableCell(toRecord(c)))
		}
		if len(row) > width {
			width = len(row)
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 || width == 0 {
		return ""
	}
	var lines []string
	writeRow := func(row []string) {
		for len(row) < width {
			row = append(row, "")
		}
		lines = append(lines, "| "+strings.Join(row, " | ")+" |")
	}
	writeRow(rows[0])
	sep := make([]string, width)
	for i := range sep {
		sep[i] = "---"
	}
	writeRow(sep)
	for _, row := range rows[1:] {
		writeRow(row)
	}
	return strings.Join(lines, "\n")
}

// renderTableCell renders one cell (a rich_text block or {"type":"raw_text"})
// flattened to a single line with pipes escaped.
func renderTableCell(cell map[string]any) string {
	var s string
	switch stringVal(cell, "type") {
	case "rich_text":
		s = renderRichText(cell)
	case "raw_text":
		s = stringVal(cell, "text")
	default:
		s = renderTextObject(cell)
	}
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
	return s
}
