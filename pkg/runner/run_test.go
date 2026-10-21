package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/internal/testutil"
)

// setupTestEnv creates a temporary XDG config directory with the given
// config.toml content and agent TOML content. It returns the temp directory
// and sets XDG_CONFIG_HOME appropriately.
func setupTestEnv(t *testing.T, configTOML, agentName, agentTOML string) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	axeDir := filepath.Join(tmpDir, "axe")
	if err := os.MkdirAll(filepath.Join(axeDir, "agents"), 0755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}
	if configTOML != "" {
		if err := os.WriteFile(filepath.Join(axeDir, "config.toml"), []byte(configTOML), 0644); err != nil {
			t.Fatalf("failed to write config.toml: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(axeDir, "agents", agentName+".toml"), []byte(agentTOML), 0644); err != nil {
		t.Fatalf("failed to write agent toml: %v", err)
	}
	return tmpDir
}

func newMockStdoutStderr() (*bytes.Buffer, *bytes.Buffer) {
	return new(bytes.Buffer), new(bytes.Buffer)
}

type capturingProvider struct {
	requests []provider.Request
}

func (p *capturingProvider) Send(_ context.Context, req *provider.Request) (*provider.Response, error) {
	p.requests = append(p.requests, *req)
	return &provider.Response{Content: "Done", Model: req.Model, StopReason: "stop"}, nil
}

func (p *capturingProvider) SendStream(_ context.Context, req *provider.Request) (*provider.EventStream, error) {
	p.requests = append(p.requests, *req)
	return makeStream([]provider.StreamEvent{
		{Type: provider.StreamEventText, Text: "Done"},
		{Type: provider.StreamEventDone, StopReason: "stop"},
	}), nil
}

func TestRun_ReasoningEffort(t *testing.T) {
	tests := []struct {
		name   string
		params string
		want   string
	}{
		{name: "omitted"},
		{name: "empty", params: "[params]\nreasoning_effort = \"\""},
		{name: "low", params: "[params]\nreasoning_effort = \"low\"", want: "low"},
		{name: "none", params: "[params]\nreasoning_effort = \"none\"", want: "none"},
		{name: "high", params: "[params]\nreasoning_effort = \"high\"", want: "high"},
		{name: "max", params: "[params]\nreasoning_effort = \"max\"", want: "max"},
		{name: "custom", params: "[params]\nreasoning_effort = \"provider-specific\"", want: "provider-specific"},
		{name: "whitespace and case", params: "[params]\nreasoning_effort = \" LOW \"", want: " LOW "},
	}
	for _, mode := range []struct {
		name   string
		stream bool
	}{
		{name: "send"},
		{name: "stream", stream: true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					setupTestEnv(t,
						"[providers.openai]\napi_key = \"fake-key\"\n",
						"test-agent",
						"name = \"test-agent\"\nmodel = \"openai/gpt-4o\"\n"+tt.params,
					)
					prov := &capturingProvider{}
					newProvider := func(name, _, _ string) (provider.Provider, error) {
						if name != "openai" {
							t.Fatalf("provider = %q, want openai", name)
						}
						return prov, nil
					}
					stdout, stderr := newMockStdoutStderr()
					_, err := run(context.Background(), Options{
						AgentName: "test-agent",
						Prompt:    "Test reasoning effort",
						Stdin:     strings.NewReader(""),
						Stdout:    stdout,
						Stderr:    stderr,
						Stream:    mode.stream,
					}, newProvider)
					if err != nil {
						t.Fatalf("run() error = %v", err)
					}
					if len(prov.requests) != 1 {
						t.Fatalf("got %d requests, want 1", len(prov.requests))
					}
					if got := prov.requests[0].ReasoningEffort; got != tt.want {
						t.Errorf("Request.ReasoningEffort = %q, want %q", got, tt.want)
					}
				})
			}
		})
	}
}

