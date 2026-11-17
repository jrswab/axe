package runner

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jrswab/axe/internal/envinterp"
	"github.com/jrswab/axe/internal/testutil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpServersRecorder struct {
	mu    sync.Mutex
	auth  []string
	count atomic.Int64
}

func newMCPServersTestServer(t *testing.T, toolName string) (*httptest.Server, *mcpServersRecorder) {
	t.Helper()

	recorder := new(mcpServersRecorder)
	s := mcp.NewServer(&mcp.Implementation{Name: "mcp-servers-test", Version: "1.0.0"}, nil)
	s.AddTool(&mcp.Tool{
		Name:        toolName,
		Description: "Returns a greeting",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
		},
	}, func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		recorder.count.Add(1)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "hello"}}}, nil
	})

	h := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server { return s }, nil)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder.mu.Lock()
		recorder.auth = append(recorder.auth, r.Header.Get("Authorization"))
		recorder.mu.Unlock()
		recorder.count.Add(1)
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts, recorder
}

func TestMCPServers_OptionsOnlyToolsInRequest(t *testing.T) {
	server, _ := newMCPServersTestServer(t, "mcp_echo")
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Call MCP tool", []testutil.MockToolCall{
			{ID: "tool_1", Name: "mcp_echo", Input: map[string]string{"name": "axe"}},
		}),
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
		MCPServers: []MCPServer{
			{Name: "code-mcp", URL: server.URL, Transport: "streamable-http"},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if mock.RequestCount() == 0 {
		t.Fatal("Run() sent no LLM request")
	}
	if !strings.Contains(mock.Requests[0].Body, "mcp_echo") {
		t.Error("first request lacks option server tool mcp_echo")
	}
}

func TestMCPServers_MergedDistinctNames(t *testing.T) {
	local, _ := newMCPServersTestServer(t, "mcp_echo")
	code, _ := newMCPServersTestServer(t, "mcp_other")
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Call MCP tool", []testutil.MockToolCall{
			{ID: "tool_1", Name: "mcp_echo", Input: map[string]string{"name": "axe"}},
		}),
		testutil.AnthropicResponse("Done"),
	})
	setupTestEnv(t, "[providers.anthropic]\napi_key = \"fake-key\"\n", "test-agent", fmt.Sprintf(`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = []

[[mcp_servers]]
name = "local-mcp"
url = %q
transport = "streamable-http"
`, local.URL))
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())
	stdout, stderr := newMockStdoutStderr()
	opts := Options{
		AgentName: "test-agent",
		Workdir:   t.TempDir(),
		Stdin:     strings.NewReader(""),
		Stdout:    stdout,
		Stderr:    stderr,
		MCPServers: []MCPServer{
			{Name: "code-mcp", URL: code.URL, Transport: "streamable-http"},
		},
	}
	if _, err := Run(context.Background(), opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if mock.RequestCount() == 0 {
		t.Fatal("Run() sent no LLM request")
	}
	for _, name := range []string{"mcp_echo", "mcp_other"} {
		if !strings.Contains(mock.Requests[0].Body, name) {
			t.Errorf("first request lacks merged server tool %s", name)
		}
	}

	opts.DryRun = true
	result, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("dry Run() error = %v", err)
	}
	if result.DryRunInfo == nil {
		t.Fatal("dry Run() returned no DryRunInfo")
	}
	want := []string{
		fmt.Sprintf("local-mcp: %s (streamable-http)", local.URL),
		fmt.Sprintf("code-mcp: %s (streamable-http)", code.URL),
	}
	if !reflect.DeepEqual(result.DryRunInfo.MCPServers, want) {
		t.Errorf("dry-run MCPServers = %v, want %v", result.DryRunInfo.MCPServers, want)
	}
}

