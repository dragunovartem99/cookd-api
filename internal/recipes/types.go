package recipes

import "strings"

// Summary is one search hit: enough to offer the dish, nothing more.
type Summary struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Minutes int    `json:"minutes"`
	// URL is the page the recipe was published on.
	URL string `json:"url"`
}

// Recipe is a full recipe.
type Recipe struct {
	Summary
	Ingredients []string `json:"ingredients"`
	Steps       []string `json:"steps"`
}

// apiRecipe is the part of Spoonacular's recipe objects that is used.
type apiRecipe struct {
	ID                  int    `json:"id"`
	Title               string `json:"title"`
	ReadyInMinutes      int    `json:"readyInMinutes"`
	SourceURL           string `json:"sourceUrl"`
	UsedIngredientCount int    `json:"usedIngredientCount"`
	MissedIngredients   []struct {
		Name string `json:"name"`
	} `json:"missedIngredients"`
	ExtendedIngredients []struct {
		Original string `json:"original"`
	} `json:"extendedIngredients"`
	AnalyzedInstructions []struct {
		Steps []struct {
			Step string `json:"step"`
		} `json:"steps"`
	} `json:"analyzedInstructions"`
}

// staples are what the cook is assumed to have besides the pantry. The API's
// own "pantry" also covers flour, sugar and butter, which this cook may lack,
// so it is not used; missing staples are forgiven here instead.
var staples = map[string]bool{
	"salt": true, "sea salt": true, "kosher salt": true,
	"pepper": true, "black pepper": true, "ground black pepper": true,
	"oil": true, "olive oil": true, "vegetable oil": true, "cooking oil": true, "sunflower oil": true,
	"water": true, "cold water": true, "warm water": true, "hot water": true, "boiling water": true,
}

// cookable reports whether nothing but staples is missing.
func (a apiRecipe) cookable() bool {
	for _, m := range a.MissedIngredients {
		if !staples[strings.ToLower(strings.TrimSpace(m.Name))] {
			return false
		}
	}
	return true
}

func (a apiRecipe) recipe() Recipe {
	r := Recipe{Summary: Summary{ID: a.ID, Title: a.Title, Minutes: a.ReadyInMinutes, URL: a.SourceURL}}
	for _, i := range a.ExtendedIngredients {
		r.Ingredients = append(r.Ingredients, strings.TrimSpace(i.Original))
	}
	for _, group := range a.AnalyzedInstructions {
		for _, s := range group.Steps {
			r.Steps = append(r.Steps, strings.TrimSpace(s.Step))
		}
	}
	return r
}
