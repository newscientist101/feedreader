package email

import (
	"context"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/newscientist101/feedreader/db"
	"github.com/newscientist101/feedreader/db/dbgen"
)

func TestParseSender(t *testing.T) {
	tests := []struct {
		from     string
		wantName string
		wantAddr string
	}{
		{`"Newsletter" <news@example.com>`, "Newsletter", "news@example.com"},
		{`news@example.com`, "", "news@example.com"},
		{`<news@example.com>`, "", "news@example.com"},
		{`Bad Header`, "", "Bad Header"},
	}

	for _, tt := range tests {
		name, addr := parseSender(tt.from)
		if name != tt.wantName || addr != tt.wantAddr {
			t.Errorf("parseSender(%q) = (%q, %q), want (%q, %q)",
				tt.from, name, addr, tt.wantName, tt.wantAddr)
		}
	}
}

func TestDecodeHeader(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Simple Subject", "Simple Subject"},
		{"=?UTF-8?B?SGVsbG8gV29ybGQ=?=", "Hello World"},
		{"=?UTF-8?Q?Hello_World?=", "Hello World"},
	}

	for _, tt := range tests {
		got := decodeHeader(tt.input)
		if got != tt.want {
			t.Errorf("decodeHeader(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractBody(t *testing.T) {
	// Simple text/plain email
	raw := "Content-Type: text/plain\r\n\r\nHello World"
	msg, _ := mail.ReadMessage(strings.NewReader(raw))
	html, text := extractBody(msg)
	if html != "" {
		t.Errorf("expected no HTML, got %q", html)
	}
	if text != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", text)
	}

	// Simple text/html email
	raw = "Content-Type: text/html\r\n\r\n<p>Hello</p>"
	msg, _ = mail.ReadMessage(strings.NewReader(raw))
	html, text = extractBody(msg)
	if html != "<p>Hello</p>" {
		t.Errorf("expected '<p>Hello</p>', got %q", html)
	}
	if text != "" {
		t.Errorf("expected no text, got %q", text)
	}

	// Multipart email
	raw = "Content-Type: multipart/alternative; boundary=\"BOUNDARY\"\r\n\r\n" +
		"--BOUNDARY\r\n" +
		"Content-Type: text/plain\r\n\r\n" +
		"Plain text\r\n" +
		"--BOUNDARY\r\n" +
		"Content-Type: text/html\r\n\r\n" +
		"<p>HTML content</p>\r\n" +
		"--BOUNDARY--\r\n"
	msg, _ = mail.ReadMessage(strings.NewReader(raw))
	html, text = extractBody(msg)
	if html != "<p>HTML content</p>" {
		t.Errorf("expected '<p>HTML content</p>', got %q", html)
	}
	if text != "Plain text" {
		t.Errorf("expected 'Plain text', got %q", text)
	}
}

func TestDecodeTransferEncoding(t *testing.T) {
	// Quoted-printable
	qp := []byte("Hello=20World")
	result := decodeTransferEncoding(qp, "quoted-printable")
	if string(result) != "Hello World" {
		t.Errorf("QP decode: got %q, want 'Hello World'", result)
	}

	// Base64
	b64 := []byte("SGVsbG8gV29ybGQ=")
	result = decodeTransferEncoding(b64, "base64")
	if string(result) != "Hello World" {
		t.Errorf("base64 decode: got %q, want 'Hello World'", result)
	}

	// None/identity
	plain := []byte("Hello World")
	result = decodeTransferEncoding(plain, "")
	if string(result) != "Hello World" {
		t.Errorf("plain decode: got %q, want 'Hello World'", result)
	}
}

func TestGenerateToken(t *testing.T) {
	t1, err := GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(t1) != 24 { // 12 bytes = 24 hex chars
		t.Errorf("token length = %d, want 24", len(t1))
	}

	t2, _ := GenerateToken()
	if t1 == t2 {
		t.Error("tokens should be unique")
	}
}

func TestEmailAddress(t *testing.T) {
	addr := EmailAddress("abc123", "lynx-fairy.exe.xyz")
	if addr != "nl-abc123@lynx-fairy.exe.xyz" {
		t.Errorf("got %q, want 'nl-abc123@lynx-fairy.exe.xyz'", addr)
	}
}

func TestIsForwarded(t *testing.T) {
	tests := []struct {
		subject string
		text    string
		want    bool
	}{
		{"Fwd: Weekly Newsletter", "", true},
		{"Fw: Update", "", true},
		{"fwd: lower case", "", true},
		{"FW: upper case", "", true},
		{"Wg: German forward", "", true},
		{"Regular Subject", "", false},
		{"Regular Subject", "---------- Forwarded message ---------", true},
		{"Regular Subject", "Begin forwarded message:", true},
		{"Regular Subject", "-------- Original Message --------", true},
	}
	for _, tt := range tests {
		got := isForwarded(tt.subject, tt.text, "")
		if got != tt.want {
			t.Errorf("isForwarded(%q, %q) = %v, want %v", tt.subject, tt.text, got, tt.want)
		}
	}
}

func TestStripForwardPrefix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Fwd: Weekly Newsletter", "Weekly Newsletter"},
		{"Fw: Update", "Update"},
		{"FW: FW: Double forward", "Double forward"},
		{"Fwd: Fwd: Triple", "Triple"},
		{"Regular Subject", "Regular Subject"},
	}
	for _, tt := range tests {
		got := stripForwardPrefix(tt.input)
		if got != tt.want {
			t.Errorf("stripForwardPrefix(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractForwardedSender(t *testing.T) {
	// Gmail format
	gmail := `Some preamble text

---------- Forwarded message ---------
From: Weekly Digest <digest@example.com>
Date: Wed, Feb 18, 2026 at 10:00 AM
Subject: This Week
To: user@gmail.com

Newsletter content here.`

	name, email := extractForwardedSender(gmail, "")
	if name != "Weekly Digest" || email != "digest@example.com" {
		t.Errorf("gmail: got (%q, %q), want ('Weekly Digest', 'digest@example.com')", name, email)
	}

	// Outlook format
	outlook := `
________________________________
From: Tech News <news@techsite.com>
Sent: Wednesday, February 18, 2026 10:00 AM
To: user@outlook.com
Subject: Daily Digest

Content here.`

	name, email = extractForwardedSender(outlook, "")
	if name != "Tech News" || email != "news@techsite.com" {
		t.Errorf("outlook: got (%q, %q), want ('Tech News', 'news@techsite.com')", name, email)
	}

	// Apple Mail format
	apple := `
Begin forwarded message:

From: Apple News <apple@news.com>
Subject: Weekly Update
Date: February 18, 2026

Content.`

	name, email = extractForwardedSender(apple, "")
	if name != "Apple News" || email != "apple@news.com" {
		t.Errorf("apple: got (%q, %q), want ('Apple News', 'apple@news.com')", name, email)
	}

	// Bare email (no name)
	bare := `---------- Forwarded message ---------
From: newsletter@example.com
Subject: Update
`
	name, email = extractForwardedSender(bare, "")
	if email != "newsletter@example.com" {
		t.Errorf("bare: got (%q, %q), want ('', 'newsletter@example.com')", name, email)
	}

	// HTML-only body
	htmlBody := `<div>---------- Forwarded message ---------<br>
<b>From:</b> HTML News &lt;html@news.com&gt;<br>
<b>Subject:</b> Test</div>`

	name, email = extractForwardedSender("", htmlBody)
	if email != "html@news.com" {
		t.Errorf("html: got (%q, %q), want ('HTML News', 'html@news.com')", name, email)
	}

	// No From: line
	noFrom := "---------- Forwarded message ---------\nSubject: Test"
	name, email = extractForwardedSender(noFrom, "")
	if email != "" {
		t.Errorf("noFrom: got (%q, %q), want ('', '')", name, email)
	}
}

// setupTestDB opens an in-memory SQLite DB with migrations applied.
func setupTestDB(t *testing.T) *dbgen.Queries {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatal(err)
	}
	return dbgen.New(sqlDB)
}

// setupTestDBRaw returns both the raw *sql.DB and *dbgen.Queries.
func setupTestDBRaw(t *testing.T) (*dbgen.Queries, *Watcher) {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatal(err)
	}
	w := &Watcher{
		DB:       sqlDB,
		Hostname: "testhost",
	}
	return dbgen.New(sqlDB), w
}

