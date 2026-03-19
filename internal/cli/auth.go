package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"
	"github.com/cluas/slackctl/internal/auth"
	"github.com/cluas/slackctl/internal/slack"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Slack authentication",
	}
	cmd.AddCommand(
		newAuthWhoamiCmd(),
		newAuthTestCmd(),
		newAuthAddCmd(),
		newAuthImportDesktopCmd(),
		newAuthImportChromeCmd(),
		newAuthImportBraveCmd(),
		newAuthImportFirefoxCmd(),
		newAuthParseCurlCmd(),
		newAuthSetDefaultCmd(),
		newAuthRemoveCmd(),
	)
	return cmd
}

func newAuthWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show configured workspaces and token sources",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, err := auth.LoadCredentials()
			if err != nil {
				return err
			}
			type sanitized struct {
				WorkspaceURL  string `json:"workspace_url"`
				WorkspaceName string `json:"workspace_name,omitempty"`
				AuthType      string `json:"auth_type"`
				Token         string `json:"token"`
			}
			var out struct {
				Default    string      `json:"default,omitempty"`
				Workspaces []sanitized `json:"workspaces"`
			}
			out.Default = creds.Default
			for _, w := range creds.Workspaces {
				s := sanitized{
					WorkspaceURL:  w.WorkspaceURL,
					WorkspaceName: w.WorkspaceName,
					AuthType:      string(w.Auth.Type),
				}
				if w.Auth.Type == slack.AuthStandard {
					s.Token = redact(w.Auth.Token)
				} else {
					s.Token = redact(w.Auth.XoxcToken)
				}
				out.Workspaces = append(out.Workspaces, s)
			}
			return printJSON(out)
		},
	}
}

func newAuthTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test",
		Short: "Verify credentials (calls Slack auth.test)",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := auth.ResolveClient(workspaceFlag)
			if err != nil {
				return err
			}
			resp, err := client.API("auth.test", nil)
			if err != nil {
				return err
			}
			return printJSON(resp)
		},
	}
}

func newAuthAddCmd() *cobra.Command {
	var wsURL, token, xoxc, xoxd string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add credentials (standard token or browser xoxc/xoxd)",
		RunE: func(cmd *cobra.Command, args []string) error {
			normalized, err := auth.NormalizeURL(wsURL)
			if err != nil {
				return fmt.Errorf("invalid workspace URL: %w", err)
			}
			ws := auth.Workspace{WorkspaceURL: normalized}
			if token != "" {
				ws.Auth = slack.Auth{Type: slack.AuthStandard, Token: token}
			} else if xoxc != "" && xoxd != "" {
				ws.Auth = slack.Auth{Type: slack.AuthBrowser, XoxcToken: xoxc, XoxdCookie: xoxd}
			} else {
				return fmt.Errorf("provide either --token or both --xoxc and --xoxd")
			}
			if err := auth.UpsertWorkspace(ws); err != nil {
				return err
			}
			fmt.Println("Saved credentials.")
			return nil
		},
	}
	cmd.Flags().StringVar(&wsURL, "workspace-url", "", "Workspace URL (required)")
	cmd.Flags().StringVar(&token, "token", "", "Standard Slack token (xoxb/xoxp)")
	cmd.Flags().StringVar(&xoxc, "xoxc", "", "Browser token (xoxc-...)")
	cmd.Flags().StringVar(&xoxd, "xoxd", "", "Browser cookie d (xoxd-...)")
	_ = cmd.MarkFlagRequired("workspace-url")
	return cmd
}

func newAuthSetDefaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-default <workspace-url>",
		Short: "Set the default workspace URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := auth.SetDefaultWorkspace(args[0]); err != nil {
				return err
			}
			fmt.Println("Default workspace updated.")
			return nil
		},
	}
}

func newAuthRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <workspace-url>",
		Short: "Remove a workspace from local config",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := auth.RemoveWorkspace(args[0]); err != nil {
				return err
			}
			fmt.Println("Removed workspace.")
			return nil
		},
	}
}

func newAuthImportDesktopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import-desktop",
		Short: "Import xoxc token(s) + d cookie from Slack Desktop",
		RunE: func(cmd *cobra.Command, args []string) error {
			extracted, err := auth.ExtractFromDesktop()
			if err != nil {
				return err
			}
			return saveExtractedTeams(extracted.CookieD, extracted.Teams, "Desktop")
		},
	}
}

func newAuthImportChromeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import-chrome",
		Short: "Import xoxc/xoxd from Google Chrome (macOS)",
		RunE: func(cmd *cobra.Command, args []string) error {
			extracted := auth.ExtractFromChrome()
			if extracted == nil {
				return fmt.Errorf("could not extract tokens from Chrome")
			}
			return saveExtractedTeams(extracted.CookieD, extracted.Teams, "Chrome")
		},
	}
}

func newAuthImportBraveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import-brave",
		Short: "Import xoxc/xoxd from Brave Browser (macOS)",
		RunE: func(cmd *cobra.Command, args []string) error {
			extracted := auth.ExtractFromBrave()
			if extracted == nil {
				return fmt.Errorf("could not extract tokens from Brave")
			}
			return saveExtractedTeams(extracted.CookieD, extracted.Teams, "Brave")
		},
	}
}

func newAuthImportFirefoxCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import-firefox",
		Short: "Import xoxc/xoxd from Firefox",
		RunE: func(cmd *cobra.Command, args []string) error {
			extracted := auth.ExtractFromFirefox("")
			if extracted == nil {
				return fmt.Errorf("could not extract tokens from Firefox")
			}
			return saveExtractedTeams(extracted.CookieD, extracted.Teams, "Firefox")
		},
	}
}

func newAuthParseCurlCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "parse-curl",
		Short: "Extract xoxc/xoxd from a cURL command on stdin",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile("/dev/stdin")
			if err != nil {
				return err
			}
			parsed, err := auth.ParseSlackCurlCommand(string(data))
			if err != nil {
				return err
			}
			normalized, err := auth.NormalizeURL(parsed.WorkspaceURL)
			if err != nil {
				return err
			}
			if err := auth.UpsertWorkspace(auth.Workspace{
				WorkspaceURL: normalized,
				Auth: slack.Auth{
					Type:       slack.AuthBrowser,
					XoxcToken:  parsed.XoxcToken,
					XoxdCookie: parsed.XoxdCookie,
				},
			}); err != nil {
				return err
			}
			fmt.Printf("Imported tokens for %s.\n", normalized)
			return nil
		},
	}
}

func saveExtractedTeams(cookieD string, teams []auth.BrowserTeam, source string) error {
	// Ensure cookie is stored in decoded form; percentEncodeCookie re-encodes on send.
	cookieD = decodeRepeatedly(cookieD)
	var workspaces []auth.Workspace
	for _, t := range teams {
		normalized, err := auth.NormalizeURL(t.URL)
		if err != nil {
			normalized = t.URL
		}
		workspaces = append(workspaces, auth.Workspace{
			WorkspaceURL:  normalized,
			WorkspaceName: t.Name,
			Auth: slack.Auth{
				Type:       slack.AuthBrowser,
				XoxcToken:  t.Token,
				XoxdCookie: cookieD,
			},
		})
	}
	if err := auth.UpsertWorkspaces(workspaces); err != nil {
		return err
	}
	fmt.Printf("Imported %d workspace token(s) from %s.\n", len(teams), source)
	return nil
}

func printJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func redact(s string) string {
	if len(s) <= 10 {
		return "***"
	}
	return s[:6] + "…" + s[len(s)-4:]
}

// decodeRepeatedly applies percent-decoding until the string stabilizes.
// Uses PathUnescape (not QueryUnescape) to preserve '+' as literal '+'.
func decodeRepeatedly(s string) string {
	current := s
	for range 3 {
		next, err := url.PathUnescape(current)
		if err != nil || next == current {
			break
		}
		current = next
	}
	return current
}
