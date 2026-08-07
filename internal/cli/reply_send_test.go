package cli

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/GodGMN/ghostmail-cli/internal/config"
	emailinternal "github.com/GodGMN/ghostmail-cli/internal/email"
	emailtypes "github.com/GodGMN/ghostmail-cli/pkg/email"
)

// fakeReplyReader returns a canned message for the reply command.
type fakeReplyReader struct {
	message emailtypes.Message
	err     error
	calls   int
}

func (f *fakeReplyReader) ReadMessage(uid uint32) (*emailtypes.Message, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	m := f.message
	return &m, nil
}

// capturingSender records the outbound send arguments so a test can assert on
// recipients, subject, body, and how many options were attached. The option
// contents themselves are verified end-to-end in the email package's SMTP
// tests; here we assert the command wires the right envelope and attaches the
// expected number of options (CC, threading headers, etc.).
type capturingSender struct {
	to      []string
	subject string
	body    string
	numOpts int
	err     error
}

func (c *capturingSender) Send(to []string, subject, body string, opts ...emailinternal.SendOption) error {
	if c.err != nil {
		return c.err
	}
	c.to = to
	c.subject = subject
	c.body = body
	c.numOpts = len(opts)
	return nil
}

func setValidSMTPEnv(t *testing.T) {
	t.Helper()
	for k, v := range map[string]string{
		"GHOSTMAIL_SMTP_HOST":     "smtp.example.com",
		"GHOSTMAIL_SMTP_PORT":     "587",
		"GHOSTMAIL_SMTP_USERNAME": "user@example.com",
		"GHOSTMAIL_SMTP_PASSWORD": "secret",
		"GHOSTMAIL_SMTP_FROM":     "user@example.com",
		"GHOSTMAIL_SMTP_STARTTLS": "true",
	} {
		t.Setenv(k, v)
	}
}

func sampleReplyMessage() emailtypes.Message {
	m := sampleMessage()
	m.References = []string{"<grandparent@example.com>", "<parent@example.com>"}
	return m
}

func TestBuildReferencesChain(t *testing.T) {
	tests := []struct {
		name      string
		ancestors []string
		msgID     string
		want      []string
	}{
		{
			name:      "ancestors then message id",
			ancestors: []string{"<a@x>", "<b@x>"},
			msgID:     "<c@x>",
			want:      []string{"<a@x>", "<b@x>", "<c@x>"},
		},
		{
			name:      "dedupes repeated message id",
			ancestors: []string{"<a@x>", "<c@x>"},
			msgID:     "<c@x>",
			want:      []string{"<a@x>", "<c@x>"},
		},
		{
			name:      "no ancestors",
			ancestors: nil,
			msgID:     "<only@x>",
			want:      []string{"<only@x>"},
		},
		{
			name:      "empty message id and ancestors",
			ancestors: nil,
			msgID:     "",
			want:      nil,
		},
		{
			name:      "skips blank entries",
			ancestors: []string{"<a@x>", "", "<b@x>"},
			msgID:     "",
			want:      []string{"<a@x>", "<b@x>"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildReferencesChain(tt.ancestors, tt.msgID)
			if !sameStrSlice(got, tt.want) {
				t.Errorf("buildReferencesChain() = %v, want %v", got, tt.want)
			}
		})
	}
}

func sameStrSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func swapReplyFactories(t *testing.T, reader replyReader, sender replySender) {
	t.Helper()
	origReader := replyReaderFactory
	origSender := replySenderFactory
	replyReaderFactory = func(_ *config.IMAPConfig) replyReader { return reader }
	replySenderFactory = func(_ *config.SMTPConfig) replySender { return sender }
	t.Cleanup(func() {
		replyReaderFactory = origReader
		replySenderFactory = origSender
	})
}

