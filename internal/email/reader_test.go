package email

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/GodGMN/ghostmail-cli/internal/config"
	"github.com/emersion/go-imap"
)

func TestConnectRejectsPlaintextCredentials(t *testing.T) {
	reader := NewReader(&config.IMAPConfig{})

	_, err := reader.Connect()
	if err == nil || !strings.Contains(err.Error(), "GHOSTMAIL_IMAP_ALLOW_INSECURE") {
		t.Errorf("Connect() error = %v, want plaintext credential rejection", err)
	}
}

func TestExtractBodyIncludesAttachmentMetadata(t *testing.T) {
	message := strings.NewReader("From: sender@example.com\r\n" +
		"Message-ID: <message@example.com>\r\n" +
		"Content-Type: multipart/mixed; boundary=boundary\r\n\r\n" +
		"--boundary\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n\r\n" +
		"Message body\r\n" +
		"--boundary\r\n" +
		"Content-Type: application/pdf\r\n" +
		"Content-Disposition: attachment; filename=report.pdf\r\n" +
		"Content-Transfer-Encoding: base64\r\n\r\n" +
		"SGVsbG8=\r\n" +
		"--boundary--\r\n")

	body, messageID, attachments, err := (&Reader{}).extractBody(message)
	if err != nil {
		t.Fatalf("extractBody() error = %v", err)
	}
	if body != "Message body" {
		t.Errorf("body = %q, want %q", body, "Message body")
	}
	if messageID != "<message@example.com>" {
		t.Errorf("messageID = %q, want %q", messageID, "<message@example.com>")
	}
	if len(attachments) != 1 {
		t.Fatalf("attachments = %d, want 1", len(attachments))
	}

	attachment := attachments[0]
	if attachment.Filename != "report.pdf" {
		t.Errorf("attachment filename = %q, want %q", attachment.Filename, "report.pdf")
	}
	if attachment.ContentType != "application/pdf" {
		t.Errorf("attachment content type = %q, want %q", attachment.ContentType, "application/pdf")
	}
	if attachment.Size != 5 {
		t.Errorf("attachment size = %d, want 5", attachment.Size)
	}
}

func TestExtractBodyReturnsUnreadableAttachmentError(t *testing.T) {
	message := strings.NewReader("Content-Type: multipart/mixed; boundary=boundary\r\n\r\n" +
		"--boundary\r\n" +
		"Content-Type: text/plain\r\n\r\n" +
		"Message body\r\n" +
		"--boundary\r\n" +
		"Content-Type: application/octet-stream\r\n" +
		"Content-Disposition: attachment; filename=broken.bin\r\n" +
		"Content-Transfer-Encoding: base64\r\n\r\n" +
		"not valid base64!\r\n" +
		"--boundary--\r\n")

	_, _, _, err := (&Reader{}).extractBody(message)
	if err == nil {
		t.Fatal("extractBody() error = nil, want MIME decoding error")
	}
}

func TestExtractBodyRejectsOversizedBody(t *testing.T) {
	message := strings.NewReader("Content-Type: text/plain\r\n\r\n" + strings.Repeat("a", maxBodyBytes+1))

	_, _, _, err := (&Reader{}).extractBody(message)
	if !errors.Is(err, errBodyTooLarge) {
		t.Errorf("extractBody() error = %v, want %v", err, errBodyTooLarge)
	}
}

func TestExtractBodyRejectsOversizedAttachment(t *testing.T) {
	message := strings.NewReader("Content-Type: multipart/mixed; boundary=boundary\r\n\r\n" +
		"--boundary\r\n" +
		"Content-Type: application/octet-stream\r\n" +
		"Content-Disposition: attachment; filename=large.bin\r\n\r\n" +
		strings.Repeat("a", maxAttachmentBytes+1) + "\r\n--boundary--\r\n")

	_, _, _, err := (&Reader{}).extractBody(message)
	if !errors.Is(err, errAttachmentTooLarge) {
		t.Errorf("extractBody() error = %v, want %v", err, errAttachmentTooLarge)
	}
}

func TestExtractBodyPrefersPlainTextOverHTML(t *testing.T) {
	message := strings.NewReader("Message-ID: <alternative@example.com>\r\n" +
		"Content-Type: multipart/alternative; boundary=boundary\r\n\r\n" +
		"--boundary\r\nContent-Type: text/html\r\n\r\n<p>HTML version</p>\r\n" +
		"--boundary\r\nContent-Type: text/plain\r\n\r\nPlain version\r\n" +
		"--boundary--\r\n")

	body, messageID, attachments, err := (&Reader{}).extractBody(message)
	if err != nil {
		t.Fatalf("extractBody() error = %v", err)
	}
	if body != "Plain version" || messageID != "<alternative@example.com>" || len(attachments) != 0 {
		t.Errorf("extractBody() = (%q, %q, %d attachments), want plain body, ID, and no attachments", body, messageID, len(attachments))
	}
}

func TestExtractBodyConvertsHTMLWhenPlainTextIsAbsent(t *testing.T) {
	message := strings.NewReader("Content-Type: text/html\r\n\r\n" +
		"<style>.hidden { display: none; }</style><p>Hello&nbsp;&amp; welcome<br>again</p><script>alert('x')</script>")

	body, _, _, err := (&Reader{}).extractBody(message)
	if err != nil {
		t.Fatalf("extractBody() error = %v", err)
	}
	if body != "Hello & welcome again" {
		t.Errorf("HTML body = %q, want stripped HTML", body)
	}
}

func TestConvertMessageHandlesEnvelopeAddressesAndNilEnvelope(t *testing.T) {
	r := &Reader{}
	date := time.Date(2026, time.August, 7, 12, 30, 0, 0, time.UTC)
	msg := &imap.Message{
		Uid:    42,
		SeqNum: 3,
		Flags:  []string{imap.SeenFlag},
		Envelope: &imap.Envelope{
			Subject: "Status",
			Date:    date,
			From:    []*imap.Address{{PersonalName: "Sender", MailboxName: "sender", HostName: "example.com"}},
			To:      []*imap.Address{{MailboxName: "recipient", HostName: "example.com"}, nil},
		},
	}

	got := r.convertMessage(msg, false)
	if got.UID != 42 || got.SeqNum != 3 || got.Subject != "Status" || !got.Date.Equal(date) {
		t.Errorf("converted message metadata = %+v, want envelope metadata", got)
	}
	if got.From != "Sender <sender@example.com>" || !sameStrings(got.To, []string{"recipient@example.com", ""}) {
		t.Errorf("converted addresses = from %q to %q", got.From, got.To)
	}

	empty := r.convertMessage(&imap.Message{Uid: 7}, false)
	if empty.UID != 7 || empty.Subject != "" || empty.From != "" {
		t.Errorf("nil envelope conversion = %+v, want only UID", empty)
	}
}

func TestCreatePreviewSkipsBlankLinesAndTruncates(t *testing.T) {
	r := &Reader{}
	if got := r.createPreview(" first \n\nsecond", 20); got != "first second" {
		t.Errorf("createPreview() = %q, want collapsed non-empty lines", got)
	}
	if got := r.createPreview("abcdefgh", 5); got != "abcde..." {
		t.Errorf("createPreview() = %q, want truncated preview", got)
	}
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
