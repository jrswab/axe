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

func TestNewOllama_Defaults(t *testing.T) {
	o, err := NewOllama()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o == nil {
		t.Fatal("expected non-nil Ollama")
	}
}

func TestNewOllama_WithBaseURL(t *testing.T) {
	o, err := NewOllama(WithOllamaBaseURL("http://custom:11434"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o == nil {
		t.Fatal("expected non-nil Ollama")
	}
}

func ollamaSuccessResponse() map[string]interface{} {
	return map[string]interface{}{
		"model": "llama3",
		"message": map[string]string{
			"content": "Hello from Ollama",
		},
		"done_reason":       "stop",
		"prompt_eval_count": 8,
		"eval_count":        12,
	}
}

func TestOllama_Send_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ollamaSuccessResponse())
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	resp, err := o.Send(context.Background(), &Request{
		Model:    "llama3",
		System:   "Be helpful.",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Hello from Ollama" {
		t.Errorf("expected 'Hello from Ollama', got %q", resp.Content)
	}
	if resp.Model != "llama3" {
		t.Errorf("expected model 'llama3', got %q", resp.Model)
	}
	if resp.InputTokens != 8 {
		t.Errorf("expected 8 input tokens, got %d", resp.InputTokens)
	}
	if resp.OutputTokens != 12 {
		t.Errorf("expected 12 output tokens, got %d", resp.OutputTokens)
	}
	if resp.StopReason != "stop" {
		t.Errorf("expected stop reason 'stop', got %q", resp.StopReason)
	}
}

func TestOllama_Send_RequestFormat(t *testing.T) {
	var gotMethod, gotPath, gotCT, gotAuth string
	var gotStream interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotCT = r.Header.Get("Content-Type")
		gotAuth = r.Header.Get("Authorization")

		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		json.Unmarshal(body, &req)
		gotStream = req["stream"]

		json.NewEncoder(w).Encode(ollamaSuccessResponse())
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:    "llama3",
		System:   "Be helpful",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != "POST" {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/api/chat" {
		t.Errorf("expected /api/chat, got %s", gotPath)
	}
	if gotCT != "application/json" {
		t.Errorf("expected 'application/json', got %q", gotCT)
	}
	if gotAuth != "" {
		t.Errorf("expected no Authorization header, got %q", gotAuth)
	}
	if gotStream != false {
		t.Errorf("expected stream=false, got %v", gotStream)
	}
}

func TestOllama_Send_SystemMessage(t *testing.T) {
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

		json.NewEncoder(w).Encode(ollamaSuccessResponse())
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:    "llama3",
		System:   "Be helpful",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMsgCount != 2 {
		t.Fatalf("expected 2 messages, got %d", gotMsgCount)
	}
	if gotFirstRole != "system" {
		t.Errorf("expected first message role 'system', got %v", gotFirstRole)
	}
}

func TestOllama_Send_OmitsEmptySystem(t *testing.T) {
	var gotMsgCount int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		json.Unmarshal(body, &req)

		if msgs, ok := req["messages"].([]interface{}); ok {
			gotMsgCount = len(msgs)
		}

		json.NewEncoder(w).Encode(ollamaSuccessResponse())
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:    "llama3",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMsgCount != 1 {
		t.Errorf("expected 1 message, got %d", gotMsgCount)
	}
}

func TestOllama_Send_OmitsZeroTemperature(t *testing.T) {
	var hasTemperature bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		json.Unmarshal(body, &raw)

		if opts, ok := raw["options"]; ok {
			var optsMap map[string]json.RawMessage
			json.Unmarshal(opts, &optsMap)
			_, hasTemperature = optsMap["temperature"]
		}

		json.NewEncoder(w).Encode(ollamaSuccessResponse())
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:       "llama3",
		Messages:    []Message{{Role: "user", Content: "Hi"}},
		Temperature: 0,
		MaxTokens:   100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasTemperature {
		t.Error("expected temperature to be omitted")
	}
}

func TestOllama_Send_OmitsZeroMaxTokens(t *testing.T) {
	var hasNumPredict bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		json.Unmarshal(body, &raw)

		if opts, ok := raw["options"]; ok {
			var optsMap map[string]json.RawMessage
			json.Unmarshal(opts, &optsMap)
			_, hasNumPredict = optsMap["num_predict"]
		}

		json.NewEncoder(w).Encode(ollamaSuccessResponse())
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:       "llama3",
		Messages:    []Message{{Role: "user", Content: "Hi"}},
		Temperature: 0.7,
		MaxTokens:   0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasNumPredict {
		t.Error("expected num_predict to be omitted")
	}
}

func TestOllama_Send_OmitsOptionsWhenEmpty(t *testing.T) {
	var hasOptions bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		json.Unmarshal(body, &raw)
		_, hasOptions = raw["options"]

		json.NewEncoder(w).Encode(ollamaSuccessResponse())
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:       "llama3",
		Messages:    []Message{{Role: "user", Content: "Hi"}},
		Temperature: 0,
		MaxTokens:   0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasOptions {
		t.Error("expected options to be omitted entirely")
	}
}

func TestOllama_Send_IncludesOptions(t *testing.T) {
	var hasOptions, hasTemperature, hasNumPredict bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		json.Unmarshal(body, &raw)

		if opts, ok := raw["options"]; ok {
			hasOptions = true
			var optsMap map[string]json.RawMessage
			json.Unmarshal(opts, &optsMap)
			_, hasTemperature = optsMap["temperature"]
			_, hasNumPredict = optsMap["num_predict"]
		}

		json.NewEncoder(w).Encode(ollamaSuccessResponse())
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:       "llama3",
		Messages:    []Message{{Role: "user", Content: "Hi"}},
		Temperature: 0.7,
		MaxTokens:   512,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasOptions {
		t.Fatal("expected options to be present")
	}
	if !hasTemperature {
		t.Error("expected temperature in options")
	}
	if !hasNumPredict {
		t.Error("expected num_predict in options")
	}
}

func TestOllama_Send_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"invalid model"}`))
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{Model: "bad", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryBadRequest {
		t.Errorf("expected ErrCategoryBadRequest, got %s", provErr.Category)
	}
}

func TestOllama_Send_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"error":"model 'xyz' not found"}`))
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{Model: "xyz", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryBadRequest {
		t.Errorf("expected ErrCategoryBadRequest, got %s", provErr.Category)
	}
}

