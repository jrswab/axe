package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
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
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"content":     []map[string]string{{"type": "text", "text": "ok"}},
			"model":       "claude-sonnet-4-20250514",
			"usage":       map[string]int{"input_tokens": 1, "output_tokens": 1},
			"stop_reason": "end_turn",
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("my-api-key", WithBaseURL(server.URL))
	_, _ = a.Send(context.Background(), &Request{
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
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"content":     []map[string]string{{"type": "text", "text": "ok"}},
			"model":       "claude-sonnet-4-20250514",
			"usage":       map[string]int{"input_tokens": 1, "output_tokens": 1},
			"stop_reason": "end_turn",
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	_, _ = a.Send(context.Background(), &Request{
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
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"content":     []map[string]string{{"type": "text", "text": "ok"}},
			"model":       "claude-sonnet-4-20250514",
			"usage":       map[string]int{"input_tokens": 1, "output_tokens": 1},
			"stop_reason": "end_turn",
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	_, _ = a.Send(context.Background(), &Request{
		Model:       "claude-sonnet-4-20250514",
		Temperature: 0, // zero
		Messages:    []Message{{Role: "user", Content: "Hi"}},
	})

	if _, exists := gotBody["temperature"]; exists {
		t.Errorf("body contains 'temperature' key when Temperature is 0: %v", gotBody["temperature"])
	}
}

func TestAnthropic_Send_IgnoresReasoningEffortAndExplicitZeroTemperature(t *testing.T) {
	cases := []struct {
		name        string
		newProvider func(string, ...AnthropicOption) (*Anthropic, error)
		model       string
	}{
		{"anthropic", NewAnthropic, "claude-sonnet-4-20250514"},
		{"minimax", NewMiniMax, "MiniMax-M2.5"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody map[string]interface{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
					t.Errorf("failed to parse request body: %v", err)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(makeAnthropicSuccessResponse("ok", tc.model)))
			}))
			defer server.Close()

			a, err := tc.newProvider("test-key", WithBaseURL(server.URL))
			if err != nil {
				t.Fatalf("unexpected constructor error: %v", err)
			}
			resp, err := a.Send(context.Background(), &Request{
				Model:           tc.model,
				Messages:        []Message{{Role: "user", Content: "Hi"}},
				ReasoningEffort: "low",
				TemperatureSet:  true,
				Temperature:     0,
			})
			if err != nil {
				t.Fatalf("unexpected Send error: %v", err)
			}
			if resp.Content != "ok" {
				t.Errorf("Content = %q, want ok", resp.Content)
			}
			if gotBody["model"] != tc.model {
				t.Errorf("model = %v, want %q", gotBody["model"], tc.model)
			}
			for _, key := range []string{"reasoning_effort", "temperature"} {
				if _, exists := gotBody[key]; exists {
					t.Errorf("body contains %q: %v", key, gotBody[key])
				}
			}
		})
	}
}

