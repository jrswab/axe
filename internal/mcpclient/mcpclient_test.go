package mcpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jrswab/axe/internal/agent"
	"github.com/jrswab/axe/internal/provider"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestConnect_StreamableHTTP_Success(t *testing.T) {
	ts := newMCPStreamableServer(t)

	client, err := Connect(context.Background(), agent.MCPServerConfig{
		Name:      "test",
		URL:       ts.URL,
		Transport: "streamable-http",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
}

func TestConnect_InvalidTransport(t *testing.T) {
	_, err := Connect(context.Background(), agent.MCPServerConfig{
		Name:      "test",
		URL:       "http://localhost:8080/mcp",
		Transport: "stdio",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `unsupported MCP transport`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConnect_UnreachableServer(t *testing.T) {
	_, err := Connect(context.Background(), agent.MCPServerConfig{
		Name:      "test",
		URL:       "http://127.0.0.1:1/mcp",
		Transport: "streamable-http",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestConnect_HeaderInjection(t *testing.T) {
	var sawAuth atomic.Bool
	t.Setenv("TOKEN", "secret")
	ts := newMCPStreamableServer(t, func(r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer secret" {
			sawAuth.Store(true)
		}
	})

	client, err := Connect(context.Background(), agent.MCPServerConfig{
		Name:      "test",
		URL:       ts.URL,
		Transport: "streamable-http",
		Headers: map[string]string{
			"Authorization": "Bearer ${TOKEN}",
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if _, err := client.ListTools(context.Background()); err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if !sawAuth.Load() {
		t.Fatal("expected Authorization header to be sent")
	}
}

func TestConnect_HeaderEnvVarMissing(t *testing.T) {
	_, err := Connect(context.Background(), agent.MCPServerConfig{
		Name:      "test",
		URL:       "http://localhost:8080/mcp",
		Transport: "streamable-http",
		Headers: map[string]string{
			"Authorization": "Bearer ${TOKEN}",
		},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `environment variable "TOKEN" is not set or empty`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListTools_ReturnsTools(t *testing.T) {
	ts := newMCPStreamableServer(t)
	client := mustConnectTestClient(t, ts.URL)

	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(tools) != 4 {
		t.Fatalf("got %d tools, want 4", len(tools))
	}
}

func TestListTools_EmptyTools(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	h := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server { return s }, nil)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)

	client := mustConnectTestClient(t, ts.URL)
	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(tools) != 0 {
		t.Fatalf("got %d tools, want 0", len(tools))
	}
}

func TestListTools_ComplexSchema(t *testing.T) {
	ts := newMCPComplexSchemaServer(t)
	client := mustConnectTestClient(t, ts.URL)

	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("got %d tools, want 1", len(tools))
	}

	tool := tools[0]
	if tool.Parameters["query"].Type != "string" {
		t.Fatalf("query type = %q, want string", tool.Parameters["query"].Type)
	}
	if tool.Parameters["filters"].Type != "string" {
		t.Fatalf("filters type = %q, want string", tool.Parameters["filters"].Type)
	}
	if !strings.HasPrefix(tool.Parameters["filters"].Description, "Type array:") {
		t.Fatalf("filters description = %q, want Type array prefix", tool.Parameters["filters"].Description)
	}
	if tool.Parameters["query"].Required != true {
		t.Fatal("query should be required")
	}
}

func TestCallTool_Success(t *testing.T) {
	ts := newMCPStreamableServer(t)
	client := mustConnectTestClient(t, ts.URL)

	result, err := client.CallTool(context.Background(), provider.ToolCall{
		ID:   "call-1",
		Name: "success",
		Arguments: map[string]string{
			"name": "axe",
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.IsError {
		t.Fatal("expected non-error result")
	}
	if result.Content != "ok" {
		t.Fatalf("content = %q, want %q", result.Content, "ok")
	}
}

func TestCallTool_ToolError(t *testing.T) {
	ts := newMCPStreamableServer(t)
	client := mustConnectTestClient(t, ts.URL)

	result, err := client.CallTool(context.Background(), provider.ToolCall{
		ID:   "call-1",
		Name: "tool_error",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result")
	}
	if result.Content != "tool failed" {
		t.Fatalf("content = %q, want %q", result.Content, "tool failed")
	}
}

func TestCallTool_NoTextContent(t *testing.T) {
	ts := newMCPStreamableServer(t)
	client := mustConnectTestClient(t, ts.URL)

	result, err := client.CallTool(context.Background(), provider.ToolCall{
		ID:   "call-1",
		Name: "no_text",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result")
	}
	if result.Content != "MCP tool returned no text content" {
		t.Fatalf("content = %q, want %q", result.Content, "MCP tool returned no text content")
	}
}

func TestCallTool_MultipleTextContent(t *testing.T) {
	ts := newMCPStreamableServer(t)
	client := mustConnectTestClient(t, ts.URL)

	result, err := client.CallTool(context.Background(), provider.ToolCall{
		ID:   "call-1",
		Name: "multi_text",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Content != "first\nsecond" {
		t.Fatalf("content = %q, want %q", result.Content, "first\nsecond")
	}
}

func TestClientName(t *testing.T) {
	ts := newMCPStreamableServer(t)
	client, err := Connect(context.Background(), agent.MCPServerConfig{
		Name:      "my-server",
		URL:       ts.URL,
		Transport: "streamable-http",
	})
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if got := client.Name(); got != "my-server" {
		t.Fatalf("Name() = %q, want %q", got, "my-server")
	}
}

func mustConnectTestClient(t *testing.T, endpoint string) *Client {
	t.Helper()
	client, err := Connect(context.Background(), agentTestConfig("test", endpoint))
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func agentTestConfig(name, endpoint string) agent.MCPServerConfig {
	return agent.MCPServerConfig{Name: name, URL: endpoint, Transport: "streamable-http"}
}

func newMCPStreamableServer(t *testing.T, onRequest ...func(*http.Request)) *httptest.Server {
	t.Helper()

	s := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)

	s.AddTool(&mcp.Tool{Name: "success", InputSchema: map[string]any{"type": "object"}}, func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
	})
	s.AddTool(&mcp.Tool{Name: "tool_error", InputSchema: map[string]any{"type": "object"}}, func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "tool failed"}}}, nil
	})
	s.AddTool(&mcp.Tool{Name: "no_text", InputSchema: map[string]any{"type": "object"}}, func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.ImageContent{Data: []byte("abc"), MIMEType: "image/png"}}}, nil
	})
	s.AddTool(&mcp.Tool{Name: "multi_text", InputSchema: map[string]any{"type": "object"}}, func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "first"}, &mcp.TextContent{Text: "second"}}}, nil
	})

	h := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server { return s }, nil)
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, fn := range onRequest {
			fn(r)
		}
		h.ServeHTTP(w, r)
	})

	ts := httptest.NewServer(wrapped)
	t.Cleanup(ts.Close)
	return ts
}

func newMCPComplexSchemaServer(t *testing.T) *httptest.Server {
	t.Helper()

	s := mcp.NewServer(&mcp.Implementation{Name: "complex-server", Version: "1.0.0"}, nil)
	s.AddTool(&mcp.Tool{
		Name: "complex",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":   map[string]any{"type": "string", "description": "search text"},
				"filters": map[string]any{"type": "array", "description": "filter list"},
			},
			"required": []any{"query"},
		},
	}, func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
	})

	h := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server { return s }, nil)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

