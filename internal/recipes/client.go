// Package recipes finds real, published recipes on Spoonacular, so the coach
// offers dishes that exist instead of inventing them.
package recipes

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	baseURL = "https://api.spoonacular.com"
	// candidates is how many pantry matches are fetched; most are dropped for
	// needing more than the pantry, and each extra one costs a hundredth of a point.
	candidates = 20
	// maxResults is how many dishes a search hands back.
	maxResults = 5
	// maxMissing is how many items a dish may lack and still be offered, so an
	// empty result can turn into "buy this one thing".
	maxMissing = 1
)

type Client struct {
	key   string
	base  string
	http  *http.Client
	cache *cache
}

func NewClient(apiKey string) *Client {
	return &Client{key: apiKey, base: baseURL, http: &http.Client{Timeout: 10 * time.Second}, cache: newCache()}
}

// Search finds dishes that can be cooked from the ingredients alone (salt,
// pepper, oil and water are assumed) or with one more item, which the result
// names in Missing. Fully cookable dishes come first, then the best use of
// the pantry. Every
// result carries its full recipe in the cache, so Get on it costs nothing.
func (c *Client) Search(ctx context.Context, ingredients []string) ([]Summary, error) {
	key := "search:" + strings.ToLower(strings.Join(ingredients, ","))
	if hit, ok := c.cache.get(key); ok {
		return hit.([]Summary), nil
	}
	var found []apiRecipe
	err := c.get(ctx, "/recipes/findByIngredients", url.Values{
		"ingredients": {strings.Join(ingredients, ",")},
		"number":      {strconv.Itoa(candidates)},
		"ranking":     {"2"},
	}, &found)
	if err != nil {
		return nil, err
	}
	missing := map[int][]string{}
	for _, r := range found {
		missing[r.ID] = r.missing()
	}
	found = slices.DeleteFunc(found, func(r apiRecipe) bool { return len(missing[r.ID]) > maxMissing })
	slices.SortStableFunc(found, func(a, b apiRecipe) int {
		if d := len(missing[a.ID]) - len(missing[b.ID]); d != 0 {
			return d
		}
		return b.UsedIngredientCount - a.UsedIngredientCount
	})
	found = found[:min(len(found), maxResults)]

	results := []Summary{}
	if len(found) > 0 {
		ids := make([]string, len(found))
		for n, r := range found {
			ids[n] = strconv.Itoa(r.ID)
		}
		var full []apiRecipe
		if err := c.get(ctx, "/recipes/informationBulk", url.Values{"ids": {strings.Join(ids, ",")}}, &full); err != nil {
			return nil, err
		}
		byID := map[int]Recipe{}
		for _, a := range full {
			r := a.recipe()
			c.cache.put("recipe:"+strconv.Itoa(r.ID), r)
			byID[r.ID] = r
		}
		// Missing depends on the pantry, so it stays out of the cached recipe.
		for _, f := range found {
			if r, ok := byID[f.ID]; ok {
				r.Missing = missing[f.ID]
				results = append(results, r.Summary)
			}
		}
	}
	c.cache.put(key, results)
	return results, nil
}

// Get returns one recipe by the ID a search gave.
func (c *Client) Get(ctx context.Context, id int) (Recipe, error) {
	key := "recipe:" + strconv.Itoa(id)
	if hit, ok := c.cache.get(key); ok {
		return hit.(Recipe), nil
	}
	var a apiRecipe
	if err := c.get(ctx, fmt.Sprintf("/recipes/%d/information", id), nil, &a); err != nil {
		return Recipe{}, err
	}
	r := a.recipe()
	c.cache.put(key, r)
	return r, nil
}
