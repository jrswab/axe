package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// resetGCCmd resets all gc command flags between tests.
func resetGCCmd(t *testing.T) {
	t.Helper()
	gcCmd.Flags().Set("dry-run", "false")
	gcCmd.Flags().Set("all", "false")
	gcCmd.Flags().Set("model", "")
}

// --- Phase 2a: Argument Validation ---

func TestGC_NoArgsNoAll(t *testing.T) {
	resetGCCmd(t)
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"gc"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing args, got nil")
	}
	if !strings.Contains(err.Error(), "agent name is required (or use --all)") {
		t.Errorf("expected 'agent name is required' error, got: %v", err)
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 1 {
		t.Errorf("expected exit code 1, got %d", exitErr.Code)
	}
}

func TestGC_AllWithAgentName(t *testing.T) {
	resetGCCmd(t)
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"gc", "myagent", "--all"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for --all with agent name, got nil")
	}
	if !strings.Contains(err.Error(), "cannot specify both --all and an agent name") {
		t.Errorf("expected 'cannot specify both --all and an agent name' error, got: %v", err)
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 1 {
		t.Errorf("expected exit code 1, got %d", exitErr.Code)
	}
}

// helper: create a temp XDG config dir with an agent TOML file for gc tests.
func setupGCTestAgent(t *testing.T, name, tomlContent string) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	agentsDir := filepath.Join(tmpDir, "axe", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, name+".toml"), []byte(tomlContent), 0644); err != nil {
		t.Fatalf("failed to write agent file: %v", err)
	}
	return tmpDir
}

// helper: generate N memory entries as a string for gc tests.
func generateGCEntries(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "## 2026-01-%02dT00:00:00Z\n**Task:** task%d\n**Result:** result%d\n\n", i, i, i)
	}
	return b.String()
}

// helper: start a mock Anthropic server for gc tests that captures the request body.
func startGCMockServer(t *testing.T, capturedBody *string, mu *sync.Mutex, responseText string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := new(bytes.Buffer)
		body.ReadFrom(r.Body)
		mu.Lock()
		*capturedBody = body.String()
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{
			"id": "msg_gc",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "` + responseText + `"}],
			"model": "claude-sonnet-4-20250514",
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 10, "output_tokens": 5}
		}`))
	}))
}

// helper: start a mock server that returns HTTP 500 for gc tests.
func startGCMockServerError(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		w.Write([]byte(`{"type": "error", "error": {"type": "server_error", "message": "internal server error"}}`))
	}))
}

// helper: populate a memory file for a gc test agent.
func populateGCMemory(t *testing.T, tmpDir, agentName string, numEntries int) string {
	t.Helper()
	memoryDir := filepath.Join(tmpDir, "data", "axe", "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		t.Fatalf("failed to create memory dir: %v", err)
	}
	memPath := filepath.Join(memoryDir, agentName+".md")
	content := generateGCEntries(numEntries)
	if err := os.WriteFile(memPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write memory file: %v", err)
	}
	return memPath
}

// --- Phase 3a: Config Loading and Memory Check ---

func TestGC_AgentNotFound(t *testing.T) {
	resetGCCmd(t)
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	// Create the agents directory but don't create any agent files
	if err := os.MkdirAll(filepath.Join(tmpDir, "axe", "agents"), 0755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"gc", "nonexistent"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent agent, got nil")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 2 {
		t.Errorf("expected exit code 2, got %d", exitErr.Code)
	}
}

func TestGC_MemoryDisabled(t *testing.T) {
	resetGCCmd(t)
	setupGCTestAgent(t, "gc-memdisabled", `name = "gc-memdisabled"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = false
`)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"gc", "gc-memdisabled"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "Warning:") {
		t.Errorf("expected warning in stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "gc-memdisabled") {
		t.Errorf("expected agent name in stderr warning, got %q", stderr)
	}
	if !strings.Contains(stderr, "memory enabled") {
		t.Errorf("expected 'memory enabled' in stderr warning, got %q", stderr)
	}
}

func TestGC_NoMemoryFile(t *testing.T) {
	resetGCCmd(t)
	tmpDir := setupGCTestAgent(t, "gc-nomem", `name = "gc-nomem"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
`)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"gc", "gc-nomem"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	stdout := buf.String()
	if !strings.Contains(stdout, "No memory entries") {
		t.Errorf("expected 'No memory entries' in stdout, got %q", stdout)
	}
	if !strings.Contains(stdout, "gc-nomem") {
		t.Errorf("expected agent name in stdout, got %q", stdout)
	}
}

func TestGC_EmptyMemoryFile(t *testing.T) {
	resetGCCmd(t)
	tmpDir := setupGCTestAgent(t, "gc-emptymem", `name = "gc-emptymem"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
