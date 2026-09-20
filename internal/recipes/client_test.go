package recipes

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSearchKeepsOnlyCookableDishes(t *testing.T) {
	var calls atomic.Int32
	c := fakeAPI(t, &calls)

	got, err := c.Search(context.Background(), []string{"eggs", "milk"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Title != "Frittata" || got[0].Minutes != 20 || got[0].URL != "https://example.com/frittata" {
		t.Errorf("Search = %+v", got)
	}
}

func TestSearchAndGetAreCached(t *testing.T) {
	var calls atomic.Int32
	c := fakeAPI(t, &calls)
	ctx := context.Background()

	if _, err := c.Search(ctx, []string{"eggs", "milk"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Search(ctx, []string{"Eggs", "milk"}); err != nil {
		t.Fatal(err)
	}
	recipe, err := c.Get(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Errorf("%d API calls, want 2: repeat search and Get of a found dish are free", calls.Load())
	}
	if len(recipe.Ingredients) != 1 || len(recipe.Steps) != 2 || recipe.Steps[1] != "Bake." {
		t.Errorf("Get = %+v", recipe)
	}

	if _, err := c.Get(ctx, 9); err != nil || calls.Load() != 3 {
		t.Errorf("uncached Get: err %v, calls %d", err, calls.Load())
	}
}

func TestErrorsDoNotLeakTheKey(t *testing.T) {
	c := NewClient("secret")
	c.base = "http://127.0.0.1:1"
	if _, err := c.Get(context.Background(), 1); err == nil || strings.Contains(err.Error(), "secret") {
		t.Errorf("err = %v, want a failure without the key", err)
	}
}

func TestBadStatusIsAnError(t *testing.T) {
	var calls atomic.Int32
	c := fakeAPI(t, &calls)
	c.key = "wrong"
	if _, err := c.Search(context.Background(), []string{"eggs", "milk"}); err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("err = %v, want the status", err)
	}
}
