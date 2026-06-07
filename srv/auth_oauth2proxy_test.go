package srv

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOAuth2ProxyProvider_WithAllHeaders(t *testing.T) {
	t.Parallel()
	p := OAuth2ProxyProvider{}
	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set("X-Forwarded-User", "alice")
	r.Header.Set("X-Forwarded-Email", "alice@example.com")
	r.Header.Set("X-Forwarded-Preferred-Username", "alice123")
	r.Header.Set("X-Forwarded-Groups", "admins,users")

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id == nil {
		t.Fatal("expected identity")
	}
	if id.ExternalID != "alice@example.com" {
		t.Errorf("ExternalID = %q, want alice@example.com", id.ExternalID)
	}
	if id.Email != "alice@example.com" {
		t.Errorf("Email = %q, want alice@example.com", id.Email)
	}
}

func TestOAuth2ProxyProvider_EmailOnly(t *testing.T) {
	t.Parallel()
	p := OAuth2ProxyProvider{}
	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set("X-Forwarded-Email", "bob@example.com")

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id == nil {
		t.Fatal("expected identity")
	}
	if id.ExternalID != "bob@example.com" {
		t.Errorf("ExternalID = %q, want bob@example.com", id.ExternalID)
	}
	if id.Email != "bob@example.com" {
		t.Errorf("Email = %q, want bob@example.com", id.Email)
	}
}

func TestOAuth2ProxyProvider_UserOnly(t *testing.T) {
	t.Parallel()
	p := OAuth2ProxyProvider{}
	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set("X-Forwarded-User", "carol")
	// No email header — user is used as fallback external ID

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id == nil {
		t.Fatal("expected identity")
	}
	if id.ExternalID != "carol" {
		t.Errorf("ExternalID = %q, want carol", id.ExternalID)
	}
	if id.Email != "" {
		t.Errorf("Email = %q, want empty", id.Email)
	}
}

func TestOAuth2ProxyProvider_NoHeaders(t *testing.T) {
	t.Parallel()
	p := OAuth2ProxyProvider{}
	r := httptest.NewRequest("GET", "/", http.NoBody)

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id != nil {
		t.Errorf("expected nil identity, got %+v", id)
	}
}

func TestOAuth2ProxyProvider_EmailPreferredOverUser(t *testing.T) {
	// When both headers are present, email should be the external ID.
	t.Parallel()
	p := OAuth2ProxyProvider{}
	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set("X-Forwarded-User", "dave")
	r.Header.Set("X-Forwarded-Email", "dave@corp.com")

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id == nil {
		t.Fatal("expected identity")
	}
	if id.ExternalID != "dave@corp.com" {
		t.Errorf("ExternalID = %q, want dave@corp.com (email preferred over user)", id.ExternalID)
	}
}

func TestAuthMiddleware_WithOAuth2ProxyHeaders(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.AuthProvider = OAuth2ProxyProvider{}

	var gotUser *User
	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = GetUser(r.Context())
		w.WriteHeader(200)
	}))

	r := httptest.NewRequest("GET", "/api/feeds", http.NoBody)
	r.Header.Set("X-Forwarded-User", "eve")
	r.Header.Set("X-Forwarded-Email", "eve@example.com")
	r.Header.Set("X-Forwarded-Preferred-Username", "eve_user")
	r.Header.Set("X-Forwarded-Groups", "users")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotUser == nil {
		t.Fatal("user not set in context")
	}
	if gotUser.ExternalID != "eve@example.com" {
		t.Errorf("external_id = %q, want eve@example.com", gotUser.ExternalID)
	}
	if gotUser.Email != "eve@example.com" {
		t.Errorf("email = %q, want eve@example.com", gotUser.Email)
	}
}
