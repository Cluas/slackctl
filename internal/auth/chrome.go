package auth

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
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
	URL           string `json:"url"`
	Name          string `json:"name,omitempty"`
	Token         string `json:"token"`
	EnterpriseID  string `json:"enterprise_id,omitempty"`
	EnterpriseDomain string `json:"enterprise_domain,omitempty"`
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
func debugAuth(format string, args ...any) {
	if os.Getenv("SLACKCTL_DEBUG") != "" {
		log.Printf("[DEBUG] "+format, args...)
	}
}

func ExtractFromChrome() *ChromeExtracted {
	if runtime.GOOS != "darwin" {
		debugAuth("Chrome: not macOS, skipping")
		return nil
	}
	cookie, err := osascript(chromeCookieScript())
	if err != nil {
		debugAuth("Chrome cookie script error: %v", err)
		fmt.Fprintf(os.Stderr, "Error: cannot control Chrome via AppleScript.\n"+
			"Grant permission in: System Settings → Privacy & Security → Automation\n"+
			"  → allow your terminal (Terminal/iTerm/etc.) to control Google Chrome.\n"+
			"Or use: slackctl auth import-desktop / slackctl auth parse-curl\n")
		return nil
	}
	if !strings.HasPrefix(cookie, "xoxd-") {
		debugAuth("Chrome cookie not xoxd: %q", cookie)
		return nil
	}
	debugAuth("Chrome cookie OK (len=%d)", len(cookie))
	teamsRaw, err := osascript(chromeTeamsScript())
	if err != nil {
		debugAuth("Chrome teams script error: %v", err)
		return nil
	}
	debugAuth("Chrome teams raw: %s", teamsRaw[:min(len(teamsRaw), 200)])
	teams := parseTeamsJSON(teamsRaw)
	if len(teams) == 0 {
		debugAuth("Chrome: no teams parsed")
		return nil
	}
	debugAuth("Chrome: found %d teams", len(teams))
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
		var t struct {
			URL              string `json:"url"`
			Name             string `json:"name"`
			Token            string `json:"token"`
			EnterpriseID     string `json:"enterprise_id"`
			EnterpriseName   string `json:"enterprise_name"`
			EnterpriseDomain string `json:"enterprise_domain"`
			Domain           string `json:"domain"`
		}
		if err := json.Unmarshal(v, &t); err != nil {
			continue
		}
		if t.URL != "" && strings.HasPrefix(t.Token, "xoxc-") {
			bt := BrowserTeam{
				URL:           t.URL,
				Name:          t.Name,
				Token:         t.Token,
				EnterpriseID:  t.EnterpriseID,
			}
			// For Enterprise Grid, derive enterprise domain from the org team entry
			// or from the enterprise_domain field if available
			if t.EnterpriseDomain != "" {
				bt.EnterpriseDomain = t.EnterpriseDomain
			}
			teams = append(teams, bt)
		}
	}
	return teams
}
