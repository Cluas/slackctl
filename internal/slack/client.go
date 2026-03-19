package slack

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"

// Client wraps Slack Web API calls, supporting both standard and browser auth.
type Client struct {
	auth         Auth
	workspaceURL string
	httpClient   *http.Client
}

func NewClient(auth Auth, workspaceURL string) *Client {
	return &Client{
		auth:         auth,
		workspaceURL: strings.TrimRight(workspaceURL, "/"),
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// API calls a Slack Web API method and returns the parsed JSON response.
func (c *Client) API(method string, params map[string]string) (map[string]any, error) {
	if c.auth.Type == AuthBrowser {
		return c.browserAPI(method, params, 0)
	}
	return c.standardAPI(method, params)
}

func (c *Client) standardAPI(method string, params map[string]string) (map[string]any, error) {
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}

	req, err := http.NewRequest("POST", "https://slack.com/api/"+method, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+c.auth.Token)
	req.Header.Set("User-Agent", userAgent)

	return c.doRequest(req, method)
}

func (c *Client) browserAPI(method string, params map[string]string, attempt int) (map[string]any, error) {
	if c.workspaceURL == "" {
		return nil, fmt.Errorf("browser auth requires workspace URL")
	}

	form := url.Values{}
	form.Set("token", c.auth.XoxcToken)
	for k, v := range params {
		form.Set(k, v)
	}

	apiURL := c.workspaceURL + "/api/" + method
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "d="+percentEncodeCookie(c.auth.XoxdCookie))
	req.Header.Set("Origin", "https://app.slack.com")
	req.Header.Set("User-Agent", userAgent)
	setBrowserHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Rate limit retry
	if resp.StatusCode == 429 && attempt < 3 {
		retryAfter := 5.0
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if v, err := strconv.ParseFloat(ra, 64); err == nil {
				retryAfter = v
			}
		}
		delay := time.Duration(math.Min(math.Max(retryAfter, 1)*1000, 30000)) * time.Millisecond
		time.Sleep(delay)
		return c.browserAPI(method, params, attempt+1)
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode response for %s: %w", method, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Slack HTTP %d calling %s", resp.StatusCode, method)
	}
	if ok, _ := data["ok"].(bool); !ok {
		if errStr, _ := data["error"].(string); errStr != "" {
			return nil, fmt.Errorf("Slack API error: %s", errStr)
		}
		return nil, fmt.Errorf("Slack API error calling %s", method)
	}
	return data, nil
}

func (c *Client) doRequest(req *http.Request, method string) (map[string]any, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode response for %s: %w", method, err)
	}

	if ok, _ := data["ok"].(bool); !ok {
		if errStr, _ := data["error"].(string); errStr != "" {
			return nil, fmt.Errorf("Slack API error: %s", errStr)
		}
		return nil, fmt.Errorf("Slack API error calling %s", method)
	}
	return data, nil
}

// WorkspaceURL returns the configured workspace URL.
func (c *Client) WorkspaceURL() string {
	return c.workspaceURL
}

// percentEncodeCookie mimics JavaScript's encodeURIComponent for cookie values.
// Unlike url.QueryEscape, it encodes spaces as %20 (not +) and preserves
// the same set of unreserved characters as encodeURIComponent.
func percentEncodeCookie(s string) string {
	var buf strings.Builder
	for _, b := range []byte(s) {
		if isUnreserved(b) {
			buf.WriteByte(b)
		} else {
			fmt.Fprintf(&buf, "%%%02X", b)
		}
	}
	return buf.String()
}

func isUnreserved(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~' ||
		c == '!' || c == '\'' || c == '(' || c == ')' || c == '*'
}

// setBrowserHeaders adds Chrome-like fingerprint headers to mimic a real browser.
func setBrowserHeaders(req *http.Request) {
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Sec-Ch-Ua", `"Chromium";v="136", "Not-A.Brand";v="24", "Google Chrome";v="136"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-site")
}
