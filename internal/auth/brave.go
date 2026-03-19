package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// BraveExtracted holds tokens extracted from Brave browser.
type BraveExtracted struct {
	CookieD string
	Teams   []BrowserTeam
}

func braveTeamsScript() string {
	var tries []string
	for _, expr := range teamJSONPaths {
		tries = append(tries, "try { var v = "+expr+"; if (v && v !== '{}' && v !== 'null') return v; } catch(e) {}")
	}
	return `
    tell application "Brave Browser"
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

func braveCookiesDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "BraveSoftware", "Brave-Browser", "Default", "Cookies")
}

func braveSafeStoragePasswords() []string {
	services := []string{
		"Brave Safe Storage",
		"Brave Browser Safe Storage",
		"Chrome Safe Storage",
		"Chromium Safe Storage",
	}
	var passwords []string
	for _, svc := range services {
		if pw := KeychainGet("", svc); pw != "" {
			passwords = append(passwords, pw)
		}
	}
	return passwords
}

// ExtractFromBrave extracts Slack auth from Brave browser (macOS only).
func ExtractFromBrave() *BraveExtracted {
	if runtime.GOOS != "darwin" {
		return nil
	}
	teamsRaw, err := osascript(braveTeamsScript())
	if err != nil {
		return nil
	}
	teams := parseTeamsJSON(teamsRaw)
	if len(teams) == 0 {
		return nil
	}

	cookieD, err := extractCookieDFromBrave()
	if err != nil || !strings.HasPrefix(cookieD, "xoxd-") {
		return nil
	}
	return &BraveExtracted{CookieD: cookieD, Teams: teams}
}

func extractCookieDFromBrave() (string, error) {
	dbPath := braveCookiesDBPath()
	if _, err := os.Stat(dbPath); err != nil {
		return "", fmt.Errorf("Brave Cookies DB not found: %s", dbPath)
	}

	rows, err := QueryReadonlySQLite(dbPath,
		"select value, encrypted_value from cookies where name = 'd' and host_key like '%slack.com' order by length(encrypted_value) desc")
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("no Slack 'd' cookie found in Brave")
	}

	row := rows[0]
	if v, ok := row["value"].(string); ok && strings.HasPrefix(v, "xoxd-") {
		return v, nil
	}

	encrypted, ok := row["encrypted_value"].([]byte)
	if !ok || len(encrypted) == 0 {
		return "", fmt.Errorf("Brave Slack 'd' cookie had no encrypted_value")
	}

	prefix := string(encrypted[:3])
	data := encrypted
	if prefix == "v10" || prefix == "v11" {
		data = encrypted[3:]
	}
	passwords := braveSafeStoragePasswords()
	xoxdRe := regexp.MustCompile(`xoxd-[A-Za-z0-9%/+_=.-]+`)
	for _, pw := range passwords {
		decrypted, err := DecryptChromiumCookie(data, pw, 1003)
		if err != nil {
			continue
		}
		if match := xoxdRe.FindString(decrypted); match != "" {
			return match, nil
		}
	}
	return "", fmt.Errorf("could not decrypt Slack 'd' cookie from Brave")
}
