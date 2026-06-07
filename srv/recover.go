package srv

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// recoverMiddleware is the outermost middleware. It recovers from panics in
// any downstream handler so a single bad request can't tear down the request
// without a clean response. net/http recovers panics at the per-connection
// level, but it abruptly closes the connection and logs a raw stack trace;
// this middleware instead logs structured detail and returns a proper 500.
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			// http.ErrAbortHandler is the documented way for a handler to
			// abort silently; re-panic so net/http handles it as intended.
			if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(rec)
			}

			attrs := []slog.Attr{
				slog.Any("panic", rec),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("stack", string(debug.Stack())),
			}
			if user := GetUser(r.Context()); user != nil {
				attrs = append(attrs, slog.Int64("user_id", user.ID))
			}
			slog.LogAttrs(r.Context(), slog.LevelError, "panic recovered in handler", attrs...)

			// Best-effort 500. If the handler already wrote a response,
			// net/http logs a benign superfluous-WriteHeader warning.
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}()

		next.ServeHTTP(w, r)
	})
}
