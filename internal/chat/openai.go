// Package chat turns a stored conversation into a streamed OpenAI reply.
package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/dragunovartem99/cookd-api/internal/store"
)

type Result struct {
	Text       string
	StopReason string
}

type OpenAI struct {
	apiKey string
	model  string
	client *http.Client
}

func NewOpenAI(apiKey, model string) *OpenAI {
	return &OpenAI{apiKey: apiKey, model: model, client: http.DefaultClient}
}

func (c *OpenAI) Stream(ctx context.Context, history []store.Message, memory Memory, onDelta func(string)) (Result, error) {
	input := buildInput(history, memory)
	body, err := json.Marshal(map[string]any{"model": c.model, "instructions": systemPrompt, "input": input, "max_output_tokens": 32000, "reasoning": map[string]string{"effort": "low"}, "stream": true, "store": false})
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := bufio.NewReader(resp.Body).ReadString('\n')
		return Result{}, fmt.Errorf("OpenAI API returned %s: %s", resp.Status, strings.TrimSpace(b))
	}
	var result Result
	refused := false
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 4096), 2*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var event struct {
			Type     string `json:"type"`
			Delta    string `json:"delta"`
			Response struct {
				Status            string `json:"status"`
				IncompleteDetails struct {
					Reason string `json:"reason"`
				} `json:"incomplete_details"`
			} `json:"response"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		switch event.Type {
		case "response.output_text.delta":
			result.Text += event.Delta
			onDelta(event.Delta)
		case "response.completed":
			result.StopReason = "end_turn"
			if event.Response.Status == "incomplete" {
				result.StopReason = event.Response.IncompleteDetails.Reason
			}
		case "response.refusal.delta":
			refused = true
		}
	}
	if err := scanner.Err(); err != nil {
		return Result{}, err
	}
	if refused {
		result.StopReason = "refusal"
	}
	return result, nil
}
