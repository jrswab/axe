package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_FileNotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Providers == nil {
		t.Fatal("expected non-nil Providers map")
	}
	if len(cfg.Providers) != 0 {
		t.Fatalf("expected empty Providers map, got %d entries", len(cfg.Providers))
	}
}

// writeConfigTOML is a test helper that creates the axe config directory and writes
// a config.toml with the given content. It calls t.Fatal on any filesystem error.
func writeConfigTOML(t *testing.T, tmpDir, content string) {
	t.Helper()
	axeDir := filepath.Join(tmpDir, "axe")
	if err := os.MkdirAll(axeDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(axeDir, "config.toml"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config.toml: %v", err)
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	writeConfigTOML(t, tmp, "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.Providers) != 0 {
		t.Fatalf("expected empty Providers map, got %d entries", len(cfg.Providers))
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	writeConfigTOML(t, tmp, `
[providers.anthropic]
api_key = "sk-ant-test"
base_url = "https://custom.anthropic.com"

[providers.openai]
api_key = "sk-openai-test"

[providers.ollama]
base_url = "http://myhost:11434"
`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.Providers) != 3 {
		t.Fatalf("expected 3 providers, got %d", len(cfg.Providers))
	}
	if cfg.Providers["anthropic"].APIKey != "sk-ant-test" {
		t.Errorf("expected anthropic api_key 'sk-ant-test', got %q", cfg.Providers["anthropic"].APIKey)
	}
	if cfg.Providers["anthropic"].BaseURL != "https://custom.anthropic.com" {
		t.Errorf("expected anthropic base_url, got %q", cfg.Providers["anthropic"].BaseURL)
	}
	if cfg.Providers["openai"].APIKey != "sk-openai-test" {
		t.Errorf("expected openai api_key, got %q", cfg.Providers["openai"].APIKey)
	}
	if cfg.Providers["ollama"].BaseURL != "http://myhost:11434" {
		t.Errorf("expected ollama base_url, got %q", cfg.Providers["ollama"].BaseURL)
	}
}

func TestLoad_MalformedTOML(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	writeConfigTOML(t, tmp, "[invalid toml\nblah blah")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for malformed TOML")
	}
	if got := err.Error(); !strings.Contains(got, "failed to parse config file") {
		t.Errorf("expected error to contain 'failed to parse config file', got %q", got)
	}
}

func TestLoad_PartialConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	writeConfigTOML(t, tmp, `
[providers.openai]
api_key = "sk-partial"
`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(cfg.Providers))
	}
	if _, ok := cfg.Providers["openai"]; !ok {
		t.Error("expected openai provider in map")
	}
	if _, ok := cfg.Providers["anthropic"]; ok {
		t.Error("did not expect anthropic provider in map")
	}
}

func TestResolveAPIKey_EnvVarTakesPrecedence(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "from-env")
	cfg := &GlobalConfig{
		Providers: map[string]ProviderConfig{
			"anthropic": {APIKey: "from-config"},
		},
	}
	if got := cfg.ResolveAPIKey("anthropic"); got != "from-env" {
		t.Errorf("expected 'from-env', got %q", got)
	}
}

func TestResolveAPIKey_FallsBackToConfig(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	cfg := &GlobalConfig{
		Providers: map[string]ProviderConfig{
			"anthropic": {APIKey: "from-config"},
		},
	}
	if got := cfg.ResolveAPIKey("anthropic"); got != "from-config" {
		t.Errorf("expected 'from-config', got %q", got)
	}
}

func TestResolveAPIKey_NeitherSet(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	cfg := &GlobalConfig{Providers: map[string]ProviderConfig{}}
	if got := cfg.ResolveAPIKey("anthropic"); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestResolveAPIKey_EmptyEnvVar(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	cfg := &GlobalConfig{
		Providers: map[string]ProviderConfig{
			"openai": {APIKey: "from-config"},
		},
	}
	if got := cfg.ResolveAPIKey("openai"); got != "from-config" {
		t.Errorf("expected 'from-config', got %q", got)
	}
}

func TestResolveAPIKey_NilProvidersMap(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "from-env")
	cfg := &GlobalConfig{Providers: nil}
	if got := cfg.ResolveAPIKey("anthropic"); got != "from-env" {
		t.Errorf("expected 'from-env', got %q", got)
	}
}

