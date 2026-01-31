package cli

import (
	"fmt"
	"strings"

	"github.com/GodGMN/ghostmail-cli/internal/config"
	emailinternal "github.com/GodGMN/ghostmail-cli/internal/email"
	"github.com/GodGMN/ghostmail-cli/internal/output"
	emailtypes "github.com/GodGMN/ghostmail-cli/pkg/email"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newReadCmd() *cobra.Command {
	var (
		uid     uint32
		mailbox string
		raw     bool
	)

	cmd := &cobra.Command{
		Use:   "read",
		Short: "Read a specific email by UID",
		Long: `Read a specific email by its UID (Unique Identifier).

The UID is displayed in the 'ghostmail inbox' output. Use it to
retrieve the full content of a message including body and attachments.

EXAMPLES:
  # Read email with UID 12345
  ghostmail read --uid 12345

  # Read from specific mailbox
  ghostmail read --uid 12345 --mailbox Archive

  # Quick preview (faster, no body parsing)
  ghostmail read --uid 12345 --raw

  # Get JSON for scripting
  ghostmail read --uid 12345 --json

  # Extract subject using jq
  ghostmail read --uid 12345 --json | jq -r '.message.subject'

For more help, use: ghostmail read --help`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if uid == 0 {
				return handleError(fmt.Errorf("UID is required (use --uid). Use --help for usage info"))
			}

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

			// Fetch message
			reader := emailinternal.NewReader(&cfg.IMAP)
			msg, err := reader.ReadMessage(uid)
			if err != nil {
				return handleError(fmt.Errorf("%w. Use --help for usage info", err))
			}

			// Output
			if jsonOutput {
				resp := emailtypes.ReadResponse{
					Success: true,
					Message: *msg,
				}
				return output.NewJSONOutput(true).Print(resp)
			}

			// Human-readable output
			if !noColor {
				color.Cyan("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			} else {
				fmt.Println("----------------------------------------")
			}

			// Header
			headerColor := color.New(color.Bold, color.FgWhite)
			if noColor {
				fmt.Printf("Subject: %s\n", msg.Subject)
				fmt.Printf("From: %s\n", msg.From)
				fmt.Printf("To: %s\n", strings.Join(msg.To, ", "))
				if len(msg.CC) > 0 {
					fmt.Printf("CC: %s\n", strings.Join(msg.CC, ", "))
				}
				if len(msg.BCC) > 0 {
					fmt.Printf("BCC: %s\n", strings.Join(msg.BCC, ", "))
				}
				fmt.Printf("Date: %s\n", msg.Date.Format("2006-01-02 15:04:05"))
				fmt.Printf("UID: %d\n", msg.UID)
			} else {
				headerColor.Printf("Subject: ")
				fmt.Println(msg.Subject)
				headerColor.Printf("From: ")
				fmt.Println(msg.From)
				headerColor.Printf("To: ")
				fmt.Println(strings.Join(msg.To, ", "))
				if len(msg.CC) > 0 {
					headerColor.Printf("CC: ")
					fmt.Println(strings.Join(msg.CC, ", "))
				}
				if len(msg.BCC) > 0 {
					headerColor.Printf("BCC: ")
					fmt.Println(strings.Join(msg.BCC, ", "))
				}
				headerColor.Printf("Date: ")
				fmt.Println(msg.Date.Format("2006-01-02 15:04:05"))
				headerColor.Printf("UID: ")
				fmt.Println(msg.UID)
			}

			if !noColor {
				color.Cyan("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			} else {
				fmt.Println("----------------------------------------")
			}

			// Body
			if raw || msg.Body == "" {
				if msg.BodyPreview != "" {
					fmt.Println(msg.BodyPreview)
				} else {
					fmt.Println("(No body content)")
				}
			} else {
				fmt.Println(msg.Body)
			}

			// Attachments
			if len(msg.Attachments) > 0 {
				if !noColor {
					color.Cyan("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
				} else {
					fmt.Println("----------------------------------------")
				}
				fmt.Printf("Attachments: %d\n", len(msg.Attachments))
				for _, att := range msg.Attachments {
					fmt.Printf("  - %s (%s, %d bytes)\n", att.Filename, att.ContentType, att.Size)
				}
			}

			if !noColor {
				color.Cyan("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			} else {
				fmt.Println("----------------------------------------")
			}

			return nil
		},
	}

	cmd.Flags().Uint32VarP(&uid, "uid", "u", 0, "Message UID (required). Get from 'ghostmail inbox'")
	cmd.Flags().StringVarP(&mailbox, "mailbox", "m", "", "Mailbox to read from (default: INBOX)")
	cmd.Flags().BoolVar(&raw, "raw", false, "Show raw/preview body only (faster)")

	cmd.MarkFlagRequired("uid")

	return cmd
}
