package email

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"testing"
	"time"

	"github.com/newscientist101/feedreader/db"
	"github.com/newscientist101/feedreader/db/dbgen"
)

// openTestSMTPServer starts an SMTPServer on a random port and returns the
// server along with its listen address. The test registers a cleanup that
// calls Stop.
func openTestSMTPServer(t *testing.T) (srv *SMTPServer, addr string) {
	t.Helper()

	// Pick a free port by binding to :0 and recording the address.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen :0: %v", err)
	}
	addr = ln.Addr().String()
	_ = ln.Close()

	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	srv = NewSMTPServer(sqlDB, addr)
	if err := srv.Start(); err != nil {
		t.Fatalf("SMTPServer.Start: %v", err)
	}
	t.Cleanup(func() { _ = srv.Stop() })

	// Wait until the port is actually accepting connections (max 2 s).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	return srv, addr
}

// TestSMTPServerStartStop verifies that the server starts, is reachable, and
// stops cleanly.
func TestSMTPServerStartStop(t *testing.T) {
	_, addr := openTestSMTPServer(t)

	// Verify the port is open.
	c, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial after Start: %v", err)
	}
	_ = c.Close()
}

// TestSMTPServerStop verifies that Stop closes the port.
func TestSMTPServerStop(t *testing.T) {
	srv, addr := openTestSMTPServer(t)

	// Stop the server explicitly before the cleanup runs.
	if err := srv.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// Give the server a moment to fully close.
	time.Sleep(50 * time.Millisecond)

	// Port should now be closed.
	c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err == nil {
		_ = c.Close()
		t.Error("expected dial to fail after Stop, but it succeeded")
	}
}

