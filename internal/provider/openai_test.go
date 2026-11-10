package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNewOpenAI_EmptyAPIKey(t *testing.T) {
	_, err := NewOpenAI("")
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
	if !strings.Contains(err.Error(), "API key is required") {
		t.Errorf("expected 'API key is required', got %q", err.Error())
	}
}

func TestNewOpenAI_ValidAPIKey(t *testing.T) {
	o, err := NewOpenAI("test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o == nil {
		t.Fatal("expected non-nil OpenAI")
	}
}

func TestOpenAI_Send_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"model": "gpt-4o",
			"choices": []map[string]interface{}{
				{
					"message":       map[string]string{"content": "Hello from OpenAI"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     10,
				"completion_tokens": 5,
			},
		})
	}))
	defer server.Close()

	o, _ := NewOpenAI("test-key", WithOpenAIBaseURL(server.URL))
	resp, err := o.Send(context.Background(), &Request{
		Model:    "gpt-4o",
		System:   "You are helpful.",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Hello from OpenAI" {
		t.Errorf("expected 'Hello from OpenAI', got %q", resp.Content)
	}
	if resp.Model != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got %q", resp.Model)
	}
	if resp.InputTokens != 10 {
		t.Errorf("expected 10 input tokens, got %d", resp.InputTokens)
	}
	if resp.OutputTokens != 5 {
		t.Errorf("expected 5 output tokens, got %d", resp.OutputTokens)
	}
	if resp.StopReason != "stop" {
		t.Errorf("expected stop reason 'stop', got %q", resp.StopReason)
	}
}

func TestOpenAI_Send_RequestFormat(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotCT, gotModel string
	var gotMsgCount int
	var gotFirstRole, gotSecondRole string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")

		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		_ = json.Unmarshal(body, &req)

		gotModel, _ = req["model"].(string)
		if msgs, ok := req["messages"].([]interface{}); ok {
			gotMsgCount = len(msgs)
			if len(msgs) >= 1 {
				if first, ok := msgs[0].(map[string]interface{}); ok {
					gotFirstRole, _ = first["role"].(string)
				}
			}
			if len(msgs) >= 2 {
				if second, ok := msgs[1].(map[string]interface{}); ok {
					gotSecondRole, _ = second["role"].(string)
				}
			}
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"model":   "gpt-4o",
			"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"}},
			"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenAI("test-key", WithOpenAIBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:    "gpt-4o",
		System:   "Be helpful",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != "POST" {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("expected /chat/completions, got %s", gotPath)
	}
	if gotAuth != "Bearer test-key" {
		t.Errorf("expected 'Bearer test-key', got %q", gotAuth)
	}
	if gotCT != "application/json" {
		t.Errorf("expected 'application/json', got %q", gotCT)
	}
	if gotModel != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got %v", gotModel)
	}
	if gotMsgCount != 2 {
		t.Fatalf("expected 2 messages, got %d", gotMsgCount)
	}
	if gotFirstRole != "system" {
		t.Errorf("expected first message role 'system', got %v", gotFirstRole)
	}
	if gotSecondRole != "user" {
		t.Errorf("expected second message role 'user', got %v", gotSecondRole)
	}
}

func TestOpenAI_Send_CustomBaseURL_NoV1Prefix(t *testing.T) {
	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"model":   "gpt-4o",
			"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"}},
			"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	// Custom base_url simulating Gloo: server.URL acts as "https://platform.ai.gloo.com/ai/v2"
	o, _ := NewOpenAI("test-key", WithOpenAIBaseURL(server.URL+"/ai/v2"))
	_, err := o.Send(context.Background(), &Request{
		Model:    "gloo-anthropic-claude-sonnet-4.5",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Custom base URL should NOT get /v1 injected — path should be /ai/v2/chat/completions
	if gotPath != "/ai/v2/chat/completions" {
		t.Errorf("expected /ai/v2/chat/completions, got %s", gotPath)
	}
}

func TestOpenAI_Send_OmitsEmptySystem(t *testing.T) {
	var gotMsgCount int
	var gotFirstRole string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		_ = json.Unmarshal(body, &req)

		if msgs, ok := req["messages"].([]interface{}); ok {
			gotMsgCount = len(msgs)
			if len(msgs) >= 1 {
				if first, ok := msgs[0].(map[string]interface{}); ok {
					gotFirstRole, _ = first["role"].(string)
				}
			}
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"model":   "gpt-4o",
			"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"}},
			"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenAI("test-key", WithOpenAIBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMsgCount != 1 {
		t.Errorf("expected 1 message (no system), got %d", gotMsgCount)
	}
	if gotFirstRole != "user" {
		t.Errorf("expected role 'user', got %v", gotFirstRole)
	}
}

func TestOpenAI_ReasoningEffort(t *testing.T) {
	methods := []struct {
		name        string
		isStream    bool
		wantLowBody string
	}{
		{
			name:        "Send",
			wantLowBody: `{"model":"gpt-4o","messages":[{"role":"user","content":"Hi"}],"reasoning_effort":"low"}`,
		},
		{
			name:        "SendStream",
			isStream:    true,
			wantLowBody: `{"model":"gpt-4o","messages":[{"role":"user","content":"Hi"}],"reasoning_effort":"low","stream":true,"stream_options":{"include_usage":true}}`,
		},
	}
	tests := []struct {
		name   string
		effort string
	}{
		{name: "empty"},
		{name: "low", effort: "low"},
		{name: "none", effort: "none"},
		{name: "high", effort: "high"},
		{name: "max", effort: "max"},
		{name: "custom", effort: "custom-effort"},
	}

	for _, method := range methods {
		t.Run(method.name, func(t *testing.T) {
			for _, tt := range tests {
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
							"model":   "gpt-4o",
							"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"}},
							"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
						})
					}))
					defer server.Close()

					o, _ := NewOpenAI("test-key", WithOpenAIBaseURL(server.URL))
					req := &Request{
						Model:           "gpt-4o",
						Messages:        []Message{{Role: "user", Content: "Hi"}},
						ReasoningEffort: tt.effort,
					}
					if method.isStream {
						stream, err := o.SendStream(context.Background(), req)
						if err != nil {
							t.Fatalf("unexpected error: %v", err)
						}
						defer func() { _ = stream.Close() }()
					} else if _, err := o.Send(context.Background(), req); err != nil {
						t.Fatalf("unexpected error: %v", err)
					}

					if tt.effort == "low" && string(gotBody) != method.wantLowBody {
						t.Errorf("request body = %s, want %s", gotBody, method.wantLowBody)
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
						if !strings.Contains(string(gotBody), `"reasoning_effort":"`+tt.effort+`"`) {
							t.Errorf("raw body does not contain reasoning_effort %q: %s", tt.effort, gotBody)
						}
					}
					if _, ok := body["reasoning"]; ok {
						t.Error("request body should not contain a nested reasoning object")
					}
					if method.isStream && body["stream"] != true {
						t.Errorf("stream = %v, want true", body["stream"])
					}
				})
			}
		})
	}
}

