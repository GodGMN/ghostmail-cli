package email

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/backend/memory"
	imapserver "github.com/emersion/go-imap/server"

	"github.com/GodGMN/ghostmail-cli/internal/config"
)

// startIMAPServer launches an in-process IMAP server backed by the reference
// in-memory backend. STARTTLS is advertised because a TLSConfig is set, which
// lets us exercise the same code path that real providers use. The returned
// config is wired to dial the server over plaintext and upgrade via STARTTLS.
// The trust pool returned alongside must be injected into the Reader so its
// STARTTLS handshake can verify the self-signed test certificate.
func startIMAPServer(t *testing.T) (*config.IMAPConfig, *x509.CertPool) {
	t.Helper()
	be := memory.New()
	srv := imapserver.New(be)
	srv.AllowInsecureAuth = true
	cert := testCertificate(t)
	srv.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	go func() { _ = srv.Serve(listener) }()

	_, port, _ := net.SplitHostPort(listener.Addr().String())
	portNumber, _ := strconv.Atoi(port)
	pool := x509.NewCertPool()
	parsed, _ := x509.ParseCertificate(cert.Certificate[0])
	pool.AddCert(parsed)

	cfg := &config.IMAPConfig{
		Host:     "127.0.0.1",
		Port:     portNumber,
		Username: "username",
		Password: "password",
		StartTLS: true,
	}
	return cfg, pool
}

// newTestReader wires the trust pool into a Reader so STARTTLS verification
// succeeds against the self-signed test certificate.
func newTestReader(t *testing.T, cfg *config.IMAPConfig, pool *x509.CertPool) *Reader {
	t.Helper()
	r := NewReader(cfg)
	r.tlsRoots = pool
	return r
}

func TestReaderListMessagesOverStartTLS(t *testing.T) {
	cfg, pool := startIMAPServer(t)
	cfg.Mailbox = "INBOX"

	messages, err := newTestReader(t, cfg, pool).ListMessages(0, false)
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("ListMessages() returned %d messages, want 1", len(messages))
	}
	msg := messages[0]
	if msg.UID != 6 {
		t.Errorf("UID = %d, want 6", msg.UID)
	}
	if msg.Subject != "A little message, just for you" {
		t.Errorf("Subject = %q", msg.Subject)
	}
	if !strings.HasSuffix(msg.From, "@example.org") {
		t.Errorf("From = %q, want example.org sender", msg.From)
	}
}

func TestReaderListMessagesUnreadFilter(t *testing.T) {
	cfg, pool := startIMAPServer(t)
	cfg.Mailbox = "INBOX"

	// The seed message carries the \Seen flag, so an unread-only filter should
	// return zero messages.
	messages, err := newTestReader(t, cfg, pool).ListMessages(0, true)
	if err != nil {
		t.Fatalf("ListMessages(unread) error = %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("ListMessages(unread) = %d messages, want 0 (seed is \\Seen)", len(messages))
	}
}

func TestReaderReadMessageOverStartTLS(t *testing.T) {
	cfg, pool := startIMAPServer(t)
	cfg.Mailbox = "INBOX"

	msg, err := newTestReader(t, cfg, pool).ReadMessage(6)
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if msg.MessageID != "<0000000@localhost/>" {
		t.Errorf("MessageID = %q", msg.MessageID)
	}
	if !strings.Contains(msg.Body, "Hi there :)") {
		t.Errorf("Body = %q, want parsed body", msg.Body)
	}
	if msg.BodyPreview == "" {
		t.Error("BodyPreview is empty, want a generated preview")
	}
}

func TestReaderReadMessagePreviewOverStartTLS(t *testing.T) {
	cfg, pool := startIMAPServer(t)
	cfg.Mailbox = "INBOX"

	msg, err := newTestReader(t, cfg, pool).ReadMessagePreview(6)
	if err != nil {
		t.Fatalf("ReadMessagePreview() error = %v", err)
	}
	// Preview path must not return the full body but should populate a preview
	// and the envelope.
	if msg.Body != "" {
		t.Errorf("Preview Body = %q, want empty (preview must not return full body)", msg.Body)
	}
	if msg.BodyPreview == "" {
		t.Error("BodyPreview is empty, want a generated preview from the prefix")
	}
	if msg.Subject != "A little message, just for you" {
		t.Errorf("Subject = %q, want envelope subject", msg.Subject)
	}
}

func TestReaderReadMessageNotFound(t *testing.T) {
	cfg, pool := startIMAPServer(t)
	cfg.Mailbox = "INBOX"

	_, err := newTestReader(t, cfg, pool).ReadMessage(99999)
	if err == nil {
		t.Errorf("ReadMessage(unknown UID) error = nil, want fetch/not-found error")
	}
}

func TestReaderConnectRejectsBadCredentials(t *testing.T) {
	cfg, pool := startIMAPServer(t)
	cfg.Mailbox = "INBOX"
	cfg.Password = "wrong"

	_, err := newTestReader(t, cfg, pool).Connect()
	if err == nil || !strings.Contains(err.Error(), "login") {
		t.Errorf("Connect() with bad password error = %v, want login failure", err)
	}
}

// Sanity check that the envelope date parsed from the message's Date header is
// surfaced correctly (the memory backend seeds a fixed 2016 date in the body).
func TestReaderListMessagesReturnsEnvelopeDate(t *testing.T) {
	cfg, pool := startIMAPServer(t)
	cfg.Mailbox = "INBOX"

	messages, err := newTestReader(t, cfg, pool).ListMessages(0, false)
	if err != nil || len(messages) != 1 {
		t.Fatalf("ListMessages() = %v, %d", err, len(messages))
	}
	want := time.Date(2016, time.May, 11, 14, 31, 59, 0, time.UTC)
	if !messages[0].Date.Equal(want) {
		t.Errorf("Date = %v, want parsed envelope date %v", messages[0].Date, want)
	}
}
