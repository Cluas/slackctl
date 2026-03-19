package cli

import (
	"github.com/spf13/cobra"
	"github.com/cluas/slackctl/internal/auth"
)

func newSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search Slack messages and files",
	}
	cmd.AddCommand(
		newSearchMessagesCmd(),
		newSearchFilesCmd(),
	)
	return cmd
}

func newSearchMessagesCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "messages <query>",
		Short: "Search messages",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := auth.ResolveClient(workspaceFlag)
			if err != nil {
				return err
			}
			result, err := client.SearchMessages(args[0], limit)
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	return cmd
}

func newSearchFilesCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "files <query>",
		Short: "Search files",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := auth.ResolveClient(workspaceFlag)
			if err != nil {
				return err
			}
			result, err := client.SearchFiles(args[0], limit)
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	return cmd
}