func TestOpenAI_TemperatureAndLegacyBody(t *testing.T) {
	methods := []struct {
		name           string
		isStream       bool
		wantLegacyBody string
	}{
		{
			name:           "Send",
			wantLegacyBody: `{"model":"gpt-4o","messages":[{"role":"user","content":"Hi"}]}`,
		},
		{
			name:           "SendStream",
			isStream:       true,
			wantLegacyBody: `{"model":"gpt-4o","messages":[{"role":"user","content":"Hi"}],"stream":true,"stream_options":{"include_usage":true}}`,
		},
	}
	tests := []struct {
		name        string
		temperature float64
		set         bool
		wantRaw     string
	}{
		{name: "explicit zero", set: true, wantRaw: "0"},
		{name: "legacy unset"},
		{name: "nonzero without presence", temperature: 0.7, wantRaw: "0.7"},
		{name: "nonzero with presence", temperature: 0.7, set: true, wantRaw: "0.7"},
	}
	for _, method := range methods {
		t.Run(method.name, func(t *testing.T) {
			for _, tt := range tests {
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
							"model":   "gpt-4o",
							"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"}},
							"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
						})
					}))
					defer server.Close()

					o, _ := NewOpenAI("test-key", WithOpenAIBaseURL(server.URL))
					req := &Request{
						Model:          "gpt-4o",
						Messages:       []Message{{Role: "user", Content: "Hi"}},
						Temperature:    tt.temperature,
						TemperatureSet: tt.set,
					}
					if method.isStream {
						stream, err := o.SendStream(context.Background(), req)
						if err != nil {
							t.Fatalf("unexpected error: %v", err)
						}
						defer func() { _ = stream.Close() }()
					} else if _, err := o.Send(context.Background(), req); err != nil {
						t.Fatalf("unexpected error: %v", err)
					}

					var body map[string]json.RawMessage
					if err := json.Unmarshal(gotBody, &body); err != nil {
						t.Fatalf("failed to parse request body: %v", err)
					}
					if got := string(body["temperature"]); got != tt.wantRaw {
						t.Errorf("temperature = %q, want %q", got, tt.wantRaw)
					}
					if tt.wantRaw != "" && !strings.Contains(string(gotBody), `"temperature":`+tt.wantRaw) {
						t.Errorf("raw body missing temperature %s: %s", tt.wantRaw, gotBody)
					}
					if tt.wantRaw == "" && string(gotBody) != method.wantLegacyBody {
						t.Errorf("legacy request body = %s, want %s", gotBody, method.wantLegacyBody)
					}
				})
			}
		})
	}
}

func TestOpenAI_Send_OmitsZeroTemperature(t *testing.T) {
	var hasTemperature bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		_ = json.Unmarshal(body, &raw)
		_, hasTemperature = raw["temperature"]

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"model":   "gpt-4o",
			"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"}},
			"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenAI("test-key", WithOpenAIBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:       "gpt-4o",
		Messages:    []Message{{Role: "user", Content: "Hi"}},
		Temperature: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasTemperature {
		t.Error("expected temperature to be omitted when zero and unset")
	}
}

func TestOpenAI_Send_OmitsZeroMaxTokens(t *testing.T) {
	var hasMaxTokens bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		_ = json.Unmarshal(body, &raw)
		_, hasMaxTokens = raw["max_completion_tokens"]

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"model":   "gpt-4o",
			"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"}},
			"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenAI("test-key", WithOpenAIBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:     "gpt-4o",
		Messages:  []Message{{Role: "user", Content: "Hi"}},
		MaxTokens: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasMaxTokens {
		t.Error("expected max_completion_tokens to be omitted when 0")
	}
}

func TestOpenAI_Send_IncludesMaxTokens(t *testing.T) {
	var hasMaxTokens bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		_ = json.Unmarshal(body, &raw)
		_, hasMaxTokens = raw["max_completion_tokens"]

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"model":   "gpt-4o",
			"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"}},
			"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenAI("test-key", WithOpenAIBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:     "gpt-4o",
		Messages:  []Message{{Role: "user", Content: "Hi"}},
		MaxTokens: 1024,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasMaxTokens {
		t.Error("expected max_completion_tokens to be present")
	}
}

func TestOpenAI_Send_AuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid API key","type":"invalid_request_error","code":"invalid_api_key"}}`))
	}))
	defer server.Close()

	o, _ := NewOpenAI("bad-key", WithOpenAIBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryAuth {
		t.Errorf("expected ErrCategoryAuth, got %s", provErr.Category)
	}
}

func TestOpenAI_Send_ForbiddenError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{Model: "gpt-4o", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryAuth {
		t.Errorf("expected ErrCategoryAuth, got %s", provErr.Category)
	}
}

func TestOpenAI_Send_NotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{Model: "gpt-4o", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryBadRequest {
		t.Errorf("expected ErrCategoryBadRequest, got %s", provErr.Category)
	}
}

func TestOpenAI_Send_RateLimitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{Model: "gpt-4o", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryRateLimit {
		t.Errorf("expected ErrCategoryRateLimit, got %s", provErr.Category)
	}
}

func TestOpenAI_Send_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{Model: "gpt-4o", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryServer {
		t.Errorf("expected ErrCategoryServer, got %s", provErr.Category)
	}
}

func TestOpenAI_Send_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := o.Send(ctx, &Request{Model: "gpt-4o", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryTimeout {
		t.Errorf("expected ErrCategoryTimeout, got %s", provErr.Category)
	}
}

func TestOpenAI_Send_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"model":   "gpt-4o",
			"choices": []interface{}{},
			"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 0},
		})
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{Model: "gpt-4o", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryServer {
		t.Errorf("expected ErrCategoryServer, got %s", provErr.Category)
	}
	if !strings.Contains(provErr.Message, "no choices") {
		t.Errorf("expected message to contain 'no choices', got %q", provErr.Message)
	}
}

func TestOpenAI_Send_ErrorResponseParsing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid model specified","type":"invalid_request_error","code":null}}`))
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{Model: "bad-model", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if !strings.Contains(provErr.Message, "Invalid model specified") {
		t.Errorf("expected parsed error message, got %q", provErr.Message)
	}
}

func TestOpenAI_Send_UnparseableErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{Model: "gpt-4o", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	// Should fall back to HTTP status text
	if provErr.Message != "Bad Request" {
		t.Errorf("expected 'Bad Request', got %q", provErr.Message)
	}
}

// --- Phase 3a: Tool definitions in request ---

