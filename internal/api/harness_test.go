package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dragunovartem99/cookd-api/internal/auth"
	"github.com/dragunovartem99/cookd-api/internal/chat"
	"github.com/dragunovartem99/cookd-api/internal/store"
)

const (
	me       = "me@example.com"
	other    = "other@example.com"
	password = "correct horse battery staple"
)

type harness struct {
	handler http.Handler
	signer  *auth.Signer
	chat    *fakeChat
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	return newHarnessWith(t, fakeGoogle{})
}

// newHarnessWith lets a test choose the Google verifier; nil turns it off.
func newHarnessWith(t *testing.T, google Google) *harness {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	h := &harness{
		signer: auth.NewSigner([]byte(strings.Repeat("k", 32))),
		chat:   &fakeChat{reply: chat.Result{Text: "Make an omelette.", StopReason: "end_turn"}},
	}
	h.handler = NewServer(Options{
		Store: db, Chat: h.chat, Google: google, Signer: h.signer, AdminPassword: password, Owner: me,
		AllowedEmails: map[string]struct{}{me: {}, other: {}},
		AllowedOrigin: "https://ui.example",
	})
	return h
}

func (h *harness) do(method, path, email string, body any) *httptest.ResponseRecorder {
	var reader bytes.Buffer
	if body != nil {
		json.NewEncoder(&reader).Encode(body)
	}
	req := httptest.NewRequest(method, path, &reader)
	if email != "" {
		token, _ := h.signer.Issue(email, time.Hour)
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

func (h *harness) newConversation(t *testing.T, email string) string {
	t.Helper()
	rec := h.do("POST", "/conversations", email, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create conversation: %d %s", rec.Code, rec.Body)
	}
	var c conversationJSON
	json.Unmarshal(rec.Body.Bytes(), &c)
	return c.ID
}

// pngBytes is the smallest thing http.DetectContentType calls a PNG.
var pngBytes = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
