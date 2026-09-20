package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dragunovartem99/cookd-api/internal/auth"
	"github.com/dragunovartem99/cookd-api/internal/store"
)

const (
	maxCredentialBytes = 8 << 10
	// maxMessageBody covers four photos as base64 (a third larger than the
	// bytes they carry) plus the text.
	maxMessageBody = 24 << 20
	maxText        = 8000
	maxImages      = 4
	// maxImageBytes is the Claude API's per-image limit.
	maxImageBytes = 5 << 20
	// messageReadTimeout is how long a slow upload gets, past the server default.
	messageReadTimeout = 2 * time.Minute
)

// imageTypes are the formats the model accepts.
var imageTypes = map[string]bool{"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true}

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

type conversationJSON struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func toConversationJSON(c store.Conversation) conversationJSON {
	return conversationJSON{ID: c.ID, Title: c.Title, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
}

func (s *server) listConversations(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListConversations(r.Context(), owner(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]conversationJSON, len(list))
	for i, c := range list {
		out[i] = toConversationJSON(c)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) createConversation(w http.ResponseWriter, r *http.Request) {
	c, err := s.store.CreateConversation(r.Context(), owner(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toConversationJSON(c))
}

func (s *server) getConversation(w http.ResponseWriter, r *http.Request) {
	c, err := s.store.Conversation(r.Context(), owner(r), r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toConversationJSON(c))
}

func (s *server) deleteConversation(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteConversation(r.Context(), owner(r), r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type messageJSON struct {
	ID        int64       `json:"id"`
	Role      string      `json:"role"`
	Text      string      `json:"text"`
	Images    []imageJSON `json:"images"`
	CreatedAt time.Time   `json:"createdAt"`
}

type imageJSON struct {
	ID        int64  `json:"id"`
	MediaType string `json:"mediaType"`
}

func (s *server) listMessages(w http.ResponseWriter, r *http.Request) {
	messages, err := s.store.Messages(r.Context(), owner(r), r.PathValue("id"), false)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]messageJSON, len(messages))
	for i, m := range messages {
		out[i] = messageJSON{ID: m.ID, Role: m.Role, Text: m.Text, Images: []imageJSON{}, CreatedAt: m.CreatedAt}
		for _, img := range m.Images {
			out[i].Images = append(out[i].Images, imageJSON{ID: img.ID, MediaType: img.MediaType})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) getImage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.fail(w, r, errNotFound)
		return
	}
	img, err := s.store.Image(r.Context(), owner(r), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", img.MediaType)
	h.Set("X-Content-Type-Options", "nosniff")
	// A photo never changes once stored, but it is private to the account.
	h.Set("Cache-Control", "private, max-age=86400, immutable")
	w.Write(img.Data)
}

type sendRequest struct {
	Text   string      `json:"text"`
	Images []sendImage `json:"images"`
}

type sendImage struct {
	MediaType string `json:"mediaType"`
	// Data is the photo, standard base64.
	Data string `json:"data"`
}

// sendMessage adds the user's message to a conversation and streams the
// answer back as Server-Sent Events:
//
//	event: delta  data: {"text": "..."}
//	event: done   data: {"messageId": 12, "title": "...", "stopReason": "end_turn"}
//	event: error  data: {"error": "..."}
//
// The exchange is stored only once the answer is complete, so a dropped
// connection leaves the conversation exactly as it was.
func (s *server) sendMessage(w http.ResponseWriter, r *http.Request) {
	// Photos over a phone connection can outlast the default read timeout.
	_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(messageReadTimeout))

	var body sendRequest
	if err := decodeJSON(w, r, maxMessageBody, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	user, err := buildUserMessage(body)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	id, who := r.PathValue("id"), owner(r)
	history, err := s.store.Messages(r.Context(), who, id, true)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	history = append(history, user)
	memory, err := s.loadMemory(r.Context(), who)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	// Everything that can fail with a proper status has been checked; from here
	// the response is a stream.
	events := startStream(w)
	result, err := s.chat.Stream(r.Context(), history, memory, func(text string) {
		_ = events.send("delta", map[string]string{"text": text})
	})
	if err != nil {
		if r.Context().Err() == nil {
			s.log.ErrorContext(r.Context(), "model call failed", slog.Any("error", err))
			_ = events.send("error", map[string]string{"error": "The model could not answer right now"})
		}
		return
	}
	if result.StopReason == "refusal" {
		_ = events.send("error", map[string]string{"error": "The model declined to answer that"})
		return
	}

	messageID, err := s.store.AppendExchange(r.Context(), who, id, user, result.Text)
	if err != nil {
		s.log.ErrorContext(r.Context(), "storing exchange failed", slog.Any("error", err))
		_ = events.send("error", map[string]string{"error": "The answer could not be saved"})
		return
	}
	conversation, _ := s.store.Conversation(r.Context(), who, id)
	_ = events.send("done", map[string]any{
		"messageId":  messageID,
		"title":      conversation.Title,
		"stopReason": result.StopReason,
	})
}

// buildUserMessage validates a request body and turns it into a message.
// A photo's declared type must match what its bytes actually are.
func buildUserMessage(body sendRequest) (store.Message, error) {
	text := strings.TrimSpace(body.Text)
	if utf8.RuneCountInString(text) > maxText {
		return store.Message{}, badRequest("text is too long")
	}
	if text == "" && len(body.Images) == 0 {
		return store.Message{}, badRequest("text or an image is required")
	}
	if len(body.Images) > maxImages {
		return store.Message{}, badRequest("at most " + strconv.Itoa(maxImages) + " images per message")
	}

	msg := store.Message{Role: "user", Text: text}
	for _, in := range body.Images {
		if !imageTypes[in.MediaType] {
			return store.Message{}, badRequest("unsupported image type; use jpeg, png, gif or webp")
		}
		data, err := base64.StdEncoding.DecodeString(in.Data)
		if err != nil {
			return store.Message{}, badRequest("image data is not valid base64")
		}
		if len(data) == 0 || len(data) > maxImageBytes {
			return store.Message{}, badRequest("each image must be between 1 byte and 5 MiB")
		}
		if http.DetectContentType(data) != in.MediaType {
			return store.Message{}, badRequest("image bytes do not match their mediaType")
		}
		msg.Images = append(msg.Images, store.Image{MediaType: in.MediaType, Data: data})
	}
	return msg, nil
}

// decodeJSON reads a size-capped JSON body, rejecting unknown fields so a typo
// in a client fails loudly instead of being ignored.
func decodeJSON(w http.ResponseWriter, r *http.Request, limit int64, into any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return errTooLarge
		}
		return badRequest("body must be valid JSON matching the documented shape")
	}
	return nil
}
