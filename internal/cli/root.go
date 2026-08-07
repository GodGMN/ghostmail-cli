// Package cli provides the command-line interface for ghostmail-cli.
package cli

import (
	"fmt"
	"os"

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
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	// Persistent flags
	rootCmd.PersistentFlags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output (prints connection details to stderr)")

	// Add commands
	rootCmd.AddCommand(newSendCmd())
	rootCmd.AddCommand(newInboxCmd())
	rootCmd.AddCommand(newReadCmd())
	rootCmd.AddCommand(newReplyCmd())
	rootCmd.AddCommand(newConfigCmd())

	err := rootCmd.Execute()
	// With SilenceErrors=true cobra no longer prints; emit a single human-readable line
	// for non-JSON runs so users still see what went wrong. JSON errors are already
	// written to stderr by handleError.
	if err != nil && !jsonOutput {
		fmt.Fprintln(os.Stderr, "Error:", err)
	}
	return err
}

// debugf prints a verbose diagnostic line to stderr when --verbose is set.
// It is a no-op otherwise, so it is safe to call from hot paths.
func debugf(format string, args ...any) {
	if !verbose {
		return
	}
	fmt.Fprintf(os.Stderr, "[ghostmail] "+format+"\n", args...)
}
