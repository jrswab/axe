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

func TestLoad_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	axeDir := filepath.Join(tmp, "axe")
	os.MkdirAll(axeDir, 0755)
	os.WriteFile(filepath.Join(axeDir, "config.toml"), []byte(""), 0644)

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

	axeDir := filepath.Join(tmp, "axe")
	os.MkdirAll(axeDir, 0755)

	content := `
[providers.anthropic]
api_key = "sk-ant-test"
base_url = "https://custom.anthropic.com"

[providers.openai]
api_key = "sk-openai-test"

[providers.ollama]
base_url = "http://myhost:11434"
`
	os.WriteFile(filepath.Join(axeDir, "config.toml"), []byte(content), 0644)

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

	axeDir := filepath.Join(tmp, "axe")
	os.MkdirAll(axeDir, 0755)
	os.WriteFile(filepath.Join(axeDir, "config.toml"), []byte("[invalid toml\nblah blah"), 0644)

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

	axeDir := filepath.Join(tmp, "axe")
	os.MkdirAll(axeDir, 0755)
	os.WriteFile(filepath.Join(axeDir, "config.toml"), []byte(`
[providers.openai]
api_key = "sk-partial"
`), 0644)

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


