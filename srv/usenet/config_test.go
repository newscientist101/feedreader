package usenet_test

import (
	"encoding/base64"
	"testing"

	"github.com/newscientist101/feedreader/srv/usenet"
)

// testKeyB64 is a base64-encoded 32-byte key for testing.
var testKeyB64 = base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))

func TestLoadConfig_DisabledByDefault(t *testing.T) {
	t.Setenv("USENET_ENABLED", "")
	t.Setenv("USENET_CREDENTIAL_KEY", "")

	cfg, err := usenet.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: unexpected error: %v", err)
	}
	if cfg.Enabled {
		t.Error("expected Enabled=false when USENET_ENABLED is not set")
	}
	if cfg.Crypto != nil {
		t.Error("expected Crypto=nil when disabled")
	}
}

func TestLoadConfig_EnabledWithValidKey(t *testing.T) {
	t.Setenv("USENET_ENABLED", "true")
	t.Setenv("USENET_CREDENTIAL_KEY", testKeyB64)

	cfg, err := usenet.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: unexpected error: %v", err)
	}
	if !cfg.Enabled {
		t.Error("expected Enabled=true")
	}
	if cfg.Crypto == nil {
		t.Error("expected Crypto to be non-nil")
	}
}

func TestLoadConfig_EnabledMissingKey(t *testing.T) {
	t.Setenv("USENET_ENABLED", "true")
	t.Setenv("USENET_CREDENTIAL_KEY", "")

	_, err := usenet.LoadConfig()
	if err == nil {
		t.Error("expected error when USENET_ENABLED=true but key is missing")
	}
}

func TestLoadConfig_EnabledInvalidBase64(t *testing.T) {
	t.Setenv("USENET_ENABLED", "true")
	t.Setenv("USENET_CREDENTIAL_KEY", "not-valid-base64!!!")

	_, err := usenet.LoadConfig()
	if err == nil {
		t.Error("expected error for invalid base64 key")
	}
}

func TestLoadConfig_EnabledKeyWrongLength(t *testing.T) {
	// base64 of 16 bytes (too short)
	shortKey := base64.StdEncoding.EncodeToString([]byte("tooshort12345678"))
	t.Setenv("USENET_ENABLED", "true")
	t.Setenv("USENET_CREDENTIAL_KEY", shortKey)

	_, err := usenet.LoadConfig()
	if err == nil {
		t.Error("expected error for key that decodes to wrong length")
	}
}

func TestLoadConfig_DisabledKeyNotRequired(t *testing.T) {
	// Even if an invalid key is set, disabled mode should not validate it.
	t.Setenv("USENET_ENABLED", "false")
	t.Setenv("USENET_CREDENTIAL_KEY", "garbage")

	cfg, err := usenet.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: unexpected error: %v", err)
	}
	if cfg.Enabled {
		t.Error("expected Enabled=false")
	}
}
