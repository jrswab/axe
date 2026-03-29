package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jrswab/axe/internal/testutil"
)

// writeAgentConfig writes a TOML agent config file to configDir/agents/<name>.toml.
func writeAgentConfig(t *testing.T, configDir, name, toml string) {
	t.Helper()
	agentsDir := filepath.Join(configDir, "agents")
	if err := os.WriteFile(filepath.Join(agentsDir, name+".toml"), []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}
}

// --- Phase 11: Single-Shot Run Tests ---

func TestIntegration_SingleShot_Anthropic(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponse("Hello from Anthropic"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "single-anthropic", `name = "single-anthropic"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "single-anthropic"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	output := buf.String()
	if output != "Hello from Anthropic" {
		t.Errorf("expected stdout %q, got %q", "Hello from Anthropic", output)
	}

	if mock.RequestCount() != 1 {
		t.Errorf("expected 1 request, got %d", mock.RequestCount())
	}

	if mock.Requests[0].Path != "/v1/messages" {
		t.Errorf("expected path /v1/messages, got %q", mock.Requests[0].Path)
	}

	if mock.Requests[0].Method != "POST" {
		t.Errorf("expected method POST, got %q", mock.Requests[0].Method)
	}

	body := mock.Requests[0].Body
	if !strings.Contains(body, "claude-sonnet-4-20250514") {
		t.Errorf("expected request body to contain model name, got %q", body)
	}

	if !strings.Contains(body, defaultUserMessage) {
		t.Errorf("expected request body to contain default user message, got %q", body)
	}
}

func TestIntegration_SingleShot_OpenAI(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.OpenAIResponse("Hello from OpenAI"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "single-openai", `name = "single-openai"
model = "openai/gpt-4o"
`)

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("AXE_OPENAI_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "single-openai"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	output := buf.String()
	if output != "Hello from OpenAI" {
		t.Errorf("expected stdout %q, got %q", "Hello from OpenAI", output)
	}

	if mock.RequestCount() != 1 {
		t.Errorf("expected 1 request, got %d", mock.RequestCount())
	}

	if mock.Requests[0].Path != "/chat/completions" {
		t.Errorf("expected path /chat/completions, got %q", mock.Requests[0].Path)
	}

	body := mock.Requests[0].Body
	if !strings.Contains(body, "gpt-4o") {
		t.Errorf("expected request body to contain model name, got %q", body)
	}
}

func TestIntegration_SingleShot_StdinPiped(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponse("Processed your input"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "stdin-agent", `name = "stdin-agent"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetIn(bytes.NewReader([]byte("user-provided input")))
	rootCmd.SetArgs([]string{"run", "stdin-agent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Processed your input") {
		t.Errorf("expected stdout to contain %q, got %q", "Processed your input", output)
	}

	body := mock.Requests[0].Body
	if !strings.Contains(body, "user-provided input") {
		t.Errorf("expected request body to contain user input, got %q", body)
	}

	if strings.Contains(body, defaultUserMessage) {
		t.Errorf("expected request body to NOT contain default message, got %q", body)
	}
}

// --- Phase 12: Conversation Loop Tests ---

func TestIntegration_ConversationLoop_SingleToolCall(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		// Turn 1: parent returns tool_use calling helper-agent
		testutil.AnthropicToolUseResponse("Delegating.", []testutil.MockToolCall{
			{ID: "toolu_1", Name: "call_agent", Input: map[string]string{"agent": "helper-agent", "task": "say hello"}},
		}),
		// Sub-agent call: helper returns a simple response
		testutil.AnthropicResponse("hello from helper"),
		// Turn 2: parent returns final text
		testutil.AnthropicResponse("The helper said: hello from helper"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "loop-parent", `name = "loop-parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["helper-agent"]
`)
	writeAgentConfig(t, configDir, "helper-agent", `name = "helper-agent"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "loop-parent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "The helper said: hello from helper") {
		t.Errorf("expected stdout to contain final response, got %q", output)
	}

	if mock.RequestCount() != 3 {
		t.Errorf("expected 3 requests, got %d", mock.RequestCount())
	}

	// Second request (index 1) is the sub-agent call - should contain the task
	if !strings.Contains(mock.Requests[1].Body, "say hello") {
		t.Errorf("expected sub-agent request to contain task 'say hello', got %q", mock.Requests[1].Body)
	}

	// Third request (index 2) should contain the tool result from the helper
	if !strings.Contains(mock.Requests[2].Body, "hello from helper") {
		t.Errorf("expected third request to contain tool result, got %q", mock.Requests[2].Body)
	}
}

func TestIntegration_ConversationLoop_MultipleRoundTrips(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		// Turn 1: tool call
		testutil.AnthropicToolUseResponse("First delegation.", []testutil.MockToolCall{
			{ID: "toolu_1", Name: "call_agent", Input: map[string]string{"agent": "multi-helper", "task": "step one"}},
		}),
		// Sub-agent 1 response
		testutil.AnthropicResponse("step one done"),
		// Turn 2: another tool call
		testutil.AnthropicToolUseResponse("Second delegation.", []testutil.MockToolCall{
			{ID: "toolu_2", Name: "call_agent", Input: map[string]string{"agent": "multi-helper", "task": "step two"}},
		}),
		// Sub-agent 2 response
		testutil.AnthropicResponse("step two done"),
		// Turn 3: final response
		testutil.AnthropicResponse("All steps completed"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "multi-parent", `name = "multi-parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["multi-helper"]
`)
	writeAgentConfig(t, configDir, "multi-helper", `name = "multi-helper"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "multi-parent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	if mock.RequestCount() != 5 {
		t.Errorf("expected 5 requests, got %d", mock.RequestCount())
	}

	output := buf.String()
	if !strings.Contains(output, "All steps completed") {
		t.Errorf("expected stdout to contain final response, got %q", output)
	}
}

func TestIntegration_ConversationLoop_MaxTurnsExceeded(t *testing.T) {
	resetRunCmd(t)

	// Queue 100 responses: 50 parent tool_use + 50 sub-agent responses
	// Each turn: parent returns tool_use -> sub-agent returns text -> loop continues
	var responses []testutil.MockLLMResponse
	for i := 0; i < 50; i++ {
		responses = append(responses,
			testutil.AnthropicToolUseResponse("Delegating again.", []testutil.MockToolCall{
				{ID: "toolu_loop", Name: "call_agent", Input: map[string]string{"agent": "loop-helper", "task": "keep going"}},
			}),
			testutil.AnthropicResponse("still going"),
		)
	}

	mock := testutil.NewMockLLMServer(t, responses)

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "max-turns-parent", `name = "max-turns-parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["loop-helper"]
`)
	writeAgentConfig(t, configDir, "loop-helper", `name = "loop-helper"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "max-turns-parent"})

	err := rootCmd.Execute()
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 1 {
		t.Errorf("expected exit code 1, got %d", exitErr.Code)
	}
	if !strings.Contains(err.Error(), "exceeded maximum conversation turns") {
		t.Errorf("expected error about max turns, got %q", err.Error())
	}
}

// --- Phase 13: Sub-Agent Orchestration Tests ---

func TestIntegration_SubAgent_DepthLimit(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		// Parent: tool call to depth-child
		testutil.AnthropicToolUseResponse("Calling child.", []testutil.MockToolCall{
			{ID: "toolu_d1", Name: "call_agent", Input: map[string]string{"agent": "depth-child", "task": "do work"}},
		}),
		// Child runs at depth 1 = max_depth, so no tools injected. Single-shot response.
		testutil.AnthropicResponse("child result"),
		// Parent second turn: final response
		testutil.AnthropicResponse("Got: child result"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "depth-parent", `name = "depth-parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["depth-child"]

[sub_agents_config]
max_depth = 1
`)
	writeAgentConfig(t, configDir, "depth-child", `name = "depth-child"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["depth-grandchild"]
`)
	writeAgentConfig(t, configDir, "depth-grandchild", `name = "depth-grandchild"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "depth-parent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Got: child result") {
		t.Errorf("expected stdout to contain final response, got %q", output)
	}

	if mock.RequestCount() != 3 {
		t.Errorf("expected 3 requests, got %d", mock.RequestCount())
	}

	// Second request (child's request at depth 1) should NOT contain tools
	childBody := mock.Requests[1].Body
	if strings.Contains(childBody, `"tools"`) {
		t.Errorf("expected child request to NOT contain tools (depth limit reached), got %q", childBody)
	}
}

func TestIntegration_SubAgent_ParallelExecution(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		// Parent: two simultaneous tool calls
		testutil.AnthropicToolUseResponse("Calling workers.", []testutil.MockToolCall{
			{ID: "toolu_a", Name: "call_agent", Input: map[string]string{"agent": "worker-a", "task": "task A"}},
			{ID: "toolu_b", Name: "call_agent", Input: map[string]string{"agent": "worker-b", "task": "task B"}},
		}),
		// Worker responses (order may vary due to parallelism)
		testutil.AnthropicResponse("result A"),
		testutil.AnthropicResponse("result B"),
		// Parent second turn: final response
		testutil.AnthropicResponse("Both workers done"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "par-parent", `name = "par-parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["worker-a", "worker-b"]
`)
	writeAgentConfig(t, configDir, "worker-a", `name = "worker-a"
model = "anthropic/claude-sonnet-4-20250514"
`)
	writeAgentConfig(t, configDir, "worker-b", `name = "worker-b"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "par-parent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	if mock.RequestCount() != 4 {
		t.Errorf("expected 4 requests, got %d", mock.RequestCount())
	}

	output := buf.String()
	if !strings.Contains(output, "Both workers done") {
		t.Errorf("expected stdout to contain final response, got %q", output)
	}
}

func TestIntegration_SubAgent_SequentialExecution(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		// Parent: two tool calls
		testutil.AnthropicToolUseResponse("Calling workers sequentially.", []testutil.MockToolCall{
			{ID: "toolu_a", Name: "call_agent", Input: map[string]string{"agent": "seq-worker-a", "task": "task A"}},
			{ID: "toolu_b", Name: "call_agent", Input: map[string]string{"agent": "seq-worker-b", "task": "task B"}},
		}),
		// Worker A response (sequential: A always before B)
		testutil.AnthropicResponse("result A"),
		// Worker B response
		testutil.AnthropicResponse("result B"),
		// Parent second turn: final response
		testutil.AnthropicResponse("Sequential workers done"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "seq-parent", `name = "seq-parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["seq-worker-a", "seq-worker-b"]

[sub_agents_config]
parallel = false
`)
	writeAgentConfig(t, configDir, "seq-worker-a", `name = "seq-worker-a"
model = "anthropic/claude-sonnet-4-20250514"
`)
	writeAgentConfig(t, configDir, "seq-worker-b", `name = "seq-worker-b"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "seq-parent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	if mock.RequestCount() != 4 {
		t.Errorf("expected 4 requests, got %d", mock.RequestCount())
	}

	output := buf.String()
	if !strings.Contains(output, "Sequential workers done") {
		t.Errorf("expected stdout to contain final response, got %q", output)
	}

	// In sequential mode, worker-a request must come before worker-b request.
	// Requests[0] = parent, Requests[1] = first worker, Requests[2] = second worker, Requests[3] = parent turn 2
	workerABody := mock.Requests[1].Body
	workerBBody := mock.Requests[2].Body
	if !strings.Contains(workerABody, "task A") {
		t.Errorf("expected first worker request to contain 'task A', got %q", workerABody)
	}
	if !strings.Contains(workerBBody, "task B") {
		t.Errorf("expected second worker request to contain 'task B', got %q", workerBBody)
	}
}

// --- Phase 14: Memory Tests ---

func TestIntegration_MemoryAppend_AfterSuccessfulRun(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponse("task completed"),
	})

	configDir, dataDir := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "mem-agent", `name = "mem-agent"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "mem-agent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	memoryFile := filepath.Join(dataDir, "memory", "mem-agent.md")
	data, readErr := os.ReadFile(memoryFile)
	if readErr != nil {
		t.Fatalf("expected memory file to exist at %s: %v", memoryFile, readErr)
	}

	content := string(data)
	if !strings.Contains(content, "## ") {
		t.Errorf("expected entry header in memory file, got %q", content)
	}
	if !strings.Contains(content, "**Task:**") {
		t.Errorf("expected Task line in memory file, got %q", content)
	}
	if !strings.Contains(content, "**Result:** task completed") {
		t.Errorf("expected Result line with 'task completed' in memory file, got %q", content)
	}
}

func TestIntegration_MemoryAppend_NotOnError(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicErrorResponse(500, "server_error", "boom"),
	})

	configDir, dataDir := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "mem-err-agent", `name = "mem-err-agent"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "mem-err-agent"})

	err := rootCmd.Execute()
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 3 {
		t.Errorf("expected exit code 3, got %d", exitErr.Code)
	}

	memoryFile := filepath.Join(dataDir, "memory", "mem-err-agent.md")
	if _, statErr := os.Stat(memoryFile); !os.IsNotExist(statErr) {
		t.Errorf("expected memory file to NOT exist after error, but got: %v", statErr)
	}
}

