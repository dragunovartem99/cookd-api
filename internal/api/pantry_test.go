package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func (h *harness) ingredients(t *testing.T, email string) []ingredientJSON {
	t.Helper()
	var list []ingredientJSON
	json.Unmarshal(h.do("GET", "/ingredients", email, nil).Body.Bytes(), &list)
	return list
}

func TestIngredientsLifecycle(t *testing.T) {
	h := newHarness(t)

	rec := h.do("POST", "/ingredients", me, map[string]string{"text": "Eggs, rice\n tomatoes ;  EGGS,, "})
	if rec.Code != http.StatusOK {
		t.Fatalf("add = %d %s", rec.Code, rec.Body)
	}
	list := h.ingredients(t, me)
	if len(list) != 3 || list[0].Name != "eggs" || list[1].Name != "rice" || list[2].Name != "tomatoes" {
		t.Fatalf("pantry = %+v, want eggs, rice, tomatoes once each, lowercased and sorted", list)
	}
	for _, in := range list {
		if !in.InStock {
			t.Errorf("%s was added out of stock", in.Name)
		}
	}

	// Run out of rice, then buy it again by typing it in.
	rice := list[1].ID
	rec = h.do("PATCH", "/ingredients/"+strconv.FormatInt(rice, 10), me, map[string]bool{"inStock": false})
	if rec.Code != http.StatusOK || h.ingredients(t, me)[1].InStock {
		t.Fatalf("switch off = %d %s", rec.Code, rec.Body)
	}
	h.do("POST", "/ingredients", me, map[string]string{"text": "Rice"})
	after := h.ingredients(t, me)
	if len(after) != 3 || !after[1].InStock || after[1].ID != rice {
		t.Errorf("re-adding rice gave %+v, want the same item switched back on", after)
	}

	if rec := h.do("DELETE", "/ingredients/"+strconv.FormatInt(rice, 10), me, nil); rec.Code != http.StatusNoContent {
		t.Errorf("delete = %d", rec.Code)
	}
	if n := len(h.ingredients(t, me)); n != 2 {
		t.Errorf("%d ingredients after delete, want 2", n)
	}
}

func TestIngredientsValidation(t *testing.T) {
	h := newHarness(t)
	var tooMany []string
	for i := range maxIngredientsAdd + 1 {
		tooMany = append(tooMany, "item"+strconv.Itoa(i))
	}

	for name, text := range map[string]string{
		"blank":    " , ;\n",
		"too long": strings.Repeat("x", maxIngredientName+1),
		"too many": strings.Join(tooMany, ","),
	} {
		if rec := h.do("POST", "/ingredients", me, map[string]string{"text": text}); rec.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400 (%s)", name, rec.Code, rec.Body)
		}
	}
	if rec := h.do("PATCH", "/ingredients/1", me, map[string]string{}); rec.Code != http.StatusBadRequest {
		t.Errorf("patch without inStock = %d, want 400", rec.Code)
	}
	if rec := h.do("PATCH", "/ingredients/nope", me, map[string]bool{"inStock": true}); rec.Code != http.StatusNotFound {
		t.Errorf("patch with a bad id = %d, want 404", rec.Code)
	}
}

func TestIngredientsArePrivate(t *testing.T) {
	h := newHarness(t)
	h.do("POST", "/ingredients", me, map[string]string{"text": "saffron"})
	id := strconv.FormatInt(h.ingredients(t, me)[0].ID, 10)

	if rec := h.do("PATCH", "/ingredients/"+id, other, map[string]bool{"inStock": false}); rec.Code != http.StatusNotFound {
		t.Errorf("another user's patch = %d, want 404", rec.Code)
	}
	if rec := h.do("DELETE", "/ingredients/"+id, other, nil); rec.Code != http.StatusNotFound {
		t.Errorf("another user's delete = %d, want 404", rec.Code)
	}
	if n := len(h.ingredients(t, other)); n != 0 {
		t.Errorf("another user sees %d ingredients", n)
	}
	if !h.ingredients(t, me)[0].InStock {
		t.Error("the other user's patch got through")
	}
}

func TestJournalLifecycle(t *testing.T) {
	h := newHarness(t)
	today := time.Now().Format(time.DateOnly)

	rec := h.do("POST", "/journal", me, map[string]any{"dish": " Omelette ", "taste": 4, "minutes": 15, "note": "too salty"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("add = %d %s", rec.Code, rec.Body)
	}
	var first journalJSON
	json.Unmarshal(rec.Body.Bytes(), &first)
	if first.Dish != "Omelette" || first.CookedOn != today || first.Minutes == nil || *first.Minutes != 15 {
		t.Errorf("entry = %+v", first)
	}

	// Time is optional; an older date sorts below a newer one whatever the insert order.
	h.do("POST", "/journal", me, map[string]any{"dish": "Fried rice", "taste": 3, "cookedOn": "2026-01-02"})
	h.do("POST", "/journal", me, map[string]any{"dish": "Soup", "taste": 5, "cookedOn": "2026-03-04"})

	var list []journalJSON
	json.Unmarshal(h.do("GET", "/journal", me, nil).Body.Bytes(), &list)
	if len(list) != 3 || list[0].Dish != "Omelette" || list[1].Dish != "Soup" || list[2].Dish != "Fried rice" || list[2].Minutes != nil {
		t.Fatalf("journal = %+v, want newest cooked first", list)
	}

	if rec := h.do("DELETE", "/journal/"+strconv.FormatInt(first.ID, 10), me, nil); rec.Code != http.StatusNoContent {
		t.Errorf("delete = %d", rec.Code)
	}
	if rec := h.do("DELETE", "/journal/"+strconv.FormatInt(first.ID, 10), me, nil); rec.Code != http.StatusNotFound {
		t.Errorf("second delete = %d, want 404", rec.Code)
	}
}

func TestJournalValidation(t *testing.T) {
	h := newHarness(t)
	tomorrowPlus := time.Now().AddDate(0, 0, 3).Format(time.DateOnly)

	for name, body := range map[string]map[string]any{
		"no dish":        {"taste": 3},
		"taste too low":  {"dish": "x", "taste": 0},
		"taste too high": {"dish": "x", "taste": 6},
		"zero minutes":   {"dish": "x", "taste": 3, "minutes": 0},
		"absurd minutes": {"dish": "x", "taste": 3, "minutes": 5000},
		"bad date":       {"dish": "x", "taste": 3, "cookedOn": "yesterday"},
		"future date":    {"dish": "x", "taste": 3, "cookedOn": tomorrowPlus},
		"long note":      {"dish": "x", "taste": 3, "note": strings.Repeat("n", maxJournalNote+1)},
		"unknown field":  {"dish": "x", "taste": 3, "rating": 5},
	} {
		if rec := h.do("POST", "/journal", me, body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400 (%s)", name, rec.Code, rec.Body)
		}
	}
}

func TestJournalIsPrivate(t *testing.T) {
	h := newHarness(t)
	rec := h.do("POST", "/journal", me, map[string]any{"dish": "Secret stew", "taste": 5})
	var entry journalJSON
	json.Unmarshal(rec.Body.Bytes(), &entry)

	if rec := h.do("DELETE", "/journal/"+strconv.FormatInt(entry.ID, 10), other, nil); rec.Code != http.StatusNotFound {
		t.Errorf("another user's delete = %d, want 404", rec.Code)
	}
	var list []journalJSON
	json.Unmarshal(h.do("GET", "/journal", other, nil).Body.Bytes(), &list)
	if len(list) != 0 {
		t.Errorf("another user sees %d entries", len(list))
	}
}

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
