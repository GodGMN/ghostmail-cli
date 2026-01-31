// Package cli provides the command-line interface for ghostmail-cli.
package cli

import (
	"github.com/spf13/cobra"
)

var (
	// Global flags
	jsonOutput bool
	noColor    bool
	verbose    bool
)

// Execute runs the CLI application.
func Execute(version, commit, date string) error {
	rootCmd := &cobra.Command{
		Use:   "ghostmail",
		Short: "A CLI tool for sending and reading emails",
		Long: `Ghostmail is a command-line email client that supports SMTP for sending
and IMAP for reading emails. All configuration is done via environment variables.`,
		Version: version,
	}

	// Persistent flags
	rootCmd.PersistentFlags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	// Add commands
	rootCmd.AddCommand(newSendCmd())
	rootCmd.AddCommand(newInboxCmd())
	rootCmd.AddCommand(newReadCmd())
	rootCmd.AddCommand(newReplyCmd())
	rootCmd.AddCommand(newConfigCmd())

	return rootCmd.Execute()
}
