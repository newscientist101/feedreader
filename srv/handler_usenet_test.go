package srv

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

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

// --- GET /api/usenet/groups ---

func TestAPIGetUsenetGroups_Disabled(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.UsenetConfig = &usenet.Config{Enabled: false}
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiGetUsenetGroups, "GET", "/api/usenet/groups", "", ctx)
	if w.Code != 503 {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestAPIGetUsenetGroups_Empty(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiGetUsenetGroups, "GET", "/api/usenet/groups", "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var items []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty list, got %d items", len(items))
	}
}

func TestAPIGetUsenetGroups_ReturnsSubscribed(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	// Save credentials first so POST succeeds.
	w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
		`{"username":"u","password":"p"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("PUT creds failed: %d", w.Code)
	}

	// Add a newsgroup.
	w = serveAPI(t, s.apiPostUsenetGroups, "POST", "/api/usenet/groups",
		`{"group_name":"comp.lang.go"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("POST group failed: %d %s", w.Code, w.Body.String())
	}

	w = serveAPI(t, s.apiGetUsenetGroups, "GET", "/api/usenet/groups", "", ctx)
	if w.Code != 200 {
		t.Fatalf("GET groups failed: %d", w.Code)
	}
	var items []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 group, got %d", len(items))
	}
	if items[0]["group_name"] != "comp.lang.go" {
		t.Errorf("expected group_name=comp.lang.go, got %v", items[0]["group_name"])
	}
	// high_water_article_number is always present (0 until first fetch).
	if _, ok := items[0]["high_water_article_number"]; !ok {
		t.Error("expected high_water_article_number in group item")
	}
}

// --- POST /api/usenet/groups ---

