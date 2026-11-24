package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jrswab/axe/internal/provider"
)

func TestWebSearchKeenable_KeylessRequest(t *testing.T) {
	var gotPath, gotTitle, gotAPIKey string
	var gotBody keenableSearchRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotTitle = r.Header.Get("X-Keenable-Title")
		gotAPIKey = r.Header.Get("X-API-Key")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{"results":[{"title":"Result 1","url":"https://example.com/1","snippet":"Snippet 1"},{"title":"Result 2","url":"https://example.com/2","snippet":"Snippet 2"}]}`))
	}))
	defer ts.Close()

	t.Setenv("AXE_WEB_SEARCH_PROVIDER", "keenable")
	t.Setenv("KEENABLE_API_KEY", "")
	t.Setenv("TAVILY_API_KEY", "")
	t.Setenv("AXE_KEENABLE_BASE_URL", ts.URL)

	call := provider.ToolCall{ID: "ws-keenable", Arguments: map[string]string{"query": "test query"}}
	result := webSearchEntry().Execute(context.Background(), call, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	if gotPath != "/v1/search/public" {
		t.Errorf("path = %q, want %q", gotPath, "/v1/search/public")
	}
	if gotTitle != "axe" {
		t.Errorf("X-Keenable-Title = %q, want %q", gotTitle, "axe")
	}
	if gotAPIKey != "" {
		t.Errorf("X-API-Key = %q, want empty", gotAPIKey)
	}
	if gotBody.Query != "test query" || gotBody.MaxResults != tavilyMaxResults {
		t.Errorf("body = %+v, want query %q and max_results %d", gotBody, "test query", tavilyMaxResults)
	}
	want := "Title: Result 1\nURL: https://example.com/1\nSnippet: Snippet 1\n\nTitle: Result 2\nURL: https://example.com/2\nSnippet: Snippet 2"
	if result.Content != want {
		t.Errorf("Content = %q, want %q", result.Content, want)
	}
	if result.CallID != call.ID {
		t.Errorf("CallID = %q, want %q", result.CallID, call.ID)
	}
}

func TestWebSearchKeenable_KeyedRequest(t *testing.T) {
	var gotPath, gotAPIKey, gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("X-API-Key")
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"results":[{"title":"Result 1","url":"https://example.com/1","snippet":"Snippet 1"}]}`))
	}))
	defer ts.Close()

	t.Setenv("AXE_WEB_SEARCH_PROVIDER", "Keenable")
	t.Setenv("KEENABLE_API_KEY", "keen-test")
	t.Setenv("TAVILY_API_KEY", "tavily-test")
	t.Setenv("AXE_KEENABLE_BASE_URL", ts.URL+"/")

	result := webSearchEntry().Execute(context.Background(), provider.ToolCall{ID: "ws-keyed", Arguments: map[string]string{"query": "q"}}, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	if gotPath != "/v1/search" {
		t.Errorf("path = %q, want %q", gotPath, "/v1/search")
	}
	if gotAPIKey != "keen-test" {
		t.Errorf("X-API-Key = %q, want %q", gotAPIKey, "keen-test")
	}
	if gotAuth != "" {
		t.Errorf("Authorization = %q, want empty (the Tavily key must not be sent)", gotAuth)
	}
}

func TestWebSearchKeenable_EmptyResults(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer ts.Close()

	t.Setenv("AXE_WEB_SEARCH_PROVIDER", "keenable")
	t.Setenv("AXE_KEENABLE_BASE_URL", ts.URL)

	result := webSearchEntry().Execute(context.Background(), provider.ToolCall{ID: "ws-empty", Arguments: map[string]string{"query": "q"}}, ExecContext{})

	if result.IsError || result.Content != "No results found." {
		t.Errorf("result = %+v, want non-error %q", result, "No results found.")
	}
}

func TestWebSearchKeenable_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"code":"public_rate_limit_hourly"}`))
	}))
	defer ts.Close()

	t.Setenv("AXE_WEB_SEARCH_PROVIDER", "keenable")
	t.Setenv("AXE_KEENABLE_BASE_URL", ts.URL)

	result := webSearchEntry().Execute(context.Background(), provider.ToolCall{ID: "ws-429", Arguments: map[string]string{"query": "q"}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	want := fmt.Sprintf("Keenable API error (HTTP %d): %s", http.StatusTooManyRequests, `{"code":"public_rate_limit_hourly"}`)
	if result.Content != want {
		t.Errorf("Content = %q, want %q", result.Content, want)
	}
}

func TestWebSearchKeenable_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer ts.Close()

	t.Setenv("AXE_WEB_SEARCH_PROVIDER", "keenable")
	t.Setenv("AXE_KEENABLE_BASE_URL", ts.URL)

	result := webSearchEntry().Execute(context.Background(), provider.ToolCall{ID: "ws-bad", Arguments: map[string]string{"query": "q"}}, ExecContext{})

	if !result.IsError || !strings.Contains(result.Content, "failed to parse Keenable response") {
		t.Errorf("result = %+v, want parse error", result)
	}
}

func TestWebSearchProvider_Unknown(t *testing.T) {
	t.Setenv("AXE_WEB_SEARCH_PROVIDER", "bing")
	t.Setenv("TAVILY_API_KEY", "test-key")

	result := webSearchEntry().Execute(context.Background(), provider.ToolCall{ID: "ws-unknown", Arguments: map[string]string{"query": "q"}}, ExecContext{})

	if !result.IsError || !strings.Contains(result.Content, `unknown AXE_WEB_SEARCH_PROVIDER "bing"`) {
		t.Errorf("result = %+v, want unknown provider error", result)
	}
}

func TestWebSearchProvider_DefaultIsTavily(t *testing.T) {
	t.Setenv("AXE_WEB_SEARCH_PROVIDER", "")
	t.Setenv("TAVILY_API_KEY", "")

	result := webSearchEntry().Execute(context.Background(), provider.ToolCall{ID: "ws-default", Arguments: map[string]string{"query": "q"}}, ExecContext{})

	if !result.IsError || !strings.Contains(result.Content, "TAVILY_API_KEY environment variable is not set") {
		t.Errorf("result = %+v, want the Tavily missing-key error", result)
	}
	if def := webSearchDefinition(); !strings.Contains(def.Description, "Tavily") {
		t.Errorf("Description = %q, want Tavily", def.Description)
	}
}

func TestWebSearchKeenable_Definition(t *testing.T) {
	t.Setenv("AXE_WEB_SEARCH_PROVIDER", "keenable")

	def := webSearchDefinition()
	if def.Name != "web_search" {
		t.Errorf("Name = %q, want %q", def.Name, "web_search")
	}
	if !strings.Contains(def.Description, "Keenable") {
		t.Errorf("Description = %q, want Keenable", def.Description)
	}
}