func TestMCPServers_DuplicateNameConfigError(t *testing.T) {
	for _, tt := range []struct {
		name string
		toml bool
	}{
		{name: "toml_and_option", toml: true},
		{name: "option_and_option"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			local, _ := newMCPServersTestServer(t, "mcp_echo")
			code, _ := newMCPServersTestServer(t, "mcp_other")
			mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{testutil.AnthropicResponse("Done")})
			agentTOML := `name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = []
`
			servers := []MCPServer{{Name: "local-mcp", URL: code.URL, Transport: "streamable-http"}}
			if tt.toml {
				agentTOML += fmt.Sprintf(`
[[mcp_servers]]
name = "local-mcp"
url = %q
transport = "streamable-http"
`, local.URL)
			} else {
				servers = append(servers, MCPServer{Name: "local-mcp", URL: local.URL, Transport: "streamable-http"})
			}
			setupTestEnv(t, "[providers.anthropic]\napi_key = \"fake-key\"\n", "test-agent", agentTOML)
			t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())
			stdout, stderr := newMockStdoutStderr()
			_, err := Run(context.Background(), Options{
				AgentName:  "test-agent",
				Workdir:    t.TempDir(),
				Stdin:      strings.NewReader(""),
				Stdout:     stdout,
				Stderr:     stderr,
				MCPServers: servers,
			})
			if !IsConfigError(err) {
				t.Errorf("Run() error = %v, want ConfigError for duplicate MCP server name", err)
			}
		})
	}
}

func TestMCPServers_InvalidServerNoConnect(t *testing.T) {
	for _, tt := range []struct {
		name      string
		server    MCPServer
		useServer bool
	}{
		{name: "transport_ws", server: MCPServer{Name: "code-mcp", Transport: "ws"}, useServer: true},
		{name: "scheme_ftp", server: MCPServer{Name: "code-mcp", URL: "ftp://host/mcp", Transport: "streamable-http"}},
		{name: "empty_url", server: MCPServer{Name: "code-mcp", Transport: "streamable-http"}},
		{name: "empty_name", server: MCPServer{Transport: "streamable-http"}, useServer: true},
		{name: "no_host", server: MCPServer{Name: "code-mcp", URL: "http:///mcp", Transport: "streamable-http"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			local, localRecorder := newMCPServersTestServer(t, "mcp_echo")
			code, codeRecorder := newMCPServersTestServer(t, "mcp_other")
			server := tt.server
			if tt.useServer {
				server.URL = code.URL
			}
			mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{testutil.AnthropicResponse("Done")})
			setupTestEnv(t, "[providers.anthropic]\napi_key = \"fake-key\"\n", "test-agent", fmt.Sprintf(`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = []

[[mcp_servers]]
name = "local-mcp"
url = %q
transport = "streamable-http"
`, local.URL))
			t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())
			for _, mode := range []struct {
				name   string
				dryRun bool
			}{
				{name: "live"},
				{name: "dry", dryRun: true},
			} {
				t.Run(mode.name, func(t *testing.T) {
					stdout, stderr := newMockStdoutStderr()
					_, err := Run(context.Background(), Options{
						AgentName:  "test-agent",
						Workdir:    t.TempDir(),
						Stdin:      strings.NewReader(""),
						Stdout:     stdout,
						Stderr:     stderr,
						DryRun:     mode.dryRun,
						MCPServers: []MCPServer{server},
					})
					if !IsConfigError(err) {
						t.Errorf("Run() error = %v, want ConfigError for invalid option server", err)
					}
					if !mode.dryRun {
						if got := localRecorder.count.Load(); got != 0 {
							t.Errorf("TOML server requests = %d, want 0 before validation", got)
						}
						if got := codeRecorder.count.Load(); got != 0 {
							t.Errorf("option server requests = %d, want 0 before validation", got)
						}
					}
				})
			}
		})
	}
}

