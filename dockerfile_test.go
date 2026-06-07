package feedreader_test

import (
	"os"
	"strings"
	"testing"
)

// TestDockerfile validates basic Dockerfile structure.
func TestDockerfile(t *testing.T) {
	data, err := os.ReadFile("Dockerfile")
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	content := string(data)

	// Must be a multi-stage build.
	fromCount := strings.Count(content, "\nFROM ")
	if content[:5] == "FROM " {
		fromCount++ // first line
	}
	if fromCount < 2 {
		t.Errorf("expected multi-stage build (>= 2 FROM), got %d", fromCount)
	}

	// Must use a builder stage.
	if !strings.Contains(content, "AS builder") {
		t.Error("missing builder stage (AS builder)")
	}

	// Must copy the binary from builder.
	if !strings.Contains(content, "COPY --from=builder") {
		t.Error("missing COPY --from=builder")
	}

	// Must expose port 8000.
	if !strings.Contains(content, "EXPOSE 8000") {
		t.Error("missing EXPOSE 8000")
	}

	// Must run as non-root user.
	if !strings.Contains(content, "USER ") {
		t.Error("missing USER directive (should run as non-root)")
	}

	// Must have a VOLUME for /data.
	if !strings.Contains(content, "VOLUME /data") {
		t.Error("missing VOLUME /data")
	}

	// Must set CONFIG_FILE env var.
	if !strings.Contains(content, "CONFIG_FILE") {
		t.Error("missing CONFIG_FILE env var")
	}

	// Must copy templates and static assets.
	if !strings.Contains(content, "srv/templates/") {
		t.Error("missing copy of srv/templates/")
	}
	if !strings.Contains(content, "srv/static/") {
		t.Error("missing copy of srv/static/")
	}
}

// TestDockerignore validates that .dockerignore exists and excludes key files.
func TestDockerignore(t *testing.T) {
	data, err := os.ReadFile(".dockerignore")
	if err != nil {
		t.Fatalf("read .dockerignore: %v", err)
	}
	content := string(data)

	for _, pattern := range []string{".git", "node_modules", "*.sqlite3"} {
		if !strings.Contains(content, pattern) {
			t.Errorf(".dockerignore should exclude %q", pattern)
		}
	}
}

// TestDockerCompose validates the docker-compose.yml exists and has expected content.
func TestDockerCompose(t *testing.T) {
	data, err := os.ReadFile("docker-compose.yml")
	if err != nil {
		t.Fatalf("read docker-compose.yml: %v", err)
	}
	content := string(data)

	// Must define the feedreader service.
	if !strings.Contains(content, "feedreader:") {
		t.Error("missing feedreader service")
	}

	// Must have a volume for persistence.
	if !strings.Contains(content, "feedreader-data") {
		t.Error("missing feedreader-data volume")
	}

	// Must include an auth proxy.
	if !strings.Contains(content, "oauth2-proxy") {
		t.Error("missing oauth2-proxy service")
	}
}
