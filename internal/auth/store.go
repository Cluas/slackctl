package auth

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/cluas/slackctl/internal/slack"
)

// Workspace represents a stored workspace with credentials.
type Workspace struct {
	WorkspaceURL  string    `json:"workspace_url"`
	WorkspaceName string    `json:"workspace_name,omitempty"`
	Auth          slack.Auth `json:"auth"`
}

// Credentials holds all workspace credentials.
type Credentials struct {
	Version    int         `json:"version"`
	Default    string      `json:"default,omitempty"`
	Workspaces []Workspace `json:"workspaces"`
}

func credentialsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "agent-slack")
}

func credentialsFile() string {
	return filepath.Join(credentialsDir(), "credentials.json")
}

// LoadCredentials reads the stored credentials file.
func LoadCredentials() (*Credentials, error) {
	data, err := os.ReadFile(credentialsFile())
	if err != nil {
		if os.IsNotExist(err) {
			return &Credentials{Version: 1}, nil
		}
		return nil, err
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}
	return &creds, nil
}

// SaveCredentials writes credentials to disk.
func SaveCredentials(creds *Credentials) error {
	dir := credentialsDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(credentialsFile(), data, 0o600)
}

// UpsertWorkspace adds or updates a workspace in credentials.
func UpsertWorkspace(ws Workspace) error {
	creds, err := LoadCredentials()
	if err != nil {
		return err
	}
	found := false
	for i, w := range creds.Workspaces {
		if w.WorkspaceURL == ws.WorkspaceURL {
			creds.Workspaces[i] = ws
			found = true
			break
		}
	}
	if !found {
		creds.Workspaces = append(creds.Workspaces, ws)
	}
	if creds.Default == "" {
		creds.Default = pickBestDefault(creds.Workspaces)
	}
	return SaveCredentials(creds)
}

// UpsertWorkspaces upserts multiple workspaces.
func UpsertWorkspaces(workspaces []Workspace) error {
	creds, err := LoadCredentials()
	if err != nil {
		return err
	}
	for _, ws := range workspaces {
		found := false
		for i, w := range creds.Workspaces {
			if w.WorkspaceURL == ws.WorkspaceURL {
				creds.Workspaces[i] = ws
				found = true
				break
			}
		}
		if !found {
			creds.Workspaces = append(creds.Workspaces, ws)
		}
	}
	if creds.Default == "" && len(creds.Workspaces) > 0 {
		creds.Default = pickBestDefault(creds.Workspaces)
	}
	return SaveCredentials(creds)
}

// RemoveWorkspace removes a workspace from credentials.
func RemoveWorkspace(workspaceURL string) error {
	creds, err := LoadCredentials()
	if err != nil {
		return err
	}
	var filtered []Workspace
	for _, w := range creds.Workspaces {
		if w.WorkspaceURL != workspaceURL {
			filtered = append(filtered, w)
		}
	}
	creds.Workspaces = filtered
	if creds.Default == workspaceURL {
		creds.Default = ""
		if len(creds.Workspaces) > 0 {
			creds.Default = creds.Workspaces[0].WorkspaceURL
		}
	}
	return SaveCredentials(creds)
}

// SetDefaultWorkspace sets the default workspace.
func SetDefaultWorkspace(workspaceURL string) error {
	creds, err := LoadCredentials()
	if err != nil {
		return err
	}
	creds.Default = workspaceURL
	return SaveCredentials(creds)
}

// ResolveDefaultWorkspace returns the default workspace.
func ResolveDefaultWorkspace() (*Workspace, error) {
	creds, err := LoadCredentials()
	if err != nil {
		return nil, err
	}
	if creds.Default == "" && len(creds.Workspaces) > 0 {
		return &creds.Workspaces[0], nil
	}
	for _, w := range creds.Workspaces {
		if w.WorkspaceURL == creds.Default {
			return &w, nil
		}
	}
	return nil, nil
}

// ResolveWorkspaceForURL finds a workspace by URL.
func ResolveWorkspaceForURL(workspaceURL string) (*Workspace, error) {
	creds, err := LoadCredentials()
	if err != nil {
		return nil, err
	}
	for _, w := range creds.Workspaces {
		if w.WorkspaceURL == workspaceURL {
			return &w, nil
		}
	}
	return nil, nil
}

// ResolveWorkspaceSelector does fuzzy matching on workspace URL/name.
func ResolveWorkspaceSelector(selector string) (*Workspace, error) {
	creds, err := LoadCredentials()
	if err != nil {
		return nil, err
	}
	lower := strings.ToLower(selector)
	var matches []Workspace
	for _, w := range creds.Workspaces {
		wURL := strings.ToLower(w.WorkspaceURL)
		wName := strings.ToLower(w.WorkspaceName)
		host := ""
		if u, err := url.Parse(w.WorkspaceURL); err == nil {
			host = strings.ToLower(u.Host)
		}
		hostNoSuffix := strings.TrimSuffix(host, ".slack.com")
		if strings.Contains(wURL, lower) ||
			strings.Contains(wName, lower) ||
			strings.Contains(host, lower) ||
			strings.Contains(hostNoSuffix, lower) {
			matches = append(matches, w)
		}
	}
	if len(matches) == 1 {
		return &matches[0], nil
	}
	if len(matches) > 1 {
		urls := make([]string, len(matches))
		for i, m := range matches {
			urls[i] = m.WorkspaceURL
		}
		return nil, fmt.Errorf("ambiguous selector %q, matches: %s", selector, strings.Join(urls, ", "))
	}
	return nil, nil
}

// NormalizeURL returns canonical workspace URL.
func NormalizeURL(u string) (string, error) {
	return slack.NormalizeURL(u)
}

// IsEnterpriseURL returns true if the URL is an Enterprise Grid org-level URL.
func IsEnterpriseURL(u string) bool {
	return strings.Contains(strings.ToLower(u), ".enterprise.slack.com")
}

// pickBestDefault selects the best workspace URL for default.
// Prefers non-enterprise URLs since enterprise URLs can't serve most APIs.
func pickBestDefault(workspaces []Workspace) string {
	for _, w := range workspaces {
		if !IsEnterpriseURL(w.WorkspaceURL) {
			return w.WorkspaceURL
		}
	}
	if len(workspaces) > 0 {
		return workspaces[0].WorkspaceURL
	}
	return ""
}