func TestRun_ParamsOutput(t *testing.T) {
	tests := []struct {
		name   string
		params string
		want   string
	}{
		{name: "omitted", want: "temperature=0, max_tokens=0"},
		{name: "empty", params: "[params]\nreasoning_effort = \"\"", want: "temperature=0, max_tokens=0"},
		{name: "low", params: "[params]\nreasoning_effort = \"low\"", want: "temperature=0, max_tokens=0, reasoning_effort=low"},
		{
			name:   "all params",
			params: "[params]\ntemperature = 0.7\nmax_tokens = 4096\nresponse_format = \"json_object\"\nreasoning_effort = \"high\"",
			want:   "temperature=0.7, max_tokens=4096, response_format=json_object, reasoning_effort=high",
		},
	}
	for _, mode := range []struct {
		name   string
		dryRun bool
	}{
		{name: "dry-run", dryRun: true},
		{name: "verbose"},
	} {
		t.Run(mode.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					setupTestEnv(t,
						"[providers.openai]\napi_key = \"fake-key\"\n",
						"test-agent",
						"name = \"test-agent\"\nmodel = \"openai/gpt-4o\"\n"+tt.params,
					)
					prov := &capturingProvider{}
					newProvider := func(_, _, _ string) (provider.Provider, error) {
						if mode.dryRun {
							t.Fatal("dry-run should not create a provider")
						}
						return prov, nil
					}
					stdout, stderr := newMockStdoutStderr()
					result, err := run(context.Background(), Options{
						AgentName: "test-agent",
						Stdin:     strings.NewReader(""),
						Stdout:    stdout,
						Stderr:    stderr,
						DryRun:    mode.dryRun,
						Verbose:   !mode.dryRun,
					}, newProvider)
					if err != nil {
						t.Fatalf("run() error = %v", err)
					}
					if mode.dryRun {
						if !result.DryRun || result.DryRunInfo == nil {
							t.Fatal("expected dry-run information")
						}
						if got := result.DryRunInfo.Params; got != tt.want {
							t.Errorf("Params = %q, want %q", got, tt.want)
						}
						return
					}
					if !strings.Contains(stderr.String(), "Params:   "+tt.want+"\n") {
						t.Errorf("verbose output missing Params line %q:\n%s", tt.want, stderr.String())
					}
				})
			}
		})
	}
}

func TestRun_TemperaturePresence(t *testing.T) {
	tests := []struct {
		name    string
		params  string
		want    float64
		wantSet bool
	}{
		{name: "explicit zero", params: "[params]\ntemperature = 0.0", wantSet: true},
		{name: "nonzero", params: "[params]\ntemperature = 0.7", want: 0.7, wantSet: true},
		{name: "field omitted", params: "[params]\nmax_tokens = 4096"},
		{name: "params omitted"},
	}
	for _, mode := range []struct {
		name   string
		stream bool
	}{
		{name: "send"},
		{name: "stream", stream: true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					setupTestEnv(t,
						"[providers.openai]\napi_key = \"fake-key\"\n",
						"test-agent",
						"name = \"test-agent\"\nmodel = \"openai/gpt-4o\"\n"+tt.params,
					)
					prov := &capturingProvider{}
					newProvider := func(_, _, _ string) (provider.Provider, error) {
						return prov, nil
					}
					stdout, stderr := newMockStdoutStderr()
					_, err := run(context.Background(), Options{
						AgentName: "test-agent",
						Prompt:    "Test temperature presence",
						Stdin:     strings.NewReader(""),
						Stdout:    stdout,
						Stderr:    stderr,
						Stream:    mode.stream,
					}, newProvider)
					if err != nil {
						t.Fatalf("run() error = %v", err)
					}
					if len(prov.requests) != 1 {
						t.Fatalf("got %d requests, want 1", len(prov.requests))
					}
					req := prov.requests[0]
					if req.Temperature != tt.want || req.TemperatureSet != tt.wantSet {
						t.Errorf("Request temperature = %g, set = %t, want %g, %t", req.Temperature, req.TemperatureSet, tt.want, tt.wantSet)
					}
				})
			}
		})
	}
}

