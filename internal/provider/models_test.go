package provider

import (
	"encoding/json"
	"testing"
)

func TestDetectModelType(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		wantType ModelType
	}{
		// Chat (default)
		{"chat_basic", "gpt-4", ModelChat},
		{"chat_gpt4o_mini", "gpt-4o-mini", ModelVision},   // contains gpt-4o → vision
		{"chat_gpt35", "gpt-3.5-turbo", ModelChat},
		{"chat_deepseek", "deepseek-v4-pro", ModelChat},
		{"chat_qwen", "qwen3.7-max", ModelChat},
		{"chat_kimi", "kimi-k2.7-code", ModelChat},
		{"chat_minimax", "minimax-m3", ModelChat},
		{"chat_glm", "glm-5.2", ModelChat},
		{"chat_mimo", "mimo-v2.5-pro", ModelChat},
		{"chat_unknown", "some-unknown-model-v42", ModelChat},

		// Vision
		{"vision_gpt4o", "gpt-4o", ModelVision},
		{"vision_gpt41", "gpt-4.1", ModelVision},
		{"vision_gpt5", "gpt-5", ModelVision},
		{"vision_claude35", "claude-3.5-sonnet", ModelVision},
		{"vision_claude4", "claude-4-opus", ModelVision},
		{"vision_gemini2", "gemini-2.0-flash", ModelVision},
		{"vision_gemini15", "gemini-1.5-pro", ModelVision},
		{"vision_qwen_vl", "qwen-vl-max", ModelVision},
		{"vision_qwen25_vl", "qwen2.5-vl-72b", ModelVision},
		{"vision_glm4v", "glm-4v", ModelVision},

		// Image generation
		{"image_gpt", "gpt-image-2-medium", ModelImageGen},
		{"image_gpt_variant", "gpt-image-1", ModelImageGen},
		{"image_dalle", "dall-e-3", ModelImageGen},
		{"image_dalle2", "dall-e-2", ModelImageGen},

		// Speech-to-text
		{"stt_whisper", "whisper-1", ModelAudioSTT},
		{"stt_whisper_large", "whisper-large-v3", ModelAudioSTT},

		// Text-to-speech
		{"tts_basic", "tts-1", ModelAudioTTS},
		{"tts_hd", "tts-1-hd", ModelAudioTTS},
		{"tts_openai", "tts-2", ModelAudioTTS},

		// Embedding
		{"embed_openai", "text-embedding-3-small", ModelEmbedding},
		{"embed_ada", "text-embedding-ada-002", ModelEmbedding},
		{"embed_generic", "embedding-v2", ModelEmbedding},

		// Video generation
		{"video_sora", "sora-turbo", ModelVideoGen},
		{"video_kling", "kling-v2", ModelVideoGen},
		{"video_runway", "runway-gen3", ModelVideoGen},
		{"video_generic", "video-gen-1", ModelVideoGen},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := DetectModelType(tt.model)
			if got != tt.wantType {
				t.Errorf("DetectModelType(%q) = %q, want %q", tt.model, got, tt.wantType)
			}
		})
	}
}

func TestDetectModelType_MultimodalBeforeImageGen(t *testing.T) {
	// "gpt-4o" contains "gpt-4o" → should match vision, not chat with default
	got, _ := DetectModelType("gpt-4o")
	if got != ModelVision {
		t.Errorf("gpt-4o should be vision, got %q", got)
	}
}

func TestParseModelsResponse_OpenAIFormat(t *testing.T) {
	body := []byte(`{
		"data": [
			{"id": "gpt-5.5", "owned_by": "openai"},
			{"id": "gpt-image-2-medium", "owned_by": "openai"},
			{"id": "whisper-1", "owned_by": "openai"}
		]
	}`)

	names, err := ParseModelsResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 3 {
		t.Fatalf("expected 3 models, got %d", len(names))
	}
	if names[0] != "gpt-5.5" || names[1] != "gpt-image-2-medium" || names[2] != "whisper-1" {
		t.Errorf("wrong model names: %v", names)
	}
}

func TestParseModelsResponse_AltFormat(t *testing.T) {
	body := []byte(`{
		"models": [
			{"id": "claude-4-opus"},
			{"id": "claude-3.5-sonnet"}
		]
	}`)

	names, err := ParseModelsResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 models, got %d", len(names))
	}
	if names[0] != "claude-4-opus" || names[1] != "claude-3.5-sonnet" {
		t.Errorf("wrong model names: %v", names)
	}
}