func TestIntegration_MemoryLoad_IntoSystemPrompt(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponse("ok"),
	})

	configDir, dataDir := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "mem-load", `name = "mem-load"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
last_n = 2
`)

	// Pre-seed memory file with 3 entries
	memoryDir := filepath.Join(dataDir, "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		t.Fatal(err)
	}
	memoryContent := `## 2026-01-01T10:00:00Z
**Task:** oldest task
**Result:** oldest result

## 2026-01-02T10:00:00Z
**Task:** middle task
**Result:** middle result

## 2026-01-03T10:00:00Z
**Task:** newest task
**Result:** newest result

`
	if err := os.WriteFile(filepath.Join(memoryDir, "mem-load.md"), []byte(memoryContent), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "mem-load"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	// Verify that the request body contains the last 2 entries but NOT the oldest
	body := mock.Requests[0].Body
	if !strings.Contains(body, "middle task") {
		t.Errorf("expected request body to contain 'middle task' (2nd entry), got body length %d", len(body))
	}
	if !strings.Contains(body, "newest task") {
		t.Errorf("expected request body to contain 'newest task' (3rd entry), got body length %d", len(body))
	}
	if strings.Contains(body, "oldest task") {
		t.Errorf("expected request body to NOT contain 'oldest task' (trimmed by last_n=2)")
	}
}

