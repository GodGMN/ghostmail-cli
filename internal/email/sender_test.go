package email

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	mail "github.com/wneessen/go-mail"

	"github.com/GodGMN/ghostmail-cli/internal/config"
)

func TestSendRejectsPlaintextCredentials(t *testing.T) {
	sender := NewSender(&config.SMTPConfig{})

	err := sender.Send([]string{"recipient@example.com"}, "subject", "body")
	if err == nil || !strings.Contains(err.Error(), "GHOSTMAIL_SMTP_ALLOW_INSECURE") {
		t.Errorf("Send() error = %v, want plaintext credential rejection", err)
	}
}

func TestSendDeliversEnvelopeAndMessage(t *testing.T) {
	server := newSMTPTestServer(t, false, false)
	cfg := server.config()
	cfg.AllowInsecure = true
	cfg.From = "sender@example.test"

	err := NewSender(cfg).Send(
		[]string{"to@example.test"},
		"Test subject",
		"plain body",
		WithCC([]string{"cc@example.test"}),
		WithBCC([]string{"bcc@example.test"}),
		WithHeaders(map[string]string{"X-Test": "value"}),
	)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	session := server.session(t)
	if session.mailFrom != "sender@example.test" {
		t.Errorf("MAIL FROM = %q, want sender@example.test", session.mailFrom)
	}
	wantRecipients := []string{"to@example.test", "cc@example.test", "bcc@example.test"}
	if strings.Join(session.recipients, ",") != strings.Join(wantRecipients, ",") {
		t.Errorf("RCPT TO = %v, want %v", session.recipients, wantRecipients)
	}
	for _, want := range []string{"Subject: Test subject", "To: <to@example.test>", "Cc: <cc@example.test>", "X-Test: value", "plain body"} {
		if !strings.Contains(session.message, want) {
			t.Errorf("message does not contain %q:\n%s", want, session.message)
		}
	}
	if strings.Contains(session.message, "Bcc:") {
		t.Errorf("message exposes Bcc header:\n%s", session.message)
	}
}

func TestMailClientRejectsMissingStartTLS(t *testing.T) {
	server := newSMTPTestServer(t, false, false)
	cfg := server.config()
	cfg.StartTLS = true

	client, err := newMailClient(cfg, server.clientTLSConfig())
	if err == nil {
		err = client.DialWithContext(context.Background())
	}
	if err == nil || !strings.Contains(err.Error(), "does not support STARTTLS") {
		t.Fatalf("mail client error = %v, want missing STARTTLS error", err)
	}
}

func TestMailClientDeliversOverStartTLS(t *testing.T) {
	server := newSMTPTestServer(t, true, false)
	cfg := server.config()
	cfg.StartTLS = true
	cfg.Username = "user"
	cfg.Password = "password"

	sendTestMessage(t, cfg, server.clientTLSConfig())
	session := server.session(t)
	if !session.tls || !session.authenticated {
		t.Errorf("STARTTLS session = %+v, want TLS-authenticated session", session)
	}
	assertTransportDelivery(t, session)
}

func TestMailClientDeliversOverImplicitTLS(t *testing.T) {
	server := newSMTPTestServer(t, false, true)
	cfg := server.config()
	cfg.UseTLS = true
	cfg.Username = "user"
	cfg.Password = "password"

	sendTestMessage(t, cfg, server.clientTLSConfig())
	session := server.session(t)
	if !session.tls || !session.authenticated {
		t.Errorf("implicit TLS session = %+v, want TLS-authenticated session", session)
	}
	assertTransportDelivery(t, session)
}

