package cli

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/GodGMN/ghostmail-cli/internal/config"
	emailtypes "github.com/GodGMN/ghostmail-cli/pkg/email"
)

// fakeLister returns a canned message list (or an error) for the inbox command.
type fakeLister struct {
	messages []emailtypes.Message
	err      error
}

func (f *fakeLister) ListMessages(limit int, unreadOnly bool) ([]emailtypes.Message, error) {
	return f.messages, f.err
}

// fakeReader returns a canned message for the read command.
type fakeReader struct {
	message emailtypes.Message
	err     error
	calls   int
}

func (f *fakeReader) ReadMessage(uid uint32) (*emailtypes.Message, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	m := f.message
	return &m, nil
}

func (f *fakeReader) ReadMessagePreview(uid uint32) (*emailtypes.Message, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	m := f.message
	m.Body = "" // preview path never returns the full body, matching the real reader
	return &m, nil
}

var errFake = errors.New("simulated IMAP failure")

func sampleMessage() emailtypes.Message {
	return emailtypes.Message{
		UID:         42,
		Subject:     "Hello",
		From:        "sender@example.com",
		To:          []string{"recipient@example.com"},
		Date:        time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC),
		Flags:       []string{"\\Seen"},
		Body:        "This is the body.",
		BodyPreview: "This is the body.",
		MessageID:   "<abc@example.com>",
	}
}

// setValidIMAPEnv populates the IMAP env vars so config.Load + ValidateIMAP
// succeed, letting the command reach the (faked) reader. It also disables
// colored output: fatih/color caches its writer at init time and would bypass
// the os.Stdout pipe swap, making assertions non-deterministic.
func setValidIMAPEnv(t *testing.T) {
	t.Helper()
	for k, v := range map[string]string{
		"GHOSTMAIL_IMAP_HOST":     "imap.example.com",
		"GHOSTMAIL_IMAP_PORT":     "993",
		"GHOSTMAIL_IMAP_USERNAME": "user@example.com",
		"GHOSTMAIL_IMAP_PASSWORD": "secret",
		"GHOSTMAIL_IMAP_USE_TLS":  "true",
	} {
		t.Setenv(k, v)
	}
	origNoColor := noColor
	noColor = true
	t.Cleanup(func() { noColor = origNoColor })
}

func swapInboxLister(t *testing.T, lst messageLister) {
	t.Helper()
	setValidIMAPEnv(t)
	orig := inboxReaderFactory
	inboxReaderFactory = func(_ *config.IMAPConfig) messageLister { return lst }
	t.Cleanup(func() { inboxReaderFactory = orig })
}

func swapReadReader(t *testing.T, r messageReader) {
	t.Helper()
	setValidIMAPEnv(t)
	orig := readReaderFactory
	readReaderFactory = func(_ *config.IMAPConfig) messageReader { return r }
	t.Cleanup(func() { readReaderFactory = orig })
}

func TestInboxCommandEmptyOutput(t *testing.T) {
	swapInboxLister(t, &fakeLister{messages: nil})

	out := captureStdout(t, func() {
		cmd := newInboxCmd()
		cmd.SetArgs([]string{"--limit", "5"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("inbox empty error = %v", err)
		}
	})
	if !strings.Contains(out, "No messages") {
		t.Errorf("empty inbox output = %q, want 'No messages'", out)
	}
}

func TestInboxCommandUnreadEmptyOutput(t *testing.T) {
	swapInboxLister(t, &fakeLister{messages: nil})

	out := captureStdout(t, func() {
		cmd := newInboxCmd()
		cmd.SetArgs([]string{"--unread"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("inbox unread error = %v", err)
		}
	})
	if !strings.Contains(out, "No unread messages") {
		t.Errorf("unread empty inbox output = %q, want 'No unread messages'", out)
	}
}

func TestInboxCommandTableOutput(t *testing.T) {
	swapInboxLister(t, &fakeLister{messages: []emailtypes.Message{sampleMessage()}})

	out := captureStdout(t, func() {
		cmd := newInboxCmd()
		cmd.SetArgs([]string{})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("inbox error = %v", err)
		}
	})
	for _, want := range []string{"UID", "42", "Hello", "Total: 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("inbox table output missing %q:\n%s", want, out)
		}
	}
}

func TestInboxCommandJSONOutput(t *testing.T) {
	swapInboxLister(t, &fakeLister{messages: []emailtypes.Message{sampleMessage()}})
	jsonOutput = true
	t.Cleanup(func() { jsonOutput = false })

	out := captureStdout(t, func() {
		cmd := newInboxCmd()
		cmd.SetArgs([]string{})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("inbox json error = %v", err)
		}
	})

	var resp emailtypes.InboxResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("inbox JSON invalid: %v\n%s", err, out)
	}
	if !resp.Success || resp.Total != 1 || len(resp.Messages) != 1 {
		t.Errorf("inbox JSON = %+v, want success with 1 message", resp)
	}
	if resp.Messages[0].UID != 42 {
		t.Errorf("inbox JSON uid = %d, want 42", resp.Messages[0].UID)
	}
}

func TestReadCommandFullBodyOutput(t *testing.T) {
	swapReadReader(t, &fakeReader{message: sampleMessage()})

	out := captureStdout(t, func() {
		cmd := newReadCmd()
		cmd.SetArgs([]string{"--uid", "42"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("read error = %v", err)
		}
	})
	for _, want := range []string{"Subject: Hello", "From: sender@example.com", "This is the body."} {
		if !strings.Contains(out, want) {
			t.Errorf("read output missing %q:\n%s", want, out)
		}
	}
}

func TestReadCommandPreviewSkipsFullBody(t *testing.T) {
	fr := &fakeReader{message: sampleMessage()}
	swapReadReader(t, fr)

	out := captureStdout(t, func() {
		cmd := newReadCmd()
		cmd.SetArgs([]string{"--uid", "42", "--raw"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("read --raw error = %v", err)
		}
	})
	// In raw mode the body is intentionally empty; the command prints the preview.
	if !strings.Contains(out, "This is the body.") {
		t.Errorf("preview output missing body preview:\n%s", out)
	}
	if fr.calls != 1 {
		t.Errorf("preview called reader %d times, want 1", fr.calls)
	}
}

func TestReadCommandJSONOutput(t *testing.T) {
	swapReadReader(t, &fakeReader{message: sampleMessage()})
	jsonOutput = true
	t.Cleanup(func() { jsonOutput = false })

	out := captureStdout(t, func() {
		cmd := newReadCmd()
		cmd.SetArgs([]string{"--uid", "42"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("read json error = %v", err)
		}
	})

	var resp emailtypes.ReadResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("read JSON invalid: %v\n%s", err, out)
	}
	if !resp.Success || resp.Message.UID != 42 || resp.Message.Body != "This is the body." {
		t.Errorf("read JSON = %+v, want success with body", resp)
	}
}

func TestReadCommandErrorPropagation(t *testing.T) {
	swapReadReader(t, &fakeReader{err: errFake})
	if err := newReadCmd().Execute(); err == nil {
		t.Fatal("read error = nil, want propagated error")
	}
}