// --- Phase 15: JSON Output Tests ---

func TestIntegration_JSONOutput_Structure(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponse("json test output"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "json-agent", `name = "json-agent"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "json-agent", "--json"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	var result map[string]interface{}
	if jsonErr := json.Unmarshal(buf.Bytes(), &result); jsonErr != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\nraw: %q", jsonErr, buf.String())
	}

	// Verify required fields
	requiredFields := []string{"model", "content", "input_tokens", "output_tokens", "stop_reason", "duration_ms", "tool_calls", "tool_call_details", "refused"}
	for _, field := range requiredFields {
		if _, ok := result[field]; !ok {
			t.Errorf("expected JSON field %q to be present", field)
		}
	}

	if content, ok := result["content"].(string); !ok || content != "json test output" {
		t.Errorf("expected content %q, got %v", "json test output", result["content"])
	}

	if stopReason, ok := result["stop_reason"].(string); !ok || stopReason != "end_turn" {
		t.Errorf("expected stop_reason %q, got %v", "end_turn", result["stop_reason"])
	}

	if toolCalls, ok := result["tool_calls"].(float64); !ok || toolCalls != 0 {
		t.Errorf("expected tool_calls 0, got %v", result["tool_calls"])
	}

	toolCallDetailsRaw, ok := result["tool_call_details"]
	if !ok {
		t.Fatalf("expected tool_call_details field to be present")
	}
	toolCallDetails, ok := toolCallDetailsRaw.([]interface{})
	if !ok {
		t.Fatalf("expected tool_call_details to be array, got %T", toolCallDetailsRaw)
	}
	if len(toolCallDetails) != 0 {
		t.Errorf("expected tool_call_details to be empty array, got length %d", len(toolCallDetails))
	}

	if durationMs, ok := result["duration_ms"].(float64); !ok || durationMs < 0 {
		t.Errorf("expected duration_ms >= 0, got %v", result["duration_ms"])
	}

	if inputTokens, ok := result["input_tokens"].(float64); !ok || inputTokens != 10 {
		t.Errorf("expected input_tokens 10, got %v", result["input_tokens"])
	}

	if outputTokens, ok := result["output_tokens"].(float64); !ok || outputTokens != 5 {
		t.Errorf("expected output_tokens 5, got %v", result["output_tokens"])
	}

	if refused, ok := result["refused"].(bool); !ok || refused {
		t.Errorf("expected refused false, got %v", result["refused"])
	}
}

func TestIntegration_JSONOutput_RefusalDetected(t *testing.T) {
	resetRunCmd(t)

	refusalText := "I'm sorry, but I cannot assist with that request."
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponse(refusalText),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "json-refusal", `name = "json-refusal"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "json-refusal", "--json"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error (exit code 0), got: %v", err)
	}

	var result map[string]interface{}
	if jsonErr := json.Unmarshal(buf.Bytes(), &result); jsonErr != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\nraw: %q", jsonErr, buf.String())
	}

	requiredFields := []string{"model", "content", "input_tokens", "output_tokens", "stop_reason", "duration_ms", "tool_calls", "tool_call_details", "refused"}
	for _, field := range requiredFields {
		if _, ok := result[field]; !ok {
			t.Errorf("expected JSON field %q to be present", field)
		}
	}

	if content, ok := result["content"].(string); !ok || content != refusalText {
		t.Errorf("expected content %q, got %v", refusalText, result["content"])
	}
	if stopReason, ok := result["stop_reason"].(string); !ok || stopReason != "end_turn" {
		t.Errorf("expected stop_reason %q, got %v", "end_turn", result["stop_reason"])
	}
	if inputTokens, ok := result["input_tokens"].(float64); !ok || inputTokens != 10 {
		t.Errorf("expected input_tokens 10, got %v", result["input_tokens"])
	}
	if outputTokens, ok := result["output_tokens"].(float64); !ok || outputTokens != 5 {
		t.Errorf("expected output_tokens 5, got %v", result["output_tokens"])
	}
	if toolCalls, ok := result["tool_calls"].(float64); !ok || toolCalls != 0 {
		t.Errorf("expected tool_calls 0, got %v", result["tool_calls"])
	}
	if details, ok := result["tool_call_details"].([]interface{}); !ok || len(details) != 0 {
		t.Errorf("expected empty tool_call_details, got %v", result["tool_call_details"])
	}
	if refused, ok := result["refused"].(bool); !ok || !refused {
		t.Errorf("expected refused true, got %v", result["refused"])
	}
}

func TestIntegration_JSONOutput_RefusalDetected_WithToolCalls(t *testing.T) {
	resetRunCmd(t)

	refusalText := "As an AI, I must decline this request."
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Delegating.", []testutil.MockToolCall{
			{ID: "toolu_r1", Name: "call_agent", Input: map[string]string{"agent": "json-refusal-helper", "task": "do work"}},
		}),
		testutil.AnthropicResponse("helper result"),
		testutil.AnthropicResponse(refusalText),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "json-refusal-parent", `name = "json-refusal-parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["json-refusal-helper"]
`)
	writeAgentConfig(t, configDir, "json-refusal-helper", `name = "json-refusal-helper"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "json-refusal-parent", "--json"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error (exit code 0), got: %v", err)
	}

	var result map[string]interface{}
	if jsonErr := json.Unmarshal(buf.Bytes(), &result); jsonErr != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\nraw: %q", jsonErr, buf.String())
	}

	if content, ok := result["content"].(string); !ok || content != refusalText {
		t.Errorf("expected refusal content %q, got %v", refusalText, result["content"])
	}
	if toolCalls, ok := result["tool_calls"].(float64); !ok || toolCalls != 1 {
		t.Errorf("expected tool_calls 1, got %v", result["tool_calls"])
	}
	if refused, ok := result["refused"].(bool); !ok || !refused {
		t.Errorf("expected refused true, got %v", result["refused"])
	}
}

func TestIntegration_JSONOutput_NoRefusal_WithToolCalls(t *testing.T) {
	resetRunCmd(t)

	finalText := "All done with the requested work."
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Delegating.", []testutil.MockToolCall{
			{ID: "toolu_nr1", Name: "call_agent", Input: map[string]string{"agent": "json-no-refusal-helper", "task": "do work"}},
		}),
		testutil.AnthropicResponse("helper result"),
		testutil.AnthropicResponse(finalText),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "json-no-refusal-parent", `name = "json-no-refusal-parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["json-no-refusal-helper"]
`)
	writeAgentConfig(t, configDir, "json-no-refusal-helper", `name = "json-no-refusal-helper"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "json-no-refusal-parent", "--json"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error (exit code 0), got: %v", err)
	}

	var result map[string]interface{}
	if jsonErr := json.Unmarshal(buf.Bytes(), &result); jsonErr != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\nraw: %q", jsonErr, buf.String())
	}

	if content, ok := result["content"].(string); !ok || content != finalText {
		t.Errorf("expected content %q, got %v", finalText, result["content"])
	}
	if toolCalls, ok := result["tool_calls"].(float64); !ok || toolCalls != 1 {
		t.Errorf("expected tool_calls 1, got %v", result["tool_calls"])
	}
	if refused, ok := result["refused"].(bool); !ok || refused {
		t.Errorf("expected refused false, got %v", result["refused"])
	}
}

