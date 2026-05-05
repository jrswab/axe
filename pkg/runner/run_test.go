package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestRun_MaxTurnsExceeded(t *testing.T) {
	// Mock always returns a tool call, never a text response.
	// Need 50 responses (one per max turn).
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
