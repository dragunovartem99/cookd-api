package api

import (
	"context"
	"time"

	"github.com/dragunovartem99/cookd-api/internal/chat"
)

// loadMemory gathers what the model is told about the cook's kitchen and past
// dishes, straight from the database so it can never be stale.
func (s *server) loadMemory(ctx context.Context, who string) (chat.Memory, error) {
	ingredients, err := s.store.Ingredients(ctx, who)
	if err != nil {
		return chat.Memory{}, err
	}
	journal, err := s.store.Journal(ctx, who, memoryJournalEntries)
	if err != nil {
		return chat.Memory{}, err
	}
	return chat.Memory{Today: time.Now().Format(time.DateOnly), Ingredients: ingredients, Journal: journal}, nil
}