func TestOpenAI_Send_WithTools(t *testing.T) {
	var gotBody map[string]json.RawMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"model": "gpt-4o",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	_, _ = o.Send(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Tools: []Tool{
			{
				Name:        "call_agent",
				Description: "Delegate a task to a sub-agent.",
				Parameters: map[string]ToolParameter{
					"agent":   {Type: "string", Description: "Agent name", Required: true},
					"task":    {Type: "string", Description: "Task description", Required: true},
					"context": {Type: "string", Description: "Additional context", Required: false},
				},
			},
		},
	})

	// Verify tools key exists in request body
	if _, ok := gotBody["tools"]; !ok {
		t.Fatal("request body does not contain 'tools' key")
	}

	// Parse the tools array
	var tools []map[string]json.RawMessage
	if err := json.Unmarshal(gotBody["tools"], &tools); err != nil {
		t.Fatalf("failed to parse tools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("len(tools) = %d, want 1", len(tools))
	}

	// Verify type: "function" wrapper
	var toolType string
	_ = json.Unmarshal(tools[0]["type"], &toolType)
	if toolType != "function" {
		t.Errorf("tools[0].type = %q, want %q", toolType, "function")
	}

	// Verify function object
	var fn map[string]json.RawMessage
	_ = json.Unmarshal(tools[0]["function"], &fn)

	var name string
	_ = json.Unmarshal(fn["name"], &name)
	if name != "call_agent" {
		t.Errorf("tools[0].function.name = %q, want %q", name, "call_agent")
	}

	var desc string
	_ = json.Unmarshal(fn["description"], &desc)
	if desc != "Delegate a task to a sub-agent." {
		t.Errorf("tools[0].function.description = %q, want %q", desc, "Delegate a task to a sub-agent.")
	}

	// Verify parameters (JSON Schema)
	var params map[string]json.RawMessage
	_ = json.Unmarshal(fn["parameters"], &params)

	var paramsType string
	_ = json.Unmarshal(params["type"], &paramsType)
	if paramsType != "object" {
		t.Errorf("parameters.type = %q, want %q", paramsType, "object")
	}

	var props map[string]map[string]interface{}
	_ = json.Unmarshal(params["properties"], &props)
	if _, ok := props["agent"]; !ok {
		t.Error("parameters.properties missing 'agent'")
	}
	if _, ok := props["task"]; !ok {
		t.Error("parameters.properties missing 'task'")
	}
	if _, ok := props["context"]; !ok {
		t.Error("parameters.properties missing 'context'")
	}

	// Verify required array
	var required []string
	_ = json.Unmarshal(params["required"], &required)
	requiredMap := make(map[string]bool)
	for _, r := range required {
		requiredMap[r] = true
	}
	if !requiredMap["agent"] {
		t.Error("required does not include 'agent'")
	}
	if !requiredMap["task"] {
		t.Error("required does not include 'task'")
	}
	if requiredMap["context"] {
		t.Error("required should not include 'context'")
	}
}

func TestOpenAI_Send_WithoutTools(t *testing.T) {
	var gotBody map[string]json.RawMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"model": "gpt-4o",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	_, _ = o.Send(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Tools:    nil,
	})

	if _, ok := gotBody["tools"]; ok {
		t.Error("request body should NOT contain 'tools' key when Tools is nil")
	}
}

// --- Phase 3b: Tool-call response parsing ---

func TestOpenAI_Send_ToolCallResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := `{
			"model": "gpt-4o",
			"choices": [{
				"message": {
					"role": "assistant",
					"content": null,
					"tool_calls": [
						{
							"id": "call_abc123",
							"type": "function",
							"function": {
								"name": "call_agent",
								"arguments": "{\"agent\": \"helper\", \"task\": \"run tests\"}"
							}
						}
					]
				},
				"finish_reason": "tool_calls"
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 5}
		}`
		_, _ = w.Write([]byte(resp))
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	resp, err := o.Send(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Tools: []Tool{
			{Name: "call_agent", Description: "test", Parameters: map[string]ToolParameter{
				"agent": {Type: "string", Description: "agent", Required: true},
				"task":  {Type: "string", Description: "task", Required: true},
			}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ID != "call_abc123" {
		t.Errorf("ToolCalls[0].ID = %q, want %q", resp.ToolCalls[0].ID, "call_abc123")
	}
	if resp.ToolCalls[0].Name != "call_agent" {
		t.Errorf("ToolCalls[0].Name = %q, want %q", resp.ToolCalls[0].Name, "call_agent")
	}
	if resp.ToolCalls[0].Arguments["agent"] != "helper" {
		t.Errorf("ToolCalls[0].Arguments[agent] = %q, want %q", resp.ToolCalls[0].Arguments["agent"], "helper")
	}
	if resp.ToolCalls[0].Arguments["task"] != "run tests" {
		t.Errorf("ToolCalls[0].Arguments[task] = %q, want %q", resp.ToolCalls[0].Arguments["task"], "run tests")
	}
}

// Spec 010 R1/R7/R8: eligible top-level integers keep exact digits;
// strings and nested collections retain their legacy text.
func TestLargeIntegerArgs_OpenAI_Buffered(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "threshold integer", raw: `1000000`, want: "1000000"},
		{name: "integer beyond float64 precision", raw: `9007199254740993`, want: "9007199254740993"},
		{name: "integral exponent", raw: `1e6`, want: "1000000"},
		{name: "integral decimal", raw: `1000000.0`, want: "1000000"},
		{name: "negative integral exponent", raw: `-1e6`, want: "-1000000"},
		{name: "exact exponent beyond float64 precision", raw: `90071992547409930e-1`, want: "9007199254740993"},
		{name: "exact decimal beyond float64 precision", raw: `9007199254740993.0`, want: "9007199254740993"},
		{name: "integral decimal beyond int64", raw: `99999999999999999999999.0`, want: "99999999999999999999999"},
		{name: "integral exponent beyond int64", raw: `999999999999999999999990e-1`, want: "99999999999999999999999"},
		{name: "below threshold", raw: `999999`, want: "999999"},
		{name: "fraction", raw: `1.5`, want: "1.5"},
		{name: "boolean", raw: `true`, want: "true"},
		{name: "text", raw: `"text"`, want: "text"},
		{name: "null", raw: `null`, want: "<nil>"},
		{name: "quoted exponent", raw: `"1e6"`, want: "1e6"},
		{name: "quoted decimal", raw: `"1000000.0"`, want: "1000000.0"},
		{name: "quoted large integer", raw: `"9007199254740993"`, want: "9007199254740993"},
		{name: "nested exponent", raw: `{"a":1e6}`, want: "map[a:1e+06]"},
		{name: "nested integer", raw: `{"a":1000000}`, want: "map[a:1e+06]"},
		{name: "nested array", raw: `[1.50]`, want: "[1.5]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode only the outer arguments string, never decode the raw value
			// into float64 or quote numeric tokens inside the argument object.
			body := openAIRawArgumentBody(t, `{"model":"gpt-4o","choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"example","arguments":%s}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`, `{"n":`+tt.raw+`}`)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()

			o, err := NewOpenAI("test-key", WithOpenAIBaseURL(server.URL))
			if err != nil {
				t.Fatalf("unexpected NewOpenAI error: %v", err)
			}

			resp, err := o.Send(context.Background(), &Request{Model: "gpt-4o"})
			if err != nil {
				t.Fatalf("unexpected Send error: %v", err)
			}

			if len(resp.ToolCalls) != 1 {
				t.Fatalf("len(ToolCalls) = %d, want 1", len(resp.ToolCalls))
			}
			if got, ok := resp.ToolCalls[0].Arguments["n"]; !ok || got != tt.want {
				t.Errorf("Arguments[n] for raw %s = %q (present=%t), want %q", tt.raw, got, ok, tt.want)
			}
		})
	}
}