func TestIntegration_JSONOutput_WithToolCalls(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		// Parent: tool call (input_tokens: 10, output_tokens: 20)
		testutil.AnthropicToolUseResponse("Delegating.", []testutil.MockToolCall{
			{ID: "toolu_j1", Name: "call_agent", Input: map[string]string{"agent": "json-helper", "task": "do work"}},
		}),
		// Sub-agent response (input_tokens: 10, output_tokens: 5)
		testutil.AnthropicResponse("helper done"),
		// Parent final (input_tokens: 10, output_tokens: 5)
		testutil.AnthropicResponse("all done"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "json-tool-parent", `name = "json-tool-parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["json-helper"]
`)
	writeAgentConfig(t, configDir, "json-helper", `name = "json-helper"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "json-tool-parent", "--json"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	var result map[string]interface{}
	if jsonErr := json.Unmarshal(buf.Bytes(), &result); jsonErr != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\nraw: %q", jsonErr, buf.String())
	}

	if toolCalls, ok := result["tool_calls"].(float64); !ok || toolCalls != 1 {
		t.Errorf("expected tool_calls 1, got %v", result["tool_calls"])
	}

	detailsRaw, ok := result["tool_call_details"]
	if !ok {
		t.Fatalf("expected tool_call_details field to be present")
	}
	details, ok := detailsRaw.([]interface{})
	if !ok {
		t.Fatalf("expected tool_call_details to be array, got %T", detailsRaw)
	}
	if len(details) != 1 {
		t.Fatalf("expected tool_call_details length 1, got %d", len(details))
	}
	entry, ok := details[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected tool_call_details[0] to be object, got %T", details[0])
	}
	if name, ok := entry["name"].(string); !ok || name != "call_agent" {
		t.Fatalf("expected tool_call_details[0].name to be call_agent, got %v", entry["name"])
	}
	inputObj, ok := entry["input"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected tool_call_details[0].input to be object, got %T", entry["input"])
	}
	if _, ok := inputObj["agent"]; !ok {
		t.Fatalf("expected input.agent key in tool_call_details[0].input")
	}
	if _, ok := inputObj["task"]; !ok {
		t.Fatalf("expected input.task key in tool_call_details[0].input")
	}
	if isError, ok := entry["is_error"].(bool); !ok || isError {
		t.Fatalf("expected tool_call_details[0].is_error to be false, got %v", entry["is_error"])
	}
	// Assert turn is 0 (first and only tool call turn)
	if turnVal, ok := entry["turn"].(float64); !ok || turnVal != 0 {
		t.Fatalf("expected tool_call_details[0].turn to be 0, got %v", entry["turn"])
	}
	// Assert duration_ms is a non-negative number
	if durationVal, ok := entry["duration_ms"].(float64); !ok || durationVal < 0 {
		t.Fatalf("expected tool_call_details[0].duration_ms to be non-negative number, got %v", entry["duration_ms"])
	}
	if out, ok := entry["output"].(string); !ok || out == "" {
		t.Fatalf("expected tool_call_details[0].output to be non-empty string, got %v", entry["output"])
	}

	// Token sums: turn 1 (10+20) + turn 2 (10+5) = input 20, output 25
	if inputTokens, ok := result["input_tokens"].(float64); !ok || inputTokens != 20 {
		t.Errorf("expected input_tokens 20 (sum of 2 parent turns), got %v", result["input_tokens"])
	}

	if outputTokens, ok := result["output_tokens"].(float64); !ok || outputTokens != 25 {
		t.Errorf("expected output_tokens 25 (sum of 2 parent turns), got %v", result["output_tokens"])
	}
}

// --- Phase 16: Timeout Handling Test ---

func TestIntegration_Timeout_ContextDeadlineExceeded(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.SlowResponse(5 * time.Second),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "timeout-agent", `name = "timeout-agent"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "timeout-agent", "--timeout", "1"})

	err := rootCmd.Execute()
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 3 {
		t.Errorf("expected exit code 3, got %d", exitErr.Code)
	}
}

// --- Error Mapping Tests (Phase 17) ---

func TestIntegration_ErrorMapping_Auth401(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicErrorResponse(401, "authentication_error", "invalid api key"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "err-auth", `name = "err-auth"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "err-auth"})

	err := rootCmd.Execute()
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 3 {
		t.Errorf("expected exit code 3, got %d", exitErr.Code)
	}
}

func TestIntegration_ErrorMapping_RateLimit429(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicErrorResponse(429, "rate_limit_error", "too many requests"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "err-rate", `name = "err-rate"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "err-rate"})

	err := rootCmd.Execute()
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 3 {
		t.Errorf("expected exit code 3, got %d", exitErr.Code)
	}
}