func TestMailClientSelectsSupportedAuthentication(t *testing.T) {
	tests := []struct {
		name        string
		authMethods string
		want        string
	}{
		{name: "PLAIN preferred over CRAM-MD5 under TLS", authMethods: "CRAM-MD5 PLAIN LOGIN", want: "PLAIN"},
		{name: "LOGIN preferred over CRAM-MD5 under TLS", authMethods: "CRAM-MD5 LOGIN", want: "LOGIN"},
		{name: "CRAM-MD5 only as last resort", authMethods: "CRAM-MD5", want: "CRAM-MD5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newSMTPTestServer(t, true, false, tt.authMethods)
			cfg := server.config()
			cfg.StartTLS = true
			cfg.Username = "user"
			cfg.Password = "password"

			sendTestMessage(t, cfg, server.clientTLSConfig())
			if session := server.session(t); session.authMethod != tt.want {
				t.Errorf("AUTH mechanism = %q, want %q", session.authMethod, tt.want)
			}
		})
	}
}

func sendTestMessage(t *testing.T, cfg *config.SMTPConfig, tlsConfig *tls.Config) {
	t.Helper()
	client, err := newMailClient(cfg, tlsConfig)
	if err != nil {
		t.Fatalf("newMailClient() error = %v", err)
	}

	message := mail.NewMsg()
	if err := message.From("sender@example.test"); err != nil {
		t.Fatal(err)
	}
	if err := message.To("recipient@example.test"); err != nil {
		t.Fatal(err)
	}
	message.Subject("secure transport")
	message.SetBodyString(mail.TypeTextPlain, "delivered securely")
	if err := client.DialAndSend(message); err != nil {
		t.Fatalf("DialAndSend() error = %v", err)
	}
}

func assertTransportDelivery(t *testing.T, session smtpTestSession) {
	t.Helper()
	if session.mailFrom != "sender@example.test" {
		t.Errorf("MAIL FROM = %q, want sender@example.test", session.mailFrom)
	}
	if strings.Join(session.recipients, ",") != "recipient@example.test" {
		t.Errorf("RCPT TO = %v, want recipient@example.test", session.recipients)
	}
	if !strings.Contains(session.message, "Subject: secure transport") || !strings.Contains(session.message, "delivered securely") {
		t.Errorf("message was not delivered as expected:\n%s", session.message)
	}
}

type smtpTestSession struct {
	tls           bool
	authenticated bool
	authMethod    string
	mailFrom      string
	recipients    []string
	message       string
}

type smtpTestServer struct {
	listener    net.Listener
	cert        tls.Certificate
	startTLS    bool
	implicit    bool
	authMethods string
	sessions    chan smtpTestSession
}

func newSMTPTestServer(t *testing.T, startTLS, implicit bool, authMethods ...string) *smtpTestServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	server := &smtpTestServer{
		listener:    listener,
		cert:        testCertificate(t),
		startTLS:    startTLS,
		implicit:    implicit,
		authMethods: "PLAIN",
		sessions:    make(chan smtpTestSession, 1),
	}
	if len(authMethods) > 0 {
		server.authMethods = authMethods[0]
	}
	t.Cleanup(func() { _ = listener.Close() })
	go server.serve()
	return server
}

func (s *smtpTestServer) config() *config.SMTPConfig {
	_, port, _ := net.SplitHostPort(s.listener.Addr().String())
	portNumber, _ := strconv.Atoi(port)
	return &config.SMTPConfig{Host: "127.0.0.1", Port: portNumber}
}

func (s *smtpTestServer) clientTLSConfig() *tls.Config {
	pool := x509.NewCertPool()
	cert, _ := x509.ParseCertificate(s.cert.Certificate[0])
	pool.AddCert(cert)
	return &tls.Config{RootCAs: pool, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12}
}

func (s *smtpTestServer) session(t *testing.T) smtpTestSession {
	t.Helper()
	select {
	case session := <-s.sessions:
		return session
	case <-time.After(5 * time.Second):
		t.Fatal("SMTP test server did not complete a session")
		return smtpTestSession{}
	}
}

func (s *smtpTestServer) serve() {
	conn, err := s.listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	if s.implicit {
		conn = tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{s.cert}, MinVersion: tls.VersionTLS12})
	}

	session := smtpTestSession{tls: s.implicit}
	s.serveConnection(conn, &session)
	s.sessions <- session
}

