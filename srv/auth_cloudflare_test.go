package srv

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

// testCFKey generates an ECDSA P-256 key pair for testing.
func testCFKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// testCFJWKS builds a JWKS JSON response containing the public key.
func testCFJWKS(t *testing.T, kid string, pub *ecdsa.PublicKey) []byte {
	t.Helper()
	jwks := cfJWKSResponse{
		Keys: []cfJWK{{
			Kid: kid,
			Kty: "EC",
			Alg: "ES256",
			Crv: "P-256",
			X:   base64.RawURLEncoding.EncodeToString(pub.X.Bytes()),
			Y:   base64.RawURLEncoding.EncodeToString(pub.Y.Bytes()),
		}},
	}
	data, err := json.Marshal(jwks)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// testCFToken creates a signed JWT for testing.
func testCFToken(t *testing.T, key *ecdsa.PrivateKey, kid, email, teamDomain string, aud []string, expiry time.Time) string {
	t.Helper()
	claims := jwt.MapClaims{
		"email": email,
		"iss":   fmt.Sprintf("https://%s.cloudflareaccess.com", teamDomain),
		"aud":   aud,
		"exp":   expiry.Unix(),
		"iat":   time.Now().Add(-time.Minute).Unix(),
		"nbf":   time.Now().Add(-time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = kid

	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

// mockCFHTTPClient returns a mock HTTPClient that serves the given JWKS JSON.
type mockCFHTTPClient struct {
	response     []byte
	statusCode   int
	requestsMade int
}

func (m *mockCFHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	m.requestsMade++
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       io.NopCloser(bytes.NewReader(m.response)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

func TestCloudflareAccessProvider_ValidJWT(t *testing.T) {
	t.Parallel()
	key := testCFKey(t)
	kid := "test-key-1"
	teamDomain := "myteam"
	email := "alice@example.com"
	audience := "test-aud-tag"

	jwksData := testCFJWKS(t, kid, &key.PublicKey)
	mockClient := &mockCFHTTPClient{response: jwksData, statusCode: 200}

	p := &CloudflareAccessProvider{
		TeamDomain: teamDomain,
		Audience:   audience,
		HTTPClient: mockClient,
	}

	tokenStr := testCFToken(t, key, kid, email, teamDomain, []string{audience}, time.Now().Add(time.Hour))

	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set(CfAccessUserEmailHeader, email)
	r.Header.Set(CfAccessJWTHeader, tokenStr)

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id == nil {
		t.Fatal("expected identity")
	}
	if id.ExternalID != email {
		t.Errorf("ExternalID = %q, want %q", id.ExternalID, email)
	}
	if id.Email != email {
		t.Errorf("Email = %q, want %q", id.Email, email)
	}
}

func TestCloudflareAccessProvider_NoHeaders(t *testing.T) {
	t.Parallel()
	p := &CloudflareAccessProvider{TeamDomain: "myteam"}

	r := httptest.NewRequest("GET", "/", http.NoBody)

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id != nil {
		t.Errorf("expected nil identity, got %+v", id)
	}
}

func TestCloudflareAccessProvider_EmailWithoutJWT(t *testing.T) {
	t.Parallel()
	p := &CloudflareAccessProvider{TeamDomain: "myteam"}

	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set(CfAccessUserEmailHeader, "bob@example.com")

	id, err := p.Authenticate(r)
	if err == nil {
		t.Fatal("expected error when email present but JWT missing")
	}
	if id != nil {
		t.Errorf("expected nil identity, got %+v", id)
	}
}

func TestCloudflareAccessProvider_JWTOnlyNoEmailHeader(t *testing.T) {
	// JWT present without email header should still work — email comes from JWT.
	t.Parallel()
	key := testCFKey(t)
	kid := "test-key-2"
	teamDomain := "myteam"
	email := "carol@example.com"

	jwksData := testCFJWKS(t, kid, &key.PublicKey)
	mockClient := &mockCFHTTPClient{response: jwksData, statusCode: 200}

	p := &CloudflareAccessProvider{
		TeamDomain: teamDomain,
		HTTPClient: mockClient,
	}

	tokenStr := testCFToken(t, key, kid, email, teamDomain, []string{}, time.Now().Add(time.Hour))

	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set(CfAccessJWTHeader, tokenStr)

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id == nil {
		t.Fatal("expected identity")
	}
	if id.ExternalID != email {
		t.Errorf("ExternalID = %q, want %q", id.ExternalID, email)
	}
}

func TestCloudflareAccessProvider_ExpiredJWT(t *testing.T) {
	t.Parallel()
	key := testCFKey(t)
	kid := "test-key-3"
	teamDomain := "myteam"

	jwksData := testCFJWKS(t, kid, &key.PublicKey)
	mockClient := &mockCFHTTPClient{response: jwksData, statusCode: 200}

	p := &CloudflareAccessProvider{
		TeamDomain: teamDomain,
		HTTPClient: mockClient,
	}

	tokenStr := testCFToken(t, key, kid, "dave@example.com", teamDomain, []string{}, time.Now().Add(-time.Hour))

	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set(CfAccessUserEmailHeader, "dave@example.com")
	r.Header.Set(CfAccessJWTHeader, tokenStr)

	id, err := p.Authenticate(r)
	if err == nil {
		t.Fatal("expected error for expired JWT")
	}
	if id != nil {
		t.Errorf("expected nil identity, got %+v", id)
	}
}

func TestCloudflareAccessProvider_WrongIssuer(t *testing.T) {
	t.Parallel()
	key := testCFKey(t)
	kid := "test-key-4"

	jwksData := testCFJWKS(t, kid, &key.PublicKey)
	mockClient := &mockCFHTTPClient{response: jwksData, statusCode: 200}

	p := &CloudflareAccessProvider{
		TeamDomain: "myteam",
		HTTPClient: mockClient,
	}

	// Sign with a different team domain.
	tokenStr := testCFToken(t, key, kid, "eve@example.com", "wrongteam", []string{}, time.Now().Add(time.Hour))

	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set(CfAccessUserEmailHeader, "eve@example.com")
	r.Header.Set(CfAccessJWTHeader, tokenStr)

	id, err := p.Authenticate(r)
	if err == nil {
		t.Fatal("expected error for wrong issuer")
	}
	if id != nil {
		t.Errorf("expected nil identity, got %+v", id)
	}
}

func TestCloudflareAccessProvider_WrongAudience(t *testing.T) {
	t.Parallel()
	key := testCFKey(t)
	kid := "test-key-5"
	teamDomain := "myteam"

	jwksData := testCFJWKS(t, kid, &key.PublicKey)
	mockClient := &mockCFHTTPClient{response: jwksData, statusCode: 200}

	p := &CloudflareAccessProvider{
		TeamDomain: teamDomain,
		Audience:   "expected-aud",
		HTTPClient: mockClient,
	}

	tokenStr := testCFToken(t, key, kid, "frank@example.com", teamDomain, []string{"wrong-aud"}, time.Now().Add(time.Hour))

	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set(CfAccessUserEmailHeader, "frank@example.com")
	r.Header.Set(CfAccessJWTHeader, tokenStr)

	id, err := p.Authenticate(r)
	if err == nil {
		t.Fatal("expected error for wrong audience")
	}
	if id != nil {
		t.Errorf("expected nil identity, got %+v", id)
	}
}

func TestCloudflareAccessProvider_NoAudienceCheck(t *testing.T) {
	// When Audience is empty, the aud claim should not be validated.
	t.Parallel()
	key := testCFKey(t)
	kid := "test-key-6"
	teamDomain := "myteam"

	jwksData := testCFJWKS(t, kid, &key.PublicKey)
	mockClient := &mockCFHTTPClient{response: jwksData, statusCode: 200}

	p := &CloudflareAccessProvider{
		TeamDomain: teamDomain,
		Audience:   "", // No audience check.
		HTTPClient: mockClient,
	}

	tokenStr := testCFToken(t, key, kid, "grace@example.com", teamDomain, []string{"any-aud"}, time.Now().Add(time.Hour))

	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set(CfAccessUserEmailHeader, "grace@example.com")
	r.Header.Set(CfAccessJWTHeader, tokenStr)

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id == nil {
		t.Fatal("expected identity")
	}
	if id.Email != "grace@example.com" {
		t.Errorf("Email = %q, want grace@example.com", id.Email)
	}
}

func TestCloudflareAccessProvider_WrongSigningKey(t *testing.T) {
	t.Parallel()
	key := testCFKey(t)
	otherKey := testCFKey(t) // Different key.
	kid := "test-key-7"
	teamDomain := "myteam"

	// JWKS contains the OTHER key, not the one used to sign.
	jwksData := testCFJWKS(t, kid, &otherKey.PublicKey)
	mockClient := &mockCFHTTPClient{response: jwksData, statusCode: 200}

	p := &CloudflareAccessProvider{
		TeamDomain: teamDomain,
		HTTPClient: mockClient,
	}

	// Sign with the original key — should fail verification.
	tokenStr := testCFToken(t, key, kid, "heidi@example.com", teamDomain, []string{}, time.Now().Add(time.Hour))

	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set(CfAccessUserEmailHeader, "heidi@example.com")
	r.Header.Set(CfAccessJWTHeader, tokenStr)

	id, err := p.Authenticate(r)
	if err == nil {
		t.Fatal("expected error for wrong signing key")
	}
	if id != nil {
		t.Errorf("expected nil identity, got %+v", id)
	}
}

func TestCloudflareAccessProvider_UnknownKid(t *testing.T) {
	t.Parallel()
	key := testCFKey(t)
	teamDomain := "myteam"

	// JWKS has kid "known-key", token uses "unknown-key".
	jwksData := testCFJWKS(t, "known-key", &key.PublicKey)
	mockClient := &mockCFHTTPClient{response: jwksData, statusCode: 200}

	p := &CloudflareAccessProvider{
		TeamDomain: teamDomain,
		HTTPClient: mockClient,
	}

	tokenStr := testCFToken(t, key, "unknown-key", "ivan@example.com", teamDomain, []string{}, time.Now().Add(time.Hour))

	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set(CfAccessUserEmailHeader, "ivan@example.com")
	r.Header.Set(CfAccessJWTHeader, tokenStr)

	id, err := p.Authenticate(r)
	if err == nil {
		t.Fatal("expected error for unknown kid")
	}
	if id != nil {
		t.Errorf("expected nil identity, got %+v", id)
	}
}

func TestCloudflareAccessProvider_JWKSFetchError(t *testing.T) {
	t.Parallel()
	key := testCFKey(t)
	kid := "test-key-8"
	teamDomain := "myteam"

	mockClient := &mockCFHTTPClient{response: []byte("not found"), statusCode: 404}

	p := &CloudflareAccessProvider{
		TeamDomain: teamDomain,
		HTTPClient: mockClient,
	}

	tokenStr := testCFToken(t, key, kid, "judy@example.com", teamDomain, []string{}, time.Now().Add(time.Hour))

	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set(CfAccessUserEmailHeader, "judy@example.com")
	r.Header.Set(CfAccessJWTHeader, tokenStr)

	id, err := p.Authenticate(r)
	if err == nil {
		t.Fatal("expected error when JWKS fetch fails")
	}
	if id != nil {
		t.Errorf("expected nil identity, got %+v", id)
	}
}

func TestCloudflareAccessProvider_JWKSCaching(t *testing.T) {
	t.Parallel()
	key := testCFKey(t)
	kid := "test-key-9"
	teamDomain := "myteam"

	jwksData := testCFJWKS(t, kid, &key.PublicKey)
	mockClient := &mockCFHTTPClient{response: jwksData, statusCode: 200}

	p := &CloudflareAccessProvider{
		TeamDomain: teamDomain,
		HTTPClient: mockClient,
	}

	// First request — should fetch JWKS.
	token1 := testCFToken(t, key, kid, "alice@example.com", teamDomain, []string{}, time.Now().Add(time.Hour))
	r1 := httptest.NewRequest("GET", "/", http.NoBody)
	r1.Header.Set(CfAccessUserEmailHeader, "alice@example.com")
	r1.Header.Set(CfAccessJWTHeader, token1)

	id, err := p.Authenticate(r1)
	if err != nil {
		t.Fatal(err)
	}
	if id == nil {
		t.Fatal("expected identity")
	}
	if mockClient.requestsMade != 1 {
		t.Errorf("expected 1 JWKS fetch, got %d", mockClient.requestsMade)
	}

	// Second request — should use cached JWKS (no additional fetch).
	token2 := testCFToken(t, key, kid, "bob@example.com", teamDomain, []string{}, time.Now().Add(time.Hour))
	r2 := httptest.NewRequest("GET", "/", http.NoBody)
	r2.Header.Set(CfAccessUserEmailHeader, "bob@example.com")
	r2.Header.Set(CfAccessJWTHeader, token2)

	id, err = p.Authenticate(r2)
	if err != nil {
		t.Fatal(err)
	}
	if id == nil {
		t.Fatal("expected identity")
	}
	if mockClient.requestsMade != 1 {
		t.Errorf("expected 1 JWKS fetch (cached), got %d", mockClient.requestsMade)
	}
}

func TestCloudflareAccessProvider_JWTEmailPreferredOverHeader(t *testing.T) {
	// When JWT email differs from header email, JWT email wins.
	t.Parallel()
	key := testCFKey(t)
	kid := "test-key-10"
	teamDomain := "myteam"

	jwksData := testCFJWKS(t, kid, &key.PublicKey)
	mockClient := &mockCFHTTPClient{response: jwksData, statusCode: 200}

	p := &CloudflareAccessProvider{
		TeamDomain: teamDomain,
		HTTPClient: mockClient,
	}

	tokenStr := testCFToken(t, key, kid, "jwt-email@example.com", teamDomain, []string{}, time.Now().Add(time.Hour))

	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set(CfAccessUserEmailHeader, "header-email@example.com") // Different!
	r.Header.Set(CfAccessJWTHeader, tokenStr)

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id == nil {
		t.Fatal("expected identity")
	}
	if id.ExternalID != "jwt-email@example.com" {
		t.Errorf("ExternalID = %q, want jwt-email@example.com (JWT should win)", id.ExternalID)
	}
	if id.Email != "jwt-email@example.com" {
		t.Errorf("Email = %q, want jwt-email@example.com (JWT should win)", id.Email)
	}
}

func TestCloudflareAccessProvider_InvalidJWTFormat(t *testing.T) {
	t.Parallel()
	p := &CloudflareAccessProvider{TeamDomain: "myteam"}

	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.Header.Set(CfAccessUserEmailHeader, "kate@example.com")
	r.Header.Set(CfAccessJWTHeader, "not.a.valid.jwt")

	id, err := p.Authenticate(r)
	if err == nil {
		t.Fatal("expected error for invalid JWT")
	}
	if id != nil {
		t.Errorf("expected nil identity, got %+v", id)
	}
}

func TestCloudflareAccessProvider_AuthMiddlewareIntegration(t *testing.T) {
	t.Parallel()
	key := testCFKey(t)
	kid := "test-key-11"
	teamDomain := "myteam"
	email := "middleware-user@example.com"

	jwksData := testCFJWKS(t, kid, &key.PublicKey)
	mockClient := &mockCFHTTPClient{response: jwksData, statusCode: 200}

	s := testServer(t)
	s.AuthProvider = &CloudflareAccessProvider{
		TeamDomain: teamDomain,
		HTTPClient: mockClient,
	}

	var gotUser *User
	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = GetUser(r.Context())
		w.WriteHeader(200)
	}))

	tokenStr := testCFToken(t, key, kid, email, teamDomain, []string{}, time.Now().Add(time.Hour))

	r := httptest.NewRequest("GET", "/api/feeds", http.NoBody)
	r.Header.Set(CfAccessUserEmailHeader, email)
	r.Header.Set(CfAccessJWTHeader, tokenStr)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	if gotUser == nil {
		t.Fatal("user not set in context")
	}
	if gotUser.ExternalID != email {
		t.Errorf("external_id = %q, want %q", gotUser.ExternalID, email)
	}
	if gotUser.Email != email {
		t.Errorf("email = %q, want %q", gotUser.Email, email)
	}
}

func TestParseECPublicKey_UnsupportedKeyType(t *testing.T) {
	t.Parallel()
	_, err := parseECPublicKey(&cfJWK{Kty: "RSA", Crv: "P-256", X: "AA", Y: "AA"})
	if err == nil {
		t.Fatal("expected error for unsupported key type")
	}
}

func TestParseECPublicKey_UnsupportedCurve(t *testing.T) {
	t.Parallel()
	_, err := parseECPublicKey(&cfJWK{Kty: "EC", Crv: "P-521", X: "AA", Y: "AA"})
	if err == nil {
		t.Fatal("expected error for unsupported curve")
	}
}

func TestParseECPublicKey_InvalidBase64(t *testing.T) {
	t.Parallel()
	_, err := parseECPublicKey(&cfJWK{Kty: "EC", Crv: "P-256", X: "!!invalid!!", Y: "AA"})
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}
