package srv

import (
	"net/http"
)

// Default header names for common reverse proxy auth setups.
const (
	DefaultUserIDHeader = "Remote-User"
	DefaultEmailHeader  = "Remote-Email"
)

// ProxyHeaderProvider reads user identity from configurable HTTP headers
// set by a reverse proxy (Caddy + Authelia, nginx + oauth2-proxy, etc.).
//
// This is the primary portable auth provider. The reverse proxy owns
// the login flow — the app just reads the resulting identity headers.
type ProxyHeaderProvider struct {
	// UserIDHeader is the header containing the unique user identifier.
	// Defaults to "Remote-User" if empty.
	UserIDHeader string

	// EmailHeader is the header containing the user's email address.
	// Defaults to "Remote-Email" if empty.
	EmailHeader string
}

// Authenticate extracts identity from proxy-injected headers.
// Returns nil identity (no error) when the headers are absent.
func (p *ProxyHeaderProvider) Authenticate(r *http.Request) (*Identity, error) {
	userHeader := p.UserIDHeader
	if userHeader == "" {
		userHeader = DefaultUserIDHeader
	}
	emailHeader := p.EmailHeader
	if emailHeader == "" {
		emailHeader = DefaultEmailHeader
	}

	externalID := r.Header.Get(userHeader)
	if externalID == "" {
		return nil, nil
	}

	email := r.Header.Get(emailHeader)
	return &Identity{ExternalID: externalID, Email: email}, nil
}
