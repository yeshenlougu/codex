package provider

import (
	"encoding/json"
	"strings"
)

// ModelType represents a model capability category.
type ModelType string

const (
	ModelChat      ModelType = "chat"
	ModelVision    ModelType = "vision"
	ModelImageGen  ModelType = "image_gen"
	ModelVideoGen  ModelType = "video_gen"
	ModelAudioSTT  ModelType = "audio_stt"
	ModelAudioTTS  ModelType = "audio_tts"
	ModelEmbedding ModelType = "embedding"
)

// AllModelTypes lists all known capability types.
var AllModelTypes = []ModelType{
	ModelChat, ModelVision, ModelImageGen,
	ModelVideoGen, ModelAudioSTT, ModelAudioTTS, ModelEmbedding,
}

// ModelTypeMeta holds display info for each type.
type ModelTypeMeta struct {
	Label string `json:"label"`
	Icon  string `json:"icon"`
	Desc  string `json:"desc"`
}

// ModelTypeMetaMap maps types to display metadata.
var ModelTypeMetaMap = map[ModelType]ModelTypeMeta{
	ModelChat:      {Label: "Chat", Icon: "💬", Desc: "LLM conversation"},
	ModelVision:    {Label: "Vision", Icon: "👁️", Desc: "Image understanding"},
	ModelImageGen:  {Label: "Image Gen", Icon: "🖼️", Desc: "Image generation"},
	ModelVideoGen:  {Label: "Video Gen", Icon: "🎥", Desc: "Video generation"},
	ModelAudioSTT:  {Label: "Speech STT", Icon: "🎤", Desc: "Speech-to-text"},
	ModelAudioTTS:  {Label: "Speech TTS", Icon: "🔊", Desc: "Text-to-speech"},
	ModelEmbedding: {Label: "Embedding", Icon: "📊", Desc: "Text embeddings"},
}

// ModelInfo holds discovered model metadata.
type ModelInfo struct {
	Name string    `json:"name"`
	Type ModelType `json:"type"`
	Auto bool      `json:"auto"` // true if auto-detected, false if manual override
}

// DetectModelType classifies a model by its name using known patterns.
// Returns (type, true) when auto-detected.
func DetectModelType(name string) (ModelType, bool) {
	lower := strings.ToLower(name)

	// 1. Multimodal vision-capable models (more specific match first)
	for _, p := range []string{
		"gpt-4o", "gpt-4.1", "gpt-5",
		"claude-3.5", "claude-3-opus", "claude-4",
		"gemini-2", "gemini-1.5",
		"qwen-vl", "qwen2.5-vl",
		"glm-4v",
	} {
		if strings.Contains(lower, p) {
			return ModelVision, true
		}
	}

	// 2. Image generation
	for _, p := range []string{"gpt-image", "dall-e"} {
		if strings.Contains(lower, p) {
			return ModelImageGen, true
		}
	}

	// 3. Speech-to-text
	if strings.Contains(lower, "whisper") {
		return ModelAudioSTT, true
	}

	// 4. Text-to-speech
	if strings.HasPrefix(lower, "tts-") || strings.HasPrefix(lower, "tts") {
		return ModelAudioTTS, true
	}

	// 5. Embedding
	if strings.Contains(lower, "text-embedding") || strings.Contains(lower, "embedding") {
		return ModelEmbedding, true
	}

	// 6. Video generation
	for _, p := range []string{"sora", "kling", "runway", "video-gen"} {
		if strings.Contains(lower, p) {
			return ModelVideoGen, true
		}
	}

	// Default: all LLMs support chat
	return ModelChat, true
}

// ParseModelsResponse extracts model names from a /models API JSON response.
// Supports both OpenAI format {"data":[{"id":"gpt-5.5"},...]} and
// simple array format ["gpt-5.5","gpt-image-2",...].
func ParseModelsResponse(body []byte) ([]string, error) {
	type modelItem struct {
		ID      string `json:"id"`
		OwnedBy string `json:"owned_by"`
	}
	type listResp struct {
		Data []modelItem `json:"data"`
	}
	type altResp struct {
		Models []modelItem `json:"models"`
	}

	// Try standard OpenAI format: {"data": [...]}
	var resp listResp
	if err := json.Unmarshal(body, &resp); err == nil && len(resp.Data) > 0 {
		names := make([]string, len(resp.Data))
		for i, m := range resp.Data {
			names[i] = m.ID
		}
		return names, nil
	}

	// Try alternative format: {"models": [...]}
	var alt altResp
	if err := json.Unmarshal(body, &alt); err == nil && len(alt.Models) > 0 {
		names := make([]string, len(alt.Models))
		for i, m := range alt.Models {
			names[i] = m.ID
		}
		return names, nil
	}

	// Try simple string array: ["model1", "model2", ...]
	var simple []string
	if err := json.Unmarshal(body, &simple); err == nil && len(simple) > 0 {
		return simple, nil
	}

	return nil, nil // empty or unrecognized format
}

// MergeModels combines auto-discovered models with manual overrides.
// Manual models (Auto=false) take precedence over auto-detected (Auto=true).
// Returns a deduplicated list sorted: manual first, then auto (without duplicates).
func MergeModels(autoModels, manualModels []ModelInfo) []ModelInfo {
	seen := make(map[string]bool)
	var result []ModelInfo

	// Manual overrides first
	for _, m := range manualModels {
		if !seen[m.Name] {
			seen[m.Name] = true
			m.Auto = false
			result = append(result, m)
		}
	}

	// Auto-detected (skip if already in manual)
	for _, m := range autoModels {
		if !seen[m.Name] {
			seen[m.Name] = true
			m.Auto = true
			result = append(result, m)
		}
	}

	return result
}

// ModelsByType groups a model list by type.
func ModelsByType(models []ModelInfo) map[ModelType][]ModelInfo {
	grouped := make(map[ModelType][]ModelInfo)
	for _, m := range models {
		grouped[m.Type] = append(grouped[m.Type], m)
	}
	return grouped
}

// HasModelType checks if any model in the list has the given type.
func HasModelType(models []ModelInfo, t ModelType) bool {
	for _, m := range models {
		if m.Type == t {
			return true
		}
	}
	return false
}
