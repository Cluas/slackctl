package auth

import (
	"encoding/json"
	"os/exec"
	"runtime"
	"strings"
)

// ChromeExtracted holds tokens extracted from Chrome.
type ChromeExtracted struct {
	CookieD string
	Teams   []BrowserTeam
}

// BrowserTeam represents a team extracted from a browser.
type BrowserTeam struct {
	URL   string `json:"url"`
	Name  string `json:"name,omitempty"`
	Token string `json:"token"`
}

func osascript(script string) (string, error) {
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

var teamJSONPaths = []string{
	"JSON.stringify(JSON.parse(localStorage.localConfig_v2).teams)",
	"JSON.stringify(JSON.parse(localStorage.localConfig_v3).teams)",
	"JSON.stringify(JSON.parse(localStorage.getItem('reduxPersist:localConfig'))?.teams || {})",
	"JSON.stringify(window.boot_data?.teams || {})",
}

func chromeCookieScript() string {
	return `
    tell application "Google Chrome"
      repeat with w in windows
        repeat with t in tabs of w
          if URL of t contains "slack.com" then
            return execute t javascript "document.cookie.split('; ').find(c => c.startsWith('d='))?.split('=')[1] || ''"
          end if
        end repeat
      end repeat
      return ""
    end tell
  `
}

func chromeTeamsScript() string {
	var tries []string
	for _, expr := range teamJSONPaths {
		tries = append(tries, "try { var v = "+expr+"; if (v && v !== '{}' && v !== 'null') return v; } catch(e) {}")
	}
	return `
    tell application "Google Chrome"
      repeat with w in windows
        repeat with t in tabs of w
          if URL of t contains "slack.com" then
            return execute t javascript "(function(){ ` + strings.Join(tries, " ") + ` return '{}'; })()"
          end if
        end repeat
      end repeat
      return "{}"
    end tell
  `
}

// ExtractFromChrome extracts Slack auth from Google Chrome (macOS only).
func ExtractFromChrome() *ChromeExtracted {
	if runtime.GOOS != "darwin" {
		return nil
	}
	cookie, err := osascript(chromeCookieScript())
	if err != nil || !strings.HasPrefix(cookie, "xoxd-") {
		return nil
	}
	teamsRaw, err := osascript(chromeTeamsScript())
	if err != nil {
		return nil
	}
	teams := parseTeamsJSON(teamsRaw)
	if len(teams) == 0 {
		return nil
	}
	return &ChromeExtracted{CookieD: cookie, Teams: teams}
}

func parseTeamsJSON(raw string) []BrowserTeam {
	if raw == "" {
		raw = "{}"
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return nil
	}
	var teams []BrowserTeam
	for _, v := range obj {
		var t BrowserTeam
		if err := json.Unmarshal(v, &t); err != nil {
			continue
		}
		if t.URL != "" && strings.HasPrefix(t.Token, "xoxc-") {
			teams = append(teams, t)
		}
	}
	return teams
}
