package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- Constructor tests ---

func TestNewOpenRouter(t *testing.T) {
	cases := []struct {
		name         string
		apiKey       string
		opts         []OpenRouterOption
		wantErr      bool
		wantErrMsg   string
		wantBaseURL  string
		wantReferer  string
		wantTitle    string
		wantCategories string
	}{
		{
			name:       "empty API key errors",
			apiKey:     "",
			wantErr:    true,
			wantErrMsg: "API key is required",
		},
		{
			name:        "valid API key",
			apiKey:      "test-key",
			wantBaseURL: defaultOpenRouterBaseURL,
		},
		{
			name:        "custom base URL",
			apiKey:      "test-key",
			opts:        []OpenRouterOption{WithOpenRouterBaseURL("https://custom.example.com/v1")},
			wantBaseURL: "https://custom.example.com/v1",
		},
		{
			name:           "all options set",
			apiKey:         "test-key",
			opts:           []OpenRouterOption{WithReferer("https://example.com"), WithTitle("My App"), WithCategories("cli,agent")},
			wantBaseURL:    defaultOpenRouterBaseURL,
			wantReferer:    "https://example.com",
			wantTitle:      "My App",
			wantCategories: "cli,agent",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewOpenRouter(tc.apiKey, tc.opts...)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if !strings.Contains(err.Error(), tc.wantErrMsg) {
					t.Errorf("expected error containing %q, got %q", tc.wantErrMsg, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if o == nil {
				t.Fatal("expected non-nil OpenRouter")
			}
			if tc.wantBaseURL != "" && o.baseURL != tc.wantBaseURL {
				t.Errorf("baseURL = %q, want %q", o.baseURL, tc.wantBaseURL)
			}
			if o.referer != tc.wantReferer {
				t.Errorf("referer = %q, want %q", o.referer, tc.wantReferer)
			}
			if o.title != tc.wantTitle {
				t.Errorf("title = %q, want %q", o.title, tc.wantTitle)
			}
			if o.categories != tc.wantCategories {
				t.Errorf("categories = %q, want %q", o.categories, tc.wantCategories)
			}
		})
	}
}

// --- Non-streaming Send tests ---

func TestOpenRouter_Send_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-OpenRouter-Cache-Status", "HIT")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"model": "anthropic/claude-sonnet-4",
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{"content": "Hello from OpenRouter"},
					"finish_reason": "stop",
					"native_finish_reason": "end_turn",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"cost":              0.00014,
			},
		})
	}))
	defer server.Close()

	o, _ := NewOpenRouter("test-key", WithOpenRouterBaseURL(server.URL))
	resp, err := o.Send(context.Background(), &Request{
		Model:    "anthropic/claude-sonnet-4",
		System:   "You are helpful.",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Hello from OpenRouter" {
		t.Errorf("expected 'Hello from OpenRouter', got %q", resp.Content)
	}
	if resp.Model != "anthropic/claude-sonnet-4" {
		t.Errorf("expected model 'anthropic/claude-sonnet-4', got %q", resp.Model)
	}
	if resp.InputTokens != 10 {
		t.Errorf("expected 10 input tokens, got %d", resp.InputTokens)
	}
	if resp.OutputTokens != 5 {
		t.Errorf("expected 5 output tokens, got %d", resp.OutputTokens)
	}
	if resp.Cost != 0.00014 {
		t.Errorf("expected cost 0.00014, got %f", resp.Cost)
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("expected native stop reason 'end_turn', got %q", resp.StopReason)
	}
	if resp.CacheStatus != "HIT" {
		t.Errorf("expected cache status 'HIT', got %q", resp.CacheStatus)
	}
}

func TestOpenRouter_Send_RequestFormat(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotCT, gotModel string
	var gotBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")

		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		gotModel, _ = gotBody["model"].(string)

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "OK"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenRouter("test-key", WithOpenRouterBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:    "openai/gpt-5",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != "POST" {
		t.Errorf("expected POST, got %q", gotMethod)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("expected path /chat/completions, got %q", gotPath)
	}
	if !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Errorf("expected Bearer auth, got %q", gotAuth)
	}
	if gotCT != "application/json" {
		t.Errorf("expected application/json, got %q", gotCT)
	}
	if gotModel != "openai/gpt-5" {
		t.Errorf("expected model 'openai/gpt-5', got %q", gotModel)
	}
}

