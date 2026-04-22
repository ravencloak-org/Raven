// Package email provides outbound email delivery for Raven notifications.
// The primary transport is AWS SES (default region ap-south-1 — Raven is
// India-based and most recipients are APAC). The package exposes a narrow
// Sender interface so tests and dev mode can swap in a stub without
// pulling in the AWS SDK.
package email

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"
)

// DefaultSESRegion is the SES region used when none is configured.
// Raven is operated from India; ap-south-1 is the latency-optimal default.
const DefaultSESRegion = "ap-south-1"

// LogHashSaltEnv is the environment variable holding a per-deployment salt
// used when hashing email addresses for log correlation. When unset, the
// unsubscribe-token secret is used as a fallback so the hash is still
// non-reversible across deployments.
const LogHashSaltEnv = "EMAIL_LOG_HASH_SALT"

// Sender is the minimal contract implemented by both the SES SMTP client and
// the development stub (which logs but does not deliver).
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// Message is a single outbound email. Both HTML and plain-text bodies are
// required — CAN-SPAM and most corporate mail filters treat HTML-only mail
// as suspicious.
type Message struct {
	To       string
	Subject  string
	HTMLBody string
	TextBody string
	// ListUnsubscribe, when non-empty, is placed verbatim in the
	// List-Unsubscribe header along with List-Unsubscribe-Post for
	// one-click CAN-SPAM compliance (RFC 8058).
	ListUnsubscribe string
}

// Config configures the SES SMTP sender. All fields come from environment
// variables — never hardcode.
type Config struct {
	Region      string // e.g. "ap-south-1"
	FromAddress string // verified SES identity, e.g. "summaries@mail.ravencloak.io"
	FromName    string // optional display name, e.g. "Raven"
	// SMTP credentials produced by AWS SES → "SMTP credentials" console.
	// These are NOT your regular AWS access keys.
	SMTPUser string
	SMTPPass string
}

// Validate returns an error when required config is missing.
func (c Config) Validate() error {
	if c.FromAddress == "" {
		return errors.New("email: FromAddress is required")
	}
	if c.SMTPUser == "" || c.SMTPPass == "" {
		return errors.New("email: SMTP credentials are required")
	}
	return nil
}

// LoadConfigFromEnv reads SES settings from environment variables:
//
//	AWS_SES_REGION        (default: ap-south-1)
//	AWS_SES_FROM_ADDRESS  (required for prod)
//	AWS_SES_FROM_NAME     (optional; default: "Raven")
//	AWS_SES_SMTP_USERNAME (required for prod; distinct from IAM access keys)
//	AWS_SES_SMTP_PASSWORD (required for prod)
func LoadConfigFromEnv() Config {
	region := os.Getenv("AWS_SES_REGION")
	if region == "" {
		region = DefaultSESRegion
	}
	from := os.Getenv("AWS_SES_FROM_ADDRESS")
	name := os.Getenv("AWS_SES_FROM_NAME")
	if name == "" {
		name = "Raven"
	}
	return Config{
		Region:      region,
		FromAddress: from,
		FromName:    name,
		SMTPUser:    os.Getenv("AWS_SES_SMTP_USERNAME"),
		SMTPPass:    os.Getenv("AWS_SES_SMTP_PASSWORD"),
	}
}

// SESSender delivers email via the AWS SES SMTP interface.
// We use SMTP rather than the REST API so the handler has zero new
// Go dependencies beyond the standard library.
type SESSender struct {
	cfg    Config
	logger *slog.Logger
}

// NewSESSender constructs a sender that posts to the SES SMTP endpoint
// for the configured region. It returns an error if config is invalid.
func NewSESSender(cfg Config, logger *slog.Logger) (*SESSender, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &SESSender{cfg: cfg, logger: logger}, nil
}

// smtpHost returns the SES SMTP endpoint for the sender's region.
func (s *SESSender) smtpHost() string {
	return fmt.Sprintf("email-smtp.%s.amazonaws.com", s.cfg.Region)
}

// sanitizeHeader strips CR/LF to prevent header injection.
func sanitizeHeader(v string) string {
	v = strings.ReplaceAll(v, "\r", "")
	return strings.ReplaceAll(v, "\n", "")
}

// Send delivers a single message. The context deadline, if any, bounds the
// SMTP dial and handshake. Partial failures are returned verbatim so the
// Asynq handler can decide whether to retry.
func (s *SESSender) Send(ctx context.Context, msg Message) error {
	if msg.To == "" {
		return errors.New("email: To is empty")
	}
	host := s.smtpHost()
	addr := host + ":587" // STARTTLS submission port

	dialer := &net.Dialer{Timeout: 15 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("email: dial ses: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// Extend the dial-only timeout to cover every subsequent SMTP op
	// (StartTLS, Auth, Mail/Rcpt, Data, Close). Without this the worker
	// can block indefinitely on a stalled SES submission.
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(60 * time.Second)
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("email: set deadline: %w", err)
	}

	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("email: smtp client: %w", err)
	}
	defer func() { _ = c.Quit() }()

	if err := c.Hello("raven"); err != nil {
		return fmt.Errorf("email: ehlo: %w", err)
	}
	if err := c.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
		return fmt.Errorf("email: starttls: %w", err)
	}
	auth := smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPass, host)
	if err := c.Auth(auth); err != nil {
		return fmt.Errorf("email: auth: %w", err)
	}

	from := s.cfg.FromAddress
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("email: mail from: %w", err)
	}
	to := sanitizeHeader(msg.To)
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("email: rcpt to: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("email: data: %w", err)
	}
	raw := buildMIME(s.cfg, msg)
	if _, err := w.Write([]byte(raw)); err != nil {
		_ = w.Close()
		return fmt.Errorf("email: write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("email: close body: %w", err)
	}
	s.logger.InfoContext(ctx, "email.sent",
		slog.String("to_hash", hashAddress(to)),
		slog.String("subject_len", fmt.Sprintf("%d", len(msg.Subject))),
	)
	return nil
}

