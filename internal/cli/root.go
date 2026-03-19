package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

var workspaceFlag string

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "slackctl",
		Short:   "Slack automation CLI for AI agents",
		Version: version,
	}
	root.PersistentFlags().StringVar(&workspaceFlag, "workspace", "",
		"Workspace selector (full URL or unique substring)")

	root.AddCommand(
		newAuthCmd(),
		newMessageCmd(),
		newCanvasCmd(),
		newSearchCmd(),
		newUserCmd(),
		newChannelCmd(),
	)
	return root
}

func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
