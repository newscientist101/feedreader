// Package email processes incoming newsletter emails from ~/Maildir/new/
// and converts them into feedreader articles.
package email

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	"srv.exe.dev/db/dbgen"
)

// Watcher polls ~/Maildir/new/ for incoming emails and processes them.
type Watcher struct {
	DB       *sql.DB
	Hostname string // e.g. "lynx-fairy.exe.xyz"
	Maildir  string // path to Maildir/new/
	stopCh   chan struct{}
}

// NewWatcher creates a new email watcher.
func NewWatcher(db *sql.DB, hostname string) *Watcher {
	home, _ := os.UserHomeDir()
	return &Watcher{
		DB:       db,
		Hostname: hostname,
		Maildir:  filepath.Join(home, "Maildir", "new"),
	}
}

// Start begins polling for new emails.
func (w *Watcher) Start(interval time.Duration) {
	w.stopCh = make(chan struct{})

	// Ensure Maildir directories exist
	for _, sub := range []string{"new", "cur"} {
		dir := filepath.Join(filepath.Dir(w.Maildir), sub)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			slog.Warn("email: failed to create maildir", "dir", dir, "error", err)
		}
	}

	go func() {
		// Process any existing mail immediately
		w.processAll()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				w.processAll()
			case <-w.stopCh:
				return
			}
		}
	}()
}

// Stop halts the email watcher.
func (w *Watcher) Stop() {
	if w.stopCh != nil {
		close(w.stopCh)
	}
}

func (w *Watcher) processAll() {
	entries, err := os.ReadDir(w.Maildir)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("email: failed to read maildir", "error", err)
		}
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(w.Maildir, entry.Name())
		if err := w.processFile(path); err != nil {
			slog.Warn("email: failed to process", "file", entry.Name(), "error", err)
			// Move to cur/ anyway to avoid re-processing failures forever
		}
		// Move to cur/ as required by exe.dev
		curDir := filepath.Join(filepath.Dir(w.Maildir), "cur")
		dst := filepath.Join(curDir, entry.Name())
		if err := os.Rename(path, dst); err != nil {
			slog.Warn("email: failed to move to cur/", "file", entry.Name(), "error", err)
		}
	}
}

func (w *Watcher) processFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = f.Close() }()

	msg, err := mail.ReadMessage(f)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	// Find the user from the Delivered-To header
	deliveredTo := msg.Header.Get("Delivered-To")
	if deliveredTo == "" {
		return fmt.Errorf("no Delivered-To header")
	}

	// Extract the local part (before @)
	addr, err := mail.ParseAddress(deliveredTo)
	if err != nil {
		// Try raw string if not a proper address
		parts := strings.SplitN(deliveredTo, "@", 2)
		if len(parts) < 1 {
			return fmt.Errorf("invalid Delivered-To: %s", deliveredTo)
		}
		addr = &mail.Address{Address: deliveredTo}
	}

	localPart := strings.SplitN(addr.Address, "@", 2)[0]

	// Expect format: nl-{token}
	if !strings.HasPrefix(localPart, "nl-") {
		return fmt.Errorf("not a newsletter address: %s", localPart)
	}
	token := strings.TrimPrefix(localPart, "nl-")

	// Look up user by newsletter token
	ctx := context.Background()
	q := dbgen.New(w.DB)
	userID, err := q.GetUserIDByNewsletterToken(ctx, token)
	if err != nil {
		return fmt.Errorf("unknown newsletter token %q: %w", token, err)
	}

	// Extract sender info
	fromHeader := msg.Header.Get("From")
	senderName, senderEmail := parseSender(fromHeader)
	if senderName == "" {
		senderName = senderEmail
	}

	// Find or create a feed for this sender
	feedURL := "newsletter://" + senderEmail
	feed, err := q.GetFeedByURL(ctx, dbgen.GetFeedByURLParams{
		Url:    feedURL,
		UserID: &userID,
	})
	if err != nil {
		// Create new feed for this sender
		interval := int64(0) // newsletters don't need polling
		feed, err = q.CreateFeed(ctx, dbgen.CreateFeedParams{
			Name:                 senderName,
			Url:                  feedURL,
			FeedType:             "newsletter",
			FetchIntervalMinutes: &interval,
			UserID:               &userID,
		})
		if err != nil {
			return fmt.Errorf("create feed: %w", err)
		}
		slog.Info("email: created newsletter feed", "feed_id", feed.ID, "sender", senderName, "user_id", userID)
	}

	// Extract article content
	subject := decodeHeader(msg.Header.Get("Subject"))
	if subject == "" {
		subject = "(no subject)"
	}

	htmlContent, textContent := extractBody(msg)

	content := htmlContent
	if content == "" {
		content = "<pre>" + escapeHTML(textContent) + "</pre>"
	}

	// Use Message-ID as GUID, fall back to subject+date hash
	guid := msg.Header.Get("Message-ID")
	if guid == "" {
		guid = fmt.Sprintf("newsletter:%s:%s", senderEmail, subject)
	}

	// Parse date
	var pubAt *time.Time
	if d, err := msg.Header.Date(); err == nil {
		utc := d.UTC()
		pubAt = &utc
	}

	var author *string
	if senderName != "" {
		author = &senderName
	}

	var summary *string
	if textContent != "" {
		s := textContent
		if len(s) > 500 {
			s = s[:500]
		}
		summary = &s
	}

	_, err = q.CreateArticle(ctx, dbgen.CreateArticleParams{
		FeedID:      feed.ID,
		Guid:        guid,
		Title:       subject,
		Author:      author,
		Content:     &content,
		Summary:     summary,
		PublishedAt: pubAt,
	})
	if err != nil {
		return fmt.Errorf("create article: %w", err)
	}

	slog.Info("email: processed newsletter", "from", senderName, "subject", subject, "feed_id", feed.ID)
	return nil
}

