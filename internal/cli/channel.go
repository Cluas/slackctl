package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/cluas/slackctl/internal/auth"
)

func newChannelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel",
		Short: "List conversations, create channels, and manage invites",
	}
	cmd.AddCommand(
		newChannelListCmd(),
		newChannelNewCmd(),
		newChannelInviteCmd(),
		newChannelMarkCmd(),
	)
	return cmd
}

func newChannelListCmd() *cobra.Command {
	var limit int
	var types string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List conversations",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := auth.ResolveClient(workspaceFlag)
			if err != nil {
				return err
			}
			channels, err := client.ListConversations(types, limit, true)
			if err != nil {
				return err
			}
			return printJSON(channels)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 100, "Max channels")
	cmd.Flags().StringVar(&types, "types", "public_channel,private_channel", "Channel types")
	return cmd
}

func newChannelNewCmd() *cobra.Command {
	var isPrivate bool
	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Create a channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := auth.ResolveClient(workspaceFlag)
			if err != nil {
				return err
			}
			id, err := client.CreateChannel(args[0], isPrivate)
			if err != nil {
				return err
			}
			fmt.Printf("Created channel %s\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&isPrivate, "private", false, "Create as private channel")
	return cmd
}

func newChannelInviteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "invite <channel> <user1,user2,...>",
		Short: "Invite users to a channel",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, channelID, err := resolveChannel(args[0])
			if err != nil {
				return err
			}
			userIDs := strings.Split(args[1], ",")
			if err := client.InviteToChannel(channelID, userIDs); err != nil {
				return err
			}
			fmt.Println("Users invited.")
			return nil
		},
	}
}

func newChannelMarkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mark <channel> <timestamp>",
		Short: "Mark channel as read",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, channelID, err := resolveChannel(args[0])
			if err != nil {
				return err
			}
			_, err = client.MarkConversation(channelID, args[1])
			if err != nil {
				return err
			}
			fmt.Println("Marked as read.")
			return nil
		},
	}
}
