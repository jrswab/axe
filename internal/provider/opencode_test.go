package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Constructor Tests
// ---------------------------------------------------------------------------

func TestNewOpenCode_EmptyAPIKey(t *testing.T) {
	_, err := NewOpenCode("")
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
	if !strings.Contains(err.Error(), "API key is required") {
		t.Errorf("expected 'API key is required', got %q", err.Error())
	}
}

func TestNewOpenCode_ValidAPIKey(t *testing.T) {
	p, err := NewOpenCode("zen-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
		return
	}
	if p.baseURL != defaultOpenCodeBaseURL {
		t.Errorf("expected baseURL %q, got %q", defaultOpenCodeBaseURL, p.baseURL)
	}
}

func TestNewOpenCode_WithBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a minimal valid chat completions response.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ocChatResponse{
			Model: "kimi-k2",
			Choices: []struct {
				FinishReason string `json:"finish_reason"`
				Message      struct {
					Content   *string          `json:"content"`
					ToolCalls []ocToolCallWire `json:"tool_calls"`
				} `json:"message"`
			}{
				{FinishReason: "stop", Message: struct {
					Content   *string          `json:"content"`
					ToolCalls []ocToolCallWire `json:"tool_calls"`
				}{Content: strPtr("hello")}},
			},
		})
	}))
	defer srv.Close()

	p, err := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := p.Send(context.Background(), &Request{Model: "kimi-k2", Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "hello" {
		t.Errorf("expected 'hello', got %q", resp.Content)
	}
}

// ---------------------------------------------------------------------------
// Anthropic Messages Format Tests (claude-* models)
// ---------------------------------------------------------------------------

func makeAnthropicSuccessResponse(content, model string) string {
	resp := map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": content},
		},
		"model":       model,
		"stop_reason": "end_turn",
		"usage": map[string]int{
			"input_tokens":  10,
			"output_tokens": 5,
		},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func TestOpenCode_Send_Claude_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(makeAnthropicSuccessResponse("Hello, world!", "claude-sonnet-4-6")))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	resp, err := p.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-6",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Hello, world!" {
		t.Errorf("expected 'Hello, world!', got %q", resp.Content)
	}
	if resp.Model != "claude-sonnet-4-6" {
		t.Errorf("expected model 'claude-sonnet-4-6', got %q", resp.Model)
	}
	if resp.InputTokens != 10 {
		t.Errorf("expected InputTokens 10, got %d", resp.InputTokens)
	}
	if resp.OutputTokens != 5 {
		t.Errorf("expected OutputTokens 5, got %d", resp.OutputTokens)
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("expected StopReason 'end_turn', got %q", resp.StopReason)
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("expected no tool calls, got %d", len(resp.ToolCalls))
	}
}

func TestOpenCode_Send_Claude_RequestFormat(t *testing.T) {
	var capturedReq *http.Request
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		var err error
		capturedBody, err = readBody(r)
		if err != nil {
			t.Errorf("failed to read body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(makeAnthropicSuccessResponse("ok", "claude-sonnet-4-6")))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("test-api-key", WithOpenCodeBaseURL(srv.URL))
	_, err := p.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-6",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedReq.Method != http.MethodPost {
		t.Errorf("expected POST, got %q", capturedReq.Method)
	}
	if capturedReq.URL.Path != "/v1/messages" {
		t.Errorf("expected path '/v1/messages', got %q", capturedReq.URL.Path)
	}
	if got := capturedReq.Header.Get("Authorization"); got != "Bearer test-api-key" {
		t.Errorf("expected 'Bearer test-api-key', got %q", got)
	}
	if got := capturedReq.Header.Get("anthropic-version"); got != "2023-06-01" {
		t.Errorf("expected anthropic-version '2023-06-01', got %q", got)
	}
	if got := capturedReq.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", got)
	}
	if !strings.Contains(string(capturedBody), `"model":"claude-sonnet-4-6"`) {
		t.Errorf("expected body to contain model field, got %s", capturedBody)
	}
	if !strings.Contains(string(capturedBody), `"max_tokens":4096`) {
		t.Errorf("expected body to contain max_tokens 4096, got %s", capturedBody)
	}
}

func TestOpenCode_Send_Claude_SystemPrompt(t *testing.T) {
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = readBody(r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(makeAnthropicSuccessResponse("ok", "claude-sonnet-4-6")))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))

	// With system prompt.
	_, _ = p.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-6",
		System:   "You are helpful.",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if !strings.Contains(string(capturedBody), `"system":"You are helpful."`) {
		t.Errorf("expected system field present, got %s", capturedBody)
	}

	// Without system prompt.
	_, _ = p.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-6",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if strings.Contains(string(capturedBody), `"system"`) {
		t.Errorf("expected system field absent, got %s", capturedBody)
	}
}

func TestOpenCode_Send_Claude_DefaultMaxTokens(t *testing.T) {
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = readBody(r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(makeAnthropicSuccessResponse("ok", "claude-sonnet-4-6")))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, _ = p.Send(context.Background(), &Request{
		Model:     "claude-sonnet-4-6",
		MaxTokens: 0,
		Messages:  []Message{{Role: "user", Content: "hi"}},
	})
	if !strings.Contains(string(capturedBody), `"max_tokens":4096`) {
		t.Errorf("expected max_tokens 4096, got %s", capturedBody)
	}
}

func TestOpenCode_Send_Claude_CustomMaxTokens(t *testing.T) {
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = readBody(r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(makeAnthropicSuccessResponse("ok", "claude-sonnet-4-6")))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, _ = p.Send(context.Background(), &Request{
		Model:     "claude-sonnet-4-6",
		MaxTokens: 1000,
		Messages:  []Message{{Role: "user", Content: "hi"}},
	})
	if !strings.Contains(string(capturedBody), `"max_tokens":1000`) {
		t.Errorf("expected max_tokens 1000, got %s", capturedBody)
	}
}

func TestOpenCode_Send_Claude_OmitsZeroTemperature(t *testing.T) {
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = readBody(r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(makeAnthropicSuccessResponse("ok", "claude-sonnet-4-6")))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, _ = p.Send(context.Background(), &Request{
		Model:       "claude-sonnet-4-6",
		Temperature: 0,
		Messages:    []Message{{Role: "user", Content: "hi"}},
	})
	if strings.Contains(string(capturedBody), `"temperature"`) {
		t.Errorf("expected temperature key absent, got %s", capturedBody)
	}
}

func TestOpenCode_Send_Claude_EmptyContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[],"model":"claude-sonnet-4-6","stop_reason":"end_turn","usage":{"input_tokens":0,"output_tokens":0}}`))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, err := p.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-6",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for empty content")
	}
	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if pe.Category != ErrCategoryServer {
		t.Errorf("expected ErrCategoryServer, got %q", pe.Category)
	}
	if !strings.Contains(pe.Message, "no content") {
		t.Errorf("expected message to contain 'no content', got %q", pe.Message)
	}
}

func TestOpenCode_Send_Claude_AuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"Invalid API key"}}`))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("bad-key", WithOpenCodeBaseURL(srv.URL))
	_, err := p.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-6",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	assertProviderError(t, err, ErrCategoryAuth)
}

func TestOpenCode_Send_Claude_ForbiddenError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	p, _ := NewOpenCode("bad-key", WithOpenCodeBaseURL(srv.URL))
	_, err := p.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-6",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	assertProviderError(t, err, ErrCategoryAuth)
}

func TestOpenCode_Send_Claude_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, err := p.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-6",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	assertProviderError(t, err, ErrCategoryRateLimit)
}

func TestOpenCode_Send_Claude_Overloaded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(529)
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, err := p.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-6",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	assertProviderError(t, err, ErrCategoryOverloaded)
}

func TestOpenCode_Send_Claude_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, err := p.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-6",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	assertProviderError(t, err, ErrCategoryServer)
}

