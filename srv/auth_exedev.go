package srv

import (
	"net/http"
)

// ExeDevProvider reads identity from exe.dev proxy headers.
// This preserves backward compatibility with the exe.dev hosting platform.
type ExeDevProvider struct{}

// exe.dev-specific header names.
const (
	exedevUserIDHeader = "X-Exedev-Userid"
	exedevEmailHeader  = "X-Exedev-Email"
)

// Authenticate extracts identity from X-Exedev-Userid and X-Exedev-Email headers.
func (ExeDevProvider) Authenticate(r *http.Request) (*Identity, error) {
	externalID := r.Header.Get(exedevUserIDHeader)
	if externalID == "" {
		return nil, nil
	}
	email := r.Header.Get(exedevEmailHeader)
	return &Identity{ExternalID: externalID, Email: email}, nil
}
