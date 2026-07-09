package provider

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// AnthropicClient talks to Anthropic's Messages API.
type AnthropicClient struct {
	baseURL    string
	secret     string
	model      string
	httpClient *http.Client
}

// NewAnthropicClient creates an Anthropic Messages API client.
func NewAnthropicClient(baseURL, secretVal, model string) *AnthropicClient {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	return &AnthropicClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		secret:     secretVal,
		model:      model,
		httpClient: &http.Client{},
	}
}

// anthropicContent represents a content block in Anthropic format.
type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// anthropicTool is an Anthropic-format tool definition.
type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// Stream sends a streaming request and calls onDelta for each text chunk.
func (c *AnthropicClient) Stream(systemPrompt string, messages []Message, tools []ToolDef,
	onDelta func(text string), onToolUse func(name, id, input string)) error {

	var anthropicMsgs []map[string]any
	for _, m := range messages {
		if m.Role == "system" {
			continue
		}
		if m.Role == "user" {
			anthropicMsgs = append(anthropicMsgs, map[string]any{
				"role":    "user",
				"content": []map[string]any{{"type": "text", "text": m.Content}},
			})
		} else if m.Role == "assistant" {
			blocks := []map[string]any{}
			if m.Content != "" {
				blocks = append(blocks, map[string]any{"type": "text", "text": m.Content})
			}
			for _, tc := range m.ToolCalls {
				var input map[string]any
				json.Unmarshal([]byte(tc.Function.Arguments), &input)
				blocks = append(blocks, map[string]any{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Function.Name,
					"input": input,
				})
			}
			anthropicMsgs = append(anthropicMsgs, map[string]any{"role": "assistant", "content": blocks})
		} else if m.Role == "tool" {
			anthropicMsgs = append(anthropicMsgs, map[string]any{
				"role":    "user",
				"content": []map[string]any{{"type": "tool_result", "tool_use_id": m.ToolCallID, "content": m.Content}},
			})
		}
	}

	var antTools []anthropicTool
	for _, t := range tools {
		antTools = append(antTools, anthropicTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		})
	}

	reqBody := map[string]any{
		"model":      c.model,
		"max_tokens": 8000,
		"messages":   anthropicMsgs,
		"stream":     true,
	}
	if systemPrompt != "" {
		reqBody["system"] = systemPrompt
	}
	if len(antTools) > 0 {
		reqBody["tools"] = antTools
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.secret)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(errBody))
	}

	return c.parseStream(resp.Body, onDelta, onToolUse)
}

func (c *AnthropicClient) parseStream(r io.Reader, onDelta func(string), onToolUse func(string, string, string)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var currentTool struct {
		id    string
		name  string
		input strings.Builder
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		var event struct {
			Type  string `json:"type"`
			Delta struct {
				Type       string `json:"type"`
				Text       string `json:"text"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
			ContentBlock struct {
				Type string `json:"type"`
				Id   string `json:"id"`
				Name string `json:"name"`
			} `json:"content_block"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "content_block_delta":
			if event.Delta.Type == "text_delta" {
				onDelta(event.Delta.Text)
			} else if event.Delta.Type == "input_json_delta" {
				currentTool.input.WriteString(event.Delta.PartialJSON)
			}
		case "content_block_start":
			if event.ContentBlock.Type == "tool_use" {
				currentTool.id = event.ContentBlock.Id
				currentTool.name = event.ContentBlock.Name
				currentTool.input.Reset()
			}
		case "content_block_stop":
			if currentTool.id != "" && onToolUse != nil {
				onToolUse(currentTool.name, currentTool.id, currentTool.input.String())
				currentTool.id = ""
			}
		}
	}
	return scanner.Err()
}
