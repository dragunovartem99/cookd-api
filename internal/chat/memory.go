package chat

import (
	"fmt"
	"strings"

	"github.com/dragunovartem99/cookd-api/internal/store"
)

// Memory is what the model should know about the cook's kitchen and past
// dishes. It is read fresh for every request and shown to the model with the
// latest message only — never stored in the conversation — so the model sees
// the current pantry once, not every stale version of it.
type Memory struct {
	// Today is the current date, YYYY-MM-DD, so "last week" means something.
	Today       string
	Ingredients []store.Ingredient
	// Journal is the recent past, newest first.
	Journal []store.JournalEntry
}

// Block renders the memory as the text the model reads; empty when there is
// nothing to say.
func (m Memory) Block() string {
	var have, out []string
	for _, i := range m.Ingredients {
		if i.InStock {
			have = append(have, clean(i.Name))
		} else {
			out = append(out, clean(i.Name))
		}
	}

	var b strings.Builder
	if len(have) > 0 || len(out) > 0 {
		b.WriteString("<pantry>\n")
		fmt.Fprintf(&b, "Available: %s\n", listOrNone(have))
		if len(out) > 0 {
			fmt.Fprintf(&b, "Out of stock: %s\n", strings.Join(out, ", "))
		}
		b.WriteString("</pantry>\n")
	}
	if len(m.Journal) > 0 {
		fmt.Fprintf(&b, "<journal>\nDishes cooked before, newest first (today is %s; taste is out of 5):\n", m.Today)
		for _, e := range m.Journal {
			fmt.Fprintf(&b, "- %s %s: taste %d/5", e.CookedOn, clean(e.Dish), e.Taste)
			if e.Minutes != nil {
				fmt.Fprintf(&b, ", took %d min", *e.Minutes)
			}
			if note := clean(e.Note); note != "" {
				fmt.Fprintf(&b, ". %s", note)
			}
			b.WriteByte('\n')
		}
		b.WriteString("</journal>\n")
	}
	return strings.TrimSpace(b.String())
}

func listOrNone(items []string) string {
	if len(items) == 0 {
		return "nothing"
	}
	return strings.Join(items, ", ")
}

// clean keeps the cook's own words from closing the tags around them or
// breaking the line layout.
func clean(s string) string {
	s = strings.NewReplacer("<", "", ">", "", "\n", " ", "\r", " ").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}
