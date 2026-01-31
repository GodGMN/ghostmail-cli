// Package email provides email sending functionality.
package email

import (
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/GodGMN/ghostmail-cli/internal/config"
	"gopkg.in/gomail.v2"
)

// Sender handles email sending operations.
type Sender struct {
	config *config.SMTPConfig
}

// NewSender creates a new email sender.
func NewSender(cfg *config.SMTPConfig) *Sender {
	return &Sender{config: cfg}
}

// Send sends an email message.
func (s *Sender) Send(to []string, subject, body string, opts ...SendOption) error {
	if len(to) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}

	m := gomail.NewMessage()

	from := s.config.From
	if from == "" {
		from = s.config.Username
	}

	m.SetHeader("From", from)
	m.SetHeader("To", to...)
	m.SetHeader("Subject", subject)

	// Apply options
	options := &sendOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Set CC recipients
	if len(options.cc) > 0 {
		m.SetHeader("Cc", options.cc...)
	}

	// Set BCC recipients
	if len(options.bcc) > 0 {
		m.SetHeader("Bcc", options.bcc...)
	}

	// Set In-Reply-To header for threading
	if options.inReplyTo != "" {
		m.SetHeader("In-Reply-To", options.inReplyTo)
	}

	// Set References header for proper threading
	if len(options.references) > 0 {
		m.SetHeader("References", options.references...)
	}

	// Set custom headers
	for key, value := range options.headers {
		m.SetHeader(key, value)
	}

	// Set body content
	if options.htmlBody != "" {
		m.SetBody("text/html", options.htmlBody)
		if body != "" {
			m.AddAlternative("text/plain", body)
		}
	} else {
		m.SetBody("text/plain", body)
	}

	// Attach files
	for _, attachment := range options.attachments {
		m.Attach(attachment)
	}

	// Create dialer
	d := gomail.NewDialer(s.config.Host, s.config.Port, s.config.Username, s.config.Password)

	if s.config.UseTLS {
		d.SSL = true
	} else if s.config.StartTLS {
		d.TLSConfig = &tls.Config{ServerName: s.config.Host}
	}

	// Send the email
	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// sendOptions holds optional parameters for Send.
type sendOptions struct {
	cc          []string
	bcc         []string
	htmlBody    string
	attachments []string
	headers     map[string]string
	inReplyTo   string   // Message-ID being replied to
	references  []string // Chain of Message-IDs for threading
}

// SendOption is a function that configures send options.
type SendOption func(*sendOptions)

// WithCC adds CC recipients.
func WithCC(cc []string) SendOption {
	return func(o *sendOptions) {
		o.cc = cc
	}
}

// WithBCC adds BCC recipients.
func WithBCC(bcc []string) SendOption {
	return func(o *sendOptions) {
		o.bcc = bcc
	}
}

// WithHTMLBody sets the HTML body.
func WithHTMLBody(html string) SendOption {
	return func(o *sendOptions) {
		o.htmlBody = html
	}
}

// WithAttachments adds file attachments.
func WithAttachments(files []string) SendOption {
	return func(o *sendOptions) {
		o.attachments = files
	}
}

// WithHeaders adds custom headers.
func WithHeaders(headers map[string]string) SendOption {
	return func(o *sendOptions) {
		o.headers = headers
	}
}

// WithInReplyTo sets the In-Reply-To header for threading.
func WithInReplyTo(messageID string) SendOption {
	return func(o *sendOptions) {
		o.inReplyTo = messageID
	}
}

// WithReferences sets the References header for threading.
func WithReferences(refs []string) SendOption {
	return func(o *sendOptions) {
		o.references = refs
	}
}

// FormatQuotedReply formats a reply body with proper quoting.
// Returns: replyBody + attribution + quoted original
func FormatQuotedReply(replyBody, originalBody, from, date string) string {
	var result strings.Builder

	// Add the reply body
	if replyBody != "" {
		result.WriteString(replyBody)
		result.WriteString("\n\n")
	}

	// Add attribution line
	result.WriteString("On ")
	result.WriteString(date)
	result.WriteString(", ")
	result.WriteString(from)
	result.WriteString(" wrote:\n")

	// Quote the original body
	lines := strings.Split(originalBody, "\n")
	for _, line := range lines {
		result.WriteString("> ")
		result.WriteString(line)
		result.WriteString("\n")
	}

	return strings.TrimRight(result.String(), "\n")
}