func TestIntegration_ErrorMapping_Server500(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicErrorResponse(500, "server_error", "internal error"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "err-server", `name = "err-server"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "err-server"})

	err := rootCmd.Execute()
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 3 {
		t.Errorf("expected exit code 3, got %d", exitErr.Code)
	}
}

func TestIntegration_ErrorMapping_OpenAI401(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.OpenAIErrorResponse(401, "invalid_api_key", "invalid api key"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "err-oai-auth", `name = "err-oai-auth"
model = "openai/gpt-4o"
`)

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("AXE_OPENAI_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "err-oai-auth"})

	err := rootCmd.Execute()
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 3 {
		t.Errorf("expected exit code 3, got %d", exitErr.Code)
	}
}

func TestIntegration_ErrorMapping_OpenAI429(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.OpenAIErrorResponse(429, "rate_limit_exceeded", "rate limit"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "err-oai-rate", `name = "err-oai-rate"
model = "openai/gpt-4o"
`)

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("AXE_OPENAI_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "err-oai-rate"})

	err := rootCmd.Execute()
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 3 {
		t.Errorf("expected exit code 3, got %d", exitErr.Code)
	}
}

func TestIntegration_ErrorMapping_OpenAI500(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.OpenAIErrorResponse(500, "server_error", "internal error"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "err-oai-server", `name = "err-oai-server"
model = "openai/gpt-4o"
`)

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("AXE_OPENAI_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "err-oai-server"})

	err := rootCmd.Execute()
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 3 {
		t.Errorf("expected exit code 3, got %d", exitErr.Code)
	}
}

// --- Phase 18: Tool Integration Tests ---

func TestIntegration_Tools_ReadOnly(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		// Turn 1: LLM requests read_file
		testutil.AnthropicToolUseResponse("Let me read the file.", []testutil.MockToolCall{
			{ID: "tc_rf1", Name: "read_file", Input: map[string]string{"path": "hello.txt"}},
		}),
		// Turn 2: LLM receives tool result, returns final response
		testutil.AnthropicResponse("The file says: Hello, World!"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "ro-tools", `name = "ro-tools"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["read_file", "list_directory"]
`)

	// Create temp workdir with hello.txt
	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "hello.txt"), []byte("Hello, World!"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "ro-tools", "--workdir", workdir})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "The file says: Hello, World!") {
		t.Errorf("expected stdout to contain final response, got %q", output)
	}

	if mock.RequestCount() != 2 {
		t.Errorf("expected 2 requests, got %d", mock.RequestCount())
	}

	// First request should contain tool definitions for both read_file and list_directory
	body0 := mock.Requests[0].Body
	if !strings.Contains(body0, "read_file") {
		t.Errorf("expected first request to contain read_file tool definition, body: %s", body0)
	}
	if !strings.Contains(body0, "list_directory") {
		t.Errorf("expected first request to contain list_directory tool definition, body: %s", body0)
	}

	// Second request should contain the tool result with file content
	body1 := mock.Requests[1].Body
	if !strings.Contains(body1, "Hello, World!") {
		t.Errorf("expected second request to contain tool result with file content, body: %s", body1)
	}
}

func TestIntegration_Tools_Mutation(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		// Turn 1: LLM requests write_file
		testutil.AnthropicToolUseResponse("Writing the file.", []testutil.MockToolCall{
			{ID: "tc_wf1", Name: "write_file", Input: map[string]string{"path": "output.txt", "content": "first draft"}},
		}),
		// Turn 2: LLM requests edit_file
		testutil.AnthropicToolUseResponse("Editing the file.", []testutil.MockToolCall{
			{ID: "tc_ef1", Name: "edit_file", Input: map[string]string{"path": "output.txt", "old_string": "first", "new_string": "final"}},
		}),
		// Turn 3: final response
		testutil.AnthropicResponse("File updated successfully."),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "mut-tools", `name = "mut-tools"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["write_file", "edit_file"]
`)

	workdir := t.TempDir()

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "mut-tools", "--workdir", workdir})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	// Verify the file was created and edited correctly
	data, readErr := os.ReadFile(filepath.Join(workdir, "output.txt"))
	if readErr != nil {
		t.Fatalf("expected output.txt to exist: %v", readErr)
	}
	if string(data) != "final draft" {
		t.Errorf("expected file content %q, got %q", "final draft", string(data))
	}

	if mock.RequestCount() != 3 {
		t.Errorf("expected 3 requests, got %d", mock.RequestCount())
	}

	output := buf.String()
	if !strings.Contains(output, "File updated successfully.") {
		t.Errorf("expected stdout to contain final response, got %q", output)
	}

	// Verify write_file success message in second request
	body1 := mock.Requests[1].Body
	if !strings.Contains(body1, "wrote 11 bytes to output.txt") {
		t.Errorf("expected second request to contain write success message, body: %s", body1)
	}

	// Verify edit_file success message in third request
	body2 := mock.Requests[2].Body
	if !strings.Contains(body2, "replaced 1 occurrence(s) in output.txt") {
		t.Errorf("expected third request to contain edit success message, body: %s", body2)
	}
}

func TestIntegration_Tools_RunCommand(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		// Turn 1: LLM requests run_command
		testutil.AnthropicToolUseResponse("Running command.", []testutil.MockToolCall{
			{ID: "tc_rc1", Name: "run_command", Input: map[string]string{"command": "echo hello"}},
		}),
		// Turn 2: final response
		testutil.AnthropicResponse("Command executed."),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "rc-tools", `name = "rc-tools"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["run_command"]
`)

	workdir := t.TempDir()

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "rc-tools", "--workdir", workdir})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	if mock.RequestCount() != 2 {
		t.Errorf("expected 2 requests, got %d", mock.RequestCount())
	}

	// Second request should contain the command output
	body1 := mock.Requests[1].Body
	if !strings.Contains(body1, "hello") {
		t.Errorf("expected second request to contain command output 'hello', body: %s", body1)
	}

	output := buf.String()
	if !strings.Contains(output, "Command executed.") {
		t.Errorf("expected stdout to contain final response, got %q", output)
	}
}

func TestIntegration_Tools_MixedWithSubAgents_Sequential(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		// Turn 1: LLM requests read_file
		testutil.AnthropicToolUseResponse("Reading file first.", []testutil.MockToolCall{
			{ID: "tc_rf1", Name: "read_file", Input: map[string]string{"path": "data.txt"}},
		}),
		// Turn 2: LLM requests call_agent
		testutil.AnthropicToolUseResponse("Now calling helper.", []testutil.MockToolCall{
			{ID: "tc_ca1", Name: "call_agent", Input: map[string]string{"agent": "seq-helper", "task": "summarize this"}},
		}),
		// Sub-agent response
		testutil.AnthropicResponse("summary result"),
		// Turn 3: parent final
		testutil.AnthropicResponse("Done."),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "seq-mixed-parent", `name = "seq-mixed-parent"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["read_file"]
sub_agents = ["seq-helper"]
`)
	writeAgentConfig(t, configDir, "seq-helper", `name = "seq-helper"
model = "anthropic/claude-sonnet-4-20250514"
`)

	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "data.txt"), []byte("test data content"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "seq-mixed-parent", "--workdir", workdir})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	if mock.RequestCount() != 4 {
		t.Errorf("expected 4 requests, got %d", mock.RequestCount())
	}

	output := buf.String()
	if output != "Done." {
		t.Errorf("expected stdout %q, got %q", "Done.", output)
	}

	// First request should contain both read_file and call_agent tool definitions
	body0 := mock.Requests[0].Body
	if !strings.Contains(body0, "read_file") {
		t.Errorf("expected first request to contain read_file tool, body: %s", body0)
	}
	if !strings.Contains(body0, "call_agent") {
		t.Errorf("expected first request to contain call_agent tool, body: %s", body0)
	}

	// Second request should contain the read_file tool result
	body1 := mock.Requests[1].Body
	if !strings.Contains(body1, "test data content") {
		t.Errorf("expected second request to contain file content, body: %s", body1)
	}

	// Fourth request should contain the call_agent tool result
	body3 := mock.Requests[3].Body
	if !strings.Contains(body3, "summary result") {
		t.Errorf("expected fourth request to contain sub-agent result, body: %s", body3)
	}
}

func TestIntegration_Tools_MixedWithSubAgents_Parallel(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		// Turn 1: LLM requests both read_file and call_agent simultaneously
		testutil.AnthropicToolUseResponse("Doing both at once.", []testutil.MockToolCall{
			{ID: "tc_rf1", Name: "read_file", Input: map[string]string{"path": "info.txt"}},
			{ID: "tc_ca1", Name: "call_agent", Input: map[string]string{"agent": "par-helper", "task": "do work"}},
		}),
		// Sub-agent response (read_file completes synchronously, so sub-agent HTTP call is next)
		testutil.AnthropicResponse("sub-agent done"),
		// Turn 2: parent final (after both tool results)
		testutil.AnthropicResponse("All done."),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "par-mixed-parent", `name = "par-mixed-parent"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["read_file"]
sub_agents = ["par-helper"]
`)
	writeAgentConfig(t, configDir, "par-helper", `name = "par-helper"
model = "anthropic/claude-sonnet-4-20250514"
`)

	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "info.txt"), []byte("info content"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "par-mixed-parent", "--workdir", workdir})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	if mock.RequestCount() != 3 {
		t.Errorf("expected 3 requests, got %d", mock.RequestCount())
	}

	output := buf.String()
	if output != "All done." {
		t.Errorf("expected stdout %q, got %q", "All done.", output)
	}

	// Final request should contain both tool results
	body2 := mock.Requests[2].Body
	if !strings.Contains(body2, "info content") {
		t.Errorf("expected final request to contain read_file result, body: %s", body2)
	}
	if !strings.Contains(body2, "sub-agent done") {
		t.Errorf("expected final request to contain call_agent result, body: %s", body2)
	}
}

func TestIntegration_Tools_MultiTurnConversation(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		// Turn 1: list_directory
		testutil.AnthropicToolUseResponse("Listing directory.", []testutil.MockToolCall{
			{ID: "tc_ld1", Name: "list_directory", Input: map[string]string{"path": "."}},
		}),
		// Turn 2: read_file
		testutil.AnthropicToolUseResponse("Reading file.", []testutil.MockToolCall{
			{ID: "tc_rf1", Name: "read_file", Input: map[string]string{"path": "readme.txt"}},
		}),
		// Turn 3: run_command
		testutil.AnthropicToolUseResponse("Running command.", []testutil.MockToolCall{
			{ID: "tc_rc1", Name: "run_command", Input: map[string]string{"command": "echo done"}},
		}),
		// Turn 4: final response
		testutil.AnthropicResponse("Completed all three steps."),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "multi-turn-tools", `name = "multi-turn-tools"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["list_directory", "read_file", "run_command"]
`)

	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "readme.txt"), []byte("project readme"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "multi-turn-tools", "--workdir", workdir})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	if mock.RequestCount() != 4 {
		t.Errorf("expected 4 requests, got %d", mock.RequestCount())
	}

	output := buf.String()
	if output != "Completed all three steps." {
		t.Errorf("expected stdout %q, got %q", "Completed all three steps.", output)
	}

	// Turn 1 tool result in request 2 should contain directory listing with readme.txt
	body1 := mock.Requests[1].Body
	if !strings.Contains(body1, "readme.txt") {
		t.Errorf("expected request 2 to contain directory listing with readme.txt, body: %s", body1)
	}

	// Turn 2 tool result in request 3 should contain file content
	body2 := mock.Requests[2].Body
	if !strings.Contains(body2, "project readme") {
		t.Errorf("expected request 3 to contain file content 'project readme', body: %s", body2)
	}

	// Turn 3 tool result in request 4 should contain command output
	body3 := mock.Requests[3].Body
	if !strings.Contains(body3, "done") {
		t.Errorf("expected request 4 to contain command output 'done', body: %s", body3)
	}
}

func TestIntegration_JSONOutput_WithBuiltInToolCalls(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		// Turn 1: 2 tool calls
		testutil.AnthropicToolUseResponse("Reading files.", []testutil.MockToolCall{
			{ID: "tc_rf1", Name: "read_file", Input: map[string]string{"path": "a.txt"}},
			{ID: "tc_ld1", Name: "list_directory", Input: map[string]string{"path": "."}},
		}),
		// Turn 2: 1 tool call
		testutil.AnthropicToolUseResponse("Running command.", []testutil.MockToolCall{
			{ID: "tc_rc1", Name: "run_command", Input: map[string]string{"command": "echo final"}},
		}),
		// Turn 3: final
		testutil.AnthropicResponse("final"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "json-builtin-tools", `name = "json-builtin-tools"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["read_file", "list_directory", "run_command"]
`)

	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "a.txt"), []byte("file a"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "json-builtin-tools", "--json", "--workdir", workdir})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	var result map[string]interface{}
	if jsonErr := json.Unmarshal(buf.Bytes(), &result); jsonErr != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\nraw: %q", jsonErr, buf.String())
	}

	// tool_calls should be 3 (2 from turn 1 + 1 from turn 2)
	if toolCalls, ok := result["tool_calls"].(float64); !ok || toolCalls != 3 {
		t.Errorf("expected tool_calls 3, got %v", result["tool_calls"])
	}

	detailsRaw, ok := result["tool_call_details"]
	if !ok {
		t.Fatalf("expected tool_call_details field to be present")
	}
	details, ok := detailsRaw.([]interface{})
	if !ok {
		t.Fatalf("expected tool_call_details to be array, got %T", detailsRaw)
	}
	if len(details) != 3 {
		t.Fatalf("expected tool_call_details length 3, got %d", len(details))
	}
	wantOrder := []string{"read_file", "list_directory", "run_command"}
	for i, raw := range details {
		entry, ok := raw.(map[string]interface{})
		if !ok {
			t.Fatalf("expected tool_call_details[%d] to be object, got %T", i, raw)
		}
		for _, key := range []string{"name", "input", "output", "is_error", "turn", "duration_ms"} {
			if _, ok := entry[key]; !ok {
				t.Fatalf("expected tool_call_details[%d] to include key %q", i, key)
			}
		}
		if name, ok := entry["name"].(string); !ok || name != wantOrder[i] {
			t.Fatalf("expected tool_call_details[%d].name=%q, got %v", i, wantOrder[i], entry["name"])
		}
		if isError, ok := entry["is_error"].(bool); !ok || isError {
			t.Fatalf("expected tool_call_details[%d].is_error=false, got %v", i, entry["is_error"])
		}
		// Assert turn is a non-negative number (all 3 tool calls happen in turn 0 since they're sequential in one LLM response)
		if turnVal, ok := entry["turn"].(float64); !ok || turnVal < 0 {
			t.Fatalf("expected tool_call_details[%d].turn to be non-negative number, got %v", i, entry["turn"])
		}
		// Assert duration_ms is a non-negative number
		if durationVal, ok := entry["duration_ms"].(float64); !ok || durationVal < 0 {
			t.Fatalf("expected tool_call_details[%d].duration_ms to be non-negative number, got %v", i, entry["duration_ms"])
		}
	}

	if content, ok := result["content"].(string); !ok || content != "final" {
		t.Errorf("expected content %q, got %v", "final", result["content"])
	}

	if _, ok := result["model"]; !ok {
		t.Error("expected model field to be present")
	}

	// Token sums: 3 turns × Anthropic (tool_use: 10+20, tool_use: 10+20, response: 10+5)
	if inputTokens, ok := result["input_tokens"].(float64); !ok || inputTokens != 30 {
		t.Errorf("expected input_tokens 30, got %v", result["input_tokens"])
	}

	if outputTokens, ok := result["output_tokens"].(float64); !ok || outputTokens != 45 {
		t.Errorf("expected output_tokens 45, got %v", result["output_tokens"])
	}
}

func TestIntegration_Verbose_ToolExecution(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		// Turn 1: read_file tool call
		testutil.AnthropicToolUseResponse("Reading.", []testutil.MockToolCall{
			{ID: "tc_rf1", Name: "read_file", Input: map[string]string{"path": "test.txt"}},
		}),
		// Turn 2: final
		testutil.AnthropicResponse("done"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "verbose-tools", `name = "verbose-tools"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["read_file"]
`)

	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "test.txt"), []byte("line one\nline two"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "verbose-tools", "--verbose", "--workdir", workdir})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	output := buf.String()
	if output != "done" {
		t.Errorf("expected stdout %q, got %q", "done", output)
	}

	stderrStr := errBuf.String()
	if !strings.Contains(stderrStr, "[turn 1]") {
		t.Errorf("expected stderr to contain '[turn 1]', got: %s", stderrStr)
	}
	if !strings.Contains(stderrStr, "[tool] read_file:") {
		t.Errorf("expected stderr to contain '[tool] read_file:', got: %s", stderrStr)
	}
	if !strings.Contains(stderrStr, "(success)") {
		t.Errorf("expected stderr to contain '(success)', got: %s", stderrStr)
	}
	if !strings.Contains(stderrStr, "Duration:") {
		t.Errorf("expected stderr to contain 'Duration:', got: %s", stderrStr)
	}
}

func TestIntegration_JSONOutput_ToolCallDetails_Truncation(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Reading big file.", []testutil.MockToolCall{
			{ID: "tc_big", Name: "read_file", Input: map[string]string{"path": "big.txt"}},
		}),
		testutil.AnthropicResponse("done"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "json-truncate", `name = "json-truncate"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["read_file"]
`)

	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "big.txt"), []byte(strings.Repeat("a", 2048)), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "json-truncate", "--json", "--workdir", workdir})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	var result map[string]interface{}
	if jsonErr := json.Unmarshal(buf.Bytes(), &result); jsonErr != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\nraw: %q", jsonErr, buf.String())
	}

	details := result["tool_call_details"].([]interface{})
	entry := details[0].(map[string]interface{})
	output := entry["output"].(string)
	if !strings.HasSuffix(output, "... (truncated)") {
		t.Fatalf("expected output to end with truncation suffix, got %q", output)
	}
	if len(output) != 1039 {
		t.Fatalf("expected output length 1039, got %d", len(output))
	}
}

func TestIntegration_JSONOutput_ToolCallDetails_Error(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Reading escaped path.", []testutil.MockToolCall{
			{ID: "tc_err", Name: "read_file", Input: map[string]string{"path": "../escape.txt"}},
		}),
		testutil.AnthropicResponse("done"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "json-error", `name = "json-error"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["read_file"]
`)

	workdir := t.TempDir()
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "json-error", "--json", "--workdir", workdir})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	var result map[string]interface{}
	if jsonErr := json.Unmarshal(buf.Bytes(), &result); jsonErr != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\nraw: %q", jsonErr, buf.String())
	}

	details := result["tool_call_details"].([]interface{})
	entry := details[0].(map[string]interface{})
	if name, ok := entry["name"].(string); !ok || name != "read_file" {
		t.Fatalf("expected name read_file, got %v", entry["name"])
	}
	if isError, ok := entry["is_error"].(bool); !ok || !isError {
		t.Fatalf("expected is_error=true, got %v", entry["is_error"])
	}
	if output, ok := entry["output"].(string); !ok || strings.TrimSpace(output) == "" {
		t.Fatalf("expected non-empty error output, got %v", entry["output"])
	}
}

func TestIntegration_DryRun_ShowsTools(t *testing.T) {
	resetRunCmd(t)

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "dryrun-tools", `name = "dryrun-tools"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["write_file", "run_command"]
`)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "dryrun-tools", "--dry-run"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "--- Tools ---") {
		t.Errorf("expected stdout to contain '--- Tools ---', got: %s", output)
	}
	if !strings.Contains(output, "write_file, run_command") {
		t.Errorf("expected stdout to contain 'write_file, run_command', got: %s", output)
	}
	if strings.Contains(output, "read_file") {
		t.Errorf("expected stdout to NOT contain 'read_file', got: %s", output)
	}
	if strings.Contains(output, "list_directory") {
		t.Errorf("expected stdout to NOT contain 'list_directory', got: %s", output)
	}
}

func TestIntegration_JSONOutput_IncludesRetryAttempts(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicResponse("retry test response"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "retry-json", `name = "retry-json"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "retry-json", "--json"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, buf.String())
	}

	retryAttempts, ok := result["retry_attempts"]
	if !ok {
		t.Fatal("JSON output missing 'retry_attempts' field")
	}
	// JSON numbers are float64
	if retryAttempts.(float64) != 0 {
		t.Errorf("expected retry_attempts=0, got %v", retryAttempts)
	}
}

