package provider

import (
	"strings"
	"testing"
)

func TestNew_Anthropic(t *testing.T) {
	p, err := New("anthropic", "test-key", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNew_Bedrock(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "SECRET")
	p, err := New("bedrock", "us-east-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNew_OpenAI(t *testing.T) {
	p, err := New("openai", "test-key", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNew_Ollama(t *testing.T) {
	p, err := New("ollama", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNew_OllamaIgnoresAPIKey(t *testing.T) {
	_, err := New("ollama", "ignored-key", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNew_WithBaseURL(t *testing.T) {
	_, err := New("openai", "test-key", "http://custom:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNew_UnsupportedProvider(t *testing.T) {
	_, err := New("groq", "key", "")
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
	if !strings.Contains(err.Error(), `unsupported provider "groq"`) {
		t.Errorf("expected error to mention 'unsupported provider \"groq\"', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "anthropic, openai, ollama, opencode, google, minimax, bedrock") {
		t.Errorf("expected error to list supported providers, got %q", err.Error())
	}
}

func TestNew_EmptyProviderName(t *testing.T) {
	_, err := New("", "key", "")
	if err == nil {
		t.Fatal("expected error for empty provider name")
	}
	if !strings.Contains(err.Error(), `unsupported provider ""`) {
		t.Errorf("expected error message about empty provider, got %q", err.Error())
	}
}

func TestNew_CaseSensitive(t *testing.T) {
	_, err := New("Anthropic", "key", "")
	if err == nil {
		t.Fatal("expected error for mixed-case provider name")
	}
}

func TestNew_MissingAPIKeyAnthropic(t *testing.T) {
	_, err := New("anthropic", "", "")
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "API key is required") {
		t.Errorf("expected 'API key is required', got %q", err.Error())
	}
}

func TestNew_MissingAPIKeyOpenAI(t *testing.T) {
	_, err := New("openai", "", "")
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "API key is required") {
		t.Errorf("expected 'API key is required', got %q", err.Error())
	}
}

func TestSupported_KnownProviders(t *testing.T) {
	for _, name := range []string{"anthropic", "openai", "ollama", "google", "minimax", "bedrock"} {
		if !Supported(name) {
			t.Errorf("expected %q to be supported", name)
		}
	}
}

func TestSupported_UnknownProvider(t *testing.T) {
	if Supported("groq") {
		t.Error("expected 'groq' to not be supported")
	}
	if Supported("") {
		t.Error("expected empty string to not be supported")
	}
	if Supported("Anthropic") {
		t.Error("expected 'Anthropic' (mixed case) to not be supported")
	}
}

func TestNew_OpenCode(t *testing.T) {
	p, err := New("opencode", "test-key", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNew_OpenCodeWithBaseURL(t *testing.T) {
	p, err := New("opencode", "test-key", "http://custom:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNew_OpenCodeMissingAPIKey(t *testing.T) {
	_, err := New("opencode", "", "")
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "API key is required") {
		t.Errorf("expected 'API key is required', got %q", err.Error())
	}
}

func TestSupported_OpenCode(t *testing.T) {
	if !Supported("opencode") {
		t.Error("expected 'opencode' to be supported")
	}
}

func TestNew_UnsupportedProvider_ErrorMessage(t *testing.T) {
	_, err := New("groq", "key", "")
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
	for _, name := range []string{"anthropic", "openai", "ollama", "opencode", "google", "minimax", "bedrock"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("expected error to mention %q, got %q", name, err.Error())
		}
	}
}

func TestNew_Google(t *testing.T) {
	p, err := New("google", "test-key", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNew_GoogleWithBaseURL(t *testing.T) {
	p, err := New("google", "test-key", "http://custom:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNew_GoogleMissingAPIKey(t *testing.T) {
	_, err := New("google", "", "")
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "API key is required") {
		t.Errorf("expected 'API key is required', got %q", err.Error())
	}
}

func TestSupported_Google(t *testing.T) {
	if !Supported("google") {
		t.Error("expected 'google' to be supported")
	}
}

func TestNew_MiniMax(t *testing.T) {
	p, err := New("minimax", "test-key", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNew_MiniMaxWithBaseURL(t *testing.T) {
	p, err := New("minimax", "test-key", "http://custom:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNew_MiniMaxMissingAPIKey(t *testing.T) {
	_, err := New("minimax", "", "")
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "API key is required") {
		t.Errorf("expected 'API key is required', got %q", err.Error())
	}
}

func TestSupported_MiniMax(t *testing.T) {
	if !Supported("minimax") {
		t.Error("expected 'minimax' to be supported")
	}
}
