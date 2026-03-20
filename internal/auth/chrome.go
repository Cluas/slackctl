package auth

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// ChromeExtracted holds tokens extracted from Chrome.
type ChromeExtracted struct {
	CookieD string
	Teams   []BrowserTeam
}

// BrowserTeam represents a team extracted from a browser.
type BrowserTeam struct {
	URL              string `json:"url"`
	Name             string `json:"name,omitempty"`
	Token            string `json:"token"`
	EnterpriseID     string `json:"enterprise_id,omitempty"`
	EnterpriseDomain string `json:"enterprise_domain,omitempty"`
}

func debugAuth(format string, args ...any) {
	if os.Getenv("SLACKCTL_DEBUG") != "" {
		log.Printf("[DEBUG] "+format, args...)
	}
}

// ExtractFromChrome extracts Slack auth from Google Chrome.
// Primary: reads Cookie DB + LevelDB directly (no AppleScript needed).
// Fallback: AppleScript on macOS if DB method fails.
func ExtractFromChrome() *ChromeExtracted {
	// 1. Try direct DB read (works on all platforms, no permissions needed)
	if result := extractFromChromeDB(); result != nil {
		return result
	}
	// 2. Fallback: AppleScript (macOS only, needs Automation permission)
	if runtime.GOOS == "darwin" {
		return extractFromChromeAppleScript()
	}
	return nil
}

// --- Direct DB method (primary) ---

func extractFromChromeDB() *ChromeExtracted {
	profileDirs := chromeProfileDirs()
	if len(profileDirs) == 0 {
		debugAuth("Chrome: no profile dirs found")
		return nil
	}

	for _, profileDir := range profileDirs {
		debugAuth("Chrome DB: trying profile %s", profileDir)

		// Extract teams from LevelDB (Local Storage)
		teams := extractTeamsFromChromeLevelDB(profileDir)
		if len(teams) == 0 {
			debugAuth("Chrome DB: no teams in %s", profileDir)
			continue
		}
		debugAuth("Chrome DB: found %d teams", len(teams))

		// Extract cookie from Cookies DB
		cookieD := extractCookieDFromChromeDB(profileDir)
		if cookieD == "" {
			debugAuth("Chrome DB: no cookie_d in %s", profileDir)
			continue
		}
		debugAuth("Chrome DB: got cookie_d (len=%d)", len(cookieD))

		return &ChromeExtracted{CookieD: cookieD, Teams: teams}
	}
	return nil
}

func chromeProfileDirs() []string {
	home, _ := os.UserHomeDir()
	var base string
	switch runtime.GOOS {
	case "darwin":
		base = filepath.Join(home, "Library", "Application Support", "Google", "Chrome")
	case "linux":
		base = filepath.Join(home, ".config", "google-chrome")
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		base = filepath.Join(localAppData, "Google", "Chrome", "User Data")
	}
	if base == "" {
		return nil
	}
	if _, err := os.Stat(base); err != nil {
		return nil
	}

	var dirs []string
	def := filepath.Join(base, "Default")
	if _, err := os.Stat(def); err == nil {
		dirs = append(dirs, def)
	}
	for i := 1; i <= 10; i++ {
		p := filepath.Join(base, "Profile "+strconv.Itoa(i))
		if _, err := os.Stat(p); err == nil {
			dirs = append(dirs, p)
		}
	}
	return dirs
}

func extractTeamsFromChromeLevelDB(profileDir string) []BrowserTeam {
	lsDir := filepath.Join(profileDir, "Local Storage", "leveldb")
	if _, err := os.Stat(lsDir); err != nil {
		return nil
	}

	// Snapshot and read LevelDB
	snapDir, err := snapshotLevelDB(lsDir)
	if err != nil {
		debugAuth("Chrome LevelDB snapshot failed: %v", err)
		return nil
	}
	defer os.RemoveAll(snapDir)

	// Use goleveldb to read
	teams, err := readTeamsFromLevelDBDir(snapDir)
	if err != nil {
		debugAuth("Chrome LevelDB read failed: %v", err)
		return nil
	}
	return teams
}

