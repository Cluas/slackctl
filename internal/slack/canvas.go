package slack

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	htmltomd "github.com/JohannesKaufmann/html-to-markdown/v2"
)

// CanvasRef represents a parsed Slack canvas URL.
type CanvasRef struct {
	WorkspaceURL string
	CanvasID     string
	Raw          string
}

var canvasIDRe = regexp.MustCompile(`^F[A-Z0-9]{8,}$`)

// ParseCanvasURL extracts workspace URL and canvas ID from a Slack canvas URL.
func ParseCanvasURL(input string) (*CanvasRef, error) {
	u, err := url.Parse(input)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %s", input)
	}
	if !strings.HasSuffix(strings.ToLower(u.Hostname()), ".slack.com") {
		return nil, fmt.Errorf("not a Slack workspace URL: %s", u.Hostname())
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) == 0 || parts[0] != "docs" {
		return nil, fmt.Errorf("unsupported Slack canvas URL path: %s", u.Path)
	}
	var canvasID string
	for _, p := range parts {
		if canvasIDRe.MatchString(p) {
			canvasID = p
			break
		}
	}
	if canvasID == "" {
		return nil, fmt.Errorf("could not find canvas id in: %s", u.Path)
	}
	return &CanvasRef{
		WorkspaceURL: u.Scheme + "://" + u.Host,
		CanvasID:     canvasID,
		Raw:          input,
	}, nil
}

// CanvasResult holds fetched canvas data.
type CanvasResult struct {
	ID       string `json:"id"`
	Title    string `json:"title,omitempty"`
	Markdown string `json:"markdown"`
}

// FetchCanvas fetches a Slack canvas and converts it to Markdown.
func (c *Client) FetchCanvas(canvasID string, maxChars int) (*CanvasResult, error) {
	// Get file info
	info, err := c.API("files.info", map[string]string{"file": canvasID})
	if err != nil {
		return nil, err
	}
	file := toRecord(info["file"])
	if file == nil {
		return nil, fmt.Errorf("canvas not found (files.info returned no file)")
	}

	title := stringVal(file, "title")
	if title == "" {
		title = stringVal(file, "name")
	}

	downloadURL := stringVal(file, "url_private_download")
	if downloadURL == "" {
		downloadURL = stringVal(file, "url_private")
	}
	if downloadURL == "" {
		return nil, fmt.Errorf("canvas has no download URL")
	}

	// Download HTML
	html, err := c.downloadCanvasHTML(downloadURL)
	if err != nil {
		return nil, err
	}

	// Convert HTML to Markdown
	md, err := htmlToMarkdown(html)
	if err != nil {
		return nil, fmt.Errorf("failed to convert HTML to markdown: %w", err)
	}
	md = strings.TrimSpace(md)

	if maxChars >= 0 && len(md) > maxChars {
		md = md[:maxChars] + "\n…"
	}

	return &CanvasResult{
		ID:       canvasID,
		Title:    strings.TrimSpace(title),
		Markdown: md,
	}, nil
}

func (c *Client) downloadCanvasHTML(downloadURL string) (string, error) {
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return "", err
	}

	if c.auth.Type == AuthStandard {
		req.Header.Set("Authorization", "Bearer "+c.auth.Token)
	} else {
		req.Header.Set("Authorization", "Bearer "+c.auth.XoxcToken)
		req.Header.Set("Cookie", "d="+url.QueryEscape(c.auth.XoxdCookie))
		req.Header.Set("Referer", "https://app.slack.com/")
	}
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to download canvas HTML (%d)", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func htmlToMarkdown(html string) (string, error) {
	// Extract primary content node if wrapped in <main>/<article>/<body>
	content := extractTag(html, "main")
	if content == "" {
		content = extractTag(html, "article")
	}
	if content == "" {
		content = extractTag(html, "body")
	}
	if content == "" {
		content = html
	}

	md, err := htmltomd.ConvertString(content)
	if err != nil {
		return "", err
	}
	return md, nil
}

var tagRe = map[string]*regexp.Regexp{}

func extractTag(html, tag string) string {
	re, ok := tagRe[tag]
	if !ok {
		re = regexp.MustCompile(`(?is)<` + tag + `\b[^>]*>([\s\S]*?)</` + tag + `>`)
		tagRe[tag] = re
	}
	m := re.FindStringSubmatch(html)
	if m == nil {
		return ""
	}
	return m[1]
}
