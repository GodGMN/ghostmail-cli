// Package email provides email reading functionality via IMAP.
package email

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"

	"github.com/GodGMN/ghostmail-cli/internal/config"
	emailtypes "github.com/GodGMN/ghostmail-cli/pkg/email"
)

// wordDecoder decodes RFC 2047 encoded-words (e.g. =?utf-8?B?...?=) found in
// header fields such as display names. It is safe to share across goroutines:
// mime.WordDecoder holds no mutable state.
var wordDecoder = newMimeWordDecoder()

func newMimeWordDecoder() *mime.WordDecoder {
	d := mime.WordDecoder{}
	return &d
}

// decodeHeader decodes an RFC 2047 encoded header value. It returns the input
// unchanged if decoding fails, so malformed encoded-words never corrupt output.
func decodeHeader(s string) string {
	if !strings.Contains(s, "=?") {
		return s
	}
	if decoded, err := wordDecoder.DecodeHeader(s); err == nil {
		return decoded
	}
	return s
}

// safeSlicePrefix returns the longest prefix of s that is at most maxBytes long
// and ends on a UTF-8 rune boundary. It never splits a multibyte sequence, so
// the result is always valid UTF-8 even when maxBytes lands mid-rune.
func safeSlicePrefix(s string, maxBytes int) string {
	if maxBytes <= 0 || s == "" {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	end := maxBytes
	// Back up to the nearest rune-start byte so we don't truncate inside a
	// multibyte codepoint.
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end]
}

// safeSliceRunes returns the first n runes of s. Use this instead of s[:n]
// whenever n is a display-width budget measured in characters, since byte
// slicing would corrupt multibyte text.
func safeSliceRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

const (
	maxMessageBytes    = 10 << 20
	maxBodyBytes       = 2 << 20
	maxAttachmentBytes = 8 << 20
	// previewMessageBytes is the maximum number of body bytes fetched in
	// preview (--raw) mode. It is intentionally small so that listing a
	// preview is cheap even when the full message is very large.
	previewMessageBytes = 4 << 10

	// connectTimeout bounds the TCP dial and initial IMAP greeting so a
	// stalled server cannot hang the CLI indefinitely at connect time.
	connectTimeout = 15 * time.Second
	// commandTimeout bounds each individual IMAP command. It is generous
	// enough for large body fetches while still preventing an unresponsive
	// server from blocking the CLI forever.
	commandTimeout = 60 * time.Second
)

var (
	errMessageTooLarge    = errors.New("message exceeds maximum size")
	errBodyTooLarge       = errors.New("message body exceeds maximum size")
	errAttachmentTooLarge = errors.New("attachment exceeds maximum size")
)

// Reader handles email reading operations via IMAP.
type Reader struct {
	config *config.IMAPConfig
	// tlsRoots, when non-nil, overrides the system root certificate pool used
	// to verify the IMAP server. It is intentionally unexported so only tests
	// in this package can inject a custom trust store; production callers rely
	// on the host's root CA set, which is the secure default.
	tlsRoots *x509.CertPool
}

// NewReader creates a new email reader.
func NewReader(cfg *config.IMAPConfig) *Reader {
	return &Reader{config: cfg}
}

// tlsConfig returns the TLS configuration for IMAP connections. When tlsRoots
// is set (tests only), it is used as the trust store; otherwise the system
// roots are used, matching the production default.
func (r *Reader) tlsConfig() *tls.Config {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: r.config.Host}
	if r.tlsRoots != nil {
		cfg.RootCAs = r.tlsRoots
	}
	return cfg
}

