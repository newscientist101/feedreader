package srv

import "net/http"

// securityHeaders wraps an http.Handler and sets security-related response
// headers on every response.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()

		// Prevent MIME-type sniffing.
		h.Set("X-Content-Type-Options", "nosniff")

		// Disallow framing by other origins.
		h.Set("X-Frame-Options", "DENY")

		// Control how much referrer information is sent.
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// CSP: allow same-origin scripts/styles plus inline (needed for the
		// manifest generator, settings injection, and inline style attrs).
		// Images/media/frames are permitted from any HTTPS source since
		// feed content regularly embeds external resources.
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' https: data: blob:; "+
				"media-src 'self' https:; "+
				"frame-src https:; "+
				"connect-src 'self'; "+
				"font-src 'self'; "+
				"manifest-src 'self' blob:; "+
				"worker-src 'self'; "+
				"object-src 'none'; "+
				"base-uri 'self'; "+
				"form-action 'self'")

		next.ServeHTTP(w, r)
	})
}
