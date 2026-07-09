package provider

import (
	"testing"
	"time"
)

func TestCapabilities(t *testing.T) {
	p := NewPool("round_robin")
	p.Add("k1", "backend-a", "https://a.example.com/v1", 1, []ModelInfo{
		{Name: "gpt-4", Type: ModelChat, Auto: true},
		{Name: "gpt-4o", Type: ModelVision, Auto: true},
	})
	p.Add("k2", "backend-b", "https://b.example.com/v1", 1, []ModelInfo{
		{Name: "dall-e-3", Type: ModelImageGen, Auto: true},
	})

	caps := p.Capabilities()

	if !caps[ModelChat] {
		t.Error("expected chat capability")
	}
	if !caps[ModelVision] {
		t.Error("expected vision capability")
	}
	if !caps[ModelImageGen] {
		t.Error("expected image_gen capability")
	}
	if caps[ModelAudioSTT] {
		t.Error("expected NO audio_stt capability")
	}
	if caps[ModelAudioTTS] {
		t.Error("expected NO audio_tts capability")
	}

	// All 7 types should be in the map
	if len(caps) != 7 {
		t.Errorf("expected 7 capability entries, got %d", len(caps))
	}
}

func TestCapabilities_Empty(t *testing.T) {
	p := NewPool("round_robin")
	caps := p.Capabilities()
	if len(caps) != 7 {
		t.Errorf("expected 7 entries even with empty pool, got %d", len(caps))
	}
	for _, mt := range AllModelTypes {
		if caps[mt] {
			t.Errorf("expected %q to be false in empty pool", mt)
		}
	}
}

func TestSelectFor_ExactMatch(t *testing.T) {
	p := NewPool("round_robin")
	p.Add("k1", "chat-backend", "https://chat.example.com/v1", 1, []ModelInfo{
		{Name: "gpt-4", Type: ModelChat, Auto: true},
	})
	p.Add("k2", "image-backend", "https://img.example.com/v1", 1, []ModelInfo{
		{Name: "dall-e-3", Type: ModelImageGen, Auto: true},
	})

	entry, model, ok := p.SelectFor(ModelImageGen)
	if !ok {
		t.Fatal("expected to find image_gen backend")
	}
	if entry.Label != "image-backend" {
		t.Errorf("expected image-backend, got %q", entry.Label)
	}
	if model.Name != "dall-e-3" {
		t.Errorf("expected dall-e-3, got %q", model.Name)
	}
}

func TestSelectFor_Fallback(t *testing.T) {
	p := NewPool("round_robin")
	p.Add("k1", "only-chat", "https://chat.example.com/v1", 1, []ModelInfo{
		{Name: "gpt-4", Type: ModelChat, Auto: true},
	})

	// Ask for image_gen but only chat is available → should fallback to first available
	entry, model, ok := p.SelectFor(ModelImageGen)
	if !ok {
		t.Fatal("expected fallback to work")
	}
	if entry.Label != "only-chat" {
		t.Errorf("expected fallback to only-chat, got %q", entry.Label)
	}
	if model.Name != "gpt-4" {
		t.Errorf("expected gpt-4 as fallback model, got %q", model.Name)
	}
}

func TestSelectFor_MultiType(t *testing.T) {
	p := NewPool("round_robin")
	p.Add("k1", "chat-only", "https://chat.example.com/v1", 1, []ModelInfo{
		{Name: "gpt-4", Type: ModelChat, Auto: true},
	})
	p.Add("k2", "vision", "https://vision.example.com/v1", 1, []ModelInfo{
		{Name: "gpt-4o", Type: ModelVision, Auto: true},
	})

	// Request vision or image_gen → should pick vision backend
	entry, _, ok := p.SelectFor(ModelVision, ModelImageGen)
	if !ok {
		t.Fatal("expected to find vision backend")
	}
	if entry.Label != "vision" {
		t.Errorf("expected vision backend, got %q", entry.Label)
	}
}

func TestSelectFor_NoBackends(t *testing.T) {
	p := NewPool("round_robin")
	_, _, ok := p.SelectFor(ModelChat)
	if ok {
		t.Error("expected no match with empty pool")
	}
}

func TestHasCapability(t *testing.T) {
	p := NewPool("round_robin")
	p.Add("k1", "be", "https://example.com/v1", 1, []ModelInfo{
		{Name: "gpt-4", Type: ModelChat, Auto: true},
	})

	if !p.HasCapability(ModelChat) {
		t.Error("expected HasCapability(chat) = true")
	}
	if p.HasCapability(ModelImageGen) {
		t.Error("expected HasCapability(image_gen) = false")
	}
}