func TestRun_SingleShot_NoTools(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponse("Hello from runner"),
	})

	setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
`,
	)

	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
	}

	result, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Content != "Hello from runner" {
		t.Errorf("Content = %q, want %q", result.Content, "Hello from runner")
	}
	if result.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want 10", result.InputTokens)
	}
	if result.OutputTokens != 5 {
		t.Errorf("OutputTokens = %d, want 5", result.OutputTokens)
	}
	if result.StopReason != "end_turn" {
		t.Errorf("StopReason = %q, want end_turn", result.StopReason)
	}
	if result.DryRun {
		t.Error("DryRun = true, want false")
	}

	// Content should have been written to stdout
	if !strings.Contains(stdout.String(), "Hello from runner") {
		t.Errorf("stdout missing content: %q", stdout.String())
	}
}

func TestRun_SingleShot_CacheTokens(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponseWithCacheTokens("Hello with cache", 10, 5, 8, 2),
	})

	setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
`,
	)

	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
	}

	result, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.CacheReadTokens != 8 {
		t.Errorf("CacheReadTokens = %d, want 8", result.CacheReadTokens)
	}
	if result.CacheWriteTokens != 2 {
		t.Errorf("CacheWriteTokens = %d, want 2", result.CacheWriteTokens)
	}
}

func TestRun_ConversationLoop_CacheTokensAccumulate(t *testing.T) {
	// First response: tool call with cache tokens
	// Second response: final text with different cache tokens
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponseWithCacheTokens("Let me list", []testutil.MockToolCall{
			{ID: "tool_1", Name: "list_directory", Input: map[string]string{"path": "."}},
		}, 20, 10, 15, 5),
		testutil.AnthropicResponseWithCacheTokens("Done", 8, 4, 3, 1),
	})

	setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["list_directory"]
`,
	)

	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
	}

	result, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.CacheReadTokens != 18 {
		t.Errorf("CacheReadTokens = %d, want 18 (15+3)", result.CacheReadTokens)
	}
	if result.CacheWriteTokens != 6 {
		t.Errorf("CacheWriteTokens = %d, want 6 (5+1)", result.CacheWriteTokens)
	}
}

func TestRun_ConversationLoop_OneToolCall(t *testing.T) {
	// First response: tool call for list_directory
	// Second response: final text
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Let me list the directory", []testutil.MockToolCall{
			{ID: "tool_1", Name: "list_directory", Input: map[string]string{"path": "."}},
		}),
		testutil.AnthropicResponse("Done listing"),
	})

	setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["list_directory"]
`,
	)

	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
	}

	result, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Content != "Done listing" {
		t.Errorf("Content = %q, want %q", result.Content, "Done listing")
	}
	if result.ToolCalls != 1 {
		t.Errorf("ToolCalls = %d, want 1", result.ToolCalls)
	}
	if mock.RequestCount() != 2 {
		t.Errorf("RequestCount = %d, want 2", mock.RequestCount())
	}

	// Content should have been written to stdout
	if !strings.Contains(stdout.String(), "Done listing") {
		t.Errorf("stdout missing content: %q", stdout.String())
	}
}

func TestRun_ParallelToolExecution(t *testing.T) {
	// Single response with two tool calls, then final text
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Let me list two directories", []testutil.MockToolCall{
			{ID: "tool_1", Name: "list_directory", Input: map[string]string{"path": "."}},
			{ID: "tool_2", Name: "list_directory", Input: map[string]string{"path": "."}},
		}),
		testutil.AnthropicResponse("Done with both"),
	})

	setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["list_directory"]
