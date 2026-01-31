package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/GodGMN/ghostmail-cli/internal/config"
	emailinternal "github.com/GodGMN/ghostmail-cli/internal/email"
	"github.com/GodGMN/ghostmail-cli/internal/output"
	emailtypes "github.com/GodGMN/ghostmail-cli/pkg/email"
)

func newInboxCmd() *cobra.Command {
	var (
		limit      int
		unreadOnly bool
		mailbox    string
	)

	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "List emails from a mailbox",
		Long: `List emails from an IMAP mailbox (default: INBOX).

Displays a table of emails with UID, sender, subject, and date.
Use the UID with 'ghostmail read' to view message contents.

EXAMPLES:
  # List last 20 emails (default)
  ghostmail inbox

  # List with custom limit
  ghostmail inbox --limit 10

  # Show only unread messages
  ghostmail inbox --unread

  # List from specific mailbox
  ghostmail inbox --mailbox "Sent Items"

  # JSON output for scripting
  ghostmail inbox --limit 50 --json

  # Get unread count
  ghostmail inbox --unread --json | jq '.messages | length'

For more help, use: ghostmail inbox --help`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			cfg, err := config.Load()
			if err != nil {
				return handleError(err)
			}

			if err := cfg.ValidateIMAP(); err != nil {
				return handleError(fmt.Errorf("%w. Use --help for usage info", err))
			}

			// Override mailbox if specified
			if mailbox != "" {
				cfg.IMAP.Mailbox = mailbox
			}

			// Fetch messages
			reader := emailinternal.NewReader(&cfg.IMAP)
			messages, err := reader.ListMessages(limit, unreadOnly)
			if err != nil {
				return handleError(fmt.Errorf("%w. Use --help for usage info", err))
			}

			// Output
			if jsonOutput {
				resp := emailtypes.InboxResponse{
					Success:  true,
					Messages: messages,
					Total:    len(messages),
				}
				return output.NewJSONOutput(true).Print(resp)
			}

			// Human-readable output
			if len(messages) == 0 {
				if unreadOnly {
					fmt.Println("No unread messages")
				} else {
					fmt.Println("No messages")
				}
				return nil
			}

			// Print table
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

			// Header
			headerFmt := "%s\t%s\t%s\t%s\n"
			if !noColor {
				headerFmt = color.New(color.Bold).Sprintf(headerFmt)
			}
			fmt.Fprintf(w, headerFmt, "UID", "FROM", "SUBJECT", "DATE")

			// Rows
			for _, msg := range messages {
				from := truncate(msg.From, 25)
				subject := truncate(msg.Subject, 40)
				date := formatDate(msg.Date)

				// Highlight unread messages
				row := fmt.Sprintf("%d\t%s\t%s\t%s\n", msg.UID, from, subject, date)
				if !noColor && !isRead(msg.Flags) {
					row = color.New(color.Bold).Sprint(row)
				}
				fmt.Fprint(w, row)
			}

			w.Flush()
			fmt.Printf("\nTotal: %d messages\n", len(messages))

			return nil
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "l", 20, "Maximum number of messages to show (0 = all)")
	cmd.Flags().BoolVarP(&unreadOnly, "unread", "u", false, "Show only unread messages")
	cmd.Flags().StringVarP(&mailbox, "mailbox", "m", "", "Mailbox to list (default: INBOX)")

	return cmd
}

// truncate truncates a string to max length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// formatDate formats a date for display.
func formatDate(t time.Time) string {
	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	msgDay := t.Truncate(24 * time.Hour)

	if msgDay.Equal(today) {
		return t.Format("15:04")
	}
	if msgDay.Equal(today.Add(-24 * time.Hour)) {
		return "Yesterday"
	}
	if msgDay.After(today.Add(-7 * 24 * time.Hour)) {
		return t.Format("Mon")
	}
	return t.Format("2006-01-02")
}

// isRead checks if a message is read based on flags.
func isRead(flags []string) bool {
	for _, flag := range flags {
		if strings.EqualFold(flag, "\\Seen") {
			return true
		}
	}
	return false
}
