package auth

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FirefoxExtracted holds tokens extracted from Firefox.
type FirefoxExtracted struct {
	CookieD string
	Teams   []BrowserTeam
	Source  FirefoxSource
}

type FirefoxSource struct {
	ProfilePath      string
	CookiesPath      string
	LocalStoragePath string
}

// junk characters from Firefox localStorage blob decoding
var urlJunkRe = regexp.MustCompile(`[\x00-\x1f\x7f-\x9f]|\x{FFFD}`)

func cleanURL(u string) string {
	return urlJunkRe.ReplaceAllString(u, "")
}

// ExtractFromFirefox extracts Slack auth from Firefox profiles.
func ExtractFromFirefox(profileSelector string) *FirefoxExtracted {
	candidates := listFirefoxProfiles()
	if profileSelector != "" {
		candidates = filterFirefoxProfiles(candidates, profileSelector)
	}
	for _, profilePath := range candidates {
		teams, lsPath := extractTeamsFromFirefoxProfile(profilePath)
		if len(teams) == 0 {
			continue
		}
		cookieD, cookiePath := extractCookieDFromFirefoxProfile(profilePath)
		if cookieD == "" {
			continue
		}
		return &FirefoxExtracted{
			CookieD: cookieD,
			Teams:   teams,
			Source: FirefoxSource{
				ProfilePath:      profilePath,
				CookiesPath:      cookiePath,
				LocalStoragePath: lsPath,
			},
		}
	}
	return nil
}

func extractTeamsFromFirefoxProfile(profilePath string) ([]BrowserTeam, string) {
	lsDir := filepath.Join(profilePath, "storage", "default", "https+++app.slack.com", "ls")
	dbPath := filepath.Join(lsDir, "data.sqlite")
	if _, err := os.Stat(dbPath); err != nil {
		return nil, ""
	}

	rows, err := QueryReadonlySQLite(dbPath,
		"select key, value from data where key in ('localConfig_v2', 'localConfig_v3') order by key desc")
	if err != nil {
		return nil, ""
	}

	for _, row := range rows {
		val := toStringFromSQLite(row["value"])
		// Try JSON parse
		teams := parseTeamsFromJSON(val)
		if len(teams) > 0 {
			return teams, dbPath
		}
		// Fallback: regex extraction from raw text
		teams = extractTeamsFromRawText(val)
		if len(teams) > 0 {
			return teams, dbPath
		}
	}
	return nil, ""
}

