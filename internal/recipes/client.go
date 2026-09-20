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
// pepper, oil and water are assumed), best use of the pantry first. Every
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
	found = slices.DeleteFunc(found, func(r apiRecipe) bool { return !r.cookable() })
	slices.SortStableFunc(found, func(a, b apiRecipe) int { return b.UsedIngredientCount - a.UsedIngredientCount })
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
		for _, a := range full {
			r := a.recipe()
			c.cache.put("recipe:"+strconv.Itoa(r.ID), r)
			results = append(results, r.Summary)
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
