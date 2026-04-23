package email

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/mail"
	"strings"
	"time"

	smtp "github.com/emersion/go-smtp"
)

// Backend implements smtp.Backend. It creates a new SMTP session for each
// incoming client connection. No authentication is required — the SMTP server
// is intended to be bound to localhost or an internal network only.
type Backend struct {
	DB             *sql.DB
	AllowedDomains []string // accepted recipient domains; empty = accept all
}

// NewSession is called after client greeting (EHLO / HELO).
func (b *Backend) NewSession(_ *smtp.Conn) (smtp.Session, error) {
	return &Session{
		db:             b.DB,
		allowedDomains: b.AllowedDomains,
	}, nil
}

// Session holds the per-connection SMTP state.
type Session struct {
	db             *sql.DB
	allowedDomains []string
	from           string
	recipients     []string
}

// Mail is called when the client issues the MAIL FROM command.
func (s *Session) Mail(from string, _ *smtp.MailOptions) error {
	s.from = from
	return nil
}

// Rcpt is called when the client issues the RCPT TO command.
// It validates that the recipient uses the newsletter address scheme (nl-*).
func (s *Session) Rcpt(to string, _ *smtp.RcptOptions) error {
	addr, err := mail.ParseAddress(to)
	if err != nil {
		return &smtp.SMTPError{
			Code:    550,
			Message: fmt.Sprintf("invalid address: %v", err),
		}
	}

	localPart := strings.SplitN(addr.Address, "@", 2)[0]

	if !strings.HasPrefix(localPart, "nl-") {
		return &smtp.SMTPError{
			Code:    550,
			Message: "recipient is not a newsletter address",
		}
	}

	if len(s.allowedDomains) > 0 {
		parts := strings.SplitN(addr.Address, "@", 2)
		if len(parts) != 2 {
			return &smtp.SMTPError{Code: 550, Message: "invalid address"}
		}
		domain := parts[1]
		if !isAllowedDomain(domain, s.allowedDomains) {
			return &smtp.SMTPError{
				Code:    550,
				Message: fmt.Sprintf("domain %q not accepted", domain),
			}
		}
	}

	s.recipients = append(s.recipients, addr.Address)
	return nil
}

// Data is called when the client sends the message body.
// It passes the raw RFC 822 message to ProcessMessage for ingestion.
func (s *Session) Data(r io.Reader) error {
	if len(s.recipients) == 0 {
		slog.Warn("smtp: Data called with no recipients")
		return nil
	}

	ctx := context.Background()
	if err := ProcessMessage(ctx, s.db, r); err != nil {
		slog.Warn("smtp: failed to process message", "from", s.from, "error", err)
		return &smtp.SMTPError{
			Code:    451,
			Message: fmt.Sprintf("processing failed: %v", err),
		}
	}

	slog.Info("smtp: message accepted", "from", s.from, "recipients", s.recipients)
	return nil
}

// Reset discards the current message state.
func (s *Session) Reset() {
	s.from = ""
	s.recipients = nil
}

// Logout frees resources associated with the session.
func (s *Session) Logout() error {
	return nil
}

// isAllowedDomain returns true if domain is in the allowed list.
func isAllowedDomain(domain string, allowed []string) bool {
	for _, d := range allowed {
		if strings.EqualFold(domain, d) {
			return true
		}
	}
	return false
}

// defaultSMTPListen is the default listen address for the built-in SMTP server.
const defaultSMTPListen = ":2525"

// SMTPServer wraps go-smtp's Server with lifecycle management.
type SMTPServer struct {
	server     *smtp.Server
	listenAddr string
}

// NewSMTPServer creates an SMTP server that pipes received messages to
// ProcessMessage. listenAddr defaults to ":2525" when empty.
func NewSMTPServer(db *sql.DB, listenAddr string) *SMTPServer {
	if listenAddr == "" {
		listenAddr = defaultSMTPListen
	}

	backend := &Backend{DB: db}
	s := smtp.NewServer(backend)
	s.Domain = "localhost"
	s.MaxMessageBytes = 10 * 1024 * 1024 // 10 MB
	s.AllowInsecureAuth = true
	s.ReadTimeout = 60 * time.Second
	s.WriteTimeout = 60 * time.Second

	return &SMTPServer{
		server:     s,
		listenAddr: listenAddr,
	}
}

// Start begins listening for SMTP connections in a background goroutine.
func (s *SMTPServer) Start() error {
	ln, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return fmt.Errorf("smtp: listen %s: %w", s.listenAddr, err)
	}

	slog.Info("smtp server listening", "addr", s.listenAddr)

	go func() {
		if err := s.server.Serve(ln); err != nil {
			// Serve returns when the server is stopped; log non-close errors.
			slog.Info("smtp server stopped", "error", err)
		}
	}()

	return nil
}

// Stop shuts down the SMTP server gracefully.
func (s *SMTPServer) Stop() error {
	return s.server.Close()
}
