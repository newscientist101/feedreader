package srv

import (
	"net/http"
)

// Tailscale header names injected by Tailscale Serve/Funnel.
const (
	TailscaleUserLoginHeader      = "Tailscale-User-Login"
	TailscaleUserNameHeader       = "Tailscale-User-Name"
	TailscaleUserProfilePicHeader = "Tailscale-User-Profile-Pic"
)

// TailscaleProvider reads identity from headers injected by the
// Tailscale daemon when using Tailscale Serve or Funnel.
//
// Tailscale-User-Login is the user's login name (typically an email
// like user@example.com). It's used as both the external ID and the
// email address since Tailscale doesn't provide a separate stable ID.
//
// These headers are trusted — they're injected by the local Tailscale
// daemon and cannot be forged by the client.
type TailscaleProvider struct{}

// Authenticate extracts identity from Tailscale-User-Login.
// Returns nil identity (no error) when the header is absent.
func (TailscaleProvider) Authenticate(r *http.Request) (*Identity, error) {
	login := r.Header.Get(TailscaleUserLoginHeader)
	if login == "" {
		return nil, nil
	}

	return &Identity{ExternalID: login, Email: login}, nil
}