func TestResolveAPIKey_UnknownProvider(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "groq-key")
	cfg := &GlobalConfig{Providers: map[string]ProviderConfig{}}
	if got := cfg.ResolveAPIKey("groq"); got != "groq-key" {
		t.Errorf("expected 'groq-key', got %q", got)
	}
}

func TestResolveBaseURL_EnvVarTakesPrecedence(t *testing.T) {
	t.Setenv("AXE_OPENAI_BASE_URL", "http://from-env")
	cfg := &GlobalConfig{
		Providers: map[string]ProviderConfig{
			"openai": {BaseURL: "http://from-config"},
		},
	}
	if got := cfg.ResolveBaseURL("openai"); got != "http://from-env" {
		t.Errorf("expected 'http://from-env', got %q", got)
	}
}

func TestResolveBaseURL_FallsBackToConfig(t *testing.T) {
	t.Setenv("AXE_OPENAI_BASE_URL", "")
	cfg := &GlobalConfig{
		Providers: map[string]ProviderConfig{
			"openai": {BaseURL: "http://from-config"},
		},
	}
	if got := cfg.ResolveBaseURL("openai"); got != "http://from-config" {
		t.Errorf("expected 'http://from-config', got %q", got)
	}
}

func TestResolveBaseURL_NeitherSet(t *testing.T) {
	t.Setenv("AXE_OPENAI_BASE_URL", "")
	cfg := &GlobalConfig{Providers: map[string]ProviderConfig{}}
	if got := cfg.ResolveBaseURL("openai"); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestResolveBaseURL_NilProvidersMap(t *testing.T) {
	t.Setenv("AXE_ANTHROPIC_BASE_URL", "http://from-env")
	cfg := &GlobalConfig{Providers: nil}
	if got := cfg.ResolveBaseURL("anthropic"); got != "http://from-env" {
		t.Errorf("expected 'http://from-env', got %q", got)
	}
}

func TestResolveBaseURL_EmptyEnvVar(t *testing.T) {
	t.Setenv("AXE_OPENAI_BASE_URL", "")
	cfg := &GlobalConfig{
		Providers: map[string]ProviderConfig{
			"openai": {BaseURL: "http://from-config"},
		},
	}
	if got := cfg.ResolveBaseURL("openai"); got != "http://from-config" {
		t.Errorf("expected 'http://from-config', got %q", got)
	}
}

func TestAPIKeyEnvVar_KnownProvider(t *testing.T) {
	if got := APIKeyEnvVar("anthropic"); got != "ANTHROPIC_API_KEY" {
		t.Errorf("expected ANTHROPIC_API_KEY, got %q", got)
	}
	if got := APIKeyEnvVar("openai"); got != "OPENAI_API_KEY" {
		t.Errorf("expected OPENAI_API_KEY, got %q", got)
	}
}

func TestAPIKeyEnvVar_UnknownProvider(t *testing.T) {
	if got := APIKeyEnvVar("groq"); got != "GROQ_API_KEY" {
		t.Errorf("expected GROQ_API_KEY, got %q", got)
	}
}

func TestResolveAPIKey_OpenCode(t *testing.T) {
	// Env var takes precedence over config file.
	t.Setenv("OPENCODE_API_KEY", "zen-key-from-env")
	cfg := &GlobalConfig{
		Providers: map[string]ProviderConfig{
			"opencode": {APIKey: "zen-key-from-config"},
		},
	}
	if got := cfg.ResolveAPIKey("opencode"); got != "zen-key-from-env" {
		t.Errorf("expected 'zen-key-from-env', got %q", got)
	}

	// Config file value used when env var is empty.
	t.Setenv("OPENCODE_API_KEY", "")
	if got := cfg.ResolveAPIKey("opencode"); got != "zen-key-from-config" {
		t.Errorf("expected 'zen-key-from-config', got %q", got)
	}
}

func TestAPIKeyEnvVar_OpenCode(t *testing.T) {
	if got := APIKeyEnvVar("opencode"); got != "OPENCODE_API_KEY" {
		t.Errorf("expected OPENCODE_API_KEY, got %q", got)
	}
}

func TestAPIKeyEnvVar_Google(t *testing.T) {
	if got := APIKeyEnvVar("google"); got != "GEMINI_API_KEY" {
		t.Errorf("expected GEMINI_API_KEY, got %q", got)
	}
}

func TestResolveAPIKey_Google(t *testing.T) {
	// Env var takes precedence over config file.
	t.Setenv("GEMINI_API_KEY", "gemini-key-from-env")
	cfg := &GlobalConfig{
		Providers: map[string]ProviderConfig{
			"google": {APIKey: "gemini-key-from-config"},
		},
	}
	if got := cfg.ResolveAPIKey("google"); got != "gemini-key-from-env" {
		t.Errorf("expected 'gemini-key-from-env', got %q", got)
	}

	// Config file value used when env var is empty.
	t.Setenv("GEMINI_API_KEY", "")
	if got := cfg.ResolveAPIKey("google"); got != "gemini-key-from-config" {
		t.Errorf("expected 'gemini-key-from-config', got %q", got)
	}

	// Empty string when neither is set.
	t.Setenv("GEMINI_API_KEY", "")
	emptyCfg := &GlobalConfig{Providers: map[string]ProviderConfig{}}
	if got := emptyCfg.ResolveAPIKey("google"); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestAPIKeyEnvVar_MiniMax(t *testing.T) {
	if got := APIKeyEnvVar("minimax"); got != "MINIMAX_API_KEY" {
		t.Errorf("expected MINIMAX_API_KEY, got %q", got)
	}
}

func TestResolveAPIKey_MiniMax(t *testing.T) {
	// Env var takes precedence over config file.
	t.Setenv("MINIMAX_API_KEY", "minimax-key-from-env")
	cfg := &GlobalConfig{
		Providers: map[string]ProviderConfig{
			"minimax": {APIKey: "minimax-key-from-config"},
		},
	}
	if got := cfg.ResolveAPIKey("minimax"); got != "minimax-key-from-env" {
		t.Errorf("expected 'minimax-key-from-env', got %q", got)
	}

	// Config file value used when env var is empty.
	t.Setenv("MINIMAX_API_KEY", "")
	if got := cfg.ResolveAPIKey("minimax"); got != "minimax-key-from-config" {
		t.Errorf("expected 'minimax-key-from-config', got %q", got)
	}

	// Empty string when neither is set.
	t.Setenv("MINIMAX_API_KEY", "")
	emptyCfg := &GlobalConfig{Providers: map[string]ProviderConfig{}}
	if got := emptyCfg.ResolveAPIKey("minimax"); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// --- ResolveRegion tests ---

func TestResolveRegion_AWS_REGION(t *testing.T) {
	t.Setenv("AWS_REGION", "us-west-2")
	t.Setenv("AWS_DEFAULT_REGION", "us-east-1")
	cfg := &GlobalConfig{Providers: map[string]ProviderConfig{"bedrock": {Region: "eu-central-1"}}}
	if got := cfg.ResolveRegion("bedrock"); got != "us-west-2" {
		t.Errorf("expected 'us-west-2' (AWS_REGION), got %q", got)
	}
}

func TestResolveRegion_AWS_DEFAULT_REGION(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "us-east-1")
	cfg := &GlobalConfig{Providers: map[string]ProviderConfig{"bedrock": {Region: "eu-central-1"}}}
	if got := cfg.ResolveRegion("bedrock"); got != "us-east-1" {
		t.Errorf("expected 'us-east-1' (AWS_DEFAULT_REGION), got %q", got)
	}
}

func TestResolveRegion_ProviderEnvVar(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AXE_BEDROCK_REGION", "ap-northeast-1")
	cfg := &GlobalConfig{Providers: map[string]ProviderConfig{"bedrock": {Region: "eu-central-1"}}}
	if got := cfg.ResolveRegion("bedrock"); got != "ap-northeast-1" {
		t.Errorf("expected 'ap-northeast-1' (AXE_BEDROCK_REGION), got %q", got)
	}
}

func TestResolveRegion_ConfigFile(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AXE_BEDROCK_REGION", "")
	cfg := &GlobalConfig{Providers: map[string]ProviderConfig{"bedrock": {Region: "eu-central-1"}}}
	if got := cfg.ResolveRegion("bedrock"); got != "eu-central-1" {
		t.Errorf("expected 'eu-central-1' from config, got %q", got)
	}
}

func TestResolveRegion_NilProviders(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	cfg := &GlobalConfig{Providers: nil}
	if got := cfg.ResolveRegion("bedrock"); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestResolveRegion_UnknownProvider(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AXE_UNKNOWN_REGION", "sa-east-1")
	cfg := &GlobalConfig{Providers: map[string]ProviderConfig{}}
	if got := cfg.ResolveRegion("unknown"); got != "sa-east-1" {
		t.Errorf("expected 'sa-east-1' (AXE_UNKNOWN_REGION), got %q", got)
	}
}

// --- OpenRouter attribution resolution tests ---

func TestResolveOpenRouterAttribution(t *testing.T) {
	cases := []struct {
		name       string
		env        map[string]string
		cfg        *GlobalConfig
		provider   string
		fn         string // "referer", "title", or "categories"
		want       string
	}{
		{
			name:     "referer env overrides config",
			env:      map[string]string{"AXE_OPENROUTER_REFERER": "https://env.example.com"},
			cfg:      &GlobalConfig{Providers: map[string]ProviderConfig{"openrouter": {Referer: "https://config.example.com"}}},
			provider: "openrouter",
			fn:       "referer",
			want:     "https://env.example.com",
		},
		{
			name:     "referer config fallback when env empty",
			env:      map[string]string{"AXE_OPENROUTER_REFERER": ""},
			cfg:      &GlobalConfig{Providers: map[string]ProviderConfig{"openrouter": {Referer: "https://config.example.com"}}},
			provider: "openrouter",
			fn:       "referer",
			want:     "https://config.example.com",
		},
		{
			name:     "referer unknown provider",
			env:      map[string]string{"AXE_OPENROUTER_REFERER": ""},
			cfg:      &GlobalConfig{Providers: map[string]ProviderConfig{}},
			provider: "unknown",
			fn:       "referer",
			want:     "",
		},
		{
			name:     "title config fallback when env empty",
			env:      map[string]string{"AXE_OPENROUTER_TITLE": ""},
			cfg:      &GlobalConfig{Providers: map[string]ProviderConfig{"openrouter": {Title: "ConfigTitle"}}},
			provider: "openrouter",
			fn:       "title",
			want:     "ConfigTitle",
		},
		{
			name:     "title unknown provider",
			env:      map[string]string{"AXE_OPENROUTER_TITLE": ""},
			cfg:      &GlobalConfig{Providers: map[string]ProviderConfig{}},
			provider: "unknown",
			fn:       "title",
			want:     "",
		},
		{
			name:     "categories config fallback when env empty",
			env:      map[string]string{"AXE_OPENROUTER_CATEGORIES": ""},
			cfg:      &GlobalConfig{Providers: map[string]ProviderConfig{"openrouter": {Categories: "config-cat"}}},
			provider: "openrouter",
			fn:       "categories",
			want:     "config-cat",
		},
		{
			name:     "categories unknown provider",
			env:      map[string]string{"AXE_OPENROUTER_CATEGORIES": ""},
			cfg:      &GlobalConfig{Providers: map[string]ProviderConfig{}},
			provider: "unknown",
			fn:       "categories",
			want:     "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			var got string
			switch tc.fn {
			case "referer":
				got = tc.cfg.ResolveReferer(tc.provider)
			case "title":
				got = tc.cfg.ResolveTitle(tc.provider)
			case "categories":
				got = tc.cfg.ResolveCategories(tc.provider)
			default:
				t.Fatalf("unknown fn %q", tc.fn)
			}
			if got != tc.want {
				t.Errorf("expected %q, got %q", tc.want, got)
			}
		})
	}
}