func TestIntegration_RetrySuccess_After429(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicErrorResponse(429, "rate_limit_error", "too many requests"),
		testutil.AnthropicResponse("success after retry"),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "retry-429", `name = "retry-429"
model = "anthropic/claude-sonnet-4-20250514"

[retry]
max_retries = 2
backoff = "fixed"
initial_delay_ms = 1
max_delay_ms = 1
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "retry-429", "--json"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, buf.String())
	}

	// Should have retried once
	retryAttempts, ok := result["retry_attempts"]
	if !ok {
		t.Fatal("JSON output missing 'retry_attempts' field")
	}
	if retryAttempts.(float64) != 1 {
		t.Errorf("expected retry_attempts=1, got %v", retryAttempts)
	}

	// Content should be from the successful response
	if result["content"] != "success after retry" {
		t.Errorf("expected content 'success after retry', got %q", result["content"])
	}

	// Mock should have received exactly 2 requests
	if mock.RequestCount() != 2 {
		t.Errorf("expected 2 requests to mock server, got %d", mock.RequestCount())
	}
}

// --- Allowlist Integration Tests ---

func TestIntegration_AllowedHosts_BlocksNonMatchingHost(t *testing.T) {
	resetRunCmd(t)

	// Turn 1: LLM requests url_fetch to a host NOT in allowed_hosts
	// Turn 2: LLM receives the error and responds with text
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Let me fetch that URL.", []testutil.MockToolCall{
			{ID: "tc_uf1", Name: "url_fetch", Input: map[string]string{"url": "https://blocked.example.com/data"}},
		}),
		testutil.AnthropicResponse("The URL was blocked by the allowlist."),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "fetch-blocked", `name = "fetch-blocked"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["url_fetch"]
allowed_hosts = ["api.example.com"]
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"run", "fetch-blocked"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the mock received 2 requests (tool call + final response)
	if mock.RequestCount() != 2 {
		t.Fatalf("expected 2 requests, got %d", mock.RequestCount())
	}

	// The second request should contain the tool error result
	body := mock.Requests[1].Body
	if !strings.Contains(body, "not in allowed_hosts") {
		t.Errorf("expected tool result to contain 'not in allowed_hosts', got request body:\n%s", body)
	}
}

