package srv

import (
	"net/http"
)

// Authelia header names injected by the Authelia authentication server
// via nginx, Caddy, or Traefik.
const (
	AutheliaRemoteUserHeader   = "Remote-User"
	AutheliaRemoteNameHeader   = "Remote-Name"
	AutheliaRemoteEmailHeader  = "Remote-Email"
	AutheliaRemoteGroupsHeader = "Remote-Groups"
)

// AutheliaProvider reads identity from headers injected by Authelia.
//
// Authelia sits behind a reverse proxy (nginx/Caddy/Traefik) and injects
// Remote-User, Remote-Name, Remote-Email, and Remote-Groups headers after
// successful authentication. These headers are trusted — the reverse proxy
// strips any client-supplied values before forwarding to Authelia.
//
// Remote-User is the stable identifier (Authelia username). Remote-Email
// provides the email address.
type AutheliaProvider struct{}

// Authenticate extracts identity from Authelia-injected headers.
// Returns nil identity (no error) when Remote-User is absent.
func (AutheliaProvider) Authenticate(r *http.Request) (*Identity, error) {
	user := r.Header.Get(AutheliaRemoteUserHeader)
	if user == "" {
		return nil, nil
	}

	email := r.Header.Get(AutheliaRemoteEmailHeader)
	return &Identity{ExternalID: user, Email: email}, nil
}
