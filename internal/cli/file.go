package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/cluas/slackctl/internal/auth"
)

func newFileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file",
		Short: "Upload, download, list, and manage Slack files",
	}
	cmd.AddCommand(
		newFileUploadCmd(),
		newFileDownloadCmd(),
		newFileListCmd(),
		newFileInfoCmd(),
		newFileDeleteCmd(),
	)
	return cmd
}

func newFileInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <file-id>",
		Short: "Get file details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := auth.ResolveClient(workspaceFlag)
			if err != nil {
				return err
			}
			fi, err := client.GetFileInfo(args[0])
			if err != nil {
				return err
			}
			return printJSON(fi)
		},
	}
}

func newFileDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <file-id>",
		Short: "Delete a file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := auth.ResolveClient(workspaceFlag)
			if err != nil {
				return err
			}
			if err := client.DeleteFile(args[0]); err != nil {
				return err
			}
			fmt.Println("File deleted.")
			return nil
		},
	}
}

func newFileListCmd() *cobra.Command {
	var channelInput string
	var user string
	var types string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List files",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := auth.ResolveClient(workspaceFlag)
			if err != nil {
				return err
			}
			var channelID string
			if channelInput != "" {
				channelID, err = client.ResolveChannelID(channelInput)
				if err != nil {
					return err
				}
			}
			result, err := client.ListFiles(channelID, user, types, limit)
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	cmd.Flags().StringVar(&channelInput, "channel", "", "Filter by channel (ID or name)")
	cmd.Flags().StringVar(&user, "user", "", "Filter by user ID")
	cmd.Flags().StringVar(&types, "types", "", "File type filter (images,pdfs,snippets,...)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max files to return")
	return cmd
}

func newFileUploadCmd() *cobra.Command {
	var threadTS string
	var title string
	var message string
	var filename string
	cmd := &cobra.Command{
		Use:   "upload <file-or-dash> <channel>",
		Short: "Upload a file to a channel (use - for stdin)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]
			channelInput := args[1]

			client, channelID, err := resolveChannel(channelInput)
			if err != nil {
				return err
			}

			var content io.Reader
			var contentLength int64
			var name string

			if filePath == "-" {
				if filename == "" {
					return fmt.Errorf("--filename is required when reading from stdin")
				}
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read stdin: %w", err)
				}
				content = bytes.NewReader(data)
				contentLength = int64(len(data))
				name = filename
			} else {
				f, err := os.Open(filePath)
				if err != nil {
					return err
				}
				defer f.Close()
				stat, err := f.Stat()
				if err != nil {
					return err
				}
				content = f
				contentLength = stat.Size()
				name = filepath.Base(filePath)
				if filename != "" {
					name = filename
				}
			}

			fi, err := client.UploadFileV2(name, content, contentLength, channelID, threadTS, title, message)
			if err != nil {
				return err
			}
			return printJSON(fi)
		},
	}
	cmd.Flags().StringVar(&threadTS, "thread-ts", "", "Reply in thread")
	cmd.Flags().StringVar(&title, "title", "", "File title")
	cmd.Flags().StringVar(&message, "message", "", "Initial comment")
	cmd.Flags().StringVar(&filename, "filename", "", "Override filename (required for stdin)")
	return cmd
}

func newFileDownloadCmd() *cobra.Command {
	var output string
	var urlOnly bool
	cmd := &cobra.Command{
		Use:   "download <file-id>",
		Short: "Download a file or show its URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := auth.ResolveClient(workspaceFlag)
			if err != nil {
				return err
			}
			fi, err := client.GetFileInfo(args[0])
			if err != nil {
				return err
			}

			if urlOnly {
				return printJSON(fi)
			}

			downloadURL := fi.URLPrivateDownload
			if downloadURL == "" {
				downloadURL = fi.URLPrivate
			}
			if downloadURL == "" {
				return fmt.Errorf("file %s has no download URL", args[0])
			}

			dest := output
			if dest == "" {
				dest = fi.Name
				if dest == "" {
					dest = args[0]
				}
			}

			f, err := os.Create(dest)
			if err != nil {
				return fmt.Errorf("failed to create %s: %w", dest, err)
			}
			defer f.Close()

			if err := client.DownloadFile(downloadURL, f); err != nil {
				os.Remove(dest)
				return err
			}

			return printJSON(map[string]string{
				"file_id": fi.ID,
				"path":    dest,
			})
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "Destination file path")
	cmd.Flags().BoolVar(&urlOnly, "url-only", false, "Output file info with URL instead of downloading")
	return cmd
}