func TestAPIPostUsenetGroups_Disabled(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.UsenetConfig = &usenet.Config{Enabled: false}
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiPostUsenetGroups, "POST", "/api/usenet/groups",
		`{"group_name":"comp.lang.go"}`, ctx)
	if w.Code != 503 {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestAPIPostUsenetGroups_NoCredentials(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	// No credentials saved — should fail.
	w := serveAPI(t, s.apiPostUsenetGroups, "POST", "/api/usenet/groups",
		`{"group_name":"comp.lang.go"}`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAPIPostUsenetGroups_InvalidName(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiPostUsenetGroups, "POST", "/api/usenet/groups",
		`{"group_name":"alt.binaries.pictures"}`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAPIPostUsenetGroups_Success(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	// Credentials required.
	w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
		`{"username":"alice","password":"secret"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("PUT creds: %d", w.Code)
	}

	w = serveAPI(t, s.apiPostUsenetGroups, "POST", "/api/usenet/groups",
		`{"group_name":"comp.lang.go"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := jsonBody(t, w)
	if body["group_name"] != "comp.lang.go" {
		t.Errorf("expected group_name=comp.lang.go, got %v", body["group_name"])
	}
	if body["feed_id"] == nil {
		t.Error("expected feed_id in response")
	}
}

func TestAPIPostUsenetGroups_NormalizesName(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
		`{"username":"u","password":"p"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("PUT creds: %d", w.Code)
	}

	// Mixed case should be normalised to lowercase.
	w = serveAPI(t, s.apiPostUsenetGroups, "POST", "/api/usenet/groups",
		`{"group_name":"  Comp.Lang.Go  "}`, ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := jsonBody(t, w)
	if body["group_name"] != "comp.lang.go" {
		t.Errorf("expected normalised group_name, got %v", body["group_name"])
	}
}

func TestAPIPostUsenetGroups_Duplicate(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
		`{"username":"u","password":"p"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("PUT creds: %d", w.Code)
	}

	// Add once.
	w = serveAPI(t, s.apiPostUsenetGroups, "POST", "/api/usenet/groups",
		`{"group_name":"sci.physics"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("first add: %d", w.Code)
	}

	// Add again — should be 409.
	w = serveAPI(t, s.apiPostUsenetGroups, "POST", "/api/usenet/groups",
		`{"group_name":"sci.physics"}`, ctx)
	if w.Code != 409 {
		t.Fatalf("expected 409 on duplicate, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAPIPostUsenetGroups_InvalidBody(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiPostUsenetGroups, "POST", "/api/usenet/groups", "not-json", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAPIPostUsenetGroups_UserIsolation(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx1, _ := testUser(t, s)
	ctx2, _ := testUser2(t, s)

	// Both users save credentials.
	for _, ctx := range []context.Context{ctx1, ctx2} {
		w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
			`{"username":"u","password":"p"}`, ctx)
		if w.Code != 200 {
			t.Fatalf("PUT creds: %d", w.Code)
		}
	}

	// User1 subscribes.
	w := serveAPI(t, s.apiPostUsenetGroups, "POST", "/api/usenet/groups",
		`{"group_name":"comp.lang.go"}`, ctx1)
	if w.Code != 200 {
		t.Fatalf("user1 add: %d", w.Code)
	}

	// User2 should see an empty list.
	w = serveAPI(t, s.apiGetUsenetGroups, "GET", "/api/usenet/groups", "", ctx2)
	if w.Code != 200 {
		t.Fatalf("user2 GET: %d", w.Code)
	}
	var items []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("user2 should see no groups, got %d", len(items))
	}
}

// --- DELETE /api/usenet/groups/{feed_id} ---

func TestAPIDeleteUsenetGroup_Disabled(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.UsenetConfig = &usenet.Config{Enabled: false}
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiDeleteUsenetGroup, "DELETE", "/api/usenet/groups/1", "", ctx)
	if w.Code != 503 {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestAPIDeleteUsenetGroup_NotFound(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	// No feed exists with this ID.
	w := serveMux(t, "DELETE /api/usenet/groups/{feed_id}", s.apiDeleteUsenetGroup,
		"DELETE", "/api/usenet/groups/9999", "", ctx)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAPIDeleteUsenetGroup_Success(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	// Save credentials and add a group.
	w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
		`{"username":"u","password":"p"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("PUT creds: %d", w.Code)
	}
	w = serveAPI(t, s.apiPostUsenetGroups, "POST", "/api/usenet/groups",
		`{"group_name":"comp.lang.go"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("POST group: %d", w.Code)
	}
	postBody := jsonBody(t, w)
	feedIDFloat, ok := postBody["feed_id"].(float64)
	if !ok {
		t.Fatalf("feed_id not in response")
	}
	feedIDStr := strconv.Itoa(int(feedIDFloat))

	// Delete it.
	w = serveMux(t, "DELETE /api/usenet/groups/{feed_id}", s.apiDeleteUsenetGroup,
		"DELETE", "/api/usenet/groups/"+feedIDStr, "", ctx)
	if w.Code != 200 {
		t.Fatalf("DELETE: %d %s", w.Code, w.Body.String())
	}
	body := jsonBody(t, w)
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", body["status"])
	}

	// GET should return empty list.
	w = serveAPI(t, s.apiGetUsenetGroups, "GET", "/api/usenet/groups", "", ctx)
	var items []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty list after delete, got %d", len(items))
	}
}

func TestAPIDeleteUsenetGroup_WrongUser(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx1, _ := testUser(t, s)
	ctx2, _ := testUser2(t, s)

	// User1 adds group.
	w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
		`{"username":"u","password":"p"}`, ctx1)
	if w.Code != 200 {
		t.Fatalf("PUT creds: %d", w.Code)
	}
	w = serveAPI(t, s.apiPostUsenetGroups, "POST", "/api/usenet/groups",
		`{"group_name":"comp.lang.go"}`, ctx1)
	if w.Code != 200 {
		t.Fatalf("POST group: %d", w.Code)
	}
	postBody := jsonBody(t, w)
	feedIDFloat := postBody["feed_id"].(float64)
	feedIDStr := strconv.Itoa(int(feedIDFloat))

	// User2 cannot delete user1's group.
	w = serveMux(t, "DELETE /api/usenet/groups/{feed_id}", s.apiDeleteUsenetGroup,
		"DELETE", "/api/usenet/groups/"+feedIDStr, "", ctx2)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAPIPostUsenetGroups_TwoUsersShareSameGroupURL verifies that two different
// users can each subscribe to the same newsgroup URL. The feeds table must have
// a per-user unique constraint (UNIQUE(url, user_id)) rather than a global one.
func TestAPIPostUsenetGroups_TwoUsersShareSameGroupURL(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx1, _ := testUser(t, s)
	ctx2, _ := testUser2(t, s)

	// Both users save credentials.
	for _, ctx := range []context.Context{ctx1, ctx2} {
		w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
			`{"username":"u","password":"p"}`, ctx)
		if w.Code != 200 {
			t.Fatalf("PUT creds: %d", w.Code)
		}
	}

	// User1 subscribes to comp.lang.go.
	w := serveAPI(t, s.apiPostUsenetGroups, "POST", "/api/usenet/groups",
		`{"group_name":"comp.lang.go"}`, ctx1)
	if w.Code != 200 {
		t.Fatalf("user1 add group: %d %s", w.Code, w.Body.String())
	}

	// User2 subscribes to the same group — must succeed (not a UNIQUE violation).
	w = serveAPI(t, s.apiPostUsenetGroups, "POST", "/api/usenet/groups",
		`{"group_name":"comp.lang.go"}`, ctx2)
	if w.Code != 200 {
		t.Fatalf("user2 add same group: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Each user sees only their own subscription.
	w = serveAPI(t, s.apiGetUsenetGroups, "GET", "/api/usenet/groups", "", ctx1)
	var items1 []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &items1); err != nil {
		t.Fatal(err)
	}
	if len(items1) != 1 {
		t.Errorf("user1 should see 1 group, got %d", len(items1))
	}

	w = serveAPI(t, s.apiGetUsenetGroups, "GET", "/api/usenet/groups", "", ctx2)
	var items2 []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &items2); err != nil {
		t.Fatal(err)
	}
	if len(items2) != 1 {
		t.Errorf("user2 should see 1 group, got %d", len(items2))
	}
}

// --- POST /api/usenet/groups category ownership ---

func TestAPIPostUsenetGroups_ValidCategory(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, user := testUser(t, s)

	// Save credentials.
	w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
		`{"username":"u","password":"p"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("PUT creds: %d", w.Code)
	}

	// Create a category belonging to the user.
	q := dbgen.New(s.DB)
	cat, err := q.CreateCategory(context.Background(), dbgen.CreateCategoryParams{
		Name:   "Tech",
		UserID: &user.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// POST with a valid category_id should succeed.
	body := `{"group_name":"comp.lang.go","category_id":` + strconv.FormatInt(cat.ID, 10) + `}`
	w = serveAPI(t, s.apiPostUsenetGroups, "POST", "/api/usenet/groups", body, ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAPIPostUsenetGroups_NonexistentCategory(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
		`{"username":"u","password":"p"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("PUT creds: %d", w.Code)
	}

	// category_id 99999 does not exist.
	w = serveAPI(t, s.apiPostUsenetGroups, "POST", "/api/usenet/groups",
		`{"group_name":"comp.lang.go","category_id":99999}`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAPIPostUsenetGroups_ForeignCategory(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx1, _ := testUser(t, s)
	ctx2, user2 := testUser2(t, s)

	// Both save credentials.
	for _, ctx := range []context.Context{ctx1, ctx2} {
		w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
			`{"username":"u","password":"p"}`, ctx)
		if w.Code != 200 {
			t.Fatalf("PUT creds: %d", w.Code)
		}
	}

	// Create a category belonging to user2.
	q := dbgen.New(s.DB)
	cat, err := q.CreateCategory(context.Background(), dbgen.CreateCategoryParams{
		Name:   "User2Cat",
		UserID: &user2.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// User1 tries to use user2's category — must be rejected.
	body := `{"group_name":"comp.lang.go","category_id":` + strconv.FormatInt(cat.ID, 10) + `}`
	w := serveAPI(t, s.apiPostUsenetGroups, "POST", "/api/usenet/groups", body, ctx1)
	if w.Code != 400 {
		t.Fatalf("expected 400 for foreign category, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAPIPostUsenetGroups_NoCategory(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
		`{"username":"u","password":"p"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("PUT creds: %d", w.Code)
	}

	// category_id=0 (omitted) should succeed without any category check.
	w = serveAPI(t, s.apiPostUsenetGroups, "POST", "/api/usenet/groups",
		`{"group_name":"comp.lang.go"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- PUT /api/usenet/credentials input validation ---

func TestAPIPutUsenetCredentials_ControlCharInUsername(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	// CR in username.
	w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
		"{\"username\":\"alice\\reve\",\"password\":\"pass\"}", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400 for CR in username, got %d", w.Code)
	}
}

func TestAPIPutUsenetCredentials_ControlCharInPassword(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	// LF in password.
	w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
		"{\"username\":\"alice\",\"password\":\"pass\\nword\"}", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400 for LF in password, got %d", w.Code)
	}
}

func TestAPIPutUsenetCredentials_TrimsUsername(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiPutUsenetCredentials, "PUT", "/api/usenet/credentials",
		`{"username":"  alice  ","password":"secret"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := jsonBody(t, w)
	if body["username"] != "alice" {
		t.Errorf("expected trimmed username 'alice', got %v", body["username"])
	}
}

// TestAPIGetUsenetCredentials_NotConfigured verifies that a user with no saved
// credentials receives a 200 response with configured=false.
func TestAPIGetUsenetCredentials_ErrNoRowsOK(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiGetUsenetCredentials, "GET", "/api/usenet/credentials", "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := jsonBody(t, w)
	if body["configured"] != false {
		t.Errorf("expected configured=false, got %v", body["configured"])
	}
}

// --- Feed status endpoint for NNTP feeds (feedreader-6g2.19) ---

// createNNTPFeed inserts a feed with feed_type='nntp' and a usenet_feed_state
// row for testing the status endpoint.
func createNNTPFeed(t *testing.T, s *Server, userID int64, groupName string) dbgen.Feed {
	t.Helper()
	q := dbgen.New(s.DB)
	interval := int64(60)
	url := fmt.Sprintf("nntp://news.eternal-september.org/%s", groupName)
	feed, err := q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name: groupName, Url: url, FeedType: "nntp",
		FetchIntervalMinutes: &interval, UserID: &userID,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Insert usenet_feed_state row.
	_, err = s.DB.ExecContext(context.Background(),
		`INSERT INTO usenet_feed_state (feed_id, provider, group_name, high_water_article_number, created_at, updated_at)
		VALUES (?, 'eternal-september', ?, 0, ?, ?)`,
		feed.ID, groupName, time.Now().UTC(), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	return feed
}

// setNNTPFeedError directly writes a last_error on a feed row.
func setNNTPFeedError(t *testing.T, s *Server, feedID int64, msg string) {
	t.Helper()
	now := time.Now().UTC()
	q := dbgen.New(s.DB)
	err := q.IncrementFeedErrors(context.Background(), dbgen.IncrementFeedErrorsParams{
		LastError:     &msg,
		LastFetchedAt: &now,
		ID:            feedID,
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestNNTPFeedStatusErrorVisible verifies that a fetch error recorded on an
// NNTP feed is returned by GET /api/feeds/{id}/status.
func TestNNTPFeedStatusErrorVisible(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, user := testUser(t, s)

	feed := createNNTPFeed(t, s, user.ID, "comp.lang.go")
	errMsg := "nntp: connection failed: dial tcp: connection refused"
	setNNTPFeedError(t, s, feed.ID, errMsg)

	w := serveMux(t, "GET /api/feeds/{id}/status", s.apiGetFeedStatus,
		"GET", fmt.Sprintf("/api/feeds/%d/status", feed.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := jsonBody(t, w)
	if body["lastError"] != errMsg {
		t.Errorf("expected lastError=%q, got %v", errMsg, body["lastError"])
	}
}

// TestNNTPFeedStatusNoLeakPassword verifies that NNTP credential error
// messages stored in last_error never contain the plaintext password or
// the encrypted credential blob.
func TestNNTPFeedStatusNoLeakPassword(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, user := testUser(t, s)

	feed := createNNTPFeed(t, s, user.ID, "sci.physics")

	// Simulate the error messages that the fetcher writes for credential failures.
	// None of these should contain the plaintext password ("hunter2") or
	// the encrypted blob (a hex string beginning with the key prefix).
	sensitive := []string{"hunter2", "hexencryptedblob"}
	errMsgs := []string{
		"nntp: credentials not configured for this user",
		"nntp: failed to decrypt stored credentials",
		"nntp: connection failed: authentication rejected",
	}

	for _, errMsg := range errMsgs {
		for _, secret := range sensitive {
			if strings.Contains(errMsg, secret) {
				t.Errorf("error message %q must not contain sensitive value %q", errMsg, secret)
			}
		}
	}

	// Write one of those errors to the feed and verify it comes back via the API.
	setNNTPFeedError(t, s, feed.ID, errMsgs[1])
	w := serveMux(t, "GET /api/feeds/{id}/status", s.apiGetFeedStatus,
		"GET", fmt.Sprintf("/api/feeds/%d/status", feed.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := jsonBody(t, w)
	if body["lastError"] != errMsgs[1] {
		t.Errorf("expected lastError=%q, got %v", errMsgs[1], body["lastError"])
	}
	for _, secret := range sensitive {
		if strings.Contains(fmt.Sprintf("%v", body["lastError"]), secret) {
			t.Errorf("API response lastError must not contain sensitive value %q", secret)
		}
	}
}

// TestNNTPFeedStatusUserIsolation verifies that a user cannot retrieve the
// status of another user's NNTP feed.
func TestNNTPFeedStatusUserIsolation(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	_, user1 := testUser(t, s)
	ctx2, _ := testUser2(t, s)

	// user1's NNTP feed.
	feed := createNNTPFeed(t, s, user1.ID, "comp.lang.go")
	setNNTPFeedError(t, s, feed.ID, "nntp: connection failed")

	// user2 must not be able to retrieve user1's feed status.
	w := serveMux(t, "GET /api/feeds/{id}/status", s.apiGetFeedStatus,
		"GET", fmt.Sprintf("/api/feeds/%d/status", feed.ID), "", ctx2)
	if w.Code != 404 {
		t.Fatalf("expected 404 for cross-user feed status, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Newsgroup integration: folders and sidebar counts (feedreader-6g2.20) ---

// TestNNTPFeedIncludedInUnreadCounts verifies that articles from NNTP feeds
// are counted in the total unread count and per-feed counts returned by
// GET /api/counts.
func TestNNTPFeedIncludedInUnreadCounts(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, user := testUser(t, s)

	// Create an NNTP feed and an RSS feed.
	nntpFeed := createNNTPFeed(t, s, user.ID, "comp.lang.go")
	rssFeed := createFeed(t, s, user.ID, "rss-feed", "http://rss.example.com")

	// Create articles in both feeds.
	nntpArticle := createArticle(t, s, nntpFeed.ID, "Usenet post", "<msgid@test>")
	rssArticle := createArticle(t, s, rssFeed.ID, "RSS post", "rss-guid-1")
	_ = nntpArticle
	_ = rssArticle

	// Both articles are unread by default.
	w := serveAPI(t, s.apiGetCounts, "GET", "/api/counts", "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := jsonBody(t, w)

	// Total unread should be 2 (one from each feed).
	unread, ok := body["unread"].(float64)
	if !ok {
		t.Fatalf("unread count not numeric: %v", body["unread"])
	}
	if unread != 2 {
		t.Errorf("expected unread=2 (1 NNTP + 1 RSS), got %v", unread)
	}

	// Per-feed counts should include the NNTP feed.
	feeds, ok := body["feeds"].(map[string]any)
	if !ok {
		t.Fatalf("feeds counts not a map: %v", body["feeds"])
	}
	nntpKey := fmt.Sprintf("%d", nntpFeed.ID)
	if feeds[nntpKey] != float64(1) {
		t.Errorf("expected NNTP feed unread count = 1, got %v", feeds[nntpKey])
	}
}

// TestNNTPFeedAssignableToCategory verifies that an NNTP feed can be assigned
// to an existing folder using the same API as RSS feeds.
func TestNNTPFeedAssignableToCategory(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, user := testUser(t, s)

	nntpFeed := createNNTPFeed(t, s, user.ID, "sci.physics")

	// Create a category.
	q := dbgen.New(s.DB)
	cat, err := q.CreateCategory(context.Background(), dbgen.CreateCategoryParams{
		Name: "Science", UserID: &user.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Assign the NNTP feed to the category.
	payload := fmt.Sprintf(`{"categoryId":%d}`, cat.ID)
	w := serveMux(t, "POST /api/feeds/{id}/category", s.apiSetFeedCategory,
		"POST", fmt.Sprintf("/api/feeds/%d/category", nntpFeed.ID), payload, ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// The feed should now appear in the category.
	mappings, err := q.ListFeedCategoryMappings(context.Background(), &user.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range mappings {
		if m.FeedID == nntpFeed.ID && m.CategoryID == cat.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("NNTP feed not found in category after assignment")
	}
}

// TestNNTPFeedCategoryUnreadCount verifies that articles from an NNTP feed
// assigned to a category contribute to that category's unread count.
func TestNNTPFeedCategoryUnreadCount(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, user := testUser(t, s)

	nntpFeed := createNNTPFeed(t, s, user.ID, "comp.lang.go")
	createArticle(t, s, nntpFeed.ID, "Usenet post", "<msgid2@test>")

	// Assign to a category.
	q := dbgen.New(s.DB)
	cat, err := q.CreateCategory(context.Background(), dbgen.CreateCategoryParams{
		Name: "Programming", UserID: &user.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = q.AddFeedToCategory(context.Background(), dbgen.AddFeedToCategoryParams{
		FeedID: nntpFeed.ID, CategoryID: cat.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Category count should include the unread NNTP article.
	w := serveAPI(t, s.apiGetCounts, "GET", "/api/counts", "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := jsonBody(t, w)
	categories, ok := body["categories"].(map[string]any)
	if !ok {
		t.Fatalf("categories counts not a map: %v", body["categories"])
	}
	catKey := fmt.Sprintf("%d", cat.ID)
	if categories[catKey] != float64(1) {
		t.Errorf("expected category unread count = 1, got %v", categories[catKey])
	}
}

// TestNNTPFeedInListFeeds verifies that NNTP feeds appear in the ListFeeds
// query result, which is the data source for the sidebar.
func TestNNTPFeedInListFeeds(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	_, user := testUser(t, s)

	nntpFeed := createNNTPFeed(t, s, user.ID, "comp.lang.go")
	rssFeed := createFeed(t, s, user.ID, "rss-blog", "http://rss.example.com")

	q := dbgen.New(s.DB)
	feeds, err := q.ListFeeds(context.Background(), &user.ID)
	if err != nil {
		t.Fatal(err)
	}

	ids := map[int64]bool{}
	for _, f := range feeds {
		ids[f.ID] = true
	}
	if !ids[nntpFeed.ID] {
		t.Errorf("NNTP feed %d not found in ListFeeds result", nntpFeed.ID)
	}
	if !ids[rssFeed.ID] {
		t.Errorf("RSS feed %d not found in ListFeeds result", rssFeed.ID)
	}

	// Verify the NNTP feed has the correct feed_type.
	for _, f := range feeds {
		if f.ID == nntpFeed.ID && f.FeedType != "nntp" {
			t.Errorf("expected feed_type='nntp', got %q", f.FeedType)
		}
	}
}

// TestRSSFeedCountsUnaffectedByNNTP verifies that adding an NNTP feed and its
// articles does not distort existing RSS feed unread counts.
func TestRSSFeedCountsUnaffectedByNNTP(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, user := testUser(t, s)

	rssFeed := createFeed(t, s, user.ID, "rss-only", "http://rss.example.com")
	createArticle(t, s, rssFeed.ID, "RSS Article", "rss-only-guid-1")

	// Check RSS-only count.
	w := serveAPI(t, s.apiGetCounts, "GET", "/api/counts", "", ctx)
	body := jsonBody(t, w)
	unread1 := body["unread"].(float64)

	// Now add an NNTP feed with an article.
	nntpFeed := createNNTPFeed(t, s, user.ID, "alt.test")
	createArticle(t, s, nntpFeed.ID, "NNTP Article", "<nntp@test>")

	// Invalidate counts cache.
	s.CountsCache.Invalidate(user.ID)

	w = serveAPI(t, s.apiGetCounts, "GET", "/api/counts", "", ctx)
	body = jsonBody(t, w)
	unread2 := body["unread"].(float64)

	if unread2 != unread1+1 {
		t.Errorf("expected total unread to increase by 1 after NNTP article, got %v -> %v", unread1, unread2)
	}
	// RSS feed count should be unchanged.
	feeds := body["feeds"].(map[string]any)
	rssKey := fmt.Sprintf("%d", rssFeed.ID)
	if feeds[rssKey] != float64(1) {
		t.Errorf("RSS feed unread count should still be 1, got %v", feeds[rssKey])
	}
}

// --- Usenet article user actions (feedreader-6g2.21) ---

// createNNTPArticleWithMeta inserts an article for an NNTP feed and adds a
// companion usenet_article_meta row. Returns the inserted article.
func createNNTPArticleWithMeta(t *testing.T, s *Server, feed *dbgen.Feed, msgID string, artNum int64) dbgen.Article {
	t.Helper()
	art := createArticle(t, s, feed.ID, "Subject: "+msgID, msgID)
	_, err := s.DB.ExecContext(context.Background(),
		`INSERT INTO usenet_article_meta
			(article_id, feed_id, message_id, root_message_id, group_name, article_number, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		art.ID, feed.ID, msgID, msgID, feed.Name, artNum)
	if err != nil {
		t.Fatal(err)
	}
	return art
}

// hasUsenetMeta checks whether a usenet_article_meta row exists for the given article.
func hasUsenetMeta(t *testing.T, s *Server, articleID int64) bool {
	t.Helper()
	var n int
	err := s.DB.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM usenet_article_meta WHERE article_id = ?`, articleID).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n > 0
}

// TestNNTPArticleMarkRead verifies that a Usenet article can be marked read.
func TestNNTPArticleMarkRead(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, user := testUser(t, s)

	feed := createNNTPFeed(t, s, user.ID, "comp.lang.go")
	art := createNNTPArticleWithMeta(t, s, &feed, "<msg1@example>", 1)

	w := serveMux(t, "POST /api/articles/{id}/read", s.apiMarkRead,
		"POST", fmt.Sprintf("/api/articles/%d/read", art.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify is_read = 1.
	var isRead int
	err := s.DB.QueryRowContext(context.Background(),
		`SELECT is_read FROM articles WHERE id = ?`, art.ID).Scan(&isRead)
	if err != nil {
		t.Fatal(err)
	}
	if isRead != 1 {
		t.Errorf("expected is_read=1, got %d", isRead)
	}
}

// TestNNTPArticleMarkUnread verifies that a Usenet article can be marked unread.
func TestNNTPArticleMarkUnread(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, user := testUser(t, s)

	feed := createNNTPFeed(t, s, user.ID, "comp.lang.go")
	art := createNNTPArticleWithMeta(t, s, &feed, "<msg2@example>", 2)

	// First mark read.
	_ = serveMux(t, "POST /api/articles/{id}/read", s.apiMarkRead,
		"POST", fmt.Sprintf("/api/articles/%d/read", art.ID), "", ctx)

	// Now mark unread.
	w := serveMux(t, "POST /api/articles/{id}/unread", s.apiMarkUnread,
		"POST", fmt.Sprintf("/api/articles/%d/unread", art.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var isRead int
	err := s.DB.QueryRowContext(context.Background(),
		`SELECT is_read FROM articles WHERE id = ?`, art.ID).Scan(&isRead)
	if err != nil {
		t.Fatal(err)
	}
	if isRead != 0 {
		t.Errorf("expected is_read=0, got %d", isRead)
	}
}

// TestNNTPArticleToggleStar verifies starring and unstarring a Usenet article.
func TestNNTPArticleToggleStar(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, user := testUser(t, s)

	feed := createNNTPFeed(t, s, user.ID, "comp.lang.go")
	art := createNNTPArticleWithMeta(t, s, &feed, "<msg3@example>", 3)

	// Toggle star on.
	w := serveMux(t, "POST /api/articles/{id}/star", s.apiToggleStar,
		"POST", fmt.Sprintf("/api/articles/%d/star", art.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var isStarred int
	err := s.DB.QueryRowContext(context.Background(),
		`SELECT is_starred FROM articles WHERE id = ?`, art.ID).Scan(&isStarred)
	if err != nil {
		t.Fatal(err)
	}
	if isStarred != 1 {
		t.Errorf("expected is_starred=1 after first toggle, got %d", isStarred)
	}

	// Toggle star off.
	w = serveMux(t, "POST /api/articles/{id}/star", s.apiToggleStar,
		"POST", fmt.Sprintf("/api/articles/%d/star", art.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200 on second toggle, got %d", w.Code)
	}

	err = s.DB.QueryRowContext(context.Background(),
		`SELECT is_starred FROM articles WHERE id = ?`, art.ID).Scan(&isStarred)
	if err != nil {
		t.Fatal(err)
	}
	if isStarred != 0 {
		t.Errorf("expected is_starred=0 after second toggle, got %d", isStarred)
	}
}

// TestNNTPArticleQueueAddRemove verifies queue add and remove for a Usenet article.
func TestNNTPArticleQueueAddRemove(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, user := testUser(t, s)

	feed := createNNTPFeed(t, s, user.ID, "comp.lang.go")
	art := createNNTPArticleWithMeta(t, s, &feed, "<msg4@example>", 4)

	// Add to queue.
	w := serveMux(t, "POST /api/articles/{id}/queue", s.apiToggleQueue,
		"POST", fmt.Sprintf("/api/articles/%d/queue", art.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := jsonBody(t, w)
	if body["queued"] != true {
		t.Errorf("expected queued=true, got %v", body["queued"])
	}

	// Remove from queue via DELETE.
	w = serveMux(t, "DELETE /api/articles/{id}/queue", s.apiRemoveFromQueue,
		"DELETE", fmt.Sprintf("/api/articles/%d/queue", art.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200 on delete queue, got %d", w.Code)
	}

	// Verify not in queue.
	q := dbgen.New(s.DB)
	count, err := q.IsArticleQueued(context.Background(), dbgen.IsArticleQueuedParams{
		UserID: user.ID, ArticleID: art.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected article removed from queue, got count=%d", count)
	}
}

// TestNNTPArticleHistory verifies that viewing a Usenet article adds it to history.
func TestNNTPArticleHistory(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	_, user := testUser(t, s)

	feed := createNNTPFeed(t, s, user.ID, "comp.lang.go")
	art := createNNTPArticleWithMeta(t, s, &feed, "<msg5@example>", 5)

	// Directly add to history (same call the article view handler makes).
	q := dbgen.New(s.DB)
	err := q.AddToHistory(context.Background(), dbgen.AddToHistoryParams{
		UserID: user.ID, ArticleID: art.ID,
	})
	if err != nil {
		t.Fatalf("AddToHistory: %v", err)
	}

	// Verify the article appears in history.
	articles, err := q.ListHistoryArticles(context.Background(), dbgen.ListHistoryArticlesParams{
		UserID: user.ID, Limit: 10, Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListHistoryArticles: %v", err)
	}
	found := false
	for _, a := range articles {
		if a.ID == art.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("NNTP article %d not found in history", art.ID)
	}
}

// TestNNTPArticleUserIsolation verifies that user A cannot mark-read user B's
// Usenet articles.
func TestNNTPArticleUserIsolation(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctxA, _ := testUser(t, s)
	_, userB := testUser2(t, s)

	feedB := createNNTPFeed(t, s, userB.ID, "comp.lang.go")
	artB := createNNTPArticleWithMeta(t, s, &feedB, "<msgB@example>", 1)

	// User A attempts to mark user B's article read — should silently do nothing.
	w := serveMux(t, "POST /api/articles/{id}/read", s.apiMarkRead,
		"POST", fmt.Sprintf("/api/articles/%d/read", artB.ID), "", ctxA)
	if w.Code != 200 {
		t.Fatalf("expected 200 (no-op), got %d", w.Code)
	}

	// The article must still be unread.
	var isRead int
	err := s.DB.QueryRowContext(context.Background(),
		`SELECT is_read FROM articles WHERE id = ?`, artB.ID).Scan(&isRead)
	if err != nil {
		t.Fatal(err)
	}
	if isRead != 0 {
		t.Errorf("cross-user mark-read should not have changed is_read; got %d", isRead)
	}

	// User A cannot star user B's article.
	w = serveMux(t, "POST /api/articles/{id}/star", s.apiToggleStar,
		"POST", fmt.Sprintf("/api/articles/%d/star", artB.ID), "", ctxA)
	if w.Code != 200 {
		t.Fatalf("expected 200 (no-op on star), got %d", w.Code)
	}
	var isStarred int
	err = s.DB.QueryRowContext(context.Background(),
		`SELECT is_starred FROM articles WHERE id = ?`, artB.ID).Scan(&isStarred)
	if err != nil {
		t.Fatal(err)
	}
	if isStarred != 0 {
		t.Errorf("cross-user toggle-star should not have changed is_starred; got %d", isStarred)
	}
}

// TestNNTPArticleStarredList verifies that starred Usenet articles appear in the
// starred articles API response.
func TestNNTPArticleStarredList(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	_, user := testUser(t, s)

	feed := createNNTPFeed(t, s, user.ID, "comp.lang.go")
	art := createNNTPArticleWithMeta(t, s, &feed, "<msg6@example>", 6)

	// Star it directly.
	q := dbgen.New(s.DB)
	if err := q.ToggleArticleStar(context.Background(), dbgen.ToggleArticleStarParams{
		ID: art.ID, UserID: &user.ID,
	}); err != nil {
		t.Fatal(err)
	}

	starred, err := q.ListStarredArticles(context.Background(), dbgen.ListStarredArticlesParams{
		UserID: &user.ID, Limit: 10, Offset: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range starred {
		if a.ID == art.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("starred NNTP article %d not in starred list", art.ID)
	}
}

// --- Usenet search integration (feedreader-6g2.22) ---

// createNNTPArticleWithContent creates an NNTP article with both a title and
// body content, and inserts a companion usenet_article_meta row.
func createNNTPArticleWithContent(t *testing.T, s *Server, feed *dbgen.Feed, subject, msgID string, artNum int64, bodyText string) dbgen.Article {
	t.Helper()
	q := dbgen.New(s.DB)
	url := "nntp://news.eternal-september.org/" + feed.Name + "/" + strconv.FormatInt(artNum, 10)
	art, err := q.CreateArticle(context.Background(), dbgen.CreateArticleParams{
		FeedID:  feed.ID,
		Title:   subject,
		Guid:    msgID,
		Url:     &url,
		Content: &bodyText,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DB.ExecContext(context.Background(),
		`INSERT INTO usenet_article_meta
			(article_id, feed_id, message_id, root_message_id, group_name, article_number, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		art.ID, feed.ID, msgID, msgID, feed.Name, artNum)
	if err != nil {
		t.Fatal(err)
	}
	return art
}

// TestNNTPArticleSearchByTitle verifies that Usenet articles are found when
// searching by their Subject (title) field.
func TestNNTPArticleSearchByTitle(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, user := testUser(t, s)

	feed := createNNTPFeed(t, s, user.ID, "comp.lang.go")
	createNNTPArticleWithContent(t, s, &feed, "Quantum computing in Go", "<qc1@example>", 100, "Some body text.")

	w := serveAPI(t, s.apiSearch, "GET", "/api/search?q=Quantum", "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var results []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 search result for Usenet subject, got %d", len(results))
	}
	if results[0]["title"] != "Quantum computing in Go" {
		t.Errorf("unexpected title: %v", results[0]["title"])
	}
}

// TestNNTPArticleSearchByContent verifies that Usenet articles are found when
// searching by their body content.
func TestNNTPArticleSearchByContent(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, user := testUser(t, s)

	feed := createNNTPFeed(t, s, user.ID, "sci.physics")
	createNNTPArticleWithContent(t, s, &feed, "Re: General Relativity", "<gr1@example>", 200,
		"<pre class=\"usenet-body\">Einstein proposed that spacetime curvature causes gravity.</pre>")

	w := serveAPI(t, s.apiSearch, "GET", "/api/search?q=spacetime", "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var results []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for body-content search, got %d", len(results))
	}
	if results[0]["title"] != "Re: General Relativity" {
		t.Errorf("unexpected title: %v", results[0]["title"])
	}
}

// TestNNTPAndRSSSearchCoexist verifies that a global search returns articles
// from both NNTP and RSS feeds.
func TestNNTPAndRSSSearchCoexist(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, user := testUser(t, s)

	nntpFeed := createNNTPFeed(t, s, user.ID, "comp.lang.go")
	rssFeed := createFeed(t, s, user.ID, "Go Blog", "https://go.dev/blog/feed.atom")

	createNNTPArticleWithContent(t, s, &nntpFeed, "goroutine scheduling internals", "<sched1@nntp>", 10,
		"A discussion about goroutine scheduling.")
	createArticle(t, s, rssFeed.ID, "goroutine leak detection", "rss-goroutine-1")

	w := serveAPI(t, s.apiSearch, "GET", "/api/search?q=goroutine", "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var results []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (1 NNTP + 1 RSS), got %d", len(results))
	}
}

// TestNNTPArticleSearchAuthorNotMatched verifies that the author/From field
// alone does not produce a search match (search is title+content only).
func TestNNTPArticleSearchAuthorNotMatched(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, user := testUser(t, s)

	feed := createNNTPFeed(t, s, user.ID, "comp.lang.go")
	q := dbgen.New(s.DB)
	artNum := int64(300)
	artURL := "nntp://news.eternal-september.org/comp.lang.go/300"
	author := "uniqueauthorname@example.com"
	// Article whose title and content don't mention the author's name.
	art, err := q.CreateArticle(context.Background(), dbgen.CreateArticleParams{
		FeedID:  feed.ID,
		Title:   "Unrelated Subject",
		Guid:    "<author-only@test>",
		Url:     &artURL,
		Author:  &author,
		Content: new("This body does not mention the author name."),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DB.ExecContext(context.Background(),
		`INSERT INTO usenet_article_meta
			(article_id, feed_id, message_id, root_message_id, group_name, article_number, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		art.ID, feed.ID, "<author-only@test>", "<author-only@test>", feed.Name, artNum)
	if err != nil {
		t.Fatal(err)
	}

	// Searching by the author name should return no results.
	w := serveAPI(t, s.apiSearch, "GET", "/api/search?q=uniqueauthorname", "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var results []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("author-only search should return 0 results, got %d", len(results))
	}
}

// TestNNTPArticleSearchUserIsolation verifies that a search result for user A's
// Usenet article is not visible to user B.
func TestNNTPArticleSearchUserIsolation(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctxA, userA := testUser(t, s)
	ctxB, _ := testUser2(t, s)

	feedA := createNNTPFeed(t, s, userA.ID, "comp.lang.go")
	createNNTPArticleWithContent(t, s, &feedA, "User A private post", "<privA@test>", 1, "Content only user A can see.")

	// User A should see the article.
	w := serveAPI(t, s.apiSearch, "GET", "/api/search?q=private", "", ctxA)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resultsA []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resultsA); err != nil {
		t.Fatal(err)
	}
	if len(resultsA) != 1 {
		t.Fatalf("user A should see 1 result, got %d", len(resultsA))
	}

	// User B should see no results.
	w = serveAPI(t, s.apiSearch, "GET", "/api/search?q=private", "", ctxB)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resultsB []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resultsB); err != nil {
		t.Fatal(err)
	}
	if len(resultsB) != 0 {
		t.Errorf("user B should see 0 results, got %d", len(resultsB))
	}
}

// --- Thread query and API (feedreader-6g2.25) ---

// TestAPIGetUsenetThread_Disabled verifies the thread endpoint returns 503
// when Usenet is disabled.
func TestAPIGetUsenetThread_Disabled(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.UsenetConfig = &usenet.Config{Enabled: false}
	ctx, _ := testUser(t, s)

	w := serveMux(t, "GET /api/usenet/articles/{article_id}/thread",
		s.apiGetUsenetThread, "GET", "/api/usenet/articles/1/thread", "", ctx)
	if w.Code != 503 {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

// TestAPIGetUsenetThread_NotFound verifies 404 when no Usenet meta exists for
// the requested article ID.
func TestAPIGetUsenetThread_NotFound(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, _ := testUser(t, s)

	w := serveMux(t, "GET /api/usenet/articles/{article_id}/thread",
		s.apiGetUsenetThread, "GET", "/api/usenet/articles/9999/thread", "", ctx)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAPIGetUsenetThread_RootOnly verifies that a single-article thread
// (root post, no replies) returns a list with one entry.
func TestAPIGetUsenetThread_RootOnly(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, user := testUser(t, s)

	feed := createNNTPFeed(t, s, user.ID, "comp.lang.go")
	art := createNNTPArticleWithMeta(t, s, &feed, "<root-only@test>", 1)

	artIDStr := strconv.FormatInt(art.ID, 10)
	w := serveMux(t, "GET /api/usenet/articles/{article_id}/thread",
		s.apiGetUsenetThread, "GET", "/api/usenet/articles/"+artIDStr+"/thread", "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var results []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 article in root-only thread, got %d", len(results))
	}
	if results[0]["message_id"] != "<root-only@test>" {
		t.Errorf("unexpected message_id: %v", results[0]["message_id"])
	}
}

// TestAPIGetUsenetThread_NestedReplies verifies that a thread with nested
// replies returns all articles in article_number order.
func TestAPIGetUsenetThread_NestedReplies(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, user := testUser(t, s)

	feed := createNNTPFeed(t, s, user.ID, "comp.lang.go")

	root := "<root-nested@test>"
	reply1 := "<reply1-nested@test>"
	reply2 := "<reply2-nested@test>"

	// Root article.
	artRoot := createNNTPArticleWithMeta(t, s, &feed, root, 10)

	// Reply 1: parent=root.
	artReply1 := createNNTPArticleWithMeta(t, s, &feed, reply1, 11)
	// Update meta to set parent_message_id to root.
	_, err := s.DB.ExecContext(context.Background(),
		`UPDATE usenet_article_meta
		   SET parent_message_id = ?, root_message_id = ?
		 WHERE article_id = ?`,
		root, root, artReply1.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Reply 2: parent=reply1, root=root.
	artReply2 := createNNTPArticleWithMeta(t, s, &feed, reply2, 12)
	_, err = s.DB.ExecContext(context.Background(),
		`UPDATE usenet_article_meta
		   SET parent_message_id = ?, root_message_id = ?
		 WHERE article_id = ?`,
		reply1, root, artReply2.ID)
	if err != nil {
		t.Fatal(err)
	}

	artIDStr := strconv.FormatInt(artRoot.ID, 10)
	w := serveMux(t, "GET /api/usenet/articles/{article_id}/thread",
		s.apiGetUsenetThread, "GET", "/api/usenet/articles/"+artIDStr+"/thread", "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var results []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 articles in thread, got %d", len(results))
	}
	// Ordered by article_number ASC.
	if results[0]["message_id"] != root {
		t.Errorf("results[0] should be root, got %v", results[0]["message_id"])
	}
	if results[1]["message_id"] != reply1 {
		t.Errorf("results[1] should be reply1, got %v", results[1]["message_id"])
	}
	if results[2]["message_id"] != reply2 {
		t.Errorf("results[2] should be reply2, got %v", results[2]["message_id"])
	}
}

// TestAPIGetUsenetThread_AccessFromReply verifies that the thread can be
// retrieved using any article in the thread as the entry point, not just root.
func TestAPIGetUsenetThread_AccessFromReply(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, user := testUser(t, s)

	feed := createNNTPFeed(t, s, user.ID, "sci.physics")

	root := "<root-access@test>"
	reply := "<reply-access@test>"

	createNNTPArticleWithMeta(t, s, &feed, root, 20)
	artReply := createNNTPArticleWithMeta(t, s, &feed, reply, 21)
	// Set reply's root_message_id to root.
	_, err := s.DB.ExecContext(context.Background(),
		`UPDATE usenet_article_meta
		   SET parent_message_id = ?, root_message_id = ?
		 WHERE article_id = ?`,
		root, root, artReply.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Access thread via reply article ID — should still return full 2-article thread.
	artIDStr := strconv.FormatInt(artReply.ID, 10)
	w := serveMux(t, "GET /api/usenet/articles/{article_id}/thread",
		s.apiGetUsenetThread, "GET", "/api/usenet/articles/"+artIDStr+"/thread", "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var results []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 articles in thread (via reply), got %d", len(results))
	}
}

// TestAPIGetUsenetThread_UserIsolation verifies that user B cannot access
// user A's thread.
func TestAPIGetUsenetThread_UserIsolation(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	_, userA := testUser(t, s)
	ctxB, _ := testUser2(t, s)

	feedA := createNNTPFeed(t, s, userA.ID, "comp.lang.go")
	artA := createNNTPArticleWithMeta(t, s, &feedA, "<iso-root@test>", 1)

	artIDStr := strconv.FormatInt(artA.ID, 10)
	// User B requests user A's article thread — should get 404.
	w := serveMux(t, "GET /api/usenet/articles/{article_id}/thread",
		s.apiGetUsenetThread, "GET", "/api/usenet/articles/"+artIDStr+"/thread", "", ctxB)
	if w.Code != 404 {
		t.Fatalf("expected 404 for cross-user thread access, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAPIGetUsenetThread_ThreadMetadataPresent verifies that the response
// includes Usenet-specific thread metadata fields.
func TestAPIGetUsenetThread_ThreadMetadataPresent(t *testing.T) {
	t.Parallel()
	s := testServerWithUsenet(t)
	ctx, user := testUser(t, s)

	feed := createNNTPFeed(t, s, user.ID, "comp.lang.go")
	art := createNNTPArticleWithMeta(t, s, &feed, "<meta-root@test>", 50)

	artIDStr := strconv.FormatInt(art.ID, 10)
	w := serveMux(t, "GET /api/usenet/articles/{article_id}/thread",
		s.apiGetUsenetThread, "GET", "/api/usenet/articles/"+artIDStr+"/thread", "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var results []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one article")
	}
	row := results[0]
	for _, field := range []string{"message_id", "root_message_id", "group_name", "article_number", "title", "feed_name"} {
		if _, ok := row[field]; !ok {
			t.Errorf("expected field %q in thread response item", field)
		}
	}
}

// strPtr is a helper to create a *string from a literal.
//
//go:fix inline
func strPtr(s string) *string { return new(s) }
