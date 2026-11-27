package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/jrswab/axe/internal/provider"
)

// Keenable is an opt-in alternative backend for web_search, selected with
// AXE_WEB_SEARCH_PROVIDER=keenable. Tavily stays the default.
const (
	webSearchProviderEnv    = "AXE_WEB_SEARCH_PROVIDER"
	webSearchProviderTavily = "tavily"
	webSearchProviderKeen   = "keenable"

	keenableDefaultURL = "https://api.keenable.ai"
	// keenableAppTitle names the calling app in X-Keenable-Title, which the
	// keyless endpoint requires.
	keenableAppTitle = "axe"
)

type keenableSearchRequest struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
}

type keenableSearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

type keenableSearchResponse struct {
	Results []keenableSearchResult `json:"results"`
}

// webSearchProvider returns the configured web_search backend. An unset or
// empty AXE_WEB_SEARCH_PROVIDER means Tavily.
func webSearchProvider() (string, error) {
	name := strings.ToLower(strings.TrimSpace(os.Getenv(webSearchProviderEnv)))
	switch name {
	case "", webSearchProviderTavily:
		return webSearchProviderTavily, nil
	case webSearchProviderKeen:
		return webSearchProviderKeen, nil
	default:
		return "", fmt.Errorf("unknown %s %q; valid values: %s, %s", webSearchProviderEnv, name, webSearchProviderTavily, webSearchProviderKeen)
	}
}

// keenableSearch runs the query against Keenable. With KEENABLE_API_KEY set it
// calls POST /v1/search with the X-API-Key header; without it, the keyless
// POST /v1/search/public endpoint, which is rate limited per IP address.
func keenableSearch(ctx context.Context, call provider.ToolCall, query string) provider.ToolResult {
	apiKey := strings.TrimSpace(os.Getenv("KEENABLE_API_KEY"))

	baseURL := os.Getenv("AXE_KEENABLE_BASE_URL")
	if baseURL == "" {
		baseURL = keenableDefaultURL
	}
	path := "/v1/search/public"
	if apiKey != "" {
		path = "/v1/search"
	}

	body, err := json.Marshal(keenableSearchRequest{Query: query, MaxResults: tavilyMaxResults})
	if err != nil {
		return provider.ToolResult{CallID: call.ID, Content: err.Error(), IsError: true}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return provider.ToolResult{CallID: call.ID, Content: err.Error(), IsError: true}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Keenable-Title", keenableAppTitle)
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return provider.ToolResult{CallID: call.ID, Content: err.Error(), IsError: true}
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, tavilyMaxResponseBytes+1))
	if err != nil {
		return provider.ToolResult{CallID: call.ID, Content: err.Error(), IsError: true}
	}
	if len(respBody) > tavilyMaxResponseBytes {
		return provider.ToolResult{CallID: call.ID, Content: "Keenable response too large to process", IsError: true}
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		errorBody := string(respBody)
		if len(errorBody) > tavilyErrorBodyMax {
			errorBody = errorBody[:tavilyErrorBodyMax]
		}
		return provider.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Keenable API error (HTTP %d): %s", resp.StatusCode, errorBody),
			IsError: true,
		}
	}

	var parsed keenableSearchResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return provider.ToolResult{CallID: call.ID, Content: fmt.Sprintf("failed to parse Keenable response: %v", err), IsError: true}
	}

	if len(parsed.Results) == 0 {
		return provider.ToolResult{CallID: call.ID, Content: "No results found.", IsError: false}
	}

	var b strings.Builder
	for i, item := range parsed.Results {
		if i > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "Title: %s\nURL: %s\nSnippet: %s", item.Title, item.URL, item.Snippet)
	}

	return provider.ToolResult{CallID: call.ID, Content: strings.TrimSpace(b.String()), IsError: false}
}
