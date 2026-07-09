// Package ccswitch provides import/export of cc-switch configuration files.
// This enables one-click migration from an external cc-switch proxy to
// Codex's built-in multi-endpoint pool.
package ccswitch

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/yeshenlougu/codex/internal/config"
)

// CCConfig represents a cc-switch configuration file (YAML or JSON).
// Supports common cc-switch formats with flexible field name matching.
type CCConfig struct {
	// Common field names across cc-switch variants
	Endpoints []CCEndpoint `yaml:"endpoints" json:"endpoints"`
	Providers []CCEndpoint `yaml:"providers" json:"providers"`
	Backends  []CCEndpoint `yaml:"backends" json:"backends"`
	Strategy  string       `yaml:"strategy" json:"strategy"`
}

// CCEndpoint is one backend entry in cc-switch format.
type CCEndpoint struct {
	Name    string   `yaml:"name" json:"name"`         // or "label"
	Label   string   `yaml:"label" json:"label"`       // alternative to name
	URL     string   `yaml:"url" json:"url"`           // or "base_url"
	BaseURL string   `yaml:"base_url" json:"base_url"` // alternative to url
	Key     string   `yaml:"key" json:"key"`           // or "api_key"
	APIKey  string   `yaml:"api_key" json:"api_key"`   // alternative to key
	Weight  int      `yaml:"weight" json:"weight"`
	Models  []string `yaml:"models" json:"models"` // optional: allowed models
}

// endpointName returns the human-readable name for an endpoint.
func (e CCEndpoint) endpointName() string {
	if e.Name != "" {
		return e.Name
	}
	return e.Label
}

// endpointURL returns the base URL for an endpoint.
func (e CCEndpoint) endpointURL() string {
	if e.URL != "" {
		return e.URL
	}
	return e.BaseURL
}

// endpointKey returns the API key for an endpoint.
func (e CCEndpoint) endpointKey() string {
	if e.Key != "" {
		return e.Key
	}
	return e.APIKey
}

// allEndpoints merges all endpoint lists into a single slice.
func (c *CCConfig) allEndpoints() []CCEndpoint {
	var all []CCEndpoint
	all = append(all, c.Endpoints...)
	all = append(all, c.Providers...)
	all = append(all, c.Backends...)
	return all
}

// ImportFile reads a cc-switch config file and returns Codex BackendConfigs.
// Supports both YAML and JSON formats.
func ImportFile(path string) ([]config.BackendConfig, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read cc-switch config: %w", err)
	}

	var cc CCConfig
	// Try YAML first, then JSON
	if err := yaml.Unmarshal(data, &cc); err != nil {
		if err2 := json.Unmarshal(data, &cc); err2 != nil {
			return nil, "", fmt.Errorf("parse cc-switch config (tried YAML and JSON): %w", err)
		}
	}

	endpoints := cc.allEndpoints()
	if len(endpoints) == 0 {
		return nil, "", fmt.Errorf("no endpoints found in cc-switch config (checked: endpoints, providers, backends)")
	}

	strategy := cc.Strategy
	if strategy == "" {
		strategy = "round_robin"
	}
	// Normalize strategy names
	switch strings.ToLower(strategy) {
	case "roundrobin", "round_robin", "rr":
		strategy = "round_robin"
	case "random", "rand":
		strategy = "random"
	case "fillfirst", "fill_first", "first":
		strategy = "fill_first"
	}

	backends := make([]config.BackendConfig, 0, len(endpoints))
	for _, ep := range endpoints {
		name := ep.endpointName()
		if name == "" {
			name = fmt.Sprintf("backend-%d", len(backends)+1)
		}
		url := ep.endpointURL()
		key := ep.endpointKey()
		weight := ep.Weight
		if weight <= 0 {
			weight = 1
		}

		backends = append(backends, config.BackendConfig{
			Label:   name,
			Key:     key,
			BaseURL: url,
			Weight:  weight,
		})
	}

	return backends, strategy, nil
}

// ExportToCCSwitch converts Codex backends to cc-switch format and writes to a file.
func ExportToCCSwitch(backends []config.BackendConfig, strategy string, path string) error {
	endpoints := make([]CCEndpoint, 0, len(backends))
	for _, be := range backends {
		w := be.Weight
		if w <= 0 {
			w = 1
		}
		endpoints = append(endpoints, CCEndpoint{
			Name:    be.Label,
			BaseURL: be.BaseURL,
			Key:     be.Key,
			Weight:  w,
		})
	}

	cc := CCConfig{
		Endpoints: endpoints,
		Strategy:  strategy,
	}

	var data []byte
	var err error

	if strings.HasSuffix(path, ".json") {
		data, err = json.MarshalIndent(cc, "", "  ")
	} else {
		data, err = yaml.Marshal(cc)
	}
	if err != nil {
		return fmt.Errorf("marshal cc-switch config: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// MergeIntoConfig takes imported backends and updates a Codex config.
func MergeIntoConfig(cfg *config.Config, backends []config.BackendConfig, strategy string) {
	cfg.Provider.Backends = backends
	if strategy != "" {
		cfg.Provider.PoolStrategy = strategy
	}
	// Clear legacy fields since we're now using backends
	if len(backends) > 0 {
		cfg.Provider.APIKey = ""
		cfg.Provider.ExtraKeys = nil
	}
}
