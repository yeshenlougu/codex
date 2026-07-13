package tool

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// ImageGenTool generates images via the AI model's image generation capability.
type ImageGenTool struct {
	endpoint string // base URL for the image generation endpoint
	apiKey   string
	model    string
}

// NewImageGenTool creates an image generation tool.
func NewImageGenTool() *ImageGenTool {
	return &ImageGenTool{}
}

// SetBackend configures the backend for image generation calls.
func (t *ImageGenTool) SetBackend(baseURL, apiKey, model string) {
	t.endpoint = baseURL
	t.apiKey = apiKey
	t.model = model
}

func (t *ImageGenTool) Name() string {
	return "image_generate"
}

func (t *ImageGenTool) Description() string {
	return "Generate high-quality images from text prompts. Use this when the user asks to create, generate, or draw an image, picture, or illustration. Provide a detailed English prompt describing the desired image style, subject, composition, and mood."
}

func (t *ImageGenTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prompt": map[string]any{
				"type":        "string",
				"description": "Detailed English text prompt describing the image to generate",
			},
			"size": map[string]any{
				"type":        "string",
				"description": "Image size: 1024x1024, 1792x1024, or 1024x1792",
				"enum":        []string{"1024x1024", "1792x1024", "1024x1792"},
			},
			"n": map[string]any{
				"type":        "integer",
				"description": "Number of images to generate (1-4)",
				"minimum":     1,
				"maximum":     4,
			},
		},
		"required": []string{"prompt"},
	}
}

func (t *ImageGenTool) Execute(rawArgs string) (*Result, error) {
	var args struct {
		Prompt string `json:"prompt"`
		Size   string `json:"size"`
		N      int    `json:"n"`
	}
	if rawArgs != "" {
		if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
			return &Result{Success: false, Error: "invalid arguments: " + err.Error()}, nil
		}
	}
	if args.Prompt == "" {
		return &Result{Success: false, Error: "prompt is required"}, nil
	}
	if args.Size == "" {
		args.Size = "1024x1024"
	}
	if args.N < 1 {
		args.N = 1
	} else if args.N > 4 {
		args.N = 4
	}

	if t.endpoint == "" || t.apiKey == "" {
		return &Result{Success: false, Error: "no image generation backend configured. Add a backend with gpt-image-* or dall-e-* models."}, nil
	}

	// Build OpenAI-compatible request
	body := map[string]any{
		"prompt": args.Prompt,
		"size":   args.Size,
		"n":      args.N,
		"model":  t.model,
	}
	bodyBytes, _ := json.Marshal(body)

	url := strings.TrimSuffix(t.endpoint, "/") + "/v1/images/generations"
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return &Result{Success: false, Error: "create request: " + err.Error()}, nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &Result{Success: false, Error: "request failed: " + err.Error()}, nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Printf("[image_gen] API error %d: %s", resp.StatusCode, string(respBody))
		return &Result{Success: false, Error: fmt.Sprintf("image generation failed (HTTP %d): %s", resp.StatusCode, string(respBody[:300]))}, nil
	}

	var result struct {
		Data []struct {
			URL           string `json:"url"`
			B64JSON       string `json:"b64_json"`
			RevisedPrompt string `json:"revised_prompt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return &Result{Success: false, Error: "parse response: " + err.Error()}, nil
	}

	if len(result.Data) == 0 {
		return &Result{Success: false, Error: "no images returned"}, nil
	}

	// Format output as markdown images so chat UI can render them
	var output strings.Builder
	output.WriteString(fmt.Sprintf("✅ Generated %d image(s):\n\n", len(result.Data)))
	for i, d := range result.Data {
		if d.URL != "" {
			output.WriteString(fmt.Sprintf("![Image %d](%s)\n\n", i+1, d.URL))
		} else if d.B64JSON != "" {
			// Decode and save to memory, return as data URL
			decoded, err := base64.StdEncoding.DecodeString(d.B64JSON)
			if err == nil {
				dataURL := fmt.Sprintf("data:image/png;base64,%s", d.B64JSON)
				output.WriteString(fmt.Sprintf("![Image %d](%s)\n\n", i+1, dataURL))
				_ = decoded
			}
		}
		if d.RevisedPrompt != "" {
			output.WriteString(fmt.Sprintf("*Revised prompt: %s*\n\n", d.RevisedPrompt))
		}
	}

	return &Result{Success: true, Output: output.String()}, nil
}
