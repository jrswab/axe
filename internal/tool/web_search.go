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
	"github.com/jrswab/axe/internal/toolname"
)

const (
	tavilyDefaultURL       = "https://api.tavily.com"
	tavilyMaxResults       = 10
	tavilyMaxResponseBytes = 102400
	tavilyErrorBodyMax     = 500
	webSearchQueryLogMax   = 80
)

type tavilySearchRequest struct {
	Query         string `json:"query"`
	MaxResults    int    `json:"max_results"`
	SearchDepth   string `json:"search_depth"`
	APIKey        string `json:"api_key"`
	IncludeImages bool   `json:"include_images"`
	IncludeVideos bool   `json:"include_videos"`
}

type tavilySearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

type tavilySearchResponse struct {
	Results []tavilySearchResult `json:"results"`
}

func webSearchEntry() ToolEntry {
	return ToolEntry{
		Definition: webSearchDefinition,
		Execute:    webSearchExecute,
	}
}

func webSearchDefinition() provider.Tool {
	return provider.Tool{
		Name:        toolname.WebSearch,
		Description: "Search the web using Tavily and return text results including titles, URLs, and snippets.",
		Parameters: map[string]provider.ToolParameter{
			"query": {
				Type:        "string",
				Description: "The search query to perform.",
				Required:    true,
			},
		},
	}
}

func webSearchExecute(ctx context.Context, call provider.ToolCall, ec ExecContext) (result provider.ToolResult) {
	query := call.Arguments["query"]

	defer func() {
		loggedQuery := query
		if len(loggedQuery) > webSearchQueryLogMax {
			loggedQuery = loggedQuery[:webSearchQueryLogMax] + "..."
		}
		summary := fmt.Sprintf("query %q", loggedQuery)
		if result.IsError {
			summary = fmt.Sprintf("%s: %s", summary, truncateResultSummary(result.Content, 120))
		}
		toolVerboseLog(ec, toolname.WebSearch, result, summary)
	}()

	if query == "" {
		return provider.ToolResult{CallID: call.ID, Content: "query is required", IsError: true}
	}

	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		return provider.ToolResult{CallID: call.ID, Content: "TAVILY_API_KEY environment variable is not set", IsError: true}
	}

	baseURL := os.Getenv("AXE_TAVILY_BASE_URL")
	if baseURL == "" {
		baseURL = tavilyDefaultURL
	}

	body, err := json.Marshal(tavilySearchRequest{
		Query:         query,
		MaxResults:    tavilyMaxResults,
		SearchDepth:   "basic",
		APIKey:        apiKey,
		IncludeImages: false,
		IncludeVideos: false,
	})
	if err != nil {
		return provider.ToolResult{CallID: call.ID, Content: err.Error(), IsError: true}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/search", bytes.NewReader(body))
	if err != nil {
		return provider.ToolResult{CallID: call.ID, Content: err.Error(), IsError: true}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

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
		return provider.ToolResult{CallID: call.ID, Content: "Tavily response too large to process", IsError: true}
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		errorBody := string(respBody)
		if len(errorBody) > tavilyErrorBodyMax {
			errorBody = errorBody[:tavilyErrorBodyMax]
		}
		return provider.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Tavily API error (HTTP %d): %s", resp.StatusCode, errorBody),
			IsError: true,
		}
	}

	var parsed tavilySearchResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return provider.ToolResult{CallID: call.ID, Content: fmt.Sprintf("failed to parse Tavily response: %v", err), IsError: true}
	}

	if len(parsed.Results) == 0 {
		return provider.ToolResult{CallID: call.ID, Content: "No results found.", IsError: false}
	}

	var b strings.Builder
	for i, item := range parsed.Results {
		if i > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "Title: %s\nURL: %s\nSnippet: %s", item.Title, item.URL, item.Content)
	}

	return provider.ToolResult{CallID: call.ID, Content: strings.TrimSpace(b.String()), IsError: false}
}

func truncateResultSummary(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