`,
	)

	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
	}

	result, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Content != "Done with both" {
		t.Errorf("Content = %q, want %q", result.Content, "Done with both")
	}
	if result.ToolCalls != 2 {
		t.Errorf("ToolCalls = %d, want 2", result.ToolCalls)
	}
	if mock.RequestCount() != 2 {
		t.Errorf("RequestCount = %d, want 2", mock.RequestCount())
	}
}

func TestRun_SequentialToolExecution(t *testing.T) {
	// Single response with two tool calls, but parallel = false
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Let me list sequentially", []testutil.MockToolCall{
			{ID: "tool_1", Name: "list_directory", Input: map[string]string{"path": "."}},
			{ID: "tool_2", Name: "list_directory", Input: map[string]string{"path": "."}},
		}),
		testutil.AnthropicResponse("Done sequentially"),
	})

	setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["list_directory"]

[sub_agents_config]
parallel = false
`,
	)

	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
	}

	result, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Content != "Done sequentially" {
		t.Errorf("Content = %q, want %q", result.Content, "Done sequentially")
	}
	if result.ToolCalls != 2 {
		t.Errorf("ToolCalls = %d, want 2", result.ToolCalls)
	}
	if mock.RequestCount() != 2 {
		t.Errorf("RequestCount = %d, want 2", mock.RequestCount())
	}
}

func TestRun_Streaming(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicStreamResponse("Hello from stream", 10, 5),
	})

	setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
stream = true
`,
	)

	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
	}

	result, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Content != "Hello from stream" {
		t.Errorf("Content = %q, want %q", result.Content, "Hello from stream")
	}
	if result.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want 10", result.InputTokens)
	}
	if result.OutputTokens != 5 {
		t.Errorf("OutputTokens = %d, want 5", result.OutputTokens)
	}

	// Content should have been streamed to stdout
	if !strings.Contains(stdout.String(), "Hello from stream") {
		t.Errorf("stdout missing streamed content: %q", stdout.String())
	}
}

func TestRun_PreBuiltMessages_Streaming(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicStreamResponse("Hello from stream with history", 10, 5),
	})

	setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
stream = true
`,
	)

	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
		Messages: []Message{
			{Role: "user", Content: "Prior context"},
		},
	}

	result, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Content != "Hello from stream with history" {
		t.Errorf("Content = %q, want %q", result.Content, "Hello from stream with history")
	}
	if result.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want 10", result.InputTokens)
	}
	if result.OutputTokens != 5 {
		t.Errorf("OutputTokens = %d, want 5", result.OutputTokens)
	}
	// Content should have been streamed to stdout
	if !strings.Contains(stdout.String(), "Hello from stream with history") {
		t.Errorf("stdout missing streamed content: %q", stdout.String())
	}
}

func TestRun_BudgetTracking(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponseWithTokens("Within budget", 8, 7),
	})

	setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"

[budget]
max_tokens = 20
`,
	)

	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
	}

	result, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Budget.Max != 20 {
		t.Errorf("Budget.Max = %d, want 20", result.Budget.Max)
	}
	if result.Budget.Used != 15 {
		t.Errorf("Budget.Used = %d, want 15", result.Budget.Used)
	}
	if result.Budget.Exceeded {
		t.Error("Budget.Exceeded = true, want false")
	}
}

func TestRun_BudgetExceeded(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponseWithTokens("Over budget", 8, 7),
	})

	setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"

[budget]
max_tokens = 10
`,
	)

	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
	}

	_, err := Run(context.Background(), opts)
	if err == nil {
		t.Fatal("Run() expected BudgetExceededError, got nil")
	}
	if !IsBudgetExceededError(err) {
		t.Fatalf("expected BudgetExceededError, got %T: %v", err, err)
	}
	be := err.(*BudgetExceededError)
	if be.Max != 10 {
		t.Errorf("Max = %d, want 10", be.Max)
	}
	if be.Used != 15 {
		t.Errorf("Used = %d, want 15", be.Used)
	}
}

