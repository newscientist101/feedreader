package srv

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetUser_NoUser(t *testing.T) {
	ctx := httptest.NewRequest("GET", "/", http.NoBody).Context()
	if u := GetUser(ctx); u != nil {
		t.Errorf("expected nil, got %+v", u)
	}
}

func TestAuthMiddleware_StaticBypass(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	called := false
	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	}))

	r := httptest.NewRequest("GET", "/static/app.js", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if !called {
		t.Error("static route should pass through without auth")
	}
}

func TestAuthMiddleware_NoHeaders_APIReturns401(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	r := httptest.NewRequest("GET", "/api/feeds", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_NoHeaders_PageReturns401HTML(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	r := httptest.NewRequest("GET", "/feeds", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	if !strings.Contains(w.Body.String(), "Not Authenticated") {
		t.Error("response should contain 'Not Authenticated'")
	}
}

func TestAuthMiddleware_WithExeDevHeaders(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	var gotUser *User
	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = GetUser(r.Context())
		w.WriteHeader(200)
	}))

	r := httptest.NewRequest("GET", "/api/feeds", http.NoBody)
	r.Header.Set("X-Exedev-Userid", "ext-123")
	r.Header.Set("X-Exedev-Email", "user@example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotUser == nil {
		t.Fatal("user not set in context")
	}
	if gotUser.ExternalID != "ext-123" {
		t.Errorf("external_id = %q", gotUser.ExternalID)
	}
	if gotUser.Email != "user@example.com" {
		t.Errorf("email = %q", gotUser.Email)
	}
}

func TestAuthMiddleware_WithProxyHeaders(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.AuthProvider = &ProxyHeaderProvider{}

	var gotUser *User
	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = GetUser(r.Context())
		w.WriteHeader(200)
	}))

	r := httptest.NewRequest("GET", "/api/feeds", http.NoBody)
	r.Header.Set("Remote-User", "proxy-user-1")
	r.Header.Set("Remote-Email", "proxy@example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotUser == nil {
		t.Fatal("user not set in context")
	}
	if gotUser.ExternalID != "proxy-user-1" {
		t.Errorf("external_id = %q, want proxy-user-1", gotUser.ExternalID)
	}
	if gotUser.Email != "proxy@example.com" {
		t.Errorf("email = %q, want proxy@example.com", gotUser.Email)
	}
}

func TestAuthMiddleware_WithCustomProxyHeaders(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.AuthProvider = &ProxyHeaderProvider{
		UserIDHeader: "X-Auth-User",
		EmailHeader:  "X-Auth-Email",
	}

	var gotUser *User
	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = GetUser(r.Context())
		w.WriteHeader(200)
	}))

	r := httptest.NewRequest("GET", "/api/feeds", http.NoBody)
	r.Header.Set("X-Auth-User", "custom-user")
	r.Header.Set("X-Auth-Email", "custom@example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotUser == nil {
		t.Fatal("user not set in context")
	}
	if gotUser.ExternalID != "custom-user" {
		t.Errorf("external_id = %q, want custom-user", gotUser.ExternalID)
	}
}

func TestAuthMiddleware_CachesUser(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	var callCount int
	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		u := GetUser(r.Context())
		if u == nil {
			t.Error("user should be set")
		}
		w.WriteHeader(200)
	}))

	// First request — populates cache
	r := httptest.NewRequest("GET", "/api/feeds", http.NoBody)
	r.Header.Set("X-Exedev-Userid", "cache-test-user")
	r.Header.Set("X-Exedev-Email", "cache@test.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("first request: got %d", w.Code)
	}

	// Second request — should use cache (same user)
	r = httptest.NewRequest("GET", "/api/feeds", http.NoBody)
	r.Header.Set("X-Exedev-Userid", "cache-test-user")
	r.Header.Set("X-Exedev-Email", "cache@test.com")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("second request: got %d", w.Code)
	}

	if callCount != 2 {
		t.Fatalf("expected handler called 2 times, got %d", callCount)
	}
}

func TestAuthMiddleware_CacheReturnsCorrectUser(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	var users []*User
	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		users = append(users, GetUser(r.Context()))
		w.WriteHeader(200)
	}))

	// Two different users
	for _, uid := range []string{"user-a", "user-b"} {
		r := httptest.NewRequest("GET", "/api/feeds", http.NoBody)
		r.Header.Set("X-Exedev-Userid", uid)
		r.Header.Set("X-Exedev-Email", uid+"@test.com")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("%s: got %d", uid, w.Code)
		}
	}

	if len(users) != 2 {
		t.Fatalf("expected 2, got %d", len(users))
	}
	if users[0].ExternalID != "user-a" {
		t.Errorf("user[0] = %q", users[0].ExternalID)
	}
	if users[1].ExternalID != "user-b" {
		t.Errorf("user[1] = %q", users[1].ExternalID)
	}
	if users[0].ID == users[1].ID {
		t.Error("different external IDs should have different DB IDs")
	}

	// Second request for user-a — should return same ID from cache
	r := httptest.NewRequest("GET", "/api/feeds", http.NoBody)
	r.Header.Set("X-Exedev-Userid", "user-a")
	r.Header.Set("X-Exedev-Email", "user-a@test.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if users[2].ID != users[0].ID {
		t.Errorf("cached user-a ID = %d, want %d", users[2].ID, users[0].ID)
	}
}