func TestOpenAI_Send_ToolCallNullContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := `{
			"model": "gpt-4o",
			"choices": [{
				"message": {
					"role": "assistant",
					"content": null,
					"tool_calls": [
						{
							"id": "call_1",
							"type": "function",
							"function": {
								"name": "call_agent",
								"arguments": "{\"agent\": \"a\", \"task\": \"b\"}"
							}
						}
					]
				},
				"finish_reason": "tool_calls"
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 5}
		}`
		_, _ = w.Write([]byte(resp))
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	resp, err := o.Send(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Tools: []Tool{
			{Name: "call_agent", Description: "test", Parameters: map[string]ToolParameter{}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "" {
		t.Errorf("Content = %q, want empty string", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(resp.ToolCalls))
	}
}

func TestOpenAI_Send_InvalidToolCallArguments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := `{
			"model": "gpt-4o",
			"choices": [{
				"message": {
					"role": "assistant",
					"content": null,
					"tool_calls": [
						{
							"id": "call_bad",
							"type": "function",
							"function": {
								"name": "call_agent",
								"arguments": "not valid json"
							}
						}
					]
				},
				"finish_reason": "tool_calls"
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 5}
		}`
		_, _ = w.Write([]byte(resp))
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	resp, err := o.Send(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Tools: []Tool{
			{Name: "call_agent", Description: "test", Parameters: map[string]ToolParameter{}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "call_agent" {
		t.Errorf("ToolCalls[0].Name = %q, want %q", resp.ToolCalls[0].Name, "call_agent")
	}
	if len(resp.ToolCalls[0].Arguments) != 0 {
		t.Errorf("ToolCalls[0].Arguments = %v, want empty map", resp.ToolCalls[0].Arguments)
	}
}

func TestOpenAI_Send_ToolsStopReason(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := `{
			"model": "gpt-4o",
			"choices": [{
				"message": {
					"role": "assistant",
					"content": null,
					"tool_calls": [
						{
							"id": "call_1",
							"type": "function",
							"function": {
								"name": "call_agent",
								"arguments": "{\"agent\": \"a\", \"task\": \"b\"}"
							}
						}
					]
				},
				"finish_reason": "tool_calls"
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 5}
		}`
		_, _ = w.Write([]byte(resp))
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	resp, err := o.Send(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Tools: []Tool{
			{Name: "call_agent", Description: "test", Parameters: map[string]ToolParameter{}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StopReason != "tool_calls" {
		t.Errorf("StopReason = %q, want %q", resp.StopReason, "tool_calls")
	}
}

// --- Phase 3c: Tool-result and assistant tool-call messages ---

func TestOpenAI_Send_ToolResultMessage(t *testing.T) {
	var gotBody map[string]json.RawMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"model": "gpt-4o",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	_, _ = o.Send(context.Background(), &Request{
		Model: "gpt-4o",
		Messages: []Message{
			{Role: "user", Content: "Hi"},
			{Role: "assistant", Content: "", ToolCalls: []ToolCall{
				{ID: "call_1", Name: "call_agent", Arguments: map[string]string{"agent": "helper"}},
			}},
			{Role: "tool", ToolResults: []ToolResult{
				{CallID: "call_1", Content: "result text", IsError: false},
			}},
		},
	})

	var messages []json.RawMessage
	_ = json.Unmarshal(gotBody["messages"], &messages)

	// Messages: system (if present) + user + assistant + tool result(s)
	// Find the tool messages - they should be role "tool" with tool_call_id
	// The tool result message should expand to one message per ToolResult
	found := false
	for _, raw := range messages {
		var msg map[string]interface{}
		_ = json.Unmarshal(raw, &msg)
		if msg["role"] == "tool" {
			found = true
			if msg["tool_call_id"] != "call_1" {
				t.Errorf("tool message tool_call_id = %v, want %q", msg["tool_call_id"], "call_1")
			}
			if msg["content"] != "result text" {
				t.Errorf("tool message content = %v, want %q", msg["content"], "result text")
			}
		}
	}
	if !found {
		t.Error("no tool result message found in request")
	}
}

func TestOpenAI_Send_DoesNotSetStreamField(t *testing.T) {
	var gotBody map[string]json.RawMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"model":   "gpt-4o",
			"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"}},
			"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := gotBody["stream"]; ok {
		t.Error("Send() request body should NOT contain 'stream' key")
	}
	if _, ok := gotBody["stream_options"]; ok {
		t.Error("Send() request body should NOT contain 'stream_options' key")
	}
}

// --- SendStream tests ---

func TestOpenAI_SendStream_RequestFormat(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	var gotBody map[string]json.RawMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")

		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1}}\n\ndata: [DONE]\n\n"))
	}))
	defer server.Close()

	o, _ := NewOpenAI("test-key", WithOpenAIBaseURL(server.URL))
	stream, err := o.SendStream(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	// Drain the stream
	for {
		_, err := stream.Next()
		if err != nil {
			break
		}
	}

	if gotMethod != "POST" {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("expected /chat/completions, got %s", gotPath)
	}
	if gotAuth != "Bearer test-key" {
		t.Errorf("expected 'Bearer test-key', got %q", gotAuth)
	}

	var streamVal bool
	if err := json.Unmarshal(gotBody["stream"], &streamVal); err != nil {
		t.Fatalf("failed to parse stream field: %v", err)
	}
	if !streamVal {
		t.Error("expected stream to be true")
	}

	var streamOpts map[string]interface{}
	if err := json.Unmarshal(gotBody["stream_options"], &streamOpts); err != nil {
		t.Fatalf("failed to parse stream_options: %v", err)
	}
	if streamOpts["include_usage"] != true {
		t.Errorf("expected include_usage to be true, got %v", streamOpts["include_usage"])
	}
}

func TestOpenAI_SendStream_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid API key","type":"invalid_request_error","code":"invalid_api_key"}}`))
	}))
	defer server.Close()

	o, _ := NewOpenAI("bad-key", WithOpenAIBaseURL(server.URL))
	_, err := o.SendStream(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if provErr.Category != ErrCategoryAuth {
		t.Errorf("expected ErrCategoryAuth, got %s", provErr.Category)
	}
}

func TestOpenAI_SendStream_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := o.SendStream(ctx, &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if provErr.Category != ErrCategoryTimeout {
		t.Errorf("expected ErrCategoryTimeout, got %s", provErr.Category)
	}
}