func TestOllama_Send_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{Model: "llama3", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryServer {
		t.Errorf("expected ErrCategoryServer, got %s", provErr.Category)
	}
}

func TestOllama_Send_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := o.Send(ctx, &Request{Model: "llama3", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryTimeout {
		t.Errorf("expected ErrCategoryTimeout, got %s", provErr.Category)
	}
}

func TestOllama_Send_ConnectionRefused(t *testing.T) {
	// Use a port that nothing is listening on
	o, _ := NewOllama(WithOllamaBaseURL("http://localhost:1"))
	_, err := o.Send(context.Background(), &Request{Model: "llama3", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryServer {
		t.Errorf("expected ErrCategoryServer, got %s", provErr.Category)
	}
	if !strings.Contains(provErr.Message, "connection refused") {
		t.Errorf("expected message to contain 'connection refused', got %q", provErr.Message)
	}
	if !strings.Contains(provErr.Message, "is Ollama running?") {
		t.Errorf("expected message to contain 'is Ollama running?', got %q", provErr.Message)
	}
	if !strings.Contains(provErr.Message, "http://localhost:1") {
		t.Errorf("expected message to contain base URL, got %q", provErr.Message)
	}
}

func TestOllama_Send_EmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"model":   "llama3",
			"message": map[string]string{"content": ""},
		})
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{Model: "llama3", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryServer {
		t.Errorf("expected ErrCategoryServer, got %s", provErr.Category)
	}
	if !strings.Contains(provErr.Message, "no content") {
		t.Errorf("expected 'no content' in message, got %q", provErr.Message)
	}
}

func TestOllama_Send_ErrorResponseParsing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"model requires more memory"}`))
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{Model: "llama3", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if !strings.Contains(provErr.Message, "model requires more memory") {
		t.Errorf("expected parsed error message, got %q", provErr.Message)
	}
}

func TestOllama_Send_UnparseableErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{Model: "llama3", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	// Should fall back to HTTP status text
	if provErr.Message != "Bad Request" {
		t.Errorf("expected 'Bad Request', got %q", provErr.Message)
	}
}

func TestOllama_Send_ZeroTokenCounts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"model":   "llama3",
			"message": map[string]string{"content": "Hello"},
		})
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	resp, err := o.Send(context.Background(), &Request{Model: "llama3", Messages: []Message{{Role: "user", Content: "Hi"}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.InputTokens != 0 {
		t.Errorf("expected 0 input tokens, got %d", resp.InputTokens)
	}
	if resp.OutputTokens != 0 {
		t.Errorf("expected 0 output tokens, got %d", resp.OutputTokens)
	}
}

// --- Phase 4a: Tool Definitions in Request ---

func TestOllama_Send_WithTools(t *testing.T) {
	var gotBody map[string]json.RawMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		json.NewEncoder(w).Encode(ollamaSuccessResponse())
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:    "llama3",
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Tools: []Tool{
			{
				Name:        "call_agent",
				Description: "Delegate a task to a sub-agent.",
				Parameters: map[string]ToolParameter{
					"agent": {Type: "string", Description: "Agent name", Required: true},
					"task":  {Type: "string", Description: "What to do", Required: true},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify tools key is present
	toolsRaw, ok := gotBody["tools"]
	if !ok {
		t.Fatal("expected 'tools' key in request body")
	}

	var tools []map[string]interface{}
	if err := json.Unmarshal(toolsRaw, &tools); err != nil {
		t.Fatalf("failed to unmarshal tools: %v", err)
	}

	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	tool := tools[0]
	if tool["type"] != "function" {
		t.Errorf("expected tool type 'function', got %v", tool["type"])
	}

	fn, ok := tool["function"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'function' to be an object")
	}

	if fn["name"] != "call_agent" {
		t.Errorf("expected function name 'call_agent', got %v", fn["name"])
	}
	if fn["description"] != "Delegate a task to a sub-agent." {
		t.Errorf("expected description 'Delegate a task to a sub-agent.', got %v", fn["description"])
	}

	params, ok := fn["parameters"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'parameters' to be an object")
	}
	if params["type"] != "object" {
		t.Errorf("expected parameters type 'object', got %v", params["type"])
	}

	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'properties' to be an object")
	}
	if _, ok := props["agent"]; !ok {
		t.Error("expected 'agent' property")
	}
	if _, ok := props["task"]; !ok {
		t.Error("expected 'task' property")
	}

	// Check required array
	req, ok := params["required"].([]interface{})
	if !ok {
		t.Fatal("expected 'required' to be an array")
	}
	reqSet := make(map[string]bool)
	for _, r := range req {
		reqSet[r.(string)] = true
	}
	if !reqSet["agent"] {
		t.Error("expected 'agent' in required list")
	}
	if !reqSet["task"] {
		t.Error("expected 'task' in required list")
	}
}

func TestOllama_Send_WithoutTools(t *testing.T) {
	var gotBody map[string]json.RawMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		json.NewEncoder(w).Encode(ollamaSuccessResponse())
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:    "llama3",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := gotBody["tools"]; ok {
		t.Error("expected no 'tools' key in request body when Tools is nil")
	}
}

// --- Phase 4b: Tool-Call Response Parsing ---

func TestOllama_Send_ToolCallResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"model": "llama3",
			"message": map[string]interface{}{
				"role":    "assistant",
				"content": "",
				"tool_calls": []map[string]interface{}{
					{
						"function": map[string]interface{}{
							"name": "call_agent",
							"arguments": map[string]interface{}{
								"agent": "test-runner",
								"task":  "run tests",
							},
						},
					},
					{
						"function": map[string]interface{}{
							"name": "call_agent",
							"arguments": map[string]interface{}{
								"agent": "linter",
								"task":  "lint code",
							},
						},
					},
				},
			},
			"done_reason":       "stop",
			"prompt_eval_count": 10,
			"eval_count":        5,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	resp, err := o.Send(context.Background(), &Request{
		Model:    "llama3",
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Tools: []Tool{
			{Name: "call_agent", Description: "test", Parameters: map[string]ToolParameter{}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(resp.ToolCalls))
	}

	// Verify generated IDs follow "ollama_<index>" format
	if resp.ToolCalls[0].ID != "ollama_0" {
		t.Errorf("expected ID 'ollama_0', got %q", resp.ToolCalls[0].ID)
	}
	if resp.ToolCalls[1].ID != "ollama_1" {
		t.Errorf("expected ID 'ollama_1', got %q", resp.ToolCalls[1].ID)
	}

	// Verify first tool call
	if resp.ToolCalls[0].Name != "call_agent" {
		t.Errorf("expected name 'call_agent', got %q", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[0].Arguments["agent"] != "test-runner" {
		t.Errorf("expected agent 'test-runner', got %q", resp.ToolCalls[0].Arguments["agent"])
	}
	if resp.ToolCalls[0].Arguments["task"] != "run tests" {
		t.Errorf("expected task 'run tests', got %q", resp.ToolCalls[0].Arguments["task"])
	}

	// Verify second tool call
	if resp.ToolCalls[1].Name != "call_agent" {
		t.Errorf("expected name 'call_agent', got %q", resp.ToolCalls[1].Name)
	}
	if resp.ToolCalls[1].Arguments["agent"] != "linter" {
		t.Errorf("expected agent 'linter', got %q", resp.ToolCalls[1].Arguments["agent"])
	}
}

func TestOllama_Send_NoToolCallsWithTools(t *testing.T) {
	// Model ignores tools and returns a normal text response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ollamaSuccessResponse())
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	resp, err := o.Send(context.Background(), &Request{
		Model:    "llama3",
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Tools: []Tool{
			{Name: "call_agent", Description: "test", Parameters: map[string]ToolParameter{}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.ToolCalls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(resp.ToolCalls))
	}
	if resp.Content != "Hello from Ollama" {
		t.Errorf("expected 'Hello from Ollama', got %q", resp.Content)
	}
}

// --- Phase 4c: Tool-Result and Assistant Tool-Call Messages ---

func TestOllama_Send_ToolResultMessage(t *testing.T) {
	var gotBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		json.NewEncoder(w).Encode(ollamaSuccessResponse())
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model: "llama3",
		Messages: []Message{
			{Role: "user", Content: "Hi"},
			{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{ID: "ollama_0", Name: "call_agent", Arguments: map[string]string{"agent": "helper", "task": "do it"}},
				},
			},
			{
				Role: "tool",
				ToolResults: []ToolResult{
					{CallID: "ollama_0", Content: "Result from helper", IsError: false},
					{CallID: "ollama_1", Content: "Another result", IsError: true},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(gotBody, &req); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	// Messages: system(if any), user, assistant(tool_calls), tool, tool
	// With no system prompt, we expect: user, assistant, tool, tool = 4 messages
	if len(req.Messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(req.Messages))
	}

	// Verify tool result messages (indices 2 and 3)
	for i, idx := range []int{2, 3} {
		var msg map[string]interface{}
		json.Unmarshal(req.Messages[idx], &msg)
		if msg["role"] != "tool" {
			t.Errorf("message %d: expected role 'tool', got %v", idx, msg["role"])
		}
		if i == 0 {
			if msg["content"] != "Result from helper" {
				t.Errorf("expected content 'Result from helper', got %v", msg["content"])
			}
		} else {
			if msg["content"] != "Another result" {
				t.Errorf("expected content 'Another result', got %v", msg["content"])
			}
		}
	}
}

func TestOllama_Send_AssistantToolCallMessage(t *testing.T) {
	var gotBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		json.NewEncoder(w).Encode(ollamaSuccessResponse())
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model: "llama3",
		Messages: []Message{
			{Role: "user", Content: "Hi"},
			{
				Role:    "assistant",
				Content: "I'll help you",
				ToolCalls: []ToolCall{
					{ID: "ollama_0", Name: "call_agent", Arguments: map[string]string{"agent": "helper", "task": "do it"}},
				},
			},
			{
				Role: "tool",
				ToolResults: []ToolResult{
					{CallID: "ollama_0", Content: "Done", IsError: false},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(gotBody, &req); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	// Messages: user, assistant(with tool_calls), tool = 3 messages
	if len(req.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(req.Messages))
	}

	// Verify assistant message with tool_calls (index 1)
	var assistantMsg map[string]interface{}
	json.Unmarshal(req.Messages[1], &assistantMsg)

	if assistantMsg["role"] != "assistant" {
		t.Errorf("expected role 'assistant', got %v", assistantMsg["role"])
	}

	toolCalls, ok := assistantMsg["tool_calls"].([]interface{})
	if !ok {
		t.Fatal("expected 'tool_calls' to be an array")
	}
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}

	tc := toolCalls[0].(map[string]interface{})
	fn := tc["function"].(map[string]interface{})
	if fn["name"] != "call_agent" {
		t.Errorf("expected function name 'call_agent', got %v", fn["name"])
	}

	args := fn["arguments"].(map[string]interface{})
	if args["agent"] != "helper" {
		t.Errorf("expected agent 'helper', got %v", args["agent"])
	}
	if args["task"] != "do it" {
		t.Errorf("expected task 'do it', got %v", args["task"])
	}
}
