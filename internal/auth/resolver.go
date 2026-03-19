package auth

import (
	"fmt"
	"os"
	"strings"

	"github.com/cluas/slackctl/internal/slack"
)

// ResolveClient resolves a Slack API client from available credential sources.
// Priority: env vars → stored creds (selector match) → default workspace.
func ResolveClient(workspaceSelector string) (*slack.Client, *Workspace, error) {
	selector := strings.TrimSpace(workspaceSelector)

	// 1. Environment variables
	if token := strings.TrimSpace(os.Getenv("SLACK_TOKEN")); token != "" {
		auth := envAuth(token)
		wsURL := os.Getenv("SLACK_WORKSPACE_URL")
		return slack.NewClient(auth, wsURL), &Workspace{WorkspaceURL: wsURL, Auth: auth}, nil
	}

	// 2. Stored credentials with selector
	if selector != "" {
		// Try as full URL first
		if normalized, err := NormalizeURL(selector); err == nil {
			ws, err := ResolveWorkspaceForURL(normalized)
			if err != nil {
				return nil, nil, err
			}
			if ws != nil {
				return slack.NewClient(ws.Auth, ws.WorkspaceURL), ws, nil
			}
		}
		// Try fuzzy match
		ws, err := ResolveWorkspaceSelector(selector)
		if err != nil {
			return nil, nil, err
		}
		if ws != nil {
			return slack.NewClient(ws.Auth, ws.WorkspaceURL), ws, nil
		}
		return nil, nil, fmt.Errorf("no workspace matches selector %q", selector)
	}

	// 3. Default workspace
	ws, err := ResolveDefaultWorkspace()
	if err != nil {
		return nil, nil, err
	}
	if ws != nil {
		return slack.NewClient(ws.Auth, ws.WorkspaceURL), ws, nil
	}

	return nil, nil, fmt.Errorf("no Slack credentials available. Run \"agent-slack auth add\" or set SLACK_TOKEN")
}

func envAuth(token string) slack.Auth {
	if strings.HasPrefix(token, "xoxc-") {
		cookie := strings.TrimSpace(os.Getenv("SLACK_COOKIE_D"))
		if cookie == "" {
			cookie = strings.TrimSpace(os.Getenv("SLACK_COOKIE"))
		}
		return slack.Auth{
			Type:       slack.AuthBrowser,
			XoxcToken:  token,
			XoxdCookie: cookie,
		}
	}
	return slack.Auth{
		Type:  slack.AuthStandard,
		Token: token,
	}
}
