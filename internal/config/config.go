package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/jrswab/axe/internal/xdg"
)

// ProviderConfig holds per-provider settings from config.toml.
type ProviderConfig struct {
	APIKey  string `toml:"api_key"`
	BaseURL string `toml:"base_url"`
}

// GlobalConfig represents the parsed global config file.
type GlobalConfig struct {
	Providers map[string]ProviderConfig `toml:"providers"`
}

// Load reads and parses the global config file at $XDG_CONFIG_HOME/axe/config.toml.
// If the file does not exist, it returns a valid GlobalConfig with an empty Providers map.
func Load() (*GlobalConfig, error) {
	configDir, err := xdg.GetConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	path := filepath.Join(configDir, "config.toml")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &GlobalConfig{Providers: map[string]ProviderConfig{}}, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg GlobalConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if cfg.Providers == nil {
		cfg.Providers = map[string]ProviderConfig{}
	}

	return &cfg, nil
}

// canonicalAPIKeyEnvVar returns the environment variable name for a provider's API key.
var knownAPIKeyEnvVars = map[string]string{
	"anthropic": "ANTHROPIC_API_KEY",
	"openai":    "OPENAI_API_KEY",
}

// ResolveAPIKey returns the API key for the given provider.
// Resolution order: env var > config file > empty string.
func (c *GlobalConfig) ResolveAPIKey(providerName string) string {
	envVar, ok := knownAPIKeyEnvVars[providerName]
	if !ok {
		envVar = strings.ToUpper(providerName) + "_API_KEY"
	}

	if v := os.Getenv(envVar); v != "" {
		return v
	}

	if c.Providers != nil {
		if pc, exists := c.Providers[providerName]; exists {
			return pc.APIKey
		}
	}

	return ""
}

// ResolveBaseURL returns the base URL for the given provider.
// Resolution order: env var > config file > empty string.
func (c *GlobalConfig) ResolveBaseURL(providerName string) string {
	envVar := "AXE_" + strings.ToUpper(providerName) + "_BASE_URL"

	if v := os.Getenv(envVar); v != "" {
		return v
	}

	if c.Providers != nil {
		if pc, exists := c.Providers[providerName]; exists {
			return pc.BaseURL
		}
	}

	return ""
}
