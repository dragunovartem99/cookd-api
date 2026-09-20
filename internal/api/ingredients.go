package api

import (
	"net/http"

	"github.com/dragunovartem99/cookd-api/internal/store"
)

const (
	maxIngredientText = 4 << 10
	maxIngredientName = 60
	maxIngredientsAdd = 50
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
