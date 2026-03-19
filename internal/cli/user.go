package cli

import (
	"github.com/spf13/cobra"
	"github.com/cluas/slackctl/internal/auth"
)

func newUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Workspace user directory",
	}
	cmd.AddCommand(
		newUserListCmd(),
		newUserGetCmd(),
	)
	return cmd
}

func newUserListCmd() *cobra.Command {
	var limit int
	var includeBots bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workspace users",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := auth.ResolveClient(workspaceFlag)
			if err != nil {
				return err
			}
			users, err := client.ListUsers(limit, includeBots)
			if err != nil {
				return err
			}
			return printJSON(users)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 100, "Max users")
	cmd.Flags().BoolVar(&includeBots, "include-bots", false, "Include bot users")
	return cmd
}

func newUserGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id-or-handle>",
		Short: "Get a user by ID or handle",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := auth.ResolveClient(workspaceFlag)
			if err != nil {
				return err
			}
			user, err := client.GetUser(args[0])
			if err != nil {
				return err
			}
			return printJSON(user)
		},
	}
}
