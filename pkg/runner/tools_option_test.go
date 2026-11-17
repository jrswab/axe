package runner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jrswab/axe/internal/testutil"
)

func TestTools_OptionReplacesTOMLList(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponse("Done"),
	})
	setupTestEnv(t, "[providers.anthropic]\napi_key = \"fake-key\"\n", "test-agent", `name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["list_directory", "read_file"]
`)
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())
	stdout, stderr := newMockStdoutStderr()
	opts := Options{
		AgentName: "test-agent",
		Workdir:   t.TempDir(),
		Stdin:     strings.NewReader(""),
		Stdout:    stdout,
		Stderr:    stderr,
		Tools:     []string{"write_file"},
	}
	if _, err := Run(context.Background(), opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if mock.RequestCount() != 1 {
		t.Fatalf("LLM request count = %d, want 1", mock.RequestCount())
	}
	body := mock.Requests[0].Body
	if !strings.Contains(body, `"write_file"`) {
		t.Error("first request lacks option tool write_file")
	}
	for _, name := range []string{`"read_file"`, `"list_directory"`} {
		if strings.Contains(body, name) {
			t.Errorf("first request contains replaced TOML tool %s", name)
		}
	}
	if !reflect.DeepEqual(opts.Tools, []string{"write_file"}) {
		t.Errorf("Run() mutated caller's Tools = %v, want [write_file]", opts.Tools)
	}

	opts.DryRun = true
	result, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("dry Run() error = %v", err)
	}
	if result.DryRunInfo == nil {
		t.Fatal("dry Run() returned no DryRunInfo")
	}
	if !reflect.DeepEqual(result.DryRunInfo.Tools, []string{"write_file"}) {
		t.Errorf("dry-run Tools = %v, want [write_file]", result.DryRunInfo.Tools)
	}
}

func TestTools_UnknownNameConfigError(t *testing.T) {
	for _, tt := range []struct {
		name string
		tool string
	}{
		{name: "bogus", tool: "bogus"},
		{name: "empty", tool: ""},
		{name: "call_agent", tool: "call_agent"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, mode := range []struct {
				name   string
				dryRun bool
			}{
				{name: "live"},
				{name: "dry", dryRun: true},
			} {
				t.Run(mode.name, func(t *testing.T) {
					mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
						testutil.AnthropicResponse("Done"),
					})
					setupTestEnv(t, "[providers.anthropic]\napi_key = \"fake-key\"\n", "test-agent", `name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = []
`)
					t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())
					stdout, stderr := newMockStdoutStderr()
					_, err := Run(context.Background(), Options{
						AgentName: "test-agent",
						Workdir:   t.TempDir(),
						Stdin:     strings.NewReader(""),
						Stdout:    stdout,
						Stderr:    stderr,
						DryRun:    mode.dryRun,
						Tools:     []string{tt.tool},
					})
					if !IsConfigError(err) {
						t.Errorf("Run() error = %v, want ConfigError for Options.Tools[0] = %q", err, tt.tool)
					}
					if err != nil && !strings.Contains(err.Error(), "Options.Tools[0]") {
						t.Errorf("Run() error = %v, want Options.Tools[0] in message", err)
					}
					if got := mock.RequestCount(); got != 0 {
						t.Errorf("LLM request count = %d, want 0 before validation", got)
					}
				})
			}
		})
	}
}

