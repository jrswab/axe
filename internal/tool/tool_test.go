package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jrswab/axe/internal/config"
	"github.com/jrswab/axe/internal/provider"
)

// helper: set up a temp XDG config dir with an agents/ subdirectory
func setupToolTestAgentsDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	agentsDir := filepath.Join(tmpDir, "axe", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}
	return agentsDir
}

func writeToolTestAgent(t *testing.T, agentsDir, name, content string) {
	t.Helper()
	path := filepath.Join(agentsDir, name+".toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write agent file: %v", err)
	}
}

func TestCallAgentTool_Definition(t *testing.T) {
	tool := CallAgentTool([]string{"helper", "runner"})

	if tool.Name != CallAgentToolName {
		t.Errorf("Name = %q, want %q", tool.Name, CallAgentToolName)
	}
	if tool.Name != "call_agent" {
		t.Errorf("Name = %q, want %q", tool.Name, "call_agent")
	}

	// Description must contain available agent names
	if !strings.Contains(tool.Description, "helper") {
		t.Errorf("Description missing agent name 'helper': %q", tool.Description)
	}
	if !strings.Contains(tool.Description, "runner") {
		t.Errorf("Description missing agent name 'runner': %q", tool.Description)
	}

	// Must have exactly three parameters
	if len(tool.Parameters) != 3 {
		t.Fatalf("Parameters count = %d, want 3", len(tool.Parameters))
	}

	// Check "agent" parameter
	agentParam, ok := tool.Parameters["agent"]
	if !ok {
		t.Fatal("missing 'agent' parameter")
	}
	if agentParam.Type != "string" {
		t.Errorf("agent.Type = %q, want %q", agentParam.Type, "string")
	}
	if !agentParam.Required {
		t.Error("agent.Required = false, want true")
	}
	if !strings.Contains(agentParam.Description, "helper") {
		t.Errorf("agent.Description missing agent name 'helper': %q", agentParam.Description)
	}

	// Check "task" parameter
	taskParam, ok := tool.Parameters["task"]
	if !ok {
		t.Fatal("missing 'task' parameter")
	}
	if taskParam.Type != "string" {
		t.Errorf("task.Type = %q, want %q", taskParam.Type, "string")
	}
	if !taskParam.Required {
		t.Error("task.Required = false, want true")
	}

	// Check "context" parameter
	contextParam, ok := tool.Parameters["context"]
	if !ok {
		t.Fatal("missing 'context' parameter")
	}
	if contextParam.Type != "string" {
		t.Errorf("context.Type = %q, want %q", contextParam.Type, "string")
	}
	if contextParam.Required {
		t.Error("context.Required = true, want false")
	}
}

func TestCallAgentTool_EmptyAgents(t *testing.T) {
	tool := CallAgentTool([]string{})

	if tool.Name != CallAgentToolName {
		t.Errorf("Name = %q, want %q", tool.Name, CallAgentToolName)
	}

	// Must still have valid structure with 3 parameters
	if len(tool.Parameters) != 3 {
		t.Fatalf("Parameters count = %d, want 3", len(tool.Parameters))
	}

	if _, ok := tool.Parameters["agent"]; !ok {
		t.Error("missing 'agent' parameter")
	}
	if _, ok := tool.Parameters["task"]; !ok {
		t.Error("missing 'task' parameter")
	}
	if _, ok := tool.Parameters["context"]; !ok {
		t.Error("missing 'context' parameter")
	}
}

// --- Phase 7a: ExecuteCallAgent argument validation tests ---

func TestExecuteCallAgent_EmptyAgentName(t *testing.T) {
	call := provider.ToolCall{
		ID:        "test-1",
		Name:      CallAgentToolName,
		Arguments: map[string]string{"agent": "", "task": "do something"},
	}
	opts := ExecuteOptions{
		AllowedAgents: []string{"helper"},
		MaxDepth:      3,
		Depth:         0,
	}
	result := ExecuteCallAgent(context.Background(), call, opts)
	if !result.IsError {
		t.Fatal("expected IsError=true for empty agent name")
	}
	want := `call_agent error: "agent" argument is required`
	if result.Content != want {
		t.Errorf("Content = %q, want %q", result.Content, want)
	}
	if result.CallID != "test-1" {
		t.Errorf("CallID = %q, want %q", result.CallID, "test-1")
	}
}

