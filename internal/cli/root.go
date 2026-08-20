package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

var (
	workspaceFlag   string
	verboseFlag     bool
	attachmentsFlag bool
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "slackctl",
		Short:   "Slack automation CLI for AI agents",
		Version: version,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if verboseFlag {
				os.Setenv("SLACKCTL_DEBUG", "1")
			}
			if attachmentsFlag {
				os.Setenv("SLACKCTL_ATTACHMENTS", "1")
			}
		},
	}
	root.PersistentFlags().StringVar(&workspaceFlag, "workspace", "",
		"Workspace selector (full URL or unique substring)")
	root.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "V", false,
		"Enable debug logging")
	root.PersistentFlags().BoolVar(&attachmentsFlag, "attachments", false,
		"Include message attachment title and text")

	root.AddCommand(
		newAuthCmd(),
		newMessageCmd(),
		newCanvasCmd(),
		newSearchCmd(),
		newUserCmd(),
		newChannelCmd(),
		newFileCmd(),
	)
	return root
}

func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