`)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	// Create an empty memory file
	memoryDir := filepath.Join(tmpDir, "data", "axe", "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		t.Fatalf("failed to create memory dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memoryDir, "gc-emptymem.md"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to write empty memory file: %v", err)
	}

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"gc", "gc-emptymem"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	stdout := buf.String()
	if !strings.Contains(stdout, "No memory entries") {
		t.Errorf("expected 'No memory entries' in stdout, got %q", stdout)
	}
	if !strings.Contains(stdout, "gc-emptymem") {
		t.Errorf("expected agent name in stdout, got %q", stdout)
	}
}

// --- Phase 4a: LLM Pattern Detection ---

func TestGC_PatternDetectionPrompt(t *testing.T) {
	resetGCCmd(t)
	var capturedBody string
	var mu sync.Mutex
	server := startGCMockServer(t, &capturedBody, &mu, "## Patterns Found\\nNo clear patterns detected.")
	defer server.Close()

	tmpDir := setupGCTestAgent(t, "gc-prompt", `name = "gc-prompt"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	populateGCMemory(t, tmpDir, "gc-prompt", 3)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"gc", "gc-prompt"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	body := capturedBody
	mu.Unlock()

	// Parse the JSON body to check system prompt, messages, temperature, max_tokens
	var reqBody map[string]interface{}
	if err := json.Unmarshal([]byte(body), &reqBody); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	// Verify system prompt contains the exact pattern detection prompt text
	system, ok := reqBody["system"].(string)
	if !ok {
		t.Fatal("expected 'system' field in request body")
	}
	if !strings.Contains(system, "You are a memory analyst for an AI agent") {
		t.Errorf("expected pattern detection prompt in system, got: %s", system)
	}
	if !strings.Contains(system, "## Patterns Found") {
		t.Errorf("expected '## Patterns Found' in system prompt, got: %s", system)
	}
	if !strings.Contains(system, "## Repeated Work") {
		t.Errorf("expected '## Repeated Work' in system prompt, got: %s", system)
	}
	if !strings.Contains(system, "## Recommendations") {
		t.Errorf("expected '## Recommendations' in system prompt, got: %s", system)
	}

	// Verify user message contains memory content
	messages, ok := reqBody["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		t.Fatal("expected non-empty 'messages' array in request body")
	}
	firstMsg, ok := messages[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected first message to be an object")
	}
	userContent, ok := firstMsg["content"].(string)
	if !ok {
		t.Fatal("expected user message content to be a string")
	}
	if !strings.Contains(userContent, "task1") || !strings.Contains(userContent, "task3") {
		t.Errorf("expected memory content in user message, got: %s", userContent)
	}

	// Verify temperature is 0.3
	temp, ok := reqBody["temperature"].(float64)
	if !ok {
		t.Fatal("expected 'temperature' field in request body")
	}
	if temp != 0.3 {
		t.Errorf("expected temperature 0.3, got %f", temp)
	}

	// Verify max_tokens is 4096
	maxTokens, ok := reqBody["max_tokens"].(float64)
	if !ok {
		t.Fatal("expected 'max_tokens' field in request body")
	}
	if int(maxTokens) != 4096 {
		t.Errorf("expected max_tokens 4096, got %d", int(maxTokens))
	}

	// Verify no tools
	if tools, ok := reqBody["tools"]; ok {
		if toolArr, ok := tools.([]interface{}); ok && len(toolArr) > 0 {
			t.Errorf("expected no tools, got %d", len(toolArr))
		}
	}

	// Verify stdout contains "--- Analysis ---"
	stdout := buf.String()
	if !strings.Contains(stdout, "--- Analysis ---") {
		t.Errorf("expected '--- Analysis ---' in stdout, got %q", stdout)
	}
}

