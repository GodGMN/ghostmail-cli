package cli

import (
	"fmt"
	"os"

	"github.com/GodGMN/ghostmail-cli/internal/config"
	emailinternal "github.com/GodGMN/ghostmail-cli/internal/email"
	"github.com/GodGMN/ghostmail-cli/internal/output"
	emailtypes "github.com/GodGMN/ghostmail-cli/pkg/email"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newSendCmd() *cobra.Command {
	var (
		to          []string
		cc          []string
		bcc         []string
		subject     string
		body        string
		bodyFile    string
		htmlFile    string
		attachments []string
		htmlBody    string
		inReplyTo   string
	)

	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send an email via SMTP",
		Long: `Send an email via SMTP.

You can provide the email body directly with --body, or read from a file with --body-file.
HTML content can be provided with --html-file for rich formatting.

REQUIRED FLAGS:
  --to      Recipient email address(es)
  --subject Email subject line
  --body    Email body text (or use --body-file)

EXAMPLES:
  # Simple text email
  ghostmail send --to recipient@example.com --subject "Hello" --body "World"

  # With CC and BCC
  ghostmail send --to a@example.com --cc b@example.com --bcc c@example.com \
    --subject "Hello" --body "World"

  # With multiple recipients
  ghostmail send -t user1@example.com -t user2@example.com -s "Hello" -b "World"

  # With attachments (max 5 files, 10MB each)
  ghostmail send --to a@example.com --subject "Documents" \
    --body "Please find attached" --attach document.pdf --attach image.png

  # HTML email with plain text fallback
  ghostmail send --to user@example.com --subject "Newsletter" \
    --html-file newsletter.html --body "Plain text version"

  # Body from file
  ghostmail send --to user@example.com --subject "Report" --body-file report.txt

  # Reply to a message (enables threading)
  ghostmail send --to user@example.com --subject "Re: Original" \
    --body "My reply" --in-reply-to "<msg-id@example.com>"

For more help, use: ghostmail send --help`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			cfg, err := config.Load()
			if err != nil {
				return handleError(err)
			}

			if err := cfg.ValidateSMTP(); err != nil {
				return handleError(err)
			}

			// Handle body from file
			if bodyFile != "" {
				data, err := os.ReadFile(bodyFile)
				if err != nil {
					return handleError(fmt.Errorf("failed to read body file: %w. Use --help for usage info", err))
				}
				body = string(data)
			}

			// Handle HTML from file
			if htmlFile != "" {
				data, err := os.ReadFile(htmlFile)
				if err != nil {
					return handleError(fmt.Errorf("failed to read HTML file: %w. Use --help for usage info", err))
				}
				htmlBody = string(data)
			}

			// Validate required fields
			if len(to) == 0 {
				return handleError(fmt.Errorf("at least one recipient (--to) is required. Use --help for usage info"))
			}
			if subject == "" {
				return handleError(fmt.Errorf("subject is required. Use --help for usage info"))
			}
			if body == "" && htmlBody == "" {
				return handleError(fmt.Errorf("either --body or --html-file must be provided. Use --help for usage info"))
			}

			// Validate attachments (max 5 files, 10MB each)
			const (
				maxAttachments = 5
				maxFileSize    = 10 * 1024 * 1024 // 10MB
			)
			if len(attachments) > maxAttachments {
				return handleError(fmt.Errorf("too many attachments: maximum is %d (you have %d). Use --help for usage info", maxAttachments, len(attachments)))
			}
			for _, att := range attachments {
				info, err := os.Stat(att)
				if err != nil {
					return handleError(fmt.Errorf("cannot access attachment %s: %w. Use --help for usage info", att, err))
				}
				if info.Size() > maxFileSize {
					return handleError(fmt.Errorf("attachment %s is too large: %s (max %s). Use --help for usage info", att, formatBytes(info.Size()), formatBytes(maxFileSize)))
				}
			}

			// Send email
			sender := emailinternal.NewSender(&cfg.SMTP)
			opts := []emailinternal.SendOption{
				emailinternal.WithCC(cc),
				emailinternal.WithBCC(bcc),
				emailinternal.WithAttachments(attachments),
			}
			if htmlBody != "" {
				opts = append(opts, emailinternal.WithHTMLBody(htmlBody))
			}
			if inReplyTo != "" {
				opts = append(opts, emailinternal.WithInReplyTo(inReplyTo))
			}

			if err := sender.Send(to, subject, body, opts...); err != nil {
				return handleError(err)
			}

			// Output result
			if jsonOutput {
				resp := emailtypes.SendResponse{
					Success: true,
					Message: "Email sent successfully",
				}
				return output.NewJSONOutput(true).Print(resp)
			}

			if !noColor {
				color.Green("✓ Email sent successfully")
			} else {
				fmt.Println("Email sent successfully")
			}
			return nil
		},
	}

	// Flags
	cmd.Flags().StringArrayVarP(&to, "to", "t", nil, "Recipient email address (can be specified multiple times)")
	cmd.Flags().StringArrayVarP(&cc, "cc", "c", nil, "CC recipient (can be specified multiple times)")
	cmd.Flags().StringArrayVarP(&bcc, "bcc", "b", nil, "BCC recipient (can be specified multiple times)")
	cmd.Flags().StringVarP(&subject, "subject", "s", "", "Email subject")
	cmd.Flags().StringVarP(&body, "body", "m", "", "Email body text")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "Read email body from file")
	cmd.Flags().StringVar(&htmlFile, "html-file", "", "Read HTML body from file")
	cmd.Flags().StringArrayVarP(&attachments, "attach", "a", nil, "File attachment (can be specified multiple times, max 5 files, 10MB each)")
	cmd.Flags().StringVar(&inReplyTo, "in-reply-to", "", "Message-ID to reply to (enables threading)")

	cmd.MarkFlagRequired("to")
	cmd.MarkFlagRequired("subject")

	return cmd
}

func handleError(err error) error {
	if jsonOutput {
		output.PrintErrorMsg(err.Error())
		os.Exit(1)
	}
	return err
}

// formatBytes formats bytes into human-readable format
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
	)
	switch {
	case bytes >= MB:
		return fmt.Sprintf("%.1fMB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1fKB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