func TestAnthropic_Send_DefaultMaxTokens(t *testing.T) {
	var gotBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"content":     []map[string]string{{"type": "text", "text": "ok"}},
			"model":       "claude-sonnet-4-20250514",
			"usage":       map[string]int{"input_tokens": 1, "output_tokens": 1},
			"stop_reason": "end_turn",
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	_, _ = a.Send(context.Background(), &Request{
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
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

// --- Phase 2a: Tool definitions in request ---

func TestAnthropic_Send_WithTools(t *testing.T) {
	var gotBody map[string]json.RawMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"content":     []map[string]string{{"type": "text", "text": "ok"}},
			"model":       "claude-sonnet-4-20250514",
			"usage":       map[string]int{"input_tokens": 1, "output_tokens": 1},
			"stop_reason": "end_turn",
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	_, _ = a.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
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

	// Verify tool name
	var name string
	_ = json.Unmarshal(tools[0]["name"], &name)
	if name != "call_agent" {
		t.Errorf("tools[0].name = %q, want %q", name, "call_agent")
	}

	// Verify tool description
	var desc string
	_ = json.Unmarshal(tools[0]["description"], &desc)
	if desc != "Delegate a task to a sub-agent." {
		t.Errorf("tools[0].description = %q, want %q", desc, "Delegate a task to a sub-agent.")
	}

	// Verify input_schema
	var schema map[string]json.RawMessage
	_ = json.Unmarshal(tools[0]["input_schema"], &schema)
	var schemaType string
	_ = json.Unmarshal(schema["type"], &schemaType)
	if schemaType != "object" {
		t.Errorf("input_schema.type = %q, want %q", schemaType, "object")
	}

	// Verify properties exist
	var props map[string]map[string]interface{}
	_ = json.Unmarshal(schema["properties"], &props)
	if _, ok := props["agent"]; !ok {
		t.Error("input_schema.properties missing 'agent'")
	}
	if _, ok := props["task"]; !ok {
		t.Error("input_schema.properties missing 'task'")
	}
	if _, ok := props["context"]; !ok {
		t.Error("input_schema.properties missing 'context'")
	}

	// Verify required array contains agent and task but not context
	var required []string
	_ = json.Unmarshal(schema["required"], &required)
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

func TestAnthropic_Send_WithoutTools(t *testing.T) {
	var gotBody map[string]json.RawMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"content":     []map[string]string{{"type": "text", "text": "ok"}},
			"model":       "claude-sonnet-4-20250514",
			"usage":       map[string]int{"input_tokens": 1, "output_tokens": 1},
			"stop_reason": "end_turn",
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	_, _ = a.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Tools:    nil,
	})

	if _, ok := gotBody["tools"]; ok {
		t.Error("request body should NOT contain 'tools' key when Tools is nil")
	}
}

// --- Phase 2b: Tool-call response parsing ---

func TestAnthropic_Send_ToolCallResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := `{
			"content": [
				{"type": "tool_use", "id": "toolu_abc123", "name": "call_agent", "input": {"agent": "helper", "task": "run tests"}}
			],
			"model": "claude-sonnet-4-20250514",
			"stop_reason": "tool_use",
			"usage": {"input_tokens": 10, "output_tokens": 5}
		}`
		_, _ = w.Write([]byte(resp))
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	resp, err := a.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
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
	if resp.ToolCalls[0].ID != "toolu_abc123" {
		t.Errorf("ToolCalls[0].ID = %q, want %q", resp.ToolCalls[0].ID, "toolu_abc123")
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

func TestAnthropic_Send_ToolCallWithText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := `{
			"content": [
				{"type": "text", "text": "I'll delegate this task."},
				{"type": "tool_use", "id": "toolu_xyz", "name": "call_agent", "input": {"agent": "helper", "task": "do work"}}
			],
			"model": "claude-sonnet-4-20250514",
			"stop_reason": "tool_use",
			"usage": {"input_tokens": 10, "output_tokens": 5}
		}`
		_, _ = w.Write([]byte(resp))
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	resp, err := a.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Tools: []Tool{
			{Name: "call_agent", Description: "test", Parameters: map[string]ToolParameter{}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "I'll delegate this task." {
		t.Errorf("Content = %q, want %q", resp.Content, "I'll delegate this task.")
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(resp.ToolCalls))
	}
}

func TestAnthropic_Send_ToolCallNoText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := `{
			"content": [
				{"type": "tool_use", "id": "toolu_abc", "name": "call_agent", "input": {"agent": "runner", "task": "test"}}
			],
			"model": "claude-sonnet-4-20250514",
			"stop_reason": "tool_use",
			"usage": {"input_tokens": 10, "output_tokens": 5}
		}`
		_, _ = w.Write([]byte(resp))
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	resp, err := a.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Tools: []Tool{
			{Name: "call_agent", Description: "test", Parameters: map[string]ToolParameter{}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "" {
		t.Errorf("Content = %q, want empty", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(resp.ToolCalls))
	}
}

func TestAnthropic_Send_ToolsStopReason(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := `{
			"content": [
				{"type": "tool_use", "id": "toolu_1", "name": "call_agent", "input": {"agent": "a", "task": "b"}}
			],
			"model": "claude-sonnet-4-20250514",
			"stop_reason": "tool_use",
			"usage": {"input_tokens": 10, "output_tokens": 5}
		}`
		_, _ = w.Write([]byte(resp))
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	resp, err := a.Send(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Tools: []Tool{
			{Name: "call_agent", Description: "test", Parameters: map[string]ToolParameter{}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StopReason != "tool_use" {
		t.Errorf("StopReason = %q, want %q", resp.StopReason, "tool_use")
	}
}

// --- Phase 2c: Tool-result and assistant tool-call messages ---

func TestAnthropic_Send_ToolResultMessage(t *testing.T) {
	var gotBody map[string]json.RawMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"content":     []map[string]string{{"type": "text", "text": "ok"}},
			"model":       "claude-sonnet-4-20250514",
			"usage":       map[string]int{"input_tokens": 1, "output_tokens": 1},
			"stop_reason": "end_turn",
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	_, _ = a.Send(context.Background(), &Request{
		Model: "claude-sonnet-4-20250514",
		Messages: []Message{
			{Role: "user", Content: "Hi"},
			{Role: "assistant", Content: "", ToolCalls: []ToolCall{
				{ID: "toolu_1", Name: "call_agent", Arguments: map[string]string{"agent": "helper"}},
			}},
			{Role: "tool", ToolResults: []ToolResult{
				{CallID: "toolu_1", Content: "result text", IsError: false},
			}},
		},
	})

	var messages []json.RawMessage
	_ = json.Unmarshal(gotBody["messages"], &messages)

	// The tool result message (index 2) should be role "user" with tool_result content blocks
	var toolMsg map[string]json.RawMessage
	_ = json.Unmarshal(messages[2], &toolMsg)

	var role string
	_ = json.Unmarshal(toolMsg["role"], &role)
	if role != "user" {
		t.Errorf("tool result message role = %q, want %q", role, "user")
	}

	var content []map[string]interface{}
	_ = json.Unmarshal(toolMsg["content"], &content)
	if len(content) != 1 {
		t.Fatalf("tool result content blocks = %d, want 1", len(content))
	}
	if content[0]["type"] != "tool_result" {
		t.Errorf("content[0].type = %v, want %q", content[0]["type"], "tool_result")
	}
	if content[0]["tool_use_id"] != "toolu_1" {
		t.Errorf("content[0].tool_use_id = %v, want %q", content[0]["tool_use_id"], "toolu_1")
	}
	if content[0]["content"] != "result text" {
		t.Errorf("content[0].content = %v, want %q", content[0]["content"], "result text")
	}
	if content[0]["is_error"] != false {
		t.Errorf("content[0].is_error = %v, want false", content[0]["is_error"])
	}
}

func TestAnthropic_Send_AssistantToolCallMessage(t *testing.T) {
	var gotBody map[string]json.RawMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"content":     []map[string]string{{"type": "text", "text": "ok"}},
			"model":       "claude-sonnet-4-20250514",
			"usage":       map[string]int{"input_tokens": 1, "output_tokens": 1},
			"stop_reason": "end_turn",
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	_, _ = a.Send(context.Background(), &Request{
		Model: "claude-sonnet-4-20250514",
		Messages: []Message{
			{Role: "user", Content: "Hi"},
			{Role: "assistant", Content: "I'll help", ToolCalls: []ToolCall{
				{ID: "toolu_1", Name: "call_agent", Arguments: map[string]string{"agent": "helper", "task": "work"}},
			}},
			{Role: "tool", ToolResults: []ToolResult{
				{CallID: "toolu_1", Content: "done", IsError: false},
			}},
		},
	})

	var messages []json.RawMessage
	_ = json.Unmarshal(gotBody["messages"], &messages)

	// The assistant message (index 1) should have tool_use content blocks
	var assistMsg map[string]json.RawMessage
	_ = json.Unmarshal(messages[1], &assistMsg)

	var role string
	_ = json.Unmarshal(assistMsg["role"], &role)
	if role != "assistant" {
		t.Errorf("assistant message role = %q, want %q", role, "assistant")
	}

	var content []map[string]interface{}
	_ = json.Unmarshal(assistMsg["content"], &content)

	// Should have text block + tool_use block
	if len(content) < 2 {
		t.Fatalf("assistant content blocks = %d, want >= 2", len(content))
	}

	// Find text block
	foundText := false
	foundToolUse := false
	for _, block := range content {
		if block["type"] == "text" && block["text"] == "I'll help" {
			foundText = true
		}
		if block["type"] == "tool_use" {
			foundToolUse = true
			if block["id"] != "toolu_1" {
				t.Errorf("tool_use.id = %v, want %q", block["id"], "toolu_1")
			}
			if block["name"] != "call_agent" {
				t.Errorf("tool_use.name = %v, want %q", block["name"], "call_agent")
			}
		}
	}
	if !foundText {
		t.Error("assistant message missing text content block")
	}
	if !foundToolUse {
		t.Error("assistant message missing tool_use content block")
	}
}

// --- Prompt Caching Tests ---

func TestAnthropic_StaticPrefixWire(t *testing.T) {
	turns := anthropicStaticPrefixTurns()
	decode := anthropicStaticPrefixDecode()
	report := func(row, mode string, passed bool) {
		t.Logf("%s/%s: pass=%t (two turns, literal wire + static stability + caller nonmutation)", row, mode, passed)
	}
	for _, row := range anthropicStaticPrefixRows() {
		t.Run(row.name, func(t *testing.T) {
			req := anthropicStaticPrefixRequest(row)
			requests := make(chan []byte, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("read local request: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				requests <- body
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"content":[{"type":"text","text":"ok"}],"model":"claude-sonnet-4-20250514","stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`)
			}))
			defer server.Close()
			a, err := NewAnthropic("local-test-key", WithBaseURL(server.URL))
			if err != nil {
				t.Fatal(err)
			}
			// Reuse the same request, tool slice and parameter maps: enabled
			// first, then disabled, so a leaked hint cannot hide in a fresh copy.
			for _, mode := range anthropicStaticPrefixModes() {
				passed := t.Run(mode.name, func(t *testing.T) {
					req.CacheConfig = mode.enabled
					var firstSystem, firstTools json.RawMessage
					for i, turn := range turns {
						t.Run(turn.name, func(t *testing.T) {
							req.Messages = turn.messages
							before, err := json.Marshal(req)
							if err != nil {
								t.Fatal(err)
							}

							_, err = a.Send(context.Background(), req)
							if err != nil {
								t.Fatalf("Send: %v", err)
							}

							after, err := json.Marshal(req)
							if err != nil {
								t.Fatal(err)
							}
							if !bytes.Equal(before, after) {
								t.Errorf("Send mutated caller request/tools/schema/history:\nbefore %s\nafter  %s", before, after)
							}
							var got map[string]json.RawMessage
							if err := json.Unmarshal(<-requests, &got); err != nil {
								t.Fatal(err)
							}
							if i == 0 {
								firstSystem, firstTools = got["system"], got["tools"]
							} else if !bytes.Equal(firstSystem, got["system"]) || !bytes.Equal(firstTools, got["tools"]) {
								t.Error("static system/tool prefix changed when only dynamic conversation changed")
							}
							assertAnthropicStaticPrefixWire(t, got, row, mode, turn, decode)
							// Reject top-level/synthetic controls as well as any controls
							// hidden in messages, schemas or earlier tools above.
							metadata, err := json.Marshal(got)
							if err != nil {
								t.Fatal(err)
							}
							if !reflect.DeepEqual(decode(t, metadata), decode(t, []byte(`{"model":"claude-sonnet-4-20250514","max_tokens":128}`))) {
								t.Errorf("unexpected request metadata: %s", metadata)
							}
						})
					}
				})
				// These parent logs remain visible in the exact non-verbose RED
				// command, providing evidence for the verification-only rows too.
				report(row.name, mode.name, passed)
			}
		})
	}
}

// TestAnthropic_StableToolSchemas covers R3/R5 of
// docs/plans/053_corrected_caching_001_spec.md. Literal Messages schemas follow
// https://platform.claude.com/docs/en/build-with-claude/prompt-caching
// (contract retrieved 2026-11-18): tools precede system and messages. Sorting
// required names is Axe's stability contract, not a claimed provider constraint.
func TestAnthropic_StableToolSchemas(t *testing.T) {
	const repetitions = 16
	// Independent wire oracle: do not marshal Tool or production wire structs.
	// The final placeholder is only the T2 tools-only cache breakpoint.
	const toolsWire = `[
		{"name":"z_lookup","description":"Look up a record","input_schema":{"type":"object","properties":{
			"account":{"type":"string","description":"Account identifier"},
			"key":{"type":"string","description":"Record key"},
			"limit":{"type":"integer","description":"Maximum records"},
			"region":{"type":"string","description":"Lookup region"},
			"verbose":{"type":"boolean","description":"Include details"}
		},"required":["account","key","region"]}},
		{"name":"a_report","description":"Report a record","input_schema":{"type":"object","properties":{
			"format":{"type":"string","description":"Report format"},
			"ratio":{"type":"number","description":"Sampling ratio"},
			"record":{"type":"string","description":"Record to report"},
			"target":{"type":"string","description":"Report destination"},
			"title":{"type":"string","description":"Optional title"}
		},"required":["format","record","target"]}%s}
	]`
	decode := anthropicStaticPrefixDecode()
	turns := anthropicStaticPrefixTurns()
	requests := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read local request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- body
		var envelope struct {
			Stream bool `json:"stream"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			t.Errorf("decode local request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if envelope.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			writeSSE(w, "message_start", `{"type":"message_start","message":{"usage":{"input_tokens":1}}}`)
			writeSSE(w, "message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`)
			writeSSE(w, "message_stop", `{"type":"message_stop"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"content":[{"type":"text","text":"ok"}],"model":"claude-sonnet-4-20250514","stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`)
	}))
	defer server.Close()
	a, err := NewAnthropic("local-test-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}

	// Cache modes/system rows may legitimately differ; transports may not.
	firstTools := make(map[string]json.RawMessage)
	for _, streaming := range []bool{false, true} {
		for _, system := range []string{"Static instructions only.", ""} {
			for _, reverse := range []bool{false, true} {
				// Reuse each construction enabled -> disabled to catch hint leaks.
				for construction := range repetitions {
					req := &Request{Model: "claude-sonnet-4-20250514", MaxTokens: 128,
						System: system, Tools: stableToolSchemaFixture(reverse)}
					for _, mode := range anthropicStaticPrefixModes() {
						name := fmt.Sprintf("stream=%t/system=%t/%s/reverse=%t/construction=%02d", streaming, system != "", mode.name, reverse, construction)
						t.Run(name, func(t *testing.T) {
							req.CacheConfig = mode.enabled
							hint := ""
							if mode.enabled && system == "" {
								hint = `,"cache_control":{"type":"ephemeral"}`
							}
							wantTools := decode(t, []byte(fmt.Sprintf(toolsWire, hint)))
							for _, turn := range turns {
								req.Messages = turn.messages
								before, err := json.Marshal(req)
								if err != nil {
									t.Fatal(err)
								}

								if streaming {
									stream, err := a.SendStream(context.Background(), req)
									if err != nil {
										t.Fatalf("SendStream: %v", err)
									}
									for {
										_, err = stream.Next()
										if err != nil {
											break
										}
									}
									closeErr := stream.Close()
									if err != io.EOF || closeErr != nil {
										t.Fatalf("drain/close stream: %v / %v", err, closeErr)
									}
								} else if _, err := a.Send(context.Background(), req); err != nil {
									t.Fatalf("Send: %v", err)
								}

								after, err := json.Marshal(req)
								if err != nil {
									t.Fatal(err)
								}
								if !bytes.Equal(before, after) {
									t.Errorf("%s mutated caller tools/schema/history: before %s; after %s", turn.name, before, after)
								}
								var got map[string]json.RawMessage
								if err := json.Unmarshal(<-requests, &got); err != nil {
									t.Fatal(err)
								}
								if !reflect.DeepEqual(decode(t, got["tools"]), wantTools) {
									t.Errorf("%s tool schema differs from literal (required must be [account key region] and [format record target], exactly once, no optionals):\ngot %s\nwant %s", turn.name, got["tools"], fmt.Sprintf(toolsWire, hint))
								}
								var required []struct {
									Name   string `json:"name"`
									Schema struct {
										Required []string `json:"required"`
									} `json:"input_schema"`
								}
								if err := json.Unmarshal(got["tools"], &required); err != nil {
									t.Fatal(err)
								}
								for _, tool := range required {
									t.Logf("%s %s required=%v", turn.name, tool.Name, tool.Schema.Required)
								}
								staticKey := fmt.Sprintf("%s/cache=%t", system, mode.enabled)
								if first, exists := firstTools[staticKey]; !exists {
									firstTools[staticKey] = append(json.RawMessage(nil), got["tools"]...)
								} else if !bytes.Equal(first, got["tools"]) {
									t.Errorf("%s static tool bytes changed across turns/constructions/insertion order:\nfirst %s\ngot   %s", turn.name, first, got["tools"])
								}
								if !reflect.DeepEqual(decode(t, got["messages"]), decode(t, []byte(turn.want))) {
									t.Errorf("%s conversation order/content changed: %s; want %s", turn.name, got["messages"], turn.want)
								}
								wantSystem := `"Static instructions only."`
								if mode.enabled {
									wantSystem = `[{"type":"text","text":"Static instructions only.","cache_control":{"type":"ephemeral"}}]`
								}
								if system == "" {
									if _, present := got["system"]; present {
										t.Errorf("tools-only must omit system: %s", got["system"])
									}
								} else if !reflect.DeepEqual(decode(t, got["system"]), decode(t, []byte(wantSystem))) {
									t.Errorf("system/cache hint changed: %s; want %s", got["system"], wantSystem)
								}
								delete(got, "tools")
								delete(got, "system")
								delete(got, "messages")
								wantMetadata := `{"model":"claude-sonnet-4-20250514","max_tokens":128}`
								if streaming {
									wantMetadata = `{"model":"claude-sonnet-4-20250514","max_tokens":128,"stream":true}`
								}
								metadata, err := json.Marshal(got)
								if err != nil {
									t.Fatal(err)
								}
								if !reflect.DeepEqual(decode(t, metadata), decode(t, []byte(wantMetadata))) {
									t.Errorf("unexpected metadata/cache control: %s; want %s", metadata, wantMetadata)
								}
							}
						})
					}
				}
			}
		}
	}
	t.Logf("exercised 2 transports x 2 system rows x 2 cache modes x 2 insertion orders x %d constructions x %d dynamic turns = %d local requests; all streams drained and closed", repetitions, len(turns), 2*2*2*2*repetitions*len(turns))
}

func TestAnthropic_Send_CachedSystemPrompt_EmitsContentBlocks(t *testing.T) {
	var gotBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"content":     []map[string]string{{"type": "text", "text": "ok"}},
			"model":       "claude-sonnet-4-20250514",
			"usage":       map[string]int{"input_tokens": 1, "output_tokens": 1},
			"stop_reason": "end_turn",
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	_, _ = a.Send(context.Background(), &Request{
		Model:       "claude-sonnet-4-20250514",
		System:      "You are a helpful assistant.",
		CacheConfig: true,
		Messages:    []Message{{Role: "user", Content: "Hi"}},
	})

	sysRaw, ok := gotBody["system"].([]interface{})
	if !ok {
		t.Fatalf("system should be array when CacheConfig=true, got %T", gotBody["system"])
	}
	if len(sysRaw) != 1 {
		t.Fatalf("system blocks count = %d, want 1", len(sysRaw))
	}
	block := sysRaw[0].(map[string]interface{})
	if block["type"] != "text" {
		t.Errorf("block.type = %q, want %q", block["type"], "text")
	}
	if block["text"] != "You are a helpful assistant." {
		t.Errorf("block.text = %q, want %q", block["text"], "You are a helpful assistant.")
	}
	cacheCtrl, ok := block["cache_control"].(map[string]interface{})
	if !ok {
		t.Fatal("cache_control missing on system block")
	}
	if cacheCtrl["type"] != "ephemeral" {
		t.Errorf("cache_control.type = %q, want %q", cacheCtrl["type"], "ephemeral")
	}
}

func TestAnthropic_Send_NonCachedSystemPrompt_PlainString(t *testing.T) {
	var gotBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"content":     []map[string]string{{"type": "text", "text": "ok"}},
			"model":       "claude-sonnet-4-20250514",
			"usage":       map[string]int{"input_tokens": 1, "output_tokens": 1},
			"stop_reason": "end_turn",
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	_, _ = a.Send(context.Background(), &Request{
		Model:       "claude-sonnet-4-20250514",
		System:      "You are a helpful assistant.",
		CacheConfig: false,
		Messages:    []Message{{Role: "user", Content: "Hi"}},
	})

	sysRaw, ok := gotBody["system"].(string)
	if !ok {
		t.Fatalf("system should be plain string when CacheConfig=false, got %T", gotBody["system"])
	}
	if sysRaw != "You are a helpful assistant." {
		t.Errorf("system = %q, want %q", sysRaw, "You are a helpful assistant.")
	}
}

func TestAnthropic_Send_CacheUsageParsing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"content":     []map[string]string{{"type": "text", "text": "Hello from Claude"}},
			"model":       "claude-sonnet-4-20250514",
			"stop_reason": "end_turn",
			"usage": map[string]interface{}{
				"input_tokens":                100,
				"output_tokens":               5,
				"cache_creation_input_tokens": 80,
				"cache_read_input_tokens":     10,
			},
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	resp, err := a.Send(context.Background(), &Request{
		Model:       "claude-sonnet-4-20250514",
		CacheConfig: true,
		Messages:    []Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", resp.InputTokens)
	}
	if resp.OutputTokens != 5 {
		t.Errorf("OutputTokens = %d, want 5", resp.OutputTokens)
	}
	if resp.CacheReadTokens != 10 {
		t.Errorf("CacheReadTokens = %d, want 10", resp.CacheReadTokens)
	}
	if resp.CacheWriteTokens != 80 {
		t.Errorf("CacheWriteTokens = %d, want 80", resp.CacheWriteTokens)
	}
}

func TestAnthropic_Send_CacheUsageOmitted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"content":     []map[string]string{{"type": "text", "text": "Hello"}},
			"model":       "claude-sonnet-4-20250514",
			"stop_reason": "end_turn",
			"usage": map[string]int{
				"input_tokens":  50,
				"output_tokens": 5,
			},
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	resp, err := a.Send(context.Background(), &Request{
		Model:       "claude-sonnet-4-20250514",
		CacheConfig: true,
		Messages:    []Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.CacheReadTokens != 0 {
		t.Errorf("CacheReadTokens = %d, want 0", resp.CacheReadTokens)
	}
	if resp.CacheWriteTokens != 0 {
		t.Errorf("CacheWriteTokens = %d, want 0", resp.CacheWriteTokens)
	}
}

func TestLargeIntegerArgs_Anthropic_Buffered(t *testing.T) {
	// Build the response by hand: numeric input tokens must not be quoted or
	// rounded by a fixture encoder before the real provider decodes them.
	bodyFor := func(input string) string {
		return `{"content":[{"type":"tool_use","id":"toolu_1","name":"example","input":` + input + `}],"model":"claude-sonnet-4-20250514","stop_reason":"tool_use","usage":{"input_tokens":10,"output_tokens":5}}`
	}
	type testCase struct {
		name    string
		body    string
		want    map[string]string
		wantErr bool
	}
	var tests []testCase
	for _, tt := range []struct {
		name string
		raw  string
		want string
	}{
		{name: "million", raw: `1000000`, want: "1000000"},
		{name: "precise", raw: `9007199254740993`, want: "9007199254740993"},
		{name: "exponent", raw: `1e6`, want: "1000000"},
		{name: "decimal", raw: `1000000.0`, want: "1000000"},
		{name: "negative", raw: `-1e6`, want: "-1000000"},
		{name: "precise_decimal", raw: `9007199254740993.0`, want: "9007199254740993"},
		{name: "precise_exponent", raw: `90071992547409930e-1`, want: "9007199254740993"},
		{name: "beyond", raw: `99999999999999999999999`, want: "99999999999999999999999"},
		{name: "beyond_decimal", raw: `99999999999999999999999.0`, want: "99999999999999999999999"},
		{name: "beyond_exponent", raw: `999999999999999999999990e-1`, want: "99999999999999999999999"},
		{name: "small", raw: `999999`, want: "999999"},
		{name: "negative_small", raw: `-999999`, want: "-999999"},
		{name: "small_decimal", raw: `999999.0`, want: "999999"},
		{name: "zero", raw: `0`, want: "0"},
		{name: "negative_zero", raw: `-0`, want: "-0"},
		{name: "negative_decimal_zero", raw: `-0.0`, want: "-0"},
		{name: "fraction", raw: `1.5`, want: "1.5"},
		{name: "rounded_fraction", raw: `1000000.00000000001`, want: "1e+06"},
		{name: "rounded_threshold", raw: `999999.99999999999`, want: "1e+06"},
		{name: "text", raw: `"text"`, want: "text"},
		{name: "string_exponent", raw: `"1e6"`, want: "1e6"},
		{name: "string_decimal", raw: `"1000000.0"`, want: "1000000.0"},
		{name: "string_precise", raw: `"9007199254740993"`, want: "9007199254740993"},
		{name: "boolean", raw: `true`, want: "true"},
		{name: "null", raw: `null`, want: "<nil>"},
		{name: "object_exponent", raw: `{"a":1e6}`, want: "map[a:1e+06]"},
		{name: "object_integer", raw: `{"a":1000000}`, want: "map[a:1e+06]"},
		{name: "array", raw: `[1.50]`, want: "[1.5]"},
		{name: "recursive", raw: `{"a":[1000000,1.50]}`, want: "map[a:[1e+06 1.5]]"},
	} {
		tests = append(tests, testCase{
			name: tt.name, body: bodyFor(`{"n":` + tt.raw + `}`),
			want: map[string]string{"n": tt.want},
		})
	}

	// These inputs all cross the same buffered response decode boundary.
	// Even overflow overwritten by a later duplicate remains a server error;
	// unknown response fields, unlike input values, must be skipped.
	overflow := "1"
	for range 399 {
		overflow += "0"
	}
	base := bodyFor(`{"n":"text"}`)
	tests = append(tests, []testCase{
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
		{name: "successful_scalar_duplicate", body: bodyFor(`{"n":999999,"n":1000000}`), want: map[string]string{"n": "1000000"}},
		{name: "successful_subtree_duplicate", body: bodyFor(`{"n":{"ok":1},"n":1000000}`), want: map[string]string{"n": "1000000"}},
		{name: "successful_map_replacement", body: bodyFor(`{"n":{"old":1},"n":{"new":2}}`), want: map[string]string{"n": "map[new:2]"}},
		{name: "empty_input", body: bodyFor(`{}`), want: map[string]string{}},
		{name: "null_input", body: bodyFor(`null`), want: map[string]string{}},
		{name: "array_input_mismatch", body: bodyFor(`[]`), wantErr: true},
		{name: "scalar_input_mismatch", body: bodyFor(`0`), wantErr: true},
		{name: "negative_zero_input_mismatch", body: bodyFor(`-0`), wantErr: true},
		{name: "decimal_zero_input_mismatch", body: bodyFor(`-0.0`), wantErr: true},
		{name: "boolean_input_mismatch", body: bodyFor(`true`), wantErr: true},
		{name: "string_input_mismatch", body: bodyFor(`"text"`), wantErr: true},
		{name: "known_integer_overflow", body: `{"usage":{"input_tokens":1e400},` + base[1:], wantErr: true},
		{name: "known_400_digit_overflow", body: `{"usage":{"output_tokens":` + overflow + `},` + base[1:], wantErr: true},
		{name: "known_type_mismatch", body: `{"usage":{"input_tokens":true},` + base[1:], wantErr: true},
		{name: "overwritten_known_scalar", body: `{"usage":{"input_tokens":1e400,"input_tokens":1},` + base[1:], wantErr: true},
		{name: "overwritten_known_subtree", body: `{"usage":{"input_tokens":1e400},` + base[1:], wantErr: true},
		{name: "unknown_scalar_overflow", body: `{"unknown":1e400,` + base[1:], want: map[string]string{"n": "text"}},
		{name: "unknown_subtree_overflow", body: `{"unknown":{"bad":1e400},` + base[1:], want: map[string]string{"n": "text"}},
		{name: "unknown_400_digits", body: `{"unknown":` + overflow + `,` + base[1:], want: map[string]string{"n": "text"}},
		{name: "unknown_content_field_overflow", body: `{"content":[{"type":"tool_use","id":"toolu_1","name":"example","input":{"n":"text"},"unknown":{"bad":1e400}}]}`, want: map[string]string{"n": "text"}},
	}...)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()
			a, err := NewAnthropic("test-key", WithBaseURL(server.URL))
			if err != nil {
				t.Fatal(err)
			}

			resp, err := a.Send(context.Background(), &Request{
				Model:    "claude-sonnet-4-20250514",
				Messages: []Message{{Role: "user", Content: "Read values"}},
			})

			if tt.wantErr {
				var providerErr *ProviderError
				if !errors.As(err, &providerErr) || providerErr.Category != ErrCategoryServer {
					t.Fatalf("Send error = %v, want server ProviderError", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Send error = %v, want success", err)
			}
			if len(resp.ToolCalls) != 1 {
				t.Fatalf("ToolCalls = %#v, want one call", resp.ToolCalls)
			}
			got := resp.ToolCalls[0].Arguments
			if len(got) != len(tt.want) {
				t.Errorf("Arguments = %#v, want %#v", got, tt.want)
			}
			for key, want := range tt.want {
				if value, ok := got[key]; !ok || value != want {
					t.Errorf("Arguments[%s] = %q (present=%t), want %q", key, value, ok, want)
				}
			}
		})
	}
}

func TestLargeIntegerArgs_Anthropic_Replay(t *testing.T) {
	requests := make(chan []byte, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- body
		var req struct {
			Stream bool `json:"stream"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"}}\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"content":[{"type":"tool_use","id":"toolu_1","name":"example","input":{"precise":9007199254740993,"beyond":99999999999999999999999,"exponent":1e6}}],"model":"claude-sonnet-4-20250514","stop_reason":"tool_use"}`)
	}))
	defer server.Close()

	a, err := NewAnthropic("test-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	req := &Request{Model: "claude-sonnet-4-20250514", Messages: []Message{{Role: "user", Content: "Read values"}}}
	resp, err := a.Send(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("decoded tool calls = %+v, want one", resp.ToolCalls)
	}
	req.Messages = append(req.Messages, Message{Role: "assistant", ToolCalls: resp.ToolCalls})
	if _, err := a.Send(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	stream, err := a.SendStream(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 3 {
		t.Fatalf("requests = %d, want 3", len(requests))
	}
	<-requests // initial raw response request
	for _, mode := range []string{"buffered", "stream"} {
		// The seed's content is a string, so inspect messages as raw JSON first.
		var envelope struct {
			Stream   bool              `json:"stream"`
			Messages []json.RawMessage `json:"messages"`
		}
		if err := json.Unmarshal(<-requests, &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Stream != (mode == "stream") || len(envelope.Messages) != 2 {
			t.Fatalf("%s replay envelope = %+v", mode, envelope)
		}
		var assistant struct {
			Role    string `json:"role"`
			Content []struct {
				Type  string            `json:"type"`
				ID    string            `json:"id"`
				Name  string            `json:"name"`
				Input map[string]string `json:"input"`
			} `json:"content"`
		}
		if err := json.Unmarshal(envelope.Messages[1], &assistant); err != nil {
			t.Fatalf("%s replay input must be strings: %v", mode, err)
		}
		if assistant.Role != "assistant" || len(assistant.Content) != 1 || assistant.Content[0].Type != "tool_use" || assistant.Content[0].ID != "toolu_1" || assistant.Content[0].Name != "example" {
			t.Fatalf("%s replay assistant = %+v", mode, assistant)
		}
		input := assistant.Content[0].Input
		for key, want := range map[string]string{"precise": "9007199254740993", "beyond": "99999999999999999999999", "exponent": "1000000"} {
			if input[key] != want {
				t.Errorf("%s replay %s = %q, want %q", mode, key, input[key], want)
			}
		}
		if len(input) != 3 {
			t.Errorf("%s replay input = %v, want exactly three values", mode, input)
		}
	}
}