func TestRun_MemoryAppend(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponse("Remember this"),
	})

	tmpDir := setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
`,
	)

	dataDir := filepath.Join(tmpDir, "data")
	t.Setenv("XDG_DATA_HOME", dataDir)
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "test-agent",
		Prompt:    "Test prompt for memory",
		Stdout:    stdout,
		Stderr:    stderr,
	}

	_, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Check memory file was created
	memPath := filepath.Join(dataDir, "axe", "memory", "test-agent.md")
	data, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("memory file not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Test prompt for memory") {
		t.Errorf("memory missing task: %q", content)
	}
	if !strings.Contains(content, "Remember this") {
		t.Errorf("memory missing result: %q", content)
	}
}

func TestRun_SubAgent(t *testing.T) {
	// Parent calls child; child returns text; parent returns final text.
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		// Request 1: parent sees call_agent tool, returns tool_use
		testutil.AnthropicToolUseResponse("Delegating to child", []testutil.MockToolCall{
			{ID: "tool_1", Name: "call_agent", Input: map[string]string{
				"agent": "child-agent",
				"task":  "say hello",
			}},
		}),
		// Request 2: child has no tools, returns text
		testutil.AnthropicResponse("hello from child"),
		// Request 3: parent receives tool result, returns final text
		testutil.AnthropicResponse("Child said hello from child"),
	})

	tmpDir := setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"parent-agent",
		`name = "parent-agent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["child-agent"]
`,
	)

	// Create child agent
	childPath := filepath.Join(tmpDir, "axe", "agents", "child-agent.toml")
	childTOML := `name = "child-agent"
model = "anthropic/claude-sonnet-4-20250514"
`
	if err := os.WriteFile(childPath, []byte(childTOML), 0644); err != nil {
		t.Fatalf("failed to write child agent: %v", err)
	}

	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "parent-agent",
		Stdout:    stdout,
		Stderr:    stderr,
	}

	result, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Content != "Child said hello from child" {
		t.Errorf("Content = %q, want %q", result.Content, "Child said hello from child")
	}
	if result.ToolCalls != 1 {
		t.Errorf("ToolCalls = %d, want 1", result.ToolCalls)
	}
	if mock.RequestCount() != 3 {
		t.Errorf("RequestCount = %d, want 3", mock.RequestCount())
	}
}

func TestRun_JSONOutput_CacheTokens(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponseWithCacheTokens("JSON cache test", 10, 5, 8, 2),
	})

	setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
`,
	)

	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "test-agent",
		JSON:      true,
		Stdout:    stdout,
		Stderr:    stderr,
	}

	result, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.CacheReadTokens != 8 {
		t.Errorf("CacheReadTokens = %d, want 8", result.CacheReadTokens)
	}
	if result.CacheWriteTokens != 2 {
		t.Errorf("CacheWriteTokens = %d, want 2", result.CacheWriteTokens)
	}

	// Verify JSON output includes cache fields
	out := stdout.String()
	if !strings.Contains(out, `"cache_read_tokens"`) {
		t.Errorf("JSON missing cache_read_tokens: %q", out)
	}
	if !strings.Contains(out, `"cache_write_tokens"`) {
		t.Errorf("JSON missing cache_write_tokens: %q", out)
	}
}

func TestRun_JSONOutput(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponse("JSON test"),
	})

	setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
