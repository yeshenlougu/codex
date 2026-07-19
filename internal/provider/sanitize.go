// Package provider — Media sanitization
// Strips unsupported media types from requests when the backend lacks capability.
package provider

import (
	"encoding/json"
	"log"
)

// SanitizeRequestForBackend checks the backend's capabilities and removes
// unsupported media content from the request body.
// Returns the (possibly modified) body and a flag indicating whether changes were made.
func SanitizeRequestForBackend(entry *PoolEntry, body []byte) ([]byte, bool) {
	if entry == nil {
		return body, false
	}

	hasVision := HasModelType(entry.Models, ModelVision)
	if hasVision {
		return body, false // no sanitization needed
	}

	// No vision capability — strip image_url from messages
	return stripImageContent(body)
}

// stripImageContent removes image_url entries from the messages array
// in a Chat Completions request body, keeping text content intact.
func stripImageContent(body []byte) ([]byte, bool) {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return body, false
	}

	msgs, ok := req["messages"].([]any)
	if !ok {
		return body, false
	}

	modified := false
	for i, m := range msgs {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}

		content := msg["content"]
		switch c := content.(type) {
		case []any:
			// Content array (multimodal): filter out image_url entries
			filtered := make([]any, 0, len(c))
			for _, part := range c {
				p, ok := part.(map[string]any)
				if !ok {
					filtered = append(filtered, part)
					continue
				}
				if _, hasImage := p["image_url"]; hasImage {
					modified = true
					continue
				}
				if _, hasImage := p["type"]; hasImage {
					if t, _ := p["type"].(string); t == "image_url" {
						modified = true
						continue
					}
				}
				filtered = append(filtered, part)
			}
			if modified {
				msg["content"] = filtered
			}
		case string:
			// Plain text — no images to strip
		}
		msgs[i] = msg
	}

	if !modified {
		return body, false
	}

	req["messages"] = msgs
	out, err := json.Marshal(req)
	if err != nil {
		return body, false
	}

	log.Printf("[sanitize] stripped image content from request (%d messages)", len(msgs))
	return out, true
}
