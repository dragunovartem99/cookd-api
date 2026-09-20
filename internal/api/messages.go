package api

import (
	"net/http"
	"strconv"
	"time"
)

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
