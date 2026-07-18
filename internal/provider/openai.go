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

// Message represents a single chat message.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolDef defines a tool that the model can call.
type ToolDef struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

// FunctionDef defines a function tool's schema.
type FunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolCall represents a tool call requested by the model.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ChatRequest is the request body for chat completions.
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    []ToolDef `json:"tools,omitempty"`
	Stream   bool      `json:"stream"`
}

// ChatResponse is the non-streaming response.
type ChatResponse struct {
	Choices []struct {
		Message struct {
			Role      string     `json:"role"`
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// StreamDelta represents one chunk from a streaming response.
type StreamDelta struct {
	Choices []struct {
		Delta struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// Client is a simple OpenAI-compatible API client.
type Client struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client

	// UsageCallback is called after each successful API call with estimated token counts.
	UsageCallback func(inputTokens, outputTokens int, model string)

	// ProtocolAdapter for chat ↔ responses conversion
	Adapter *ProtocolAdapter
}

// NewClient creates a new LLM client.
func NewClient(baseURL, apiKey, model string) *Client {
	if apiKey == "" {
		// Return a client that will produce a clear error on first request
		apiKey = "MISSING_API_KEY"
	}
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		APIKey:     apiKey,
		Model:      model,
		HTTPClient: &http.Client{},
	}
}

// NewClientFromEntry creates a Client from a PoolEntry.
func NewClientFromEntry(entry *PoolEntry, model string) *Client {
	return NewClient(entry.BaseURL, entry.Key, model)
}

// Chat sends a chat completion request (non-streaming).
func (c *Client) Chat(messages []Message, tools []ToolDef) (*ChatResponse, error) {
	req := ChatRequest{
		Model:    c.Model,
		Messages: messages,
		Tools:    tools,
		Stream:   false,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(errBody))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &chatResp, nil
}

// ChatStream sends a streaming chat completion request.
// It calls onDelta for each content chunk and onToolCall for completed tool calls.
func (c *Client) ChatStream(messages []Message, tools []ToolDef, reasoningEffort string,
	onDelta func(content string), onToolCalls func(toolCalls []ToolCall)) error {

	reqBody := map[string]any{
		"model":    c.Model,
		"messages": messages,
		"stream":   true,
	}
	if len(tools) > 0 {
		reqBody["tools"] = tools
	}
	if reasoningEffort != "" {
		reqBody["reasoning_effort"] = reasoningEffort
	}

	// ── Protocol adaptation ──
	endpoint := "/chat/completions"
	if c.Adapter != nil && c.Adapter.APIMode == "responses" {
		var err error
		reqBody, err = c.Adapter.ConvertChatToResponses(reqBody)
		if err != nil {
			return fmt.Errorf("protocol adapter (chat→responses): %w", err)
		}
		endpoint = "/responses"
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.BaseURL+endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(errBody))
	}

	// Estimate input tokens (rough: ~4 chars per token for English text)
	inputTokens := estimateTokens(reqBody)
	_ = inputTokens

	err = c.parseStream(resp.Body, onDelta, onToolCalls)

	// Log usage after successful stream (rough estimate)
	if err == nil && c.UsageCallback != nil {
		// Estimate output from messages — rough: total content length / 4
		outputLen := 0
		for _, m := range messages {
			outputLen += len(m.Content)
		}
		// Subtract the pre-existing messages to get new output estimate
		// This is rough; real implementation would use token counts from API response
		c.UsageCallback(inputTokens, outputLen/4, c.Model)
	}

	return err
}

// estimateTokens provides a rough token count (4 chars ≈ 1 token).
func estimateTokens(reqBody map[string]any) int {
	raw, _ := json.Marshal(reqBody)
	return len(raw) / 4
}

func (c *Client) parseStream(r io.Reader, onDelta func(string), onToolCalls func([]ToolCall)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	// Accumulate tool calls across chunks
	toolCallsAcc := make(map[int]*ToolCall)
	hadToolCalls := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line == "data: [DONE]" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		var delta StreamDelta
		if err := json.Unmarshal([]byte(data), &delta); err != nil {
			continue // skip malformed chunks
		}

		for _, choice := range delta.Choices {
			// Handle content deltas
			if choice.Delta.Content != "" {
				onDelta(choice.Delta.Content)
			}

			// Handle tool call deltas
			for _, tc := range choice.Delta.ToolCalls {
				hadToolCalls = true
				existing, ok := toolCallsAcc[tc.Index]
				if !ok {
					existing = &ToolCall{ID: tc.ID, Type: tc.Type}
					toolCallsAcc[tc.Index] = existing
				}
				if tc.ID != "" {
					existing.ID = tc.ID
				}
				if tc.Function.Name != "" {
					existing.Function.Name = tc.Function.Name
				}
				existing.Function.Arguments += tc.Function.Arguments
			}
		}
	}

	// If we accumulated tool calls, emit them
	if hadToolCalls && onToolCalls != nil {
		result := make([]ToolCall, 0, len(toolCallsAcc))
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
