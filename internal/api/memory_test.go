package api

import (
	"strconv"
	"testing"
)

// The point of the feature: what is in the pantry and the journal when a
// message is sent is what the model is handed.
func TestMessageCarriesMemory(t *testing.T) {
	h := newHarness(t)
	h.do("POST", "/ingredients", me, map[string]string{"text": "eggs, milk"})
	h.do("PATCH", "/ingredients/"+strconv.FormatInt(h.ingredients(t, me)[1].ID, 10), me, map[string]bool{"inStock": false})
	h.do("POST", "/journal", me, map[string]any{"dish": "Omelette", "taste": 4, "minutes": 15})
	h.do("POST", "/ingredients", other, map[string]string{"text": "someone else's truffle"})

	id := h.newConversation(t, me)
	h.do("POST", "/conversations/"+id+"/messages", me, map[string]string{"text": "dinner?"})

	mem := h.chat.memory
	if len(mem.Ingredients) != 2 || mem.Ingredients[0].Name != "eggs" || !mem.Ingredients[0].InStock || mem.Ingredients[1].InStock {
		t.Errorf("memory ingredients = %+v, want eggs in stock and milk out", mem.Ingredients)
	}
	if len(mem.Journal) != 1 || mem.Journal[0].Dish != "Omelette" || mem.Today == "" {
		t.Errorf("memory journal = %+v, today = %q", mem.Journal, mem.Today)
	}

	// Only the last few dishes are shared, not the whole notebook.
	for range memoryJournalEntries + 5 {
		h.do("POST", "/journal", me, map[string]any{"dish": "Toast", "taste": 3})
	}
	h.do("POST", "/conversations/"+id+"/messages", me, map[string]string{"text": "and now?"})
	if n := len(h.chat.memory.Journal); n != memoryJournalEntries {
		t.Errorf("model was shown %d journal entries, want %d", n, memoryJournalEntries)
	}
}
