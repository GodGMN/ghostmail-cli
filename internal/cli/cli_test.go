package cli

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCommandFlags(t *testing.T) {
	tests := []struct {
		name  string
		cmd   func() *cobra.Command
		flags []string
	}{
		{name: "send", cmd: newSendCmd, flags: []string{"to", "subject", "body", "body-file", "html-file", "attach", "in-reply-to"}},
		{name: "inbox", cmd: newInboxCmd, flags: []string{"limit", "unread", "mailbox"}},
		{name: "read", cmd: newReadCmd, flags: []string{"uid", "mailbox", "raw"}},
		{name: "reply", cmd: newReplyCmd, flags: []string{"uid", "body", "body-file", "all", "no-quote"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, name := range tt.flags {
				if tt.cmd().Flags().Lookup(name) == nil {
					t.Errorf("%s command is missing --%s", tt.name, name)
				}
			}
		})
	}
}

func TestSendCommandRequiresArgumentsBeforeSMTP(t *testing.T) {
	cmd := newSendCmd()
	cmd.SetArgs([]string{"--to", "recipient@example.com"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "required flag(s) \"subject\" not set") {
		t.Fatalf("send without subject error = %v, want required flag error", err)
	}
}

func TestReadAndReplyRejectZeroUIDBeforeConfiguration(t *testing.T) {
	for _, tt := range []struct {
		name string
		cmd  func() *cobra.Command
	}{
		{name: "read", cmd: newReadCmd},
		{name: "reply", cmd: newReplyCmd},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.cmd()
			cmd.SetArgs([]string{"--uid", "0"})
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), "UID is required") {
				t.Fatalf("%s --uid 0 error = %v, want UID validation error", tt.name, err)
			}
		})
	}
}

func TestFormattingHelpers(t *testing.T) {
	if got := formatBytes(1024); got != "1.0KB" {
		t.Errorf("formatBytes(1024) = %q, want 1.0KB", got)
	}
	if got := formatBytes(10 * 1024 * 1024); got != "10.0MB" {
		t.Errorf("formatBytes(10MB) = %q, want 10.0MB", got)
	}
	if got := truncate("abcdef", 5); got != "ab..." {
		t.Errorf("truncate() = %q, want ab...", got)
	}
	if !isRead([]string{"\\seen"}) || isRead([]string{"\\Flagged"}) {
		t.Error("isRead() did not identify the Seen flag correctly")
	}
	if !isSelf("Sender <ME@example.com>", "me@example.com", "other@example.com") {
		t.Error("isSelf() should match display-name addresses case-insensitively")
	}
	if isSelf("other@example.com", "me@example.com", "me@example.com") {
		t.Error("isSelf() matched an unrelated address")
	}
	// A display-name formatted From/username must still match the bare address.
	if !isSelf("me@example.com", "Me <me@example.com>", "alias <alias@example.com>") {
		t.Error("isSelf() should strip display names from all inputs")
	}
	if isSelf("cc@example.com", "Me <me@example.com>", "me@example.com") {
		t.Error("isSelf() matched an unrelated address against a display-name From")
	}
	// bareEmail normalization.
	if got := bareEmail("Name <USER@Example.COM>"); got != "user@example.com" {
		t.Errorf("bareEmail() = %q, want user@example.com", got)
	}
	if got := bareEmail("user@example.com"); got != "user@example.com" {
		t.Errorf("bareEmail() = %q, want unchanged", got)
	}
	// ccContains matches after normalization, deduping bare vs display-name forms.
	if !ccContains([]string{"Name <a@example.com>"}, "a@example.com") {
		t.Error("ccContains() should match bare address against display-name entry")
	}
	if ccContains([]string{"a@example.com"}, "b@example.com") {
		t.Error("ccContains() matched an unrelated address")
	}
}

func TestMaskPassword(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  string
	}{
		{"", ""},
		{"abcd", "********(4 chars)"},
		{"secret", "********(6 chars)"},
	} {
		if got := maskPassword(tt.input); got != tt.want {
			t.Errorf("maskPassword(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestConfigExampleCommandPrintsUsableTemplate(t *testing.T) {
	output := captureStdout(t, func() {
		if err := newConfigExampleCmd().Execute(); err != nil {
			t.Fatalf("config example error = %v", err)
		}
	})

	for _, line := range []string{
		"export GHOSTMAIL_SMTP_HOST=\"smtp.gmail.com\"",
		"export GHOSTMAIL_IMAP_MAILBOX=\"INBOX\"",
	} {
		if !strings.Contains(output, line) {
			t.Errorf("config example output missing %q", line)
		}
	}
}

func TestConfigCheckCommandReportsMissingAndMasksCredentials(t *testing.T) {
	t.Setenv("GHOSTMAIL_SMTP_PASSWORD", "supersecret")
	t.Setenv("GHOSTMAIL_IMAP_PASSWORD", "imapsecret")

	var commandErr error
	output := captureStdout(t, func() {
		commandErr = newConfigCheckCmd().Execute()
	})

	if commandErr == nil || !strings.Contains(commandErr.Error(), "required configuration is missing") {
		t.Fatalf("config check error = %v, want missing configuration error", commandErr)
	}
	if !strings.Contains(output, "Some required configuration is missing") {
		t.Errorf("config check output = %q, want missing configuration status", output)
	}
	if strings.Contains(output, "supersecret") || strings.Contains(output, "imapsecret") {
		t.Error("config check output exposed a password")
	}
	if !strings.Contains(output, "********(11 chars)") || !strings.Contains(output, "********(10 chars)") {
		t.Errorf("config check output = %q, want masked passwords with length only", output)
	}
}

func captureStdout(t *testing.T, run func()) string {
	t.Helper()
	original := os.Stdout
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = write
	t.Cleanup(func() {
		os.Stdout = original
	})

	run()
	if err := write.Close(); err != nil {
		t.Fatalf("closing stdout pipe: %v", err)
	}
	output, err := io.ReadAll(read)
	if err != nil {
		t.Fatalf("reading stdout pipe: %v", err)
	}
	if err := read.Close(); err != nil {
		t.Fatalf("closing stdout reader: %v", err)
	}
	return string(output)
}

func TestConfigCheckExitStatus(t *testing.T) {
	config := map[string]string{
		"GHOSTMAIL_SMTP_HOST":     "smtp.example.com",
		"GHOSTMAIL_SMTP_PORT":     "587",
		"GHOSTMAIL_SMTP_USERNAME": "smtp-user",
		"GHOSTMAIL_SMTP_PASSWORD": "smtp-password",
		"GHOSTMAIL_SMTP_FROM":     "sender@example.com",
		"GHOSTMAIL_IMAP_HOST":     "imap.example.com",
		"GHOSTMAIL_IMAP_PORT":     "993",
		"GHOSTMAIL_IMAP_USERNAME": "imap-user",
		"GHOSTMAIL_IMAP_PASSWORD": "imap-password",
	}

	for name, value := range config {
		t.Setenv(name, value)
	}
	t.Setenv("GHOSTMAIL_IMAP_MAILBOX", "")

	if err := newConfigCheckCmd().Execute(); err != nil {
		t.Fatalf("config check with complete configuration error = %v, want nil", err)
	}

	t.Setenv("GHOSTMAIL_IMAP_PASSWORD", "")
	err := newConfigCheckCmd().Execute()
	if err == nil || !strings.Contains(err.Error(), "required configuration is missing") {
		t.Fatalf("config check with missing configuration error = %v, want missing configuration error", err)
	}
}
