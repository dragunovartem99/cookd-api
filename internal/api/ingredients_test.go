package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
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
