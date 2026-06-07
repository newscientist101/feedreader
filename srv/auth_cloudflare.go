package srv

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

// Cloudflare Access header names.
const (
	CfAccessUserEmailHeader = "Cf-Access-Authenticated-User-Email"
	CfAccessJWTHeader       = "Cf-Access-Jwt-Assertion"
)

// cfJWKSResponse is the JSON structure returned by the Cloudflare Access
// JWKS (JSON Web Key Set) endpoint.
type cfJWKSResponse struct {
	Keys []cfJWK `json:"keys"`
}

// cfJWK represents a single JWK entry from the Cloudflare JWKS response.
// Cloudflare Access uses ES256 keys (P-256 curve).
type cfJWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

// CloudflareAccessProvider reads identity from headers set by Cloudflare
// Access. It validates the JWT in Cf-Access-Jwt-Assertion against
// Cloudflare's JWKS endpoint to prevent header spoofing.
//
// Cloudflare Access doesn't provide a stable user ID separate from email,
// so email is used as the external identifier.
type CloudflareAccessProvider struct {
	// TeamDomain is the Cloudflare Access team domain (e.g., "myteam").
	// Used to construct the JWKS URL: https://<TeamDomain>.cloudflareaccess.com/cdn-cgi/access/certs
	TeamDomain string

	// Audience is the Application Audience (AUD) tag from Cloudflare Access
	// settings. If non-empty, the JWT's aud claim must contain this value.
	Audience string

	// HTTPClient is used to fetch the JWKS. If nil, http.DefaultClient is used.
	HTTPClient HTTPClient

	// keyCache stores parsed public keys from the JWKS endpoint.
	keyMu       sync.RWMutex
	keyCache    map[string]crypto.PublicKey
	keyCachedAt time.Time
}

// HTTPClient is an interface for making HTTP requests, enabling test injection.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

const (
	// cfJWKSCacheDuration is how long cached JWKS keys are considered valid.
	cfJWKSCacheDuration = 10 * time.Minute

	// cfJWKSFetchTimeout is the timeout for fetching the JWKS endpoint.
	cfJWKSFetchTimeout = 10 * time.Second
)

// Authenticate extracts identity from Cloudflare Access headers and
// validates the JWT. Returns nil identity (no error) when headers are absent.
func (p *CloudflareAccessProvider) Authenticate(r *http.Request) (*Identity, error) {
	email := r.Header.Get(CfAccessUserEmailHeader)
	tokenStr := r.Header.Get(CfAccessJWTHeader)

	// No Cloudflare Access headers present — not authenticated via this provider.
	if email == "" && tokenStr == "" {
		return nil, nil
	}

	// JWT is required when the email header is present.
	if tokenStr == "" {
		return nil, fmt.Errorf("cloudflare access: email header present but JWT missing")
	}

	token, err := p.validateJWT(r.Context(), tokenStr)
	if err != nil {
		return nil, fmt.Errorf("cloudflare access: JWT validation failed: %w", err)
	}

	// Extract email from validated JWT claims.
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("cloudflare access: unexpected claims type")
	}

	// Prefer the email from the validated JWT over the header value.
	jwtEmail, _ := claims["email"].(string)
	if jwtEmail == "" {
		return nil, fmt.Errorf("cloudflare access: JWT missing email claim")
	}

	return &Identity{ExternalID: jwtEmail, Email: jwtEmail}, nil
}

// certsURL returns the JWKS endpoint URL for the configured team domain.
func (p *CloudflareAccessProvider) certsURL() string {
	return fmt.Sprintf("https://%s.cloudflareaccess.com/cdn-cgi/access/certs", p.TeamDomain)
}

// httpClient returns the configured HTTP client or the default.
func (p *CloudflareAccessProvider) httpClient() HTTPClient {
	if p.HTTPClient != nil {
		return p.HTTPClient
	}
	return http.DefaultClient
}

// validateJWT parses and validates the Cloudflare Access JWT.
func (p *CloudflareAccessProvider) validateJWT(ctx context.Context, tokenStr string) (*jwt.Token, error) {
	// Parse the token with key lookup.
	parserOpts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"ES256"}),
		jwt.WithIssuer(fmt.Sprintf("https://%s.cloudflareaccess.com", p.TeamDomain)),
	}
	if p.Audience != "" {
		parserOpts = append(parserOpts, jwt.WithAudience(p.Audience))
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		kid, ok := token.Header["kid"].(string)
		if !ok || kid == "" {
			return nil, fmt.Errorf("token has no kid header")
		}
		return p.getKey(ctx, kid)
	}, parserOpts...)
	if err != nil {
		return nil, err
	}

	return token, nil
}

// getKey returns the public key for the given kid, fetching from the
// JWKS endpoint if the cache is empty or expired.
func (p *CloudflareAccessProvider) getKey(ctx context.Context, kid string) (crypto.PublicKey, error) {
	// Try cache first.
	p.keyMu.RLock()
	if p.keyCache != nil && time.Since(p.keyCachedAt) < cfJWKSCacheDuration {
		if key, ok := p.keyCache[kid]; ok {
			p.keyMu.RUnlock()
			return key, nil
		}
	}
	p.keyMu.RUnlock()

	// Fetch and cache.
	keys, err := p.fetchJWKS(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching JWKS: %w", err)
	}

	p.keyMu.Lock()
	p.keyCache = keys
	p.keyCachedAt = time.Now()
	p.keyMu.Unlock()

	key, ok := keys[kid]
	if !ok {
		return nil, fmt.Errorf("key %q not found in JWKS", kid)
	}
	return key, nil
}

// fetchJWKS retrieves the JSON Web Key Set from the Cloudflare Access endpoint.
func (p *CloudflareAccessProvider) fetchJWKS(ctx context.Context) (map[string]crypto.PublicKey, error) {
	ctx, cancel := context.WithTimeout(ctx, cfJWKSFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.certsURL(), http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", p.certsURL(), err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", p.certsURL(), resp.StatusCode)
	}

	var jwks cfJWKSResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("decoding JWKS: %w", err)
	}

	keys := make(map[string]crypto.PublicKey, len(jwks.Keys))
	for i := range jwks.Keys {
		pub, err := parseECPublicKey(&jwks.Keys[i])
		if err != nil {
			continue // Skip keys we can't parse.
		}
		keys[jwks.Keys[i].Kid] = pub
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no usable keys in JWKS response")
	}

	return keys, nil
}

// parseECPublicKey parses an EC public key from a JWK.
func parseECPublicKey(k *cfJWK) (*ecdsa.PublicKey, error) {
	if !strings.EqualFold(k.Kty, "EC") {
		return nil, fmt.Errorf("unsupported key type %q", k.Kty)
	}

	var curve elliptic.Curve
	switch k.Crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	default:
		return nil, fmt.Errorf("unsupported curve %q", k.Crv)
	}

	xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, fmt.Errorf("decoding x: %w", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, fmt.Errorf("decoding y: %w", err)
	}

	// Build the uncompressed point (0x04 || X || Y) and parse via the
	// non-deprecated API instead of setting the big.Int fields directly.
	uncompressed := make([]byte, 1+len(xBytes)+len(yBytes))
	uncompressed[0] = 0x04
	copy(uncompressed[1:], xBytes)
	copy(uncompressed[1+len(xBytes):], yBytes)
	return ecdsa.ParseUncompressedPublicKey(curve, uncompressed)
}