// parseSender extracts name and email from a From header.
func parseSender(from string) (name, email string) {
	addr, err := mail.ParseAddress(from)
	if err != nil {
		// Best effort: treat the whole thing as an email
		return "", strings.TrimSpace(from)
	}
	return addr.Name, addr.Address
}

// decodeHeader decodes RFC 2047 encoded header values.
func decodeHeader(s string) string {
	dec := new(mime.WordDecoder)
	result, err := dec.DecodeHeader(s)
	if err != nil {
		return s
	}
	return result
}

// extractBody extracts HTML and plain text content from an email message.
func extractBody(msg *mail.Message) (html, text string) {
	contentType := msg.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "text/plain"
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		// Fall back to reading body as plain text
		body, _ := io.ReadAll(msg.Body)
		return "", decodeBodyText(string(body), msg.Header.Get("Content-Transfer-Encoding"))
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		return extractMultipart(msg.Body, params["boundary"])
	}

	body, _ := io.ReadAll(msg.Body)
	decoded := decodeTransferEncoding(body, msg.Header.Get("Content-Transfer-Encoding"))

	if strings.HasPrefix(mediaType, "text/html") {
		return string(decoded), ""
	}
	return "", string(decoded)
}

// extractMultipart recursively extracts content from multipart messages.
func extractMultipart(r io.Reader, boundary string) (htmlContent, textContent string) {
	mr := multipart.NewReader(r, boundary)
	for {
		part, err := mr.NextPart()
		if err != nil {
			break
		}

		partType := part.Header.Get("Content-Type")
		medType, params, err := mime.ParseMediaType(partType)
		if err != nil {
			continue
		}

		if strings.HasPrefix(medType, "multipart/") {
			h, t := extractMultipart(part, params["boundary"])
			if h != "" {
				htmlContent = h
			}
			if t != "" {
				textContent = t
			}
			continue
		}

		body, _ := io.ReadAll(part)
		decoded := decodeTransferEncoding(body, part.Header.Get("Content-Transfer-Encoding"))

		switch {
		case strings.HasPrefix(medType, "text/html"):
			htmlContent = string(decoded)
		case strings.HasPrefix(medType, "text/plain"):
			textContent = string(decoded)
		}
	}
	return
}

// decodeTransferEncoding decodes content based on Content-Transfer-Encoding.
func decodeTransferEncoding(data []byte, encoding string) []byte {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "quoted-printable":
		result, err := io.ReadAll(quotedprintable.NewReader(bytes.NewReader(data)))
		if err != nil {
			return data
		}
		return result
	case "base64":
		// Remove whitespace before decoding
		cleaned := strings.NewReplacer("\r", "", "\n", "", " ", "").Replace(string(data))
		result, err := base64.StdEncoding.DecodeString(cleaned)
		if err != nil {
			return data
		}
		return result
	default:
		return data
	}
}

func decodeBodyText(s, encoding string) string {
	return string(decodeTransferEncoding([]byte(s), encoding))
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// GenerateToken creates a random token for newsletter email addresses.
func GenerateToken() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// EmailAddress returns the newsletter email address for a given token.
func EmailAddress(token, hostname string) string {
	return fmt.Sprintf("nl-%s@%s", token, hostname)
}
