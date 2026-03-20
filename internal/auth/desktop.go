package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

// DesktopExtracted holds tokens extracted from Slack Desktop.
type DesktopExtracted struct {
	CookieD string
	Teams   []BrowserTeam
	Source  DesktopSource
}

type DesktopSource struct {
	LevelDBPath string
	CookiesPath string
}

// ExtractFromDesktop extracts Slack auth from Slack Desktop application.
func ExtractFromDesktop() (*DesktopExtracted, error) {
	allPaths := getAllSlackPaths()
	if len(allPaths) == 0 {
		return nil, fmt.Errorf("Slack Desktop data not found")
	}

	var errors []string
	for _, p := range allPaths {
		teams, err := extractTeamsFromLevelDB(p.leveldbDir)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", p.baseDir, err))
			continue
		}
		cookieD, err := extractCookieDFromCookiesDB(p.cookiesDB, p.baseDir)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", p.baseDir, err))
			continue
		}
		return &DesktopExtracted{
			CookieD: cookieD,
			Teams:   teams,
			Source:  DesktopSource{LevelDBPath: p.leveldbDir, CookiesPath: p.cookiesDB},
		}, nil
	}
	return nil, fmt.Errorf("could not extract from any location:\n  - %s", strings.Join(errors, "\n  - "))
}

type slackPath struct {
	leveldbDir string
	cookiesDB  string
	baseDir    string
}

func getAllSlackPaths() []slackPath {
	home, _ := os.UserHomeDir()
	var candidates []string

	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			filepath.Join(home, "Library", "Application Support", "Slack"),
			filepath.Join(home, "Library", "Containers", "com.tinyspeck.slackmacgap", "Data", "Library", "Application Support", "Slack"),
		}
	case "linux":
		candidates = []string{
			filepath.Join(home, ".var", "app", "com.slack.Slack", "config", "Slack"),
			filepath.Join(home, ".config", "Slack"),
		}
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		candidates = []string{filepath.Join(appData, "Slack")}
		// Windows Store path
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		pkgBase := filepath.Join(localAppData, "Packages")
		if entries, err := os.ReadDir(pkgBase); err == nil {
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), "com.tinyspeck.slackdesktop_") {
					candidates = append(candidates, filepath.Join(pkgBase, e.Name(), "LocalCache", "Roaming", "Slack"))
				}
			}
		}
	}

	var results []slackPath
	for _, dir := range candidates {
		ldbDir := filepath.Join(dir, "Local Storage", "leveldb")
		if _, err := os.Stat(ldbDir); err != nil {
			continue
		}
		cookieCandidates := []string{
			filepath.Join(dir, "Network", "Cookies"),
			filepath.Join(dir, "Cookies"),
		}
		cookiesDB := cookieCandidates[0]
		for _, c := range cookieCandidates {
			if _, err := os.Stat(c); err == nil {
				cookiesDB = c
				break
			}
		}
		results = append(results, slackPath{leveldbDir: ldbDir, cookiesDB: cookiesDB, baseDir: dir})
	}
	return results
}

func extractTeamsFromLevelDB(ldbDir string) ([]BrowserTeam, error) {
	// Snapshot the LevelDB directory to avoid lock contention with running Slack
	snapDir, err := snapshotLevelDB(ldbDir)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(snapDir)
	return readTeamsFromLevelDBDir(snapDir)
}

// readTeamsFromLevelDBDir reads teams from an already-accessible LevelDB directory.
func readTeamsFromLevelDBDir(ldbDir string) ([]BrowserTeam, error) {
	db, err := leveldb.OpenFile(ldbDir, &opt.Options{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("failed to open LevelDB: %w", err)
	}
	defer db.Close()

	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	var configData []byte
	for iter.Next() {
		key := string(iter.Key())
		if strings.Contains(key, "localConfig_v") {
			val := iter.Value()
			if len(val) > len(configData) {
				configData = make([]byte, len(val))
				copy(configData, val)
			}
		}
	}

	if len(configData) == 0 {
		return nil, fmt.Errorf("no localConfig found in LevelDB")
	}

	teams, err := parseDesktopLocalConfig(configData)
	if err != nil {
		return nil, err
	}
	if len(teams) == 0 {
		return nil, fmt.Errorf("no xoxc tokens found in Slack localConfig")
	}
	return teams, nil
}

func snapshotLevelDB(srcDir string) (string, error) {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".config", "agent-slack", "cache", "leveldb-snapshots")
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}
	dest, err := os.MkdirTemp(base, "snap-*")
	if err != nil {
		return "", err
	}

	// Try macOS clonefile first (instant, COW)
	if runtime.GOOS == "darwin" {
		if err := exec.Command("cp", "-cR", srcDir+"/.", dest).Run(); err == nil {
			os.Remove(filepath.Join(dest, "LOCK"))
			return dest, nil
		}
	}

	// Fallback: regular copy
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.Name() == "LOCK" {
			continue
		}
		src := filepath.Join(srcDir, e.Name())
		dst := filepath.Join(dest, e.Name())
		data, err := os.ReadFile(src)
		if err != nil {
			continue
		}
		_ = os.WriteFile(dst, data, 0o600)
	}
	return dest, nil
}

