// Package api serves the cookd HTTP API.
package api

import (
	"crypto/sha256"
	"log/slog"
	"net/http"
	"time"

	"github.com/dragunovartem99/cookd-api/internal/auth"
	"github.com/dragunovartem99/cookd-api/internal/store"
)

// Budgets. Sign-ins are rationed per address because they are the only
// unauthenticated write; messages per account because each one spends money.
const (
	signInRequests = 10
	signInWindow   = 10 * time.Minute

	messageRequests = 30
	messageWindow   = 10 * time.Minute

	readRequests = 120
	readWindow   = time.Minute

	// sessionTTL is how long a sign-in lasts before Google has to be asked again.
	sessionTTL = 30 * 24 * time.Hour
)

type server struct {
	store   *store.Store
	chat    Chat
	google  Google
	signer  *auth.Signer
	allowed map[string]struct{}
	// passwordHash is the digest of the admin password; comparing digests keeps
	// the comparison constant-time whatever the guess's length.
	passwordHash [sha256.Size]byte
	owner        string
	log          *slog.Logger
}

// NewServer builds the API handler.
func NewServer(opts Options) http.Handler {
	s := &server{
		store:   opts.Store,
		chat:    opts.Chat,
		google:  opts.Google,
		signer:  opts.Signer,
		allowed: opts.AllowedEmails,
		owner:   opts.Owner,
		log:     opts.Logger,
	}
	if opts.AdminPassword != "" {
		s.passwordHash = sha256.Sum256([]byte(opts.AdminPassword))
	}
	if s.log == nil {
		s.log = slog.Default()
	}

	signIn := newLimiter(signInRequests, signInWindow).limit(clientIP)
	send := newLimiter(messageRequests, messageWindow).limit(userKey)
	authed := s.requireAuth

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	if opts.Google != nil {
		mux.Handle("POST /auth/google", signIn(http.HandlerFunc(s.signIn)))
	}
	if opts.AdminPassword != "" {
		mux.Handle("POST /auth/login", signIn(http.HandlerFunc(s.passwordLogin)))
	}

	mux.Handle("GET /me", authed(http.HandlerFunc(s.me)))
	mux.Handle("GET /conversations", authed(http.HandlerFunc(s.listConversations)))
	mux.Handle("POST /conversations", authed(http.HandlerFunc(s.createConversation)))
	mux.Handle("GET /conversations/{id}", authed(http.HandlerFunc(s.getConversation)))
	mux.Handle("DELETE /conversations/{id}", authed(http.HandlerFunc(s.deleteConversation)))
	mux.Handle("GET /conversations/{id}/messages", authed(http.HandlerFunc(s.listMessages)))
	mux.Handle("POST /conversations/{id}/messages", authed(send(http.HandlerFunc(s.sendMessage))))
	mux.Handle("GET /images/{id}", authed(http.HandlerFunc(s.getImage)))

	mux.Handle("GET /ingredients", authed(http.HandlerFunc(s.listIngredients)))
	mux.Handle("POST /ingredients", authed(http.HandlerFunc(s.addIngredients)))
	mux.Handle("PATCH /ingredients/{id}", authed(http.HandlerFunc(s.updateIngredient)))
	mux.Handle("DELETE /ingredients/{id}", authed(http.HandlerFunc(s.deleteIngredient)))

	mux.Handle("GET /journal", authed(http.HandlerFunc(s.listJournal)))
	mux.Handle("POST /journal", authed(http.HandlerFunc(s.addJournalEntry)))
	mux.Handle("DELETE /journal/{id}", authed(http.HandlerFunc(s.deleteJournalEntry)))
	mux.HandleFunc("/", s.notFound)

	return chain(mux,
		recoverPanics(s.log),
		logRequests(s.log),
		cors(opts.AllowedOrigin),
		newLimiter(readRequests, readWindow).limit(clientIP),
	)
}

type middleware func(http.Handler) http.Handler

// chain wraps h so that the first middleware listed is the outermost one, i.e.
// the order they are written is the order a request travels through them.
func chain(h http.Handler, wrappers ...middleware) http.Handler {
	for i := len(wrappers) - 1; i >= 0; i-- {
		h = wrappers[i](h)
	}
	return h
}