func TestReplyCommandSendsThreadedReply(t *testing.T) {
	setValidIMAPEnv(t)
	setValidSMTPEnv(t)
	origNoColor := noColor
	noColor = true
	t.Cleanup(func() { noColor = origNoColor })

	fr := &fakeReplyReader{message: sampleReplyMessage()}
	sender := &capturingSender{}
	swapReplyFactories(t, fr, sender)

	out := captureStdout(t, func() {
		cmd := newReplyCmd()
		cmd.SetArgs([]string{"--uid", "42", "--body", "Thanks!"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("reply error = %v", err)
		}
	})

	if fr.calls != 1 {
		t.Errorf("reader called %d times, want 1", fr.calls)
	}
	// Reply goes to the original sender only.
	if !sameStrSlice(sender.to, []string{"sender@example.com"}) {
		t.Errorf("reply to = %v, want [sender@example.com]", sender.to)
	}
	if !strings.HasPrefix(sender.subject, "Re: ") {
		t.Errorf("reply subject = %q, want Re: prefix", sender.subject)
	}
	if !strings.Contains(sender.body, "Thanks!") {
		t.Errorf("reply body = %q, want reply text", sender.body)
	}
	// Quoted original should be appended.
	if !strings.Contains(sender.body, "sender@example.com wrote:") {
		t.Errorf("reply body = %q, want attribution line", sender.body)
	}
	// Threading options: In-Reply-To + References = at least 2 options.
	if sender.numOpts < 2 {
		t.Errorf("reply attached %d options, want >=2 (In-Reply-To + References)", sender.numOpts)
	}
	if !strings.Contains(out, "Reply sent successfully") {
		t.Errorf("reply output = %q, want success message", out)
	}
}

func TestReplyCommandNoQuoteSendsBodyOnly(t *testing.T) {
	setValidIMAPEnv(t)
	setValidSMTPEnv(t)
	origNoColor := noColor
	noColor = true
	t.Cleanup(func() { noColor = origNoColor })

	fr := &fakeReplyReader{message: sampleReplyMessage()}
	sender := &capturingSender{}
	swapReplyFactories(t, fr, sender)

	cmd := newReplyCmd()
	cmd.SetArgs([]string{"--uid", "42", "--body", "Quick", "--no-quote"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("reply --no-quote error = %v", err)
	}
	if sender.body != "Quick" {
		t.Errorf("no-quote body = %q, want %q", sender.body, "Quick")
	}
}

func TestReplyCommandReplyAllAttachesCCOption(t *testing.T) {
	setValidIMAPEnv(t)
	setValidSMTPEnv(t)
	origNoColor := noColor
	noColor = true
	t.Cleanup(func() { noColor = origNoColor })

	msg := sampleReplyMessage()
	msg.To = []string{"recipient@example.com", "user@example.com"} // includes self
	msg.CC = []string{"cc1@example.com", "user@example.com"}       // self deduped out
	fr := &fakeReplyReader{message: msg}
	sender := &capturingSender{}
	swapReplyFactories(t, fr, sender)

	cmd := newReplyCmd()
	cmd.SetArgs([]string{"--uid", "42", "--body", "Hi all", "--all"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("reply --all error = %v", err)
	}

	// Reply-to-all keeps the original sender as the primary recipient...
	if !sameStrSlice(sender.to, []string{"sender@example.com"}) {
		t.Errorf("reply-all to = %v, want [sender@example.com]", sender.to)
	}
	// ...and attaches CC + In-Reply-To + References (>= 3 options).
	if sender.numOpts < 3 {
		t.Errorf("reply-all attached %d options, want >=3 (CC + threading)", sender.numOpts)
	}
}

func TestReplyCommandRequiresBody(t *testing.T) {
	setValidIMAPEnv(t)
	setValidSMTPEnv(t)
	cmd := newReplyCmd()
	cmd.SetArgs([]string{"--uid", "42"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("reply without body error = nil, want body-required error")
	}
}

func TestReplyCommandReaderErrorPropagates(t *testing.T) {
	setValidIMAPEnv(t)
	setValidSMTPEnv(t)
	fr := &fakeReplyReader{err: errors.New("simulated IMAP failure")}
	sender := &capturingSender{}
	swapReplyFactories(t, fr, sender)

	cmd := newReplyCmd()
	cmd.SetArgs([]string{"--uid", "42", "--body", "Hi"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "failed to fetch original message") {
		t.Fatalf("reply reader error = %v, want fetch failure", err)
	}
	if sender.subject != "" {
		t.Errorf("sender was invoked despite reader error: %+v", sender)
	}
}

func TestSendCommandHappyPathInvokesSender(t *testing.T) {
	setValidSMTPEnv(t)
	origNoColor := noColor
	noColor = true
	t.Cleanup(func() { noColor = origNoColor })

	sender := &capturingSender{}
	origSender := sendSenderFactory
	sendSenderFactory = func(_ *config.SMTPConfig) emailSender { return sender }
	t.Cleanup(func() { sendSenderFactory = origSender })

	out := captureStdout(t, func() {
		cmd := newSendCmd()
		cmd.SetArgs([]string{
			"--to", "recipient@example.com",
			"--subject", "Hello",
			"--body", "World",
		})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("send error = %v", err)
		}
	})

	if !sameStrSlice(sender.to, []string{"recipient@example.com"}) {
		t.Errorf("send to = %v", sender.to)
	}
	if sender.subject != "Hello" || sender.body != "World" {
		t.Errorf("send subject/body = %q/%q", sender.subject, sender.body)
	}
	if !strings.Contains(out, "Email sent successfully") {
		t.Errorf("send output = %q, want success message", out)
	}
}

func TestSendCommandJSONOutput(t *testing.T) {
	setValidSMTPEnv(t)
	sender := &capturingSender{}
	origSender := sendSenderFactory
	sendSenderFactory = func(_ *config.SMTPConfig) emailSender { return sender }
	t.Cleanup(func() { sendSenderFactory = origSender })
	jsonOutput = true
	t.Cleanup(func() { jsonOutput = false })

	out := captureStdout(t, func() {
		cmd := newSendCmd()
		cmd.SetArgs([]string{"--to", "a@example.com", "--subject", "s", "--body", "b"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("send json error = %v", err)
		}
	})

	var resp emailtypes.SendResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("send JSON invalid: %v\n%s", err, out)
	}
	if !resp.Success {
		t.Errorf("send JSON success = false, want true")
	}
}

// TestSendCommandRejectsTooManyAttachments exercises the cheap validation that
// runs before any file is opened, so it needs no fixtures.
func TestSendCommandRejectsTooManyAttachments(t *testing.T) {
	setValidSMTPEnv(t)
	sender := &capturingSender{}
	origSender := sendSenderFactory
	sendSenderFactory = func(_ *config.SMTPConfig) emailSender { return sender }
	t.Cleanup(func() { sendSenderFactory = origSender })

	cmd := newSendCmd()
	cmd.SetArgs([]string{
		"--to", "a@example.com",
		"--subject", "s",
		"--body", "b",
		"--attach", "1", "--attach", "2", "--attach", "3",
		"--attach", "4", "--attach", "5", "--attach", "6",
	})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "too many attachments") {
		t.Fatalf("send too-many-attachments error = %v, want 'too many attachments'", err)
	}
	if sender.subject != "" {
		t.Errorf("sender was invoked despite attachment-count rejection")
	}
}

func TestFormatDateBoundaries(t *testing.T) {
	// Cover the today/yesterday/this-week/older branches of formatDate.
	now := time.Now()
	today := now
	yesterday := now.AddDate(0, 0, -1)
	thisWeek := now.AddDate(0, 0, -3)
	older := now.AddDate(0, 0, -30)

	if got := formatDate(today); got == "" {
		t.Error("formatDate(today) returned empty")
	}
	if got := formatDate(yesterday); !strings.Contains(got, "Yesterday") {
		t.Errorf("formatDate(yesterday) = %q, want 'Yesterday'", got)
	}
	if got := formatDate(thisWeek); len(got) != 3 {
		t.Errorf("formatDate(thisWeek) = %q, want 3-letter weekday", got)
	}
	if got := formatDate(older); !strings.Contains(got, "-") {
		t.Errorf("formatDate(older) = %q, want YYYY-MM-DD", got)
	}
}
