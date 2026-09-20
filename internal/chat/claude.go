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
	// maxRounds caps model calls per turn: a search, a recipe fetch and a
	// retry or two, so a confused model cannot loop on the API bill.
	maxRounds = 4
)

// Result is a finished reply.
type Result struct {
	Text string
	// StopReason is the API's: "end_turn", "max_tokens" or "refusal".
	StopReason string
}

type Claude struct {
	client  anthropic.Client
	recipes Recipes
}

// NewClaude builds the chat. recipes may be nil: the coach then answers from
// its own knowledge, without lookups or links.
func NewClaude(apiKey string, recipes Recipes) *Claude {
	return &Claude{client: anthropic.NewClient(option.WithAPIKey(apiKey)), recipes: recipes}
}

// Stream sends the conversation to Claude and calls onDelta with each piece of
// the answer as it arrives. history must end with the user's new message;
// memory is shown to the model alongside that message.
func (c *Claude) Stream(ctx context.Context, history []store.Message, memory Memory, onDelta func(string)) (Result, error) {
	messages := toParams(history, memory)
	var text string
	// A turn that looks up recipes takes several calls: each tool result goes
	// back to the model, which then carries on.
	for range maxRounds {
		message, err := c.streamOnce(ctx, messages, onDelta)
		if err != nil {
			return Result{}, err
		}
		for _, block := range message.Content {
			if t, ok := block.AsAny().(anthropic.TextBlock); ok {
				text += t.Text
			}
		}
		if message.StopReason != anthropic.StopReasonToolUse {
			return Result{Text: text, StopReason: string(message.StopReason)}, nil
		}
		messages = append(messages, message.ToParam(), anthropic.NewUserMessage(c.runTools(ctx, message)...))
	}
	return Result{Text: text, StopReason: string(anthropic.StopReasonToolUse)}, nil
}

func (c *Claude) streamOnce(ctx context.Context, messages []anthropic.MessageParam, onDelta func(string)) (anthropic.Message, error) {
	params := anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: maxTokens,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Messages:  messages,
		// Recipes come from the lookup now, so the model mostly rewrites them:
		// little thinking needed, and thinking is billed as output.
		OutputConfig: anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffortLow},
		// The history is resent every turn, so let the API cache the growing prefix.
		CacheControl: anthropic.NewCacheControlEphemeralParam(),
	}
	if c.recipes != nil {
		params.Tools = tools()
	}
	stream := c.client.Messages.NewStreaming(ctx, params)

	message := anthropic.Message{}
	for stream.Next() {
		event := stream.Current()
		if err := message.Accumulate(event); err != nil {
			return message, fmt.Errorf("accumulate stream: %w", err)
		}
		if delta, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
			if text, ok := delta.Delta.AsAny().(anthropic.TextDelta); ok {
				onDelta(text.Text)
			}
		}
	}
	return message, stream.Err()
}
