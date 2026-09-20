package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// stream is a Server-Sent Events response. It is used instead of EventSource on
// the client because EventSource cannot send an Authorization header.
type stream struct {
	w  http.ResponseWriter
	rc *http.ResponseController
}

// startStream commits the response to SSE: after this the status is 200 and
// failures can only be reported as events.
func startStream(w http.ResponseWriter) *stream {
	s := &stream{w: w, rc: http.NewResponseController(w)}

	// The server's write timeout suits JSON replies; a model answer can outlast
	// it. Errors here only mean the writer has no deadlines (tests).
	_ = s.rc.SetWriteDeadline(time.Time{})

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	// Ask proxies that buffer by default (nginx) not to sit on the stream.
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	_ = s.rc.Flush()
	return s
}

func (s *stream) send(event string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}
	return s.rc.Flush()
}