func TestModelsByCapability(t *testing.T) {
	p := NewPool("round_robin")
	p.Add("k1", "chat-be", "https://chat.example.com/v1", 1, []ModelInfo{
		{Name: "gpt-4", Type: ModelChat, Auto: true},
	})
	p.Add("k2", "multi-be", "https://multi.example.com/v1", 1, []ModelInfo{
		{Name: "gpt-4o", Type: ModelVision, Auto: true},
		{Name: "dall-e-3", Type: ModelImageGen, Auto: true},
	})

	result := p.ModelsByCapability()

	// All 7 types should have entries, even if empty
	if len(result) != 7 {
		t.Errorf("expected 7 capability groups, got %d", len(result))
	}

	// Chat should have 1 backend
	if len(result[ModelChat]) != 1 {
		t.Errorf("expected 1 chat backend, got %d", len(result[ModelChat]))
	}

	// Vision should have 1 backend (multi-be)
	if len(result[ModelVision]) != 1 {
		t.Errorf("expected 1 vision backend, got %d", len(result[ModelVision]))
	}

	// ImageGen should have 1 backend (multi-be)
	if len(result[ModelImageGen]) != 1 {
		t.Errorf("expected 1 image_gen backend, got %d", len(result[ModelImageGen]))
	}

	// Audio STT should have 0
	if len(result[ModelAudioSTT]) != 0 {
		t.Errorf("expected 0 audio_stt backends, got %d", len(result[ModelAudioSTT]))
	}
}

func TestPoolStatus(t *testing.T) {
	p := NewPool("round_robin")
	p.Add("k1", "backend-a", "https://a.example.com/v1", 1, []ModelInfo{
		{Name: "gpt-4", Type: ModelChat, Auto: true},
	})

	statuses := p.Status()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status entry, got %d", len(statuses))
	}
	if statuses[0].Label != "backend-a" {
		t.Errorf("expected label backend-a, got %q", statuses[0].Label)
	}
	if statuses[0].BaseURL != "https://a.example.com/v1" {
		t.Errorf("wrong base_url: %q", statuses[0].BaseURL)
	}
	if statuses[0].Weight != 1 {
		t.Errorf("expected weight 1, got %d", statuses[0].Weight)
	}
	if len(statuses[0].Models) != 1 {
		t.Errorf("expected 1 model, got %d", len(statuses[0].Models))
	}
}

func TestPoolEntryStatus_Snapshot(t *testing.T) {
	e := &PoolEntry{
		Key:     "sk-test",
		Label:   "test-entry",
		BaseURL: "https://test.example.com/v1",
		Weight:  2,
		Health:  HealthHealthy,
		Models: []ModelInfo{
			{Name: "gpt-4", Type: ModelChat, Auto: true},
		},
	}

	s := e.Status()
	if s.Label != "test-entry" {
		t.Errorf("wrong label: %q", s.Label)
	}
	if s.Health != "healthy" {
		t.Errorf("wrong health: %q", s.Health)
	}
	if s.Weight != 2 {
		t.Errorf("wrong weight: %d", s.Weight)
	}
}

func TestPoolEntry_IsAvailable(t *testing.T) {
	e := &PoolEntry{
		Key:      "k",
		Label:    "be",
		BaseURL:  "https://example.com/v1",
		Weight:   1,
		Health:   HealthHealthy,
		Cooldown: 30 * time.Second,
	}

	if !e.IsAvailable() {
		t.Error("healthy entry should be available")
	}

	e.Health = HealthUnhealthy
	e.LastFail = time.Now() // cooldown not expired yet
	if e.IsAvailable() {
		t.Error("unhealthy entry with recent LastFail should NOT be available")
	}

	e.Weight = 0
	e.Health = HealthHealthy
	if e.IsAvailable() {
		t.Error("zero-weight entry should NOT be available")
	}

	e.Weight = 1
	e.Health = HealthUnknown
	// Unknown + 0 failures = available (will be probed)
	if !e.IsAvailable() {
		t.Error("unknown with 0 failures should be available")
	}
}

func TestPool_Add(t *testing.T) {
	p := NewPool("fill_first")
	p.Add("k1", "be1", "https://1.example.com", 1, nil)

	if p.Len() != 1 {
		t.Errorf("expected len 1, got %d", p.Len())
	}

	p.Add("k2", "be2", "https://2.example.com", 5, nil)
	if p.Len() != 2 {
		t.Errorf("expected len 2, got %d", p.Len())
	}
	if p.Available() != 2 {
		t.Errorf("expected 2 available, got %d", p.Available())
	}
}

func TestPool_Select(t *testing.T) {
	p := NewPool("fill_first")
	p.Add("k1", "first", "https://1.example.com", 1, nil)
	p.Add("k2", "second", "https://2.example.com", 1, nil)

	// fill_first always returns first
	e, ok := p.Select()
	if !ok {
		t.Fatal("expected selection to succeed")
	}
	if e.Label != "first" {
		t.Errorf("fill_first should pick first, got %q", e.Label)
	}
}

func TestPool_Select_Empty(t *testing.T) {
	p := NewPool("round_robin")
	_, ok := p.Select()
	if ok {
		t.Error("select on empty pool should fail")
	}
}
