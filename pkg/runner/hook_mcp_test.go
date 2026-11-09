package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jrswab/axe/internal/testutil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newHookTestMCPServer(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()

	calls := new(atomic.Int64)
	s := mcp.NewServer(&mcp.Implementation{Name: "hook-test", Version: "1.0.0"}, nil)
	s.AddTool(&mcp.Tool{
		Name:        "mcp_echo",
		Description: "Echoes the given name",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "description": "name to echo"},
			},
			"required": []any{"name"},
		},
	}, func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		calls.Add(1)
		var args map[string]any
		name := "unknown"
		if err := json.Unmarshal(req.Params.Arguments, &args); err == nil {
			if n, ok := args["name"].(string); ok && n != "" {
				name = n
			}
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("hello %s", name)}}}, nil
	})

	h := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server { return s }, nil)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts, calls
}

func runHookTestMCP(t *testing.T, decision ToolDecision) (*testutil.MockLLMServer, *atomic.Int64) {
	t.Helper()
	mcpServer, calls := newHookTestMCPServer(t)
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Call MCP tool", []testutil.MockToolCall{
			{ID: "tool_1", Name: "mcp_echo", Input: map[string]string{"name": "axe"}},
		}),
		testutil.AnthropicResponse("Done"),
	})
	setupTestEnv(t,
		"[providers.anthropic]\napi_key = \"fake-key\"\n",
		"test-agent",
		fmt.Sprintf(`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = []

[[mcp_servers]]
name = "local-mcp"
url = %q
transport = "streamable-http"
`, mcpServer.URL),
	)
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())
	stdout, stderr := newMockStdoutStderr()
	_, err := Run(context.Background(), Options{
		AgentName: "test-agent",
		Workdir:   t.TempDir(),
		Stdin:     strings.NewReader(""),
		Stdout:    stdout,
		Stderr:    stderr,
		ToolHook: func(_ context.Context, _ ToolCall) (ToolDecision, error) {
			return decision, nil
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	return mock, calls
}

func TestToolHookMCPDeny(t *testing.T) {
	mock, calls := runHookTestMCP(t, ToolDecision{Action: ToolDeny})
	if got := calls.Load(); got != 0 {
		t.Errorf("denied MCP tool calls = %d, want 0", got)
	}
	tr := hookTestToolResult(t, mock)
	if !tr.IsError {
		t.Error("denied MCP tool result IsError = false, want true")
	}
	if tr.Content != "tool call denied by host policy" {
		t.Errorf("denied MCP tool result Content = %q, want tool call denied by host policy", tr.Content)
	}
}

func TestToolHookMCPStub(t *testing.T) {
	mock, calls := runHookTestMCP(t, ToolDecision{Action: ToolStub, Content: "stub text"})
	if got := calls.Load(); got != 0 {
		t.Errorf("stubbed MCP tool calls = %d, want 0", got)
	}
	tr := hookTestToolResult(t, mock)
	if tr.IsError {
		t.Error("stubbed MCP tool result IsError = true, want false")
	}
	if tr.Content != "stub text" {
		t.Errorf("stubbed MCP tool result Content = %q, want stub text", tr.Content)
	}
}

func TestToolHookMCPAllow(t *testing.T) {
	mock, calls := runHookTestMCP(t, ToolDecision{Action: ToolAllow, Content: "ignored"})
	if got := calls.Load(); got != 1 {
		t.Errorf("allowed MCP tool calls = %d, want 1", got)
	}
	tr := hookTestToolResult(t, mock)
	if tr.IsError {
		t.Error("allowed MCP tool result IsError = true, want false")
	}
	if !strings.Contains(tr.Content, "hello axe") {
		t.Errorf("allowed MCP tool result Content = %q, want hello axe", tr.Content)
	}
	if strings.Contains(mock.Requests[1].Body, "ignored") {
		t.Error("allowed MCP hook content leaked ignored in second request")
	}
}
