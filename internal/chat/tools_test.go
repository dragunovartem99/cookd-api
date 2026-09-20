package chat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/dragunovartem99/cookd-api/internal/recipes"
)

type fakeRecipes struct {
	found []recipes.Summary
	err   error
	asked []string
}

func (f *fakeRecipes) Search(_ context.Context, ingredients []string) ([]recipes.Summary, error) {
	f.asked = ingredients
	return f.found, f.err
}

func (f *fakeRecipes) Get(_ context.Context, id int) (recipes.Recipe, error) {
	return recipes.Recipe{Summary: recipes.Summary{ID: id, Title: "Soup"}}, f.err
}

func toolCall(t *testing.T, name, input string) anthropic.Message {
	t.Helper()
	var m anthropic.Message
	raw := `{"content":[{"type":"tool_use","id":"tu_1","name":"` + name + `","input":` + input + `}]}`
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func result(t *testing.T, blocks []anthropic.ContentBlockParamUnion) (text string, isError bool) {
	t.Helper()
	if len(blocks) != 1 || blocks[0].OfToolResult == nil {
		t.Fatalf("blocks = %+v, want one tool result", blocks)
	}
	r := blocks[0].OfToolResult
	return r.Content[0].OfText.Text, r.IsError.Value
}

func TestRunToolsSearch(t *testing.T) {
	f := &fakeRecipes{found: []recipes.Summary{{ID: 7, Title: "Omelette", Minutes: 10}}}
	c := &Claude{recipes: f}

	text, isErr := result(t, c.runTools(context.Background(), toolCall(t, "search_recipes", `{"ingredients":["eggs","milk"]}`)))
	if isErr || !strings.Contains(text, `"title":"Omelette"`) || strings.Join(f.asked, ",") != "eggs,milk" {
		t.Errorf("text %q, error %v, asked %v", text, isErr, f.asked)
	}

	f.found = nil
	text, _ = result(t, c.runTools(context.Background(), toolCall(t, "search_recipes", `{"ingredients":["kiwi"]}`)))
	if !strings.Contains(text, "No dish") {
		t.Errorf("empty search said %q", text)
	}
}

func TestRunToolsGet(t *testing.T) {
	c := &Claude{recipes: &fakeRecipes{}}
	text, isErr := result(t, c.runTools(context.Background(), toolCall(t, "get_recipe", `{"id":5}`)))
	if isErr || !strings.Contains(text, `"id":5`) {
		t.Errorf("text %q, error %v", text, isErr)
	}
}

// A failed lookup must reach the model as an error result, without the cause.
func TestRunToolsFailure(t *testing.T) {
	c := &Claude{recipes: &fakeRecipes{err: errors.New("status 402 for key abc")}}
	for _, call := range []anthropic.Message{
		toolCall(t, "get_recipe", `{"id":5}`),
		toolCall(t, "search_recipes", `{"ingredients":["eggs"]}`),
		toolCall(t, "search_recipes", `"garbage"`),
		toolCall(t, "bake_cake", `{}`),
	} {
		text, isErr := result(t, c.runTools(context.Background(), call))
		if !isErr || strings.Contains(text, "abc") {
			t.Errorf("text %q, error %v", text, isErr)
		}
	}
}