func TestIntegration_AllowedHosts_BlocksPrivateIP(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Let me check the metadata.", []testutil.MockToolCall{
			{ID: "tc_uf2", Name: "url_fetch", Input: map[string]string{"url": "http://169.254.169.254/latest/meta-data/"}},
		}),
		testutil.AnthropicResponse("The request was blocked."),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	// No allowed_hosts — private IP blocking is unconditional
	writeAgentConfig(t, configDir, "fetch-private", `name = "fetch-private"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["url_fetch"]
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"run", "fetch-private"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.RequestCount() != 2 {
		t.Fatalf("expected 2 requests, got %d", mock.RequestCount())
	}

	// The second request should contain the private IP error
	body := mock.Requests[1].Body
	if !strings.Contains(body, "private/reserved IP") {
		t.Errorf("expected tool result to mention 'private/reserved IP', got request body:\n%s", body)
	}
}

func TestIntegration_AllowedHosts_EmptyAllowlistPermitsPublicHosts(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Let me fetch that.", []testutil.MockToolCall{
			{ID: "tc_uf3", Name: "url_fetch", Input: map[string]string{"url": "https://test.invalid/data"}},
		}),
		testutil.AnthropicResponse("Got a DNS error, as expected."),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "fetch-open", `name = "fetch-open"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["url_fetch"]
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"run", "fetch-open"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.RequestCount() != 2 {
		t.Fatalf("expected 2 requests, got %d", mock.RequestCount())
	}

	body := mock.Requests[1].Body
	if strings.Contains(body, "not in allowed_hosts") {
		t.Errorf("empty allowlist should not produce 'not in allowed_hosts' error, got:\n%s", body)
	}
}

