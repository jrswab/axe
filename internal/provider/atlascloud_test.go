package provider

import "testing"

func TestNewAtlasCloud_DefaultBaseURL(t *testing.T) {
	provider, err := NewAtlasCloud("test-key", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider.baseURL != defaultAtlasCloudBaseURL {
		t.Errorf("baseURL = %q, want %q", provider.baseURL, defaultAtlasCloudBaseURL)
	}
}

func TestNewAtlasCloud_CustomBaseURL(t *testing.T) {
	const customURL = "https://example.com/v1"
	provider, err := NewAtlasCloud("test-key", customURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider.baseURL != customURL {
		t.Errorf("baseURL = %q, want %q", provider.baseURL, customURL)
	}
}

func TestNewAtlasCloud_MissingAPIKey(t *testing.T) {
	if _, err := NewAtlasCloud("", ""); err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestAtlasCloudSupportsStreaming(t *testing.T) {
	provider, err := NewAtlasCloud("test-key", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := interface{}(provider).(StreamProvider); !ok {
		t.Fatal("expected Atlas Cloud to support streaming")
	}
}
