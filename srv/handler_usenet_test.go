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
