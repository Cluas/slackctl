package slack

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Header profiles per auth source.
// Desktop uses Electron UA; browsers use Chrome UA.
var headerProfiles = map[AuthSource]headerProfile{
	SourceDesktop: {
		UserAgent:    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Slack/4.49.75 Chrome/146.0.7680.72 Electron/41.0.1 Safari/537.36",
		SecChUa:      `"Chromium";v="146", "Not)A;Brand";v="99", "Electron";v="41"`,
		SecChPlatform: `"macOS"`,
	},
	SourceChrome: {
		UserAgent:    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36",
		SecChUa:      `"Chromium";v="136", "Not-A.Brand";v="24", "Google Chrome";v="136"`,
		SecChPlatform: `"macOS"`,
	},
	SourceFirefox: {
		UserAgent:    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:138.0) Gecko/20100101 Firefox/138.0",
		SecChUa:      "", // Firefox doesn't send sec-ch-ua
		SecChPlatform: "",
	},
	SourceBrave: {
		UserAgent:    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36",
		SecChUa:      `"Chromium";v="136", "Not-A.Brand";v="24", "Brave";v="136"`,
		SecChPlatform: `"macOS"`,
	},
}

// defaultProfile is used for manual/env/unknown sources.
var defaultProfile = headerProfiles[SourceChrome]

type headerProfile struct {
	UserAgent     string
	SecChUa       string
	SecChPlatform string
}

func isDebug() bool {
	return os.Getenv("SLACKCTL_DEBUG") != ""
}

func debugLog(format string, args ...any) {
	if isDebug() {
		log.Printf("[DEBUG] "+format, args...)
	}
}

func debugRequest(req *http.Request) {
	if !isDebug() {
		return
	}
	dump, _ := httputil.DumpRequestOut(req, false)
	log.Printf("[DEBUG] >>> REQUEST:\n%s", dump)
}

func debugResponse(resp *http.Response, body []byte) {
	if !isDebug() {
		return
	}
	bodyPreview := string(body)
	if len(bodyPreview) > 500 {
		bodyPreview = bodyPreview[:500] + "..."
	}
	log.Printf("[DEBUG] <<< RESPONSE: %d\n%s", resp.StatusCode, bodyPreview)
}

// Client wraps Slack Web API calls, supporting both standard and browser auth.
type Client struct {
	auth               Auth
	workspaceURL       string
	httpClient         *http.Client
	enterpriseResolved bool // true after resolving enterprise URL to workspace URL
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
	c.setSourceHeaders(req)

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
	c.setSourceHeaders(req)

	debugLog("API %s → %s (source=%s)", method, apiURL, c.auth.Source)
	debugRequest(req)

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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response for %s: %w", method, err)
	}
	debugResponse(resp, body)

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to decode response for %s: %w", method, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Slack HTTP %d calling %s", resp.StatusCode, method)
	}
	if ok, _ := data["ok"].(bool); !ok {
		errStr, _ := data["error"].(string)

		// Enterprise Grid: enterprise URL can't serve most APIs.
		if errStr == "enterprise_is_restricted" && !c.enterpriseResolved && method != "auth.test" {
			if resolved := c.resolveEnterpriseWorkspaceURL(); resolved != "" {
				debugLog("Enterprise resolve: %s → %s", c.workspaceURL, resolved)
				c.workspaceURL = resolved
				c.enterpriseResolved = true
				return c.browserAPI(method, params, 0)
			}
		}

		if errStr != "" {
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

// resolveEnterpriseWorkspaceURL discovers a usable workspace URL for Enterprise Grid.
// Tries auth.teams.list first (returns all workspaces), then falls back to auth.test.
func (c *Client) resolveEnterpriseWorkspaceURL() string {
	// Try auth.teams.list to get workspace-level URLs
	c.enterpriseResolved = true // prevent recursion
	resp, err := c.browserAPI("auth.teams.list", nil, 0)
	if err == nil {
		teams := getArray(resp, "teams")
		for _, t := range teams {
			rec := toRecord(t)
			domain := stringVal(rec, "domain")
			if domain != "" && !strings.Contains(domain, ".enterprise.") {
				return "https://" + domain + ".slack.com"
			}
		}
	}
	// Fallback: auth.test url field
	resp, err = c.browserAPI("auth.test", nil, 0)
	if err == nil {
		if wsURL, ok := resp["url"].(string); ok && wsURL != "" {
			resolved := strings.TrimRight(wsURL, "/")
			if !strings.Contains(resolved, ".enterprise.") {
				return resolved
			}
		}
	}
	c.enterpriseResolved = false
	return ""
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

// profile returns the header profile matching the auth source.
func (c *Client) profile() headerProfile {
	if p, ok := headerProfiles[c.auth.Source]; ok {
		return p
	}
	return defaultProfile
}

// setSourceHeaders sets User-Agent and fingerprint headers based on auth source.
func (c *Client) setSourceHeaders(req *http.Request) {
	p := c.profile()
	req.Header.Set("User-Agent", p.UserAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-site")

	if p.SecChUa != "" {
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Sec-Ch-Ua", p.SecChUa)
		req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
		req.Header.Set("Sec-Ch-Ua-Platform", p.SecChPlatform)
	} else {
		// Firefox style
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7")
	}
}