`,
	)

	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "test-agent",
		JSON:      true,
		Stdout:    stdout,
		Stderr:    stderr,
	}

	result, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Content != "JSON test" {
		t.Errorf("Content = %q, want %q", result.Content, "JSON test")
	}

	// stdout should contain JSON, not raw text
	out := stdout.String()
	if strings.Contains(out, "JSON test") && !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("stdout should contain JSON envelope, got: %q", out)
	}
	if !strings.Contains(out, `"content"`) {
		t.Errorf("stdout missing JSON content field: %q", out)
	}
	if !strings.Contains(out, `"input_tokens"`) {
		t.Errorf("stdout missing JSON input_tokens field: %q", out)
	}
}

func TestRun_PreBuiltMessages_SingleShot(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponse("Response to pre-built"),
	})

	setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
`,
	)

	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
		Messages: []Message{
			{Role: "user", Content: "Hello from history"},
		},
	}

	result, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Content != "Response to pre-built" {
		t.Errorf("Content = %q, want %q", result.Content, "Response to pre-built")
	}
	if len(result.Messages) != 2 {
		t.Fatalf("Result.Messages len = %d, want 2 (user + assistant)", len(result.Messages))
	}
	if result.Messages[0].Role != "user" || result.Messages[0].Content != "Hello from history" {
		t.Errorf("Messages[0] = %+v", result.Messages[0])
	}
	if result.Messages[1].Role != "assistant" || result.Messages[1].Content != "Response to pre-built" {
		t.Errorf("Messages[1] = %+v", result.Messages[1])
	}
}

func TestRun_PreBuiltMessages_ToolLoop(t *testing.T) {
	// Pre-built history: prior user + assistant tool call + tool result.
	// Then agent responds with another tool call, then final text.
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Next tool", []testutil.MockToolCall{
			{ID: "tool_2", Name: "list_directory", Input: map[string]string{"path": "."}},
		}),
		testutil.AnthropicResponse("All done"),
	})

	setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["list_directory"]
`,
	)

	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
		Messages: []Message{
			{Role: "user", Content: "First turn"},
			{
				Role:    "assistant",
				Content: "Let me list",
				ToolCalls: []ToolCall{
					{ID: "tool_1", Name: "list_directory", Arguments: map[string]string{"path": "/tmp"}},
				},
			},
			{
				Role: "tool",
				ToolResults: []ToolResult{
					{CallID: "tool_1", Content: "a.txt\nb.txt"},
				},
			},
		},
	}

	result, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Content != "All done" {
		t.Errorf("Content = %q, want %q", result.Content, "All done")
	}
	if result.ToolCalls != 1 {
		t.Errorf("ToolCalls = %d, want 1", result.ToolCalls)
	}
	// Should have: user, assistant(1), tool(1), assistant(2), tool(2), assistant(final)
	if len(result.Messages) != 6 {
		t.Fatalf("Result.Messages len = %d, want 6", len(result.Messages))
	}
	if result.Messages[0].Content != "First turn" {
		t.Errorf("Messages[0].Content = %q, want 'First turn'", result.Messages[0].Content)
	}
	if len(result.Messages[1].ToolCalls) != 1 || result.Messages[1].ToolCalls[0].ID != "tool_1" {
		t.Errorf("Messages[1].ToolCalls = %+v", result.Messages[1].ToolCalls)
	}
	if result.Messages[2].ToolResults[0].CallID != "tool_1" {
		t.Errorf("Messages[2].ToolResults = %+v", result.Messages[2].ToolResults)
	}
	if len(result.Messages[3].ToolCalls) != 1 || result.Messages[3].ToolCalls[0].ID != "tool_2" {
		t.Errorf("Messages[3].ToolCalls = %+v", result.Messages[3].ToolCalls)
	}
	if result.Messages[5].Role != "assistant" || result.Messages[5].Content != "All done" {
		t.Errorf("Messages[5] = %+v", result.Messages[5])
	}
}

func TestRun_PreBuiltMessages_InvalidRole(t *testing.T) {
	setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
`,
	)

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
		Messages: []Message{
			{Role: "user", Content: "ok"},
			{Role: "system", Content: "bad"},
		},
	}

	_, err := Run(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
	if !IsConfigError(err) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
}