func TestOpenCode_Send_Claude_ErrorBodyParsed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"max_tokens too large"}}`))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, err := p.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-6",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if pe.Message != "max_tokens too large" {
		t.Errorf("expected message from error body, got %q", pe.Message)
	}
}

func TestOpenCode_Send_Claude_ErrorBodyUnparseable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`not json at all`))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, err := p.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-6",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if pe.Message != http.StatusText(http.StatusBadRequest) {
		t.Errorf("expected HTTP status text fallback %q, got %q", http.StatusText(http.StatusBadRequest), pe.Message)
	}
}

// ---------------------------------------------------------------------------
// OpenAI Responses Format Tests (gpt-* models)
// ---------------------------------------------------------------------------

func makeGPTSuccessResponse(text, model string) string {
	resp := map[string]interface{}{
		"model":  model,
		"status": "completed",
		"output": []map[string]interface{}{
			{
				"type": "message",
				"content": []map[string]interface{}{
					{"type": "output_text", "text": text},
				},
			},
		},
		"usage": map[string]int{
			"input_tokens":  8,
			"output_tokens": 4,
		},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func TestOpenCode_Send_GPT_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(makeGPTSuccessResponse("Hello from GPT!", "gpt-5")))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	resp, err := p.Send(context.Background(), &Request{
		Model:    "gpt-5",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Hello from GPT!" {
		t.Errorf("expected 'Hello from GPT!', got %q", resp.Content)
	}
	if resp.Model != "gpt-5" {
		t.Errorf("expected model 'gpt-5', got %q", resp.Model)
	}
	if resp.InputTokens != 8 {
		t.Errorf("expected InputTokens 8, got %d", resp.InputTokens)
	}
	if resp.OutputTokens != 4 {
		t.Errorf("expected OutputTokens 4, got %d", resp.OutputTokens)
	}
	if resp.StopReason != "completed" {
		t.Errorf("expected StopReason 'completed', got %q", resp.StopReason)
	}
}

func TestOpenCode_Send_GPT_RequestFormat(t *testing.T) {
	var capturedReq *http.Request
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		capturedBody, _ = readBody(r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(makeGPTSuccessResponse("ok", "gpt-5")))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("test-api-key", WithOpenCodeBaseURL(srv.URL))
	_, err := p.Send(context.Background(), &Request{
		Model:    "gpt-5",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedReq.Method != http.MethodPost {
		t.Errorf("expected POST, got %q", capturedReq.Method)
	}
	if capturedReq.URL.Path != "/v1/responses" {
		t.Errorf("expected path '/v1/responses', got %q", capturedReq.URL.Path)
	}
	if got := capturedReq.Header.Get("Authorization"); got != "Bearer test-api-key" {
		t.Errorf("expected 'Bearer test-api-key', got %q", got)
	}
	if got := capturedReq.Header.Get("anthropic-version"); got != "" {
		t.Errorf("expected no anthropic-version header, got %q", got)
	}
	if got := capturedReq.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", got)
	}
	if !strings.Contains(string(capturedBody), `"model":"gpt-5"`) {
		t.Errorf("expected body to contain model field, got %s", capturedBody)
	}
	if !strings.Contains(string(capturedBody), `"input"`) {
		t.Errorf("expected body to contain 'input' field, got %s", capturedBody)
	}
	if strings.Contains(string(capturedBody), `"messages"`) {
		t.Errorf("expected no 'messages' field (should use 'input'), got %s", capturedBody)
	}
}

func TestOpenCode_Send_GPT_SystemPrompt(t *testing.T) {
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = readBody(r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(makeGPTSuccessResponse("ok", "gpt-5")))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))

	// With system prompt - should appear in the input array.
	_, _ = p.Send(context.Background(), &Request{
		Model:    "gpt-5",
		System:   "Be helpful.",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if !strings.Contains(string(capturedBody), `"system"`) {
		t.Errorf("expected system message in input, got %s", capturedBody)
	}
	if !strings.Contains(string(capturedBody), "Be helpful.") {
		t.Errorf("expected system content in input, got %s", capturedBody)
	}

	// Without system prompt.
	_, _ = p.Send(context.Background(), &Request{
		Model:    "gpt-5",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if strings.Contains(string(capturedBody), `"system"`) {
		t.Errorf("expected no system message, got %s", capturedBody)
	}
}

func TestOpenCode_Send_GPT_OmitsZeroMaxOutputTokens(t *testing.T) {
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = readBody(r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(makeGPTSuccessResponse("ok", "gpt-5")))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, _ = p.Send(context.Background(), &Request{
		Model:     "gpt-5",
		MaxTokens: 0,
		Messages:  []Message{{Role: "user", Content: "hi"}},
	})
	if strings.Contains(string(capturedBody), `"max_output_tokens"`) {
		t.Errorf("expected max_output_tokens absent, got %s", capturedBody)
	}
}

func TestOpenCode_Send_GPT_IncludesMaxOutputTokens(t *testing.T) {
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = readBody(r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(makeGPTSuccessResponse("ok", "gpt-5")))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, _ = p.Send(context.Background(), &Request{
		Model:     "gpt-5",
		MaxTokens: 500,
		Messages:  []Message{{Role: "user", Content: "hi"}},
	})
	if !strings.Contains(string(capturedBody), `"max_output_tokens":500`) {
		t.Errorf("expected max_output_tokens 500, got %s", capturedBody)
	}
}

func TestOpenCode_Send_GPT_EmptyOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"gpt-5","status":"completed","output":[],"usage":{"input_tokens":0,"output_tokens":0}}`))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, err := p.Send(context.Background(), &Request{
		Model:    "gpt-5",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if pe.Category != ErrCategoryServer {
		t.Errorf("expected ErrCategoryServer, got %q", pe.Category)
	}
	if !strings.Contains(pe.Message, "no output") {
		t.Errorf("expected message to contain 'no output', got %q", pe.Message)
	}
}

func TestOpenCode_Send_GPT_AuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	p, _ := NewOpenCode("bad-key", WithOpenCodeBaseURL(srv.URL))
	_, err := p.Send(context.Background(), &Request{
		Model:    "gpt-5",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	assertProviderError(t, err, ErrCategoryAuth)
}

func TestOpenCode_Send_GPT_ForbiddenError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, err := p.Send(context.Background(), &Request{
		Model:    "gpt-5",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	assertProviderError(t, err, ErrCategoryAuth)
}

func TestOpenCode_Send_GPT_NotFoundError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, err := p.Send(context.Background(), &Request{
		Model:    "gpt-5",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	assertProviderError(t, err, ErrCategoryBadRequest)
}

// ---------------------------------------------------------------------------
// OpenAI Chat Completions Format Tests (all other models)
// ---------------------------------------------------------------------------

func makeChatCompletionsSuccessResponse(content, model string) string {
	resp := map[string]interface{}{
		"model": model,
		"choices": []map[string]interface{}{
			{
				"finish_reason": "stop",
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": content,
				},
			},
		},
		"usage": map[string]int{
			"prompt_tokens":     12,
			"completion_tokens": 6,
		},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func TestOpenCode_Send_ChatCompletions_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(makeChatCompletionsSuccessResponse("Kimi response!", "kimi-k2")))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	resp, err := p.Send(context.Background(), &Request{
		Model:    "kimi-k2",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Kimi response!" {
		t.Errorf("expected 'Kimi response!', got %q", resp.Content)
	}
	if resp.Model != "kimi-k2" {
		t.Errorf("expected model 'kimi-k2', got %q", resp.Model)
	}
	if resp.InputTokens != 12 {
		t.Errorf("expected InputTokens 12, got %d", resp.InputTokens)
	}
	if resp.OutputTokens != 6 {
		t.Errorf("expected OutputTokens 6, got %d", resp.OutputTokens)
	}
	if resp.StopReason != "stop" {
		t.Errorf("expected StopReason 'stop', got %q", resp.StopReason)
	}
}

func TestOpenCode_Send_ChatCompletions_RequestFormat(t *testing.T) {
	var capturedReq *http.Request
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		capturedBody, _ = readBody(r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(makeChatCompletionsSuccessResponse("ok", "kimi-k2")))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("test-api-key", WithOpenCodeBaseURL(srv.URL))
	_, err := p.Send(context.Background(), &Request{
		Model:    "kimi-k2",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedReq.Method != http.MethodPost {
		t.Errorf("expected POST, got %q", capturedReq.Method)
	}
	if capturedReq.URL.Path != "/v1/chat/completions" {
		t.Errorf("expected path '/v1/chat/completions', got %q", capturedReq.URL.Path)
	}
	if got := capturedReq.Header.Get("Authorization"); got != "Bearer test-api-key" {
		t.Errorf("expected 'Bearer test-api-key', got %q", got)
	}
	if got := capturedReq.Header.Get("anthropic-version"); got != "" {
		t.Errorf("expected no anthropic-version header, got %q", got)
	}
	if got := capturedReq.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", got)
	}
	if !strings.Contains(string(capturedBody), `"model":"kimi-k2"`) {
		t.Errorf("expected body to contain model field, got %s", capturedBody)
	}
	if !strings.Contains(string(capturedBody), `"messages"`) {
		t.Errorf("expected body to contain 'messages' field, got %s", capturedBody)
	}
}

func TestOpenCode_Send_ChatCompletions_EmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"kimi-k2","choices":[],"usage":{"prompt_tokens":0,"completion_tokens":0}}`))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, err := p.Send(context.Background(), &Request{
		Model:    "kimi-k2",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if pe.Category != ErrCategoryServer {
		t.Errorf("expected ErrCategoryServer, got %q", pe.Category)
	}
	if !strings.Contains(pe.Message, "no choices") {
		t.Errorf("expected message to contain 'no choices', got %q", pe.Message)
	}
}

// ---------------------------------------------------------------------------
// Routing Tests
// ---------------------------------------------------------------------------

func TestOpenCode_Send_RoutesClaudeToMessages(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(makeAnthropicSuccessResponse("ok", "claude-sonnet-4-6")))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, _ = p.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-6",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if capturedPath != "/v1/messages" {
		t.Errorf("expected POST to /v1/messages, got %q", capturedPath)
	}
}

func TestOpenCode_Send_RoutesGPTToResponses(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(makeGPTSuccessResponse("ok", "gpt-5")))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, _ = p.Send(context.Background(), &Request{
		Model:    "gpt-5",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if capturedPath != "/v1/responses" {
		t.Errorf("expected POST to /v1/responses, got %q", capturedPath)
	}
}

func TestOpenCode_Send_RoutesOtherToChatCompletions(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(makeChatCompletionsSuccessResponse("ok", "kimi-k2")))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, _ = p.Send(context.Background(), &Request{
		Model:    "kimi-k2",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if capturedPath != "/v1/chat/completions" {
		t.Errorf("expected POST to /v1/chat/completions, got %q", capturedPath)
	}
}

func TestOpenCode_Send_RoutesGeminiToChatCompletions(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		// Return an error (Zen would too), but we only care about the path.
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"gemini not supported via this endpoint"}}`))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, _ = p.Send(context.Background(), &Request{
		Model:    "gemini-3-pro",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if capturedPath != "/v1/chat/completions" {
		t.Errorf("expected POST to /v1/chat/completions, got %q", capturedPath)
	}
}

func TestOpenCode_Send_CaseSensitiveRouting_ClaudeUppercase(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad"}}`))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, _ = p.Send(context.Background(), &Request{
		Model:    "CLAUDE-sonnet-4-6",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if capturedPath != "/v1/chat/completions" {
		t.Errorf("expected uppercase CLAUDE to route to /v1/chat/completions, got %q", capturedPath)
	}
}

func TestOpenCode_Send_CaseSensitiveRouting_GPTUppercase(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad"}}`))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, _ = p.Send(context.Background(), &Request{
		Model:    "GPT-5",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if capturedPath != "/v1/chat/completions" {
		t.Errorf("expected uppercase GPT to route to /v1/chat/completions, got %q", capturedPath)
	}
}

// ---------------------------------------------------------------------------
// Shared Behavior Tests
// ---------------------------------------------------------------------------

func TestOpenCode_Send_ContextTimeout(t *testing.T) {
	done := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until either the request context or the test is done.
		select {
		case <-r.Context().Done():
		case <-done:
		}
	}))
	defer func() {
		close(done)
		srv.Close()
	}()

	models := []struct {
		model string
		name  string
	}{
		{"claude-sonnet-4-6", "claude"},
		{"gpt-5", "gpt"},
		{"kimi-k2", "chat"},
	}

	for _, tc := range models {
		t.Run(tc.name, func(t *testing.T) {
			p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()

			_, err := p.Send(ctx, &Request{
				Model:    tc.model,
				Messages: []Message{{Role: "user", Content: "hi"}},
			})
			var pe *ProviderError
			if !errors.As(err, &pe) {
				t.Fatalf("expected ProviderError, got %T: %v", err, err)
			}
			if pe.Category != ErrCategoryTimeout {
				t.Errorf("expected ErrCategoryTimeout, got %q", pe.Category)
			}
		})
	}
}

