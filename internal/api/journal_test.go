package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

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
