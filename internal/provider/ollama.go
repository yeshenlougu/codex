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

// OllamaClient talks to a local Ollama instance.
type OllamaClient struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewOllamaClient creates an Ollama client.
func NewOllamaClient(baseURL, model string) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		model:      model,
		httpClient: &http.Client{},
	}
}

// Stream sends a streaming chat request to Ollama.
func (c *OllamaClient) Stream(messages []Message, tools []ToolDef,
	onDelta func(text string), onToolCalls func(calls []ToolCall)) error {

	// Convert messages to Ollama format
	var ollamaMsgs []map[string]string
	for _, m := range messages {
		ollamaMsgs = append(ollamaMsgs, map[string]string{
			"role":    m.Role,
			"content": m.Content,
		})
	}

	// Convert tools to Ollama format
	var ollamaTools []map[string]any
	for _, t := range tools {
		ollamaTools = append(ollamaTools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Function.Name,
				"description": t.Function.Description,
				"parameters":  t.Function.Parameters,
			},
		})
	}

	reqBody := map[string]any{
		"model":    c.model,
		"messages": ollamaMsgs,
		"stream":   true,
	}
	if len(ollamaTools) > 0 {
		reqBody["tools"] = ollamaTools
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/api/chat", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("Ollama not running? %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Ollama error %d: %s", resp.StatusCode, string(errBody))
	}

	return c.parseStream(resp.Body, onDelta, onToolCalls)
}

func (c *OllamaClient) parseStream(r io.Reader, onDelta func(string), onToolCalls func(calls []ToolCall)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var fullContent strings.Builder
	var toolCallsAcc map[int]*ToolCall
	hasTools := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var chunk struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					Function struct {
						Name      string `json:"name"`
						Arguments map[string]any `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			Done bool `json:"done"`
		}
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}

		if chunk.Message.Content != "" {
			onDelta(chunk.Message.Content)
			fullContent.WriteString(chunk.Message.Content)
		}

		if len(chunk.Message.ToolCalls) > 0 {
			hasTools = true
			if toolCallsAcc == nil {
				toolCallsAcc = make(map[int]*ToolCall)
			}
			for i, tc := range chunk.Message.ToolCalls {
				argsBytes, _ := json.Marshal(tc.Function.Arguments)
				toolCallsAcc[i] = &ToolCall{
					ID:   fmt.Sprintf("call_%d", i),
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      tc.Function.Name,
						Arguments: string(argsBytes),
					},
				}
			}
		}
	}

	if hasTools && onToolCalls != nil && toolCallsAcc != nil {
		var result []ToolCall
		for i := 0; i < len(toolCallsAcc); i++ {
			if tc, ok := toolCallsAcc[i]; ok {
				result = append(result, *tc)
			}
		}
		if len(result) > 0 {
			onToolCalls(result)
		}
	}

	return scanner.Err()
}
