// Package email provides email reading functionality via IMAP.
package email

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/GodGMN/ghostmail-cli/internal/config"
	emailtypes "github.com/GodGMN/ghostmail-cli/pkg/email"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
)

const (
	maxMessageBytes    = 10 << 20
	maxBodyBytes       = 2 << 20
	maxAttachmentBytes = 8 << 20
)

var (
	errMessageTooLarge    = errors.New("message exceeds maximum size")
	errBodyTooLarge       = errors.New("message body exceeds maximum size")
	errAttachmentTooLarge = errors.New("attachment exceeds maximum size")
)

// Reader handles email reading operations via IMAP.
type Reader struct {
	config *config.IMAPConfig
}

// NewReader creates a new email reader.
func NewReader(cfg *config.IMAPConfig) *Reader {
	return &Reader{config: cfg}
}

// Connect establishes a connection to the IMAP server.
func (r *Reader) Connect() (*client.Client, error) {
	addr := net.JoinHostPort(r.config.Host, strconv.Itoa(r.config.Port))
	if !r.config.UseTLS && !r.config.StartTLS && !r.config.AllowInsecure {
		return nil, fmt.Errorf("refusing to send IMAP credentials without TLS or STARTTLS (set GHOSTMAIL_IMAP_ALLOW_INSECURE=true only for an explicitly trusted plaintext server)")
	}

	var c *client.Client
	var err error

	if r.config.UseTLS {
		c, err = client.DialTLS(addr, &tls.Config{MinVersion: tls.VersionTLS12, ServerName: r.config.Host})
	} else {
		c, err = client.Dial(addr)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP server: %w", err)
	}
	if r.config.StartTLS && !r.config.UseTLS {
		if err := c.StartTLS(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: r.config.Host}); err != nil {
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
		emsg := r.convertMessage(msg, false)
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
		imap.FetchBody,
	}

	// Request only enough data to detect messages that exceed the parsing limit.
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
		emsg := r.convertMessage(msg, true)

		// Extract body and Message-ID
		if sectionData := msg.GetBody(section); sectionData != nil {
			if msg.Size > maxMessageBytes || sectionData.Len() > maxMessageBytes {
				parseErr = errMessageTooLarge
			} else {
				body, messageID, attachments, err := r.extractBody(sectionData)
				if err != nil {
					parseErr = err
				} else {
					emsg.Body = body
					emsg.MessageID = messageID
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

// convertMessage converts an IMAP message to our Message type.
func (r *Reader) convertMessage(msg *imap.Message, fullBody bool) emailtypes.Message {
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

// formatAddress formats an IMAP address.
func (r *Reader) formatAddress(addr *imap.Address) string {
	if addr == nil {
		return ""
	}
	if addr.PersonalName != "" {
		return fmt.Sprintf("%s <%s@%s>", addr.PersonalName, addr.MailboxName, addr.HostName)
	}
	return fmt.Sprintf("%s@%s", addr.MailboxName, addr.HostName)
}

// extractBody extracts the text body, Message-ID, and attachment metadata from an email message.
func (r *Reader) extractBody(reader io.Reader) (string, string, []emailtypes.Attachment, error) {
	mr, err := mail.CreateReader(reader)
	if err != nil {
		return "", "", nil, fmt.Errorf("create MIME reader: %w", err)
	}

	// Extract Message-ID from headers
	messageID := mr.Header.Get("Message-Id")
	if messageID == "" {
		messageID = mr.Header.Get("Message-ID")
	}

	var textBody string
	var htmlBody string
	var attachments []emailtypes.Attachment

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			return "", "", nil, fmt.Errorf("read MIME part: %w", err)
		}

		switch h := part.Header.(type) {
		case *mail.InlineHeader:
			contentType, _, err := h.ContentType()
			if err != nil {
				return "", "", nil, fmt.Errorf("read inline content type: %w", err)
			}
			data, err := readPart(part.Body, maxBodyBytes, errBodyTooLarge)
			if err != nil {
				return "", "", nil, err
			}

			if strings.HasPrefix(strings.ToLower(contentType), "text/plain") {
				textBody = string(data)
			} else if strings.HasPrefix(strings.ToLower(contentType), "text/html") {
				htmlBody = string(data)
			}
		case *mail.AttachmentHeader:
			filename, err := h.Filename()
			if err != nil {
				return "", "", nil, fmt.Errorf("read attachment filename: %w", err)
			}
			contentType, _, err := h.ContentType()
			if err != nil {
				return "", "", nil, fmt.Errorf("read attachment content type: %w", err)
			}
			size, err := io.Copy(io.Discard, io.LimitReader(part.Body, maxAttachmentBytes+1))
			if err != nil {
				return "", "", nil, fmt.Errorf("read attachment: %w", err)
			}
			if size > maxAttachmentBytes {
				return "", "", nil, errAttachmentTooLarge
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
		return textBody, messageID, attachments, nil
	}
	if htmlBody != "" {
		return r.stripHTML(htmlBody), messageID, attachments, nil
	}

	return "", messageID, attachments, nil
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

// createPreview creates a preview of the body.
func (r *Reader) createPreview(body string, maxLen int) string {
	lines := strings.Split(body, "\n")
	var preview []string
	currentLen := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if currentLen+len(line) > maxLen {
			remaining := maxLen - currentLen
			if remaining > 0 {
				preview = append(preview, line[:remaining])
			}
			break
		}
		preview = append(preview, line)
		currentLen += len(line) + 1
	}

	result := strings.Join(preview, " ")
	if len(body) > maxLen {
		result += "..."
	}
	return result
}
