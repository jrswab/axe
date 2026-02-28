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
