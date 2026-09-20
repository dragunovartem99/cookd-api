package chat

import (
	"encoding/base64"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/dragunovartem99/cookd-api/internal/store"
)

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

// toParams maps stored messages to API messages, photos first so the model has
// seen them before it reads the question about them. Photos older than
// photoMessages are replaced by a note. The memory block goes between the
// photos and the question, on the last message only: it changes every turn, so
// keeping it at the end leaves everything before it cacheable.
func toParams(history []store.Message, memory Memory) []anthropic.MessageParam {
	history = recent(history)
	params := make([]anthropic.MessageParam, 0, len(history))
	for n, m := range history {
		var blocks []anthropic.ContentBlockParamUnion
		if len(history)-n <= photoMessages {
			for _, img := range m.Images {
				blocks = append(blocks, anthropic.NewImageBlockBase64(img.MediaType, base64.StdEncoding.EncodeToString(img.Data)))
			}
		} else if len(m.Images) > 0 {
			blocks = append(blocks, anthropic.NewTextBlock(fmt.Sprintf("[%d photo(s) sent here, no longer shown]", len(m.Images))))
		}
		if block := memory.Block(); block != "" && n == len(history)-1 {
			blocks = append(blocks, anthropic.NewTextBlock(block))
		}
		if m.Text != "" {
			blocks = append(blocks, anthropic.NewTextBlock(m.Text))
		}
		if m.Role == "assistant" {
			params = append(params, anthropic.NewAssistantMessage(blocks...))
		} else {
			params = append(params, anthropic.NewUserMessage(blocks...))
		}
	}
	return params
}
