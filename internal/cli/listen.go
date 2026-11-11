package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/cluas/slackctl/internal/auth"
	"github.com/cluas/slackctl/internal/listen"
)

func newListenCmd() *cobra.Command {
	var (
		types         string
		mentionsOrDMs bool
		includeBots   bool
		allSubtypes   bool
	)
	cmd := &cobra.Command{
		Use:   "listen",
		Short: "Stream real-time events as JSON Lines (RTM websocket)",
		Long: `Open a real-time websocket to Slack (rtm.connect) and print each event
as one JSON object per line on stdout. Diagnostics go to stderr.

Control lines: {"type":"_connected"}, {"type":"_reconnected"}, and
{"type":"_auth_error"} / {"type":"_connect_error"} (both followed by a
non-zero exit). Enterprise Grid org-level sessions are not supported: Slack
refuses rtm.connect there (enterprise_is_restricted). Read-only: never sends
messages, marks read, or reacts.`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := auth.ResolveClient(workspaceFlag)
			if err != nil {
				return err
			}
			filter := listen.Filter{
				Types:         listen.ParseTypes(types),
				MentionsOrDMs: mentionsOrDMs,
				IncludeBots:   includeBots,
				AllSubtypes:   allSubtypes,
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return listen.NewListener(client, filter, os.Stdout, os.Stderr).Run(ctx)
		},
	}
	cmd.Flags().StringVar(&types, "types", "message", "Comma-separated event types to emit (empty = all)")
	cmd.Flags().BoolVar(&mentionsOrDMs, "mentions-or-dms", false,
		"Only messages in DMs / group DMs or that @-mention you")
	cmd.Flags().BoolVar(&includeBots, "include-bots", true, "Include bot messages")
	cmd.Flags().BoolVar(&allSubtypes, "all-subtypes", false,
		"Also emit message_changed / message_deleted / join etc.")
	return cmd
}
