package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/cluas/slackctl/internal/auth"
	islack "github.com/cluas/slackctl/internal/slack"
)

func newMessageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "message",
		Short: "Read/write Slack messages",
	}
	cmd.AddCommand(
		newMessageGetCmd(),
		newMessageListCmd(),
		newMessageSendCmd(),
		newMessageEditCmd(),
		newMessageDeleteCmd(),
		newMessageReactCmd(),
	)
	return cmd
}

func newMessageGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <url-or-channel> [timestamp]",
		Short: "Fetch a single message",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, channelID, ts, err := resolveMessageTarget(args)
			if err != nil {
				return err
			}
			msg, err := client.FetchMessage(channelID, ts)
			if err != nil {
				return err
			}
			return printJSON(msg)
		},
	}
}

func newMessageListCmd() *cobra.Command {
	var threadTS string
	var limit int
	cmd := &cobra.Command{
		Use:   "list <url-or-channel> [timestamp]",
		Short: "List channel history or thread replies",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, channelID, _, err := resolveMessageTarget(args)
			if err != nil {
				return err
			}
			if threadTS != "" {
				msgs, err := client.FetchThread(channelID, threadTS, limit)
				if err != nil {
					return err
				}
				return printJSON(msgs)
			}
			msgs, err := client.FetchChannelHistory(channelID, limit)
			if err != nil {
				return err
			}
			return printJSON(msgs)
		},
	}
	cmd.Flags().StringVar(&threadTS, "thread-ts", "", "Thread parent timestamp")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max messages to return")
	return cmd
}

func newMessageSendCmd() *cobra.Command {
	var threadTS string
	cmd := &cobra.Command{
		Use:   "send <channel-or-url> <text>",
		Short: "Post a message to a channel or thread",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, channelID, err := resolveChannel(args[0])
			if err != nil {
				return err
			}
			text := strings.Join(args[1:], " ")
			resp, err := client.SendMessage(channelID, text, threadTS)
			if err != nil {
				return err
			}
			return printJSON(resp)
		},
	}
	cmd.Flags().StringVar(&threadTS, "thread-ts", "", "Reply in thread")
	return cmd
}

func newMessageEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <url-or-channel> <timestamp> <text>",
		Short: "Edit a message",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, channelID, ts, err := resolveMessageTarget(args[:2])
			if err != nil {
				return err
			}
			text := strings.Join(args[2:], " ")
			resp, err := client.EditMessage(channelID, ts, text)
			if err != nil {
				return err
			}
			return printJSON(resp)
		},
	}
}

func newMessageDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <url-or-channel> [timestamp]",
		Short: "Delete a message",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, channelID, ts, err := resolveMessageTarget(args)
			if err != nil {
				return err
			}
			resp, err := client.DeleteMessage(channelID, ts)
			if err != nil {
				return err
			}
			return printJSON(resp)
		},
	}
}

func newMessageReactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "react",
		Short: "Manage reactions",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "add <url-or-channel> <timestamp> <emoji>",
			Short: "Add a reaction",
			Args:  cobra.ExactArgs(3),
			RunE: func(cmd *cobra.Command, args []string) error {
				client, channelID, ts, err := resolveMessageTarget(args[:2])
				if err != nil {
					return err
				}
				_, err = client.AddReaction(channelID, ts, normalizeEmoji(args[2]))
				if err != nil {
					return err
				}
				fmt.Println("Reaction added.")
				return nil
			},
		},
		&cobra.Command{
			Use:   "remove <url-or-channel> <timestamp> <emoji>",
			Short: "Remove a reaction",
			Args:  cobra.ExactArgs(3),
			RunE: func(cmd *cobra.Command, args []string) error {
				client, channelID, ts, err := resolveMessageTarget(args[:2])
				if err != nil {
					return err
				}
				_, err = client.RemoveReaction(channelID, ts, normalizeEmoji(args[2]))
				if err != nil {
					return err
				}
				fmt.Println("Reaction removed.")
				return nil
			},
		},
	)
	return cmd
}

// resolveMessageTarget parses args into client + channelID + ts.
// args[0] can be a Slack URL or channel name/ID; args[1] is optional ts.
func resolveMessageTarget(args []string) (*islack.Client, string, string, error) {
	// Try as Slack URL
	ref, err := islack.ParseMessageURL(args[0])
	if err == nil {
		client, _, err := auth.ResolveClient(ref.WorkspaceURL)
		if err != nil {
			return nil, "", "", err
		}
		ts := ref.Timestamp
		if len(args) > 1 {
			ts = args[1]
		}
		return client, ref.ChannelID, ts, nil
	}
	// Channel name/ID + ts
	client, channelID, err := resolveChannel(args[0])
	if err != nil {
		return nil, "", "", err
	}
	ts := ""
	if len(args) > 1 {
		ts = args[1]
	}
	return client, channelID, ts, nil
}

// resolveChannel resolves a channel input to client + channelID.
// Handles channel IDs, channel names, and user IDs (opens DM).
func resolveChannel(input string) (*islack.Client, string, error) {
	client, _, err := auth.ResolveClient(workspaceFlag)
	if err != nil {
		return nil, "", err
	}
	// User ID → open DM
	if islack.IsUserID(input) {
		dmID, err := client.OpenDMChannel(input)
		if err != nil {
			return nil, "", fmt.Errorf("failed to open DM with %s: %w", input, err)
		}
		return client, dmID, nil
	}
	channelID, err := client.ResolveChannelID(input)
	if err != nil {
		return nil, "", err
	}
	return client, channelID, nil
}

func normalizeEmoji(s string) string {
	return strings.Trim(s, ":")
}