func TestExecuteCallAgent_EmptyTask(t *testing.T) {
	call := provider.ToolCall{
		ID:        "test-2",
		Name:      CallAgentToolName,
		Arguments: map[string]string{"agent": "helper", "task": ""},
	}
	opts := ExecuteOptions{
		AllowedAgents: []string{"helper"},
		MaxDepth:      3,
		Depth:         0,
	}
	result := ExecuteCallAgent(context.Background(), call, opts)
	if !result.IsError {
		t.Fatal("expected IsError=true for empty task")
	}
	want := `call_agent error: "task" argument is required`
	if result.Content != want {
		t.Errorf("Content = %q, want %q", result.Content, want)
	}
}

func TestExecuteCallAgent_AgentNotAllowed(t *testing.T) {
	call := provider.ToolCall{
		ID:        "test-3",
		Name:      CallAgentToolName,
		Arguments: map[string]string{"agent": "unknown", "task": "do something"},
	}
	opts := ExecuteOptions{
		AllowedAgents: []string{"helper", "runner"},
		MaxDepth:      3,
		Depth:         0,
	}
	result := ExecuteCallAgent(context.Background(), call, opts)
	if !result.IsError {
		t.Fatal("expected IsError=true for unknown agent")
	}
	want := `call_agent error: agent "unknown" is not in this agent's sub_agents list`
	if result.Content != want {
		t.Errorf("Content = %q, want %q", result.Content, want)
	}
}

func TestExecuteCallAgent_DepthLimitReached(t *testing.T) {
	call := provider.ToolCall{
		ID:        "test-4",
		Name:      CallAgentToolName,
		Arguments: map[string]string{"agent": "helper", "task": "do something"},
	}
	opts := ExecuteOptions{
		AllowedAgents: []string{"helper"},
		MaxDepth:      3,
		Depth:         3,
	}
	result := ExecuteCallAgent(context.Background(), call, opts)
	if !result.IsError {
		t.Fatal("expected IsError=true for depth limit")
	}
	want := fmt.Sprintf("call_agent error: maximum sub-agent depth (%d) reached", 3)
	if result.Content != want {
		t.Errorf("Content = %q, want %q", result.Content, want)
	}
}

// --- Phase 7b: Sub-Agent Loading and Execution tests ---

func TestExecuteCallAgent_AgentNotFound(t *testing.T) {
	_ = setupToolTestAgentsDir(t)

	call := provider.ToolCall{
		ID:        "test-5",
		Name:      CallAgentToolName,
		Arguments: map[string]string{"agent": "nonexistent", "task": "do something"},
	}
	opts := ExecuteOptions{
		AllowedAgents: []string{"nonexistent"},
		MaxDepth:      3,
		Depth:         0,
	}
	result := ExecuteCallAgent(context.Background(), call, opts)
	if !result.IsError {
		t.Fatal("expected IsError=true for missing agent")
	}
	if !strings.Contains(result.Content, "failed to load agent") {
		t.Errorf("Content = %q, want to contain 'failed to load agent'", result.Content)
	}
	if result.CallID != "test-5" {
		t.Errorf("CallID = %q, want %q", result.CallID, "test-5")
	}
}