func TestOpenRouter_Send_AttributionHeaders(t *testing.T) {
	var gotReferer, gotTitle, gotCategories string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReferer = r.Header.Get("HTTP-Referer")
		gotTitle = r.Header.Get("X-OpenRouter-Title")
		gotCategories = r.Header.Get("X-OpenRouter-Categories")

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "OK"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenRouter("test-key",
		WithOpenRouterBaseURL(server.URL),
		WithReferer("https://github.com/jrswab/axe"),
		WithTitle("Axe CLI"),
		WithCategories("cli-agent"),
	)
	_, err := o.Send(context.Background(), &Request{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotReferer != "https://github.com/jrswab/axe" {
		t.Errorf("referer = %q, want %q", gotReferer, "https://github.com/jrswab/axe")
	}
	if gotTitle != "Axe CLI" {
		t.Errorf("title = %q, want %q", gotTitle, "Axe CLI")
	}
	if gotCategories != "cli-agent" {
		t.Errorf("categories = %q, want %q", gotCategories, "cli-agent")
	}
}

func TestOpenRouter_Send_NoAttributionHeadersByDefault(t *testing.T) {
	var gotReferer, gotTitle, gotCategories string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReferer = r.Header.Get("HTTP-Referer")
		gotTitle = r.Header.Get("X-OpenRouter-Title")
		gotCategories = r.Header.Get("X-OpenRouter-Categories")

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "OK"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenRouter("test-key", WithOpenRouterBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotReferer != "" {
		t.Errorf("expected no referer, got %q", gotReferer)
	}
	if gotTitle != "" {
		t.Errorf("expected no title, got %q", gotTitle)
	}
	if gotCategories != "" {
		t.Errorf("expected no categories, got %q", gotCategories)
	}
}

func TestOpenRouter_Send_GracefulDegradation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "OK"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{
				"prompt_tokens":     1,
				"completion_tokens": 1,
				// cost omitted intentionally
			},
		})
	}))
	defer server.Close()

	o, _ := NewOpenRouter("test-key", WithOpenRouterBaseURL(server.URL))
	resp, err := o.Send(context.Background(), &Request{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Cost != 0 {
		t.Errorf("expected cost 0 when absent, got %f", resp.Cost)
	}
	if resp.CacheStatus != "" {
		t.Errorf("expected empty cache status when header absent, got %q", resp.CacheStatus)
	}
}

func TestOpenRouter_Send_ErrorResponses(t *testing.T) {
	tests := []struct {
		status   int
		wantCat  ErrorCategory
	}{
		{400, ErrCategoryBadRequest},
		{401, ErrCategoryAuth},
		{403, ErrCategoryAuth},
		{404, ErrCategoryBadRequest},
		{429, ErrCategoryRateLimit},
		{500, ErrCategoryServer},
		{502, ErrCategoryServer},
		{503, ErrCategoryServer},
	}

	for _, tc := range tests {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error": map[string]string{"message": "something went wrong"},
				})
			}))
			defer server.Close()

			o, _ := NewOpenRouter("test-key", WithOpenRouterBaseURL(server.URL))
			_, err := o.Send(context.Background(), &Request{
				Model:    "test-model",
				Messages: []Message{{Role: "user", Content: "Hello"}},
			})
			if err == nil {
				t.Fatal("expected error")
			}
			provErr, ok := err.(*ProviderError)
			if !ok {
				t.Fatalf("expected *ProviderError, got %T", err)
			}
			if provErr.Category != tc.wantCat {
				t.Errorf("category = %q, want %q", provErr.Category, tc.wantCat)
			}
		})
	}
}

// --- Streaming tests ---