func extractCookieDFromChromeDB(profileDir string) string {
	cookiesDB := filepath.Join(profileDir, "Cookies")
	if _, err := os.Stat(cookiesDB); err != nil {
		return ""
	}

	rows, err := QueryReadonlySQLite(cookiesDB,
		"SELECT value, encrypted_value FROM cookies WHERE name = 'd' AND host_key LIKE '%slack.com' ORDER BY last_access_utc DESC LIMIT 1")
	if err != nil {
		debugAuth("Chrome cookie DB query error: %v", err)
		return ""
	}
	if len(rows) == 0 {
		debugAuth("Chrome cookie DB: no 'd' cookie rows found")
		return ""
	}

	row := rows[0]
	// Try plain value first
	if v, ok := row["value"].(string); ok && strings.HasPrefix(v, "xoxd-") {
		debugAuth("Chrome cookie DB: found plain xoxd cookie")
		return v
	}

	// Decrypt encrypted_value
	encrypted, ok := row["encrypted_value"].([]byte)
	if !ok || len(encrypted) == 0 {
		debugAuth("Chrome cookie DB: no encrypted_value (type=%T)", row["encrypted_value"])
		return ""
	}
	debugAuth("Chrome cookie DB: encrypted_value len=%d", len(encrypted))

	prefix := ""
	if len(encrypted) >= 3 {
		prefix = string(encrypted[:3])
	}
	data := encrypted
	if prefix == "v10" || prefix == "v11" {
		data = encrypted[3:]
	}

	// Get Chrome Safe Storage passwords
	passwords := GetSafeStoragePasswords(prefix)
	debugAuth("Chrome cookie DB: got %d passwords, prefix=%q", len(passwords), prefix)
	xoxdRe := regexp.MustCompile(`xoxd-[A-Za-z0-9%/+_=.-]+`)
	iterations := 1003
	if runtime.GOOS == "linux" {
		iterations = 1
	}

	for _, pw := range passwords {
		decrypted, err := DecryptChromiumCookie(data, pw, iterations)
		if err != nil {
			continue
		}
		if match := xoxdRe.FindString(decrypted); match != "" {
			return match
		}
	}
	return ""
}


// --- AppleScript fallback (macOS only) ---

var teamJSONPaths = []string{
	"JSON.stringify(JSON.parse(localStorage.localConfig_v2).teams)",
	"JSON.stringify(JSON.parse(localStorage.localConfig_v3).teams)",
	"JSON.stringify(JSON.parse(localStorage.getItem('reduxPersist:localConfig'))?.teams || {})",
	"JSON.stringify(window.boot_data?.teams || {})",
}

func osascript(script string) (string, error) {
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func extractFromChromeAppleScript() *ChromeExtracted {
	cookie, err := osascript(chromeCookieScript())
	if err != nil {
		debugAuth("Chrome AppleScript cookie error: %v", err)
		fmt.Fprintf(os.Stderr, "Note: AppleScript fallback failed. If Chrome DB method also failed,\n"+
			"grant Automation permission in: System Settings → Privacy & Security → Automation\n"+
			"  → allow your terminal to control Google Chrome.\n"+
			"Or use: slackctl auth import-desktop / slackctl auth parse-curl\n")
		return nil
	}
	if !strings.HasPrefix(cookie, "xoxd-") {
		debugAuth("Chrome AppleScript cookie not xoxd: %q", cookie)
		return nil
	}
	teamsRaw, err := osascript(chromeTeamsScript())
	if err != nil {
		debugAuth("Chrome AppleScript teams error: %v", err)
		return nil
	}
	teams := parseTeamsJSON(teamsRaw)
	if len(teams) == 0 {
		return nil
	}
	return &ChromeExtracted{CookieD: cookie, Teams: teams}
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
			EnterpriseDomain string `json:"enterprise_domain"`
		}
		if err := json.Unmarshal(v, &t); err != nil {
			continue
		}
		if t.URL != "" && strings.HasPrefix(t.Token, "xoxc-") {
			teams = append(teams, BrowserTeam{
				URL:              t.URL,
				Name:             t.Name,
				Token:            t.Token,
				EnterpriseID:     t.EnterpriseID,
				EnterpriseDomain: t.EnterpriseDomain,
			})
		}
	}
	return teams
}
