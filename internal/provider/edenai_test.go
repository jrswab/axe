package provider

import (
	"errors"
	"strings"
	"testing"
)

func TestNewEdenAI(t *testing.T) {
	type input struct {
		apiKey  string
		baseURL string
	}
	type want struct {
		baseURL     string
		errCategory ErrorCategory
		errContains string
		streaming   bool
	}

	tests := []struct {
		name  string
		input input
		want  want
	}{
		{
			name:  "default base URL",
			input: input{apiKey: "test-key"},
			want:  want{baseURL: defaultEdenAIBaseURL},
		},
		{
			name:  "custom base URL",
			input: input{apiKey: "test-key", baseURL: "https://example.com/v3"},
			want:  want{baseURL: "https://example.com/v3"},
		},
		{
			name:  "trailing slash normalized",
			input: input{apiKey: "test-key", baseURL: "https://example.com/v3/"},
			want:  want{baseURL: "https://example.com/v3"},
		},
		{
			name:  "localhost HTTP exception",
			input: input{apiKey: "test-key", baseURL: "http://localhost:8080/v3"},
			want:  want{baseURL: "http://localhost:8080/v3"},
		},
		{
			name:  "IPv4 loopback HTTP exception",
			input: input{apiKey: "test-key", baseURL: "http://127.0.0.1:8080/v3"},
			want:  want{baseURL: "http://127.0.0.1:8080/v3"},
		},
		{
			name:  "IPv6 loopback HTTP exception",
			input: input{apiKey: "test-key", baseURL: "http://[::1]:8080/v3"},
			want:  want{baseURL: "http://[::1]:8080/v3"},
		},
		{
			name:  "private IPv4 HTTP exception",
			input: input{apiKey: "test-key", baseURL: "http://10.0.0.2:8080/v3"},
			want:  want{baseURL: "http://10.0.0.2:8080/v3"},
		},
		{
			name:  "private IPv6 HTTP exception",
			input: input{apiKey: "test-key", baseURL: "http://[fd00::2]:8080/v3"},
			want:  want{baseURL: "http://[fd00::2]:8080/v3"},
		},
		{
			name:  "mapped private IPv4 HTTP exception",
			input: input{apiKey: "test-key", baseURL: "http://[::ffff:192.168.1.2]:8080/v3"},
			want:  want{baseURL: "http://[::ffff:192.168.1.2]:8080/v3"},
		},
		{
			name:  "relative URL rejected",
			input: input{apiKey: "test-key", baseURL: "/v3"},
			want:  want{errCategory: ErrCategoryBadRequest, errContains: "absolute HTTP(S) URL"},
		},
		{
			name:  "scheme relative URL rejected",
			input: input{apiKey: "test-key", baseURL: "//example.com/v3"},
			want:  want{errCategory: ErrCategoryBadRequest, errContains: "absolute HTTP(S) URL"},
		},
		{
			name:  "malformed URL rejected",
			input: input{apiKey: "test-key", baseURL: "https://[::1/v3"},
			want:  want{errCategory: ErrCategoryBadRequest, errContains: "absolute HTTP(S) URL"},
		},
		{
			name:  "missing host rejected",
			input: input{apiKey: "test-key", baseURL: "https:///v3"},
			want:  want{errCategory: ErrCategoryBadRequest, errContains: "absolute HTTP(S) URL"},
		},
		{
			name:  "unsupported scheme rejected",
			input: input{apiKey: "test-key", baseURL: "ftp://example.com/v3"},
			want:  want{errCategory: ErrCategoryBadRequest, errContains: "absolute HTTP(S) URL"},
		},
		{
			name:  "public HTTP hostname rejected",
			input: input{apiKey: "test-key", baseURL: "http://api.edenai.run/v3"},
			want:  want{errCategory: ErrCategoryBadRequest, errContains: "use HTTPS"},
		},
		{
			name:  "public HTTP IP rejected",
			input: input{apiKey: "test-key", baseURL: "http://8.8.8.8/v3"},
			want:  want{errCategory: ErrCategoryBadRequest, errContains: "use HTTPS"},
		},
		{
			name:  "localhost lookalike rejected",
			input: input{apiKey: "test-key", baseURL: "http://localhost.example.com/v3"},
			want:  want{errCategory: ErrCategoryBadRequest, errContains: "use HTTPS"},
		},
		{
			name:  "userinfo rejected",
			input: input{apiKey: "test-key", baseURL: "https://user:secret@example.com/v3"},
			want:  want{errCategory: ErrCategoryBadRequest, errContains: "without credentials, query, or fragment"},
		},
		{
			name:  "query rejected",
			input: input{apiKey: "test-key", baseURL: "https://example.com/v3?key=secret"},
			want:  want{errCategory: ErrCategoryBadRequest, errContains: "without credentials, query, or fragment"},
		},
		{
			name:  "fragment rejected",
			input: input{apiKey: "test-key", baseURL: "https://example.com/v3#fragment"},
			want:  want{errCategory: ErrCategoryBadRequest, errContains: "without credentials, query, or fragment"},
		},
		{
			name:  "empty fragment rejected",
			input: input{apiKey: "test-key", baseURL: "https://example.com/v3#"},
			want:  want{errCategory: ErrCategoryBadRequest, errContains: "without credentials, query, or fragment"},
		},
		{
			name:  "missing API key",
			input: input{},
			want: want{
				errCategory: ErrCategoryAuth,
				errContains: "EDENAI_API_KEY",
			},
		},
		{
			name:  "streaming support",
			input: input{apiKey: "test-key"},
			want: want{
				baseURL:   defaultEdenAIBaseURL,
				streaming: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewEdenAI(tt.input.apiKey, tt.input.baseURL)
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
					t.Fatal("expected Eden AI to support streaming")
				}
			}
		})
	}
}
