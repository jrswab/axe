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
	if !strings.Contains(err.Error(), "anthropic, openai, ollama") {
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
