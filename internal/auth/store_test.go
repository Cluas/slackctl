package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cluas/slackctl/internal/slack"
)

func setupTestCredentials(t *testing.T) (cleanup func()) {
	t.Helper()
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	// Create the config dir
	os.MkdirAll(filepath.Join(tmpDir, ".config", "agent-slack"), 0o700)
	return func() {
		os.Setenv("HOME", origHome)
	}
}

func TestUpsertAndLoadWorkspace(t *testing.T) {
	cleanup := setupTestCredentials(t)
	defer cleanup()

	ws := Workspace{
		WorkspaceURL:  "https://myteam.slack.com",
		WorkspaceName: "My Team",
		Auth: slack.Auth{
			Type:  slack.AuthStandard,
			Token: "xoxb-test-token",
		},
	}

	if err := UpsertWorkspace(ws); err != nil {
		t.Fatalf("UpsertWorkspace: %v", err)
	}

	creds, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if len(creds.Workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(creds.Workspaces))
	}
	if creds.Workspaces[0].WorkspaceURL != "https://myteam.slack.com" {
		t.Errorf("unexpected URL: %s", creds.Workspaces[0].WorkspaceURL)
	}
	if creds.Default != "https://myteam.slack.com" {
		t.Errorf("expected default to be set, got %q", creds.Default)
	}
}

func TestUpsertWorkspace_Update(t *testing.T) {
	cleanup := setupTestCredentials(t)
	defer cleanup()

	ws1 := Workspace{
		WorkspaceURL: "https://team.slack.com",
		Auth:         slack.Auth{Type: slack.AuthStandard, Token: "old-token"},
	}
	ws2 := Workspace{
		WorkspaceURL: "https://team.slack.com",
		Auth:         slack.Auth{Type: slack.AuthStandard, Token: "new-token"},
	}

	_ = UpsertWorkspace(ws1)
	_ = UpsertWorkspace(ws2)

	creds, _ := LoadCredentials()
	if len(creds.Workspaces) != 1 {
		t.Fatalf("expected 1 workspace after upsert, got %d", len(creds.Workspaces))
	}
	if creds.Workspaces[0].Auth.Token != "new-token" {
		t.Errorf("token not updated: %s", creds.Workspaces[0].Auth.Token)
	}
}

func TestRemoveWorkspace(t *testing.T) {
	cleanup := setupTestCredentials(t)
	defer cleanup()

	_ = UpsertWorkspace(Workspace{
		WorkspaceURL: "https://a.slack.com",
		Auth:         slack.Auth{Type: slack.AuthStandard, Token: "t1"},
	})
	_ = UpsertWorkspace(Workspace{
		WorkspaceURL: "https://b.slack.com",
		Auth:         slack.Auth{Type: slack.AuthStandard, Token: "t2"},
	})

	if err := RemoveWorkspace("https://a.slack.com"); err != nil {
		t.Fatalf("RemoveWorkspace: %v", err)
	}

	creds, _ := LoadCredentials()
	if len(creds.Workspaces) != 1 {
		t.Fatalf("expected 1 workspace after remove, got %d", len(creds.Workspaces))
	}
	if creds.Workspaces[0].WorkspaceURL != "https://b.slack.com" {
		t.Errorf("wrong workspace remaining: %s", creds.Workspaces[0].WorkspaceURL)
	}
}

func TestSetDefaultWorkspace(t *testing.T) {
	cleanup := setupTestCredentials(t)
	defer cleanup()

	_ = UpsertWorkspace(Workspace{
		WorkspaceURL: "https://a.slack.com",
		Auth:         slack.Auth{Type: slack.AuthStandard, Token: "t1"},
	})
	_ = UpsertWorkspace(Workspace{
		WorkspaceURL: "https://b.slack.com",
		Auth:         slack.Auth{Type: slack.AuthStandard, Token: "t2"},
	})

	if err := SetDefaultWorkspace("https://b.slack.com"); err != nil {
		t.Fatalf("SetDefaultWorkspace: %v", err)
	}

	creds, _ := LoadCredentials()
	if creds.Default != "https://b.slack.com" {
		t.Errorf("default = %q, want https://b.slack.com", creds.Default)
	}
}

func TestResolveWorkspaceSelector(t *testing.T) {
	cleanup := setupTestCredentials(t)
	defer cleanup()

	_ = UpsertWorkspace(Workspace{
		WorkspaceURL:  "https://alpha.slack.com",
		WorkspaceName: "Alpha Corp",
		Auth:          slack.Auth{Type: slack.AuthStandard, Token: "t1"},
	})
	_ = UpsertWorkspace(Workspace{
		WorkspaceURL:  "https://beta.slack.com",
		WorkspaceName: "Beta Inc",
		Auth:          slack.Auth{Type: slack.AuthStandard, Token: "t2"},
	})

	tests := []struct {
		selector string
		wantURL  string
		wantErr  bool
	}{
		{"alpha", "https://alpha.slack.com", false},
		{"beta", "https://beta.slack.com", false},
		{"Beta Inc", "https://beta.slack.com", false},
		{"nonexistent", "", false}, // nil result, no error
		// Both match ".slack.com" but that's fine since each URL is unique enough
	}
	for _, tt := range tests {
		t.Run(tt.selector, func(t *testing.T) {
			ws, err := ResolveWorkspaceSelector(tt.selector)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantURL == "" {
				if ws != nil {
					t.Errorf("expected nil, got %v", ws)
				}
				return
			}
			if ws == nil {
				t.Fatal("expected workspace, got nil")
			}
			if ws.WorkspaceURL != tt.wantURL {
				t.Errorf("got %q, want %q", ws.WorkspaceURL, tt.wantURL)
			}
		})
	}
}

func TestLoadCredentials_Empty(t *testing.T) {
	cleanup := setupTestCredentials(t)
	defer cleanup()

	creds, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials on empty: %v", err)
	}
	if creds.Version != 1 {
		t.Errorf("version = %d, want 1", creds.Version)
	}
	if len(creds.Workspaces) != 0 {
		t.Errorf("expected 0 workspaces, got %d", len(creds.Workspaces))
	}
}