// Connect establishes a connection to the IMAP server.
func (r *Reader) Connect() (*client.Client, error) {
	addr := net.JoinHostPort(r.config.Host, strconv.Itoa(r.config.Port))
	if !r.config.UseTLS && !r.config.StartTLS && !r.config.AllowInsecure {
		return nil, fmt.Errorf("refusing to send IMAP credentials without TLS or STARTTLS (set GHOSTMAIL_IMAP_ALLOW_INSECURE=true only for an explicitly trusted plaintext server)")
	}

	// A dialer timeout bounds both the TCP connection and the greeting wait
	// (the go-imap client applies it as a connection deadline until the first
	// command clears it).
	dialer := &net.Dialer{Timeout: connectTimeout}

	var c *client.Client
	var err error

	if r.config.UseTLS {
		c, err = client.DialWithDialerTLS(dialer, addr, r.tlsConfig())
	} else {
		c, err = client.DialWithDialer(dialer, addr)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP server: %w", err)
	}
	// Bound individual commands so a server that stops responding mid-session
	// cannot hang the CLI.
	c.Timeout = commandTimeout
	if r.config.StartTLS && !r.config.UseTLS {
		if err := c.StartTLS(r.tlsConfig()); err != nil {
			c.Logout()
			return nil, fmt.Errorf("failed to start IMAP TLS: %w", err)
		}
	}

	if err := c.Login(r.config.Username, r.config.Password); err != nil {
		c.Logout()
		return nil, fmt.Errorf("failed to login: %w", err)
	}

	return c, nil
}

// ListMessages retrieves messages from the inbox.
func (r *Reader) ListMessages(limit int, unreadOnly bool) ([]emailtypes.Message, error) {
	c, err := r.Connect()
	if err != nil {
		return nil, err
	}
	defer c.Logout()

	// Select mailbox
	mbox, err := c.Select(r.config.Mailbox, false)
	if err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	if mbox.Messages == 0 {
		return []emailtypes.Message{}, nil
	}

	// Build search criteria
	var criteria imap.SearchCriteria
	if unreadOnly {
		criteria.WithoutFlags = []string{imap.SeenFlag}
	}

	var uids []uint32
	if unreadOnly {
		uids, err = c.UidSearch(&criteria)
		if err != nil {
			return nil, fmt.Errorf("failed to search messages: %w", err)
		}
	} else {
		// Search for all messages (UIDs are not necessarily sequential)
		allCriteria := &imap.SearchCriteria{}
		uids, err = c.UidSearch(allCriteria)
		if err != nil {
			return nil, fmt.Errorf("failed to search messages: %w", err)
		}
	}

	if len(uids) == 0 {
		return []emailtypes.Message{}, nil
	}

	// Apply limit
	if limit > 0 && len(uids) > limit {
		// Get the most recent messages
		start := len(uids) - limit
		uids = uids[start:]
	}

	// Fetch messages
	seqSet := new(imap.SeqSet)
	seqSet.AddNum(uids...)

	items := []imap.FetchItem{
		imap.FetchUid,
		imap.FetchEnvelope,
		imap.FetchFlags,
		imap.FetchRFC822Size,
	}

	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)

	go func() {
		done <- c.UidFetch(seqSet, items, messages)
	}()

	var result []emailtypes.Message
	for msg := range messages {
		emsg := r.convertMessage(msg)
		result = append(result, emsg)
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	return result, nil
}

// ReadMessage retrieves a specific message by UID.
func (r *Reader) ReadMessage(uid uint32) (*emailtypes.Message, error) {
	c, err := r.Connect()
	if err != nil {
		return nil, err
	}
	defer c.Logout()

	// Select mailbox
	_, err = c.Select(r.config.Mailbox, false)
	if err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	// Fetch message
	seqSet := new(imap.SeqSet)
	seqSet.AddNum(uid)

	items := []imap.FetchItem{
		imap.FetchUid,
		imap.FetchEnvelope,
		imap.FetchFlags,
		imap.FetchRFC822Size,
	}

	// Request only enough data to detect messages that exceed the parsing limit.
	// Note: BODY[] (FetchBody) is deliberately NOT requested alongside the
	// partial section; requesting both makes servers transfer the full message
	// twice. The partial section alone caps the transfer at maxMessageBytes.
	section := &imap.BodySectionName{Peek: true, Partial: []int{0, maxMessageBytes + 1}}
	items = append(items, section.FetchItem())

	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)

	go func() {
		done <- c.UidFetch(seqSet, items, messages)
	}()

	var result *emailtypes.Message
	var parseErr error
	for msg := range messages {
		emsg := r.convertMessage(msg)

		// Extract body and Message-ID
		if sectionData := msg.GetBody(section); sectionData != nil {
			if msg.Size > maxMessageBytes || sectionData.Len() > maxMessageBytes {
				parseErr = errMessageTooLarge
			} else {
				body, messageID, references, attachments, err := r.extractBody(sectionData)
				if err != nil {
					parseErr = err
				} else {
					emsg.Body = body
					emsg.MessageID = messageID
					emsg.References = references
					emsg.Attachments = attachments
					// Create preview
					emsg.BodyPreview = r.createPreview(body, 200)
				}
			}
		}

		result = &emsg
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("failed to fetch message: %w", err)
	}
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse message: %w", parseErr)
	}

	if result == nil {
		return nil, fmt.Errorf("message not found")
	}

	return result, nil
}

