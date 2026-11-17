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

func TestAtlasCloud_ReasoningEffort(t *testing.T) {
	for _, method := range []struct {
		name     string
		isStream bool
	}{
		{name: "Send"},
		{name: "SendStream", isStream: true},
	} {
		t.Run(method.name, func(t *testing.T) {
			for _, tt := range []struct {
				name   string
				effort string
			}{
				{name: "empty"},
				{name: "low", effort: "low"},
			} {
				t.Run(tt.name, func(t *testing.T) {
					var gotBody []byte
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						gotBody, _ = io.ReadAll(r.Body)
						if method.isStream {
							w.Header().Set("Content-Type", "text/event-stream")
							_, _ = w.Write([]byte("data: [DONE]\n\n"))
							return
						}
						_ = json.NewEncoder(w).Encode(map[string]interface{}{
							"model":   "test-model",
							"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"}},
							"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
						})
					}))
					defer server.Close()

					provider, err := NewAtlasCloud("test-key", server.URL)
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
					req := &Request{
						Model:           "test-model",
						Messages:        []Message{{Role: "user", Content: "Hi"}},
						ReasoningEffort: tt.effort,
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

					var body map[string]interface{}
					if err := json.Unmarshal(gotBody, &body); err != nil {
						t.Fatalf("failed to parse request body: %v", err)
					}
					effort, hasEffort := body["reasoning_effort"]
					if tt.effort == "" {
						if hasEffort {
							t.Error("expected reasoning_effort to be omitted when empty")
						}
					} else {
						if effort != tt.effort {
							t.Errorf("reasoning_effort = %v, want %q", effort, tt.effort)
						}
						if !strings.Contains(string(gotBody), `"reasoning_effort":"low"`) {
							t.Errorf("raw body does not contain reasoning_effort low: %s", gotBody)
						}
					}
					if _, ok := body["reasoning"]; ok {
						t.Error("request body should not contain a nested reasoning object")
					}
				})
			}
		})
	}
}

func TestAtlasCloud_TemperaturePresence(t *testing.T) {
	for _, method := range []struct {
		name     string
		isStream bool
	}{
		{name: "Send"},
		{name: "SendStream", isStream: true},
	} {
		t.Run(method.name, func(t *testing.T) {
			for _, tt := range []struct {
				name    string
				set     bool
				wantRaw string
			}{
				{name: "explicit zero", set: true, wantRaw: "0"},
				{name: "unset"},
			} {
				t.Run(tt.name, func(t *testing.T) {
					var gotBody []byte
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						gotBody, _ = io.ReadAll(r.Body)
						if method.isStream {
							w.Header().Set("Content-Type", "text/event-stream")
							_, _ = w.Write([]byte("data: [DONE]\n\n"))
							return
						}
						_ = json.NewEncoder(w).Encode(map[string]interface{}{
							"model":   "test-model",
							"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"}},
							"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
						})
					}))
					defer server.Close()

					provider, err := NewAtlasCloud("test-key", server.URL)
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
					req := &Request{
						Model:          "test-model",
						Messages:       []Message{{Role: "user", Content: "Hi"}},
						TemperatureSet: tt.set,
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

					var body map[string]json.RawMessage
					if err := json.Unmarshal(gotBody, &body); err != nil {
						t.Fatalf("failed to parse request body: %v", err)
					}
					if got := string(body["temperature"]); got != tt.wantRaw {
						t.Errorf("temperature = %q, want %q", got, tt.wantRaw)
					}
				})
			}
		})
	}
}

func TestNewAtlasCloud(t *testing.T) {
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