func TestAuthMiddleware_CachePerServer(t *testing.T) {
	// Each server's AuthMiddleware should have its own cache — ensures
	// parallel tests with different DBs don't interfere.
	t.Parallel()
	s1 := testServer(t)
	s2 := testServer(t)

	var user1, user2 *User
	h1 := s1.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user1 = GetUser(r.Context())
		w.WriteHeader(200)
	}))
	h2 := s2.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user2 = GetUser(r.Context())
		w.WriteHeader(200)
	}))

	// Same external ID, different server DBs
	for _, h := range []http.Handler{h1, h2} {
		r := httptest.NewRequest("GET", "/api/feeds", http.NoBody)
		r.Header.Set("X-Exedev-Userid", "shared-ext-id")
		r.Header.Set("X-Exedev-Email", "shared@test.com")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("got %d", w.Code)
		}
	}

	if user1 == nil || user2 == nil {
		t.Fatal("users should be set")
	}
	// Both should be valid users in their respective DBs
	if user1.ExternalID != "shared-ext-id" || user2.ExternalID != "shared-ext-id" {
		t.Error("external IDs should match")
	}
}

// --- Provider unit tests ---

func TestExeDevProvider_WithHeaders(t *testing.T) {
	t.Parallel()
	p := ExeDevProvider{}
	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set("X-Exedev-Userid", "u1")
	r.Header.Set("X-Exedev-Email", "u1@example.com")

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id == nil {
		t.Fatal("expected identity")
	}
	if id.ExternalID != "u1" {
		t.Errorf("ExternalID = %q", id.ExternalID)
	}
	if id.Email != "u1@example.com" {
		t.Errorf("Email = %q", id.Email)
	}
}

func TestExeDevProvider_NoHeaders(t *testing.T) {
	t.Parallel()
	p := ExeDevProvider{}
	r := httptest.NewRequest("GET", "/", http.NoBody)

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id != nil {
		t.Errorf("expected nil identity, got %+v", id)
	}
}

func TestProxyHeaderProvider_DefaultHeaders(t *testing.T) {
	t.Parallel()
	p := &ProxyHeaderProvider{}
	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set("Remote-User", "alice")
	r.Header.Set("Remote-Email", "alice@example.com")

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id == nil {
		t.Fatal("expected identity")
	}
	if id.ExternalID != "alice" {
		t.Errorf("ExternalID = %q", id.ExternalID)
	}
	if id.Email != "alice@example.com" {
		t.Errorf("Email = %q", id.Email)
	}
}

func TestProxyHeaderProvider_CustomHeaders(t *testing.T) {
	t.Parallel()
	p := &ProxyHeaderProvider{
		UserIDHeader: "X-Forwarded-User",
		EmailHeader:  "X-Forwarded-Email",
	}
	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set("X-Forwarded-User", "bob")
	r.Header.Set("X-Forwarded-Email", "bob@corp.com")

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id == nil {
		t.Fatal("expected identity")
	}
	if id.ExternalID != "bob" {
		t.Errorf("ExternalID = %q", id.ExternalID)
	}
	if id.Email != "bob@corp.com" {
		t.Errorf("Email = %q", id.Email)
	}
}

func TestProxyHeaderProvider_NoHeaders(t *testing.T) {
	t.Parallel()
	p := &ProxyHeaderProvider{}
	r := httptest.NewRequest("GET", "/", http.NoBody)

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id != nil {
		t.Errorf("expected nil identity, got %+v", id)
	}
}

func TestProxyHeaderProvider_UserIDOnly(t *testing.T) {
	t.Parallel()
	p := &ProxyHeaderProvider{}
	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set("Remote-User", "carol")
	// No email header

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id == nil {
		t.Fatal("expected identity")
	}
	if id.ExternalID != "carol" {
		t.Errorf("ExternalID = %q", id.ExternalID)
	}
	if id.Email != "" {
		t.Errorf("Email = %q, want empty", id.Email)
	}
}

func TestDevProvider(t *testing.T) {
	t.Parallel()
	p := DevProvider{}
	r := httptest.NewRequest("GET", "/", http.NoBody)

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id == nil {
		t.Fatal("expected identity")
	}
	if id.ExternalID != "dev-user" {
		t.Errorf("ExternalID = %q", id.ExternalID)
	}
	if id.Email != "dev@localhost" {
		t.Errorf("Email = %q", id.Email)
	}
}

