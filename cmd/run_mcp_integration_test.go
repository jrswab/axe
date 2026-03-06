package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jrswab/axe/internal/testutil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newRunTestMCPServer(t *testing.T) *httptest.Server {
	t.Helper()

	s := mcp.NewServer(&mcp.Implementation{Name: "mcp-smoke", Version: "1.0.0"}, nil)
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
	return ts
}

func TestIntegration_MCP_RunVerboseAndJSON(t *testing.T) {
	resetRunCmd(t)

	mcpServer := newRunTestMCPServer(t)
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Running MCP tool.", []testutil.MockToolCall{{
			ID:    "tc_mcp_1",
			Name:  "mcp_echo",
			Input: map[string]string{"name": "axe"},
		}}),
		testutil.AnthropicResponse("MCP call complete."),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "mcp-smoke", fmt.Sprintf(`name = "mcp-smoke"
model = "anthropic/claude-sonnet-4-20250514"

[[mcp_servers]]
name = "local-mcp"
url = %q
transport = "streamable-http"
`, mcpServer.URL))

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "mcp-smoke", "--json", "--verbose"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, `[mcp] Connecting to "local-mcp"`) {
		t.Fatalf("expected connect log, got %q", stderr)
	}
	if !strings.Contains(stderr, `[mcp] "local-mcp" discovered`) {
		t.Fatalf("expected discovery log, got %q", stderr)
	}
	if !strings.Contains(stderr, `[mcp] Routing tool "mcp_echo" to server "local-mcp"`) {
		t.Fatalf("expected routing log, got %q", stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON output, got error: %v\nraw: %q", err, buf.String())
	}

	if toolCalls, ok := payload["tool_calls"].(float64); !ok || toolCalls != 1 {
		t.Fatalf("expected tool_calls=1, got %v", payload["tool_calls"])
	}

	details, ok := payload["tool_call_details"].([]any)
	if !ok || len(details) != 1 {
		t.Fatalf("expected one tool call detail, got %v", payload["tool_call_details"])
	}

	detail, ok := details[0].(map[string]any)
	if !ok {
		t.Fatalf("expected object tool call detail, got %T", details[0])
	}
	if name, _ := detail["name"].(string); name != "mcp_echo" {
		t.Fatalf("expected tool name mcp_echo, got %q", name)
	}
	if output, _ := detail["output"].(string); !strings.Contains(output, "hello axe") {
		t.Fatalf("expected MCP output in tool detail, got %q", output)
	}
	if isError, _ := detail["is_error"].(bool); isError {
		t.Fatalf("expected MCP tool result non-error, got %v", isError)
	}
}
