package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dragunovartem99/cookd-api/internal/chat"
	"github.com/dragunovartem99/cookd-api/internal/store"
)

const (
	maxIngredientText = 4 << 10
	maxIngredientName = 60
	maxIngredientsAdd = 50

	maxDishName    = 100
	maxJournalNote = 500
	maxMinutes     = 24 * 60
	// journalListLimit caps GET /journal; memoryJournalEntries is how much of it
	// the model reads. A handful of recent dishes is enough to adapt to, and
	// keeps every message cheap.
	journalListLimit     = 200
	memoryJournalEntries = 15
)

type ingredientJSON struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	InStock bool   `json:"inStock"`
}

func toIngredientsJSON(list []store.Ingredient) []ingredientJSON {
	out := make([]ingredientJSON, len(list))
	for i, in := range list {
		out[i] = ingredientJSON{ID: in.ID, Name: in.Name, InStock: in.InStock}
	}
	return out
}

func (s *server) listIngredients(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.Ingredients(r.Context(), owner(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toIngredientsJSON(list))
}

// addIngredients takes a pasted list ("eggs, rice\ntomatoes") and answers with
// the whole pantry, so the UI can replace what it shows in one step.
func (s *server) addIngredients(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(w, r, maxIngredientText, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	names, err := parseIngredientNames(body.Text)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.store.AddIngredients(r.Context(), owner(r), names); err != nil {
		s.fail(w, r, err)
		return
	}
	s.listIngredients(w, r)
}

func (s *server) updateIngredient(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	var body struct {
		InStock *bool `json:"inStock"`
	}
	if err := decodeJSON(w, r, maxCredentialBytes, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	if body.InStock == nil {
		s.fail(w, r, badRequest("inStock is required"))
		return
	}
	in, err := s.store.SetInStock(r.Context(), owner(r), id, *body.InStock)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, ingredientJSON{ID: in.ID, Name: in.Name, InStock: in.InStock})
}

func (s *server) deleteIngredient(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.store.DeleteIngredient(r.Context(), owner(r), id); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// parseIngredientNames splits a pasted list on commas, semicolons and line
// breaks. Names are lowercased so "Milk" and "milk" are one ingredient in any
// alphabet.
func parseIngredientNames(text string) ([]string, error) {
	seen := map[string]bool{}
	var names []string
	for _, part := range strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ';' || r == '\n' }) {
		name := strings.ToLower(strings.Join(strings.Fields(part), " "))
		if name == "" || seen[name] {
			continue
		}
		if utf8.RuneCountInString(name) > maxIngredientName {
			return nil, badRequest("an ingredient name is longer than " + strconv.Itoa(maxIngredientName) + " characters")
		}
		seen[name] = true
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil, badRequest("text must list at least one ingredient")
	}
	if len(names) > maxIngredientsAdd {
		return nil, badRequest("at most " + strconv.Itoa(maxIngredientsAdd) + " ingredients at a time")
	}
	return names, nil
}

type journalJSON struct {
	ID        int64     `json:"id"`
	Dish      string    `json:"dish"`
	CookedOn  string    `json:"cookedOn"`
	Taste     int       `json:"taste"`
	Minutes   *int      `json:"minutes"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"createdAt"`
}

func toJournalJSON(e store.JournalEntry) journalJSON {
	return journalJSON{ID: e.ID, Dish: e.Dish, CookedOn: e.CookedOn, Taste: e.Taste, Minutes: e.Minutes, Note: e.Note, CreatedAt: e.CreatedAt}
}

func (s *server) listJournal(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.Journal(r.Context(), owner(r), journalListLimit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]journalJSON, len(list))
	for i, e := range list {
		out[i] = toJournalJSON(e)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) addJournalEntry(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Dish     string `json:"dish"`
		Taste    int    `json:"taste"`
		Minutes  *int   `json:"minutes"`
		Note     string `json:"note"`
		CookedOn string `json:"cookedOn"`
	}
	if err := decodeJSON(w, r, maxCredentialBytes, &body); err != nil {
		s.fail(w, r, err)
		return
	}

	entry := store.JournalEntry{
		Dish:     strings.TrimSpace(body.Dish),
		Taste:    body.Taste,
		Minutes:  body.Minutes,
		Note:     strings.TrimSpace(body.Note),
		CookedOn: body.CookedOn,
	}
	if entry.CookedOn == "" {
		entry.CookedOn = time.Now().Format(time.DateOnly)
	}
	switch {
	case entry.Dish == "" || utf8.RuneCountInString(entry.Dish) > maxDishName:
		s.fail(w, r, badRequest("dish is required, up to "+strconv.Itoa(maxDishName)+" characters"))
		return
	case entry.Taste < 1 || entry.Taste > 5:
		s.fail(w, r, badRequest("taste must be from 1 to 5"))
		return
	case entry.Minutes != nil && (*entry.Minutes < 1 || *entry.Minutes > maxMinutes):
		s.fail(w, r, badRequest("minutes must be from 1 to "+strconv.Itoa(maxMinutes)))
		return
	case utf8.RuneCountInString(entry.Note) > maxJournalNote:
		s.fail(w, r, badRequest("note is too long"))
		return
	}
	// A day of slack so a phone in an earlier timezone than the server can still log "today".
	if day, err := time.Parse(time.DateOnly, entry.CookedOn); err != nil || day.After(time.Now().AddDate(0, 0, 1)) {
		s.fail(w, r, badRequest("cookedOn must be a date like 2026-09-20, not in the future"))
		return
	}

	saved, err := s.store.AddJournalEntry(r.Context(), owner(r), entry)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toJournalJSON(saved))
}

func (s *server) deleteJournalEntry(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.store.DeleteJournalEntry(r.Context(), owner(r), id); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

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

// pathID reads a numeric {id}; anything else cannot exist, so it is a 404.
func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return 0, errNotFound
	}
	return id, nil
}
