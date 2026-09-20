package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORS(t *testing.T) {
	h := newHarness(t)

	preflight := httptest.NewRequest("OPTIONS", "/conversations", nil)
	preflight.Header.Set("Origin", "https://ui.example")
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, preflight)
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "https://ui.example" {
		t.Errorf("preflight from the UI = %d %v", rec.Code, rec.Header())
	}
	if !strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Error("preflight does not allow the Authorization header")
	}

	foreign := httptest.NewRequest("OPTIONS", "/conversations", nil)
	foreign.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	h.handler.ServeHTTP(rec, foreign)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("a foreign origin was allowed")
	}
}
