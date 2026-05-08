package usenet

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// ErrInvalidCredential is returned by ValidateCredentials when a username
// or password fails validation.
var ErrInvalidCredential = errors.New("invalid credential")

// ValidateCredentials validates a username/password pair before encryption
// and storage.
//
// Rules:
//   - Username is trimmed of surrounding whitespace; the trimmed value must be
//     non-empty.
//   - Password is not trimmed.
//   - Neither value may contain ASCII control characters (including CR, LF, NUL,
//     or any other character with code point < 0x20 or equal to 0x7F). This
//     prevents NNTP AUTHINFO command injection.
//
// On success it returns the trimmed username. On failure it returns an
// ErrInvalidCredential-wrapping error.
func ValidateCredentials(username, password string) (string, error) {
	trimmed := strings.TrimSpace(username)
	if trimmed == "" {
		return "", fmt.Errorf("%w: username must not be empty", ErrInvalidCredential)
	}
	if containsControlChar(trimmed) {
		return "", fmt.Errorf("%w: username contains invalid control characters", ErrInvalidCredential)
	}
	if password == "" {
		return "", fmt.Errorf("%w: password must not be empty", ErrInvalidCredential)
	}
	if containsControlChar(password) {
		return "", fmt.Errorf("%w: password contains invalid control characters", ErrInvalidCredential)
	}
	return trimmed, nil
}

// containsControlChar reports whether s contains any ASCII control character
// (code points 0x00–0x1F or 0x7F).
func containsControlChar(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
