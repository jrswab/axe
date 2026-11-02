package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewRuncrate(t *testing.T) {
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
			want:  want{baseURL: defaultRuncrateBaseURL},
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
				errContains: "RUNCRATE_API_KEY",
			},
		},
		{
			name:  "streaming support",
			input: input{apiKey: "test-key"},
			want: want{
				baseURL:   defaultRuncrateBaseURL,
				streaming: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewRuncrate(tt.input.apiKey, tt.input.baseURL)
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
					t.Fatal("expected Runcrate to support streaming")
				}
			}
		})
	}
}

func TestRuncrate_ChatCompletionsRequest(t *testing.T) {
	for _, method := range []struct {
		name     string
		isStream bool
	}{
		{name: "Send"},
		{name: "SendStream", isStream: true},
	} {
		t.Run(method.name, func(t *testing.T) {
			var gotPath, gotAuth string
			var gotBody []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotAuth = r.Header.Get("Authorization")
				gotBody, _ = io.ReadAll(r.Body)
				if method.isStream {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = w.Write([]byte("data: [DONE]\n\n"))
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"model":   "deepseek-ai/DeepSeek-V4.1-Flash",
					"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"}},
					"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
				})
			}))
			defer server.Close()

			provider, err := NewRuncrate("test-key", server.URL+"/v1")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			req := &Request{
				Model:    "deepseek-ai/DeepSeek-V4.1-Flash",
				Messages: []Message{{Role: "user", Content: "Hi"}},
			}
			if method.isStream {
				stream, err := provider.SendStream(context.Background(), req)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				defer func() { _ = stream.Close() }()
			} else if _, err := provider.Send(context.Background(), req); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if gotPath != "/v1/chat/completions" {
				t.Errorf("path = %q, want /v1/chat/completions", gotPath)
			}
			if gotAuth != "Bearer test-key" {
				t.Errorf("Authorization = %q, want Bearer test-key", gotAuth)
			}
			var body map[string]interface{}
			if err := json.Unmarshal(gotBody, &body); err != nil {
				t.Fatalf("failed to parse request body: %v", err)
			}
			if body["model"] != "deepseek-ai/DeepSeek-V4.1-Flash" {
				t.Errorf("model = %v, want deepseek-ai/DeepSeek-V4.1-Flash", body["model"])
			}
		})
	}
}