// buildMIME assembles a multipart/alternative message with both text and HTML parts.
// Boundary is a fixed constant because every message is independent — a
// predictable boundary aids debugging and doesn't weaken security.
func buildMIME(cfg Config, m Message) string {
	const boundary = "raven-boundary-ae23"
	from := sanitizeHeader(cfg.FromAddress)
	fromName := sanitizeHeader(cfg.FromName)
	subject := sanitizeHeader(m.Subject)
	to := sanitizeHeader(m.To)
	listUnsub := sanitizeHeader(m.ListUnsubscribe)

	var sb strings.Builder
	if fromName != "" {
		// net/mail.Address.String handles RFC 2047 Q-encoding when the
		// display name contains non-ASCII characters (accents, emoji, CJK).
		addr := (&mail.Address{Name: fromName, Address: from}).String()
		fmt.Fprintf(&sb, "From: %s\r\n", addr)
	} else {
		fmt.Fprintf(&sb, "From: %s\r\n", from)
	}
	fmt.Fprintf(&sb, "To: %s\r\n", to)
	fmt.Fprintf(&sb, "Subject: %s\r\n", encodeHeaderValue(subject))
	sb.WriteString("MIME-Version: 1.0\r\n")
	if listUnsub != "" {
		fmt.Fprintf(&sb, "List-Unsubscribe: <%s>\r\n", listUnsub)
		sb.WriteString("List-Unsubscribe-Post: List-Unsubscribe=One-Click\r\n")
	}
	fmt.Fprintf(&sb, "Content-Type: multipart/alternative; boundary=%q\r\n", boundary)
	sb.WriteString("\r\n")
	// Plain-text part.
	fmt.Fprintf(&sb, "--%s\r\n", boundary)
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	sb.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(m.TextBody)
	sb.WriteString("\r\n")
	// HTML part.
	fmt.Fprintf(&sb, "--%s\r\n", boundary)
	sb.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	sb.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(m.HTMLBody)
	sb.WriteString("\r\n")
	fmt.Fprintf(&sb, "--%s--\r\n", boundary)
	return sb.String()
}

// encodeHeaderValue returns the RFC 2047 Q-encoded form of v when it
// contains non-ASCII bytes; otherwise it returns v unchanged. Mail clients
// render unencoded non-ASCII header values as mojibake and some SMTP relays
// reject them outright as 8-bit violations of RFC 5322.
func encodeHeaderValue(v string) string {
	for i := 0; i < len(v); i++ {
		if v[i] > 0x7F {
			return mime.QEncoding.Encode("utf-8", v)
		}
	}
	return v
}

// hashAddress returns a short non-reversible marker so delivery logs can be
// correlated across events without persisting the recipient's email address
// in plaintext. The marker is derived from HMAC-SHA256(salt, addr) truncated
// to 12 hex chars; the salt comes from EMAIL_LOG_HASH_SALT and falls back to
// UNSUBSCRIBE_TOKEN_SECRET so a deployed environment always has a salt.
// When neither is set we return a coarse prefix/suffix marker — enough to
// tell log entries apart for a single recipient in a debug shell, and no
// more PII than was already visible in the To: header on disk.
func hashAddress(addr string) string {
	salt := os.Getenv(LogHashSaltEnv)
	if salt == "" {
		salt = os.Getenv(UnsubscribeSecretEnv)
	}
	if salt == "" {
		if len(addr) <= 4 {
			return "xxxx"
		}
		return addr[:2] + "***" + addr[len(addr)-2:]
	}
	mac := hmac.New(sha256.New, []byte(salt))
	_, _ = mac.Write([]byte(addr))
	sum := mac.Sum(nil)
	return hex.EncodeToString(sum)[:12]
}

// StubSender is a Sender that only logs — used in dev when SES is not configured.
type StubSender struct {
	logger *slog.Logger
}

// NewStubSender returns a no-op sender.
func NewStubSender(logger *slog.Logger) *StubSender {
	if logger == nil {
		logger = slog.Default()
	}
	return &StubSender{logger: logger}
}

// Send logs and discards the message.
func (s *StubSender) Send(ctx context.Context, msg Message) error {
	s.logger.WarnContext(ctx, "email.stub.send",
		slog.String("subject", msg.Subject),
		slog.String("to_hash", hashAddress(msg.To)),
	)
	return nil
}

// compile-time interface checks
var (
	_ Sender = (*SESSender)(nil)
	_ Sender = (*StubSender)(nil)
)
