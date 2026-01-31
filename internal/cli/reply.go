package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/GodGMN/ghostmail-cli/internal/config"
	emailinternal "github.com/GodGMN/ghostmail-cli/internal/email"
	"github.com/GodGMN/ghostmail-cli/internal/output"
	emailtypes "github.com/GodGMN/ghostmail-cli/pkg/email"
)

func newReplyCmd() *cobra.Command {
	var (
		uid      uint32
		mailbox  string
		body     string
		bodyFile string
		all      bool // Reply to all (include CC)
		noQuote  bool // Skip quoting original
	)

	cmd := &cobra.Command{
		Use:   "reply",
		Short: "Reply to an email by UID",
		Long: `Reply to an email by its UID (Unique Identifier).

Fetches the original email, formats a proper reply with quoted content,
and sets threading headers (In-Reply-To, References) for email clients
to display the conversation correctly.

The reply body will be formatted as:
  <your reply text>

  On <date>, <sender> wrote:
  > Original message line 1
  > Original message line 2
  > ...

REQUIRED FLAGS:
  --uid     The UID of the email to reply to (from 'ghostmail inbox')
  --body    Your reply text (or use --body-file)

EXAMPLES:
  # Simple reply
  ghostmail reply --uid 12345 --body "Thanks for the info!"

  # Reply to all recipients (includes CC)
  ghostmail reply --uid 12345 --body "Thanks everyone" --all

  # Reply without quoting original (just your text)
  ghostmail reply --uid 12345 --body "Quick reply" --no-quote

  # Reply from specific mailbox
  ghostmail reply --uid 12345 --body "Got it" --mailbox Archive

  # Body from file
  ghostmail reply --uid 12345 --body-file response.txt

For more help, use: ghostmail reply --help`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if uid == 0 {
				return handleError(fmt.Errorf("UID is required (use --uid). Get from 'ghostmail inbox'. Use --help for usage info"))
			}

			// Load configuration
			cfg, err := config.Load()
			if err != nil {
				return handleError(err)
			}

			if err := cfg.ValidateIMAP(); err != nil {
				return handleError(fmt.Errorf("IMAP config error: %w. Use --help for usage info", err))
			}
			if err := cfg.ValidateSMTP(); err != nil {
				return handleError(fmt.Errorf("SMTP config error: %w. Use --help for usage info", err))
			}

			// Handle body from file
			if bodyFile != "" {
				data, err := os.ReadFile(bodyFile)
				if err != nil {
					return handleError(fmt.Errorf("failed to read body file: %w. Use --help for usage info", err))
				}
				body = string(data)
			}

			if body == "" {
				return handleError(fmt.Errorf("reply body is required (use --body or --body-file). Use --help for usage info"))
			}

			// Override mailbox if specified
			if mailbox != "" {
				cfg.IMAP.Mailbox = mailbox
			}

			// Fetch original message
			reader := emailinternal.NewReader(&cfg.IMAP)
			original, err := reader.ReadMessage(uid)
			if err != nil {
				return handleError(fmt.Errorf("failed to fetch original message: %w. Use --help for usage info", err))
			}

			// Build reply
			to := []string{original.From}
			var cc []string

			if all {
				// Reply to all: include original CC recipients (excluding self)
				for _, addr := range original.CC {
					if !isSelf(addr, cfg.SMTP.From, cfg.SMTP.Username) {
						cc = append(cc, addr)
					}
				}
				// Also add original To recipients if not the sender
				for _, addr := range original.To {
					if !isSelf(addr, cfg.SMTP.From, cfg.SMTP.Username) && addr != original.From {
						// Avoid duplicates
						found := false
						for _, existing := range cc {
							if existing == addr {
								found = true
								break
							}
						}
						if !found {
							cc = append(cc, addr)
						}
					}
				}
			}

			// Format subject with Re: prefix (if not already present)
			subject := original.Subject
			if !strings.HasPrefix(strings.ToLower(subject), "re:") {
				subject = "Re: " + subject
			}

			// Format reply body
			var replyBody string
			if noQuote {
				replyBody = body
			} else {
				dateStr := original.Date.Format("2006-01-02 15:04")
				replyBody = emailinternal.FormatQuotedReply(body, original.Body, original.From, dateStr)
			}

			// Build references chain
			var references []string
			if original.MessageID != "" {
				references = append(references, original.MessageID)
			}

			// Send the reply
			sender := emailinternal.NewSender(&cfg.SMTP)
			opts := []emailinternal.SendOption{}

			if len(cc) > 0 {
				opts = append(opts, emailinternal.WithCC(cc))
			}

			// Set threading headers
			if original.MessageID != "" {
				opts = append(opts, emailinternal.WithInReplyTo(original.MessageID))
				opts = append(opts, emailinternal.WithReferences(references))
			}

			if err := sender.Send(to, subject, replyBody, opts...); err != nil {
				return handleError(err)
			}

			// Output result
			if jsonOutput {
				resp := emailtypes.SendResponse{
					Success: true,
					Message: fmt.Sprintf("Reply sent to %s", strings.Join(to, ", ")),
				}
				return output.NewJSONOutput(true).Print(resp)
			}

			if !noColor {
				color.Green("✓ Reply sent successfully to %s", to[0])
			} else {
				fmt.Printf("Reply sent successfully to %s\n", to[0])
			}

			if verbose {
				fmt.Printf("  Subject: %s\n", subject)
				fmt.Printf("  In-Reply-To: %s\n", original.MessageID)
				if len(cc) > 0 {
					fmt.Printf("  CC: %s\n", strings.Join(cc, ", "))
				}
			}

			return nil
		},
	}

	cmd.Flags().Uint32VarP(&uid, "uid", "u", 0, "Message UID to reply to (required). Get from 'ghostmail inbox'")
	cmd.Flags().StringVarP(&mailbox, "mailbox", "m", "", "Mailbox containing the message (default: INBOX)")
	cmd.Flags().StringVarP(&body, "body", "b", "", "Reply body text")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "Read reply body from file")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "Reply to all recipients (include CC)")
	cmd.Flags().BoolVar(&noQuote, "no-quote", false, "Don't quote the original message")

	cmd.MarkFlagRequired("uid")

	return cmd
}

// isSelf checks if an address belongs to the current user
func isSelf(addr, from, username string) bool {
	addr = strings.ToLower(addr)
	from = strings.ToLower(from)
	username = strings.ToLower(username)

	// Extract email from "Name <email>" format
	if idx := strings.LastIndex(addr, "<"); idx != -1 {
		if endIdx := strings.LastIndex(addr, ">"); endIdx != -1 && endIdx > idx {
			addr = addr[idx+1 : endIdx]
		}
	}

	return addr == from || addr == username
}