func TestAuthMiddleware_WithTailscaleHeaders(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.AuthProvider = TailscaleProvider{}

	var gotUser *User
	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = GetUser(r.Context())
		w.WriteHeader(200)
	}))

	r := httptest.NewRequest("GET", "/api/feeds", http.NoBody)
	r.Header.Set("Tailscale-User-Login", "alice@example.com")
	r.Header.Set("Tailscale-User-Name", "Alice")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotUser == nil {
		t.Fatal("user not set in context")
	}
	if gotUser.ExternalID != "alice@example.com" {
		t.Errorf("external_id = %q, want alice@example.com", gotUser.ExternalID)
	}
	if gotUser.Email != "alice@example.com" {
		t.Errorf("email = %q, want alice@example.com", gotUser.Email)
	}
}

// --- Provider unit tests: TailscaleProvider ---

func TestTailscaleProvider_WithLogin(t *testing.T) {
	t.Parallel()
	p := TailscaleProvider{}
	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set("Tailscale-User-Login", "bob@tailnet.ts.net")
	r.Header.Set("Tailscale-User-Name", "Bob")

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id == nil {
		t.Fatal("expected identity")
	}
	if id.ExternalID != "bob@tailnet.ts.net" {
		t.Errorf("ExternalID = %q", id.ExternalID)
	}
	if id.Email != "bob@tailnet.ts.net" {
		t.Errorf("Email = %q", id.Email)
	}
}

func TestTailscaleProvider_LoginOnly(t *testing.T) {
	t.Parallel()
	p := TailscaleProvider{}
	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set("Tailscale-User-Login", "carol@example.com")
	// No name or profile pic headers

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id == nil {
		t.Fatal("expected identity")
	}
	if id.ExternalID != "carol@example.com" {
		t.Errorf("ExternalID = %q", id.ExternalID)
	}
	if id.Email != "carol@example.com" {
		t.Errorf("Email = %q", id.Email)
	}
}

func TestTailscaleProvider_NoHeaders(t *testing.T) {
	t.Parallel()
	p := TailscaleProvider{}
	r := httptest.NewRequest("GET", "/", http.NoBody)

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id != nil {
		t.Errorf("expected nil identity, got %+v", id)
	}
}

// TestAuthMiddleware_ProviderError verifies that an error from the
// provider results in a 500 response.
func TestAuthMiddleware_ProviderError(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.AuthProvider = errorProvider{}

	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	r := httptest.NewRequest("GET", "/api/feeds", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// errorProvider is a test helper that always returns an error.
type errorProvider struct{}

func (errorProvider) Authenticate(_ *http.Request) (*Identity, error) {
	return nil, http.ErrAbortHandler
}

// --- Provider unit tests: AutheliaProvider ---

func TestAutheliaProvider_WithAllHeaders(t *testing.T) {
	t.Parallel()
	p := AutheliaProvider{}
	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set("Remote-User", "alice")
	r.Header.Set("Remote-Name", "Alice Smith")
	r.Header.Set("Remote-Email", "alice@example.com")
	r.Header.Set("Remote-Groups", "admins,users")

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id == nil {
		t.Fatal("expected identity")
	}
	if id.ExternalID != "alice" {
		t.Errorf("ExternalID = %q, want alice", id.ExternalID)
	}
	if id.Email != "alice@example.com" {
		t.Errorf("Email = %q, want alice@example.com", id.Email)
	}
}

func TestAutheliaProvider_UserOnly(t *testing.T) {
	t.Parallel()
	p := AutheliaProvider{}
	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set("Remote-User", "bob")
	// No email, name, or groups headers

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id == nil {
		t.Fatal("expected identity")
	}
	if id.ExternalID != "bob" {
		t.Errorf("ExternalID = %q, want bob", id.ExternalID)
	}
	if id.Email != "" {
		t.Errorf("Email = %q, want empty", id.Email)
	}
}

func TestAutheliaProvider_NoHeaders(t *testing.T) {
	t.Parallel()
	p := AutheliaProvider{}
	r := httptest.NewRequest("GET", "/", http.NoBody)

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id != nil {
		t.Errorf("expected nil identity, got %+v", id)
	}
}

func TestAutheliaProvider_EmailOnly_NoUser(t *testing.T) {
	t.Parallel()
	p := AutheliaProvider{}
	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set("Remote-Email", "alice@example.com")
	// No Remote-User header

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id != nil {
		t.Errorf("expected nil identity without Remote-User, got %+v", id)
	}
}

func TestAuthMiddleware_WithAutheliaHeaders(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.AuthProvider = AutheliaProvider{}

	var gotUser *User
	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = GetUser(r.Context())
		w.WriteHeader(200)
	}))

	r := httptest.NewRequest("GET", "/api/feeds", http.NoBody)
	r.Header.Set("Remote-User", "carol")
	r.Header.Set("Remote-Name", "Carol Jones")
	r.Header.Set("Remote-Email", "carol@example.com")
	r.Header.Set("Remote-Groups", "users")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotUser == nil {
		t.Fatal("user not set in context")
	}
	if gotUser.ExternalID != "carol" {
		t.Errorf("external_id = %q, want carol", gotUser.ExternalID)
	}
	if gotUser.Email != "carol@example.com" {
		t.Errorf("email = %q, want carol@example.com", gotUser.Email)
	}
}
