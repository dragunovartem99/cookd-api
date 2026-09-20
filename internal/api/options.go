package api

import (
	"context"
	"log/slog"

	"github.com/dragunovartem99/cookd-api/internal/auth"
	"github.com/dragunovartem99/cookd-api/internal/chat"
	"github.com/dragunovartem99/cookd-api/internal/store"
)

// Google verifies a Google ID token and returns the email it was issued for.
// It is declared here, rather than imported, so the handlers can be exercised
// without a network.
type Google interface {
	Verify(ctx context.Context, idToken string) (string, error)
}

// Chat streams a Claude reply to a conversation.
type Chat interface {
	Stream(ctx context.Context, history []store.Message, memory chat.Memory, onDelta func(string)) (chat.Result, error)
}

// Options configure NewServer. Everything but Logger is required.
type Options struct {
	Store *store.Store
	Chat  Chat
	// Google is nil when Google sign-in is off.
	Google Google
	Signer *auth.Signer
	// AdminPassword and Owner enable POST /auth/login: the password signs in
	// as Owner. Both blank turns it off.
	AdminPassword string
	Owner         string
	// AllowedEmails are the accounts that may sign in and use the API.
	AllowedEmails map[string]struct{}
	// AllowedOrigin is the one browser origin allowed to call the API.
	AllowedOrigin string
	// Logger receives request and failure records. Defaults to slog's default.
	Logger *slog.Logger
}