func TestMCPServers_UnsetHeaderEnvConfigError(t *testing.T) {
	for _, tt := range []struct {
		name  string
		empty bool
		toml  bool
	}{
		{name: "unset"},
		{name: "empty", empty: true},
		{name: "toml_source", toml: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.empty {
				t.Setenv("TOKEN_UNSET_006", "")
			}
			server, _ := newMCPServersTestServer(t, "mcp_echo")
			mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{testutil.AnthropicResponse("Done")})
			agentTOML := `name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = []
`
			servers := []MCPServer{{
				Name: "code-mcp", URL: server.URL, Transport: "streamable-http",
				Headers: map[string]string{
					"Authorization": "Bearer ${TOKEN_UNSET_006}",
					"X-Secret":      "secret-literal-value",
				},
			}}
			if tt.toml {
				agentTOML += fmt.Sprintf(`
[[mcp_servers]]
name = "local-mcp"
url = %q
transport = "streamable-http"
headers = { Authorization = "Bearer ${TOKEN_UNSET_006}", X-Secret = "secret-literal-value" }
`, server.URL)
				servers = nil
			}
			setupTestEnv(t, "[providers.anthropic]\napi_key = \"fake-key\"\n", "test-agent", agentTOML)
			t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())
			stdout, stderr := newMockStdoutStderr()
			_, err := Run(context.Background(), Options{
				AgentName:  "test-agent",
				Workdir:    t.TempDir(),
				Stdin:      strings.NewReader(""),
				Stdout:     stdout,
				Stderr:     stderr,
				MCPServers: servers,
			})
			if !IsConfigError(err) {
				t.Errorf("Run() error = %v, want ConfigError for unset header variable", err)
			}
			if !errors.Is(err, envinterp.ErrUnsetEnv) {
				t.Errorf("errors.Is(Run() error, ErrUnsetEnv) = false, error = %v", err)
			}
			if err != nil && strings.Contains(err.Error(), "secret-literal-value") {
				t.Error("Run() error contains a literal header secret")
			}
		})
	}
}

func TestMCPServers_HeaderExpandedOnWire(t *testing.T) {
	t.Setenv("MCP_TEST_TOKEN", "tok-123")
	server, recorder := newMCPServersTestServer(t, "mcp_echo")
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Call MCP tool", []testutil.MockToolCall{
			{ID: "tool_1", Name: "mcp_echo", Input: map[string]string{"name": "axe"}},
		}),
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
		MCPServers: []MCPServer{{
			Name: "code-mcp", URL: server.URL, Transport: "streamable-http",
			Headers: map[string]string{"Authorization": "Bearer ${MCP_TEST_TOKEN}"},
		}},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if len(recorder.auth) == 0 {
		t.Error("option MCP server recorded no requests")
	}
	for _, auth := range recorder.auth {
		if auth != "Bearer tok-123" {
			t.Errorf("Authorization = %q, want Bearer tok-123", auth)
		}
	}
	if mock.RequestCount() == 0 {
		t.Fatal("Run() sent no LLM request")
	}
	if !strings.Contains(mock.Requests[0].Body, "mcp_echo") {
		t.Error("first request lacks option server tool mcp_echo")
	}
}

func TestMCPServers_DryRunMergedNoHeaders(t *testing.T) {
	local, localRecorder := newMCPServersTestServer(t, "mcp_echo")
	code, codeRecorder := newMCPServersTestServer(t, "mcp_other")
	setupTestEnv(t, "", "test-agent", fmt.Sprintf(`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = []

[[mcp_servers]]
name = "local-mcp"
url = %q
transport = "streamable-http"
`, local.URL))
	stdout, stderr := newMockStdoutStderr()
	result, err := Run(context.Background(), Options{
		AgentName: "test-agent",
		Workdir:   t.TempDir(),
		Stdin:     strings.NewReader(""),
		Stdout:    stdout,
		Stderr:    stderr,
		DryRun:    true,
		MCPServers: []MCPServer{{
			Name: "code-mcp", URL: code.URL, Transport: "streamable-http",
			Headers: map[string]string{
				"Authorization": "Bearer ${MCP_UNSET_006}",
				"X-Secret":      "secret-literal-value",
			},
		}},
	})
	if err != nil {
		t.Fatalf("dry Run() error = %v", err)
	}
	if result.DryRunInfo == nil {
		t.Fatal("dry Run() returned no DryRunInfo")
	}
	want := []string{
		fmt.Sprintf("local-mcp: %s (streamable-http)", local.URL),
		fmt.Sprintf("code-mcp: %s (streamable-http)", code.URL),
	}
	if !reflect.DeepEqual(result.DryRunInfo.MCPServers, want) {
		t.Errorf("dry-run MCPServers = %v, want %v", result.DryRunInfo.MCPServers, want)
	}
	for _, secret := range []string{"secret-literal-value", "Authorization", "${MCP_UNSET_006}"} {
		if strings.Contains(fmt.Sprintf("%+v", result), secret) {
			t.Errorf("dry-run result contains header text %q", secret)
		}
	}
	if got := localRecorder.count.Load(); got != 0 {
		t.Errorf("dry-run TOML server requests = %d, want 0", got)
	}
	if got := codeRecorder.count.Load(); got != 0 {
		t.Errorf("dry-run option server requests = %d, want 0", got)
	}
}

