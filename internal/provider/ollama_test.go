package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
		_ = json.NewEncoder(w).Encode(ollamaSuccessResponse())
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
		_ = json.Unmarshal(body, &req)
		gotStream = req["stream"]

		_ = json.NewEncoder(w).Encode(ollamaSuccessResponse())
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
		_ = json.Unmarshal(body, &req)

		if msgs, ok := req["messages"].([]interface{}); ok {
			gotMsgCount = len(msgs)
			if len(msgs) >= 1 {
				if first, ok := msgs[0].(map[string]interface{}); ok {
					gotFirstRole, _ = first["role"].(string)
				}
			}
		}

		_ = json.NewEncoder(w).Encode(ollamaSuccessResponse())
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
		_ = json.Unmarshal(body, &req)

		if msgs, ok := req["messages"].([]interface{}); ok {
			gotMsgCount = len(msgs)
		}

		_ = json.NewEncoder(w).Encode(ollamaSuccessResponse())
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
		_ = json.Unmarshal(body, &raw)

		if opts, ok := raw["options"]; ok {
			var optsMap map[string]json.RawMessage
			_ = json.Unmarshal(opts, &optsMap)
			_, hasTemperature = optsMap["temperature"]
		}

		_ = json.NewEncoder(w).Encode(ollamaSuccessResponse())
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

func TestOllama_IgnoresReasoningEffortAndExplicitZeroTemperature(t *testing.T) {
	cases := []struct {
		name   string
		stream bool
	}{
		{"Send", false},
		{"SendStream", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var err error
				gotBody, err = io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("failed to read request body: %v", err)
				}
				if tc.stream {
					_, _ = fmt.Fprintln(w, `{"message":{"content":"ok"},"done":true,"done_reason":"stop"}`)
					return
				}
				_ = json.NewEncoder(w).Encode(ollamaSuccessResponse())
			}))
			defer server.Close()

			o, err := NewOllama(WithOllamaBaseURL(server.URL))
			if err != nil {
				t.Fatalf("unexpected constructor error: %v", err)
			}
			req := &Request{
				Model:           "llama3",
				Messages:        []Message{{Role: "user", Content: "Hi"}},
				ReasoningEffort: "low",
				TemperatureSet:  true,
				Temperature:     0,
				MaxTokens:       100,
			}
			if tc.stream {
				stream, err := o.SendStream(context.Background(), req)
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
			} else if _, err := o.Send(context.Background(), req); err != nil {
				t.Fatalf("unexpected Send error: %v", err)
			}

			var body map[string]interface{}
			if err := json.Unmarshal(gotBody, &body); err != nil {
				t.Fatalf("failed to parse request body: %v", err)
			}
			if body["stream"] != tc.stream {
				t.Errorf("stream = %v, want %v", body["stream"], tc.stream)
			}
			opts, ok := body["options"].(map[string]interface{})
			if !ok || opts["num_predict"] != float64(100) {
				t.Errorf("options = %v, want num_predict 100", body["options"])
			}
			for _, key := range []string{"reasoning_effort", "temperature"} {
				if strings.Contains(string(gotBody), `"`+key+`"`) {
					t.Errorf("body contains %q: %s", key, gotBody)
				}
			}
		})
	}
}