func TestExecuteCallAgent_Success(t *testing.T) {
	agentsDir := setupToolTestAgentsDir(t)

	// Start mock provider server that returns a simple text response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"id":    "msg_123",
			"type":  "message",
			"model": "claude-sonnet-4-20250514",
			"role":  "assistant",
			"content": []map[string]interface{}{
				{"type": "text", "text": "Sub-agent result here"},
			},
			"stop_reason": "end_turn",
			"usage": map[string]int{
				"input_tokens":  10,
				"output_tokens": 5,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Write sub-agent TOML pointing at our test server
	toml := fmt.Sprintf(`name = "helper"
model = "anthropic/claude-sonnet-4-20250514"
system_prompt = "You are a helper."
`)
	writeToolTestAgent(t, agentsDir, "helper", toml)

	// Set API key env var for anthropic
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	// Set base URL to our test server
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	call := provider.ToolCall{
		ID:        "test-6",
		Name:      CallAgentToolName,
		Arguments: map[string]string{"agent": "helper", "task": "say hello"},
	}
	opts := ExecuteOptions{
		AllowedAgents: []string{"helper"},
		MaxDepth:      3,
		Depth:         0,
		GlobalConfig:  &config.GlobalConfig{},
	}
	result := ExecuteCallAgent(context.Background(), call, opts)
	if result.IsError {
		t.Fatalf("expected IsError=false, got error: %s", result.Content)
	}
	if result.Content != "Sub-agent result here" {
		t.Errorf("Content = %q, want %q", result.Content, "Sub-agent result here")
	}
	if result.CallID != "test-6" {
		t.Errorf("CallID = %q, want %q", result.CallID, "test-6")
	}
}

func TestExecuteCallAgent_WithContext(t *testing.T) {
	agentsDir := setupToolTestAgentsDir(t)

	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		resp := map[string]interface{}{
			"id":    "msg_123",
			"type":  "message",
			"model": "claude-sonnet-4-20250514",
			"role":  "assistant",
			"content": []map[string]interface{}{
				{"type": "text", "text": "done"},
			},
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 10, "output_tokens": 5},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	writeToolTestAgent(t, agentsDir, "helper", `name = "helper"`+"\n"+`model = "anthropic/claude-sonnet-4-20250514"`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	call := provider.ToolCall{
		ID:   "test-7",
		Name: CallAgentToolName,
		Arguments: map[string]string{
			"agent":   "helper",
			"task":    "analyze code",
			"context": "The code is in main.go",
		},
	}
	opts := ExecuteOptions{
		AllowedAgents: []string{"helper"},
		MaxDepth:      3,
		Depth:         0,
		GlobalConfig:  &config.GlobalConfig{},
	}
	result := ExecuteCallAgent(context.Background(), call, opts)
	if result.IsError {
		t.Fatalf("expected IsError=false, got error: %s", result.Content)
	}

	// Verify the user message contains both task and context
	messages, ok := receivedBody["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		t.Fatal("no messages in request body")
	}
	firstMsg, ok := messages[0].(map[string]interface{})
	if !ok {
		t.Fatal("first message is not a map")
	}
	content, _ := firstMsg["content"].(string)
	if !strings.Contains(content, "Task: analyze code") {
		t.Errorf("user message missing task: %q", content)
	}
	if !strings.Contains(content, "Context:\nThe code is in main.go") {
		t.Errorf("user message missing context: %q", content)
	}
}

func TestExecuteCallAgent_WithoutContext(t *testing.T) {
	agentsDir := setupToolTestAgentsDir(t)

	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		resp := map[string]interface{}{
			"id":    "msg_123",
			"type":  "message",
			"model": "claude-sonnet-4-20250514",
			"role":  "assistant",
			"content": []map[string]interface{}{
				{"type": "text", "text": "done"},
			},
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 10, "output_tokens": 5},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	writeToolTestAgent(t, agentsDir, "helper", `name = "helper"`+"\n"+`model = "anthropic/claude-sonnet-4-20250514"`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	call := provider.ToolCall{
		ID:        "test-8",
		Name:      CallAgentToolName,
		Arguments: map[string]string{"agent": "helper", "task": "analyze code"},
	}
	opts := ExecuteOptions{
		AllowedAgents: []string{"helper"},
		MaxDepth:      3,
		Depth:         0,
		GlobalConfig:  &config.GlobalConfig{},
	}
	result := ExecuteCallAgent(context.Background(), call, opts)
	if result.IsError {
		t.Fatalf("expected IsError=false, got error: %s", result.Content)
	}

	// Verify the user message contains only task (no context section)
	messages, ok := receivedBody["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		t.Fatal("no messages in request body")
	}
	firstMsg, ok := messages[0].(map[string]interface{})
	if !ok {
		t.Fatal("first message is not a map")
	}
	content, _ := firstMsg["content"].(string)
	if !strings.Contains(content, "Task: analyze code") {
		t.Errorf("user message missing task: %q", content)
	}
	if strings.Contains(content, "Context:") {
		t.Errorf("user message should not contain Context section: %q", content)
	}
}

// --- Phase 7c: Error Handling tests ---

func TestExecuteCallAgent_APIError(t *testing.T) {
	agentsDir := setupToolTestAgentsDir(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"type":"error","error":{"type":"server_error","message":"Internal server error"}}`))
	}))
	defer server.Close()

	writeToolTestAgent(t, agentsDir, "helper", `name = "helper"`+"\n"+`model = "anthropic/claude-sonnet-4-20250514"`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	call := provider.ToolCall{
		ID:        "test-9",
		Name:      CallAgentToolName,
		Arguments: map[string]string{"agent": "helper", "task": "do something"},
	}
	opts := ExecuteOptions{
		AllowedAgents: []string{"helper"},
		MaxDepth:      3,
		Depth:         0,
		GlobalConfig:  &config.GlobalConfig{},
	}
	result := ExecuteCallAgent(context.Background(), call, opts)
	if !result.IsError {
		t.Fatal("expected IsError=true for API error")
	}
	if !strings.Contains(result.Content, "sub-agent") {
		t.Errorf("Content = %q, want to contain 'sub-agent'", result.Content)
	}
	if !strings.Contains(result.Content, "You may retry or proceed without this result") {
		t.Errorf("Content missing retry suggestion: %q", result.Content)
	}
}

// TestExecuteCallAgent_DepthLimitNoTools verifies that a sub-agent at the depth limit
// runs without tools injected, even when the sub-agent has sub_agents configured.
// This tests Req 10.3: tools are only injected when newDepth < MaxDepth.
func TestExecuteCallAgent_DepthLimitNoTools(t *testing.T) {
	agentsDir := setupToolTestAgentsDir(t)

	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		receivedBody = string(bodyBytes)
		resp := map[string]interface{}{
			"id":    "msg_depth",
			"type":  "message",
			"model": "claude-sonnet-4-20250514",
			"role":  "assistant",
			"content": []map[string]interface{}{
				{"type": "text", "text": "depth-limited result"},
			},
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 10, "output_tokens": 5},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Sub-agent has sub_agents configured, but depth should prevent tool injection
	writeToolTestAgent(t, agentsDir, "deep-helper", `name = "deep-helper"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["another-agent"]
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	call := provider.ToolCall{
		ID:        "test-depth-tools",
		Name:      CallAgentToolName,
		Arguments: map[string]string{"agent": "deep-helper", "task": "do something"},
	}

	// Depth=2, MaxDepth=3: newDepth will be 3, which is NOT < 3, so no tools should be injected
	opts := ExecuteOptions{
		AllowedAgents: []string{"deep-helper"},
		MaxDepth:      3,
		Depth:         2,
		GlobalConfig:  &config.GlobalConfig{},
	}
	result := ExecuteCallAgent(context.Background(), call, opts)
	if result.IsError {
		t.Fatalf("expected IsError=false, got error: %s", result.Content)
	}
	if result.Content != "depth-limited result" {
		t.Errorf("Content = %q, want %q", result.Content, "depth-limited result")
	}

	// The request sent to the mock server should NOT contain tools
	if strings.Contains(receivedBody, `"tools"`) {
		t.Errorf("expected no 'tools' in request body when at depth limit, but found tools in: %s", receivedBody)
	}
}

func TestExecuteCallAgent_Timeout(t *testing.T) {
	agentsDir := setupToolTestAgentsDir(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		resp := map[string]interface{}{
			"id":          "msg_123",
			"type":        "message",
			"model":       "claude-sonnet-4-20250514",
			"role":        "assistant",
			"content":     []map[string]interface{}{{"type": "text", "text": "done"}},
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 10, "output_tokens": 5},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	writeToolTestAgent(t, agentsDir, "helper", `name = "helper"`+"\n"+`model = "anthropic/claude-sonnet-4-20250514"`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	call := provider.ToolCall{
		ID:        "test-10",
		Name:      CallAgentToolName,
		Arguments: map[string]string{"agent": "helper", "task": "do something"},
	}
	opts := ExecuteOptions{
		AllowedAgents: []string{"helper"},
		MaxDepth:      3,
		Depth:         0,
		Timeout:       1, // 1 second timeout
		GlobalConfig:  &config.GlobalConfig{},
	}
	result := ExecuteCallAgent(context.Background(), call, opts)
	if !result.IsError {
		t.Fatal("expected IsError=true for timeout")
	}
	if !strings.Contains(result.Content, "sub-agent") {
		t.Errorf("Content = %q, want to contain 'sub-agent'", result.Content)
	}
}
