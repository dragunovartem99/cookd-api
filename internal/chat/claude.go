// Package chat turns a stored conversation into a streamed Claude reply.
package chat

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/dragunovartem99/cookd-api/internal/store"
)

const (
	model = "claude-sonnet-5"
	// maxTokens leaves room for adaptive thinking on top of a full recipe;
	// the reply is streamed, so a large ceiling does not risk a request timeout.
	maxTokens = 32000
)

// Result is a finished reply.
type Result struct {
	Text string
	// StopReason is the API's: "end_turn", "max_tokens" or "refusal".
	StopReason string
}

type Claude struct {
	client anthropic.Client
}

func NewClaude(apiKey string) *Claude {
	return &Claude{client: anthropic.NewClient(option.WithAPIKey(apiKey))}
}

// Stream sends the conversation to Claude and calls onDelta with each piece of
// the answer as it arrives. history must end with the user's new message;
// memory is shown to the model alongside that message.
func (c *Claude) Stream(ctx context.Context, history []store.Message, memory Memory, onDelta func(string)) (Result, error) {
	stream := c.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: maxTokens,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Messages:  toParams(history, memory),
		// Chat wants quick answers; the default effort thinks longer than a
		// "what can I make with eggs" question deserves.
		OutputConfig: anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffortMedium},
		// The history is resent every turn, so let the API cache the growing prefix.
		CacheControl: anthropic.NewCacheControlEphemeralParam(),
	})

	message := anthropic.Message{}
	for stream.Next() {
		event := stream.Current()
		if err := message.Accumulate(event); err != nil {
			return Result{}, fmt.Errorf("accumulate stream: %w", err)
		}
		if delta, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
			if text, ok := delta.Delta.AsAny().(anthropic.TextDelta); ok {
				onDelta(text.Text)
			}
		}
	}
	if err := stream.Err(); err != nil {
		return Result{}, err
	}

	var text string
	for _, block := range message.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			text += t.Text
		}
	}
	return Result{Text: text, StopReason: string(message.StopReason)}, nil
}
