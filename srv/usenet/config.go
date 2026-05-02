package usenet

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
)

// Config holds resolved Usenet feature configuration.
// Crypto is non-nil only when USENET_ENABLED=true and a valid
// USENET_CREDENTIAL_KEY is present.
type Config struct {
	// Enabled reports whether the Usenet feature is active.
	Enabled bool
	// Crypto is the credential encryption helper. Nil when not enabled.
	Crypto *Crypto
}

// ErrUsenetDisabled is returned by helpers that require Usenet to be enabled.
var ErrUsenetDisabled = errors.New("usenet: feature not enabled (USENET_ENABLED not set)")

// ErrUsenetNotConfigured is returned when Usenet is enabled but the
// credential encryption key is missing or invalid.
var ErrUsenetNotConfigured = errors.New("usenet: USENET_CREDENTIAL_KEY is not configured")

// LoadConfig reads USENET_ENABLED and USENET_CREDENTIAL_KEY from the
// environment and returns a validated Config.
//
// Rules:
//   - If USENET_ENABLED is not "true", Enabled is false and Crypto is nil.
//     The key need not be set in this case and no error is returned.
//   - If USENET_ENABLED=true, USENET_CREDENTIAL_KEY must be a base64-encoded
//     string that decodes to exactly 32 bytes. Any other value is an error and
//     the caller (server startup) should abort.
func LoadConfig() (*Config, error) {
	if os.Getenv("USENET_ENABLED") != "true" {
		return &Config{Enabled: false}, nil
	}

	keyB64 := os.Getenv("USENET_CREDENTIAL_KEY")
	if keyB64 == "" {
		return nil, fmt.Errorf("usenet: USENET_ENABLED=true but USENET_CREDENTIAL_KEY is not set")
	}

	rawKey, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, fmt.Errorf("usenet: USENET_CREDENTIAL_KEY is not valid base64: %w", err)
	}
	if len(rawKey) != 32 {
		return nil, fmt.Errorf("usenet: USENET_CREDENTIAL_KEY must decode to exactly 32 bytes, got %d", len(rawKey))
	}

	crypto, err := NewCrypto(rawKey)
	if err != nil {
		return nil, fmt.Errorf("usenet: init crypto: %w", err)
	}

	return &Config{Enabled: true, Crypto: crypto}, nil
}
