package usenet_test

import (
	"errors"
	"testing"

	"github.com/newscientist101/feedreader/srv/usenet"
)

func TestValidateCredentials_Valid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		user     string
		pass     string
		wantUser string
	}{
		{"simple", "alice", "hunter2", "alice"},
		{"trims username whitespace", "  alice  ", "pass", "alice"},
		{"password with spaces", "bob", "my secret pass", "bob"},
		{"unicode username", "ülrich", "pass", "ülrich"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := usenet.ValidateCredentials(tc.user, tc.pass)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantUser {
				t.Errorf("username: got %q, want %q", got, tc.wantUser)
			}
		})
	}
}

func TestValidateCredentials_EmptyUsername(t *testing.T) {
	t.Parallel()
	_, err := usenet.ValidateCredentials("", "pass")
	if !errors.Is(err, usenet.ErrInvalidCredential) {
		t.Fatalf("expected ErrInvalidCredential, got %v", err)
	}
}

func TestValidateCredentials_WhitespaceOnlyUsername(t *testing.T) {
	t.Parallel()
	_, err := usenet.ValidateCredentials("   ", "pass")
	if !errors.Is(err, usenet.ErrInvalidCredential) {
		t.Fatalf("expected ErrInvalidCredential, got %v", err)
	}
}

func TestValidateCredentials_EmptyPassword(t *testing.T) {
	t.Parallel()
	_, err := usenet.ValidateCredentials("alice", "")
	if !errors.Is(err, usenet.ErrInvalidCredential) {
		t.Fatalf("expected ErrInvalidCredential, got %v", err)
	}
}

func TestValidateCredentials_ControlCharsInUsername(t *testing.T) {
	t.Parallel()
	controls := []string{
		"alice\reve",
		"alice\neve",
		"alice\x00eve",
		"alice\x01eve",
		"alice\x7feve",
	}
	for _, u := range controls {
		_, err := usenet.ValidateCredentials(u, "pass")
		if !errors.Is(err, usenet.ErrInvalidCredential) {
			t.Errorf("username %q: expected ErrInvalidCredential, got %v", u, err)
		}
	}
}

func TestValidateCredentials_ControlCharsInPassword(t *testing.T) {
	t.Parallel()
	controls := []string{
		"pass\rword",
		"pass\nword",
		"pass\x00word",
		"pass\x1fword",
		"pass\x7fword",
	}
	for _, p := range controls {
		_, err := usenet.ValidateCredentials("alice", p)
		if !errors.Is(err, usenet.ErrInvalidCredential) {
			t.Errorf("password %q: expected ErrInvalidCredential, got %v", p, err)
		}
	}
}
