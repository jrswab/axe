package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jrswab/axe/internal/provider"
)

func TestWebSearch_EmptyQuery(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "test-key")

	entry := webSearchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "ws-empty", Arguments: map[string]string{}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	if !strings.Contains(result.Content, "query is required") {
		t.Errorf("Content = %q, want contains %q", result.Content, "query is required")
	}
}

func TestWebSearch_MissingQueryArgument(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "test-key")

	entry := webSearchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "ws-missing-query", Arguments: map[string]string{"query": ""}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	if !strings.Contains(result.Content, "query is required") {
		t.Errorf("Content = %q, want contains %q", result.Content, "query is required")
	}
}

func TestWebSearch_MissingAPIKey(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "")

	entry := webSearchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "ws-missing-key", Arguments: map[string]string{"query": "test"}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	if !strings.Contains(result.Content, "TAVILY_API_KEY environment variable is not set") {
		t.Errorf("Content = %q, want contains %q", result.Content, "TAVILY_API_KEY environment variable is not set")
	}
}

func TestWebSearch_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"title":"Result 1","url":"https://example.com/1","content":"Snippet 1"},{"title":"Result 2","url":"https://example.com/2","content":"Snippet 2"}]}`))
	}))
	defer ts.Close()

	t.Setenv("TAVILY_API_KEY", "test-key")
	t.Setenv("AXE_TAVILY_BASE_URL", ts.URL)

	entry := webSearchEntry()
	call := provider.ToolCall{ID: "ws-success", Arguments: map[string]string{"query": "test query"}}
	result := entry.Execute(context.Background(), call, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	if !strings.Contains(result.Content, "Title: Result 1") {
		t.Errorf("Content = %q, want contains %q", result.Content, "Title: Result 1")
	}
	if !strings.Contains(result.Content, "URL: https://example.com/1") {
		t.Errorf("Content = %q, want contains %q", result.Content, "URL: https://example.com/1")
	}
	if !strings.Contains(result.Content, "Snippet: Snippet 1") {
		t.Errorf("Content = %q, want contains %q", result.Content, "Snippet: Snippet 1")
	}
	if !strings.Contains(result.Content, "Title: Result 2") {
		t.Errorf("Content = %q, want contains %q", result.Content, "Title: Result 2")
	}
	if result.CallID != call.ID {
		t.Errorf("CallID = %q, want %q", result.CallID, call.ID)
	}
}

func TestWebSearch_CallIDPassthrough(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"title":"Result","url":"https://example.com","content":"Snippet"}]}`))
	}))
	defer ts.Close()

	t.Setenv("TAVILY_API_KEY", "test-key")
	t.Setenv("AXE_TAVILY_BASE_URL", ts.URL)

	entry := webSearchEntry()
	call := provider.ToolCall{ID: "ws-unique-42", Arguments: map[string]string{"query": "test"}}
	result := entry.Execute(context.Background(), call, ExecContext{})

	if result.CallID != "ws-unique-42" {
		t.Errorf("CallID = %q, want %q", result.CallID, "ws-unique-42")
	}
}

func TestWebSearch_EmptyResults(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer ts.Close()

	t.Setenv("TAVILY_API_KEY", "test-key")
	t.Setenv("AXE_TAVILY_BASE_URL", ts.URL)

	entry := webSearchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "ws-empty-results", Arguments: map[string]string{"query": "test"}}, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	if result.Content != "No results found." {
		t.Errorf("Content = %q, want %q", result.Content, "No results found.")
	}
}

func TestWebSearch_NullResults(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":null}`))
	}))
	defer ts.Close()

	t.Setenv("TAVILY_API_KEY", "test-key")
	t.Setenv("AXE_TAVILY_BASE_URL", ts.URL)

	entry := webSearchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "ws-null-results", Arguments: map[string]string{"query": "test"}}, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	if result.Content != "No results found." {
		t.Errorf("Content = %q, want %q", result.Content, "No results found.")
	}
}

func TestWebSearch_HTTPError_401(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":{"error":"Invalid API key"}}`))
	}))
	defer ts.Close()

	t.Setenv("TAVILY_API_KEY", "bad-key")
	t.Setenv("AXE_TAVILY_BASE_URL", ts.URL)

	entry := webSearchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "ws-401", Arguments: map[string]string{"query": "test"}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	if !strings.Contains(result.Content, "Tavily API error (HTTP 401)") {
		t.Errorf("Content = %q, want contains %q", result.Content, "Tavily API error (HTTP 401)")
	}
	if !strings.Contains(result.Content, "Invalid API key") {
		t.Errorf("Content = %q, want contains %q", result.Content, "Invalid API key")
	}
}

