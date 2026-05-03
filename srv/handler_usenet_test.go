package srv

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/newscientist101/feedreader/db/dbgen"
	"github.com/newscientist101/feedreader/srv/usenet"
)

// testServerWithUsenet builds a test Server with Usenet enabled and a test key.
func testServerWithUsenet(t *testing.T) *Server {
	t.Helper()
	s := testServer(t)

	// Use a deterministic 32-byte key for tests.
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	crypto, err := usenet.NewCrypto(key)
	if err != nil {
		t.Fatal(err)
	}
	s.UsenetConfig = &usenet.Config{
		Enabled: true,
		Crypto:  crypto,
	}
	return s
}

// testUser2 creates a second user with a distinct external ID.
func testUser2(t *testing.T, s *Server) (context.Context, *User) {
	t.Helper()
	q := dbgen.New(s.DB)
	dbUser, err := q.GetOrCreateUser(context.Background(), dbgen.GetOrCreateUserParams{
		ExternalID: "test-user-2",
		Email:      "test2@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	user := &User{ID: dbUser.ID, ExternalID: dbUser.ExternalID, Email: dbUser.Email}
	ctx := context.WithValue(context.Background(), userContextKey, user)
	return ctx, user
}

// --- GET /api/usenet/credentials ---

func TestAPIGetUsenetCredentials_Disabled(t *testing.T) {
	t.Parallel()
	s := testServer(t) // no UsenetConfig set
	s.UsenetConfig = &usenet.Config{Enabled: false}
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiGetUsenetCredentials, "GET", "/api/usenet/credentials", "", ctx)
	if w.Code != 503 {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestAPIGetUsenetCredentials_NotConfigured(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiGetUsenetCredentials, "GET", "/api/usenet/credentials", "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["enabled"] != true {
		t.Errorf("expected enabled=true, got %v", body["enabled"])
	}
	if body["configured"] != false {
		t.Errorf("expected configured=false, got %v", body["configured"])
	}
	if body["username"] != "" {
		t.Errorf("expected username=\"\", got %v", body["username"])
	}
}

func TestAPIGetUsenetCredentials_Configured(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	// Save credentials first.
	payload := `{"username":"testuser","password":"hunter2"}`
	w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials", payload, ctx)
	if w.Code != 200 {
		t.Fatalf("PUT failed: %d %s", w.Code, w.Body.String())
	}

	// Now GET should report configured.
	w = serveAPI(t, s.apiGetUsenetCredentials, "GET", "/api/usenet/credentials", "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["enabled"] != true {
		t.Errorf("expected enabled=true")
	}
	if body["configured"] != false && body["configured"] != true {
		t.Errorf("configured has unexpected value: %v", body["configured"])
	}
	if body["configured"] != true {
		t.Errorf("expected configured=true, got %v", body["configured"])
	}
	if body["username"] != "testuser" {
		t.Errorf("expected username=testuser, got %v", body["username"])
	}
	// Password must never be returned.
	if _, ok := body["password"]; ok {
		t.Error("response must not contain 'password' field")
	}
	if _, ok := body["password_enc"]; ok {
		t.Error("response must not contain 'password_enc' field")
	}
	if body["key_version"] != "v1" {
		t.Errorf("expected key_version=v1, got %v", body["key_version"])
	}
}

// --- PUT /api/usenet/credentials ---

func TestAPIPutUsenetCredentials_Disabled(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.UsenetConfig = &usenet.Config{Enabled: false}
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
		`{"username":"u","password":"p"}`, ctx)
	if w.Code != 503 {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestAPIPutUsenetCredentials_Success(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
		`{"username":"alice","password":"secret"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["username"] != "alice" {
		t.Errorf("expected username=alice, got %v", body["username"])
	}
	if _, ok := body["password"]; ok {
		t.Error("response must not contain 'password'")
	}
	if _, ok := body["password_enc"]; ok {
		t.Error("response must not contain 'password_enc'")
	}
}

func TestAPIPutUsenetCredentials_Overwrite(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	// First save.
	w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
		`{"username":"alice","password":"secret"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("first PUT failed: %d %s", w.Code, w.Body.String())
	}

	// Overwrite.
	w = serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
		`{"username":"bob","password":"newpass"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("second PUT failed: %d %s", w.Code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["username"] != "bob" {
		t.Errorf("expected username=bob after overwrite, got %v", body["username"])
	}
}

func TestAPIPutUsenetCredentials_MissingUsername(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
		`{"password":"secret"}`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAPIPutUsenetCredentials_MissingPassword(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
		`{"username":"alice"}`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAPIPutUsenetCredentials_InvalidBody(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
		"not-json", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAPIPutUsenetCredentials_AuthScoping(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx1, _ := testUser(t, s)
	// Create a second user.
	ctx2, user2 := testUser2(t, s)

	// user1 saves credentials.
	w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
		`{"username":"user1name","password":"pass1"}`, ctx1)
	if w.Code != 200 {
		t.Fatalf("user1 PUT failed: %d", w.Code)
	}

	// user2 should see no credentials.
	w = serveAPI(t, s.apiGetUsenetCredentials, "GET", "/api/usenet/credentials", "", ctx2)
	if w.Code != 200 {
		t.Fatalf("user2 GET failed: %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["configured"] != false {
		t.Errorf("user2 should not see user1's credentials, got configured=%v", body["configured"])
	}
	_ = user2
}

// --- DELETE /api/usenet/credentials ---

func TestAPIDeleteUsenetCredentials_Disabled(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.UsenetConfig = &usenet.Config{Enabled: false}
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiDeleteUsenetCredentials, "DELETE", "/api/usenet/credentials", "", ctx)
	if w.Code != 503 {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestAPIDeleteUsenetCredentials_NoRowOK(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	// Delete when nothing is stored — should still return 200.
	w := serveAPI(t, s.apiDeleteUsenetCredentials, "DELETE", "/api/usenet/credentials", "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	m := jsonBody(t, w)
	if m["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", m["status"])
	}
}

func TestAPIDeleteUsenetCredentials_DeletesCredentials(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	// Save first.
	w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
		`{"username":"alice","password":"secret"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("PUT failed: %d", w.Code)
	}

	// Delete.
	w = serveAPI(t, s.apiDeleteUsenetCredentials, "DELETE", "/api/usenet/credentials", "", ctx)
	if w.Code != 200 {
		t.Fatalf("DELETE failed: %d %s", w.Code, w.Body.String())
	}

	// GET should now show not configured.
	w = serveAPI(t, s.apiGetUsenetCredentials, "GET", "/api/usenet/credentials", "", ctx)
	if w.Code != 200 {
		t.Fatalf("GET after DELETE failed: %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["configured"] != false {
		t.Errorf("expected configured=false after delete, got %v", body["configured"])
	}
}