func parseDesktopLocalConfig(raw []byte) ([]BrowserTeam, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("localConfig is empty")
	}

	// Strip codec marker byte if present
	data := raw
	if len(data) > 0 {
		first := data[0]
		if first == 0x00 || first == 0x01 || first == 0x02 {
			data = data[1:]
		}
	}

	// Determine encoding: if high proportion of NUL bytes, likely UTF-16LE
	nulCount := 0
	for _, b := range data {
		if b == 0 {
			nulCount++
		}
	}

	type encoding int
	const (
		encUTF8 encoding = iota
		encUTF16LE
	)
	encodings := []encoding{encUTF8, encUTF16LE}
	if nulCount > len(data)/4 {
		encodings = []encoding{encUTF16LE, encUTF8}
	}

	for _, enc := range encodings {
		var text string
		switch enc {
		case encUTF8:
			text = string(data)
		case encUTF16LE:
			text = decodeUTF16LE(data)
		}

		teams := tryParseConfigJSON(text)
		if len(teams) > 0 {
			return teams, nil
		}
	}
	return nil, fmt.Errorf("localConfig not parseable")
}

func tryParseConfigJSON(text string) []BrowserTeam {
	// Try direct parse
	if teams := extractTeamsFromConfigText(text); len(teams) > 0 {
		return teams
	}
	// Try extracting JSON substring
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		if teams := extractTeamsFromConfigText(text[start : end+1]); len(teams) > 0 {
			return teams
		}
	}
	return nil
}

func extractTeamsFromConfigText(text string) []BrowserTeam {
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &cfg); err != nil {
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

func extractCookieDFromCookiesDB(cookiesPath, slackDataDir string) (string, error) {
	if _, err := os.Stat(cookiesPath); err != nil {
		return "", fmt.Errorf("Slack Cookies DB not found: %s", cookiesPath)
	}

	rows, err := QueryReadonlySQLite(cookiesPath,
		"select host_key, name, value, encrypted_value from cookies where name = 'd' and host_key like '%slack.com' order by length(encrypted_value) desc")
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("no Slack 'd' cookie found")
	}

	row := rows[0]
	if v, ok := row["value"].(string); ok && strings.HasPrefix(v, "xoxd-") {
		return v, nil
	}

	encrypted, ok := row["encrypted_value"].([]byte)
	if !ok || len(encrypted) == 0 {
		return "", fmt.Errorf("Slack 'd' cookie had no encrypted_value")
	}

	prefix := string(encrypted[:3])

	// Windows: DPAPI
	if runtime.GOOS == "windows" && (prefix == "v10" || prefix == "v11") {
		return DecryptCookieWindows(encrypted, slackDataDir)
	}

	// macOS/Linux: password-based AES-128-CBC
	data := encrypted
	if prefix == "v10" || prefix == "v11" {
		data = encrypted[3:]
	}

	iterations := 1003
	if runtime.GOOS == "linux" {
		iterations = 1
	}
	passwords := GetSafeStoragePasswords(prefix)
	xoxdRe := regexp.MustCompile(`xoxd-[A-Za-z0-9%/+_=.-]+`)

	for _, pw := range passwords {
		decrypted, err := DecryptChromiumCookie(data, pw, iterations)
		if err != nil {
			continue
		}
		if match := xoxdRe.FindString(decrypted); match != "" {
			return match, nil
		}
	}
	return "", fmt.Errorf("could not decrypt Slack 'd' cookie")
}

func decodeUTF16LE(data []byte) string {
	if len(data)%2 != 0 {
		data = data[:len(data)-1]
	}
	u16s := make([]uint16, len(data)/2)
	for i := range u16s {
		u16s[i] = uint16(data[2*i]) | uint16(data[2*i+1])<<8
	}
	runes := utf16.Decode(u16s)
	var buf strings.Builder
	for _, r := range runes {
		if r == utf8.RuneError {
			continue
		}
		buf.WriteRune(r)
	}
	return buf.String()
}