// ReadMessagePreview retrieves only envelope metadata and a short body prefix
// for a message. It is the fast path used by --raw: it never fetches the full
// message body and therefore avoids full MIME parsing. A best-effort preview is
// produced from the prefix; MIME decoding errors are tolerated so that headers
// are still returned.
func (r *Reader) ReadMessagePreview(uid uint32) (*emailtypes.Message, error) {
	c, err := r.Connect()
	if err != nil {
		return nil, err
	}
	defer c.Logout()

	if _, err := c.Select(r.config.Mailbox, false); err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddNum(uid)

	items := []imap.FetchItem{
		imap.FetchUid,
		imap.FetchEnvelope,
		imap.FetchFlags,
		imap.FetchRFC822Size,
	}
	section := &imap.BodySectionName{Peek: true, Partial: []int{0, previewMessageBytes}}
	items = append(items, section.FetchItem())

	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() {
		done <- c.UidFetch(seqSet, items, messages)
	}()

	var result *emailtypes.Message
	for msg := range messages {
		emsg := r.convertMessage(msg)
		if sectionData := msg.GetBody(section); sectionData != nil {
			// Preview mode tolerates parse failures: the envelope is still useful.
			if body, _, _, _, err := r.extractBody(sectionData); err == nil {
				emsg.BodyPreview = r.createPreview(body, 200)
			}
		}
		result = &emsg
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("failed to fetch message: %w", err)
	}
	if result == nil {
		return nil, fmt.Errorf("message not found")
	}
	return result, nil
}

// convertMessage converts an IMAP message envelope into our Message type. It
// only reads envelope metadata; body extraction is handled separately by the
// caller so that the preview and full-fetch paths can share this logic.
func (r *Reader) convertMessage(msg *imap.Message) emailtypes.Message {
	emsg := emailtypes.Message{
		UID:    msg.Uid,
		SeqNum: msg.SeqNum,
		Flags:  msg.Flags,
	}

	if msg.Envelope != nil {
		emsg.Subject = msg.Envelope.Subject
		emsg.Date = msg.Envelope.Date

		if len(msg.Envelope.From) > 0 {
			emsg.From = r.formatAddress(msg.Envelope.From[0])
		}

		for _, addr := range msg.Envelope.To {
			emsg.To = append(emsg.To, r.formatAddress(addr))
		}

		for _, addr := range msg.Envelope.Cc {
			emsg.CC = append(emsg.CC, r.formatAddress(addr))
		}

		for _, addr := range msg.Envelope.Bcc {
			emsg.BCC = append(emsg.BCC, r.formatAddress(addr))
		}
	}

	return emsg
}

// formatAddress formats an IMAP address. Display names are RFC 2047-decoded so
// that internationalized (=?utf-8?B?...?=) senders render as readable text
// instead of raw encoded-words.
func (r *Reader) formatAddress(addr *imap.Address) string {
	if addr == nil {
		return ""
	}
	if addr.PersonalName != "" {
		return fmt.Sprintf("%s <%s@%s>", decodeHeader(addr.PersonalName), addr.MailboxName, addr.HostName)
	}
	return fmt.Sprintf("%s@%s", addr.MailboxName, addr.HostName)
}