func TestIntegration_AllowedHosts_BlocksLoopbackEvenWithAllowlist(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Fetching localhost.", []testutil.MockToolCall{
			{ID: "tc_uf4", Name: "url_fetch", Input: map[string]string{"url": "http://127.0.0.1:8080/secret"}},
		}),
		testutil.AnthropicResponse("Blocked."),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "fetch-loopback", `name = "fetch-loopback"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["url_fetch"]
allowed_hosts = ["api.example.com"]
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"run", "fetch-loopback"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.RequestCount() != 2 {
		t.Fatalf("expected 2 requests, got %d", mock.RequestCount())
	}

	body := mock.Requests[1].Body
	// Should be blocked by allowlist check (raw IP doesn't match hostname "api.example.com")
	if !strings.Contains(body, "not in allowed_hosts") {
		t.Errorf("expected 'not in allowed_hosts' error for raw IP, got:\n%s", body)
	}
}

// --- Phase 19: Artifact Management Tests ---

func TestIntegration_Artifacts_WriteReadRoundTrip(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Writing artifact.", []testutil.MockToolCall{
			{ID: "tc_w1", Name: "write_file", Input: map[string]string{"path": "report.md", "content": "artifact content", "artifact": "true"}},
		}),
		testutil.AnthropicToolUseResponse("Reading artifact.", []testutil.MockToolCall{
			{ID: "tc_r1", Name: "read_file", Input: map[string]string{"path": "report.md", "artifact": "true"}},
		}),
		testutil.AnthropicResponse("Round-trip complete."),
	})

	artifactDir := t.TempDir()
	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "artifact-roundtrip", fmt.Sprintf(`name = "artifact-roundtrip"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["write_file", "read_file"]
[artifacts]
enabled = true
dir = %q
`, artifactDir))

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "artifact-roundtrip", "--json"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	var result map[string]interface{}
	if jsonErr := json.Unmarshal(buf.Bytes(), &result); jsonErr != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\nraw: %q", jsonErr, buf.String())
	}

	// Check "artifacts" key exists
	artifactsRaw, ok := result["artifacts"]
	if !ok {
		t.Fatalf("expected JSON output to contain 'artifacts' key")
	}

	artifacts, ok := artifactsRaw.([]interface{})
	if !ok {
		t.Fatalf("expected artifacts to be array, got %T", artifactsRaw)
	}

	if len(artifacts) < 1 {
		t.Errorf("expected at least 1 artifact entry, got %d", len(artifacts))
	}

	// Check artifact entry has correct path
	entry, ok := artifacts[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected artifact[0] to be object, got %T", artifacts[0])
	}

	if path, ok := entry["path"].(string); !ok || path != "report.md" {
		t.Errorf("expected artifact path 'report.md', got %v", entry["path"])
	}

	// Second LLM request body should contain "wrote" and "(artifact)"
	body1 := mock.Requests[1].Body
	if !strings.Contains(body1, "wrote") || !strings.Contains(body1, "(artifact)") {
		t.Errorf("expected second request body to contain 'wrote' and '(artifact)', got: %s", body1)
	}

	// Third LLM request body should contain "artifact content"
	body2 := mock.Requests[2].Body
	if !strings.Contains(body2, "artifact content") {
		t.Errorf("expected third request body to contain 'artifact content', got: %s", body2)
	}

	// Verify the artifact file was actually written to disk
	artifactFile := filepath.Join(artifactDir, "report.md")
	data, readErr := os.ReadFile(artifactFile)
	if readErr != nil {
		t.Fatalf("expected artifact file %q to exist on disk, got error: %v", artifactFile, readErr)
	}
	if !strings.Contains(string(data), "artifact content") {
		t.Errorf("expected artifact file content to contain %q, got %q", "artifact content", string(data))
	}
}

func TestIntegration_Artifacts_InactiveSystemToolError(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Trying artifact write.", []testutil.MockToolCall{
			{ID: "tc_err1", Name: "write_file", Input: map[string]string{"path": "test.md", "content": "hello", "artifact": "true"}},
		}),
		testutil.AnthropicResponse("Got the error, continuing."),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "inactive-artifact", `name = "inactive-artifact"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["write_file"]
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	// Clear artifact-related env vars
	t.Setenv("AXE_ARTIFACT_DIR", "")

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "inactive-artifact"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	// Output should contain the final response
	output := buf.String()
	if !strings.Contains(output, "Got the error, continuing.") {
		t.Errorf("expected output to contain 'Got the error, continuing.', got: %s", output)
	}

	// Second LLM request body should contain the error message
	body1 := mock.Requests[1].Body
	if !strings.Contains(body1, "artifact directory not configured for this agent") {
		t.Errorf("expected second request body to contain 'artifact directory not configured for this agent', got: %s", body1)
	}
}
