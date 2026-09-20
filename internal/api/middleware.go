package api

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
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
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Max-Age", "86400")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// logRequests records one structured line per request.
func logRequests(log *slog.Logger) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(recorder, r)

			log.InfoContext(r.Context(), "request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", recorder.status),
				slog.Duration("duration", time.Since(started)),
				slog.String("ip", clientIP(r)),
			)
		})
	}
}

// recoverPanics keeps one bad request from taking the process down, and hides
// the stack from the caller.
func recoverPanics(log *slog.Logger) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					log.ErrorContext(r.Context(), "panic serving request",
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
						slog.Any("panic", recovered),
					)
					writeError(w, http.StatusInternalServerError, "Internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// statusRecorder remembers the status code so the log can report it. Unwrap
// hands http.ResponseController the real writer, which keeps Flush and the
// per-request deadlines working through the wrapper — streaming depends on it.
type statusRecorder struct {
	http.ResponseWriter
	status  int
	written bool
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func (r *statusRecorder) WriteHeader(status int) {
	if !r.written {
		r.status, r.written = status, true
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.written = true
	return r.ResponseWriter.Write(b)
}

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
