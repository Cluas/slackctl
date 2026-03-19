package cli

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/cluas/slackctl/internal/auth"
	islack "github.com/cluas/slackctl/internal/slack"
)

var canvasIDRe = regexp.MustCompile(`^F[A-Z0-9]{8,}$`)

func newCanvasCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "canvas",
		Short: "Work with Slack canvases",
	}
	cmd.AddCommand(newCanvasGetCmd())
	return cmd
}

func newCanvasGetCmd() *cobra.Command {
	var maxCharsStr string
	cmd := &cobra.Command{
		Use:   "get <canvas-url-or-id>",
		Short: "Fetch a Slack canvas and convert it to Markdown",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := strings.TrimSpace(args[0])
			var wsURL, canvasID string

			// Try parsing as Slack canvas URL first
			ref, err := islack.ParseCanvasURL(input)
			if err == nil {
				wsURL = ref.WorkspaceURL
				canvasID = ref.CanvasID
			} else {
				// Fallback: treat as bare canvas ID
				if !canvasIDRe.MatchString(input) {
					return fmt.Errorf("unsupported canvas input: %s (expected Slack canvas URL or id like F...)", input)
				}
				canvasID = input
			}

			// Resolve workspace selector (from flag or URL)
			selector := workspaceFlag
			if selector == "" && wsURL != "" {
				selector = wsURL
			}

			client, _, err := auth.ResolveClient(selector)
			if err != nil {
				return err
			}

			maxChars, _ := strconv.Atoi(maxCharsStr)
			if maxCharsStr == "" {
				maxChars = 20000
			}

			result, err := client.FetchCanvas(canvasID, maxChars)
			if err != nil {
				return err
			}

			return printJSON(map[string]any{
				"canvas": result,
			})
		},
	}
	cmd.Flags().StringVar(&maxCharsStr, "max-chars", "20000",
		"Max markdown characters (-1 for unlimited)")
	return cmd
}