func TestOpenAI_SendStream_TextDeltas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\" world\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"!\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	stream, err := o.SendStream(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	// Event 1: text "Hello"
	ev, err := stream.Next()
	if err != nil {
		t.Fatalf("unexpected error on event 1: %v", err)
	}
	if ev.Type != StreamEventText || ev.Text != "Hello" {
		t.Errorf("event 1: got type=%q text=%q, want type=%q text=%q", ev.Type, ev.Text, StreamEventText, "Hello")
	}

	// Event 2: text " world"
	ev, err = stream.Next()
	if err != nil {
		t.Fatalf("unexpected error on event 2: %v", err)
	}
	if ev.Type != StreamEventText || ev.Text != " world" {
		t.Errorf("event 2: got type=%q text=%q, want type=%q text=%q", ev.Type, ev.Text, StreamEventText, " world")
	}

	// Event 3: text "!"
	ev, err = stream.Next()
	if err != nil {
		t.Fatalf("unexpected error on event 3: %v", err)
	}
	if ev.Type != StreamEventText || ev.Text != "!" {
		t.Errorf("event 3: got type=%q text=%q, want type=%q text=%q", ev.Type, ev.Text, StreamEventText, "!")
	}

	// Event 4: done with usage
	ev, err = stream.Next()
	if err != nil {
		t.Fatalf("unexpected error on event 4: %v", err)
	}
	if ev.Type != StreamEventDone {
		t.Errorf("event 4: got type=%q, want %q", ev.Type, StreamEventDone)
	}
	if ev.InputTokens != 10 {
		t.Errorf("event 4: InputTokens=%d, want 10", ev.InputTokens)
	}
	if ev.OutputTokens != 5 {
		t.Errorf("event 4: OutputTokens=%d, want 5", ev.OutputTokens)
	}
	if ev.StopReason != "stop" {
		t.Errorf("event 4: StopReason=%q, want %q", ev.StopReason, "stop")
	}

	// Event 5: io.EOF
	_, err = stream.Next()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestOpenAI_SendStream_ToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"read_file\",\"arguments\":\"\"}}]},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"{\\\"pa\"}}]},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"th\\\": \\\"f.go\\\"}\"}}]},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	stream, err := o.SendStream(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	// Event 1: tool start
	ev, err := stream.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != StreamEventToolStart {
		t.Errorf("event 1: type=%q, want %q", ev.Type, StreamEventToolStart)
	}
	if ev.ToolCallID != "call_1" {
		t.Errorf("event 1: ToolCallID=%q, want %q", ev.ToolCallID, "call_1")
	}
	if ev.ToolName != "read_file" {
		t.Errorf("event 1: ToolName=%q, want %q", ev.ToolName, "read_file")
	}

	// Event 2: tool delta
	ev, err = stream.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != StreamEventToolDelta {
		t.Errorf("event 2: type=%q, want %q", ev.Type, StreamEventToolDelta)
	}
	if ev.ToolCallID != "call_1" {
		t.Errorf("event 2: ToolCallID=%q, want %q", ev.ToolCallID, "call_1")
	}
	if ev.ToolInput != "{\"pa" {
		t.Errorf("event 2: ToolInput=%q, want %q", ev.ToolInput, "{\"pa")
	}

	// Event 3: tool delta
	ev, err = stream.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != StreamEventToolDelta {
		t.Errorf("event 3: type=%q, want %q", ev.Type, StreamEventToolDelta)
	}
	if ev.ToolInput != "th\": \"f.go\"}" {
		t.Errorf("event 3: ToolInput=%q, want %q", ev.ToolInput, "th\": \"f.go\"}")
	}

	// Event 4: tool end
	ev, err = stream.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != StreamEventToolEnd {
		t.Errorf("event 4: type=%q, want %q", ev.Type, StreamEventToolEnd)
	}
	if ev.ToolCallID != "call_1" {
		t.Errorf("event 4: ToolCallID=%q, want %q", ev.ToolCallID, "call_1")
	}

	// Event 5: done
	ev, err = stream.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != StreamEventDone {
		t.Errorf("event 5: type=%q, want %q", ev.Type, StreamEventDone)
	}

	// Event 6: EOF
	_, err = stream.Next()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestOpenAI_SendStream_MultipleToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_a\",\"type\":\"function\",\"function\":{\"name\":\"read_file\",\"arguments\":\"\"}}]},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":1,\"id\":\"call_b\",\"type\":\"function\",\"function\":{\"name\":\"write_file\",\"arguments\":\"\"}}]},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"{\\\"p\\\":\\\"a\\\"}\"}}]},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":1,\"function\":{\"arguments\":\"{\\\"q\\\":\\\"b\\\"}\"}}]},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	stream, err := o.SendStream(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	// Collect all events
	var events []StreamEvent
	for {
		ev, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		events = append(events, ev)
	}

	// Expected: tool_start(call_a), tool_start(call_b), tool_delta(call_a), tool_delta(call_b), tool_end(call_a), tool_end(call_b), done
	if len(events) != 7 {
		t.Fatalf("got %d events, want 7", len(events))
	}

	if events[0].Type != StreamEventToolStart || events[0].ToolCallID != "call_a" || events[0].ToolName != "read_file" {
		t.Errorf("event 0: %+v", events[0])
	}
	if events[1].Type != StreamEventToolStart || events[1].ToolCallID != "call_b" || events[1].ToolName != "write_file" {
		t.Errorf("event 1: %+v", events[1])
	}
	if events[2].Type != StreamEventToolDelta || events[2].ToolCallID != "call_a" {
		t.Errorf("event 2: %+v", events[2])
	}
	if events[3].Type != StreamEventToolDelta || events[3].ToolCallID != "call_b" {
		t.Errorf("event 3: %+v", events[3])
	}
	if events[4].Type != StreamEventToolEnd || events[4].ToolCallID != "call_a" {
		t.Errorf("event 4: %+v", events[4])
	}
	if events[5].Type != StreamEventToolEnd || events[5].ToolCallID != "call_b" {
		t.Errorf("event 5: %+v", events[5])
	}
	if events[6].Type != StreamEventDone {
		t.Errorf("event 6: %+v", events[6])
	}
}

func TestOpenAI_SendStream_EmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	stream, err := o.SendStream(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	ev, err := stream.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != StreamEventText || ev.Text != "hello" {
		t.Errorf("got type=%q text=%q, want type=%q text=%q", ev.Type, ev.Text, StreamEventText, "hello")
	}

	ev, err = stream.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != StreamEventDone {
		t.Errorf("got type=%q, want %q", ev.Type, StreamEventDone)
	}

	_, err = stream.Next()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestOpenAI_SendStream_NoUsageChunk(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	stream, err := o.SendStream(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	ev, err := stream.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != StreamEventText {
		t.Errorf("got type=%q, want %q", ev.Type, StreamEventText)
	}

	ev, err = stream.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != StreamEventDone {
		t.Errorf("got type=%q, want %q", ev.Type, StreamEventDone)
	}
	if ev.InputTokens != 0 || ev.OutputTokens != 0 {
		t.Errorf("expected zero tokens, got input=%d output=%d", ev.InputTokens, ev.OutputTokens)
	}
	if ev.StopReason != "stop" {
		t.Errorf("StopReason=%q, want %q", ev.StopReason, "stop")
	}

	_, err = stream.Next()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestOpenAI_SendStream_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {not json}\n\n"))
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	stream, err := o.SendStream(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	_, err = stream.Next()
	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if provErr.Category != ErrCategoryServer {
		t.Errorf("expected ErrCategoryServer, got %s", provErr.Category)
	}
}

func TestOpenAI_SendStream_FinishReasonLength(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"length\"}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":100}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	stream, err := o.SendStream(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	// Skip text event
	_, _ = stream.Next()

	ev, err := stream.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != StreamEventDone {
		t.Errorf("got type=%q, want %q", ev.Type, StreamEventDone)
	}
	if ev.StopReason != "length" {
		t.Errorf("StopReason=%q, want %q", ev.StopReason, "length")
	}
}

func TestOpenAI_Send_AssistantToolCallMessage(t *testing.T) {
	var gotBody map[string]json.RawMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"model": "gpt-4o",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	_, _ = o.Send(context.Background(), &Request{
		Model: "gpt-4o",
		Messages: []Message{
			{Role: "user", Content: "Hi"},
			{Role: "assistant", Content: "I'll help", ToolCalls: []ToolCall{
				{ID: "call_1", Name: "call_agent", Arguments: map[string]string{"agent": "helper", "task": "work"}},
			}},
			{Role: "tool", ToolResults: []ToolResult{
				{CallID: "call_1", Content: "done", IsError: false},
			}},
		},
	})

	var messages []json.RawMessage
	_ = json.Unmarshal(gotBody["messages"], &messages)

	// Find assistant message with tool_calls
	found := false
	for _, raw := range messages {
		var msg map[string]json.RawMessage
		_ = json.Unmarshal(raw, &msg)

		var role string
		_ = json.Unmarshal(msg["role"], &role)

		if role == "assistant" {
			if _, ok := msg["tool_calls"]; !ok {
				t.Error("assistant message missing 'tool_calls' field")
				continue
			}
			found = true

			var toolCalls []map[string]json.RawMessage
			_ = json.Unmarshal(msg["tool_calls"], &toolCalls)
			if len(toolCalls) != 1 {
				t.Fatalf("len(tool_calls) = %d, want 1", len(toolCalls))
			}

			var id string
			_ = json.Unmarshal(toolCalls[0]["id"], &id)
			if id != "call_1" {
				t.Errorf("tool_calls[0].id = %q, want %q", id, "call_1")
			}

			var tcType string
			_ = json.Unmarshal(toolCalls[0]["type"], &tcType)
			if tcType != "function" {
				t.Errorf("tool_calls[0].type = %q, want %q", tcType, "function")
			}

			var fn map[string]json.RawMessage
			_ = json.Unmarshal(toolCalls[0]["function"], &fn)

			var fnName string
			_ = json.Unmarshal(fn["name"], &fnName)
			if fnName != "call_agent" {
				t.Errorf("tool_calls[0].function.name = %q, want %q", fnName, "call_agent")
			}

			// Arguments should be a JSON string
			var argsStr string
			_ = json.Unmarshal(fn["arguments"], &argsStr)
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
				t.Fatalf("failed to parse function.arguments: %v", err)
			}
			if args["agent"] != "helper" {
				t.Errorf("arguments.agent = %v, want %q", args["agent"], "helper")
			}
			if args["task"] != "work" {
				t.Errorf("arguments.task = %v, want %q", args["task"], "work")
			}
		}
	}
	if !found {
		t.Error("no assistant message with tool_calls found in request")
	}
}

func TestOpenAI_Send_OmitsResponseFormatWhenEmpty(t *testing.T) {
	var hasResponseFormat bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		_ = json.Unmarshal(body, &raw)
		_, hasResponseFormat = raw["response_format"]

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"model":   "gpt-4o",
			"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"}},
			"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenAI("test-key", WithOpenAIBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasResponseFormat {
		t.Error("expected response_format to be omitted when not set")
	}
}

func TestOpenAI_Send_IncludesResponseFormatJsonObject(t *testing.T) {
	var gotResponseFormat map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		_ = json.Unmarshal(body, &raw)
		if rf, ok := raw["response_format"]; ok {
			_ = json.Unmarshal(rf, &gotResponseFormat)
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"model":   "gpt-4o",
			"choices": []map[string]interface{}{{"message": map[string]string{"content": `{"result":"ok"}`}, "finish_reason": "stop"}},
			"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenAI("test-key", WithOpenAIBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:          "gpt-4o",
		Messages:       []Message{{Role: "user", Content: "Hi"}},
		ResponseFormat: ResponseFormat{Type: "json_object"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotResponseFormat == nil {
		t.Fatal("expected response_format to be present")
	}
	if gotResponseFormat["type"] != "json_object" {
		t.Errorf("expected type 'json_object', got %v", gotResponseFormat["type"])
	}
}

func TestOpenAI_Send_IncludesResponseFormatJsonSchema(t *testing.T) {
	var gotResponseFormat map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		_ = json.Unmarshal(body, &raw)
		if rf, ok := raw["response_format"]; ok {
			_ = json.Unmarshal(rf, &gotResponseFormat)
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"model":   "gpt-4o",
			"choices": []map[string]interface{}{{"message": map[string]string{"content": `{"name":"test"}`}, "finish_reason": "stop"}},
			"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenAI("test-key", WithOpenAIBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
		ResponseFormat: ResponseFormat{
			Type: "json_schema",
			Schema: map[string]interface{}{
				"name": "leadership",
				"type": "object",
				"properties": map[string]interface{}{
					"pastor_name": map[string]interface{}{"type": "string"},
				},
				"required": []string{"pastor_name"},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotResponseFormat == nil {
		t.Fatal("expected response_format to be present")
	}
	if gotResponseFormat["type"] != "json_schema" {
		t.Errorf("expected type 'json_schema', got %v", gotResponseFormat["type"])
	}
	jsonSchema, ok := gotResponseFormat["json_schema"].(map[string]interface{})
	if !ok {
		t.Fatal("expected json_schema object in response_format")
	}
	if jsonSchema["name"] != "leadership" {
		t.Errorf("expected schema name 'leadership', got %v", jsonSchema["name"])
	}
	if jsonSchema["strict"] != true {
		t.Errorf("expected strict=true, got %v", jsonSchema["strict"])
	}
	schema, ok := jsonSchema["schema"].(map[string]interface{})
	if !ok {
		t.Fatal("expected schema object inside json_schema")
	}
	if schema["type"] != "object" {
		t.Errorf("expected schema type 'object', got %v", schema["type"])
	}
}

func TestOpenAI_Send_JsonSchemaWithoutSchemaReturnsError(t *testing.T) {
	o, _ := NewOpenAI("test-key", WithOpenAIBaseURL("http://localhost:1"))
	_, err := o.Send(context.Background(), &Request{
		Model:          "gpt-4o",
		Messages:       []Message{{Role: "user", Content: "Hi"}},
		ResponseFormat: ResponseFormat{Type: "json_schema"},
	})
	if err == nil {
		t.Fatal("expected error for json_schema without schema, got nil")
	}
	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if provErr.Category != ErrCategoryBadRequest {
		t.Errorf("expected ErrCategoryBadRequest, got %s", provErr.Category)
	}
}

func TestOpenAI_CacheUsage(t *testing.T) {
	cases := []struct {
		name            string
		isStream        bool
		responsePayload []byte
		wantCacheRead   int
	}{
		{
			name: "send with cached_tokens",
			responsePayload: []byte(`{
				"model": "gpt-4o",
				"choices": [{"message": {"content": "Hello from cached prompt"}, "finish_reason": "stop"}],
				"usage": {"prompt_tokens": 100, "completion_tokens": 5, "prompt_tokens_details": {"cached_tokens": 80}}
			}`),
			wantCacheRead: 80,
		},
		{
			name: "send without cached_tokens",
			responsePayload: []byte(`{
				"model": "gpt-4o",
				"choices": [{"message": {"content": "Hello"}, "finish_reason": "stop"}],
				"usage": {"prompt_tokens": 10, "completion_tokens": 5}
			}`),
			wantCacheRead: 0,
		},
		{
			name:     "stream with cached_tokens",
			isStream: true,
			responsePayload: []byte(
				"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
					"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":100,\"completion_tokens\":5,\"prompt_tokens_details\":{\"cached_tokens\":60}}}\n\n" +
					"data: [DONE]\n\n"),
			wantCacheRead: 60,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.isStream {
					w.Header().Set("Content-Type", "text/event-stream")
				}
				_, _ = w.Write(tc.responsePayload)
			}))
			defer server.Close()

			o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
			req := &Request{Model: "gpt-4o", Messages: []Message{{Role: "user", Content: "Hi"}}}

			var got int
			if tc.isStream {
				stream, err := o.SendStream(context.Background(), req)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				defer func() { _ = stream.Close() }()
				// Read the first (and only) event with usage
				ev, err := stream.Next()
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if ev.Type != StreamEventDone {
					t.Fatalf("expected StreamEventDone, got %q", ev.Type)
				}
				got = ev.CacheReadTokens
			} else {
				resp, err := o.Send(context.Background(), req)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				got = resp.CacheReadTokens
			}

			if got != tc.wantCacheRead {
				t.Errorf("CacheReadTokens = %d, want %d", got, tc.wantCacheRead)
			}
		})
	}
}

func TestLargeIntegerArgs_OpenAI_DecodeErrors(t *testing.T) {
	// Only the outer string is encoded; numeric argument tokens stay raw.
	const envelope = `{"model":"gpt-4o","choices":[{"message":{"tool_calls":[{"id":"call_1","type":"function","function":{"name":"example","arguments":%s}}]},"finish_reason":"tool_calls"}]}`
	base := openAIRawArgumentBody(t, envelope, `{"n":1000000}`)
	type decodeErrorCase struct {
		name    string
		body    string
		wantErr bool
		want    map[string]string
	}
	tests := []decodeErrorCase{
		{name: "outer malformed", body: `{"choices":`, wantErr: true},
		{name: "outer trailing value", body: base + `{}`, wantErr: true},
		{name: "outer trailing junk", body: base + `junk`, wantErr: true},
		{name: "known integer overflow", body: `{"choices":[{}],"usage":{"prompt_tokens":1e400}}`, wantErr: true},
		{name: "known 400 digit overflow", body: `{"choices":[{}],"usage":{"completion_tokens":` + strings.Repeat("9", 400) + `}}`, wantErr: true},
		{name: "known nested overflow", body: `{"choices":[{}],"usage":{"prompt_tokens_details":{"cached_tokens":1e400}}}`, wantErr: true},
		{name: "overwritten known scalar", body: `{"choices":[{}],"usage":{"prompt_tokens":1e400,"prompt_tokens":1}}`, wantErr: true},
		{name: "overwritten known subtree", body: `{"choices":[{}],"usage":{"prompt_tokens":1e400},"usage":{"prompt_tokens":1}}`, wantErr: true},
		{name: "outer type mismatch", body: `{"choices":[{}],"usage":{"prompt_tokens":true}}`, wantErr: true},
		{name: "unknown scalar overflow", body: `{"unknown":1e400,` + base[1:], want: map[string]string{"n": "1000000"}},
		{name: "unknown subtree overflow", body: `{"unknown":{"bad":1e400},` + base[1:], want: map[string]string{"n": "1000000"}},
		{name: "unknown 400 digits", body: `{"unknown":` + strings.Repeat("9", 400) + `,` + base[1:], want: map[string]string{"n": "1000000"}},
		{name: "successful outer duplicate", body: `{"usage":{"prompt_tokens":1,"prompt_tokens":2},` + base[1:], want: map[string]string{"n": "1000000"}},
		{name: "successful outer subtree", body: `{"usage":{"prompt_tokens":1},"usage":{"completion_tokens":2},` + base[1:], want: map[string]string{"n": "1000000"}},
	}
	for _, tt := range []struct {
		name string
		raw  string
		want map[string]string
	}{
		{name: "inner malformed", raw: `{"n":`},
		{name: "inner trailing value", raw: `{"n":1} {}`},
		{name: "inner trailing junk", raw: `{"n":1} junk`},
		{name: "inner overflow", raw: `{"n":1e400}`},
		{name: "inner 400 digits", raw: `{"n":` + strings.Repeat("9", 400) + `}`},
		{name: "inner nested overflow", raw: `{"n":[{"bad":1e400}]}`},
		{name: "inner nested 400 digits", raw: `{"n":{"bad":` + strings.Repeat("9", 400) + `}}`},
		{name: "inner overwritten scalar overflow", raw: `{"n":1e400,"n":1000000}`},
		{name: "inner overwritten subtree overflow", raw: `{"n":{"bad":1e400},"n":1000000}`},
		{name: "inner successful scalar duplicate", raw: `{"n":999999,"n":1000000}`, want: map[string]string{"n": "1000000"}},
		{name: "inner successful subtree duplicate", raw: `{"n":{"ok":1},"n":1000000}`, want: map[string]string{"n": "1000000"}},
		{name: "inner successful map replacement", raw: `{"n":{"old":1},"n":{"new":2}}`, want: map[string]string{"n": "map[new:2]"}},
		{name: "inner empty", raw: `{}`},
		{name: "inner null", raw: `null`},
		{name: "inner array mismatch", raw: `[]`},
		{name: "inner scalar mismatch", raw: `0`},
		{name: "inner negative zero mismatch", raw: `-0`},
		{name: "inner decimal zero mismatch", raw: `-0.0`},
		{name: "inner boolean mismatch", raw: `true`},
		{name: "inner string mismatch", raw: `"text"`},
	} {
		tests = append(tests, decodeErrorCase{name: tt.name, body: openAIRawArgumentBody(t, envelope, tt.raw), want: tt.want})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			o, err := NewOpenAI("test-key", WithOpenAIBaseURL(server.URL))
			if err != nil {
				t.Fatal(err)
			}
			resp, err := o.Send(context.Background(), &Request{Model: "gpt-4o"})
			if tt.wantErr {
				var providerErr *ProviderError
				if !errors.As(err, &providerErr) || providerErr.Category != ErrCategoryServer {
					t.Fatalf("Send error = %v, want server ProviderError", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(resp.ToolCalls) != 1 {
				t.Fatalf("ToolCalls = %#v, want one call", resp.ToolCalls)
			}
			want := tt.want
			if want == nil {
				want = map[string]string{}
			}
			if !reflect.DeepEqual(resp.ToolCalls[0].Arguments, want) {
				t.Errorf("Arguments = %#v, want %#v", resp.ToolCalls[0].Arguments, want)
			}
		})
	}
}

func openAIRawArgumentBody(t *testing.T, envelope, raw string) string {
	t.Helper()
	quoted, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("failed to encode arguments string: %v", err)
	}
	return strings.Replace(envelope, "%s", string(quoted), 1)
}

func TestLargeIntegerArgs_OpenAI_Replay(t *testing.T) {
	const raw = `{"million":1000000,"precise":9007199254740993,"exponent":1e6,"decimal":1000000.0,"negative":-1e6,"precise_decimal":9007199254740993.0,"precise_exponent":90071992547409930e-1,"beyond":99999999999999999999999,"beyond_decimal":99999999999999999999999.0,"beyond_exponent":999999999999999999999990e-1,"small":999999,"negative_small":-999999,"small_decimal":999999.0,"zero":0,"negative_zero":-0,"negative_decimal_zero":-0.0,"fraction":1.5,"rounded_fraction":1000000.00000000001,"rounded_threshold":999999.99999999999,"text":"text","string_exponent":"1e6","string_decimal":"1000000.0","string_precise":"9007199254740993","boolean":true,"null":null,"object_exponent":{"a":1e6},"object_integer":{"a":1000000},"array":[1.50],"recursive":{"a":[1000000,1.50]}}`
	want := map[string]string{
		"million": "1000000", "precise": "9007199254740993",
		"exponent": "1000000", "decimal": "1000000", "negative": "-1000000",
		"precise_decimal": "9007199254740993", "precise_exponent": "9007199254740993",
		"beyond": "99999999999999999999999", "beyond_decimal": "99999999999999999999999", "beyond_exponent": "99999999999999999999999",
		"small": "999999", "negative_small": "-999999", "small_decimal": "999999",
		"zero": "0", "negative_zero": "-0", "negative_decimal_zero": "-0",
		"fraction": "1.5", "rounded_fraction": "1e+06", "rounded_threshold": "1e+06",
		"text": "text", "string_exponent": "1e6", "string_decimal": "1000000.0", "string_precise": "9007199254740993",
		"boolean": "true", "null": "<nil>", "object_exponent": "map[a:1e+06]", "object_integer": "map[a:1e+06]",
		"array": "[1.5]", "recursive": "map[a:[1e+06 1.5]]",
	}
	body := openAIRawArgumentBody(t, `{"model":"gpt-4o","choices":[{"message":{"content":"Read values","tool_calls":[{"id":"call_1","type":"function","function":{"name":"example","arguments":%s}}]},"finish_reason":"tool_calls"}]}`, raw)
	first := make(chan struct{}, 1)
	first <- struct{}{}
	requests := make(chan openaiRequest, 3)
	replayed := make(chan map[string]string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openaiRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- req
		w.Header().Set("Content-Type", "application/json")
		select {
		case <-first:
			_, _ = io.WriteString(w, body)
		default:
			if len(req.Messages) != 3 || len(req.Messages[1].ToolCalls) != 1 {
				t.Errorf("follow-up messages = %+v, want user, assistant call, tool result", req.Messages)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			// Decoding into strings rejects replayed JSON numbers, proving OS5.
			var args map[string]string
			if err := json.Unmarshal([]byte(req.Messages[1].ToolCalls[0].Function.Arguments), &args); err != nil {
				t.Errorf("decode replay arguments as strings: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			replayed <- args
			_, _ = io.WriteString(w, `{"model":"gpt-4o","choices":[{"message":{"content":"All done"},"finish_reason":"stop"}]}`)
		}
	}))
	defer server.Close()
	o, err := NewOpenAI("test-key", WithOpenAIBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	seed := Message{Role: "user", Content: "Keep this history"}
	resp, err := o.Send(context.Background(), &Request{Model: "gpt-4o", Messages: []Message{seed}})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ToolCalls) != 1 || !reflect.DeepEqual(resp.ToolCalls[0].Arguments, want) {
		t.Fatalf("decoded ToolCalls = %+v, want literal arguments %v", resp.ToolCalls, want)
	}
	final, err := o.Send(context.Background(), &Request{Model: "gpt-4o", Messages: []Message{
		seed,
		{Role: "assistant", Content: resp.Content, ToolCalls: resp.ToolCalls},
		{Role: "tool", ToolResults: []ToolResult{{CallID: resp.ToolCalls[0].ID, Content: "controlled result"}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if final.Content != "All done" {
		t.Errorf("final content = %q, want All done", final.Content)
	}
	if len(requests) != 2 || len(replayed) != 1 {
		t.Fatalf("requests = %d, replay captures = %d, want 2 and 1", len(requests), len(replayed))
	}
	if got := <-replayed; !reflect.DeepEqual(got, want) {
		t.Errorf("replayed arguments = %v, want %v", got, want)
	}
	<-requests
	followup := <-requests
	call := followup.Messages[1].ToolCalls[0]
	if call.ID != "call_1" || call.Type != "function" || call.Function.Name != "example" {
		t.Errorf("replayed call = %+v", call)
	}
	toolResult := followup.Messages[2]
	if toolResult.Role != "tool" || toolResult.ToolCallID != "call_1" || toolResult.Content == nil || *toolResult.Content != "controlled result" {
		t.Errorf("replayed tool result = %+v", toolResult)
	}
	for _, key := range []string{"million", "precise", "exponent", "decimal", "negative", "precise_decimal", "precise_exponent", "beyond", "beyond_decimal", "beyond_exponent"} {
		if strings.ContainsAny(want[key], "eE") || !strings.Contains(call.Function.Arguments, `"`+key+`":"`+want[key]+`"`) {
			t.Errorf("replay lacks quoted exact digits for %s: %s", key, call.Function.Arguments)
		}
	}
}