func TestTools_EmptyListNoToolsKey(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponse("Done"),
	})
	setupTestEnv(t, "[providers.anthropic]\napi_key = \"fake-key\"\n", "test-agent", `name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["read_file"]
`)
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())
	stdout, stderr := newMockStdoutStderr()
	_, err := Run(context.Background(), Options{
		AgentName: "test-agent",
		Workdir:   t.TempDir(),
		Stdin:     strings.NewReader(""),
		Stdout:    stdout,
		Stderr:    stderr,
		Tools:     []string{},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if mock.RequestCount() != 1 {
		t.Fatalf("LLM request count = %d, want 1", mock.RequestCount())
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal([]byte(mock.Requests[0].Body), &body); err != nil {
		t.Fatalf("decode first request: %v", err)
	}
	if _, ok := body["tools"]; ok {
		t.Error("first request has tools key, want no tools key for empty Options.Tools")
	}
}

// [guard] A nil option preserves the TOML tool list.
func TestTools_NilUsesTOMLList(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponse("Done"),
	})
	setupTestEnv(t, "[providers.anthropic]\napi_key = \"fake-key\"\n", "test-agent", `name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["read_file"]
`)
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())
	stdout, stderr := newMockStdoutStderr()
	_, err := Run(context.Background(), Options{
		AgentName: "test-agent",
		Workdir:   t.TempDir(),
		Stdin:     strings.NewReader(""),
		Stdout:    stdout,
		Stderr:    stderr,
		Tools:     nil,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if mock.RequestCount() != 1 {
		t.Fatalf("LLM request count = %d, want 1", mock.RequestCount())
	}
	if !strings.Contains(mock.Requests[0].Body, `"read_file"`) {
		t.Error("first request lacks TOML tool read_file with nil Options.Tools")
	}
}

func TestTools_MCPShadowingFollowsEffectiveList(t *testing.T) {
	for _, tt := range []struct {
		name    string
		tools   []string
		wantMCP bool
	}{
		// [guard] Keeping the built-in continues to shadow the MCP tool.
		{name: "a_builtin_kept", tools: []string{"read_file"}},
		{name: "b_builtin_removed", tools: []string{"list_directory"}, wantMCP: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server, _ := newMCPServersTestServer(t, "read_file")
			responses := []testutil.MockLLMResponse{testutil.AnthropicResponse("Done")}
			if tt.wantMCP {
				responses = []testutil.MockLLMResponse{
					testutil.AnthropicToolUseResponse("Call MCP tool", []testutil.MockToolCall{
						{ID: "tool_1", Name: "read_file", Input: map[string]string{"name": "axe"}},
					}),
					testutil.AnthropicResponse("Done"),
				}
			}
			mock := testutil.NewMockLLMServer(t, responses)
			setupTestEnv(t, "[providers.anthropic]\napi_key = \"fake-key\"\n", "test-agent", `name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["read_file"]
`)
			t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())
			stdout, stderr := newMockStdoutStderr()
			_, err := Run(context.Background(), Options{
				AgentName: "test-agent",
				Workdir:   t.TempDir(),
				Stdin:     strings.NewReader(""),
				Stdout:    stdout,
				Stderr:    stderr,
				Tools:     tt.tools,
				MCPServers: []MCPServer{
					{Name: "code-mcp", URL: server.URL, Transport: "streamable-http"},
				},
			})
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if mock.RequestCount() != len(responses) {
				t.Fatalf("LLM request count = %d, want %d", mock.RequestCount(), len(responses))
			}
			body := mock.Requests[0].Body
			if got := strings.Count(body, `"name":"read_file"`); got != 1 {
				t.Errorf("first request read_file definition count = %d, want 1", got)
			}
			if tt.wantMCP {
				if !strings.Contains(body, "Returns a greeting") {
					t.Error("first request lacks MCP read_file description Returns a greeting")
				}
				if !strings.Contains(mock.Requests[1].Body, "hello") {
					t.Error("second request lacks MCP read_file result hello")
				}
			} else if strings.Contains(body, "Returns a greeting") {
				t.Error("first request contains shadowed MCP read_file description Returns a greeting")
			}
		})
	}
}

func TestTools_SubAgentNotInherited(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Delegating", []testutil.MockToolCall{
			{ID: "tool_1", Name: "call_agent", Input: map[string]string{"agent": "child-agent", "task": "say hello"}},
		}),
		testutil.AnthropicResponse("hello from child"),
		testutil.AnthropicResponse("done"),
	})
	tmpDir := setupTestEnv(t, "[providers.anthropic]\napi_key = \"fake-key\"\n", "parent-agent", `name = "parent-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = []
sub_agents = ["child-agent"]
`)
	childTOML := `name = "child-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["read_file"]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "axe", "agents", "child-agent.toml"), []byte(childTOML), 0644); err != nil {
		t.Fatalf("failed to write child agent: %v", err)
	}
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())
	stdout, stderr := newMockStdoutStderr()
	_, err := Run(context.Background(), Options{
		AgentName: "parent-agent",
		Workdir:   t.TempDir(),
		Stdin:     strings.NewReader(""),
		Stdout:    stdout,
		Stderr:    stderr,
		Tools:     []string{"write_file"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if mock.RequestCount() != 3 {
		t.Fatalf("LLM request count = %d, want parent, child, parent", mock.RequestCount())
	}
	if !strings.Contains(mock.Requests[0].Body, `"write_file"`) {
		t.Error("parent request lacks option tool write_file")
	}
	if !strings.Contains(mock.Requests[1].Body, `"read_file"`) {
		t.Error("child request lacks its TOML tool read_file")
	}
	if strings.Contains(mock.Requests[1].Body, `"write_file"`) {
		t.Error("child request inherited the parent's option tool write_file")
	}
}
