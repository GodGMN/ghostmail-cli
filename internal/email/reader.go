// Package email provides email reading functionality via IMAP.
package email

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
	"github.com/GodGMN/ghostmail-cli/internal/config"
	emailtypes "github.com/GodGMN/ghostmail-cli/pkg/email"
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
	addr := fmt.Sprintf("%s:%d", r.config.Host, r.config.Port)

	var c *client.Client
	var err error

	if r.config.UseTLS {
		c, err = client.DialTLS(addr, nil)
	} else {
		c, err = client.Dial(addr)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP server: %w", err)
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

	section := &imap.BodySectionName{}
	items = append(items, section.FetchItem())

	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)

	go func() {
		done <- c.UidFetch(seqSet, items, messages)
	}()

	var result *emailtypes.Message
	for msg := range messages {
		emsg := r.convertMessage(msg, true)

		// Extract body and Message-ID
		if sectionData := msg.GetBody(section); sectionData != nil {
			body, messageID, err := r.extractBody(sectionData)
			if err == nil {
				emsg.Body = body
				emsg.MessageID = messageID
				// Create preview
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

// extractBody extracts the text body and Message-ID from an email message.
func (r *Reader) extractBody(reader io.Reader) (string, string, error) {
	mr, err := mail.CreateReader(reader)
	if err != nil {
		// Fallback: read raw
		data, err := io.ReadAll(reader)
		if err != nil {
			return "", "", err
		}
		return string(data), "", nil
	}

	// Extract Message-ID from headers
	messageID := mr.Header.Get("Message-Id")
	if messageID == "" {
		messageID = mr.Header.Get("Message-ID")
	}

	var textBody string
	var htmlBody string

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			continue
		}

		switch h := part.Header.(type) {
		case *mail.InlineHeader:
			contentType, _, _ := h.ContentType()
			data, _ := io.ReadAll(part.Body)

			if strings.HasPrefix(contentType, "text/plain") {
				textBody = string(data)
			} else if strings.HasPrefix(contentType, "text/html") {
				htmlBody = string(data)
			}
		}
	}

	// Prefer plain text, fallback to HTML
	if textBody != "" {
		return textBody, messageID, nil
	}
	if htmlBody != "" {
		return r.stripHTML(htmlBody), messageID, nil
	}

	return "", messageID, nil
}

// stripHTML removes HTML tags and returns plain text.
func (r *Reader) stripHTML(html string) string {
	// Remove script and style elements
	scriptRe := regexp.MustCompile(`(?i)<(script|style)[^>]*>[\s\S]*?</\1>`)
	html = scriptRe.ReplaceAllString(html, "")

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
