package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dragunovartem99/cookd-api/internal/store"
)

const (
	maxDishName    = 100
	maxJournalNote = 500
	maxMinutes     = 24 * 60
	// journalListLimit caps GET /journal; memoryJournalEntries is how much of it
	// the model reads. A handful of recent dishes is enough to adapt to, and
	// keeps every message cheap.
	journalListLimit     = 200
	memoryJournalEntries = 15
)

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