func TestOpenCode_Send_NoRedirectFollowing(t *testing.T) {
	// Destination server that would respond if followed.
	dest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(makeChatCompletionsSuccessResponse("should not reach here", "kimi-k2")))
	}))
	defer dest.Close()

	// Redirect server that returns 302.
	redirectSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, dest.URL+r.URL.Path, http.StatusFound)
	}))
	defer redirectSrv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(redirectSrv.URL))
	_, err := p.Send(context.Background(), &Request{
		Model:    "kimi-k2",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	// The provider should not follow the redirect; we expect an error
	// (the 302 is treated as a non-2xx response).
	if err == nil {
		t.Fatal("expected error when server returns 302, provider must not follow redirect")
	}
}

// ---------------------------------------------------------------------------
// Tool Call Tests
// ---------------------------------------------------------------------------

func TestOpenCode_Send_Claude_ToolUse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"content": [{"type": "tool_use", "id": "tu_1", "name": "read_file", "input": {"path": "/tmp/x"}}],
			"model": "claude-sonnet-4-6",
			"stop_reason": "tool_use",
			"usage": {"input_tokens": 5, "output_tokens": 3}
		}`))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	resp, err := p.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-6",
		Messages: []Message{{Role: "user", Content: "read it"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ID != "tu_1" {
		t.Errorf("expected ID 'tu_1', got %q", resp.ToolCalls[0].ID)
	}
	if resp.ToolCalls[0].Name != "read_file" {
		t.Errorf("expected Name 'read_file', got %q", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[0].Arguments["path"] != "/tmp/x" {
		t.Errorf("expected Arguments[\"path\"] '/tmp/x', got %q", resp.ToolCalls[0].Arguments["path"])
	}
}

func TestOpenCode_Send_Claude_ToolResultMessages(t *testing.T) {
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = readBody(r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(makeAnthropicSuccessResponse("done", "claude-sonnet-4-6")))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, err := p.Send(context.Background(), &Request{
		Model: "claude-sonnet-4-6",
		Messages: []Message{
			{Role: "user", Content: "call the tool"},
			{Role: "tool", ToolResults: []ToolResult{{CallID: "tc1", Content: "result text", IsError: false}}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(capturedBody), "tool_result") {
		t.Errorf("expected body to contain 'tool_result', got %s", capturedBody)
	}
	if !strings.Contains(string(capturedBody), "tc1") {
		t.Errorf("expected body to contain 'tc1', got %s", capturedBody)
	}
}

func TestOpenCode_Send_GPT_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, err := p.Send(context.Background(), &Request{
		Model:    "gpt-5",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	assertProviderError(t, err, ErrCategoryServer)
}

func TestOpenCode_Send_ChatCompletions_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	_, err := p.Send(context.Background(), &Request{
		Model:    "kimi-k2",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	assertProviderError(t, err, ErrCategoryServer)
}

// ---------------------------------------------------------------------------
// Streaming Tests
// ---------------------------------------------------------------------------

func sseData(jsonStr string) string {
	return "data: " + jsonStr + "\n\n"
}

func TestOpenCode_SendStream_ClaudeRoute(t *testing.T) {
	var capturedPath string
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedBody, _ = readBody(r)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, sseData(`{"type":"message_start","message":{"id":"msg_1","usage":{"input_tokens":10}}}`))
		_, _ = fmt.Fprint(w, sseData(`{"type":"content_block_start","index":0,"content_block":{"type":"text"}}`))
		_, _ = fmt.Fprint(w, sseData(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`))
		_, _ = fmt.Fprint(w, sseData(`{"type":"content_block_stop","index":0}`))
		_, _ = fmt.Fprint(w, sseData(`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`))
		_, _ = fmt.Fprint(w, sseData(`{"type":"message_stop"}`))
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	stream, err := p.SendStream(context.Background(), &Request{
		Model:    "claude-sonnet-4-6",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var events []StreamEvent
	for {
		ev, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected stream error: %v", err)
		}
		events = append(events, ev)
	}

	if capturedPath != "/v1/messages" {
		t.Errorf("expected path '/v1/messages', got %q", capturedPath)
	}
	if !strings.Contains(string(capturedBody), `"stream":true`) {
		t.Errorf("expected stream:true in body, got %s", capturedBody)
	}

	// text + done = 2
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d: %+v", len(events), events)
	}
	if events[0].Type != StreamEventText || events[0].Text != "Hello" {
		t.Errorf("event 0: expected text 'Hello', got type=%s text=%q", events[0].Type, events[0].Text)
	}
	if events[1].Type != StreamEventDone {
		t.Errorf("event 1: expected done, got type=%s", events[1].Type)
	}
	if events[1].InputTokens != 10 {
		t.Errorf("expected 10 input tokens, got %d", events[1].InputTokens)
	}
	if events[1].OutputTokens != 5 {
		t.Errorf("expected 5 output tokens, got %d", events[1].OutputTokens)
	}
}

func TestOpenCode_SendStream_GPTRoute(t *testing.T) {
	var capturedPath string
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedBody, _ = readBody(r)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, sseData(`{"type":"response.content_part.delta","delta":"Hello"}`))
		_, _ = fmt.Fprint(w, sseData(`{"type":"response.output_item.added","item":{"type":"function_call","id":"fc_1","name":"read_file"}}`))
		_, _ = fmt.Fprint(w, sseData(`{"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"{\"path\":\"/tmp\"}"}`))
		_, _ = fmt.Fprint(w, sseData(`{"type":"response.output_item.done","item":{"type":"function_call","id":"fc_1"}}`))
		_, _ = fmt.Fprint(w, sseData(`{"type":"response.completed","response":{"usage":{"input_tokens":8,"output_tokens":4}}}`))
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	stream, err := p.SendStream(context.Background(), &Request{
		Model:    "gpt-5",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var events []StreamEvent
	for {
		ev, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected stream error: %v", err)
		}
		events = append(events, ev)
	}

	if capturedPath != "/v1/responses" {
		t.Errorf("expected path '/v1/responses', got %q", capturedPath)
	}
	if !strings.Contains(string(capturedBody), `"stream":true`) {
		t.Errorf("expected stream:true in body, got %s", capturedBody)
	}

	// text + tool_start + tool_delta + tool_end + done = 5
	if len(events) != 5 {
		t.Fatalf("expected 5 events, got %d: %+v", len(events), events)
	}
	if events[0].Type != StreamEventText || events[0].Text != "Hello" {
		t.Errorf("event 0: expected text, got type=%s", events[0].Type)
	}
	if events[1].Type != StreamEventToolStart || events[1].ToolName != "read_file" {
		t.Errorf("event 1: expected tool_start read_file, got type=%s name=%s", events[1].Type, events[1].ToolName)
	}
	if events[2].Type != StreamEventToolDelta {
		t.Errorf("event 2: expected tool_delta, got type=%s", events[2].Type)
	}
	if events[3].Type != StreamEventToolEnd || events[3].ToolCallID != "fc_1" {
		t.Errorf("event 3: expected tool_end fc_1, got type=%s id=%s", events[3].Type, events[3].ToolCallID)
	}
	if events[4].Type != StreamEventDone {
		t.Errorf("event 4: expected done, got type=%s", events[4].Type)
	}
	if events[4].InputTokens != 8 {
		t.Errorf("expected 8 input tokens, got %d", events[4].InputTokens)
	}
}

