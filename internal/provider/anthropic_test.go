package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// --- Phase 5a: Constructor tests ---

func TestNewAnthropic_EmptyAPIKey(t *testing.T) {
	_, err := NewAnthropic("")
	if err == nil {
		t.Fatal("expected error for empty API key, got nil")
	}
}

func TestNewAnthropic_ValidAPIKey(t *testing.T) {
	a, err := NewAnthropic("test-key-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil *Anthropic, got nil")
	}
}

// --- Phase 5b: Send method - Success path tests ---

func TestAnthropic_Send_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": "Hello from Claude"},
			},
			"model": "claude-sonnet-4-20250514",
			"usage": map[string]int{
				"input_tokens":  10,
				"output_tokens": 5,
			},
			"stop_reason": "end_turn",
		})
	}))
	defer server.Close()

	a, err := NewAnthropic("test-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := a.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
		System:   "You are helpful.",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "Hello from Claude" {
		t.Errorf("Content = %q, want %q", resp.Content, "Hello from Claude")
	}
	if resp.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want %q", resp.Model, "claude-sonnet-4-20250514")
	}
	if resp.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want 10", resp.InputTokens)
	}
	if resp.OutputTokens != 5 {
		t.Errorf("OutputTokens = %d, want 5", resp.OutputTokens)
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("StopReason = %q, want %q", resp.StopReason, "end_turn")
	}
}

func TestAnthropic_Send_RequestFormat(t *testing.T) {
	var (
		gotMethod  string
		gotPath    string
		gotHeaders http.Header
		gotBody    map[string]interface{}
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotHeaders = r.Header

		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content":     []map[string]string{{"type": "text", "text": "ok"}},
			"model":       "claude-sonnet-4-20250514",
			"usage":       map[string]int{"input_tokens": 1, "output_tokens": 1},
			"stop_reason": "end_turn",
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("my-api-key", WithBaseURL(server.URL))
	a.Send(context.Background(), &Request{
		Model:       "claude-sonnet-4-20250514",
		System:      "Be helpful",
		Messages:    []Message{{Role: "user", Content: "Hi"}},
		Temperature: 0.7,
		MaxTokens:   1024,
	})

	if gotMethod != "POST" {
		t.Errorf("Method = %q, want POST", gotMethod)
	}
	if gotPath != "/v1/messages" {
		t.Errorf("Path = %q, want /v1/messages", gotPath)
	}
	if gotHeaders.Get("x-api-key") != "my-api-key" {
		t.Errorf("x-api-key = %q, want %q", gotHeaders.Get("x-api-key"), "my-api-key")
	}
	if gotHeaders.Get("anthropic-version") != "2023-06-01" {
		t.Errorf("anthropic-version = %q, want %q", gotHeaders.Get("anthropic-version"), "2023-06-01")
	}
	if gotHeaders.Get("content-type") != "application/json" {
		t.Errorf("content-type = %q, want %q", gotHeaders.Get("content-type"), "application/json")
	}

	if gotBody["model"] != "claude-sonnet-4-20250514" {
		t.Errorf("body model = %v, want %q", gotBody["model"], "claude-sonnet-4-20250514")
	}
	if gotBody["system"] != "Be helpful" {
		t.Errorf("body system = %v, want %q", gotBody["system"], "Be helpful")
	}
	// JSON numbers decode as float64
	if gotBody["max_tokens"] != float64(1024) {
		t.Errorf("body max_tokens = %v, want 1024", gotBody["max_tokens"])
	}
	if gotBody["temperature"] != float64(0.7) {
		t.Errorf("body temperature = %v, want 0.7", gotBody["temperature"])
	}

	msgs, ok := gotBody["messages"].([]interface{})
	if !ok || len(msgs) != 1 {
		t.Fatalf("body messages = %v, want 1-element array", gotBody["messages"])
	}
	msg := msgs[0].(map[string]interface{})
	if msg["role"] != "user" || msg["content"] != "Hi" {
		t.Errorf("body messages[0] = %v, want role=user content=Hi", msg)
	}
}

func TestAnthropic_Send_OmitsEmptySystem(t *testing.T) {
	var gotBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content":     []map[string]string{{"type": "text", "text": "ok"}},
			"model":       "claude-sonnet-4-20250514",
			"usage":       map[string]int{"input_tokens": 1, "output_tokens": 1},
			"stop_reason": "end_turn",
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	a.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
		System:   "", // empty
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	if _, exists := gotBody["system"]; exists {
		t.Errorf("body contains 'system' key when System is empty: %v", gotBody["system"])
	}
}

func TestAnthropic_Send_OmitsZeroTemperature(t *testing.T) {
	var gotBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content":     []map[string]string{{"type": "text", "text": "ok"}},
			"model":       "claude-sonnet-4-20250514",
			"usage":       map[string]int{"input_tokens": 1, "output_tokens": 1},
			"stop_reason": "end_turn",
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	a.Send(context.Background(), &Request{
		Model:       "claude-sonnet-4-20250514",
		Temperature: 0, // zero
		Messages:    []Message{{Role: "user", Content: "Hi"}},
	})

	if _, exists := gotBody["temperature"]; exists {
		t.Errorf("body contains 'temperature' key when Temperature is 0: %v", gotBody["temperature"])
	}
}

func TestAnthropic_Send_DefaultMaxTokens(t *testing.T) {
	var gotBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content":     []map[string]string{{"type": "text", "text": "ok"}},
			"model":       "claude-sonnet-4-20250514",
			"usage":       map[string]int{"input_tokens": 1, "output_tokens": 1},
			"stop_reason": "end_turn",
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	a.Send(context.Background(), &Request{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 0, // zero → default to 4096
		Messages:  []Message{{Role: "user", Content: "Hi"}},
	})

	if gotBody["max_tokens"] != float64(defaultMaxTokens) {
		t.Errorf("body max_tokens = %v, want %d", gotBody["max_tokens"], defaultMaxTokens)
	}
}

// --- Phase 5c: Send method - Error handling tests ---

func TestAnthropic_Send_EmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content":     []map[string]string{},
			"model":       "claude-sonnet-4-20250514",
			"usage":       map[string]int{"input_tokens": 1, "output_tokens": 0},
			"stop_reason": "end_turn",
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	_, err := a.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if pe.Category != ErrCategoryServer {
		t.Errorf("Category = %q, want %q", pe.Category, ErrCategoryServer)
	}
}

