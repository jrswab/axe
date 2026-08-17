package provider

import (
	"errors"
	"strings"
	"testing"
)

func TestNewAtlasCloud(t *testing.T) {
	type input struct {
		apiKey  string
		baseURL string
	}
	type want struct {
		baseURL      string
		errCategory  ErrorCategory
		errContains  string
		streaming    bool
	}

	tests := []struct {
		name  string
		input input
		want  want
	}{
		{
			name:  "default base URL",
			input: input{apiKey: "test-key"},
			want:  want{baseURL: defaultAtlasCloudBaseURL},
		},
		{
			name:  "custom base URL",
			input: input{apiKey: "test-key", baseURL: "https://example.com/v1"},
			want:  want{baseURL: "https://example.com/v1"},
		},
		{
			name:  "missing API key",
			input: input{},
			want: want{
				errCategory: ErrCategoryAuth,
				errContains: "ATLASCLOUD_API_KEY",
			},
		},
		{
			name:  "streaming support",
			input: input{apiKey: "test-key"},
			want: want{
				baseURL:   defaultAtlasCloudBaseURL,
				streaming: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewAtlasCloud(tt.input.apiKey, tt.input.baseURL)
			if tt.want.errCategory != "" {
				if err == nil {
					t.Fatal("expected error")
				}
				var providerErr *ProviderError
				if !errors.As(err, &providerErr) {
					t.Fatalf("expected ProviderError, got %T", err)
				}
				if providerErr.Category != tt.want.errCategory {
					t.Errorf("error category = %q, want %q", providerErr.Category, tt.want.errCategory)
				}
				if !strings.Contains(providerErr.Message, tt.want.errContains) {
					t.Errorf("error message = %q, want it to contain %q", providerErr.Message, tt.want.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if provider.baseURL != tt.want.baseURL {
				t.Errorf("baseURL = %q, want %q", provider.baseURL, tt.want.baseURL)
			}
			if tt.want.streaming {
				if _, ok := interface{}(provider).(StreamProvider); !ok {
					t.Fatal("expected Atlas Cloud to support streaming")
				}
			}
		})
	}
}