func TestOpenCode_SendStream_ChatCompletionsRoute(t *testing.T) {
	var capturedPath string
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedBody, _ = readBody(r)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, sseData(`{"choices":[{"index":0,"delta":{"content":"Hello"}}]}`))
		_, _ = fmt.Fprint(w, sseData(`{"choices":[{"index":0,"delta":{"content":" world"}}]}`))
		_, _ = fmt.Fprint(w, sseData(`{"choices":[{"index":0,"finish_reason":"stop","delta":{}}]}`))
		_, _ = fmt.Fprint(w, sseData(`{"choices":[],"usage":{"prompt_tokens":12,"completion_tokens":6}}`))
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	stream, err := p.SendStream(context.Background(), &Request{
		Model:    "kimi-k2",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var events []StreamEvent
	for {
		ev, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected stream error: %v", err)
		}
		events = append(events, ev)
	}

	if capturedPath != "/v1/chat/completions" {
		t.Errorf("expected path '/v1/chat/completions', got %q", capturedPath)
	}
	if !strings.Contains(string(capturedBody), `"stream":true`) {
		t.Errorf("expected stream:true in body, got %s", capturedBody)
	}
	if !strings.Contains(string(capturedBody), `"stream_options"`) {
		t.Errorf("expected stream_options in body, got %s", capturedBody)
	}

	// 2 text + 1 done = 3
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d: %+v", len(events), events)
	}
	if events[0].Type != StreamEventText || events[0].Text != "Hello" {
		t.Errorf("event 0: expected text 'Hello', got type=%s text=%q", events[0].Type, events[0].Text)
	}
	if events[1].Type != StreamEventText || events[1].Text != " world" {
		t.Errorf("event 1: expected text ' world', got type=%s text=%q", events[1].Type, events[1].Text)
	}
	if events[2].Type != StreamEventDone {
		t.Errorf("event 2: expected done, got type=%s", events[2].Type)
	}
	if events[2].InputTokens != 12 {
		t.Errorf("expected 12 input tokens, got %d", events[2].InputTokens)
	}
	if events[2].OutputTokens != 6 {
		t.Errorf("expected 6 output tokens, got %d", events[2].OutputTokens)
	}
}

func TestOpenCode_SendStream_UnknownModelFallsThrough(t *testing.T) {
	var capturedPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, sseData(`{"choices":[{"index":0,"delta":{"content":"ok"}}]}`))
		_, _ = fmt.Fprint(w, sseData(`{"choices":[{"index":0,"finish_reason":"stop","delta":{}}]}`))
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
	stream, err := p.SendStream(context.Background(), &Request{
		Model:    "mistral-7b",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	for {
		_, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected stream error: %v", err)
		}
	}

	if capturedPath != "/v1/chat/completions" {
		t.Errorf("expected path '/v1/chat/completions', got %q", capturedPath)
	}
}

func TestOpenCode_IgnoresReasoningEffortAndExplicitZeroTemperature(t *testing.T) {
	routes := []struct {
		name           string
		model          string
		path           string
		response       string
		streamResponse string
	}{
		{
			name:           "messages",
			model:          "claude-sonnet-4-6",
			path:           "/v1/messages",
			response:       makeAnthropicSuccessResponse("ok", "claude-sonnet-4-6"),
			streamResponse: sseData(`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`) + sseData(`{"type":"message_stop"}`),
		},
		{
			name:           "responses",
			model:          "gpt-5",
			path:           "/v1/responses",
			response:       makeGPTSuccessResponse("ok", "gpt-5"),
			streamResponse: sseData(`{"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1}}}`) + sseData(`[DONE]`),
		},
		{
			name:           "chat/completions",
			model:          "kimi-k2",
			path:           "/v1/chat/completions",
			response:       makeChatCompletionsSuccessResponse("ok", "kimi-k2"),
			streamResponse: sseData(`{"choices":[{"index":0,"delta":{"content":"ok"}}]}`) + sseData(`{"choices":[{"index":0,"finish_reason":"stop","delta":{}}]}`) + sseData(`[DONE]`),
		},
	}
	methods := []struct {
		name   string
		stream bool
	}{
		{"Send", false},
		{"SendStream", true},
	}

	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			for _, method := range methods {
				t.Run(method.name, func(t *testing.T) {
					var capturedPath string
					var capturedBody []byte
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						capturedPath = r.URL.Path
						var err error
						capturedBody, err = readBody(r)
						if err != nil {
							t.Errorf("failed to read request body: %v", err)
						}
						if method.stream {
							w.Header().Set("Content-Type", "text/event-stream")
							_, _ = fmt.Fprint(w, route.streamResponse)
							return
						}
						w.Header().Set("Content-Type", "application/json")
						_, _ = fmt.Fprint(w, route.response)
					}))
					defer server.Close()

					p, err := NewOpenCode("test-key", WithOpenCodeBaseURL(server.URL))
					if err != nil {
						t.Fatalf("unexpected constructor error: %v", err)
					}
					req := &Request{
						Model:           route.model,
						Messages:        []Message{{Role: "user", Content: "Hi"}},
						ReasoningEffort: "low",
						TemperatureSet:  true,
						Temperature:     0,
					}
					if method.stream {
						stream, err := p.SendStream(context.Background(), req)
						if err != nil {
							t.Fatalf("unexpected SendStream error: %v", err)
						}
						defer func() { _ = stream.Close() }()
						for {
							_, err := stream.Next()
							if err == io.EOF {
								break
							}
							if err != nil {
								t.Fatalf("unexpected stream error: %v", err)
							}
						}
					} else if _, err := p.Send(context.Background(), req); err != nil {
						t.Fatalf("unexpected Send error: %v", err)
					}

					if capturedPath != route.path {
						t.Errorf("path = %q, want %q", capturedPath, route.path)
					}
					var body map[string]interface{}
					if err := json.Unmarshal(capturedBody, &body); err != nil {
						t.Fatalf("failed to parse request body: %v", err)
					}
					if body["model"] != route.model {
						t.Errorf("model = %v, want %q", body["model"], route.model)
					}
					if method.stream && body["stream"] != true {
						t.Errorf("stream = %v, want true", body["stream"])
					}
					for _, key := range []string{"reasoning_effort", "temperature"} {
						if strings.Contains(string(capturedBody), `"`+key+`"`) {
							t.Errorf("body contains %q: %s", key, capturedBody)
						}
					}
				})
			}
		})
	}
}

func TestOpenCode_SendStream_ErrorResponse(t *testing.T) {
	routes := []struct {
		model string
		name  string
	}{
		{"claude-sonnet-4-6", "claude"},
		{"gpt-5", "gpt"},
		{"kimi-k2", "chat"},
	}

	for _, tc := range routes {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			}))
			defer srv.Close()

			p, _ := NewOpenCode("bad-key", WithOpenCodeBaseURL(srv.URL))
			_, err := p.SendStream(context.Background(), &Request{
				Model:    tc.model,
				Messages: []Message{{Role: "user", Content: "hi"}},
			})
			assertProviderError(t, err, ErrCategoryAuth)
		})
	}
}

func TestOpenCode_SendStream_ContextCancelled(t *testing.T) {
	routes := []struct {
		model string
		name  string
	}{
		{"claude-sonnet-4-6", "claude"},
		{"gpt-5", "gpt"},
		{"kimi-k2", "chat"},
	}

	for _, tc := range routes {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.(http.Flusher).Flush()
				<-r.Context().Done()
			}))
			defer srv.Close()

			p, _ := NewOpenCode("zen-key", WithOpenCodeBaseURL(srv.URL))
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()

			stream, err := p.SendStream(ctx, &Request{
				Model:    tc.model,
				Messages: []Message{{Role: "user", Content: "hi"}},
			})
			if err != nil {
				assertProviderError(t, err, ErrCategoryTimeout)
				return
			}
			defer func() { _ = stream.Close() }()

			for {
				_, err := stream.Next()
				if err == nil {
					continue
				}
				if err == io.EOF {
					t.Fatal("expected timeout error, got EOF")
				}
				assertProviderError(t, err, ErrCategoryTimeout)
				break
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// readBody reads and returns the HTTP request body bytes.
func readBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	defer func() { _ = r.Body.Close() }()
	return io.ReadAll(r.Body)
}

