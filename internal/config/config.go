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
	APIKey     string `toml:"api_key"`
	BaseURL    string `toml:"base_url"`
	Region     string `toml:"region"`
	Referer    string `toml:"referer"`
	Title      string `toml:"title"`
	Categories string `toml:"categories"`
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

// knownAPIKeyEnvVars maps provider names to their canonical API key environment variables.
var knownAPIKeyEnvVars = map[string]string{
	"anthropic":  "ANTHROPIC_API_KEY",
	"openai":     "OPENAI_API_KEY",
	"opencode":   "OPENCODE_API_KEY",
	"google":     "GEMINI_API_KEY",
	"minimax":    "MINIMAX_API_KEY",
	"openrouter": "OPENROUTER_API_KEY",
}

// APIKeyEnvVar returns the environment variable name used to resolve the API key
// for the given provider. Known providers use canonical names; unknown providers
// use the convention <PROVIDER_UPPER>_API_KEY.
func APIKeyEnvVar(providerName string) string {
	if v, ok := knownAPIKeyEnvVars[providerName]; ok {
		return v
	}
	return strings.ToUpper(providerName) + "_API_KEY"
}

// ResolveAPIKey returns the API key for the given provider.
// Resolution order: env var > config file > empty string.
func (c *GlobalConfig) ResolveAPIKey(providerName string) string {
	envVar := APIKeyEnvVar(providerName)

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

// ResolveRegion returns the region for the given provider (primarily for AWS Bedrock).
// Resolution order: AWS_REGION > AWS_DEFAULT_REGION > config file > empty string.
func (c *GlobalConfig) ResolveRegion(providerName string) string {
	// Check standard AWS environment variables first
	if v := os.Getenv("AWS_REGION"); v != "" {
		return v
	}
	if v := os.Getenv("AWS_DEFAULT_REGION"); v != "" {
		return v
	}

	// Check provider-specific env var
	envVar := "AXE_" + strings.ToUpper(providerName) + "_REGION"
	if v := os.Getenv(envVar); v != "" {
		return v
	}

	// Check config file
	if c.Providers != nil {
		if pc, exists := c.Providers[providerName]; exists {
			return pc.Region
		}
	}

	return ""
}

// ResolveReferer returns the HTTP-Referer for the given provider.
// Resolution order: AXE_{PROVIDER_UPPER}_REFERER env var > config file > empty string.
func (c *GlobalConfig) ResolveReferer(providerName string) string {
	envVar := "AXE_" + strings.ToUpper(providerName) + "_REFERER"
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	if c.Providers != nil {
		if pc, exists := c.Providers[providerName]; exists {
			return pc.Referer
		}
	}
	return ""
}

// ResolveTitle returns the X-OpenRouter-Title for the given provider.
// Resolution order: AXE_{PROVIDER_UPPER}_TITLE env var > config file > empty string.
func (c *GlobalConfig) ResolveTitle(providerName string) string {
	envVar := "AXE_" + strings.ToUpper(providerName) + "_TITLE"
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	if c.Providers != nil {
		if pc, exists := c.Providers[providerName]; exists {
			return pc.Title
		}
	}
	return ""
}

// ResolveCategories returns the X-OpenRouter-Categories for the given provider.
// Resolution order: AXE_{PROVIDER_UPPER}_CATEGORIES env var > config file > empty string.
func (c *GlobalConfig) ResolveCategories(providerName string) string {
	envVar := "AXE_" + strings.ToUpper(providerName) + "_CATEGORIES"
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	if c.Providers != nil {
		if pc, exists := c.Providers[providerName]; exists {
			return pc.Categories
		}
	}
	return ""
}
