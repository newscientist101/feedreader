package srv

import (
	"context"
	"os"
	"testing"
)

func TestGetUser_Present(t *testing.T) {
	user := &User{ID: 42, ExternalID: "ext-42", Email: "test@example.com"}
	ctx := context.WithValue(context.Background(), userContextKey, user)

	got := GetUser(ctx)
	if got == nil {
		t.Fatal("expected non-nil user")
	}
	if got.ID != 42 {
		t.Errorf("ID = %d, want 42", got.ID)
	}
	if got.ExternalID != "ext-42" {
		t.Errorf("ExternalID = %q", got.ExternalID)
	}
	if got.Email != "test@example.com" {
		t.Errorf("Email = %q", got.Email)
	}
}

func TestGetUser_Missing(t *testing.T) {
	got := GetUser(context.Background())
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestGetUser_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), userContextKey, "not a user")
	got := GetUser(ctx)
	if got != nil {
		t.Errorf("expected nil for wrong type, got %+v", got)
	}
}

func TestIsDevelopment(t *testing.T) {
	// Save and restore
	orig := os.Getenv("DEV")
	defer os.Setenv("DEV", orig)

	os.Setenv("DEV", "")
	if isDevelopment() {
		t.Error("expected false when DEV is empty")
	}

	os.Setenv("DEV", "1")
	if !isDevelopment() {
		t.Error("expected true when DEV=1")
	}

	os.Setenv("DEV", "anything")
	if !isDevelopment() {
		t.Error("expected true when DEV=anything")
	}
}
