package tool

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/internal/toolname"
)

const maxReadBytes = 10000

func truncateURL(urlStr string, maxLen int) string {
	if len(urlStr) <= maxLen {
		return urlStr
	}
	return urlStr[:maxLen] + "..."
}

func sanitizeURL(urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "invalid-url"
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "invalid-url"
	}

	path := parsedURL.EscapedPath()
	if path == "" {
		return parsedURL.Scheme + "://" + parsedURL.Host
	}

	return parsedURL.Scheme + "://" + parsedURL.Host + path
}

func urlFetchEntry() ToolEntry {
	return ToolEntry{
		Definition: urlFetchDefinition,
		Execute:    urlFetchExecute,
	}
}

func urlFetchDefinition() provider.Tool {
	return provider.Tool{
		Name:        toolname.URLFetch,
		Description: "Fetch content from a URL using HTTP GET and return the response body as text.",
		Parameters: map[string]provider.ToolParameter{
			"url": {
				Type:        "string",
				Description: "The URL to fetch.",
				Required:    true,
			},
		},
	}
}

func urlFetchExecute(ctx context.Context, call provider.ToolCall, ec ExecContext) (result provider.ToolResult) {
	urlStr := call.Arguments["url"]
	statusCode := 0

	defer func() {
		safeURL := truncateURL(sanitizeURL(urlStr), 120)
		summary := fmt.Sprintf("url %q", safeURL)
		if statusCode != 0 {
			summary = fmt.Sprintf("url %q (HTTP %d)", safeURL, statusCode)
		} else if result.IsError {
			summary = fmt.Sprintf("%s: %s", summary, truncateURL(result.Content, 120))
		}
		toolVerboseLog(ec, toolname.URLFetch, result, summary)
	}()

	if urlStr == "" {
		return provider.ToolResult{CallID: call.ID, Content: "url is required", IsError: true}
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return provider.ToolResult{CallID: call.ID, Content: err.Error(), IsError: true}
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("unsupported scheme %q: only http and https are allowed", parsedURL.Scheme),
			IsError: true,
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return provider.ToolResult{CallID: call.ID, Content: err.Error(), IsError: true}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return provider.ToolResult{CallID: call.ID, Content: err.Error(), IsError: true}
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	statusCode = resp.StatusCode

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReadBytes+1))
	if err != nil {
		return provider.ToolResult{CallID: call.ID, Content: err.Error(), IsError: true}
	}

	bodyStr := string(body)
	if len(body) > maxReadBytes {
		bodyStr = string(body[:maxReadBytes]) + "\n... [response truncated, exceeded 10000 characters]"
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, bodyStr),
			IsError: true,
		}
	}

	return provider.ToolResult{CallID: call.ID, Content: bodyStr, IsError: false}
}
