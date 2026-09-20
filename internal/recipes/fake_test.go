package recipes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const found = `[
 {"id":1,"title":"Omelette","usedIngredientCount":2,"missedIngredients":[{"name":"Salt"}]},
 {"id":2,"title":"Frittata","usedIngredientCount":3,"missedIngredients":[{"name":"olive oil"}]},
 {"id":3,"title":"Cheesecake","usedIngredientCount":2,"missedIngredients":[{"name":"cream cheese"}]},
 {"id":4,"title":"Boiled egg","usedIngredientCount":1,"missedIngredients":[]}
]`

const bulk = `[
 {"id":2,"title":"Frittata","readyInMinutes":20,"sourceUrl":"https://example.com/frittata",
  "extendedIngredients":[{"original":"3 eggs"}],
  "analyzedInstructions":[{"steps":[{"step":"Whisk."},{"step":"Bake."}]}]},
 {"id":1,"title":"Omelette","readyInMinutes":10,"sourceUrl":"https://example.com/omelette"}
]`

func fakeAPI(t *testing.T, calls *atomic.Int32) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Query().Get("apiKey") != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.URL.Path == "/recipes/findByIngredients":
			if r.URL.Query().Get("ingredients") != "eggs,milk" {
				t.Errorf("ingredients = %q", r.URL.Query().Get("ingredients"))
			}
			_, _ = w.Write([]byte(found))
		case r.URL.Path == "/recipes/informationBulk":
			if got := r.URL.Query().Get("ids"); got != "2,1,4" {
				t.Errorf("ids = %q, want the cookable ones, most-used first", got)
			}
			_, _ = w.Write([]byte(bulk))
		case strings.HasSuffix(r.URL.Path, "/information"):
			_, _ = w.Write([]byte(`{"id":9,"title":"Soup"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	c := NewClient("secret")
	c.base = srv.URL
	return c
}
