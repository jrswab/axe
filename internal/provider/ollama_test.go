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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/chat" {
			t.Errorf("expected /api/chat, got %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected 'application/json', got %q", ct)
		}
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("expected no Authorization header, got %q", auth)
		}

		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		json.Unmarshal(body, &req)

		if req["stream"] != false {
			t.Errorf("expected stream=false, got %v", req["stream"])
		}

		json.NewEncoder(w).Encode(ollamaSuccessResponse())
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	o.Send(context.Background(), &Request{
		Model:    "llama3",
		System:   "Be helpful",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
}

func TestOllama_Send_SystemMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		json.Unmarshal(body, &req)

		msgs := req["messages"].([]interface{})
		if len(msgs) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(msgs))
		}
		first := msgs[0].(map[string]interface{})
		if first["role"] != "system" {
			t.Errorf("expected first message role 'system', got %v", first["role"])
		}

		json.NewEncoder(w).Encode(ollamaSuccessResponse())
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	o.Send(context.Background(), &Request{
		Model:    "llama3",
		System:   "Be helpful",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
}

func TestOllama_Send_OmitsEmptySystem(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		json.Unmarshal(body, &req)

		msgs := req["messages"].([]interface{})
		if len(msgs) != 1 {
			t.Errorf("expected 1 message, got %d", len(msgs))
		}

		json.NewEncoder(w).Encode(ollamaSuccessResponse())
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	o.Send(context.Background(), &Request{
		Model:    "llama3",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
}

func TestOllama_Send_OmitsZeroTemperature(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		json.Unmarshal(body, &raw)

		if opts, ok := raw["options"]; ok {
			var optsMap map[string]json.RawMessage
			json.Unmarshal(opts, &optsMap)
			if _, has := optsMap["temperature"]; has {
				t.Error("expected temperature to be omitted")
			}
		}

		json.NewEncoder(w).Encode(ollamaSuccessResponse())
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	o.Send(context.Background(), &Request{
		Model:       "llama3",
		Messages:    []Message{{Role: "user", Content: "Hi"}},
		Temperature: 0,
		MaxTokens:   100,
	})
}

func TestOllama_Send_OmitsZeroMaxTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		json.Unmarshal(body, &raw)

		if opts, ok := raw["options"]; ok {
			var optsMap map[string]json.RawMessage
			json.Unmarshal(opts, &optsMap)
			if _, has := optsMap["num_predict"]; has {
				t.Error("expected num_predict to be omitted")
			}
		}

		json.NewEncoder(w).Encode(ollamaSuccessResponse())
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	o.Send(context.Background(), &Request{
		Model:       "llama3",
		Messages:    []Message{{Role: "user", Content: "Hi"}},
		Temperature: 0.7,
		MaxTokens:   0,
	})
}

func TestOllama_Send_OmitsOptionsWhenEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		json.Unmarshal(body, &raw)

		if _, ok := raw["options"]; ok {
			t.Error("expected options to be omitted entirely")
		}

		json.NewEncoder(w).Encode(ollamaSuccessResponse())
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	o.Send(context.Background(), &Request{
		Model:       "llama3",
		Messages:    []Message{{Role: "user", Content: "Hi"}},
		Temperature: 0,
		MaxTokens:   0,
	})
}

func TestOllama_Send_IncludesOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		json.Unmarshal(body, &raw)

		opts, ok := raw["options"]
		if !ok {
			t.Fatal("expected options to be present")
		}
		var optsMap map[string]json.RawMessage
		json.Unmarshal(opts, &optsMap)

		if _, has := optsMap["temperature"]; !has {
			t.Error("expected temperature in options")
		}
		if _, has := optsMap["num_predict"]; !has {
			t.Error("expected num_predict in options")
		}

		json.NewEncoder(w).Encode(ollamaSuccessResponse())
	}))
	defer server.Close()

	o, _ := NewOllama(WithOllamaBaseURL(server.URL))
	o.Send(context.Background(), &Request{
		Model:       "llama3",
		Messages:    []Message{{Role: "user", Content: "Hi"}},
		Temperature: 0.7,
		MaxTokens:   512,
	})
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
