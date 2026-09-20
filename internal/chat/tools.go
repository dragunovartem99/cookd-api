package chat

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/dragunovartem99/cookd-api/internal/recipes"
)

// Recipes finds real recipes; see package recipes.
type Recipes interface {
	Search(ctx context.Context, ingredients []string) ([]recipes.Summary, error)
	Get(ctx context.Context, id int) (recipes.Recipe, error)
}

var errUnknownTool = errors.New("unknown tool")

func tools() []anthropic.ToolUnionParam {
	return []anthropic.ToolUnionParam{
		{OfTool: &anthropic.ToolParam{
			Name:        "search_recipes",
			Description: anthropic.String("Find real published recipes that can be cooked from the given ingredients alone (salt, pepper, oil and water are assumed). Returns up to 5 dishes with id, title, minutes and source url."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{"ingredients": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Available pantry ingredients, in English",
				}},
				Required: []string{"ingredients"},
			},
		}},
		{OfTool: &anthropic.ToolParam{
			Name:        "get_recipe",
			Description: anthropic.String("Get the full recipe (ingredients and steps) for an id returned by search_recipes."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{"id": map[string]any{"type": "integer"}},
				Required:   []string{"id"},
			},
		}},
	}
}

// runTools answers every tool call in the message. A failed lookup is reported
// to the model as an error result so it can carry on without one.
func (c *Claude) runTools(ctx context.Context, message anthropic.Message) []anthropic.ContentBlockParamUnion {
	var results []anthropic.ContentBlockParamUnion
	for _, block := range message.Content {
		use, ok := block.AsAny().(anthropic.ToolUseBlock)
		if !ok {
			continue
		}
		out, err := c.runTool(ctx, use)
		if err != nil {
			slog.WarnContext(ctx, "recipe tool failed", slog.String("tool", use.Name), slog.Any("error", err))
			out = "recipe lookup is unavailable"
		}
		results = append(results, anthropic.NewToolResultBlock(use.ID, out, err != nil))
	}
	return results
}

func (c *Claude) runTool(ctx context.Context, use anthropic.ToolUseBlock) (string, error) {
	var value any
	switch use.Name {
	case "search_recipes":
		var in struct {
			Ingredients []string `json:"ingredients"`
		}
		if err := json.Unmarshal(use.Input, &in); err != nil {
			return "", err
		}
		found, err := c.recipes.Search(ctx, in.Ingredients)
		if err != nil {
			return "", err
		}
		if len(found) == 0 {
			return "No dish can be cooked from these ingredients alone.", nil
		}
		value = found
	case "get_recipe":
		var in struct {
			ID int `json:"id"`
		}
		if err := json.Unmarshal(use.Input, &in); err != nil {
			return "", err
		}
		recipe, err := c.recipes.Get(ctx, in.ID)
		if err != nil {
			return "", err
		}
		value = recipe
	default:
		return "", errUnknownTool
	}
	out, err := json.Marshal(value)
	return string(out), err
}
