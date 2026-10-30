package chat

import (
	"encoding/base64"
	"fmt"

	"github.com/dragunovartem99/cookd-api/internal/store"
)

type inputPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type inputMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

func buildInput(history []store.Message, memory Memory) []inputMessage {
	input := make([]inputMessage, 0, len(history))
	history = recent(history)
	for n, m := range history {
		parts := make([]inputPart, 0, len(m.Images)+2)
		if len(history)-n <= photoMessages {
			for _, img := range m.Images {
				parts = append(parts, inputPart{Type: "input_image", ImageURL: "data:" + img.MediaType + ";base64," + base64.StdEncoding.EncodeToString(img.Data)})
			}
		} else if len(m.Images) > 0 {
			parts = append(parts, inputPart{Type: "input_text", Text: fmt.Sprintf("[%d photo(s) sent here, no longer shown]", len(m.Images))})
		}
		if block := memory.Block(); block != "" && n == len(history)-1 {
			parts = append(parts, inputPart{Type: "input_text", Text: block})
		}
		if m.Text != "" {
			parts = append(parts, inputPart{Type: "input_text", Text: m.Text})
		}
		role := m.Role
		if role != "assistant" {
			role = "user"
		}
		input = append(input, inputMessage{Role: role, Content: parts})
	}
	return input
}