func TestMCPServers_EmptySliceUnchanged(t *testing.T) {
	for _, tt := range []struct {
		name    string
		servers []MCPServer
	}{
		{name: "nil"},
		{name: "empty", servers: []MCPServer{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			workdir := t.TempDir()
			live := func(servers []MCPServer, set bool) (string, error) {
				server, _ := newMCPServersTestServer(t, "mcp_echo")
				mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
					testutil.AnthropicToolUseResponse("Call MCP tool", []testutil.MockToolCall{
						{ID: "tool_1", Name: "mcp_echo", Input: map[string]string{"name": "axe"}},
					}),
					testutil.AnthropicResponse("Done"),
				})
				setupTestEnv(t, "[providers.anthropic]\napi_key = \"fake-key\"\n", "test-agent", fmt.Sprintf(`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = []

[[mcp_servers]]
name = "local-mcp"
url = %q
transport = "streamable-http"
`, server.URL))
				t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())
				stdout, stderr := newMockStdoutStderr()
				opts := Options{
					AgentName: "test-agent",
					Workdir:   workdir,
					Stdin:     strings.NewReader(""),
					Stdout:    stdout,
					Stderr:    stderr,
				}
				if set {
					opts.MCPServers = servers
				}
				_, err := Run(context.Background(), opts)
				if err != nil {
					t.Fatalf("live Run() error = %v", err)
				}
				if mock.RequestCount() != 2 {
					t.Fatalf("live LLM request count = %d, want 2", mock.RequestCount())
				}
				tr := hookTestToolResult(t, mock)
				if tr.IsError || tr.Content != "hello" {
					t.Fatalf("TOML MCP tool result = %+v, want hello without error", tr)
				}
				return mock.Requests[0].Body, err
			}

			wantBody, wantErr := live(nil, false)
			gotBody, gotErr := live(tt.servers, true)
			if gotBody != wantBody {
				t.Errorf("first request = %q, want unset request %q", gotBody, wantBody)
			}
			if gotErr != wantErr {
				t.Errorf("live Run() error = %v, want unset error %v", gotErr, wantErr)
			}

			// Use the same TOML fixture for exact dry-run URL comparisons.
			stdout, stderr := newMockStdoutStderr()
			opts := Options{
				AgentName: "test-agent",
				Workdir:   workdir,
				Stdin:     strings.NewReader(""),
				Stdout:    stdout,
				Stderr:    stderr,
				DryRun:    true,
			}
			unset, err := Run(context.Background(), opts)
			if err != nil {
				t.Fatalf("unset dry Run() error = %v", err)
			}
			if unset.DryRunInfo == nil {
				t.Fatal("unset dry Run() returned no DryRunInfo")
			}
			for _, servers := range [][]MCPServer{nil, {}} {
				opts.MCPServers = servers
				result, err := Run(context.Background(), opts)
				if err != nil {
					t.Fatalf("dry Run() error = %v", err)
				}
				if result.DryRunInfo == nil {
					t.Fatal("dry Run() returned no DryRunInfo")
				}
				if !reflect.DeepEqual(result.DryRunInfo.MCPServers, unset.DryRunInfo.MCPServers) {
					t.Errorf("dry-run MCPServers = %v, want unset %v", result.DryRunInfo.MCPServers, unset.DryRunInfo.MCPServers)
				}
			}
		})
	}
}

