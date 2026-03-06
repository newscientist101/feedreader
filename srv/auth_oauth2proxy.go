package srv

import (
	"net/http"
)

// OAuth2 Proxy header names.
const (
	OAuth2ProxyUserHeader              = "X-Forwarded-User"
	OAuth2ProxyEmailHeader             = "X-Forwarded-Email"
	OAuth2ProxyPreferredUsernameHeader = "X-Forwarded-Preferred-Username"
	OAuth2ProxyGroupsHeader            = "X-Forwarded-Groups"
)

// OAuth2ProxyProvider reads identity from headers injected by
// oauth2-proxy (https://oauth2-proxy.github.io/oauth2-proxy/).
//
// OAuth2 Proxy sits in front of the application and handles the OAuth2
// login flow. After authentication it injects X-Forwarded-User,
// X-Forwarded-Email, and optionally X-Forwarded-Preferred-Username and
// X-Forwarded-Groups headers.
//
// X-Forwarded-Email is the primary identifier. If absent,
// X-Forwarded-User is used as fallback.
type OAuth2ProxyProvider struct{}

// Authenticate extracts identity from OAuth2 Proxy headers.
// Returns nil identity (no error) when neither email nor user header
// is present.
func (OAuth2ProxyProvider) Authenticate(r *http.Request) (*Identity, error) {
	email := r.Header.Get(OAuth2ProxyEmailHeader)
	user := r.Header.Get(OAuth2ProxyUserHeader)

	// Determine the external ID: prefer email, fall back to user.
	externalID := email
	if externalID == "" {
		externalID = user
	}

	if externalID == "" {
		return nil, nil
	}

	return &Identity{ExternalID: externalID, Email: email}, nil
}