func TestWebSearch_HTTPError_429(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"detail":{"error":"Rate limit exceeded"}}`))
	}))
	defer ts.Close()

	t.Setenv("TAVILY_API_KEY", "test-key")
	t.Setenv("AXE_TAVILY_BASE_URL", ts.URL)

	entry := webSearchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "ws-429", Arguments: map[string]string{"query": "test"}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	if !strings.Contains(result.Content, "Tavily API error (HTTP 429)") {
		t.Errorf("Content = %q, want contains %q", result.Content, "Tavily API error (HTTP 429)")
	}
}

func TestWebSearch_HTTPError_500(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer ts.Close()

	t.Setenv("TAVILY_API_KEY", "test-key")
	t.Setenv("AXE_TAVILY_BASE_URL", ts.URL)

	entry := webSearchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "ws-500", Arguments: map[string]string{"query": "test"}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	if !strings.Contains(result.Content, "Tavily API error (HTTP 500)") {
		t.Errorf("Content = %q, want contains %q", result.Content, "Tavily API error (HTTP 500)")
	}
	if !strings.Contains(result.Content, "internal error") {
		t.Errorf("Content = %q, want contains %q", result.Content, "internal error")
	}
}

func TestWebSearch_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer ts.Close()

	t.Setenv("TAVILY_API_KEY", "test-key")
	t.Setenv("AXE_TAVILY_BASE_URL", ts.URL)

	entry := webSearchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "ws-invalid-json", Arguments: map[string]string{"query": "test"}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	if !strings.Contains(result.Content, "failed to parse Tavily response") {
		t.Errorf("Content = %q, want contains %q", result.Content, "failed to parse Tavily response")
	}
}

func TestWebSearch_ContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()

		select {
		case <-r.Context().Done():
			return
		case <-timer.C:
			_, _ = w.Write([]byte(`{"results":[]}`))
		}
	}))
	defer ts.Close()

	t.Setenv("TAVILY_API_KEY", "test-key")
	t.Setenv("AXE_TAVILY_BASE_URL", ts.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	entry := webSearchEntry()
	result := entry.Execute(ctx, provider.ToolCall{ID: "ws-timeout", Arguments: map[string]string{"query": "test"}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
}

func TestWebSearch_ConnectionRefused(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("listener.Close error: %v", err)
	}

	t.Setenv("TAVILY_API_KEY", "test-key")
	t.Setenv("AXE_TAVILY_BASE_URL", "http://"+addr)

	entry := webSearchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "ws-refused", Arguments: map[string]string{"query": "test"}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	if result.Content == "" {
		t.Fatal("expected non-empty error content")
	}
}

func TestWebSearch_BaseURLOverride(t *testing.T) {
	reqPath := ""
	reqMethod := ""
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqPath = r.URL.Path
		reqMethod = r.Method
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer ts.Close()

	t.Setenv("TAVILY_API_KEY", "test-key")
	t.Setenv("AXE_TAVILY_BASE_URL", ts.URL)

	entry := webSearchEntry()
	_ = entry.Execute(context.Background(), provider.ToolCall{ID: "ws-base-url", Arguments: map[string]string{"query": "test"}}, ExecContext{})

	if reqPath != "/search" {
		t.Errorf("request path = %q, want %q", reqPath, "/search")
	}
	if reqMethod != http.MethodPost {
		t.Errorf("request method = %q, want %q", reqMethod, http.MethodPost)
	}
}

func TestWebSearch_RequestBody(t *testing.T) {
	type captured struct {
		Query       string `json:"query"`
		MaxResults  int    `json:"max_results"`
		SearchDepth string `json:"search_depth"`
		APIKey      string `json:"api_key"`
	}

	var got captured
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll error: %v", err)
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer ts.Close()

	t.Setenv("TAVILY_API_KEY", "test-key")
	t.Setenv("AXE_TAVILY_BASE_URL", ts.URL)

	entry := webSearchEntry()
	_ = entry.Execute(context.Background(), provider.ToolCall{ID: "ws-body", Arguments: map[string]string{"query": "my search"}}, ExecContext{})

	if got.Query != "my search" {
		t.Errorf("query = %q, want %q", got.Query, "my search")
	}
	if got.MaxResults != 10 {
		t.Errorf("max_results = %d, want %d", got.MaxResults, 10)
	}
	if got.SearchDepth != "basic" {
		t.Errorf("search_depth = %q, want %q", got.SearchDepth, "basic")
	}
	if got.APIKey != "test-key" {
		t.Errorf("api_key = %q, want %q", got.APIKey, "test-key")
	}
}

func TestWebSearch_AuthorizationHeader(t *testing.T) {
	authHeader := ""
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer ts.Close()

	t.Setenv("TAVILY_API_KEY", "test-key-123")
	t.Setenv("AXE_TAVILY_BASE_URL", ts.URL)

	entry := webSearchEntry()
	_ = entry.Execute(context.Background(), provider.ToolCall{ID: "ws-auth", Arguments: map[string]string{"query": "test"}}, ExecContext{})

	if authHeader != "Bearer test-key-123" {
		t.Errorf("Authorization = %q, want %q", authHeader, "Bearer test-key-123")
	}
}

func TestWebSearch_VerboseLog(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer ts.Close()

	t.Setenv("TAVILY_API_KEY", "secret-test-key")
	t.Setenv("AXE_TAVILY_BASE_URL", ts.URL)

	var stderr bytes.Buffer
	entry := webSearchEntry()
	_ = entry.Execute(
		context.Background(),
		provider.ToolCall{ID: "ws-verbose", Arguments: map[string]string{"query": "my verbose test"}},
		ExecContext{Verbose: true, Stderr: &stderr},
	)

	logOutput := stderr.String()
	if !strings.Contains(logOutput, "my verbose test") {
		t.Errorf("verbose log missing query: %q", logOutput)
	}
	if strings.Contains(logOutput, "secret-test-key") {
		t.Errorf("verbose log leaked API key: %q", logOutput)
	}
}

func TestWebSearch_VerboseLogQueryTruncation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer ts.Close()

	t.Setenv("TAVILY_API_KEY", "test-key")
	t.Setenv("AXE_TAVILY_BASE_URL", ts.URL)

	longQuery := strings.Repeat("q", 120)
	var stderr bytes.Buffer
	entry := webSearchEntry()
	_ = entry.Execute(
		context.Background(),
		provider.ToolCall{ID: "ws-verbose-trunc", Arguments: map[string]string{"query": longQuery}},
		ExecContext{Verbose: true, Stderr: &stderr},
	)

	logOutput := stderr.String()
	if strings.Contains(logOutput, longQuery) {
		t.Errorf("expected full query to be truncated in verbose log: %q", logOutput)
	}
	if !strings.Contains(logOutput, strings.Repeat("q", 80)) {
		t.Errorf("expected verbose log to contain 80-char query prefix: %q", logOutput)
	}
}

func TestWebSearch_LargeResponseTruncation(t *testing.T) {
	largeBody := strings.Repeat("A", 102401)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(largeBody))
	}))
	defer ts.Close()

	t.Setenv("TAVILY_API_KEY", "test-key")
	t.Setenv("AXE_TAVILY_BASE_URL", ts.URL)

	entry := webSearchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "ws-large", Arguments: map[string]string{"query": "test"}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	if !strings.Contains(result.Content, "Tavily response too large to process") {
		t.Errorf("Content = %q, want contains %q", result.Content, "Tavily response too large to process")
	}
}

func TestWebSearch_ErrorBodyTruncation(t *testing.T) {
	largeErrBody := strings.Repeat("E", 1000)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(largeErrBody))
	}))
	defer ts.Close()

	t.Setenv("TAVILY_API_KEY", "test-key")
	t.Setenv("AXE_TAVILY_BASE_URL", ts.URL)

	entry := webSearchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "ws-err-trunc", Arguments: map[string]string{"query": "test"}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	prefix := "Tavily API error (HTTP 400): "
	if !strings.HasPrefix(result.Content, prefix) {
		t.Fatalf("Content = %q, want prefix %q", result.Content, prefix)
	}
	if len(result.Content)-len(prefix) > 500 {
		t.Errorf("error body length = %d, want <= 500", len(result.Content)-len(prefix))
	}
}

func TestWebSearch_Definition(t *testing.T) {
	entry := webSearchEntry()
	def := entry.Definition()

	if def.Name != "web_search" {
		t.Errorf("Name = %q, want %q", def.Name, "web_search")
	}
	param, ok := def.Parameters["query"]
	if !ok {
		t.Fatal("missing query parameter")
	}
	if param.Type != "string" {
		t.Errorf("query type = %q, want %q", param.Type, "string")
	}
	if !param.Required {
		t.Error("query required = false, want true")
	}
}

func TestWebSearch_BaseURLDefault(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "test-key")
	t.Setenv("AXE_TAVILY_BASE_URL", "")

	entry := webSearchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "ws-default-url", Arguments: map[string]string{"query": "test"}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true because default URL is unreachable in test")
	}
	if result.Content == "" {
		t.Fatal("expected non-empty error content")
	}
}

func TestWebSearch_Non2xxBodyFormatting(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream error"))
	}))
	defer ts.Close()

	t.Setenv("TAVILY_API_KEY", "test-key")
	t.Setenv("AXE_TAVILY_BASE_URL", ts.URL)

	entry := webSearchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "ws-502", Arguments: map[string]string{"query": "test"}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	want := fmt.Sprintf("Tavily API error (HTTP %d): %s", http.StatusBadGateway, "upstream error")
	if result.Content != want {
		t.Errorf("Content = %q, want %q", result.Content, want)
	}
}