func TestRun_PreBuiltMessages_DanglingCallID(t *testing.T) {
	setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
`,
	)

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
		Messages: []Message{
			{Role: "user", Content: "ok"},
			{Role: "assistant", ToolCalls: []ToolCall{{ID: "a", Name: "n"}}},
			{Role: "tool", ToolResults: []ToolResult{{CallID: "b", Content: "x"}}},
		},
	}

	_, err := Run(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error for dangling call ID")
	}
	if !IsConfigError(err) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
}

func TestRun_PreBuiltMessages_EmptySlice_FallsBackToPrompt(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponse("Got prompt"),
	})

	setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
`,
	)

	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "test-agent",
		Prompt:    "My prompt",
		Stdout:    stdout,
		Stderr:    stderr,
		Messages:  []Message{}, // empty but non-nil
	}

	result, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Content != "Got prompt" {
		t.Errorf("Content = %q, want %q", result.Content, "Got prompt")
	}
}

func TestRun_PreBuiltMessages_DryRun(t *testing.T) {
	setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
`,
	)

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "test-agent",
		DryRun:    true,
		Stdout:    stdout,
		Stderr:    stderr,
		Messages: []Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi"},
		},
	}

	result, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.DryRun {
		t.Fatal("expected DryRun = true")
	}
	if result.DryRunInfo == nil {
		t.Fatal("expected DryRunInfo")
	}
	if result.DryRunInfo.UserMessage != "(pre-built message history)" {
		t.Errorf("UserMessage = %q, want %q", result.DryRunInfo.UserMessage, "(pre-built message history)")
	}
	if result.DryRunInfo.MessageCount != 2 {
		t.Errorf("MessageCount = %d, want 2", result.DryRunInfo.MessageCount)
	}
}

func TestRun_PreBuiltMessages_MemoryAppend(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponse("Stored history"),
	})

	tmpDir := setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
`,
	)

	dataDir := filepath.Join(tmpDir, "data")
	t.Setenv("XDG_DATA_HOME", dataDir)
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
		Messages: []Message{
			{Role: "assistant", Content: "Say hi"},
			{Role: "user", Content: "User task"},
		},
	}

	_, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	memPath := filepath.Join(dataDir, "axe", "memory", "test-agent.md")
	data, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("memory file not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "User task") {
		t.Errorf("memory missing first user message task: %q", content)
	}
	if !strings.Contains(content, "Stored history") {
		t.Errorf("memory missing result: %q", content)
	}
}

func TestRun_PreBuiltMessages_MemoryAppend_NoUserMessage(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponse("Stored fallback"),
	})

	tmpDir := setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
`,
	)

	dataDir := filepath.Join(tmpDir, "data")
	t.Setenv("XDG_DATA_HOME", dataDir)
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
		Messages: []Message{
			{Role: "assistant", Content: "No user here"},
		},
	}

	_, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	memPath := filepath.Join(dataDir, "axe", "memory", "test-agent.md")
	data, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("memory file not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "(pre-built message history with no user message)") {
		t.Errorf("memory missing fallback task: %q", content)
	}
}

func TestRun_MaxTurnsExceeded(t *testing.T) {
	// Mock always returns a tool call, never a text response.
	// Need 50 responses (one per default max turn).
	responses := make([]testutil.MockLLMResponse, 50)
	for i := range responses {
		responses[i] = testutil.AnthropicToolUseResponse("Tool call", []testutil.MockToolCall{
			{ID: fmt.Sprintf("tool_%d", i), Name: "list_directory", Input: map[string]string{"path": "."}},
		})
	}
	mock := testutil.NewMockLLMServer(t, responses)

	setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["list_directory"]
`,
	)

	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()

	opts := Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
	}

	_, err := Run(context.Background(), opts)
	if err == nil {
		t.Fatal("Run() expected error for max turns exceeded, got nil")
	}
	if !IsRuntimeError(err) {
		t.Fatalf("expected RuntimeError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "exceeded maximum conversation turns") {
		t.Errorf("error message missing 'exceeded maximum conversation turns': %v", err)
	}
}

