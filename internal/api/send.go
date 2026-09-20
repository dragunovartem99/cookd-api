package api

import (
	"log/slog"
	"net/http"
	"time"
)

const (
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