// createUserWithToken creates a user and sets a newsletter token, returning the user ID.
func createUserWithToken(t *testing.T, q *dbgen.Queries, token string) int64 {
	t.Helper()
	ctx := context.Background()
	user, err := q.GetOrCreateUser(ctx, dbgen.GetOrCreateUserParams{
		ExternalID: "ext-test-user",
		Email:      "testuser@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.SetNewsletterToken(ctx, dbgen.SetNewsletterTokenParams{
		UserID: user.ID,
		Value:  token,
	}); err != nil {
		t.Fatal(err)
	}
	return user.ID
}

// writeEmailFile writes a minimal RFC 2822 email to the given path.
func writeEmailFile(t *testing.T, path, token, from, subject, body string) {
	t.Helper()
	content := "Delivered-To: nl-" + token + "@testhost\r\n" +
		"From: " + from + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		body
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeBodyText(t *testing.T) {
	// quoted-printable
	got := decodeBodyText("Hello=20World", "quoted-printable")
	if got != "Hello World" {
		t.Errorf("decodeBodyText QP = %q, want %q", got, "Hello World")
	}

	// base64
	got = decodeBodyText("SGVsbG8gV29ybGQ=", "base64")
	if got != "Hello World" {
		t.Errorf("decodeBodyText base64 = %q, want %q", got, "Hello World")
	}

	// identity / no encoding
	got = decodeBodyText("plain text", "")
	if got != "plain text" {
		t.Errorf("decodeBodyText plain = %q, want %q", got, "plain text")
	}
}

func TestEscapeHTML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"<b>bold</b>", "&lt;b&gt;bold&lt;/b&gt;"},
		{`"quoted" & done`, "&#34;quoted&#34; &amp; done"},
		{"no special chars", "no special chars"},
		{"", ""},
	}
	for _, tt := range tests {
		got := escapeHTML(tt.input)
		if got != tt.want {
			t.Errorf("escapeHTML(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestProcessFile(t *testing.T) {
	q, w := setupTestDBRaw(t)
	ctx := context.Background()

	token := "testtoken123"
	userID := createUserWithToken(t, q, token)

	// Write a temp email file
	tmpDir := t.TempDir()
	emailPath := filepath.Join(tmpDir, "test-email")
	writeEmailFile(t, emailPath, token, "sender@example.com", "Test Newsletter", "Hello world")

	// Process the file
	if err := w.processFile(emailPath); err != nil {
		t.Fatalf("processFile: %v", err)
	}

	// Verify an article was created by listing articles for this user
	feeds, err := q.ListFeeds(ctx, &userID)
	if err != nil {
		t.Fatalf("ListFeeds: %v", err)
	}
	if len(feeds) == 0 {
		t.Fatal("expected at least one feed, got none")
	}

	feed := feeds[0]
	if feed.Url != "newsletter://sender@example.com" {
		t.Errorf("feed URL = %q, want %q", feed.Url, "newsletter://sender@example.com")
	}
	if feed.FeedType != "newsletter" {
		t.Errorf("feed type = %q, want %q", feed.FeedType, "newsletter")
	}

	articles, err := q.ListArticlesByFeed(ctx, dbgen.ListArticlesByFeedParams{
		FeedID: feed.ID,
		UserID: &userID,
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListArticlesByFeed: %v", err)
	}
	if len(articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(articles))
	}
	art := articles[0]
	if art.Title != "Test Newsletter" {
		t.Errorf("article title = %q, want %q", art.Title, "Test Newsletter")
	}
	if art.Author == nil || *art.Author != "sender@example.com" {
		t.Errorf("article author = %v, want %q", art.Author, "sender@example.com")
	}
	if art.Content == nil || !strings.Contains(*art.Content, "Hello world") {
		t.Errorf("article content = %v, want it to contain 'Hello world'", art.Content)
	}
}

func TestProcessFile_NoDeliveredTo(t *testing.T) {
	_, w := setupTestDBRaw(t)

	tmpDir := t.TempDir()
	emailPath := filepath.Join(tmpDir, "bad-email")
	content := "From: sender@example.com\r\nSubject: Test\r\n\r\nBody"
	if err := os.WriteFile(emailPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	err := w.processFile(emailPath)
	if err == nil {
		t.Fatal("expected error for missing Delivered-To header")
	}
	if !strings.Contains(err.Error(), "no Delivered-To") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProcessFile_BadToken(t *testing.T) {
	_, w := setupTestDBRaw(t)

	tmpDir := t.TempDir()
	emailPath := filepath.Join(tmpDir, "bad-token-email")
	writeEmailFile(t, emailPath, "nonexistenttoken", "sender@example.com", "Test", "Body")

	err := w.processFile(emailPath)
	if err == nil {
		t.Fatal("expected error for unknown token")
	}
	if !strings.Contains(err.Error(), "unknown newsletter token") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProcessFile_NotNewsletter(t *testing.T) {
	_, w := setupTestDBRaw(t)

	tmpDir := t.TempDir()
	emailPath := filepath.Join(tmpDir, "not-nl-email")
	content := "Delivered-To: regular@testhost\r\nFrom: sender@example.com\r\nSubject: Test\r\n\r\nBody"
	if err := os.WriteFile(emailPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	err := w.processFile(emailPath)
	if err == nil {
		t.Fatal("expected error for non-newsletter address")
	}
	if !strings.Contains(err.Error(), "not a newsletter address") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProcessFile_HTMLEmail(t *testing.T) {
	q, w := setupTestDBRaw(t)
	ctx := context.Background()

	token := "htmltoken456"
	userID := createUserWithToken(t, q, token)

	tmpDir := t.TempDir()
	emailPath := filepath.Join(tmpDir, "html-email")
	htmlContent := "Delivered-To: nl-" + token + "@testhost\r\n" +
		"From: html@example.com\r\n" +
		"Subject: HTML Newsletter\r\n" +
		"Content-Type: text/html\r\n" +
		"\r\n" +
		"<h1>Hello</h1>"
	if err := os.WriteFile(emailPath, []byte(htmlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := w.processFile(emailPath); err != nil {
		t.Fatalf("processFile: %v", err)
	}

	feeds, err := q.ListFeeds(ctx, &userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) == 0 {
		t.Fatal("expected a feed")
	}
	articles, err := q.ListArticlesByFeed(ctx, dbgen.ListArticlesByFeedParams{
		FeedID: feeds[0].ID,
		UserID: &userID,
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(articles))
	}
	if articles[0].Content == nil || !strings.Contains(*articles[0].Content, "<h1>Hello</h1>") {
		t.Errorf("expected HTML content, got %v", articles[0].Content)
	}
}

func TestProcessAll(t *testing.T) {
	q, w := setupTestDBRaw(t)
	ctx := context.Background()

	token := "processalltoken"
	userID := createUserWithToken(t, q, token)

	// Create temp maildir structure
	maildir := t.TempDir()
	newDir := filepath.Join(maildir, "new")
	curDir := filepath.Join(maildir, "cur")
	if err := os.MkdirAll(newDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(curDir, 0o700); err != nil {
		t.Fatal(err)
	}

	w.Maildir = newDir

	// Write an email to new/
	emailPath := filepath.Join(newDir, "test-msg-001")
	writeEmailFile(t, emailPath, token, "news@example.com", "Process All Test", "Body content")

	// Run processAll
	w.processAll()

	// Verify the file was moved to cur/
	if _, err := os.Stat(emailPath); !os.IsNotExist(err) {
		t.Error("expected email file to be moved from new/, but it still exists")
	}
	movedPath := filepath.Join(curDir, "test-msg-001")
	if _, err := os.Stat(movedPath); err != nil {
		t.Errorf("expected email file in cur/, got error: %v", err)
	}

	// Verify article was created
	feeds, err := q.ListFeeds(ctx, &userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) == 0 {
		t.Fatal("expected at least one feed")
	}
	articles, err := q.ListArticlesByFeed(ctx, dbgen.ListArticlesByFeedParams{
		FeedID: feeds[0].ID,
		UserID: &userID,
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(articles))
	}
	if articles[0].Title != "Process All Test" {
		t.Errorf("article title = %q, want %q", articles[0].Title, "Process All Test")
	}
}

func TestProcessAll_NonexistentDir(t *testing.T) {
	_, w := setupTestDBRaw(t)
	w.Maildir = filepath.Join(t.TempDir(), "does-not-exist", "new")

	// Should not panic, just return gracefully
	w.processAll()
}

func TestProcessAll_SkipsDirs(t *testing.T) {
	_, w := setupTestDBRaw(t)

	maildir := t.TempDir()
	newDir := filepath.Join(maildir, "new")
	curDir := filepath.Join(maildir, "cur")
	os.MkdirAll(newDir, 0o700)
	os.MkdirAll(curDir, 0o700)
	// Create a subdirectory inside new/ — should be skipped
	os.MkdirAll(filepath.Join(newDir, "subdir"), 0o700)

	w.Maildir = newDir
	w.processAll() // should not fail
}

func TestNewWatcher(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	w := NewWatcher(sqlDB, "myhost.example.com")
	if w.DB != sqlDB {
		t.Error("DB not set correctly")
	}
	if w.Hostname != "myhost.example.com" {
		t.Errorf("Hostname = %q, want %q", w.Hostname, "myhost.example.com")
	}
	if !strings.HasSuffix(w.Maildir, filepath.Join("Maildir", "new")) {
		t.Errorf("Maildir = %q, expected to end with Maildir/new", w.Maildir)
	}
}

func TestStartStop(t *testing.T) {
	q, w := setupTestDBRaw(t)
	ctx := context.Background()

	token := "startstoptoken"
	userID := createUserWithToken(t, q, token)

	// Create temp maildir
	maildir := t.TempDir()
	newDir := filepath.Join(maildir, "new")
	curDir := filepath.Join(maildir, "cur")
	os.MkdirAll(newDir, 0o700)
	os.MkdirAll(curDir, 0o700)
	w.Maildir = newDir

	// Start the watcher with a short interval
	w.Start(10 * time.Millisecond)

	// Write an email file after start
	time.Sleep(5 * time.Millisecond)
	emailPath := filepath.Join(newDir, "start-stop-msg")
	writeEmailFile(t, emailPath, token, "lifecycle@example.com", "Lifecycle Test", "Body")

	// Wait for the ticker to fire and process
	time.Sleep(50 * time.Millisecond)

	// Stop the watcher
	w.Stop()

	// Verify the file was processed (moved to cur/)
	if _, err := os.Stat(emailPath); !os.IsNotExist(err) {
		t.Error("expected email to be moved from new/")
	}
	movedPath := filepath.Join(curDir, "start-stop-msg")
	if _, err := os.Stat(movedPath); err != nil {
		t.Errorf("expected email in cur/, got: %v", err)
	}

	// Verify article was created
	feeds, err := q.ListFeeds(ctx, &userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) == 0 {
		t.Fatal("expected a feed after start/stop lifecycle")
	}
	articles, err := q.ListArticlesByFeed(ctx, dbgen.ListArticlesByFeedParams{
		FeedID: feeds[0].ID,
		UserID: &userID,
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(articles))
	}
	if articles[0].Title != "Lifecycle Test" {
		t.Errorf("article title = %q, want %q", articles[0].Title, "Lifecycle Test")
	}
}

func TestStopNil(t *testing.T) {
	// Calling Stop on a watcher that was never started should not panic.
	w := &Watcher{}
	w.Stop() // should be a no-op
}

func TestProcessMessage(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatal(err)
	}

	q := dbgen.New(sqlDB)
	ctx := context.Background()
	token := "procmsg12345"
	user, err := q.GetOrCreateUser(ctx, dbgen.GetOrCreateUserParams{
		ExternalID: "pm-user",
		Email:      "pmuser@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.SetNewsletterToken(ctx, dbgen.SetNewsletterTokenParams{
		UserID: user.ID,
		Value:  token,
	}); err != nil {
		t.Fatal(err)
	}

	raw := "Delivered-To: nl-" + token + "@example.com\r\n" +
		"From: \"Test Sender\" <test@sender.com>\r\n" +
		"Subject: ProcessMessage Test\r\n" +
		"Message-ID: <unique-msg@sender.com>\r\n" +
		"Date: Thu, 01 Jan 2026 00:00:00 +0000\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Newsletter body text."

	if err := ProcessMessage(ctx, sqlDB, strings.NewReader(raw)); err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}

	feeds, err := q.ListFeeds(ctx, &user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) != 1 {
		t.Fatalf("expected 1 feed, got %d", len(feeds))
	}
	if feeds[0].Url != "newsletter://test@sender.com" {
		t.Errorf("feed url = %q, want %q", feeds[0].Url, "newsletter://test@sender.com")
	}
	if feeds[0].Name != "Test Sender" {
		t.Errorf("feed name = %q, want %q", feeds[0].Name, "Test Sender")
	}

	articles, err := q.ListArticlesByFeed(ctx, dbgen.ListArticlesByFeedParams{
		FeedID: feeds[0].ID,
		UserID: &user.ID,
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(articles))
	}
	if articles[0].Title != "ProcessMessage Test" {
		t.Errorf("title = %q, want %q", articles[0].Title, "ProcessMessage Test")
	}
	if articles[0].Guid != "<unique-msg@sender.com>" {
		t.Errorf("guid = %q, want %q", articles[0].Guid, "<unique-msg@sender.com>")
	}
}

func TestProcessMessage_MissingDeliveredTo(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatal(err)
	}

	raw := "From: sender@example.com\r\n" +
		"Subject: No Delivered-To\r\n" +
		"\r\n" +
		"Body"

	err = ProcessMessage(context.Background(), sqlDB, strings.NewReader(raw))
	if err == nil {
		t.Fatal("expected error for missing Delivered-To")
	}
	if !strings.Contains(err.Error(), "no Delivered-To") {
		t.Errorf("unexpected error: %v", err)
	}
}