func TestMCPServers_RunDoesNotMutateOptions(t *testing.T) {
	t.Setenv("MCP_TEST_TOKEN", "tok-123")
	server, _ := newMCPServersTestServer(t, "mcp_echo")
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Call MCP tool", []testutil.MockToolCall{
			{ID: "tool_1", Name: "mcp_echo", Input: map[string]string{"name": "axe"}},
		}),
		testutil.AnthropicResponse("Done"),
	})
	setupTestEnv(t, "[providers.anthropic]\napi_key = \"fake-key\"\n", "test-agent", `name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = []
`)
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())
	stdout, stderr := newMockStdoutStderr()
	opts := Options{
		AgentName: "test-agent",
		Workdir:   t.TempDir(),
		Stdin:     strings.NewReader(""),
		Stdout:    stdout,
		Stderr:    stderr,
		MCPServers: []MCPServer{{
			Name: "code-mcp", URL: server.URL, Transport: "streamable-http",
			Headers: map[string]string{"Authorization": "Bearer ${MCP_TEST_TOKEN}"},
		}},
	}
	before := make([]MCPServer, len(opts.MCPServers))
	for i, server := range opts.MCPServers {
		before[i] = server
		before[i].Headers = make(map[string]string, len(server.Headers))
		for key, value := range server.Headers {
			before[i].Headers[key] = value
		}
	}
	if _, err := Run(context.Background(), opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if mock.RequestCount() == 0 {
		t.Fatal("Run() sent no LLM request")
	}
	if !strings.Contains(mock.Requests[0].Body, "mcp_echo") {
		t.Error("first request lacks option server tool mcp_echo")
	}
	if !reflect.DeepEqual(opts.MCPServers, before) {
		t.Error("Run() mutated the caller's MCP server slice or headers")
	}
}

func TestMCPServers_SubAgentNotInherited(t *testing.T) {
	server, _ := newMCPServersTestServer(t, "mcp_parent_only")
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Call child agent", []testutil.MockToolCall{
			{ID: "tool_1", Name: "call_agent", Input: map[string]string{"agent": "child-agent", "task": "say hello"}},
		}),
		testutil.AnthropicResponse("hello from child"),
		testutil.AnthropicResponse("Child said hello from child"),
	})
	tmpDir := setupTestEnv(t, "[providers.anthropic]\napi_key = \"fake-key\"\n", "parent-agent", `name = "parent-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = []
sub_agents = ["child-agent"]
`)
	childTOML := `name = "child-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = []
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
		MCPServers: []MCPServer{
			{Name: "code-mcp", URL: server.URL, Transport: "streamable-http"},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if mock.RequestCount() != 3 {
		t.Fatalf("LLM request count = %d, want parent, child, parent", mock.RequestCount())
	}
	if !strings.Contains(mock.Requests[0].Body, "mcp_parent_only") {
		t.Error("parent request lacks option server tool mcp_parent_only")
	}
	if strings.Contains(mock.Requests[1].Body, "mcp_parent_only") {
		t.Error("child request inherited the parent's option server tool mcp_parent_only")
	}
}

func TestMCPServers_VerboseNoHeaders(t *testing.T) {
	server, _ := newMCPServersTestServer(t, "mcp_echo")
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Call MCP tool", []testutil.MockToolCall{
			{ID: "tool_1", Name: "mcp_echo", Input: map[string]string{"name": "axe"}},
		}),
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
		Verbose:   true,
		MCPServers: []MCPServer{{
			Name: "code-mcp", URL: server.URL, Transport: "streamable-http",
			Headers: map[string]string{"Authorization": "Bearer secret-literal-value"},
		}},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(stderr.String(), server.URL) {
		t.Error("verbose stderr lacks option MCP server URL")
	}
	if strings.Contains(stderr.String(), "secret-literal-value") {
		t.Error("verbose stderr contains a literal header secret")
	}
	if mock.RequestCount() == 0 {
		t.Fatal("Run() sent no LLM request")
	}
	if !strings.Contains(mock.Requests[0].Body, "mcp_echo") {
		t.Error("first request lacks option server tool mcp_echo")
	}
}