func TestOpenRouter_SendStream_TextDeltas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("X-OpenRouter-Cache-Status", "MISS")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected flusher")
		}

		events := []string{
			`data: {"choices":[{"delta":{"content":"Hello"},"finish_reason":null}],"model":"openai/gpt-5"}`,
			`data: {"choices":[{"delta":{"content":" world"},"finish_reason":null}],"model":"openai/gpt-5"}`,
			`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`,
			`data: {"choices":[],"usage":{"prompt_tokens":3,"completion_tokens":2,"cost":0.00001}}`,
			`data: [DONE]`,
		}
		for _, ev := range events {
			_, _ = w.Write([]byte(ev + "\n\n"))
			flusher.Flush()
		}
	}))
	defer server.Close()

	o, _ := NewOpenRouter("test-key", WithOpenRouterBaseURL(server.URL))
	stream, err := o.SendStream(context.Background(), &Request{
		Model:    "openai/gpt-5",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var texts []string
	var doneEvent *StreamEvent
	for {
		ev, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("stream error: %v", err)
		}
		if ev.Type == StreamEventText {
			texts = append(texts, ev.Text)
		}
		if ev.Type == StreamEventDone {
			doneEvent = &ev
		}
	}

	if len(texts) != 2 || texts[0] != "Hello" || texts[1] != " world" {
		t.Errorf("texts = %v, want [Hello,  world]", texts)
	}
	if doneEvent == nil {
		t.Fatal("expected StreamEventDone")
	}
	if doneEvent.InputTokens != 3 {
		t.Errorf("input tokens = %d, want 3", doneEvent.InputTokens)
	}
	if doneEvent.OutputTokens != 2 {
		t.Errorf("output tokens = %d, want 2", doneEvent.OutputTokens)
	}
	if doneEvent.Cost != 0.00001 {
		t.Errorf("cost = %f, want 0.00001", doneEvent.Cost)
	}
	if doneEvent.CacheStatus != "MISS" {
		t.Errorf("cache status = %q, want MISS", doneEvent.CacheStatus)
	}
}

func TestOpenRouter_SendStream_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{"message": "invalid key"},
		})
	}))
	defer server.Close()

	o, _ := NewOpenRouter("test-key", WithOpenRouterBaseURL(server.URL))
	_, err := o.SendStream(context.Background(), &Request{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	provErr, ok := err.(*ProviderError)
	if !ok {
		t.Fatalf("expected *ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryAuth {
		t.Errorf("category = %q, want %q", provErr.Category, ErrCategoryAuth)
	}
}

func TestOpenRouter_SendStream_ToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected flusher")
		}

		events := []string{
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"run_command"}}]}}]}`,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"command\":\"echo hi"}}]}}]}`,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"}"}}]}}]}`,
			`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
			`data: {"usage":{"prompt_tokens":10,"completion_tokens":5}}`,
			`data: [DONE]`,
		}
		for _, ev := range events {
			_, _ = w.Write([]byte(ev + "\n\n"))
			flusher.Flush()
		}
	}))
	defer server.Close()

	o, _ := NewOpenRouter("test-key", WithOpenRouterBaseURL(server.URL))
	stream, err := o.SendStream(context.Background(), &Request{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: "Run echo hi"}},
		Tools: []Tool{{
			Name:        "run_command",
			Description: "Run a shell command",
			Parameters: map[string]ToolParameter{
				"command": {Type: "string", Description: "command to run", Required: true},
			},
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var toolStarts int
	var toolEnds int
	var doneEvent *StreamEvent
	for {
		ev, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("stream error: %v", err)
		}
		switch ev.Type {
		case StreamEventToolStart:
			toolStarts++
		case StreamEventToolEnd:
			toolEnds++
		case StreamEventDone:
			doneEvent = &ev
		}
	}

	if toolStarts != 1 {
		t.Errorf("tool starts = %d, want 1", toolStarts)
	}
	if toolEnds != 1 {
		t.Errorf("tool ends = %d, want 1", toolEnds)
	}
	if doneEvent == nil {
		t.Fatal("expected StreamEventDone")
	}
}