// extractBody extracts the text body, Message-ID, References, and attachment
// metadata from an email message.
func (r *Reader) extractBody(reader io.Reader) (string, string, []string, []emailtypes.Attachment, error) {
	mr, err := mail.CreateReader(reader)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("create MIME reader: %w", err)
	}

	// Extract Message-ID from headers
	messageID := mr.Header.Get("Message-Id")
	if messageID == "" {
		messageID = mr.Header.Get("Message-ID")
	}

	// Extract the References header (RFC 5322 §3.6.4). It is a whitespace-
	// separated list of ancestor Message-IDs; we preserve it so replies can
	// extend the threading chain instead of starting a new one.
	var references []string
	if refs := strings.Fields(mr.Header.Get("References")); len(refs) > 0 {
		references = refs
	}

	var textBody string
	var htmlBody string
	var attachments []emailtypes.Attachment

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			return "", "", nil, nil, fmt.Errorf("read MIME part: %w", err)
		}

		switch h := part.Header.(type) {
		case *mail.InlineHeader:
			contentType, _, err := h.ContentType()
			if err != nil {
				return "", "", nil, nil, fmt.Errorf("read inline content type: %w", err)
			}
			data, err := readPart(part.Body, maxBodyBytes, errBodyTooLarge)
			if err != nil {
				return "", "", nil, nil, err
			}

			if strings.HasPrefix(strings.ToLower(contentType), "text/plain") {
				textBody = string(data)
			} else if strings.HasPrefix(strings.ToLower(contentType), "text/html") {
				htmlBody = string(data)
			}
		case *mail.AttachmentHeader:
			filename, err := h.Filename()
			if err != nil {
				return "", "", nil, nil, fmt.Errorf("read attachment filename: %w", err)
			}
			contentType, _, err := h.ContentType()
			if err != nil {
				return "", "", nil, nil, fmt.Errorf("read attachment content type: %w", err)
			}
			size, err := io.Copy(io.Discard, io.LimitReader(part.Body, maxAttachmentBytes+1))
			if err != nil {
				return "", "", nil, nil, fmt.Errorf("read attachment: %w", err)
			}
			if size > maxAttachmentBytes {
				return "", "", nil, nil, errAttachmentTooLarge
			}
			attachments = append(attachments, emailtypes.Attachment{
				Filename:    filename,
				ContentType: contentType,
				Size:        int(size),
			})
		}
	}

	// Prefer plain text, fallback to HTML
	if textBody != "" {
		return textBody, messageID, references, attachments, nil
	}
	if htmlBody != "" {
		return r.stripHTML(htmlBody), messageID, references, attachments, nil
	}

	return "", messageID, references, attachments, nil
}

func readPart(reader io.Reader, limit int64, limitErr error) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read message body: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, limitErr
	}
	return data, nil
}

// stripHTML removes HTML tags and returns plain text.
func (r *Reader) stripHTML(html string) string {
	// Go's regexp engine does not support backreferences, so match each element explicitly.
	scriptRe := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	html = scriptRe.ReplaceAllString(html, "")
	styleRe := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = styleRe.ReplaceAllString(html, "")

	// Replace <br>, <p> with newlines
	brRe := regexp.MustCompile(`(?i)<br\s*/?>`)
	html = brRe.ReplaceAllString(html, "\n")
	pRe := regexp.MustCompile(`(?i)</p>`)
	html = pRe.ReplaceAllString(html, "\n")

	// Remove all HTML tags
	tagRe := regexp.MustCompile(`<[^>]+>`)
	html = tagRe.ReplaceAllString(html, "")

	// Decode HTML entities (basic ones)
	html = strings.ReplaceAll(html, "&nbsp;", " ")
	html = strings.ReplaceAll(html, "&lt;", "<")
	html = strings.ReplaceAll(html, "&gt;", ">")
	html = strings.ReplaceAll(html, "&amp;", "&")
	html = strings.ReplaceAll(html, "&quot;", "\"")
	html = strings.ReplaceAll(html, "&#39;", "'")

	// Normalize whitespace
	wsRe := regexp.MustCompile(`\s+`)
	html = wsRe.ReplaceAllString(html, " ")

	return strings.TrimSpace(html)
}

// createPreview creates a preview of the body. It collapses blank lines and
// appends an ellipsis only when the preview was actually truncated, so a short
// message never gains a spurious "...".
func (r *Reader) createPreview(body string, maxLen int) string {
	lines := strings.Split(body, "\n")
	var preview []string
	currentLen := 0
	truncated := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if currentLen+len(line) > maxLen {
			remaining := maxLen - currentLen
			if remaining > 0 {
				// Truncate on a UTF-8 rune boundary so multibyte text is not
				// corrupted when the byte budget runs out mid-codepoint.
				preview = append(preview, safeSlicePrefix(line, remaining))
			}
			truncated = true
			break
		}
		preview = append(preview, line)
		currentLen += len(line) + 1
	}

	result := strings.Join(preview, " ")
	if truncated {
		result += "..."
	}
	return result
}