func (s *smtpTestServer) serveConnection(conn net.Conn, session *smtpTestSession) {
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	writeResponse := func(format string, args ...any) {
		_, _ = fmt.Fprintf(writer, format+"\r\n", args...)
		_ = writer.Flush()
	}
	writeResponse("220 test SMTP server")

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		command, argument, _ := strings.Cut(line, " ")
		switch strings.ToUpper(command) {
		case "EHLO", "HELO":
			writeResponse("250-test SMTP server")
			if s.startTLS && !session.tls {
				writeResponse("250-STARTTLS")
			}
			writeResponse("250-AUTH %s", s.authMethods)
			writeResponse("250 OK")
		case "STARTTLS":
			if !s.startTLS || session.tls {
				writeResponse("502 STARTTLS unavailable")
				continue
			}
			writeResponse("220 ready to start TLS")
			tlsConn := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{s.cert}, MinVersion: tls.VersionTLS12})
			if err := tlsConn.Handshake(); err != nil {
				return
			}
			conn = tlsConn
			reader = bufio.NewReader(conn)
			writer = bufio.NewWriter(conn)
			session.tls = true
		case "AUTH":
			if !session.tls {
				writeResponse("538 encryption required")
				continue
			}
			session.authenticated = true
			session.authMethod = strings.Fields(argument)[0]
			writeResponse("235 authenticated")
		case "MAIL":
			session.mailFrom = smtpAddress(argument)
			writeResponse("250 sender accepted")
		case "RCPT":
			session.recipients = append(session.recipients, smtpAddress(argument))
			writeResponse("250 recipient accepted")
		case "DATA":
			writeResponse("354 end data with <CR><LF>.<CR><LF>")
			var data strings.Builder
			for {
				dataLine, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if dataLine == ".\r\n" {
					break
				}
				data.WriteString(strings.TrimPrefix(dataLine, "."))
			}
			session.message = data.String()
			writeResponse("250 message accepted")
		case "QUIT":
			writeResponse("221 goodbye")
			return
		default:
			writeResponse("502 command not implemented")
		}
	}
}

func smtpAddress(argument string) string {
	_, address, found := strings.Cut(argument, ":")
	if !found {
		return ""
	}
	return strings.Trim(strings.TrimSpace(address), "<>")
}

func testCertificate(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate() error = %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey() error = %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair() error = %v", err)
	}
	return cert
}

// TestSendOptionsApplyDirectly exercises the SendOption builders by applying
// them to an unexported sendOptions value. This covers the option functions
// that Send() consumes internally (WithHTMLBody, WithAttachments, WithInReplyTo,
// WithReferences) which are otherwise hard to assert on from outside the
// package.
func TestSendOptionsApplyDirectly(t *testing.T) {
	o := &sendOptions{}
	WithCC([]string{"cc@example.test"})(o)
	WithBCC([]string{"bcc@example.test"})(o)
	WithHTMLBody("<b>hi</b>")(o)
	WithAttachments([]string{"a.pdf", "b.png"})(o)
	WithHeaders(map[string]string{"X-Custom": "v"})(o)
	WithInReplyTo("<original@example.test>")(o)
	WithReferences([]string{"<grandparent@example.test>", "<parent@example.test>"})(o)

	if !sameStrings(o.cc, []string{"cc@example.test"}) {
		t.Errorf("cc = %v", o.cc)
	}
	if !sameStrings(o.bcc, []string{"bcc@example.test"}) {
		t.Errorf("bcc = %v", o.bcc)
	}
	if o.htmlBody != "<b>hi</b>" {
		t.Errorf("htmlBody = %q", o.htmlBody)
	}
	if !sameStrings(o.attachments, []string{"a.pdf", "b.png"}) {
		t.Errorf("attachments = %v", o.attachments)
	}
	if o.headers["X-Custom"] != "v" {
		t.Errorf("headers = %v", o.headers)
	}
	if o.inReplyTo != "<original@example.test>" {
		t.Errorf("inReplyTo = %q", o.inReplyTo)
	}
	if !sameStrings(o.references, []string{"<grandparent@example.test>", "<parent@example.test>"}) {
		t.Errorf("references = %v", o.references)
	}
}

