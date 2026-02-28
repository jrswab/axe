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

// --- Phase 2a: Tool definitions in request ---

func TestAnthropic_Send_WithTools(t *testing.T) {
	var gotBody map[string]json.RawMessage

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
	json.Unmarshal(tools[0]["name"], &name)
	if name != "call_agent" {
		t.Errorf("tools[0].name = %q, want %q", name, "call_agent")
	}

	// Verify tool description
	var desc string
	json.Unmarshal(tools[0]["description"], &desc)
	if desc != "Delegate a task to a sub-agent." {
		t.Errorf("tools[0].description = %q, want %q", desc, "Delegate a task to a sub-agent.")
	}

	// Verify input_schema
	var schema map[string]json.RawMessage
	json.Unmarshal(tools[0]["input_schema"], &schema)
	var schemaType string
	json.Unmarshal(schema["type"], &schemaType)
	if schemaType != "object" {
		t.Errorf("input_schema.type = %q, want %q", schemaType, "object")
	}

	// Verify properties exist
	var props map[string]map[string]interface{}
	json.Unmarshal(schema["properties"], &props)
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
	json.Unmarshal(schema["required"], &required)
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
		w.Write([]byte(resp))
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
		w.Write([]byte(resp))
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
		w.Write([]byte(resp))
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
		w.Write([]byte(resp))
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
	json.Unmarshal(gotBody["messages"], &messages)

	// The tool result message (index 2) should be role "user" with tool_result content blocks
	var toolMsg map[string]json.RawMessage
	json.Unmarshal(messages[2], &toolMsg)

	var role string
	json.Unmarshal(toolMsg["role"], &role)
	if role != "user" {
		t.Errorf("tool result message role = %q, want %q", role, "user")
	}

	var content []map[string]interface{}
	json.Unmarshal(toolMsg["content"], &content)
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
	json.Unmarshal(gotBody["messages"], &messages)

	// The assistant message (index 1) should have tool_use content blocks
	var assistMsg map[string]json.RawMessage
	json.Unmarshal(messages[1], &assistMsg)

	var role string
	json.Unmarshal(assistMsg["role"], &role)
	if role != "assistant" {
		t.Errorf("assistant message role = %q, want %q", role, "assistant")
	}

	var content []map[string]interface{}
	json.Unmarshal(assistMsg["content"], &content)

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
