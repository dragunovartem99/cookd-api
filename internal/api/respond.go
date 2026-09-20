package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/dragunovartem99/cookd-api/internal/store"
)

// apiError is a failure whose status and message are safe to show the client.
type apiError struct {
	status  int
	message string
}

func (e *apiError) Error() string { return e.message }

func badRequest(message string) error { return &apiError{http.StatusBadRequest, message} }

var (
	errUnauthorized = &apiError{http.StatusUnauthorized, "Sign in required"}
	errForbidden    = &apiError{http.StatusForbidden, "This account is not allowed"}
	errNotFound     = &apiError{http.StatusNotFound, "Not found"}
	errTooLarge     = &apiError{http.StatusRequestEntityTooLarge, "Request body is too large"}
)

// fail answers with the status the error asks for. Anything that is not an
// apiError is a bug or an upstream fault on our side: it becomes a 500 with the
// details kept to the log.
func (s *server) fail(w http.ResponseWriter, r *http.Request, err error) {
	var known *apiError
	switch {
	case errors.As(err, &known):
	case errors.Is(err, store.ErrNotFound):
		known = errNotFound
	default:
		s.log.ErrorContext(r.Context(), "request failed",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Any("error", err),
		)
		known = &apiError{http.StatusInternalServerError, "Internal server error"}
	}
	writeError(w, known.status, known.message)
}

// writeError renders the { "error": string } body the OpenAPI document
// promises for every failure.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, `{"error":"Internal server error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	w.Write(body)
}

func (s *server) notFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "Not found")
}