func TestParseModelsResponse_SimpleArray(t *testing.T) {
	body := []byte(`["gpt-4", "gpt-4o", "dall-e-3"]`)

	names, err := ParseModelsResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 3 {
		t.Fatalf("expected 3 models, got %d", len(names))
	}
}

func TestParseModelsResponse_Empty(t *testing.T) {
	names, err := ParseModelsResponse([]byte(`{"data":[]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0 models, got %d", len(names))
	}
}

func TestParseModelsResponse_InvalidJSON(t *testing.T) {
	names, err := ParseModelsResponse([]byte(`not json`))
	if err == nil && len(names) > 0 {
		t.Error("expected no models on invalid JSON")
	}
}

func TestMergeModels_ManualOverrideWins(t *testing.T) {
	auto := []ModelInfo{
		{Name: "gpt-4o", Type: ModelVision, Auto: true},
		{Name: "gpt-4", Type: ModelChat, Auto: true},
	}
	manual := []ModelInfo{
		{Name: "gpt-4o", Type: ModelChat, Auto: false}, // override vision → chat
	}

	result := MergeModels(auto, manual)
	if len(result) != 2 {
		t.Fatalf("expected 2 models, got %d", len(result))
	}

	// Find gpt-4o — should be manual (chat, not vision)
	for _, m := range result {
		if m.Name == "gpt-4o" {
			if m.Type != ModelChat {
				t.Errorf("manual override should win: got %q, want %q", m.Type, ModelChat)
			}
			if m.Auto {
				t.Error("manual model should have Auto=false")
			}
		}
	}
}

func TestMergeModels_ManualAddsNew(t *testing.T) {
	auto := []ModelInfo{
		{Name: "gpt-4", Type: ModelChat, Auto: true},
	}
	manual := []ModelInfo{
		{Name: "my-custom-model", Type: ModelImageGen, Auto: false},
	}

	result := MergeModels(auto, manual)
	if len(result) != 2 {
		t.Fatalf("expected 2 models, got %d", len(result))
	}

	hasCustom := false
	for _, m := range result {
		if m.Name == "my-custom-model" {
			hasCustom = true
			if m.Type != ModelImageGen || m.Auto {
				t.Error("manual model metadata wrong")
			}
		}
	}
	if !hasCustom {
		t.Error("custom model not found in result")
	}
}

func TestMergeModels_ManualFirst(t *testing.T) {
	auto := []ModelInfo{
		{Name: "a", Type: ModelChat, Auto: true},
	}
	manual := []ModelInfo{
		{Name: "b", Type: ModelVision, Auto: false},
	}

	result := MergeModels(auto, manual)
	// Manual should come first
	if result[0].Name != "b" {
		t.Errorf("manual model should come first, got %q", result[0].Name)
	}
}

func TestModelsByType(t *testing.T) {
	models := []ModelInfo{
		{Name: "gpt-4", Type: ModelChat},
		{Name: "gpt-4o", Type: ModelVision},
		{Name: "deepseek-v4", Type: ModelChat},
		{Name: "dall-e-3", Type: ModelImageGen},
	}

	grouped := ModelsByType(models)
	if len(grouped[ModelChat]) != 2 {
		t.Errorf("expected 2 chat models, got %d", len(grouped[ModelChat]))
	}
	if len(grouped[ModelVision]) != 1 {
		t.Errorf("expected 1 vision model, got %d", len(grouped[ModelVision]))
	}
	if len(grouped[ModelImageGen]) != 1 {
		t.Errorf("expected 1 image_gen model, got %d", len(grouped[ModelImageGen]))
	}
	if len(grouped[ModelAudioSTT]) != 0 {
		t.Errorf("expected 0 audio_stt models, got %d", len(grouped[ModelAudioSTT]))
	}
}

func TestHasModelType(t *testing.T) {
	models := []ModelInfo{
		{Name: "gpt-4", Type: ModelChat},
	}

	if !HasModelType(models, ModelChat) {
		t.Error("expected HasModelType = true for chat")
	}
	if HasModelType(models, ModelImageGen) {
		t.Error("expected HasModelType = false for image_gen")
	}
}

// Ensure json is used (it is by ParseModelsResponse)
var _ = json.Marshal
