package api

import (
	"context"
	"net"
	"net/http"
	"strings"
)

type ctxKey struct{}

// requireAuth admits requests carrying a valid session token whose email is
// still on the allow-list, and hands the email to the handler. Checking the
// list on every request is what makes removing an address take effect at once.
func (s *server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			s.fail(w, r, errUnauthorized)
			return
		}
		email, err := s.signer.Verify(token)
		if err != nil {
			s.fail(w, r, errUnauthorized)
			return
		}
		if _, allowed := s.allowed[email]; !allowed {
			s.fail(w, r, errForbidden)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, email)))
	})
}

// owner is the email requireAuth stored on the request.
func owner(r *http.Request) string {
	email, _ := r.Context().Value(ctxKey{}).(string)
	return email
}

// userKey buckets a request by the signed-in account.
func userKey(r *http.Request) string { return owner(r) }

// clientIP identifies the caller for rate limiting. The API only ever runs
// behind Caddy, which replaces any client-supplied X-Forwarded-For with the
// address it actually accepted the connection from.
func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		first, _, _ := strings.Cut(forwarded, ",")
		if first = strings.TrimSpace(first); first != "" {
			return first
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