// strPtr returns a pointer to the given string.
func strPtr(s string) *string {
	return &s
}

// assertProviderError checks that err is a *ProviderError with the expected category.
func assertProviderError(t *testing.T, err error, want ErrorCategory) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *ProviderError, got %T: %v", err, err)
	}
	if pe.Category != want {
		t.Errorf("expected category %q, got %q", want, pe.Category)
	}
}

func TestLargeIntegerArgs_OpenCodeClaude_Buffered(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "million", raw: `1000000`, want: "1000000"},
		{name: "negative_million", raw: `-1000000`, want: "-1000000"},
		{name: "non_round_integer", raw: `1234567`, want: "1234567"},
		{name: "negative_non_round_integer", raw: `-1234567`, want: "-1234567"},
		{name: "precise", raw: `9007199254740993`, want: "9007199254740993"},
		{name: "negative_precise", raw: `-9007199254740993`, want: "-9007199254740993"},
		{name: "exponent", raw: `1e6`, want: "1000000"},
		{name: "negative_exponent", raw: `-1e6`, want: "-1000000"},
		{name: "decimal", raw: `1000000.0`, want: "1000000"},
		{name: "negative_decimal", raw: `-1000000.0`, want: "-1000000"},
		{name: "precise_decimal", raw: `9007199254740993.0`, want: "9007199254740993"},
		{name: "negative_precise_decimal", raw: `-9007199254740993.0`, want: "-9007199254740993"},
		{name: "precise_exponent", raw: `90071992547409930e-1`, want: "9007199254740993"},
		{name: "negative_precise_exponent", raw: `-90071992547409930e-1`, want: "-9007199254740993"},
		{name: "beyond_int64", raw: `99999999999999999999999`, want: "99999999999999999999999"},
		{name: "negative_beyond_int64", raw: `-99999999999999999999999`, want: "-99999999999999999999999"},
		{name: "beyond_int64_decimal", raw: `99999999999999999999999.0`, want: "99999999999999999999999"},
		{name: "negative_beyond_int64_decimal", raw: `-99999999999999999999999.0`, want: "-99999999999999999999999"},
		{name: "beyond_int64_exponent", raw: `999999999999999999999990e-1`, want: "99999999999999999999999"},
		{name: "negative_beyond_int64_exponent", raw: `-999999999999999999999990e-1`, want: "-99999999999999999999999"},
		{name: "small", raw: `999999`, want: "999999"},
		{name: "negative_small", raw: `-999999`, want: "-999999"},
		{name: "small_decimal", raw: `999999.0`, want: "999999"},
		{name: "negative_small_decimal", raw: `-999999.0`, want: "-999999"},
		{name: "zero", raw: `0`, want: "0"},
		{name: "decimal_zero", raw: `0.0`, want: "0"},
		{name: "negative_zero", raw: `-0`, want: "-0"},
		{name: "negative_decimal_zero", raw: `-0.0`, want: "-0"},
		{name: "fraction", raw: `1.5`, want: "1.5"},
		{name: "rounded_threshold", raw: `999999.99999999999`, want: "1e+06"},
		{name: "rounded_fraction", raw: `1000000.00000000001`, want: "1e+06"},
		{name: "text", raw: `"text"`, want: "text"},
		{name: "string_integer", raw: `"1000000"`, want: "1000000"},
		{name: "string_exponent", raw: `"1e6"`, want: "1e6"},
		{name: "string_decimal", raw: `"1000000.0"`, want: "1000000.0"},
		{name: "string_precise", raw: `"9007199254740993"`, want: "9007199254740993"},
		{name: "boolean_true", raw: `true`, want: "true"},
		{name: "boolean_false", raw: `false`, want: "false"},
		{name: "null", raw: `null`, want: "<nil>"},
		{name: "object_exponent", raw: `{"a":1e6}`, want: "map[a:1e+06]"},
		{name: "object_integer", raw: `{"a":1000000}`, want: "map[a:1e+06]"},
		{name: "array", raw: `[1.50]`, want: "[1.5]"},
		{name: "recursive_collection", raw: `{"a":[1000000,1.50]}`, want: "map[a:[1e+06 1.5]]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Keep input an object with raw numeric tokens, never a quoted
			// argument string or a fixture map that rounds through float64.
			body := `{"content":[{"type":"tool_use","id":"toolu_1","name":"example","input":{"n":` + tt.raw + `}}],"model":"claude-sonnet-4-6","stop_reason":"tool_use","usage":{"input_tokens":10,"output_tokens":5}}`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
					t.Errorf("request = %s %s, want POST /v1/messages", r.Method, r.URL.Path)
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()
			o, err := NewOpenCode("test-key", WithOpenCodeBaseURL(server.URL))
			if err != nil {
				t.Fatalf("NewOpenCode: %v", err)
			}

			resp, err := o.Send(context.Background(), &Request{
				Model:    "claude-sonnet-4-6",
				Messages: []Message{{Role: "user", Content: "Read values"}},
			})

			if err != nil {
				t.Fatalf("Send: %v", err)
			}
			if resp == nil {
				t.Fatal("Send returned nil response without error")
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

func TestLargeIntegerArgs_OpenCodeResponses_Buffered(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "million", raw: `1000000`, want: "1000000"},
		{name: "negative_million", raw: `-1000000`, want: "-1000000"},
		{name: "non_round_integer", raw: `1234567`, want: "1234567"},
		{name: "negative_non_round_integer", raw: `-1234567`, want: "-1234567"},
		{name: "precise", raw: `9007199254740993`, want: "9007199254740993"},
		{name: "negative_precise", raw: `-9007199254740993`, want: "-9007199254740993"},
		{name: "exponent", raw: `1e6`, want: "1000000"},
		{name: "negative_exponent", raw: `-1e6`, want: "-1000000"},
		{name: "decimal", raw: `1000000.0`, want: "1000000"},
		{name: "negative_decimal", raw: `-1000000.0`, want: "-1000000"},
		{name: "precise_decimal", raw: `9007199254740993.0`, want: "9007199254740993"},
		{name: "negative_precise_decimal", raw: `-9007199254740993.0`, want: "-9007199254740993"},
		{name: "precise_exponent", raw: `90071992547409930e-1`, want: "9007199254740993"},
		{name: "negative_precise_exponent", raw: `-90071992547409930e-1`, want: "-9007199254740993"},
		{name: "beyond_int64", raw: `99999999999999999999999`, want: "99999999999999999999999"},
		{name: "negative_beyond_int64", raw: `-99999999999999999999999`, want: "-99999999999999999999999"},
		{name: "beyond_int64_decimal", raw: `99999999999999999999999.0`, want: "99999999999999999999999"},
		{name: "negative_beyond_int64_decimal", raw: `-99999999999999999999999.0`, want: "-99999999999999999999999"},
		{name: "beyond_int64_exponent", raw: `999999999999999999999990e-1`, want: "99999999999999999999999"},
		{name: "negative_beyond_int64_exponent", raw: `-999999999999999999999990e-1`, want: "-99999999999999999999999"},
		// 10^308 has 309 digits, is finite, and is inside the approved cap.
		{name: "eligible_309_digits", raw: `1e308`, want: "100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"},
		{name: "negative_eligible_309_digits", raw: `-1e308`, want: "-100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"},
		{name: "small", raw: `999999`, want: "999999"},
		{name: "negative_small", raw: `-999999`, want: "-999999"},
		{name: "small_decimal", raw: `999999.0`, want: "999999"},
		{name: "negative_small_decimal", raw: `-999999.0`, want: "-999999"},
		{name: "zero", raw: `0`, want: "0"},
		{name: "decimal_zero", raw: `0.0`, want: "0"},
		{name: "negative_zero", raw: `-0`, want: "-0"},
		{name: "negative_decimal_zero", raw: `-0.0`, want: "-0"},
		{name: "fraction", raw: `1.5`, want: "1.5"},
		{name: "negative_fraction", raw: `-1.5`, want: "-1.5"},
		{name: "rounded_threshold", raw: `999999.99999999999`, want: "1e+06"},
		{name: "negative_rounded_threshold", raw: `-999999.99999999999`, want: "-1e+06"},
		{name: "rounded_fraction", raw: `1000000.00000000001`, want: "1e+06"},
		{name: "negative_rounded_fraction", raw: `-1000000.00000000001`, want: "-1e+06"},
		{name: "huge_negative_exponent", raw: `1e-1000000000`, want: "0"},
		{name: "negative_huge_negative_exponent", raw: `-1e-1000000000`, want: "-0"},
		{name: "zero_huge_exponent", raw: `0e1000000000`, want: "0"},
		{name: "negative_zero_huge_exponent", raw: `-0e1000000000`, want: "-0"},
		// Approved exceptional tokens A/B: exact normalization would violate
		// legacy ParseFloat equality / the 309-digit cap. No token-length limit.
		{name: "exception_A_parse_equality_guard", raw: "1" + strings.Repeat("0", 100006) + "e-100000", want: "0"},
		{name: "exception_B_digit_cap_guard", raw: "0." + strings.Repeat("0", 9999) + "1e100000", want: "1"},
		{name: "text", raw: `"text"`, want: "text"},
		{name: "string_integer", raw: `"1000000"`, want: "1000000"},
		{name: "string_exponent", raw: `"1e6"`, want: "1e6"},
		{name: "string_decimal", raw: `"1000000.0"`, want: "1000000.0"},
		{name: "string_precise", raw: `"9007199254740993"`, want: "9007199254740993"},
		{name: "boolean_true", raw: `true`, want: "true"},
		{name: "boolean_false", raw: `false`, want: "false"},
		{name: "null", raw: `null`, want: "<nil>"},
		{name: "object_exponent", raw: `{"a":1e6}`, want: "map[a:1e+06]"},
		{name: "object_integer", raw: `{"a":1000000}`, want: "map[a:1e+06]"},
		{name: "array", raw: `[1.50]`, want: "[1.5]"},
		{name: "recursive_collection", raw: `{"a":[1000000,1.50]}`, want: "map[a:[1e+06 1.5]]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Quote only the wire arguments string; never decode/re-encode its
			// numeric tokens through float64 or derive expectations from a formatter.
			arguments, err := json.Marshal(`{"n":` + tt.raw + `}`)
			if err != nil {
				t.Fatalf("encode arguments string: %v", err)
			}
			body := `{"model":"gpt-5","status":"completed","output":[{"type":"function_call","id":"call_1","name":"example","arguments":` + string(arguments) + `}],"usage":{"input_tokens":10,"output_tokens":5}}`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/v1/responses" {
					t.Errorf("request = %s %s, want POST /v1/responses", r.Method, r.URL.Path)
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()
			o, err := NewOpenCode("test-key", WithOpenCodeBaseURL(server.URL))
			if err != nil {
				t.Fatalf("NewOpenCode: %v", err)
			}

			resp, err := o.Send(context.Background(), &Request{
				Model:    "gpt-5",
				Messages: []Message{{Role: "user", Content: "Read values"}},
			})

			if err != nil {
				t.Fatalf("Send: %v", err)
			}
			if resp == nil {
				t.Fatal("Send returned nil response without error")
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

func TestLargeIntegerArgs_OpenCodeChat_Buffered(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "million", raw: `1000000`, want: "1000000"},
		{name: "negative_million", raw: `-1000000`, want: "-1000000"},
		{name: "non_round_integer", raw: `1234567`, want: "1234567"},
		{name: "negative_non_round_integer", raw: `-1234567`, want: "-1234567"},
		{name: "precise", raw: `9007199254740993`, want: "9007199254740993"},
		{name: "negative_precise", raw: `-9007199254740993`, want: "-9007199254740993"},
		{name: "exponent", raw: `1e6`, want: "1000000"},
		{name: "negative_exponent", raw: `-1e6`, want: "-1000000"},
		{name: "decimal", raw: `1000000.0`, want: "1000000"},
		{name: "negative_decimal", raw: `-1000000.0`, want: "-1000000"},
		{name: "precise_decimal", raw: `9007199254740993.0`, want: "9007199254740993"},
		{name: "negative_precise_decimal", raw: `-9007199254740993.0`, want: "-9007199254740993"},
		{name: "precise_exponent", raw: `90071992547409930e-1`, want: "9007199254740993"},
		{name: "negative_precise_exponent", raw: `-90071992547409930e-1`, want: "-9007199254740993"},
		{name: "beyond_int64", raw: `99999999999999999999999`, want: "99999999999999999999999"},
		{name: "negative_beyond_int64", raw: `-99999999999999999999999`, want: "-99999999999999999999999"},
		{name: "beyond_int64_decimal", raw: `99999999999999999999999.0`, want: "99999999999999999999999"},
		{name: "negative_beyond_int64_decimal", raw: `-99999999999999999999999.0`, want: "-99999999999999999999999"},
		{name: "beyond_int64_exponent", raw: `999999999999999999999990e-1`, want: "99999999999999999999999"},
		{name: "negative_beyond_int64_exponent", raw: `-999999999999999999999990e-1`, want: "-99999999999999999999999"},
		// 10^308 has 309 digits, is finite, and is inside the approved cap.
		{name: "eligible_309_digits", raw: `1e308`, want: "100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"},
		{name: "negative_eligible_309_digits", raw: `-1e308`, want: "-100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"},
		{name: "small", raw: `999999`, want: "999999"},
		{name: "negative_small", raw: `-999999`, want: "-999999"},
		{name: "small_decimal", raw: `999999.0`, want: "999999"},
		{name: "negative_small_decimal", raw: `-999999.0`, want: "-999999"},
		{name: "zero", raw: `0`, want: "0"},
		{name: "decimal_zero", raw: `0.0`, want: "0"},
		{name: "negative_zero", raw: `-0`, want: "-0"},
		{name: "negative_decimal_zero", raw: `-0.0`, want: "-0"},
		{name: "fraction", raw: `1.5`, want: "1.5"},
		{name: "negative_fraction", raw: `-1.5`, want: "-1.5"},
		{name: "rounded_threshold", raw: `999999.99999999999`, want: "1e+06"},
		{name: "negative_rounded_threshold", raw: `-999999.99999999999`, want: "-1e+06"},
		{name: "rounded_fraction", raw: `1000000.00000000001`, want: "1e+06"},
		{name: "negative_rounded_fraction", raw: `-1000000.00000000001`, want: "-1e+06"},
		{name: "huge_negative_exponent", raw: `1e-1000000000`, want: "0"},
		{name: "negative_huge_negative_exponent", raw: `-1e-1000000000`, want: "-0"},
		{name: "zero_huge_exponent", raw: `0e1000000000`, want: "0"},
		{name: "negative_zero_huge_exponent", raw: `-0e1000000000`, want: "-0"},
		// Approved exceptional tokens A/B retain legacy text without a token-length limit.
		{name: "exception_A_parse_equality_guard", raw: "1" + strings.Repeat("0", 100006) + "e-100000", want: "0"},
		{name: "exception_B_digit_cap_guard", raw: "0." + strings.Repeat("0", 9999) + "1e100000", want: "1"},
		{name: "text", raw: `"text"`, want: "text"},
		{name: "string_integer", raw: `"1000000"`, want: "1000000"},
		{name: "string_exponent", raw: `"1e6"`, want: "1e6"},
		{name: "string_decimal", raw: `"1000000.0"`, want: "1000000.0"},
		{name: "string_precise", raw: `"9007199254740993"`, want: "9007199254740993"},
		{name: "boolean_true", raw: `true`, want: "true"},
		{name: "boolean_false", raw: `false`, want: "false"},
		{name: "null", raw: `null`, want: "<nil>"},
		{name: "object_exponent", raw: `{"a":1e6}`, want: "map[a:1e+06]"},
		{name: "object_integer", raw: `{"a":1000000}`, want: "map[a:1e+06]"},
		{name: "array", raw: `[1.50]`, want: "[1.5]"},
		{name: "recursive_collection", raw: `{"a":[1000000,1.50]}`, want: "map[a:[1e+06 1.5]]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Quote only the Chat wire arguments string, preserving raw numeric
			// tokens inside it and independent literal expected Arguments text.
			arguments, err := json.Marshal(`{"n":` + tt.raw + `}`)
			if err != nil {
				t.Fatalf("encode arguments string: %v", err)
			}
			body := `{"model":"gemini-2.5-pro","choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"example","arguments":` + string(arguments) + `}}]}}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
					t.Errorf("request = %s %s, want POST /v1/chat/completions", r.Method, r.URL.Path)
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()
			o, err := NewOpenCode("test-key", WithOpenCodeBaseURL(server.URL))
			if err != nil {
				t.Fatalf("NewOpenCode: %v", err)
			}

			resp, err := o.Send(context.Background(), &Request{
				Model:    "gemini-2.5-pro",
				Messages: []Message{{Role: "user", Content: "Read values"}},
			})

			if err != nil {
				t.Fatalf("Send: %v", err)
			}
			if resp == nil {
				t.Fatal("Send returned nil response without error")
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

func TestLargeIntegerArgs_OpenCodeClaude_DecodeErrors(t *testing.T) {
	bodyFor := func(input string) string {
		return `{"content":[{"type":"tool_use","id":"toolu_1","name":"example","input":` + input + `}],"usage":{"input_tokens":10,"output_tokens":5}}`
	}
	base := bodyFor(`{"n":"text"}`)
	overflow := "1" + strings.Repeat("0", 399)
	tests := []struct {
		name, body string
		want       map[string]string
		wantTokens int
		wantErr    bool
	}{
		{name: "malformed_response", body: `{"content":`, wantErr: true},
		{name: "malformed_input", body: bodyFor(`{"n":`), wantErr: true},
		{name: "trailing_value", body: base + `{}`, wantErr: true},
		{name: "trailing_junk", body: base + `junk`, wantErr: true},
		{name: "input_overflow", body: bodyFor(`{"n":1e400}`), wantErr: true},
		{name: "input_400_digits", body: bodyFor(`{"n":` + overflow + `}`), wantErr: true},
		{name: "nested_overflow", body: bodyFor(`{"n":[{"bad":1e400}]}`), wantErr: true},
		{name: "nested_400_digits", body: bodyFor(`{"n":{"bad":` + overflow + `}}`), wantErr: true},
		{name: "overwritten_scalar_overflow", body: bodyFor(`{"n":1e400,"n":1000000}`), wantErr: true},
		{name: "overwritten_subtree_overflow", body: bodyFor(`{"n":{"bad":1e400},"n":1000000}`), wantErr: true},
		{name: "successful_scalar_duplicate", body: bodyFor(`{"n":999999,"n":1000000}`), want: map[string]string{"n": "1000000"}, wantTokens: 10},
		{name: "successful_subtree_duplicate", body: bodyFor(`{"n":{"ok":1},"n":1000000}`), want: map[string]string{"n": "1000000"}, wantTokens: 10},
		{name: "empty_input", body: bodyFor(`{}`), want: map[string]string{}, wantTokens: 10},
		{name: "null_input", body: bodyFor(`null`), want: map[string]string{}, wantTokens: 10},
		{name: "array_input", body: bodyFor(`[]`), wantErr: true},
		{name: "scalar_input", body: bodyFor(`0`), wantErr: true},
		{name: "string_input", body: bodyFor(`"text"`), wantErr: true},
		{name: "bool_input", body: bodyFor(`true`), wantErr: true},
		{name: "usage_overflow", body: `{"usage":{"input_tokens":1e400},` + base[1:], wantErr: true},
		{name: "usage_400_digits", body: `{"usage":{"output_tokens":` + overflow + `},` + base[1:], wantErr: true},
		{name: "usage_duplicate_overflow", body: `{"usage":{"input_tokens":1e400,"input_tokens":1},` + base[1:], wantErr: true},
		{name: "usage_overwritten_subtree", body: `{"usage":{"input_tokens":1e400},` + base[1:], wantErr: true},
		{name: "usage_duplicate_success", body: `{"content":[{"type":"tool_use","input":{}}],"usage":{"input_tokens":2,"input_tokens":7}}`, want: map[string]string{}, wantTokens: 7},
		{name: "usage_subtree_success", body: `{"content":[{"type":"tool_use","input":{}}],"usage":{"input_tokens":2},"usage":{"input_tokens":7}}`, want: map[string]string{}, wantTokens: 7},
		{name: "unknown_scalar_overflow", body: `{"unknown":1e400,` + base[1:], want: map[string]string{"n": "text"}, wantTokens: 10},
		{name: "unknown_nested_overflow", body: `{"unknown":{"bad":1e400},` + base[1:], want: map[string]string{"n": "text"}, wantTokens: 10},
		{name: "unknown_400_digits", body: `{"unknown":` + overflow + `,` + base[1:], want: map[string]string{"n": "text"}, wantTokens: 10},
		{name: "unknown_content_overflow", body: `{"content":[{"type":"tool_use","input":{"n":"text"},"unknown":{"bad":1e400}}]}`, want: map[string]string{"n": "text"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()
			p, err := NewOpenCode("test-key", WithOpenCodeBaseURL(server.URL))
			if err != nil {
				t.Fatal(err)
			}
			resp, err := p.Send(context.Background(), &Request{Model: "claude-sonnet-4-6", Messages: []Message{{Role: "user", Content: "Read values"}}})
			if tt.wantErr {
				var pe *ProviderError
				if !errors.As(err, &pe) || pe.Category != ErrCategoryServer {
					t.Fatalf("Send error = %v, want server ProviderError", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Send: %v", err)
			}
			if len(resp.ToolCalls) != 1 || resp.InputTokens != tt.wantTokens {
				t.Fatalf("response = %+v, want one tool call and %d input tokens", resp, tt.wantTokens)
			}
			got := resp.ToolCalls[0].Arguments
			if len(got) != len(tt.want) {
				t.Errorf("Arguments = %v, want %v", got, tt.want)
			}
			for key, want := range tt.want {
				if value, ok := got[key]; !ok || value != want {
					t.Errorf("Arguments[%s] = %q (present=%t), want %q", key, value, ok, want)
				}
			}
			for _, value := range got {
				if strings.Contains(value, "+Inf") {
					t.Errorf("Arguments contain +Inf: %v", got)
				}
			}
		})
	}
}

func TestLargeIntegerArgs_OpenCodeChat_DecodeErrors(t *testing.T) {
	bodyFor := func(arguments string) string {
		quoted, err := json.Marshal(arguments)
		if err != nil {
			t.Fatal(err)
		}
		return `{"model":"gemini-2.5-pro","choices":[{"message":{"tool_calls":[{"id":"call_1","type":"function","function":{"name":"example","arguments":` + string(quoted) + `}}]}}],"usage":{"prompt_tokens":10}}`
	}
	base := bodyFor(`{"n":"text"}`)
	overflow := "1" + strings.Repeat("0", 399)
	const outer = "failed to parse response:"
	const inner = "failed to parse tool call arguments:"
	tests := []struct {
		name, body, errorPrefix string
		want                    map[string]string
		wantTokens              int
	}{
		{name: "outer_malformed", body: `{"choices":`, errorPrefix: outer},
		{name: "outer_trailing_value", body: base + `{}`, errorPrefix: outer},
		{name: "outer_trailing_junk", body: base + `junk`, errorPrefix: outer},
		{name: "outer_usage_overflow", body: `{"usage":{"prompt_tokens":1e400},` + base[1:], errorPrefix: outer},
		{name: "outer_usage_400_digits", body: `{"usage":{"completion_tokens":` + overflow + `},` + base[1:], errorPrefix: outer},
		{name: "outer_usage_type_error", body: `{"usage":{"prompt_tokens":"bad"},` + base[1:], errorPrefix: outer},
		{name: "outer_usage_nested_overflow_type", body: `{"usage":{"prompt_tokens":{"bad":1e400}},` + base[1:], errorPrefix: outer},
		{name: "outer_usage_nested_400_digits_type", body: `{"usage":{"completion_tokens":[` + overflow + `]},` + base[1:], errorPrefix: outer},
		{name: "outer_duplicate_overflow", body: `{"usage":{"prompt_tokens":1e400,"prompt_tokens":1},` + base[1:], errorPrefix: outer},
		{name: "outer_overwritten_subtree_overflow", body: `{"usage":{"prompt_tokens":1e400},` + base[1:], errorPrefix: outer},
		{name: "outer_duplicate_success", body: `{"choices":[{"message":{"tool_calls":[{"function":{"arguments":"{}"}}]}}],"usage":{"prompt_tokens":2,"prompt_tokens":7}}`, want: map[string]string{}, wantTokens: 7},
		{name: "outer_subtree_success", body: `{"choices":[{"message":{"tool_calls":[{"function":{"arguments":"{}"}}]}}],"usage":{"prompt_tokens":2},"usage":{"prompt_tokens":7}}`, want: map[string]string{}, wantTokens: 7},
		{name: "outer_unknown_scalar_overflow", body: `{"unknown":1e400,` + base[1:], want: map[string]string{"n": "text"}, wantTokens: 10},
		{name: "outer_unknown_nested_overflow", body: `{"unknown":{"bad":1e400},` + base[1:], want: map[string]string{"n": "text"}, wantTokens: 10},
		{name: "outer_unknown_400_digits", body: `{"unknown":` + overflow + `,` + base[1:], want: map[string]string{"n": "text"}, wantTokens: 10},
		{name: "outer_unknown_nested_400_digits", body: `{"unknown":{"bad":` + overflow + `},` + base[1:], want: map[string]string{"n": "text"}, wantTokens: 10},
		{name: "outer_unknown_choice_overflow", body: `{"choices":[{"message":{"tool_calls":[{"function":{"arguments":"{}","unknown":{"bad":1e400}}}]}}]}`, want: map[string]string{}},
		{name: "outer_arguments_wrong_type", body: `{"choices":[{"message":{"tool_calls":[{"function":{"arguments":{}}}]}}]}`, errorPrefix: outer},
		{name: "arguments_malformed", body: bodyFor(`{"n":`), errorPrefix: inner},
		{name: "arguments_trailing_value", body: bodyFor(`{} {}`), errorPrefix: inner},
		{name: "arguments_trailing_junk", body: bodyFor(`{} junk`), errorPrefix: inner},
		{name: "arguments_overflow", body: bodyFor(`{"n":1e400}`), errorPrefix: inner},
		{name: "arguments_400_digits", body: bodyFor(`{"n":` + overflow + `}`), errorPrefix: inner},
		{name: "arguments_nested_overflow", body: bodyFor(`{"n":[{"bad":1e400}]}`), errorPrefix: inner},
		{name: "arguments_nested_400_digits", body: bodyFor(`{"n":{"bad":` + overflow + `}}`), errorPrefix: inner},
		{name: "arguments_overwritten_scalar", body: bodyFor(`{"n":1e400,"n":1000000}`), errorPrefix: inner},
		{name: "arguments_overwritten_subtree", body: bodyFor(`{"n":{"bad":1e400},"n":1000000}`), errorPrefix: inner},
		{name: "arguments_scalar_duplicate", body: bodyFor(`{"n":999999,"n":1000000}`), want: map[string]string{"n": "1000000"}, wantTokens: 10},
		{name: "arguments_subtree_duplicate", body: bodyFor(`{"n":{"ok":1},"n":1000000}`), want: map[string]string{"n": "1000000"}, wantTokens: 10},
		{name: "arguments_empty", body: bodyFor(`{}`), want: map[string]string{}, wantTokens: 10},
		{name: "arguments_null", body: bodyFor(`null`), want: map[string]string{}, wantTokens: 10},
		{name: "arguments_array", body: bodyFor(`[]`), errorPrefix: inner},
		{name: "arguments_scalar", body: bodyFor(`0`), errorPrefix: inner},
		{name: "arguments_string", body: bodyFor(`"text"`), errorPrefix: inner},
		{name: "arguments_bool", body: bodyFor(`true`), errorPrefix: inner},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
					t.Errorf("request = %s %s", r.Method, r.URL.Path)
				}
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()
			p, err := NewOpenCode("test-key", WithOpenCodeBaseURL(server.URL))
			if err != nil {
				t.Fatal(err)
			}
			resp, err := p.Send(context.Background(), &Request{Model: "gemini-2.5-pro", Messages: []Message{{Role: "user", Content: "Read values"}}})
			if tt.errorPrefix != "" {
				var pe *ProviderError
				if !errors.As(err, &pe) || pe.Category != ErrCategoryServer || !strings.HasPrefix(pe.Message, tt.errorPrefix) {
					t.Fatalf("Send error = %v, want server ProviderError with %q", err, tt.errorPrefix)
				}
				if resp != nil {
					t.Errorf("response on decode error = %+v", resp)
				}
				return
			}
			if err != nil || resp == nil {
				t.Fatalf("Send = %+v, %v", resp, err)
			}
			if len(resp.ToolCalls) != 1 || resp.InputTokens != tt.wantTokens {
				t.Fatalf("response = %+v, want one tool call and %d input tokens", resp, tt.wantTokens)
			}
			got := resp.ToolCalls[0].Arguments
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Arguments = %v, want %v", got, tt.want)
			}
			for _, value := range got {
				if strings.Contains(value, "+Inf") {
					t.Errorf("Arguments contain +Inf: %v", got)
				}
			}
		})
	}
}

func TestLargeIntegerArgs_OpenCodeResponses_DecodeErrors(t *testing.T) {
	bodyFor := func(arguments string) string {
		quoted, err := json.Marshal(arguments)
		if err != nil {
			t.Fatal(err)
		}
		return `{"model":"gpt-5","output":[{"type":"function_call","id":"call_1","name":"example","arguments":` + string(quoted) + `}],"usage":{"input_tokens":10}}`
	}
	base := bodyFor(`{"n":"text"}`)
	overflow := "1" + strings.Repeat("0", 399)
	tests := []struct {
		name, body, errorPrefix string
		want                    map[string]string
		wantTokens              int
	}{
		{name: "outer_malformed", body: `{"output":`, errorPrefix: "failed to parse response:"},
		{name: "outer_trailing_value", body: base + `{}`, errorPrefix: "failed to parse response:"},
		{name: "outer_trailing_junk", body: base + `junk`, errorPrefix: "failed to parse response:"},
		{name: "outer_usage_overflow", body: `{"usage":{"input_tokens":1e400},` + base[1:], errorPrefix: "failed to parse response:"},
		{name: "outer_usage_400_digits", body: `{"usage":{"output_tokens":` + overflow + `},` + base[1:], errorPrefix: "failed to parse response:"},
		{name: "outer_duplicate_overflow", body: `{"usage":{"input_tokens":1e400,"input_tokens":1},` + base[1:], errorPrefix: "failed to parse response:"},
		{name: "outer_overwritten_subtree_overflow", body: `{"usage":{"input_tokens":1e400},` + base[1:], errorPrefix: "failed to parse response:"},
		{name: "outer_duplicate_success", body: `{"output":[{"type":"function_call","arguments":"{}"}],"usage":{"input_tokens":2,"input_tokens":7}}`, want: map[string]string{}, wantTokens: 7},
		{name: "outer_subtree_success", body: `{"output":[{"type":"function_call","arguments":"{}"}],"usage":{"input_tokens":2},"usage":{"input_tokens":7}}`, want: map[string]string{}, wantTokens: 7},
		{name: "outer_unknown_scalar_overflow", body: `{"unknown":1e400,` + base[1:], want: map[string]string{"n": "text"}, wantTokens: 10},
		{name: "outer_unknown_nested_overflow", body: `{"unknown":{"bad":1e400},` + base[1:], want: map[string]string{"n": "text"}, wantTokens: 10},
		{name: "outer_unknown_400_digits", body: `{"unknown":` + overflow + `,` + base[1:], want: map[string]string{"n": "text"}, wantTokens: 10},
		{name: "outer_unknown_output_overflow", body: `{"output":[{"type":"function_call","arguments":"{}","unknown":{"bad":1e400}}]}`, want: map[string]string{}},
		{name: "outer_arguments_wrong_type", body: `{"output":[{"type":"function_call","arguments":{}}]}`, errorPrefix: "failed to parse response:"},
		{name: "arguments_malformed", body: bodyFor(`{"n":`), errorPrefix: "failed to parse tool call arguments:"},
		{name: "arguments_trailing_value", body: bodyFor(`{} {}`), errorPrefix: "failed to parse tool call arguments:"},
		{name: "arguments_trailing_junk", body: bodyFor(`{} junk`), errorPrefix: "failed to parse tool call arguments:"},
		{name: "arguments_overflow", body: bodyFor(`{"n":1e400}`), errorPrefix: "failed to parse tool call arguments:"},
		{name: "arguments_400_digits", body: bodyFor(`{"n":` + overflow + `}`), errorPrefix: "failed to parse tool call arguments:"},
		{name: "arguments_nested_overflow", body: bodyFor(`{"n":[{"bad":1e400}]}`), errorPrefix: "failed to parse tool call arguments:"},
		{name: "arguments_nested_400_digits", body: bodyFor(`{"n":{"bad":` + overflow + `}}`), errorPrefix: "failed to parse tool call arguments:"},
		{name: "arguments_overwritten_scalar", body: bodyFor(`{"n":1e400,"n":1000000}`), errorPrefix: "failed to parse tool call arguments:"},
		{name: "arguments_overwritten_subtree", body: bodyFor(`{"n":{"bad":1e400},"n":1000000}`), errorPrefix: "failed to parse tool call arguments:"},
		{name: "arguments_scalar_duplicate", body: bodyFor(`{"n":999999,"n":1000000}`), want: map[string]string{"n": "1000000"}, wantTokens: 10},
		{name: "arguments_subtree_duplicate", body: bodyFor(`{"n":{"ok":1},"n":1000000}`), want: map[string]string{"n": "1000000"}, wantTokens: 10},
		{name: "arguments_empty", body: bodyFor(`{}`), want: map[string]string{}, wantTokens: 10},
		{name: "arguments_null", body: bodyFor(`null`), want: map[string]string{}, wantTokens: 10},
		{name: "arguments_array", body: bodyFor(`[]`), errorPrefix: "failed to parse tool call arguments:"},
		{name: "arguments_scalar", body: bodyFor(`0`), errorPrefix: "failed to parse tool call arguments:"},
		{name: "arguments_string", body: bodyFor(`"text"`), errorPrefix: "failed to parse tool call arguments:"},
		{name: "arguments_bool", body: bodyFor(`true`), errorPrefix: "failed to parse tool call arguments:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/v1/responses" {
					t.Errorf("request = %s %s", r.Method, r.URL.Path)
				}
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()
			p, err := NewOpenCode("test-key", WithOpenCodeBaseURL(server.URL))
			if err != nil {
				t.Fatal(err)
			}
			resp, err := p.Send(context.Background(), &Request{Model: "gpt-5", Messages: []Message{{Role: "user", Content: "Read values"}}})
			if tt.errorPrefix != "" {
				var pe *ProviderError
				if !errors.As(err, &pe) || pe.Category != ErrCategoryServer || !strings.HasPrefix(pe.Message, tt.errorPrefix) {
					t.Fatalf("Send error = %v, want server ProviderError with %q", err, tt.errorPrefix)
				}
				if resp != nil {
					t.Errorf("response on decode error = %+v", resp)
				}
				return
			}
			if err != nil {
				t.Fatalf("Send: %v", err)
			}
			if len(resp.ToolCalls) != 1 || resp.InputTokens != tt.wantTokens {
				t.Fatalf("response = %+v, want one tool call and %d input tokens", resp, tt.wantTokens)
			}
			got := resp.ToolCalls[0].Arguments
			if len(got) != len(tt.want) {
				t.Errorf("Arguments = %v, want %v", got, tt.want)
			}
			for key, want := range tt.want {
				if value, ok := got[key]; !ok || value != want {
					t.Errorf("Arguments[%s] = %q (present=%t), want %q", key, value, ok, want)
				}
			}
			for _, value := range got {
				if strings.Contains(value, "+Inf") {
					t.Errorf("Arguments contain +Inf: %v", got)
				}
			}
		})
	}
}
