package api

import (
	"net/http"
)

// cors admits the one browser origin the UI is served from. A blank or "*"
// origin allows any caller, which is what local development and the tests use.
// The API authenticates with a bearer header, not cookies, so no credentials
// mode is needed.
func cors(allowedOrigin string) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			// Responses vary by Origin even when the header is absent, so
			// caches must not reuse one origin's answer for another.
			w.Header().Add("Vary", "Origin")

			if origin != "" && (allowedOrigin == "" || allowedOrigin == "*" || allowedOrigin == origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Max-Age", "86400")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
