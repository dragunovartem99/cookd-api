package api

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

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
