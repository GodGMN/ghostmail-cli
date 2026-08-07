package email

import (
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/emersion/go-imap"

	"github.com/GodGMN/ghostmail-cli/internal/config"
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
		"References: <parent@example.com> <grandparent@example.com>\r\n" +
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

	body, messageID, references, attachments, err := (&Reader{}).extractBody(message)
	if err != nil {
		t.Fatalf("extractBody() error = %v", err)
	}
	if body != "Message body" {
		t.Errorf("body = %q, want %q", body, "Message body")
	}
	if messageID != "<message@example.com>" {
		t.Errorf("messageID = %q, want %q", messageID, "<message@example.com>")
	}
	if want := []string{"<parent@example.com>", "<grandparent@example.com>"}; !sameStrings(references, want) {
		t.Errorf("references = %v, want %v", references, want)
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

	_, _, _, _, err := (&Reader{}).extractBody(message)
	if err == nil {
		t.Fatal("extractBody() error = nil, want MIME decoding error")
	}
}

func TestExtractBodyRejectsOversizedBody(t *testing.T) {
	message := strings.NewReader("Content-Type: text/plain\r\n\r\n" + strings.Repeat("a", maxBodyBytes+1))

	_, _, _, _, err := (&Reader{}).extractBody(message)
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

	_, _, _, _, err := (&Reader{}).extractBody(message)
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

	body, messageID, references, attachments, err := (&Reader{}).extractBody(message)
	if err != nil {
		t.Fatalf("extractBody() error = %v", err)
	}
	if body != "Plain version" || messageID != "<alternative@example.com>" || len(attachments) != 0 || references != nil {
		t.Errorf("extractBody() = (%q, %q, %v, %d attachments), want plain body, ID, no refs, no attachments", body, messageID, references, len(attachments))
	}
}

func TestExtractBodyConvertsHTMLWhenPlainTextIsAbsent(t *testing.T) {
	message := strings.NewReader("Content-Type: text/html\r\n\r\n" +
		"<style>.hidden { display: none; }</style><p>Hello&nbsp;&amp; welcome<br>again</p><script>alert('x')</script>")

	body, _, _, _, err := (&Reader{}).extractBody(message)
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

	got := r.convertMessage(msg)
	if got.UID != 42 || got.SeqNum != 3 || got.Subject != "Status" || !got.Date.Equal(date) {
		t.Errorf("converted message metadata = %+v, want envelope metadata", got)
	}
	if got.From != "Sender <sender@example.com>" || !sameStrings(got.To, []string{"recipient@example.com", ""}) {
		t.Errorf("converted addresses = from %q to %q", got.From, got.To)
	}

	empty := r.convertMessage(&imap.Message{Uid: 7})
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

func TestCreatePreviewTruncatesOnRuneBoundary(t *testing.T) {
	r := &Reader{}
	// "éé" is two 2-byte runes (4 bytes total). A 3-byte budget lands inside the
	// second rune; before the rune-safe fix this produced invalid UTF-8
	// (0xC3 0xA9 0xC3 — a dangling leading byte). It must instead emit one whole
	// rune followed by the ellipsis.
	got := r.createPreview("éé", 3)
	if !utf8.ValidString(got) {
		t.Errorf("createPreview() produced invalid UTF-8: %q", got)
	}
	if got != "é..." {
		t.Errorf("createPreview() = %q, want %q", got, "é...")
	}
}

func TestSafeSlicePrefixNeverSplitsMultibyte(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"empty", "", 4, ""},
		{"nonpositive budget", "abc", 0, ""},
		{"fits exactly", "abc", 3, "abc"},
		{"ascii truncate", "abcdef", 3, "abc"},
		{"budget inside single multibyte backs to empty", "é", 1, ""}, // é = bytes 0,1; max=1 lands mid-rune
		{"partial budget across two multibyte", "éé", 3, "é"},         // max=3 lands in 2nd é → back to byte 2
		{"whole CJK runes", "你好世界", 6, "你好"},                          // each CJK rune is 3 bytes
		{"mid CJK rune backs up", "你好世界", 7, "你好"},                    // 7 lands in 3rd char → back to 6
		{"budget past end", "abc", 99, "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safeSlicePrefix(tt.in, tt.max)
			if !utf8.ValidString(got) {
				t.Errorf("safeSlicePrefix() returned invalid UTF-8: %q", got)
			}
			if got != tt.want {
				t.Errorf("safeSlicePrefix(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
			}
		})
	}
}

func TestSafeSliceRunes(t *testing.T) {
	if got := safeSliceRunes("你好世界", 2); got != "你好" {
		t.Errorf("safeSliceRunes() = %q, want %q", got, "你好")
	}
	if got := safeSliceRunes("你好世界", 99); got != "你好世界" {
		t.Errorf("safeSliceRunes() = %q, want full string", got)
	}
	if got := safeSliceRunes("abc", 0); got != "" {
		t.Errorf("safeSliceRunes() = %q, want empty", got)
	}
}

func TestDecodeHeaderHandlesRFC2047EncodedWords(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain ascii passthrough", "Alice", "Alice"},
		{"no encoded-word marker", "Alice <alice>", "Alice <alice>"},
		{"UTF-8 Base64 encoded word", "=?utf-8?B?0JzQsNGI0LA=?=", "Маша"},
		{"UTF-8 Quoted-printable encoded word", "=?UTF-8?Q?Caf=C3=A9?=", "Café"},
		{"ISO-8859-1 QP", "=?iso-8859-1?Q?Keld_J=F8rn_Simonsen?=", "Keld Jørn Simonsen"},
		{"malformed encoded word falls back", "=?bogus??invalid", "=?bogus??invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeHeader(tt.in)
			if got != tt.want {
				t.Errorf("decodeHeader(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("decodeHeader(%q) produced invalid UTF-8: %q", tt.in, got)
			}
		})
	}
}

func TestFormatAddressDecodesInternationalDisplayName(t *testing.T) {
	r := &Reader{}
	// RFC 2047 encoded UTF-8 display name (Base64 of "Маша").
	addr := &imap.Address{
		PersonalName: "=?utf-8?B?0JzQsNGI0LA=?=",
		MailboxName:  "masha",
		HostName:     "example.com",
	}
	got := r.formatAddress(addr)
	if !strings.Contains(got, "Маша") {
		t.Errorf("formatAddress() = %q, want decoded display name Маша", got)
	}
	if !strings.Contains(got, "<masha@example.com>") {
		t.Errorf("formatAddress() = %q, want mailbox address", got)
	}
}

func TestFormatAddressHandlesNilAndNoDisplayName(t *testing.T) {
	r := &Reader{}
	if got := r.formatAddress(nil); got != "" {
		t.Errorf("formatAddress(nil) = %q, want empty", got)
	}
	addr := &imap.Address{MailboxName: "bob", HostName: "example.com"}
	if got := r.formatAddress(addr); got != "bob@example.com" {
		t.Errorf("formatAddress() = %q, want bob@example.com", got)
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
