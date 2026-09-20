package api

import (
	"net/http"
	"time"

	"github.com/dragunovartem99/cookd-api/internal/store"
)

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