// TestSMTPReceiveEmail delivers a test newsletter via net/smtp and checks that
// an article is created in the database.
func TestSMTPReceiveEmail(t *testing.T) {
	// Build a self-contained test so we can inspect the same DB.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	addr2 := ln.Addr().String()
	_ = ln.Close()

	sqlDB2, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	if err := db.RunMigrations(sqlDB2); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	defer func() { _ = sqlDB2.Close() }()

	q2 := dbgen.New(sqlDB2)
	ctx := context.Background()
	user2, err := q2.GetOrCreateUser(ctx, dbgen.GetOrCreateUserParams{
		ExternalID: "smtp-recv-user",
		Email:      "smtprecv@example.com",
	})
	if err != nil {
		t.Fatalf("GetOrCreateUser: %v", err)
	}
	token2 := "recvtoken12345"
	if err := q2.SetNewsletterToken(ctx, dbgen.SetNewsletterTokenParams{
		UserID: user2.ID,
		Value:  token2,
	}); err != nil {
		t.Fatalf("SetNewsletterToken: %v", err)
	}

	srv2 := NewSMTPServer(sqlDB2, addr2)
	if err := srv2.Start(); err != nil {
		t.Fatalf("SMTPServer.Start: %v", err)
	}
	defer func() { _ = srv2.Stop() }()

	// Wait for readiness.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err2 := net.DialTimeout("tcp", addr2, 100*time.Millisecond)
		if err2 == nil {
			_ = c.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Build the email body with a Delivered-To header that contains a valid
	// nl-{token}@domain address so ProcessMessage can route it.
	recipient := fmt.Sprintf("nl-%s@localhost", token2)
	body := buildTestEmail(t, recipient, "Newsletter Sender <sender@newsletters.example.com>",
		"SMTP Test Newsletter", "This is the SMTP test body.")

	// Send via net/smtp (the standard library SMTP client).
	err = smtp.SendMail(
		addr2,
		nil, // no auth
		"sender@newsletters.example.com",
		[]string{recipient},
		[]byte(body),
	)
	if err != nil {
		t.Fatalf("smtp.SendMail: %v", err)
	}

	// Give the server a moment to process the message asynchronously.
	// (ProcessMessage runs synchronously inside Data(), so no real wait needed.)
	time.Sleep(20 * time.Millisecond)

	// Verify an article was created.
	feeds, err := q2.ListFeeds(ctx, &user2.ID)
	if err != nil {
		t.Fatalf("ListFeeds: %v", err)
	}
	if len(feeds) == 0 {
		t.Fatal("expected at least one feed after email delivery")
	}
	if feeds[0].FeedType != "newsletter" {
		t.Errorf("feed_type = %q, want %q", feeds[0].FeedType, "newsletter")
	}

	articles, err := q2.ListArticlesByFeed(ctx, dbgen.ListArticlesByFeedParams{
		FeedID: feeds[0].ID,
		UserID: &user2.ID,
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListArticlesByFeed: %v", err)
	}
	if len(articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(articles))
	}
	if articles[0].Title != "SMTP Test Newsletter" {
		t.Errorf("title = %q, want %q", articles[0].Title, "SMTP Test Newsletter")
	}

}

// TestSMTPRejectInvalidRecipient sends to an address that does NOT start with
// nl- and verifies it is rejected by the server (SMTP 550 error).
func TestSMTPRejectInvalidRecipient(t *testing.T) {
	_, addr := openTestSMTPServer(t)

	// Attempt to deliver to a non-newsletter address.
	err := smtp.SendMail(
		addr,
		nil,
		"sender@example.com",
		[]string{"regular@localhost"},
		[]byte(buildTestEmail(t,
			"regular@localhost",
			"sender@example.com",
			"Should be rejected",
			"body",
		)),
	)
	if err == nil {
		t.Fatal("expected SendMail to return an error for non-nl- recipient, got nil")
	}
	if !strings.Contains(err.Error(), "550") {
		t.Errorf("expected 550 error, got: %v", err)
	}
}

// TestSMTPAllowedDomains verifies that the AllowedDomains filter rejects
// recipients on disallowed domains.
func TestSMTPAllowedDomains(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	// Build backend with domain restriction.
	backend := &Backend{
		DB:             sqlDB,
		AllowedDomains: []string{"allowed.example.com"},
	}

	// Test that a matching domain is accepted by Rcpt.
	session := &Session{db: sqlDB, allowedDomains: backend.AllowedDomains}
	if err := session.Rcpt("nl-token@allowed.example.com", nil); err != nil {
		t.Errorf("expected no error for allowed domain, got: %v", err)
	}

	// Test that a non-matching domain is rejected.
	session2 := &Session{db: sqlDB, allowedDomains: backend.AllowedDomains}
	if err := session2.Rcpt("nl-token@other.example.com", nil); err == nil {
		t.Error("expected rejection for non-allowed domain, got nil error")
	}
}

// TestSMTPSessionReset verifies that Reset clears from and recipients.
func TestSMTPSessionReset(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	s := &Session{
		db:         sqlDB,
		from:       "sender@example.com",
		recipients: []string{"nl-abc@localhost"},
	}
	s.Reset()
	if s.from != "" {
		t.Errorf("from not cleared after Reset, got %q", s.from)
	}
	if len(s.recipients) != 0 {
		t.Errorf("recipients not cleared after Reset, got %v", s.recipients)
	}
}

// TestSMTPSessionLogout verifies Logout does not error.
func TestSMTPSessionLogout(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	s := &Session{db: sqlDB}
	if err := s.Logout(); err != nil {
		t.Errorf("Logout returned error: %v", err)
	}
}

// TestSMTPNewSessionCreatesSession verifies that Backend.NewSession returns a
// valid session without error.
func TestSMTPNewSessionCreatesSession(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	b := &Backend{DB: sqlDB}
	sess, err := b.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if sess == nil {
		t.Error("NewSession returned nil session")
	}
}

// TestNewSMTPServerDefaultAddr verifies that NewSMTPServer uses the default
// listen address when an empty string is passed.
func TestNewSMTPServerDefaultAddr(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	srv := NewSMTPServer(sqlDB, "")
	if srv.listenAddr != defaultSMTPListen {
		t.Errorf("listenAddr = %q, want %q", srv.listenAddr, defaultSMTPListen)
	}
}

// TestIsAllowedDomain tests the domain allowlist helper.
func TestIsAllowedDomain(t *testing.T) {
	allowed := []string{"example.com", "test.org"}

	tests := []struct {
		domain string
		want   bool
	}{
		{"example.com", true},
		{"EXAMPLE.COM", true}, // case-insensitive
		{"test.org", true},
		{"other.com", false},
		{"notexample.com", false},
	}

	for _, tt := range tests {
		got := isAllowedDomain(tt.domain, allowed)
		if got != tt.want {
			t.Errorf("isAllowedDomain(%q) = %v, want %v", tt.domain, got, tt.want)
		}
	}
}

// buildTestEmail constructs a minimal RFC 2822 email string suitable for
// sending via net/smtp. The Delivered-To header is set to recipient so
// ProcessMessage can route it to the correct user.
func buildTestEmail(t *testing.T, deliveredTo, from, subject, body string) string {
	t.Helper()
	return "Delivered-To: " + deliveredTo + "\r\n" +
		"From: " + from + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		body
}
