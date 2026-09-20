package api

import (
	"context"
	"errors"
	"strings"

	"github.com/dragunovartem99/cookd-api/internal/auth"
	"github.com/dragunovartem99/cookd-api/internal/chat"
	"github.com/dragunovartem99/cookd-api/internal/store"
)

type fakeGoogle struct{}

func (fakeGoogle) Verify(_ context.Context, idToken string) (string, error) {
	switch idToken {
	case "good":
		return me, nil
	case "stranger":
		return "stranger@example.com", nil
	case "down":
		return "", errors.New("google unreachable")
	}
	return "", auth.ErrRejected
}

// fakeChat answers with a canned reply and records what it was asked.
type fakeChat struct {
	reply   chat.Result
	err     error
	history []store.Message
	memory  chat.Memory
}

func (f *fakeChat) Stream(_ context.Context, history []store.Message, memory chat.Memory, onDelta func(string)) (chat.Result, error) {
	f.history, f.memory = history, memory
	if f.err != nil {
		return chat.Result{}, f.err
	}
	for _, piece := range strings.SplitAfter(f.reply.Text, " ") {
		onDelta(piece)
	}
	return f.reply, nil
}
