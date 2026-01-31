package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const configTemplate = `# Ghostmail Configuration
# Copy these environment variables to your shell profile or .env file

# SMTP Configuration (for sending emails)
export GHOSTMAIL_SMTP_HOST="smtp.gmail.com"
export GHOSTMAIL_SMTP_PORT="587"
export GHOSTMAIL_SMTP_USERNAME="your-email@gmail.com"
export GHOSTMAIL_SMTP_PASSWORD="your-app-password"
export GHOSTMAIL_SMTP_FROM="your-email@gmail.com"
export GHOSTMAIL_SMTP_STARTTLS="true"

# IMAP Configuration (for reading emails)
export GHOSTMAIL_IMAP_HOST="imap.gmail.com"
export GHOSTMAIL_IMAP_PORT="993"
export GHOSTMAIL_IMAP_USERNAME="your-email@gmail.com"
export GHOSTMAIL_IMAP_PASSWORD="your-app-password"
export GHOSTMAIL_IMAP_USE_TLS="true"
export GHOSTMAIL_IMAP_MAILBOX="INBOX"
`

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration helper commands",
		Long: `Helper commands for managing ghostmail configuration.

Environment variables are used for all configuration. No config files needed.

COMMANDS:
  example  Print example configuration with all env vars
  check    Verify that required environment variables are set

EXAMPLES:
  # Print example configuration
  ghostmail config example

  # Check current configuration
  ghostmail config check

  # Source example config (edit first!)
  eval "$(ghostmail config example)"

For more help, use: ghostmail config --help`,
	}

	cmd.AddCommand(newConfigExampleCmd())
	cmd.AddCommand(newConfigCheckCmd())

	return cmd
}

func newConfigExampleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "example",
		Short: "Print example configuration",
		Long: `Prints an example configuration file with all environment variables.

Copy the output to your shell profile (~/.bashrc, ~/.zshrc) or source it directly
after editing with your credentials.

For Gmail, use an App Password instead of your regular password:
https://support.google.com/accounts/answer/185833

EXAMPLE:
  # Print example
  ghostmail config example

  # Save to file for editing
  ghostmail config example > ~/.ghostmail-env
  # Edit ~/.ghostmail-env with your credentials, then:
  source ~/.ghostmail-env`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print(configTemplate)
		},
	}
}

func newConfigCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check current configuration",
		Long: `Verifies that all required environment variables are set.

Checks for the presence of SMTP and IMAP configuration variables
and shows which ones are set or missing.

EXAMPLE:
  ghostmail config check

TIP: If configuration is missing, run 'ghostmail config example' to see
what variables need to be set.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// This will be implemented to check config
			fmt.Println("Checking configuration...")

			vars := []struct {
				name  string
				value string
				smtp  bool
				imap  bool
			}{
				{"GHOSTMAIL_SMTP_HOST", os.Getenv("GHOSTMAIL_SMTP_HOST"), true, false},
				{"GHOSTMAIL_SMTP_PORT", os.Getenv("GHOSTMAIL_SMTP_PORT"), true, false},
				{"GHOSTMAIL_SMTP_USERNAME", os.Getenv("GHOSTMAIL_SMTP_USERNAME"), true, false},
				{"GHOSTMAIL_SMTP_PASSWORD", maskPassword(os.Getenv("GHOSTMAIL_SMTP_PASSWORD")), true, false},
				{"GHOSTMAIL_SMTP_FROM", os.Getenv("GHOSTMAIL_SMTP_FROM"), true, false},
				{"GHOSTMAIL_IMAP_HOST", os.Getenv("GHOSTMAIL_IMAP_HOST"), false, true},
				{"GHOSTMAIL_IMAP_PORT", os.Getenv("GHOSTMAIL_IMAP_PORT"), false, true},
				{"GHOSTMAIL_IMAP_USERNAME", os.Getenv("GHOSTMAIL_IMAP_USERNAME"), false, true},
				{"GHOSTMAIL_IMAP_PASSWORD", maskPassword(os.Getenv("GHOSTMAIL_IMAP_PASSWORD")), false, true},
				{"GHOSTMAIL_IMAP_MAILBOX", os.Getenv("GHOSTMAIL_IMAP_MAILBOX"), false, true},
			}

			fmt.Println("\nSMTP Configuration:")
			fmt.Println("-------------------")
			smtpOK := true
			for _, v := range vars {
				if !v.smtp {
					continue
				}
				status := "✓"
				if v.value == "" {
					status = "✗"
					smtpOK = false
				}
				fmt.Printf("  %s %s=%s\n", status, v.name, v.value)
			}

			fmt.Println("\nIMAP Configuration:")
			fmt.Println("-------------------")
			imapOK := true
			for _, v := range vars {
				if !v.imap {
					continue
				}
				status := "✓"
				if v.value == "" {
					status = "✗"
					imapOK = false
				}
				fmt.Printf("  %s %s=%s\n", status, v.name, v.value)
			}

			fmt.Println()
			if smtpOK && imapOK {
				fmt.Println("✓ All required configuration is set")
			} else {
				fmt.Println("✗ Some required configuration is missing")
				fmt.Println("\nRun 'ghostmail config example' to see an example configuration")
			}

			return nil
		},
	}
}

func maskPassword(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + "****" + s[len(s)-2:]
}