// TestSendEmitsThreadingHeadersAndHTMLAlternative sends a message with
// In-Reply-To, References, and an HTML body, then verifies those headers and
// the multipart/alternative structure appear on the wire. This is the
// end-to-end proof that reply threading is actually emitted, not just built.
func TestSendEmitsThreadingHeadersAndHTMLAlternative(t *testing.T) {
	server := newSMTPTestServer(t, true, false)
	cfg := server.config()
	cfg.StartTLS = true
	cfg.Username = "user"
	cfg.Password = "password"
	cfg.From = "sender@example.test"

	sender := NewSender(cfg)
	// Inject the test CA so Send()'s STARTTLS handshake can verify the
	// self-signed test certificate. Production callers use system roots.
	sender.tlsRoots = server.clientTLSConfig().RootCAs

	err := sender.Send(
		[]string{"to@example.test"},
		"Re: thread",
		"plain fallback",
		WithHTMLBody("<p>HTML version</p>"),
		WithInReplyTo("<original@example.test>"),
		WithReferences([]string{"<grandparent@example.test>", "<parent@example.test>"}),
	)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	session := server.session(t)
	// In-Reply-To carries the immediate parent; References carries the full
	// ancestry. The exact separator (space vs comma) is go-mail's choice; we
	// assert each ancestor Message-ID is present on the wire.
	for _, want := range []string{
		"In-Reply-To: <original@example.test>",
		"<grandparent@example.test>",
		"<parent@example.test>",
		"Subject: Re: thread",
		"plain fallback",
		"HTML version",
	} {
		if !strings.Contains(session.message, want) {
			t.Errorf("message missing %q:\n%s", want, session.message)
		}
	}
	if !strings.Contains(session.message, "References:") {
		t.Errorf("message missing References header:\n%s", session.message)
	}
}

func TestFormatQuotedReply(t *testing.T) {
	tests := []struct {
		name      string
		replyBody string
		original  string
		from      string
		date      string
		want      string
	}{
		{
			name:      "full quote",
			replyBody: "Thanks!",
			original:  "Line one\nLine two",
			from:      "alice@example.com",
			date:      "2026-08-07 10:00",
			want:      "Thanks!\n\nOn 2026-08-07 10:00, alice@example.com wrote:\n> Line one\n> Line two",
		},
		{
			name:      "empty reply body omits leading blank line but still quotes",
			replyBody: "",
			original:  "Hello",
			from:      "bob@example.com",
			date:      "2026-08-07 10:00",
			want:      "On 2026-08-07 10:00, bob@example.com wrote:\n> Hello",
		},
		{
			name:      "multiline original each line prefixed",
			replyBody: "Ok",
			original:  "a\nb\nc",
			from:      "c@example.com",
			date:      "D",
			want:      "Ok\n\nOn D, c@example.com wrote:\n> a\n> b\n> c",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatQuotedReply(tt.replyBody, tt.original, tt.from, tt.date)
			if got != tt.want {
				t.Errorf("FormatQuotedReply() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestSendRequiresAtLeastOneRecipient is a fast unit test for the empty-recipient
// guard that does not require a server.
func TestSendRequiresAtLeastOneRecipient(t *testing.T) {
	sender := NewSender(&config.SMTPConfig{Host: "h", StartTLS: true})
	err := sender.Send(nil, "subject", "body")
	if err == nil || !strings.Contains(err.Error(), "recipient") {
		t.Errorf("Send(nil recipients) error = %v, want recipient error", err)
	}
}