func toStringFromSQLite(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		// Firefox localStorage blobs may have codec marker bytes.
		// Try multiple decodings and pick the longest that contains JSON.
		candidates := []string{
			string(val),
		}
		if len(val) > 0 {
			first := val[0]
			if first == 0x00 || first == 0x01 || first == 0x02 {
				candidates = append(candidates, string(val[1:]))
			}
		}
		// Pick the one that contains a JSON object start
		best := ""
		for _, c := range candidates {
			if strings.Contains(c, "{") && len(c) > len(best) {
				best = c
			}
		}
		if best != "" {
			return best
		}
		if len(candidates) > 0 {
			return candidates[0]
		}
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

func parseTeamsFromJSON(raw string) []BrowserTeam {
	// Try to find JSON object in the string
	start := strings.Index(raw, "{")
	if start == -1 {
		return nil
	}
	// Strip control characters
	cleaned := regexp.MustCompile("[\x00-\x1f]").ReplaceAllString(raw[start:], "")

	var cfg map[string]json.RawMessage
	if err := json.Unmarshal([]byte(cleaned), &cfg); err != nil {
		return nil
	}
	teamsRaw, ok := cfg["teams"]
	if !ok {
		return nil
	}
	var teamsObj map[string]json.RawMessage
	if err := json.Unmarshal(teamsRaw, &teamsObj); err != nil {
		return nil
	}
	var teams []BrowserTeam
	for _, v := range teamsObj {
		var t struct {
			URL   string `json:"url"`
			Name  string `json:"name"`
			Token string `json:"token"`
		}
		if err := json.Unmarshal(v, &t); err != nil {
			continue
		}
		if t.URL != "" && strings.HasPrefix(t.Token, "xoxc-") {
			teams = append(teams, BrowserTeam{
				URL:   cleanURL(t.URL),
				Name:  t.Name,
				Token: t.Token,
			})
		}
	}
	return teams
}

var (
	richTeamRe  = regexp.MustCompile(`"name":"([^"]+)".*?"url":"(https://[^"\s]+slack\.com/)".*?"token":"(xoxc-[^"]+)"`)
	urlTokenRe  = regexp.MustCompile(`"url":"(https://[^"\s]+slack\.com/)"`)
	tokenOnlyRe = regexp.MustCompile(`"token":"(xoxc-[^"]+)"`)
)

func extractTeamsFromRawText(raw string) []BrowserTeam {
	seen := make(map[string]bool)
	var teams []BrowserTeam

	// Try rich pattern first (name + url + token)
	for _, m := range richTeamRe.FindAllStringSubmatch(raw, -1) {
		name, rawURL, token := m[1], m[2], m[3]
		u := cleanURL(rawURL)
		key := u + "::" + token
		if seen[key] {
			continue
		}
		seen[key] = true
		teams = append(teams, BrowserTeam{Name: name, URL: u, Token: token})
	}
	if len(teams) > 0 {
		return teams
	}

	// Fallback: pair URLs and tokens by position
	urls := urlTokenRe.FindAllStringSubmatch(raw, -1)
	tokens := tokenOnlyRe.FindAllStringSubmatch(raw, -1)
	count := len(urls)
	if len(tokens) < count {
		count = len(tokens)
	}
	for i := 0; i < count; i++ {
		u := cleanURL(urls[i][1])
		token := tokens[i][1]
		key := u + "::" + token
		if seen[key] {
			continue
		}
		seen[key] = true
		teams = append(teams, BrowserTeam{URL: u, Token: token})
	}
	return teams
}

func extractCookieDFromFirefoxProfile(profilePath string) (string, string) {
	dbPath := filepath.Join(profilePath, "cookies.sqlite")
	if _, err := os.Stat(dbPath); err != nil {
		return "", ""
	}

	rows, err := QueryReadonlySQLite(dbPath,
		"select value from moz_cookies where host like '%slack.com%' and name='d' order by length(value) desc")
	if err != nil {
		return "", ""
	}
	for _, row := range rows {
		val := fmt.Sprintf("%v", row["value"])
		if strings.HasPrefix(val, "xoxd-") {
			return decodeFirefoxCookie(val), dbPath
		}
	}
	return "", ""
}

func decodeFirefoxCookie(cookie string) string {
	current := cookie
	for i := 0; i < 3; i++ {
		next, err := url.QueryUnescape(current)
		if err != nil || next == current {
			break
		}
		current = next
	}
	return current
}

func listFirefoxProfiles() []string {
	home, _ := os.UserHomeDir()
	var roots []string
	switch {
	case fileExists(filepath.Join(home, "Library", "Application Support", "Firefox")):
		roots = append(roots, filepath.Join(home, "Library", "Application Support", "Firefox"))
	case fileExists(filepath.Join(home, ".mozilla", "firefox")):
		roots = append(roots, filepath.Join(home, ".mozilla", "firefox"))
	}

	var profiles []string
	for _, root := range roots {
		entries, err := os.ReadDir(filepath.Join(root, "Profiles"))
		if err != nil {
			// Try root directly (Linux style)
			entries, err = os.ReadDir(root)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if e.IsDir() && strings.Contains(e.Name(), ".") {
					profiles = append(profiles, filepath.Join(root, e.Name()))
				}
			}
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				profiles = append(profiles, filepath.Join(root, "Profiles", e.Name()))
			}
		}
	}
	return profiles
}

func filterFirefoxProfiles(profiles []string, selector string) []string {
	lower := strings.ToLower(selector)
	var filtered []string
	for _, p := range profiles {
		base := strings.ToLower(filepath.Base(p))
		if strings.Contains(base, lower) || strings.Contains(strings.ToLower(p), lower) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