func TestCoerceArg(t *testing.T) {
	tests := []struct {
		value    string
		typeName string
		want     any
	}{
		{"3", "integer", int64(3)},
		{"-7", "integer", int64(-7)},
		{"1.5", "number", 1.5},
		{"-0.5", "number", -0.5},
		{"true", "boolean", true},
		{"false", "boolean", false},
		{"hello", "string", "hello"},
		{"hello", "", "hello"},
		{"notanint", "integer", "notanint"},
		{"notabool", "boolean", "notabool"},
		{"notanum", "number", "notanum"},
	}
	for _, tc := range tests {
		t.Run(tc.value+"_"+tc.typeName, func(t *testing.T) {
			got := coerceArg(tc.value, tc.typeName)
			if got != tc.want {
				t.Errorf("coerceArg(%q, %q) = %v (%T), want %v (%T)",
					tc.value, tc.typeName, got, got, tc.want, tc.want)
			}
		})
	}
}

func newMCPTypedArgsServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "typed-server", Version: "1.0.0"}, nil)
	s.AddTool(&mcp.Tool{
		Name: "typed",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"count":   map[string]any{"type": "integer", "description": "a count"},
				"enabled": map[string]any{"type": "boolean", "description": "flag"},
				"ratio":   map[string]any{"type": "number", "description": "a ratio"},
				"label":   map[string]any{"type": "string", "description": "a label"},
			},
		},
	}, func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("unmarshal error: %v", err)},
			}}, nil
		}
		var parts []string
		for k, v := range args {
			parts = append(parts, fmt.Sprintf("%s:%T", k, v))
		}
		sort.Strings(parts)
		return &mcp.CallToolResult{Content: []mcp.Content{
			&mcp.TextContent{Text: strings.Join(parts, ",")},
		}}, nil
	})
	h := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server { return s }, nil)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

func TestCallTool_CoercesArgTypes(t *testing.T) {
	ts := newMCPTypedArgsServer(t)
	client := mustConnectTestClient(t, ts.URL)

	// ListTools must be called first to populate schemas.
	if _, err := client.ListTools(context.Background()); err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	result, err := client.CallTool(context.Background(), provider.ToolCall{
		ID:   "call-typed",
		Name: "typed",
		Arguments: map[string]string{
			"count":   "3",
			"enabled": "true",
			"ratio":   "1.5",
			"label":   "hello",
		},
	})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected non-error, got: %s", result.Content)
	}
	// After coercion, count should arrive as a numeric type, not string.
	if strings.Contains(result.Content, "count:string") {
		t.Fatalf("count should be coerced from string, got: %s", result.Content)
	}
}
