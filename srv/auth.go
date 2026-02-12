package srv

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"srv.exe.dev/db/dbgen"
)

// Context key for user
type contextKey string

const userContextKey contextKey = "user"

// User represents an authenticated user
type User struct {
	ID         int64
	ExternalID string
	Email      string
}

// GetUser extracts the user from context
func GetUser(ctx context.Context) *User {
	if user, ok := ctx.Value(userContextKey).(*User); ok {
		return user
	}
	return nil
}

// isDevelopment checks if running in development mode (no exe.dev proxy)
func isDevelopment() bool {
	// If DEV environment variable is set, use dev mode
	if os.Getenv("DEV") != "" {
		return true
	}
	return false
}

// AuthMiddleware extracts user from exe.dev headers and ensures authentication
func (s *Server) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get exe.dev auth headers
		externalID := r.Header.Get("X-Exedev-Userid")
		email := r.Header.Get("X-Exedev-Email")

		// Allow static files without auth
		if len(r.URL.Path) >= 7 && r.URL.Path[:7] == "/static" {
			next.ServeHTTP(w, r)
			return
		}

		// Development mode - create a default user
		if externalID == "" && isDevelopment() {
			externalID = "dev-user"
			email = "dev@localhost"
			slog.Debug("using development user")
		}

		// If no auth headers, redirect to login
		if externalID == "" {
			// Check if this is an API request
			if len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			// Redirect to exe.dev login
			redirectURL := "/__exe.dev/login?redirect=" + r.URL.Path
			http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
			return
		}

		// Get or create user
		ctx := r.Context()
		q := dbgen.New(s.DB)

		dbUser, err := q.GetOrCreateUser(ctx, dbgen.GetOrCreateUserParams{
			ExternalID: externalID,
			Email:      email,
		})
		if err != nil {
			http.Error(w, "Failed to authenticate user", http.StatusInternalServerError)
			return
		}

		user := &User{
			ID:         dbUser.ID,
			ExternalID: dbUser.ExternalID,
			Email:      dbUser.Email,
		}

		// Add user to context
		ctx = context.WithValue(ctx, userContextKey, user)
		if r.Method == "POST" {
			slog.Info("POST request", "path", r.URL.Path, "remote", r.RemoteAddr, "request_id", r.Header.Get("X-Request-Id"), "content_type", r.Header.Get("Content-Type"))
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
