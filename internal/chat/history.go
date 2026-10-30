package chat

import "github.com/dragunovartem99/cookd-api/internal/store"

const (
	// keepMessages is the least history sent: a conversation is trimmed only
	// once it is a full step longer.
	keepMessages = 12
	// trimStep is how many messages are dropped at a time. The window moves in
	// jumps, not one message per turn, so the cached prefix survives most turns.
	// Even, so the window always starts on a user message.
	trimStep = 8
	// photoMessages is how far back photos are still sent: the new message and
	// the exchange before it. Older ones are a large share of every request.
	photoMessages = 3
)

// recent drops the oldest messages of a long conversation.
func recent(history []store.Message) []store.Message {
	if extra := len(history) - keepMessages; extra > 0 {
		history = history[extra/trimStep*trimStep:]
	}
	for len(history) > 1 && history[0].Role != "user" {
		history = history[1:]
	}
	return history
}
