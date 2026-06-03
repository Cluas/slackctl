package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// BraveExtracted holds tokens extracted from Brave browser.
type BraveExtracted struct {
	CookieD string
	Teams   []BrowserTeam
}

// braveMacServices are the macOS Keychain "Safe Storage" service names Brave
// may use, in addition to the Chromium defaults.
var braveMacServices = []string{"Brave Safe Storage", "Brave Browser Safe Storage"}

// braveLinuxApps are the Linux secret-tool "application" attribute values Brave
// may use for its OSCrypt key.
var braveLinuxApps = []string{"brave", "Brave", "Brave-Browser", "chrome", "chromium"}

func bravePasswords(prefix string) []string {
	return GetChromiumSafeStoragePasswords(prefix, braveMacServices, braveLinuxApps)
}

// braveBaseDir returns Brave's "User Data" base directory for the current OS.
func braveBaseDir() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "BraveSoftware", "Brave-Browser")
	case "linux":
		return filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser")
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(localAppData, "BraveSoftware", "Brave-Browser", "User Data")
	}
	return ""
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

// ExtractFromBrave extracts Slack auth from Brave browser.
// Primary: reads Cookie DB + LevelDB directly (works on macOS/Linux/Windows).
// Fallback: AppleScript on macOS if the DB method fails.
func ExtractFromBrave() *BraveExtracted {
	if result := extractFromBraveDB(); result != nil {
		return result
	}
	if runtime.GOOS == "darwin" {
		return extractFromBraveAppleScript()
	}
	return nil
}

func extractFromBraveDB() *BraveExtracted {
	profileDirs := chromiumProfileDirs(braveBaseDir())
	if len(profileDirs) == 0 {
		debugAuth("Brave: no profile dirs found")
		return nil
	}
	for _, profileDir := range profileDirs {
		debugAuth("Brave DB: trying profile %s", profileDir)
		teams := extractTeamsFromChromiumLevelDB(profileDir)
		if len(teams) == 0 {
			debugAuth("Brave DB: no teams in %s", profileDir)
			continue
		}
		cookieD := extractCookieDFromChromiumProfile(profileDir, bravePasswords)
		if cookieD == "" {
			debugAuth("Brave DB: no cookie_d in %s", profileDir)
			continue
		}
		return &BraveExtracted{CookieD: cookieD, Teams: teams}
	}
	return nil
}

func extractFromBraveAppleScript() *BraveExtracted {
	teamsRaw, err := osascript(braveTeamsScript())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot control Brave via AppleScript.\n"+
			"Grant permission in: System Settings → Privacy & Security → Automation\n"+
			"  → allow your terminal to control Brave Browser.\n"+
			"Or use: slackctl auth import-desktop / slackctl auth parse-curl\n")
		return nil
	}
	teams := parseTeamsJSON(teamsRaw)
	if len(teams) == 0 {
		return nil
	}

	var cookieD string
	for _, profileDir := range chromiumProfileDirs(braveBaseDir()) {
		if c := extractCookieDFromChromiumProfile(profileDir, bravePasswords); c != "" {
			cookieD = c
			break
		}
	}
	if !strings.HasPrefix(cookieD, "xoxd-") {
		return nil
	}
	return &BraveExtracted{CookieD: cookieD, Teams: teams}
}
