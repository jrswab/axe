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

// --- Phase 3a: Tool definitions in request ---

func TestOpenAI_Send_WithTools(t *testing.T) {
	var gotBody map[string]json.RawMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"model": "gpt-4o",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	o.Send(context.Background(), &Request{
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
	json.Unmarshal(tools[0]["type"], &toolType)
	if toolType != "function" {
		t.Errorf("tools[0].type = %q, want %q", toolType, "function")
	}

	// Verify function object
	var fn map[string]json.RawMessage
	json.Unmarshal(tools[0]["function"], &fn)

	var name string
	json.Unmarshal(fn["name"], &name)
	if name != "call_agent" {
		t.Errorf("tools[0].function.name = %q, want %q", name, "call_agent")
	}

	var desc string
	json.Unmarshal(fn["description"], &desc)
	if desc != "Delegate a task to a sub-agent." {
		t.Errorf("tools[0].function.description = %q, want %q", desc, "Delegate a task to a sub-agent.")
	}

	// Verify parameters (JSON Schema)
	var params map[string]json.RawMessage
	json.Unmarshal(fn["parameters"], &params)

	var paramsType string
	json.Unmarshal(params["type"], &paramsType)
	if paramsType != "object" {
		t.Errorf("parameters.type = %q, want %q", paramsType, "object")
	}

	var props map[string]map[string]interface{}
	json.Unmarshal(params["properties"], &props)
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
	json.Unmarshal(params["required"], &required)
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
		json.Unmarshal(body, &gotBody)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"model": "gpt-4o",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	o.Send(context.Background(), &Request{
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
		w.Write([]byte(resp))
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
		w.Write([]byte(resp))
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
		w.Write([]byte(resp))
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
		w.Write([]byte(resp))
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
		json.Unmarshal(body, &gotBody)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"model": "gpt-4o",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	o.Send(context.Background(), &Request{
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
	json.Unmarshal(gotBody["messages"], &messages)

	// Messages: system (if present) + user + assistant + tool result(s)
	// Find the tool messages - they should be role "tool" with tool_call_id
	// The tool result message should expand to one message per ToolResult
	found := false
	for _, raw := range messages {
		var msg map[string]interface{}
		json.Unmarshal(raw, &msg)
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

func TestOpenAI_Send_AssistantToolCallMessage(t *testing.T) {
	var gotBody map[string]json.RawMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"model": "gpt-4o",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
	o.Send(context.Background(), &Request{
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
	json.Unmarshal(gotBody["messages"], &messages)

	// Find assistant message with tool_calls
	found := false
	for _, raw := range messages {
		var msg map[string]json.RawMessage
		json.Unmarshal(raw, &msg)

		var role string
		json.Unmarshal(msg["role"], &role)

		if role == "assistant" {
			if _, ok := msg["tool_calls"]; !ok {
				t.Error("assistant message missing 'tool_calls' field")
				continue
			}
			found = true

			var toolCalls []map[string]json.RawMessage
			json.Unmarshal(msg["tool_calls"], &toolCalls)
			if len(toolCalls) != 1 {
				t.Fatalf("len(tool_calls) = %d, want 1", len(toolCalls))
			}

			var id string
			json.Unmarshal(toolCalls[0]["id"], &id)
			if id != "call_1" {
				t.Errorf("tool_calls[0].id = %q, want %q", id, "call_1")
			}

			var tcType string
			json.Unmarshal(toolCalls[0]["type"], &tcType)
			if tcType != "function" {
				t.Errorf("tool_calls[0].type = %q, want %q", tcType, "function")
			}

			var fn map[string]json.RawMessage
			json.Unmarshal(toolCalls[0]["function"], &fn)

			var fnName string
			json.Unmarshal(fn["name"], &fnName)
			if fnName != "call_agent" {
				t.Errorf("tool_calls[0].function.name = %q, want %q", fnName, "call_agent")
			}

			// Arguments should be a JSON string
			var argsStr string
			json.Unmarshal(fn["arguments"], &argsStr)
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