func TestRun_MaxTurnsConfigured(t *testing.T) {
	responses := make([]testutil.MockLLMResponse, 2)
	for i := range responses {
		responses[i] = testutil.AnthropicToolUseResponse("Tool call", []testutil.MockToolCall{
			{ID: fmt.Sprintf("tool_%d", i), Name: "list_directory", Input: map[string]string{"path": "."}},
		})
	}
	mock := testutil.NewMockLLMServer(t, responses)

	setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
max_turns = 2
tools = ["list_directory"]
`,
	)

	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()
	opts := Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
	}

	_, err := Run(context.Background(), opts)
	if err == nil {
		t.Fatal("Run() expected error for configured max turns exceeded, got nil")
	}
	if !IsRuntimeError(err) {
		t.Fatalf("expected RuntimeError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "exceeded maximum conversation turns (2)") {
		t.Errorf("expected configured max turns in error, got: %v", err)
	}
	if got := mock.RequestCount(); got != 2 {
		t.Errorf("RequestCount = %d, want 2", got)
	}
}

// --- parseModel edge case tests ---

func TestParseModel_EmptyProvider(t *testing.T) {
	_, _, err := parseModel("/model-name")
	if err == nil {
		t.Fatal("expected error for empty provider")
	}
	if !strings.Contains(err.Error(), "empty provider") {
		t.Errorf("expected 'empty provider' error, got: %v", err)
	}
}

func TestParseModel_EmptyModelName(t *testing.T) {
	_, _, err := parseModel("provider/")
	if err == nil {
		t.Fatal("expected error for empty model name")
	}
	if !strings.Contains(err.Error(), "empty model name") {
		t.Errorf("expected 'empty model name' error, got: %v", err)
	}
}

func TestParseModel_MultipleSlashes(t *testing.T) {
	// Should split on first slash only
	prov, model, err := parseModel("anthropic/claude-sonnet-4-20250514/extra")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prov != "anthropic" {
		t.Errorf("provider = %q, want 'anthropic'", prov)
	}
	if model != "claude-sonnet-4-20250514/extra" {
		t.Errorf("model = %q, want 'claude-sonnet-4-20250514/extra'", model)
	}
}

// --- mapProviderError tests ---

func TestMapProviderError_Auth(t *testing.T) {
	inner := &provider.ProviderError{Category: provider.ErrCategoryAuth, Message: "bad key"}
	err := mapProviderError(inner)
	if !IsRuntimeError(err) {
		t.Fatalf("expected RuntimeError, got %T", err)
	}
	if !strings.Contains(err.Error(), "auth error") {
		t.Errorf("expected auth error message, got: %v", err)
	}
}

func TestMapProviderError_RateLimit(t *testing.T) {
	inner := &provider.ProviderError{Category: provider.ErrCategoryRateLimit, Message: "slow down"}
	err := mapProviderError(inner)
	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("expected rate limit message, got: %v", err)
	}
}

func TestMapProviderError_Server(t *testing.T) {
	inner := &provider.ProviderError{Category: provider.ErrCategoryServer, Message: "boom"}
	err := mapProviderError(inner)
	if !strings.Contains(err.Error(), "server") {
		t.Errorf("expected server error message, got: %v", err)
	}
}

func TestMapProviderError_BadRequest(t *testing.T) {
	inner := &provider.ProviderError{Category: provider.ErrCategoryBadRequest, Message: "invalid"}
	err := mapProviderError(inner)
	if !strings.Contains(err.Error(), "bad request") {
		t.Errorf("expected bad request message, got: %v", err)
	}
}

func TestMapProviderError_NonProviderError(t *testing.T) {
	inner := errors.New("plain failure")
	err := mapProviderError(inner)
	if !IsRuntimeError(err) {
		t.Fatalf("expected RuntimeError, got %T", err)
	}
	if !strings.Contains(err.Error(), "provider call failed") {
		t.Errorf("expected generic provider failure message, got: %v", err)
	}
}
