// Package email provides email sending functionality.
package email

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"
	"time"

	mail "github.com/wneessen/go-mail"

	"github.com/GodGMN/ghostmail-cli/internal/config"
)

// Sender handles email sending operations.
type Sender struct {
	config *config.SMTPConfig
	// tlsRoots, when non-nil, overrides the system root certificate pool used
	// to verify the SMTP server. It is intentionally unexported so only tests
	// in this package can inject a custom trust store; production callers rely
	// on the host's root CA set, which is the secure default.
	tlsRoots *x509.CertPool
}

// NewSender creates a new email sender.
func NewSender(cfg *config.SMTPConfig) *Sender {
	return &Sender{config: cfg}
}

// tlsConfig returns the TLS configuration for SMTP connections. When tlsRoots
// is set (tests only), it is used as the trust store; otherwise the system
// roots are used, matching the production default.
func (s *Sender) tlsConfig() *tls.Config {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: s.config.Host}
	if s.tlsRoots != nil {
		cfg.RootCAs = s.tlsRoots
	}
	return cfg
}

// Send sends an email message.
func (s *Sender) Send(to []string, subject, body string, opts ...SendOption) error {
	if len(to) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}
	if !s.config.UseTLS && !s.config.StartTLS && !s.config.AllowInsecure {
		return fmt.Errorf("refusing to send SMTP credentials without TLS or STARTTLS (set GHOSTMAIL_SMTP_ALLOW_INSECURE=true only for an explicitly trusted plaintext server)")
	}

	m := mail.NewMsg()

	from := s.config.From
	if from == "" {
		from = s.config.Username
	}

	if err := m.From(from); err != nil {
		return fmt.Errorf("invalid From address: %w", err)
	}
	if err := m.To(to...); err != nil {
		return fmt.Errorf("invalid To address: %w", err)
	}
	m.Subject(subject)

	// Apply options
	options := &sendOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Set CC recipients
	if len(options.cc) > 0 {
		if err := m.Cc(options.cc...); err != nil {
			return fmt.Errorf("invalid Cc address: %w", err)
		}
	}

	// Set BCC recipients
	if len(options.bcc) > 0 {
		if err := m.Bcc(options.bcc...); err != nil {
			return fmt.Errorf("invalid Bcc address: %w", err)
		}
	}

	// Set In-Reply-To header for threading
	if options.inReplyTo != "" {
		m.SetGenHeader(mail.HeaderInReplyTo, options.inReplyTo)
	}

	// Set References header for proper threading
	if len(options.references) > 0 {
		m.SetGenHeader(mail.HeaderReferences, options.references...)
	}

	// Set custom headers
	for key, value := range options.headers {
		m.SetGenHeader(mail.Header(key), value)
	}

	// Set body content
	if options.htmlBody != "" {
		m.SetBodyString(mail.TypeTextHTML, options.htmlBody)
		if body != "" {
			m.AddAlternativeString(mail.TypeTextPlain, body)
		}
	} else {
		m.SetBodyString(mail.TypeTextPlain, body)
	}

	// Attach files
	for _, attachment := range options.attachments {
		m.AttachFile(attachment)
	}

	tlsConfig := s.tlsConfig()
	client, err := newMailClient(s.config, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}

	// Send the email
	if err := client.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func newMailClient(cfg *config.SMTPConfig, tlsConfig *tls.Config) (*mail.Client, error) {
	tlsPolicy := mail.NoTLS
	if cfg.StartTLS && !cfg.UseTLS {
		tlsPolicy = mail.TLSMandatory
	}

	options := []mail.Option{
		mail.WithPort(cfg.Port),
		mail.WithTimeout(10 * time.Second),
		mail.WithTLSConfig(tlsConfig),
		mail.WithTLSPolicy(tlsPolicy),
		mail.WithoutNoop(),
		mail.WithoutRset(),
	}
	if cfg.UseTLS {
		options = append(options, mail.WithSSL())
	}
	if cfg.Username != "" {
		var authTypes []mail.SMTPAuthType
		if cfg.AllowInsecure && !cfg.UseTLS && !cfg.StartTLS {
			// Plaintext transport: prefer challenge-response mechanisms that do
			// not expose the password, falling back to cleartext only if the
			// server does not offer them.
			authTypes = []mail.SMTPAuthType{mail.SMTPAuthCramMD5, mail.SMTPAuthPlainNoEnc, mail.SMTPAuthLoginNoEnc}
		} else {
			// TLS-protected transport: PLAIN and LOGIN are the strong,
			// widely-supported choices. CRAM-MD5 (MD5-based, susceptible to
			// offline dictionary attacks) is only a last-resort fallback for
			// legacy servers that offer nothing else.
			authTypes = []mail.SMTPAuthType{mail.SMTPAuthPlain, mail.SMTPAuthLogin, mail.SMTPAuthCramMD5}
		}
		options = append(options,
			mail.WithUsername(cfg.Username),
			mail.WithPassword(cfg.Password),
			mail.WithOpportunisticSMTPAuth(authTypes...),
		)
	}
	return mail.NewClient(cfg.Host, options...)
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