func TestGC_LLMError(t *testing.T) {
	resetGCCmd(t)
	server := startGCMockServerError(t)
	defer server.Close()

	tmpDir := setupGCTestAgent(t, "gc-llmerr", `name = "gc-llmerr"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	memPath := populateGCMemory(t, tmpDir, "gc-llmerr", 5)

	// Read memory file before
	beforeData, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("failed to read memory file: %v", err)
	}

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"gc", "gc-llmerr"})

	err = rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for LLM failure, got nil")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 3 {
		t.Errorf("expected exit code 3, got %d", exitErr.Code)
	}

	// Verify memory file is unchanged
	afterData, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("failed to read memory file after: %v", err)
	}
	if string(afterData) != string(beforeData) {
		t.Errorf("memory file should be unchanged after LLM error")
	}
}

func TestGC_ModelOverride(t *testing.T) {
	resetGCCmd(t)
	var capturedBody string
	var mu sync.Mutex
	// We use an Anthropic mock server but will pass --model ollama/llama3.
	// However, since ollama uses a different request format, let's use anthropic for both.
	// The test just needs to verify the model name in the request.
	server := startGCMockServer(t, &capturedBody, &mu, "analysis result")
	defer server.Close()

	tmpDir := setupGCTestAgent(t, "gc-override", `name = "gc-override"
model = "anthropic/claude-3"

[memory]
enabled = true
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	populateGCMemory(t, tmpDir, "gc-override", 3)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"gc", "gc-override", "--model", "anthropic/claude-override-model"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	body := capturedBody
	mu.Unlock()

	// Parse the request body and verify the model
	var reqBody map[string]interface{}
	if err := json.Unmarshal([]byte(body), &reqBody); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	model, ok := reqBody["model"].(string)
	if !ok {
		t.Fatal("expected 'model' field in request body")
	}
	if model != "claude-override-model" {
		t.Errorf("expected model 'claude-override-model', got %q", model)
	}
	if model == "claude-3" {
		t.Errorf("model should be overridden, but still got agent's default model 'claude-3'")
	}
}

// --- Phase 5a: Dry-Run and Trim ---

func TestGC_DryRun(t *testing.T) {
	resetGCCmd(t)
	var capturedBody string
	var mu sync.Mutex
	server := startGCMockServer(t, &capturedBody, &mu, "dry run analysis")
	defer server.Close()

	tmpDir := setupGCTestAgent(t, "gc-dryrun", `name = "gc-dryrun"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
last_n = 3
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	memPath := populateGCMemory(t, tmpDir, "gc-dryrun", 10)

	// Read memory file before
	beforeData, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("failed to read memory file: %v", err)
	}

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"gc", "gc-dryrun", "--dry-run"})

	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	stdout := buf.String()
	// Verify analysis is printed
	if !strings.Contains(stdout, "--- Analysis ---") {
		t.Errorf("expected '--- Analysis ---' in stdout, got %q", stdout)
	}
	// Verify dry run message
	if !strings.Contains(stdout, "Dry run: no entries trimmed.") {
		t.Errorf("expected 'Dry run: no entries trimmed.' in stdout, got %q", stdout)
	}

	// Verify memory file is unchanged (still 10 entries)
	afterData, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("failed to read memory file after: %v", err)
	}
	if string(afterData) != string(beforeData) {
		t.Errorf("memory file should be unchanged after dry run")
	}
}

func TestGC_AnalyzeAndTrim(t *testing.T) {
	resetGCCmd(t)
	var capturedBody string
	var mu sync.Mutex
	server := startGCMockServer(t, &capturedBody, &mu, "trim analysis")
	defer server.Close()

	tmpDir := setupGCTestAgent(t, "gc-trim", `name = "gc-trim"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
last_n = 3
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	memPath := populateGCMemory(t, tmpDir, "gc-trim", 10)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"gc", "gc-trim"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	stdout := buf.String()
	// Verify analysis is printed
	if !strings.Contains(stdout, "--- Analysis ---") {
		t.Errorf("expected '--- Analysis ---' in stdout, got %q", stdout)
	}
	// Verify trim message
	if !strings.Contains(stdout, "Trimmed: 7 entries removed, 3 entries kept.") {
		t.Errorf("expected 'Trimmed: 7 entries removed, 3 entries kept.' in stdout, got %q", stdout)
	}

	// Verify the memory file now contains exactly 3 entries
	afterData, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("failed to read memory file after trim: %v", err)
	}
	// Count ## headers in the trimmed file
	entryCount := strings.Count(string(afterData), "## ")
	if entryCount != 3 {
		t.Errorf("expected 3 entries in trimmed file, got %d", entryCount)
	}
	// Verify the kept entries are the last 3 (entries 8, 9, 10)
	if !strings.Contains(string(afterData), "task8") {
		t.Errorf("expected entry task8 in trimmed file")
	}
	if !strings.Contains(string(afterData), "task10") {
		t.Errorf("expected entry task10 in trimmed file")
	}
	if strings.Contains(string(afterData), "task7") {
		t.Errorf("entry task7 should have been trimmed")
	}
}

func TestGC_NoTrimTarget(t *testing.T) {
	resetGCCmd(t)
	var capturedBody string
	var mu sync.Mutex
	server := startGCMockServer(t, &capturedBody, &mu, "no trim analysis")
	defer server.Close()

	tmpDir := setupGCTestAgent(t, "gc-notrim", `name = "gc-notrim"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
last_n = 0
max_entries = 0
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	memPath := populateGCMemory(t, tmpDir, "gc-notrim", 10)

	// Read memory file before
	beforeData, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("failed to read memory file: %v", err)
	}

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"gc", "gc-notrim"})

	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	stdout := buf.String()
	// Verify analysis is printed
	if !strings.Contains(stdout, "--- Analysis ---") {
		t.Errorf("expected '--- Analysis ---' in stdout, got %q", stdout)
	}
	// Verify no trim target message
	if !strings.Contains(stdout, "No trim target configured") {
		t.Errorf("expected 'No trim target configured' in stdout, got %q", stdout)
	}

	// Verify memory file is unchanged
	afterData, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("failed to read memory file after: %v", err)
	}
	if string(afterData) != string(beforeData) {
		t.Errorf("memory file should be unchanged when no trim target configured")
	}
}

func TestGC_FallbackToMaxEntries(t *testing.T) {
	resetGCCmd(t)
	var capturedBody string
	var mu sync.Mutex
	server := startGCMockServer(t, &capturedBody, &mu, "fallback analysis")
	defer server.Close()

	tmpDir := setupGCTestAgent(t, "gc-fallback", `name = "gc-fallback"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
last_n = 0
max_entries = 5
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	memPath := populateGCMemory(t, tmpDir, "gc-fallback", 10)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"gc", "gc-fallback"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	stdout := buf.String()
	// Verify trim message with fallback to max_entries
	if !strings.Contains(stdout, "Trimmed: 5 entries removed, 5 entries kept.") {
		t.Errorf("expected 'Trimmed: 5 entries removed, 5 entries kept.' in stdout, got %q", stdout)
	}

	// Verify the memory file now contains exactly 5 entries
	afterData, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("failed to read memory file after trim: %v", err)
	}
	entryCount := strings.Count(string(afterData), "## ")
	if entryCount != 5 {
		t.Errorf("expected 5 entries in trimmed file, got %d", entryCount)
	}
}

func TestGC_NothingToTrim(t *testing.T) {
	resetGCCmd(t)
	var capturedBody string
	var mu sync.Mutex
	server := startGCMockServer(t, &capturedBody, &mu, "nothing to trim analysis")
	defer server.Close()

	tmpDir := setupGCTestAgent(t, "gc-noop", `name = "gc-noop"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
last_n = 10
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	populateGCMemory(t, tmpDir, "gc-noop", 3)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"gc", "gc-noop"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	stdout := buf.String()
	// Verify "nothing to trim" message
	if !strings.Contains(stdout, "No trimming needed: 3 entries within limit (10).") {
		t.Errorf("expected 'No trimming needed: 3 entries within limit (10).' in stdout, got %q", stdout)
	}
}

// --- Phase 6a: All-Agents GC Flow ---

// helper: set up multiple agents in a single temp directory for --all tests.
// agents is a map of agent name to TOML content.
func setupGCMultipleAgents(t *testing.T, agents map[string]string) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	agentsDir := filepath.Join(tmpDir, "axe", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}
	for name, toml := range agents {
		if err := os.WriteFile(filepath.Join(agentsDir, name+".toml"), []byte(toml), 0644); err != nil {
			t.Fatalf("failed to write agent file %s: %v", name, err)
		}
	}
	return tmpDir
}

func TestGC_AllFlag_NoMemoryAgents(t *testing.T) {
	resetGCCmd(t)
	tmpDir := setupGCMultipleAgents(t, map[string]string{
		"agent-x": `name = "agent-x"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = false
`,
		"agent-y": `name = "agent-y"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = false
`,
	})
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"gc", "--all"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	stdout := buf.String()
	if !strings.Contains(stdout, "No agents with memory enabled.") {
		t.Errorf("expected 'No agents with memory enabled.' in stdout, got %q", stdout)
	}
}

func TestGC_AllFlag_MultipleAgents(t *testing.T) {
	resetGCCmd(t)
	var capturedBody string
	var mu sync.Mutex
	server := startGCMockServer(t, &capturedBody, &mu, "all agents analysis")
	defer server.Close()

	tmpDir := setupGCMultipleAgents(t, map[string]string{
		"agent-a": `name = "agent-a"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
last_n = 5
`,
		"agent-b": `name = "agent-b"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = false
`,
		"agent-c": `name = "agent-c"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
last_n = 3
`,
	})
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	populateGCMemory(t, tmpDir, "agent-a", 10)
	populateGCMemory(t, tmpDir, "agent-c", 5)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"gc", "--all"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	stdout := buf.String()
	// Verify separators for processed agents
	if !strings.Contains(stdout, "=== GC: agent-a ===") {
		t.Errorf("expected separator for agent-a, got %q", stdout)
	}
	if !strings.Contains(stdout, "=== GC: agent-c ===") {
		t.Errorf("expected separator for agent-c, got %q", stdout)
	}
	// Verify agent-b (memory disabled) is not processed
	if strings.Contains(stdout, "=== GC: agent-b ===") {
		t.Errorf("agent-b should not have a separator (memory disabled)")
	}
}

func TestGC_AllFlag_PartialFailure(t *testing.T) {
	resetGCCmd(t)
	var capturedBody string
	var mu sync.Mutex
	server := startGCMockServer(t, &capturedBody, &mu, "partial failure analysis")
	defer server.Close()

	tmpDir := setupGCMultipleAgents(t, map[string]string{
		"agent-ok": `name = "agent-ok"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
last_n = 5
`,
		"agent-bad": `name = "agent-bad"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
last_n = 3
`,
	})
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	// Populate agent-ok's memory normally
	populateGCMemory(t, tmpDir, "agent-ok", 10)

	// Create an unreadable memory file for agent-bad
	memoryDir := filepath.Join(tmpDir, "data", "axe", "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		t.Fatalf("failed to create memory dir: %v", err)
	}
	badMemPath := filepath.Join(memoryDir, "agent-bad.md")
	if err := os.WriteFile(badMemPath, []byte(generateGCEntries(5)), 0644); err != nil {
		t.Fatalf("failed to write bad memory file: %v", err)
	}
	// Make the file unreadable
	if err := os.Chmod(badMemPath, 0000); err != nil {
		t.Fatalf("failed to chmod memory file: %v", err)
	}
	t.Cleanup(func() { os.Chmod(badMemPath, 0644) })

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"gc", "--all"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for partial failure, got nil")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 1 {
		t.Errorf("expected exit code 1, got %d", exitErr.Code)
	}

	// Verify stderr has error for agent-bad
	stderr := errBuf.String()
	if !strings.Contains(stderr, "agent-bad") {
		t.Errorf("expected error mentioning agent-bad in stderr, got %q", stderr)
	}

	// Verify the summary message
	if !strings.Contains(err.Error(), "gc completed with errors") {
		t.Errorf("expected 'gc completed with errors' in error message, got %q", err.Error())
	}
}

func TestGC_AllFlag_WithDryRun(t *testing.T) {
	resetGCCmd(t)
	var capturedBody string
	var mu sync.Mutex
	server := startGCMockServer(t, &capturedBody, &mu, "dry run all analysis")
	defer server.Close()

	tmpDir := setupGCMultipleAgents(t, map[string]string{
		"agent-d": `name = "agent-d"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
last_n = 3
`,
		"agent-e": `name = "agent-e"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
last_n = 3
`,
	})
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	memPathD := populateGCMemory(t, tmpDir, "agent-d", 10)
	memPathE := populateGCMemory(t, tmpDir, "agent-e", 8)

	// Read memory files before
	beforeD, err := os.ReadFile(memPathD)
	if err != nil {
		t.Fatalf("failed to read memory file: %v", err)
	}
	beforeE, err := os.ReadFile(memPathE)
	if err != nil {
		t.Fatalf("failed to read memory file: %v", err)
	}

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"gc", "--all", "--dry-run"})

	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	stdout := buf.String()
	// Verify dry run message appears (at least once for each agent)
	dryRunCount := strings.Count(stdout, "Dry run: no entries trimmed.")
	if dryRunCount < 2 {
		t.Errorf("expected 'Dry run: no entries trimmed.' at least twice, got %d times in %q", dryRunCount, stdout)
	}

	// Verify memory files are unchanged
	afterD, err := os.ReadFile(memPathD)
	if err != nil {
		t.Fatalf("failed to read memory file after: %v", err)
	}
	if string(afterD) != string(beforeD) {
		t.Errorf("agent-d memory file should be unchanged after dry run")
	}
	afterE, err := os.ReadFile(memPathE)
	if err != nil {
		t.Fatalf("failed to read memory file after: %v", err)
	}
	if string(afterE) != string(beforeE) {
		t.Errorf("agent-e memory file should be unchanged after dry run")
	}
}