func TestOllama_Send_OmitsZeroMaxTokens(t *testing.T) {
	var hasNumPredict bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		_ = json.Unmarshal(body, &raw)

		if opts, ok := raw["options"]; ok {
			var optsMap map[string]json.RawMessage
			_ = json.Unmarshal(opts, &optsMap)
			_, hasNumPredict = optsMap["num_predict"]
		}

		_ = json.NewEncoder(w).Encode(ollamaSuccessResponse())
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
		_ = json.Unmarshal(body, &raw)
		_, hasOptions = raw["options"]

		_ = json.NewEncoder(w).Encode(ollamaSuccessResponse())
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
		_ = json.Unmarshal(body, &raw)

		if opts, ok := raw["options"]; ok {
			hasOptions = true
			var optsMap map[string]json.RawMessage
			_ = json.Unmarshal(opts, &optsMap)
			_, hasTemperature = optsMap["temperature"]
			_, hasNumPredict = optsMap["num_predict"]
		}

		_ = json.NewEncoder(w).Encode(ollamaSuccessResponse())
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
		_, _ = w.Write([]byte(`{"error":"invalid model"}`))
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
		_, _ = w.Write([]byte(`{"error":"model 'xyz' not found"}`))
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
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
		_, _ = w.Write([]byte(`{"error":"model requires more memory"}`))
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
		_, _ = w.Write([]byte("not json"))
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
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
		_ = json.Unmarshal(body, &gotBody)
		_ = json.NewEncoder(w).Encode(ollamaSuccessResponse())
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
		_ = json.Unmarshal(body, &gotBody)
		_ = json.NewEncoder(w).Encode(ollamaSuccessResponse())
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
		_ = json.NewEncoder(w).Encode(resp)
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

func TestLargeIntegerArgs_Ollama_Buffered(t *testing.T) {
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
		// 10^308 has 309 digits and is finite: an independently specified cap boundary.
		{name: "eligible_309_digits", raw: `1e308`, want: "1" + strings.Repeat("0", 308)},
		{name: "negative_eligible_309_digits", raw: `-1e308`, want: "-1" + strings.Repeat("0", 308)},
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
		// Approved exceptional tokens A/B retain legacy text, without length rejection.
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
			// Arguments is a native JSON object: send raw numbers, not quoted helper values.
			body := `{"model":"llama3","message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"example","arguments":{"n":` + tt.raw + `}}}]},"done":true,"done_reason":"stop"}` + "\n"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/chat" {
					t.Errorf("request = %s %s, want POST /api/chat", r.Method, r.URL.Path)
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()
			o, err := NewOllama(WithOllamaBaseURL(server.URL))
			if err != nil {
				t.Fatalf("NewOllama: %v", err)
			}

			resp, err := o.Send(context.Background(), &Request{
				Model:    "llama3",
				Messages: []Message{{Role: "user", Content: "Read values"}},
				Tools:    []Tool{{Name: "example", Description: "Read values", Parameters: map[string]ToolParameter{}}},
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

func TestOllama_Send_NoToolCallsWithTools(t *testing.T) {
	// Model ignores tools and returns a normal text response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(ollamaSuccessResponse())
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
		_ = json.NewEncoder(w).Encode(ollamaSuccessResponse())
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
		_ = json.Unmarshal(req.Messages[idx], &msg)
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

// --- Streaming Tests ---

func TestOllama_SendStream_RequestFormat(t *testing.T) {
	var gotMethod, gotPath string
	var gotStream interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path

		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		_ = json.Unmarshal(body, &req)
		gotStream = req["stream"]

		// Return NDJSON: one text chunk then done
		_, _ = fmt.Fprintln(w, `{"message":{"content":"hi"},"done":false}`)
		_, _ = fmt.Fprintln(w, `{"message":{"content":""},"done":true,"done_reason":"stop","prompt_eval_count":5,"eval_count":2}`)
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	stream, err := o.SendStream(context.Background(), &Request{
		Model:    "llama3",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	// Drain the stream
	for {
		_, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected stream error: %v", err)
		}
	}

	if gotMethod != "POST" {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/api/chat" {
		t.Errorf("expected /api/chat, got %s", gotPath)
	}
	if gotStream != true {
		t.Errorf("expected stream=true, got %v", gotStream)
	}
}

func TestOllama_SendStream_TextDeltas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, `{"message":{"content":"Hello"},"done":false}`)
		_, _ = fmt.Fprintln(w, `{"message":{"content":" "},"done":false}`)
		_, _ = fmt.Fprintln(w, `{"message":{"content":"world"},"done":false}`)
		_, _ = fmt.Fprintln(w, `{"message":{"content":""},"done":true,"done_reason":"stop","prompt_eval_count":10,"eval_count":20}`)
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	stream, err := o.SendStream(context.Background(), &Request{
		Model:    "llama3",
		Messages: []Message{{Role: "user", Content: "Hi"}},
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

	// Expect 3 text events + 1 done
	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}

	if events[0].Type != StreamEventText || events[0].Text != "Hello" {
		t.Errorf("event 0: expected text 'Hello', got type=%s text=%q", events[0].Type, events[0].Text)
	}
	if events[1].Type != StreamEventText || events[1].Text != " " {
		t.Errorf("event 1: expected text ' ', got type=%s text=%q", events[1].Type, events[1].Text)
	}
	if events[2].Type != StreamEventText || events[2].Text != "world" {
		t.Errorf("event 2: expected text 'world', got type=%s text=%q", events[2].Type, events[2].Text)
	}
	if events[3].Type != StreamEventDone {
		t.Errorf("event 3: expected done, got type=%s", events[3].Type)
	}
	if events[3].StopReason != "stop" {
		t.Errorf("expected stop reason 'stop', got %q", events[3].StopReason)
	}
	if events[3].InputTokens != 10 {
		t.Errorf("expected 10 input tokens, got %d", events[3].InputTokens)
	}
	if events[3].OutputTokens != 20 {
		t.Errorf("expected 20 output tokens, got %d", events[3].OutputTokens)
	}
}

func TestOllama_SendStream_ToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, `{"message":{"content":"","tool_calls":[{"function":{"name":"get_weather","arguments":{"city":"NYC"}}},{"function":{"name":"get_time","arguments":{"tz":"EST"}}}]},"done":false}`)
		_, _ = fmt.Fprintln(w, `{"message":{"content":""},"done":true,"done_reason":"stop","prompt_eval_count":8,"eval_count":4}`)
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	stream, err := o.SendStream(context.Background(), &Request{
		Model:    "llama3",
		Messages: []Message{{Role: "user", Content: "What's the weather?"}},
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

	// 2 tool calls × (start + end) + 1 done = 5 events
	if len(events) != 5 {
		t.Fatalf("expected 5 events, got %d: %+v", len(events), events)
	}

	if events[0].Type != StreamEventToolStart || events[0].ToolName != "get_weather" {
		t.Errorf("event 0: expected tool_start get_weather, got type=%s name=%s", events[0].Type, events[0].ToolName)
	}
	if events[0].ToolCallID != "ollama_0" {
		t.Errorf("event 0: expected ID 'ollama_0', got %q", events[0].ToolCallID)
	}
	if events[1].Type != StreamEventToolEnd || events[1].ToolCallID != "ollama_0" {
		t.Errorf("event 1: expected tool_end ollama_0, got type=%s id=%s", events[1].Type, events[1].ToolCallID)
	}
	if events[2].Type != StreamEventToolStart || events[2].ToolName != "get_time" {
		t.Errorf("event 2: expected tool_start get_time, got type=%s name=%s", events[2].Type, events[2].ToolName)
	}
	if events[2].ToolCallID != "ollama_1" {
		t.Errorf("event 2: expected ID 'ollama_1', got %q", events[2].ToolCallID)
	}
	if events[3].Type != StreamEventToolEnd || events[3].ToolCallID != "ollama_1" {
		t.Errorf("event 3: expected tool_end ollama_1, got type=%s id=%s", events[3].Type, events[3].ToolCallID)
	}
	if events[4].Type != StreamEventDone {
		t.Errorf("event 4: expected done, got type=%s", events[4].Type)
	}
}

func TestLargeIntegerArgs_Ollama_NativeToolInput(t *testing.T) {
	// Arrange native JSON arguments at the local NDJSON boundary, not string-valued inputs.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = fmt.Fprintln(w, `{"message":{"content":"","tool_calls":[{"function":{"name":"example","arguments":{"million":1000000,"precise":9007199254740993,"enabled":true,"disabled":false,"nothing":null,"object":{"a":1000000},"array":[1,true,null,{"a":2}]}}}]},"done":false}`)
		_, _ = fmt.Fprintln(w, `{"message":{"content":""},"done":true,"done_reason":"stop","prompt_eval_count":8,"eval_count":4}`)
	}))
	defer server.Close()
	o, err := NewOllama(WithOllamaBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewOllama: %v", err)
	}

	// Act through the real provider and consume its public event stream to EOF.
	stream, err := o.SendStream(context.Background(), &Request{
		Model:    "llama3",
		Messages: []Message{{Role: "user", Content: "Read values"}},
	})
	if err != nil {
		t.Fatalf("SendStream: %v", err)
	}
	defer func() { _ = stream.Close() }()
	var inputs []string
	for {
		ev, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("stream.Next: %v", err)
		}
		if ev.Type == StreamEventToolStart {
			if ev.ToolName != "example" {
				t.Errorf("ToolName = %q, want example", ev.ToolName)
			}
			inputs = append(inputs, ev.ToolInput)
		}
	}

	// Assert raw tokens so neither float64 decoding nor string coercion can hide type/digit loss.
	if len(inputs) != 1 {
		t.Fatalf("got %d tool inputs, want 1", len(inputs))
	}
	var args map[string]json.RawMessage
	if err := json.Unmarshal([]byte(inputs[0]), &args); err != nil {
		t.Fatalf("ToolInput = %q, want JSON object: %v", inputs[0], err)
	}
	wants := []struct {
		key, token string
	}{
		{key: "million", token: `1000000`},
		{key: "precise", token: `9007199254740993`},
		{key: "enabled", token: `true`},
		{key: "disabled", token: `false`},
		{key: "nothing", token: `null`},
		{key: "object", token: `{"a":1000000}`},
		{key: "array", token: `[1,true,null,{"a":2}]`},
	}
	if len(args) != len(wants) {
		t.Errorf("ToolInput has %d arguments, want %d: %s", len(args), len(wants), inputs[0])
	}
	for _, want := range wants {
		if got := string(args[want.key]); got != want.token {
			t.Errorf("ToolInput[%s] = %s, want native JSON token %s (not a quoted/flattened string)", want.key, got, want.token)
		}
	}
}

func TestOllama_SendStream_EmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, `{"message":{"content":""},"done":false}`)
		_, _ = fmt.Fprintln(w, `{"message":{"content":"Hello"},"done":false}`)
		_, _ = fmt.Fprintln(w, `{"message":{"content":""},"done":false}`)
		_, _ = fmt.Fprintln(w, `{"message":{"content":""},"done":true,"done_reason":"stop","prompt_eval_count":5,"eval_count":3}`)
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	stream, err := o.SendStream(context.Background(), &Request{
		Model:    "llama3",
		Messages: []Message{{Role: "user", Content: "Hi"}},
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

	// Only 1 text event (empty content skipped) + 1 done = 2
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != StreamEventText || events[0].Text != "Hello" {
		t.Errorf("event 0: expected text 'Hello', got type=%s text=%q", events[0].Type, events[0].Text)
	}
	if events[1].Type != StreamEventDone {
		t.Errorf("event 1: expected done, got type=%s", events[1].Type)
	}
}

func TestOllama_SendStream_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"error":"internal error"}`))
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	_, err := o.SendStream(context.Background(), &Request{
		Model:    "llama3",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryServer {
		t.Errorf("expected ErrCategoryServer, got %s", provErr.Category)
	}
}

func TestOllama_SendStream_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, `not valid json at all`)
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	stream, err := o.SendStream(context.Background(), &Request{
		Model:    "llama3",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error from SendStream: %v", err)
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

func TestOllama_SendStream_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	stream, err := o.SendStream(ctx, &Request{
		Model:    "llama3",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		var provErr *ProviderError
		if !errors.As(err, &provErr) {
			t.Fatalf("expected ProviderError, got %T", err)
		}
		if provErr.Category != ErrCategoryTimeout {
			t.Errorf("expected ErrCategoryTimeout, got %s", provErr.Category)
		}
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
		var provErr *ProviderError
		if !errors.As(err, &provErr) {
			t.Fatalf("expected ProviderError, got %T: %v", err, err)
		}
		if provErr.Category != ErrCategoryTimeout {
			t.Errorf("expected ErrCategoryTimeout, got %s", provErr.Category)
		}
		break
	}
}

func TestOllama_Send_AssistantToolCallMessage(t *testing.T) {
	var gotBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		_ = json.NewEncoder(w).Encode(ollamaSuccessResponse())
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
	_ = json.Unmarshal(req.Messages[1], &assistantMsg)

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

func TestOllama_Send_OmitsFormatWhenEmpty(t *testing.T) {
	var hasFormat bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		_ = json.Unmarshal(body, &raw)
		_, hasFormat = raw["format"]

		resp := ollamaSuccessResponse()
		resp["done"] = true
		_ = json.NewEncoder(w).Encode(resp)
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
	if hasFormat {
		t.Error("expected format to be omitted when not set")
	}
}

func TestOllama_Send_IncludesFormatJsonObject(t *testing.T) {
	var gotFormat string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		_ = json.Unmarshal(body, &raw)
		if f, ok := raw["format"]; ok {
			_ = json.Unmarshal(f, &gotFormat)
		}

		resp := ollamaSuccessResponse()
		resp["done"] = true
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:          "llama3",
		Messages:       []Message{{Role: "user", Content: "Hi"}},
		ResponseFormat: ResponseFormat{Type: "json_object"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotFormat != "json" {
		t.Errorf("expected format 'json', got %q", gotFormat)
	}
}

func TestOllama_Send_IncludesFormatJsonSchema(t *testing.T) {
	var gotFormat map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		_ = json.Unmarshal(body, &raw)
		if f, ok := raw["format"]; ok {
			_ = json.Unmarshal(f, &gotFormat)
		}

		resp := ollamaSuccessResponse()
		resp["done"] = true
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:    "llama3",
		Messages: []Message{{Role: "user", Content: "Hi"}},
		ResponseFormat: ResponseFormat{
			Type: "json_schema",
			Schema: map[string]interface{}{
				"name": "extraction",
				"type": "object",
				"properties": map[string]interface{}{
					"result": map[string]interface{}{"type": "string"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotFormat == nil {
		t.Fatal("expected format to be a schema object")
	}
	if gotFormat["type"] != "object" {
		t.Errorf("expected schema type 'object', got %v", gotFormat["type"])
	}
	if _, hasName := gotFormat["name"]; hasName {
		t.Error("expected 'name' to be stripped from Ollama schema (it's not part of JSON Schema)")
	}
}

func TestLargeIntegerArgs_Ollama_BufferedErrors(t *testing.T) {
	bodyFor := func(args string) string {
		return `{"model":"llama3","message":{"tool_calls":[{"function":{"name":"example","arguments":` + args + `}}]},"prompt_eval_count":10}`
	}
	base := bodyFor(`{"n":"text"}`)
	overflow := "1" + strings.Repeat("0", 399)
	tests := []struct {
		name, body string
		want       map[string]string
		wantErr    bool
	}{
		{name: "malformed_response", body: `{"message":`, wantErr: true},
		{name: "malformed_arguments", body: bodyFor(`{"n":`), wantErr: true},
		{name: "trailing_value", body: base + `{}`, wantErr: true},
		{name: "trailing_junk", body: base + `junk`, wantErr: true},
		{name: "scalar_overflow", body: bodyFor(`{"n":1e400}`), wantErr: true},
		{name: "scalar_400_digits", body: bodyFor(`{"n":` + overflow + `}`), wantErr: true},
		{name: "nested_overflow", body: bodyFor(`{"n":[{"bad":1e400}]}`), wantErr: true},
		{name: "nested_400_digits", body: bodyFor(`{"n":{"bad":` + overflow + `}}`), wantErr: true},
		{name: "overwritten_scalar_overflow", body: bodyFor(`{"n":1e400,"n":1000000}`), wantErr: true},
		{name: "replaced_subtree_overflow", body: bodyFor(`{"n":{"bad":1e400},"n":1000000}`), wantErr: true},
		{name: "overwritten_scalar_400_digits", body: bodyFor(`{"n":` + overflow + `,"n":1000000}`), wantErr: true},
		{name: "replaced_subtree_400_digits", body: bodyFor(`{"n":{"bad":` + overflow + `},"n":1000000}`), wantErr: true},
		{name: "successful_scalar_duplicate", body: bodyFor(`{"n":999999,"n":1000000}`), want: map[string]string{"n": "1000000"}},
		{name: "successful_subtree_duplicate", body: bodyFor(`{"n":{"ok":1},"n":1000000}`), want: map[string]string{"n": "1000000"}},
		{name: "empty_map", body: bodyFor(`{}`), want: map[string]string{}},
		{name: "null_map", body: bodyFor(`null`), want: map[string]string{}},
		{name: "array_map", body: bodyFor(`[]`), wantErr: true},
		{name: "number_map", body: bodyFor(`0`), wantErr: true},
		{name: "negative_zero_map", body: bodyFor(`-0`), wantErr: true},
		{name: "decimal_zero_map", body: bodyFor(`-0.0`), wantErr: true},
		{name: "string_map", body: bodyFor(`"text"`), wantErr: true},
		{name: "bool_map", body: bodyFor(`true`), wantErr: true},
		{name: "known_count_overflow", body: `{"eval_count":1e400,` + base[1:], wantErr: true},
		{name: "known_count_400_digits", body: `{"eval_count":` + overflow + `,` + base[1:], wantErr: true},
		{name: "overwritten_count_overflow", body: `{"prompt_eval_count":1e400,` + base[1:], wantErr: true},
		{name: "unknown_scalar_overflow", body: `{"unknown":1e400,` + base[1:], want: map[string]string{"n": "text"}},
		{name: "unknown_nested_overflow", body: `{"unknown":{"bad":1e400},` + base[1:], want: map[string]string{"n": "text"}},
		{name: "unknown_400_digits", body: `{"unknown":` + overflow + `,` + base[1:], want: map[string]string{"n": "text"}},
		{name: "unknown_nested_400_digits", body: `{"unknown":{"bad":` + overflow + `},` + base[1:], want: map[string]string{"n": "text"}},
		{name: "unknown_message_overflow", body: `{"message":{"unknown":[1e400],"tool_calls":[{"function":{"name":"example","arguments":{"n":"text"}}}]}}`, want: map[string]string{"n": "text"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()
			o, err := NewOllama(WithOllamaBaseURL(server.URL))
			if err != nil {
				t.Fatal(err)
			}
			resp, err := o.Send(context.Background(), &Request{Model: "llama3", Messages: []Message{{Role: "user", Content: "Read values"}}})
			if tt.wantErr {
				var pe *ProviderError
				if resp != nil || !errors.As(err, &pe) || pe.Category != ErrCategoryServer || !strings.HasPrefix(pe.Message, "failed to parse response: ") {
					t.Fatalf("Send = %+v, %v; want nil response and parse server ProviderError", resp, err)
				}
				return
			}
			if err != nil || resp == nil || len(resp.ToolCalls) != 1 {
				t.Fatalf("Send = %+v, %v; want one call", resp, err)
			}
			got := resp.ToolCalls[0].Arguments
			if len(got) != len(tt.want) {
				t.Errorf("arguments = %v, want %v", got, tt.want)
			}
			for key, want := range tt.want {
				if value, ok := got[key]; !ok || value != want {
					t.Errorf("arguments[%s] = %q (present=%t), want %q", key, value, ok, want)
				}
			}
			for _, value := range got {
				if strings.Contains(value, "+Inf") {
					t.Errorf("arguments contain +Inf: %v", got)
				}
			}
		})
	}
}

func TestLargeIntegerArgs_Ollama_StreamErrors(t *testing.T) {
	bodyFor := func(args string) string {
		return `{"message":{"tool_calls":[{"function":{"name":"example","arguments":` + args + `}}]},"done":false}`
	}
	base := bodyFor(`{"n":"text"}`)
	overflow := "1" + strings.Repeat("0", 399)
	tests := []struct {
		name, body, want string
		wantErr          bool
	}{
		{name: "malformed_response", body: `{"message":`, wantErr: true},
		{name: "malformed_arguments", body: bodyFor(`{"n":`), wantErr: true},
		{name: "trailing_value", body: base + `{}`, wantErr: true},
		{name: "trailing_junk", body: base + `junk`, wantErr: true},
		{name: "scalar_overflow", body: bodyFor(`{"n":1e400}`), wantErr: true},
		{name: "scalar_400_digits", body: bodyFor(`{"n":` + overflow + `}`), wantErr: true},
		{name: "nested_overflow", body: bodyFor(`{"n":[{"bad":1e400}]}`), wantErr: true},
		{name: "nested_400_digits", body: bodyFor(`{"n":{"bad":` + overflow + `}}`), wantErr: true},
		{name: "overwritten_scalar_overflow", body: bodyFor(`{"n":1e400,"n":1000000}`), wantErr: true},
		{name: "replaced_subtree_overflow", body: bodyFor(`{"n":{"bad":1e400},"n":1000000}`), wantErr: true},
		{name: "overwritten_scalar_400_digits", body: bodyFor(`{"n":` + overflow + `,"n":1000000}`), wantErr: true},
		{name: "replaced_subtree_400_digits", body: bodyFor(`{"n":{"bad":` + overflow + `},"n":1000000}`), wantErr: true},
		{name: "successful_scalar_duplicate", body: bodyFor(`{"n":999999,"n":1000000}`), want: `{"n":1000000}`},
		{name: "successful_subtree_duplicate", body: bodyFor(`{"n":{"ok":1},"n":1000000}`), want: `{"n":1000000}`},
		{name: "empty_map", body: bodyFor(`{}`), want: `{}`},
		{name: "null_map", body: bodyFor(`null`), want: `null`},
		{name: "array_map", body: bodyFor(`[]`), wantErr: true},
		{name: "number_map", body: bodyFor(`0`), wantErr: true},
		{name: "negative_zero_map", body: bodyFor(`-0`), wantErr: true},
		{name: "decimal_zero_map", body: bodyFor(`-0.0`), wantErr: true},
		{name: "string_map", body: bodyFor(`"text"`), wantErr: true},
		{name: "bool_map", body: bodyFor(`true`), wantErr: true},
		{name: "known_count_overflow", body: `{"eval_count":1e400,` + base[1:], wantErr: true},
		{name: "known_count_400_digits", body: `{"eval_count":` + overflow + `,` + base[1:], wantErr: true},
		{name: "overwritten_count_overflow", body: `{"prompt_eval_count":1e400,"prompt_eval_count":7,` + base[1:], wantErr: true},
		{name: "unknown_scalar_overflow", body: `{"unknown":1e400,` + base[1:], want: `{"n":"text"}`},
		{name: "unknown_nested_overflow", body: `{"unknown":{"bad":1e400},` + base[1:], want: `{"n":"text"}`},
		{name: "unknown_400_digits", body: `{"unknown":` + overflow + `,` + base[1:], want: `{"n":"text"}`},
		{name: "unknown_nested_400_digits", body: `{"unknown":{"bad":` + overflow + `},` + base[1:], want: `{"n":"text"}`},
		{name: "unknown_message_overflow", body: `{"message":{"unknown":[1e400],"tool_calls":[{"function":{"name":"example","arguments":{"n":"text"}}}]}}`, want: `{"n":"text"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprintln(w, tt.body)
			}))
			defer server.Close()
			o, err := NewOllama(WithOllamaBaseURL(server.URL))
			if err != nil {
				t.Fatal(err)
			}
			stream, err := o.SendStream(context.Background(), &Request{Model: "llama3", Messages: []Message{{Role: "user", Content: "Read values"}}})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = stream.Close() }()
			ev, err := stream.Next()
			if tt.wantErr {
				var pe *ProviderError
				if !errors.As(err, &pe) || pe.Category != ErrCategoryServer || !strings.HasPrefix(pe.Message, "failed to parse streaming chunk: ") {
					t.Fatalf("Next = %+v, %v; want parse server ProviderError", ev, err)
				}
				return
			}
			if err != nil || ev.Type != StreamEventToolStart || ev.ToolCallID != "ollama_0" || ev.ToolName != "example" || ev.ToolInput != tt.want {
				t.Fatalf("Next = %+v, %v; want native input %s", ev, err, tt.want)
			}
		})
	}
}

func TestOllama_Send_JsonSchemaWithoutSchemaReturnsError(t *testing.T) {
	o, _ := NewOllama(WithOllamaBaseURL("http://localhost:11434"))
	_, err := o.Send(context.Background(), &Request{
		Model:          "llama3",
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
