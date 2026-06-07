package srv

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/newscientist101/feedreader/db/dbgen"
)

// buildRFC822 constructs a minimal RFC 822 email for testing.
func buildRFC822(to, from, subject, body string) string {
	return fmt.Sprintf(
		"Delivered-To: %s\r\nFrom: %s\r\nSubject: %s\r\nMessage-ID: <test@example.com>\r\nContent-Type: text/plain\r\n\r\n%s",
		to, from, subject, body,
	)
}

func TestNewsletterIngest_Success(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.WebhookSecret = "test-secret-123"

	// Create a user and set up a newsletter token.
	q := dbgen.New(s.DB)
	dbUser, err := q.GetOrCreateUser(context.Background(), dbgen.GetOrCreateUserParams{
		ExternalID: "test-user",
		Email:      "test@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	token := "abc123def456"
	if err := q.SetNewsletterToken(context.Background(), dbgen.SetNewsletterTokenParams{
		UserID: dbUser.ID,
		Value:  token,
	}); err != nil {
		t.Fatal(err)
	}

	// Build a raw RFC 822 email addressed to the newsletter token.
	email := buildRFC822(
		fmt.Sprintf("nl-%s@test.example.com", token),
		"sender@newsletter.com",
		"Weekly Update",
		"Hello, this is a newsletter.",
	)

	r := httptest.NewRequest("POST", newsletterIngestPath, strings.NewReader(email))
	r.Header.Set("Authorization", "Bearer test-secret-123")
	r.Header.Set("Content-Type", "message/rfc822")
	w := httptest.NewRecorder()
	s.apiNewsletterIngest(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify an article was created.
	var count int
	if err := s.DB.QueryRow("SELECT COUNT(*) FROM articles").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 article, got %d", count)
	}

	// Verify article content.
	var title, guid string
	if err := s.DB.QueryRow("SELECT title, guid FROM articles").Scan(&title, &guid); err != nil {
		t.Fatal(err)
	}
	if title != "Weekly Update" {
		t.Errorf("title = %q, want %q", title, "Weekly Update")
	}
	if guid != "<test@example.com>" {
		t.Errorf("guid = %q, want %q", guid, "<test@example.com>")
	}

	// Verify a newsletter feed was created.
	var feedName, feedURL, feedType string
	if err := s.DB.QueryRow("SELECT name, url, feed_type FROM feeds").Scan(&feedName, &feedURL, &feedType); err != nil {
		t.Fatal(err)
	}
	if feedType != "newsletter" {
		t.Errorf("feed_type = %q, want %q", feedType, "newsletter")
	}
	if feedURL != "newsletter://sender@newsletter.com" {
		t.Errorf("feed url = %q, want %q", feedURL, "newsletter://sender@newsletter.com")
	}
}

func TestNewsletterIngest_NotConfigured(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	// WebhookSecret is empty — not configured.

	r := httptest.NewRequest("POST", newsletterIngestPath, http.NoBody)
	w := httptest.NewRecorder()
	s.apiNewsletterIngest(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNewsletterIngest_MissingAuth(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.WebhookSecret = "test-secret"

	r := httptest.NewRequest("POST", newsletterIngestPath, http.NoBody)
	// No Authorization header.
	w := httptest.NewRecorder()
	s.apiNewsletterIngest(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNewsletterIngest_WrongSecret(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.WebhookSecret = "correct-secret"

	r := httptest.NewRequest("POST", newsletterIngestPath, http.NoBody)
	r.Header.Set("Authorization", "Bearer wrong-secret")
	w := httptest.NewRecorder()
	s.apiNewsletterIngest(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNewsletterIngest_BadContentType(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.WebhookSecret = "test-secret"

	r := httptest.NewRequest("POST", newsletterIngestPath, strings.NewReader(`{"bad":"json"}`))
	r.Header.Set("Authorization", "Bearer test-secret")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.apiNewsletterIngest(w, r)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNewsletterIngest_InvalidEmail(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.WebhookSecret = "test-secret"

	// Not a valid RFC 822 message (just garbage).
	r := httptest.NewRequest("POST", newsletterIngestPath, strings.NewReader("not-an-email"))
	r.Header.Set("Authorization", "Bearer test-secret")
	r.Header.Set("Content-Type", "message/rfc822")
	w := httptest.NewRecorder()
	s.apiNewsletterIngest(w, r)

	// The email package returns an error for missing headers → 422.
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNewsletterIngest_UnknownToken(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.WebhookSecret = "test-secret"

	eml := buildRFC822(
		"nl-unknown999@test.example.com",
		"sender@example.com",
		"Test",
		"Body",
	)

	r := httptest.NewRequest("POST", newsletterIngestPath, strings.NewReader(eml))
	r.Header.Set("Authorization", "Bearer test-secret")
	r.Header.Set("Content-Type", "message/rfc822")
	w := httptest.NewRecorder()
	s.apiNewsletterIngest(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "unknown newsletter token") {
		t.Errorf("expected unknown token error, got: %s", w.Body.String())
	}
}

func TestNewsletterIngest_NoContentType(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.WebhookSecret = "test-secret"

	// Create user and token.
	q := dbgen.New(s.DB)
	dbUser, err := q.GetOrCreateUser(context.Background(), dbgen.GetOrCreateUserParams{
		ExternalID: "test-user",
		Email:      "test@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	token := "noct12345678"
	if err := q.SetNewsletterToken(context.Background(), dbgen.SetNewsletterTokenParams{
		UserID: dbUser.ID,
		Value:  token,
	}); err != nil {
		t.Fatal(err)
	}

	eml := buildRFC822(
		fmt.Sprintf("nl-%s@test.example.com", token),
		"sender@example.com",
		"No CT",
		"Body without content-type",
	)

	// Send without Content-Type header — should be accepted.
	r := httptest.NewRequest("POST", newsletterIngestPath, strings.NewReader(eml))
	r.Header.Set("Authorization", "Bearer test-secret")
	// Explicitly remove any Content-Type set by httptest.
	r.Header.Del("Content-Type")
	w := httptest.NewRecorder()
	s.apiNewsletterIngest(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNewsletterIngest_OctetStream(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.WebhookSecret = "test-secret"

	// Create user and token.
	q := dbgen.New(s.DB)
	dbUser, err := q.GetOrCreateUser(context.Background(), dbgen.GetOrCreateUserParams{
		ExternalID: "test-user",
		Email:      "test@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	token := "octet1234567"
	if err := q.SetNewsletterToken(context.Background(), dbgen.SetNewsletterTokenParams{
		UserID: dbUser.ID,
		Value:  token,
	}); err != nil {
		t.Fatal(err)
	}

	eml := buildRFC822(
		fmt.Sprintf("nl-%s@test.example.com", token),
		"sender@example.com",
		"Octet Test",
		"Body via octet-stream",
	)

	// application/octet-stream is accepted (used by some webhook providers).
	r := httptest.NewRequest("POST", newsletterIngestPath, strings.NewReader(eml))
	r.Header.Set("Authorization", "Bearer test-secret")
	r.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	s.apiNewsletterIngest(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}
