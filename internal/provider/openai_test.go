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
		json.NewEncoder(w).Encode(map[string]interface{}{
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
		json.Unmarshal(body, &req)

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

		json.NewEncoder(w).Encode(map[string]interface{}{
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
	if gotPath != "/v1/chat/completions" {
		t.Errorf("expected /v1/chat/completions, got %s", gotPath)
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

func TestOpenAI_Send_OmitsEmptySystem(t *testing.T) {
	var gotMsgCount int
	var gotFirstRole string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		json.Unmarshal(body, &req)

		if msgs, ok := req["messages"].([]interface{}); ok {
			gotMsgCount = len(msgs)
			if len(msgs) >= 1 {
				if first, ok := msgs[0].(map[string]interface{}); ok {
					gotFirstRole, _ = first["role"].(string)
				}
			}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
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

func TestOpenAI_Send_OmitsZeroTemperature(t *testing.T) {
	var hasTemperature bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		json.Unmarshal(body, &raw)
		_, hasTemperature = raw["temperature"]

		json.NewEncoder(w).Encode(map[string]interface{}{
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
		t.Error("expected temperature to be omitted when 0")
	}
}

func TestOpenAI_Send_OmitsZeroMaxTokens(t *testing.T) {
	var hasMaxTokens bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		json.Unmarshal(body, &raw)
		_, hasMaxTokens = raw["max_tokens"]

		json.NewEncoder(w).Encode(map[string]interface{}{
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
		t.Error("expected max_tokens to be omitted when 0")
	}
}

func TestOpenAI_Send_IncludesMaxTokens(t *testing.T) {
	var hasMaxTokens bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		json.Unmarshal(body, &raw)
		_, hasMaxTokens = raw["max_tokens"]

		json.NewEncoder(w).Encode(map[string]interface{}{
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
		t.Error("expected max_tokens to be present")
	}
}

func TestOpenAI_Send_AuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"error":{"message":"Invalid API key","type":"invalid_request_error","code":"invalid_api_key"}}`))
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
		json.NewEncoder(w).Encode(map[string]interface{}{
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
		w.Write([]byte(`{"error":{"message":"Invalid model specified","type":"invalid_request_error","code":null}}`))
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
		w.Write([]byte("not json"))
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