func TestAnthropic_Send_AuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"type": "error",
			"error": map[string]string{
				"type":    "authentication_error",
				"message": "invalid x-api-key",
			},
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("bad-key", WithBaseURL(server.URL))
	_, err := a.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if pe.Category != ErrCategoryAuth {
		t.Errorf("Category = %q, want %q", pe.Category, ErrCategoryAuth)
	}
}

func TestAnthropic_Send_RateLimitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"type": "error",
			"error": map[string]string{
				"type":    "rate_limit_error",
				"message": "rate limited",
			},
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	_, err := a.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if pe.Category != ErrCategoryRateLimit {
		t.Errorf("Category = %q, want %q", pe.Category, ErrCategoryRateLimit)
	}
}

func TestAnthropic_Send_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"type": "error",
			"error": map[string]string{
				"type":    "api_error",
				"message": "internal server error",
			},
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	_, err := a.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if pe.Category != ErrCategoryServer {
		t.Errorf("Category = %q, want %q", pe.Category, ErrCategoryServer)
	}
}

func TestAnthropic_Send_OverloadedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(529)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"type": "error",
			"error": map[string]string{
				"type":    "overloaded_error",
				"message": "API is overloaded",
			},
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	_, err := a.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if pe.Category != ErrCategoryOverloaded {
		t.Errorf("Category = %q, want %q", pe.Category, ErrCategoryOverloaded)
	}
}

func TestAnthropic_Send_BadRequestError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"type": "error",
			"error": map[string]string{
				"type":    "invalid_request_error",
				"message": "model not found",
			},
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	_, err := a.Send(context.Background(), &Request{
		Model:    "invalid-model",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if pe.Category != ErrCategoryBadRequest {
		t.Errorf("Category = %q, want %q", pe.Category, ErrCategoryBadRequest)
	}
}

func TestAnthropic_Send_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(200)
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := a.Send(ctx, &Request{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if pe.Category != ErrCategoryTimeout {
		t.Errorf("Category = %q, want %q", pe.Category, ErrCategoryTimeout)
	}
}

func TestAnthropic_Send_ErrorResponseParsing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"type": "error",
			"error": map[string]string{
				"type":    "invalid_request_error",
				"message": "model: field is required",
			},
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	_, err := a.Send(context.Background(), &Request{
		Model:    "bad",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if pe.Message != "model: field is required" {
		t.Errorf("Message = %q, want %q", pe.Message, "model: field is required")
	}
}
