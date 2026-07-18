package provider

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ProtocolAdapter converts between OpenAI Chat Completions and Responses API formats.
// Per SPEC §3.6, it supports bidirectional conversion and model alias mapping.
type ProtocolAdapter struct {
	// APIMode selects the output protocol: "chat_completions" or "responses"
	APIMode string
	// ModelAliases maps display names → actual API model names
	ModelAliases map[string]string
}

// NewProtocolAdapter creates an adapter with the given API mode.
func NewProtocolAdapter(apiMode string) *ProtocolAdapter {
	return &ProtocolAdapter{
		APIMode:      apiMode,
		ModelAliases: make(map[string]string),
	}
}

// ResolveModel returns the actual model name for the API, applying aliases.
func (pa *ProtocolAdapter) ResolveModel(model string) string {
	if alias, ok := pa.ModelAliases[model]; ok {
		return alias
	}
	return model
}

// ConvertChatToResponses transforms a Chat Completions request body
// into a Responses API request body.
//
// Key differences:
//   - Chat: {"model":"gpt-4o","messages":[...],"stream":true}
//   - Responses: {"model":"gpt-4o","input":[...messages...],"stream":true}
//   - Responses uses "input" instead of "messages"
//   - Responses requires "instructions" for system messages
func (pa *ProtocolAdapter) ConvertChatToResponses(chatReq map[string]any) (map[string]any, error) {
	resp := make(map[string]any)

	// Copy scalar fields
	for _, k := range []string{"model", "stream", "temperature", "max_tokens", "top_p"} {
		if v, ok := chatReq[k]; ok {
			resp[k] = v
		}
	}

	// Resolve model alias
	if model, ok := resp["model"].(string); ok {
		resp["model"] = pa.ResolveModel(model)
	}

	// Convert messages → input
	messages, ok := chatReq["messages"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("messages field is required")
	}

	var input []map[string]any
	var instructions string

	for _, msg := range messages {
		m, ok := msg.(map[string]any)
		if !ok {
			continue
		}
		role, _ := m["role"].(string)

		if role == "system" {
			// Responses uses "instructions" for system prompt
			if content, ok := m["content"].(string); ok {
				if instructions != "" {
					instructions += "\n"
				}
				instructions += content
			}
			continue
		}

		item := map[string]any{"role": role}
		if content, ok := m["content"].(string); ok {
			item["content"] = content
		}
		// Copy tool_calls if present
		if tc, ok := m["tool_calls"]; ok {
			item["tool_calls"] = tc
		}
		if tcid, ok := m["tool_call_id"]; ok {
			item["tool_call_id"] = tcid
		}
		input = append(input, item)
	}

	resp["input"] = input
	if instructions != "" {
		resp["instructions"] = instructions
	}

	// Convert tools if present
	if tools, ok := chatReq["tools"]; ok {
		resp["tools"] = tools
	}

	// Convert reasoning_effort
	if effort, ok := chatReq["reasoning_effort"]; ok {
		resp["reasoning"] = map[string]any{"effort": effort}
	}

	return resp, nil
}

// ConvertResponsesToChat transforms a Responses API response
// into a Chat Completions delta format (for SSE streaming).
func (pa *ProtocolAdapter) ConvertResponsesToChat(respBody map[string]any) (map[string]any, error) {
	chat := make(map[string]any)

	// Map output → choices
	output, ok := respBody["output"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("output field is required in Responses API")
	}

	var choices []map[string]any
	for _, item := range output {
		out, ok := item.(map[string]any)
		if !ok {
			continue
		}

		choice := map[string]any{"index": 0}

		// Extract content from output item
		if content, ok := out["content"]; ok {
			// Content can be a string or an array of content blocks
			switch c := content.(type) {
			case string:
				choice["delta"] = map[string]any{"content": c}
			case []interface{}:
				var textParts []string
				for _, block := range c {
					if b, ok := block.(map[string]any); ok {
						if t, ok := b["text"].(string); ok {
							textParts = append(textParts, t)
						}
					}
				}
				choice["delta"] = map[string]any{"content": strings.Join(textParts, "")}
			}
		}

		// Handle tool calls
		if tc, ok := out["tool_calls"]; ok {
			choice["delta"] = map[string]any{"tool_calls": tc}
		}

		// Handle status/finish_reason
		if status, ok := out["status"].(string); ok {
			choice["finish_reason"] = status
		}

		choices = append(choices, choice)
	}

	chat["choices"] = choices

	// Map usage
	if usage, ok := respBody["usage"]; ok {
		chat["usage"] = usage
	}

	return chat, nil
}

// NormalizeSSE converts an SSE event from one protocol to the internal format.
func (pa *ProtocolAdapter) NormalizeSSE(data []byte, sourceProtocol string) ([]byte, error) {
	if sourceProtocol == "chat_completions" || sourceProtocol == "" {
		return data, nil // already in chat format
	}

	// Parse Responses API SSE event
	var respEvent map[string]any
	if err := json.Unmarshal(data, &respEvent); err != nil {
		return data, nil // pass through on parse error
	}

	// Check if this is a Responses API event
	if _, ok := respEvent["type"]; !ok {
		return data, nil
	}

	eventType, _ := respEvent["type"].(string)

	switch eventType {
	case "response.output_text.delta":
		// Convert to Chat delta format
		delta, _ := respEvent["delta"].(string)
		chatEvent := map[string]any{
			"choices": []map[string]any{{
				"index": 0,
				"delta": map[string]any{"content": delta},
			}},
		}
		return json.Marshal(chatEvent)

	case "response.completed":
		// End of stream — done
		return []byte(`{"choices":[{"finish_reason":"stop"}]}`), nil

	case "response.output_item.done":
		// Item completed
		return nil, nil // skip

	case "error":
		return data, nil // pass through errors
	}

	return data, nil
}
