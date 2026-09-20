package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"

	"github.com/dragunovartem99/cookd-api/internal/auth"
)

const maxCredentialBytes = 8 << 10

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// signIn trades a Google ID token for one of our own session tokens.
func (s *server) signIn(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Credential string `json:"credential"`
	}
	if err := decodeJSON(w, r, maxCredentialBytes, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	if body.Credential == "" {
		s.fail(w, r, badRequest("credential is required"))
		return
	}

	email, err := s.google.Verify(r.Context(), body.Credential)
	switch {
	case errors.Is(err, auth.ErrRejected):
		s.log.InfoContext(r.Context(), "sign-in rejected", slog.Any("reason", err))
		s.fail(w, r, errUnauthorized)
		return
	case err != nil:
		// Not the caller's fault: we could not reach Google.
		s.log.ErrorContext(r.Context(), "sign-in failed", slog.Any("error", err))
		writeError(w, http.StatusBadGateway, "Could not verify the sign-in with Google")
		return
	}
	if _, ok := s.allowed[email]; !ok {
		s.log.WarnContext(r.Context(), "sign-in by unlisted account", slog.String("email", email))
		s.fail(w, r, errForbidden)
		return
	}

	token, expires := s.signer.Issue(email, sessionTTL)
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "expiresAt": expires, "email": email})
}

// passwordLogin trades the admin password for a session token. It is the
// simple way in while the owner is the only user; the per-address limiter on
// the route is what stands between it and guessing.
func (s *server) passwordLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, maxCredentialBytes, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	guess := sha256.Sum256([]byte(body.Password))
	if body.Password == "" || subtle.ConstantTimeCompare(guess[:], s.passwordHash[:]) != 1 {
		s.log.WarnContext(r.Context(), "password sign-in rejected", slog.String("ip", clientIP(r)))
		s.fail(w, r, errUnauthorized)
		return
	}

	token, expires := s.signer.Issue(s.owner, sessionTTL)
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "expiresAt": expires, "email": s.owner})
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"email": owner(r)})
}
