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
	"time"
)

// resetRunCmd resets all run command flags and stdin to their defaults between tests.
func resetRunCmd(t *testing.T) {
	t.Helper()
	runCmd.Flags().Set("skill", "")
	runCmd.Flags().Set("workdir", "")
	runCmd.Flags().Set("model", "")
	runCmd.Flags().Set("timeout", "120")
	runCmd.Flags().Set("dry-run", "false")
	runCmd.Flags().Set("verbose", "false")
	runCmd.Flags().Set("json", "false")
	rootCmd.SetIn(os.Stdin)
}

// helper: create a temp XDG config dir with an agent TOML file.
func setupRunTestAgent(t *testing.T, name, toml string) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	agentsDir := filepath.Join(tmpDir, "axe", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, name+".toml"), []byte(toml), 0644); err != nil {
		t.Fatalf("failed to write agent file: %v", err)
	}
	return tmpDir
}

// helper: start a mock Anthropic API server returning a successful response.
func startMockAnthropicServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{
			"id": "msg_test",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "Hello from mock"}],
			"model": "claude-sonnet-4-20250514",
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 10, "output_tokens": 5}
		}`))
	}))
}

// --- Phase 11a: Command Registration and Flags ---

func TestRun_NoArgs(t *testing.T) {
	resetRunCmd(t)
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing args, got nil")
	}

	// Error should be about exact args, not "unknown command"
	if strings.Contains(err.Error(), "unknown command") {
		t.Errorf("run command not registered; got 'unknown command' error: %v", err)
	}
}

// --- Phase 11b: Model Parsing and Provider Validation ---

func TestRun_InvalidModelFormat(t *testing.T) {
	resetRunCmd(t)
	setupRunTestAgent(t, "badmodel", `name = "badmodel"
model = "noprefix"
`)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "badmodel"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid model format, got nil")
	}
	if !strings.Contains(err.Error(), "invalid model format") {
		t.Errorf("expected 'invalid model format' error, got: %v", err)
	}
}

func TestRun_UnsupportedProvider(t *testing.T) {
	resetRunCmd(t)
	setupRunTestAgent(t, "fakeprov-agent", `name = "fakeprov-agent"
model = "fakeprovider/some-model"
`)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "fakeprov-agent"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for unsupported provider, got nil")
	}
	if !strings.Contains(err.Error(), `unsupported provider "fakeprovider"`) {
		t.Errorf("expected 'unsupported provider' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "anthropic, openai, ollama") {
		t.Errorf("expected supported providers list, got: %v", err)
	}
}

// --- Phase 11c: Config Loading and Overrides ---

func TestRun_MissingAgent(t *testing.T) {
	resetRunCmd(t)
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	agentsDir := filepath.Join(tmpDir, "axe", "agents")
	os.MkdirAll(agentsDir, 0755)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "nonexistent"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing agent, got nil")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 2 {
		t.Errorf("expected exit code 2, got %d", exitErr.Code)
	}
}

func TestRun_MissingAPIKey(t *testing.T) {
	resetRunCmd(t)
	setupRunTestAgent(t, "valid-agent", `name = "valid-agent"
model = "anthropic/claude-sonnet-4-20250514"
`)
	t.Setenv("ANTHROPIC_API_KEY", "")

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "valid-agent"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing API key, got nil")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 3 {
		t.Errorf("expected exit code 3, got %d", exitErr.Code)
	}
	if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Errorf("expected error mentioning ANTHROPIC_API_KEY, got: %v", err)
	}
	if !strings.Contains(err.Error(), "config.toml") {
		t.Errorf("expected error mentioning config.toml hint, got: %v", err)
	}
}

// --- Phase 11d: Dry-Run Mode ---

func TestRun_DryRun(t *testing.T) {
	resetRunCmd(t)
	tmpDir := setupRunTestAgent(t, "dry-agent", `name = "dry-agent"
model = "anthropic/claude-sonnet-4-20250514"
system_prompt = "You are a test agent."
`)
	// Create a skill file
	skillDir := filepath.Join(tmpDir, "axe", "skills")
	os.MkdirAll(skillDir, 0755)
	skillPath := filepath.Join(skillDir, "test.md")
	os.WriteFile(skillPath, []byte("# Test Skill"), 0644)

	// Set the agent to use the skill (relative path from config dir)
	agentPath := filepath.Join(tmpDir, "axe", "agents", "dry-agent.toml")
	os.WriteFile(agentPath, []byte(`name = "dry-agent"
model = "anthropic/claude-sonnet-4-20250514"
system_prompt = "You are a test agent."
skill = "skills/test.md"
`), 0644)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "dry-agent", "--dry-run"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "=== Dry Run ===") {
		t.Error("dry run output missing header")
	}
	if !strings.Contains(output, "Model:") {
		t.Error("dry run output missing Model field")
	}
	if !strings.Contains(output, "--- System Prompt ---") {
		t.Error("dry run output missing system prompt section")
	}
	if !strings.Contains(output, "You are a test agent.") {
		t.Error("dry run output missing system prompt content")
	}
	if !strings.Contains(output, "--- Skill ---") {
		t.Error("dry run output missing skill section")
	}
	if !strings.Contains(output, "# Test Skill") {
		t.Error("dry run output missing skill content")
	}
}

func TestRun_DryRunNoFiles(t *testing.T) {
	resetRunCmd(t)
	setupRunTestAgent(t, "nofiles-agent", `name = "nofiles-agent"
model = "anthropic/claude-sonnet-4-20250514"
`)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "nofiles-agent", "--dry-run"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "(none)") {
		t.Error("dry run output should show (none) for empty files")
	}
}

// --- Phase 11e: LLM Call and Default Output ---

func TestRun_Success(t *testing.T) {
	resetRunCmd(t)
	server := startMockAnthropicServer(t)
	defer server.Close()

	setupRunTestAgent(t, "test-agent", `name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "test-agent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "Hello from mock") {
		t.Errorf("expected 'Hello from mock' in output, got %q", buf.String())
	}
}

func TestRun_StdinPiped(t *testing.T) {
	resetRunCmd(t)
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := new(bytes.Buffer)
		body.ReadFrom(r.Body)
		receivedBody = body.String()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{
			"id": "msg_test",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "response"}],
			"model": "claude-sonnet-4-20250514",
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 10, "output_tokens": 5}
		}`))
	}))
	defer server.Close()

	setupRunTestAgent(t, "stdin-agent", `name = "stdin-agent"
model = "anthropic/claude-sonnet-4-20250514"
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	stdinBuf := strings.NewReader("piped input content")
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetIn(stdinBuf)
	rootCmd.SetArgs([]string{"run", "stdin-agent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(receivedBody, "piped input content") {
		t.Errorf("expected piped stdin content in request body, got %q", receivedBody)
	}
}

func TestRun_ModelOverride(t *testing.T) {
	resetRunCmd(t)
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := new(bytes.Buffer)
		body.ReadFrom(r.Body)
		receivedBody = body.String()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{
			"id": "msg_test",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "ok"}],
			"model": "claude-haiku-3-20240307",
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 10, "output_tokens": 5}
		}`))
	}))
	defer server.Close()

	setupRunTestAgent(t, "override-agent", `name = "override-agent"
model = "anthropic/claude-sonnet-4-20250514"
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "override-agent", "--model", "anthropic/claude-haiku-3-20240307"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(receivedBody, "claude-haiku-3-20240307") {
		t.Errorf("expected overridden model in request body, got %q", receivedBody)
	}
}

func TestRun_SkillOverride(t *testing.T) {
	resetRunCmd(t)
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := new(bytes.Buffer)
		body.ReadFrom(r.Body)
		receivedBody = body.String()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{
			"id": "msg_test",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "ok"}],
			"model": "claude-sonnet-4-20250514",
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 10, "output_tokens": 5}
		}`))
	}))
	defer server.Close()

	setupRunTestAgent(t, "skill-override-agent", `name = "skill-override-agent"
model = "anthropic/claude-sonnet-4-20250514"
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	// Create a separate skill file
	skillDir := t.TempDir()
	skillFile := filepath.Join(skillDir, "override-skill.md")
	os.WriteFile(skillFile, []byte("# Override Skill Content"), 0644)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "skill-override-agent", "--skill", skillFile})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(receivedBody, "Override Skill Content") {
		t.Errorf("expected overridden skill content in request body, got %q", receivedBody)
	}
}

func TestRun_WorkdirOverride(t *testing.T) {
	resetRunCmd(t)
	setupRunTestAgent(t, "workdir-agent", `name = "workdir-agent"
model = "anthropic/claude-sonnet-4-20250514"
files = ["*.txt"]
`)

	// Create a separate workdir with a file
	workdir := t.TempDir()
	os.WriteFile(filepath.Join(workdir, "test.txt"), []byte("workdir file content"), 0644)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "workdir-agent", "--dry-run", "--workdir", workdir})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "test.txt") {
		t.Errorf("expected test.txt in dry-run output, got %q", output)
	}
}

// --- Phase 11f: JSON Output Mode ---

func TestRun_JSONOutput(t *testing.T) {
	resetRunCmd(t)
	server := startMockAnthropicServer(t)
	defer server.Close()

	setupRunTestAgent(t, "json-agent", `name = "json-agent"
model = "anthropic/claude-sonnet-4-20250514"
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "json-agent", "--json"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, buf.String())
	}

	// Check required fields
	for _, field := range []string{"model", "content", "input_tokens", "output_tokens", "stop_reason", "duration_ms"} {
		if _, ok := result[field]; !ok {
			t.Errorf("JSON output missing field %q", field)
		}
	}
	if result["content"] != "Hello from mock" {
		t.Errorf("expected content 'Hello from mock', got %q", result["content"])
	}
}

// --- Phase 11g: Verbose Output Mode ---

func TestRun_VerboseOutput(t *testing.T) {
	resetRunCmd(t)
	server := startMockAnthropicServer(t)
	defer server.Close()

	setupRunTestAgent(t, "verbose-agent", `name = "verbose-agent"
model = "anthropic/claude-sonnet-4-20250514"
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "verbose-agent", "--verbose"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Response should be on stdout
	if !strings.Contains(buf.String(), "Hello from mock") {
		t.Errorf("expected response on stdout, got %q", buf.String())
	}

	// Debug info should be on stderr
	stderr := errBuf.String()
	for _, field := range []string{"Model:", "Workdir:", "Skill:", "Files:", "Stdin:", "Timeout:", "Params:", "Duration:", "Tokens:", "Stop:"} {
		if !strings.Contains(stderr, field) {
			t.Errorf("verbose stderr missing %q\nfull stderr:\n%s", field, stderr)
		}
	}
}

// --- Phase 11h: Error Exit Code Mapping ---

func TestRun_TimeoutExceeded(t *testing.T) {
	resetRunCmd(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.WriteHeader(200)
	}))
	defer server.Close()

	setupRunTestAgent(t, "timeout-agent", `name = "timeout-agent"
model = "anthropic/claude-sonnet-4-20250514"
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "timeout-agent", "--timeout", "1"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for timeout, got nil")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 3 {
		t.Errorf("expected exit code 3, got %d", exitErr.Code)
	}
}

func TestRun_APIError(t *testing.T) {
	resetRunCmd(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		w.Write([]byte(`{"type": "error", "error": {"type": "server_error", "message": "Internal server error"}}`))
	}))
	defer server.Close()

	setupRunTestAgent(t, "error-agent", `name = "error-agent"
model = "anthropic/claude-sonnet-4-20250514"
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "error-agent"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for server error, got nil")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 3 {
		t.Errorf("expected exit code 3, got %d", exitErr.Code)
	}
}

// --- M4: Multi-Provider Tests ---

func startMockOpenAIServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"model": "gpt-4o",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "Hello from OpenAI mock"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5},
		})
	}))
}

func startMockOllamaServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"model":             "llama3",
			"message":           map[string]string{"content": "Hello from Ollama mock"},
			"done_reason":       "stop",
			"prompt_eval_count": 8,
			"eval_count":        12,
		})
	}))
}

func TestRun_OpenAIProviderSuccess(t *testing.T) {
	resetRunCmd(t)
	server := startMockOpenAIServer(t)
	defer server.Close()

	setupRunTestAgent(t, "openai-agent", `name = "openai-agent"
model = "openai/gpt-4o"
`)
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("AXE_OPENAI_BASE_URL", server.URL)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "openai-agent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "Hello from OpenAI mock") {
		t.Errorf("expected 'Hello from OpenAI mock', got %q", buf.String())
	}
}

func TestRun_OllamaProviderSuccess(t *testing.T) {
	resetRunCmd(t)
	server := startMockOllamaServer(t)
	defer server.Close()

	setupRunTestAgent(t, "ollama-agent", `name = "ollama-agent"
model = "ollama/llama3"
`)
	t.Setenv("AXE_OLLAMA_BASE_URL", server.URL)
	// Ensure no API key is needed
	t.Setenv("OLLAMA_API_KEY", "")

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "ollama-agent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "Hello from Ollama mock") {
		t.Errorf("expected 'Hello from Ollama mock', got %q", buf.String())
	}
}

func TestRun_MissingAPIKeyOpenAI(t *testing.T) {
	resetRunCmd(t)
	setupRunTestAgent(t, "openai-nokey", `name = "openai-nokey"
model = "openai/gpt-4o"
`)
	t.Setenv("OPENAI_API_KEY", "")

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "openai-nokey"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing API key")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T", err)
	}
	if exitErr.Code != 3 {
		t.Errorf("expected exit code 3, got %d", exitErr.Code)
	}
}

func TestRun_OllamaNoAPIKeyRequired(t *testing.T) {
	resetRunCmd(t)
	server := startMockOllamaServer(t)
	defer server.Close()

	setupRunTestAgent(t, "ollama-nokey", `name = "ollama-nokey"
model = "ollama/llama3"
`)
	t.Setenv("AXE_OLLAMA_BASE_URL", server.URL)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "ollama-nokey"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_APIKeyFromConfigFile(t *testing.T) {
	resetRunCmd(t)
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("x-api-key")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "msg_test", "type": "message", "role": "assistant",
			"content": [{"type": "text", "text": "ok"}],
			"model": "claude-sonnet-4-20250514", "stop_reason": "end_turn",
			"usage": {"input_tokens": 10, "output_tokens": 5}
		}`))
	}))
	defer server.Close()

	tmpDir := setupRunTestAgent(t, "config-key-agent", `name = "config-key-agent"
model = "anthropic/claude-sonnet-4-20250514"
`)
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	// Write config.toml with API key
	configDir := filepath.Join(tmpDir, "axe")
	os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(`
[providers.anthropic]
api_key = "from-config-file"
`), 0644)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "config-key-agent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedAuth != "from-config-file" {
		t.Errorf("expected API key 'from-config-file' in request, got %q", receivedAuth)
	}
}

func TestRun_MalformedGlobalConfig(t *testing.T) {
	resetRunCmd(t)
	tmpDir := setupRunTestAgent(t, "malformed-cfg-agent", `name = "malformed-cfg-agent"
model = "anthropic/claude-sonnet-4-20250514"
`)

	// Write invalid config.toml
	configDir := filepath.Join(tmpDir, "axe")
	os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("[invalid toml\nblah"), 0644)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "malformed-cfg-agent"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for malformed config")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T", err)
	}
	if exitErr.Code != 2 {
		t.Errorf("expected exit code 2, got %d", exitErr.Code)
	}
}

// --- Phase 8a: Tool Injection ---

func TestRun_SubAgentToolInjection(t *testing.T) {
	resetRunCmd(t)
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := new(bytes.Buffer)
		body.ReadFrom(r.Body)
		receivedBody = body.String()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{
			"id": "msg_test",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "No delegation needed."}],
			"model": "claude-sonnet-4-20250514",
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 10, "output_tokens": 5}
		}`))
	}))
	defer server.Close()

	setupRunTestAgent(t, "parent-agent", `name = "parent-agent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["helper"]
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "parent-agent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the request body contains a tools array with call_agent
	if !strings.Contains(receivedBody, `"tools"`) {
		t.Errorf("expected 'tools' in request body, got %q", receivedBody)
	}
	if !strings.Contains(receivedBody, `"call_agent"`) {
		t.Errorf("expected 'call_agent' tool name in request body, got %q", receivedBody)
	}
}

func TestRun_NoSubAgents_NoTools(t *testing.T) {
	resetRunCmd(t)
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := new(bytes.Buffer)
		body.ReadFrom(r.Body)
		receivedBody = body.String()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{
			"id": "msg_test",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "Hello from mock"}],
			"model": "claude-sonnet-4-20250514",
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 10, "output_tokens": 5}
		}`))
	}))
	defer server.Close()

	setupRunTestAgent(t, "no-sub-agent", `name = "no-sub-agent"
model = "anthropic/claude-sonnet-4-20250514"
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "no-sub-agent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the request body does NOT contain a tools key
	if strings.Contains(receivedBody, `"tools"`) {
		t.Errorf("expected NO 'tools' in request body when sub_agents is empty, got %q", receivedBody)
	}
}

// --- Phase 8b: Conversation Loop Core ---

func TestRun_ConversationLoop_ToolCall(t *testing.T) {
	resetRunCmd(t)

	// Parent server: first request returns a tool call, second returns text
	parentCallCount := 0
	parentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parentCallCount++
		w.Header().Set("Content-Type", "application/json")
		if parentCallCount == 1 {
			// First call: return a tool_use calling "helper"
			w.Write([]byte(`{
				"id": "msg_1",
				"type": "message",
				"role": "assistant",
				"content": [
					{"type": "text", "text": "Let me delegate this."},
					{"type": "tool_use", "id": "toolu_123", "name": "call_agent", "input": {"agent": "helper", "task": "say hello"}}
				],
				"model": "claude-sonnet-4-20250514",
				"stop_reason": "tool_use",
				"usage": {"input_tokens": 10, "output_tokens": 20}
			}`))
		} else {
			// Second call: return final text
			w.Write([]byte(`{
				"id": "msg_2",
				"type": "message",
				"role": "assistant",
				"content": [{"type": "text", "text": "The helper said: greetings!"}],
				"model": "claude-sonnet-4-20250514",
				"stop_reason": "end_turn",
				"usage": {"input_tokens": 30, "output_tokens": 10}
			}`))
		}
	}))
	defer parentServer.Close()

	// Helper sub-agent server
	helperServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "msg_helper",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "greetings!"}],
			"model": "claude-sonnet-4-20250514",
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 5, "output_tokens": 3}
		}`))
	}))
	defer helperServer.Close()

	// Setup both agents - helper uses the same provider but different server
	tmpDir := setupRunTestAgent(t, "parent-loop", `name = "parent-loop"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["helper-loop"]
`)
	// Create helper agent TOML
	agentsDir := filepath.Join(tmpDir, "axe", "agents")
	os.WriteFile(filepath.Join(agentsDir, "helper-loop.toml"), []byte(`name = "helper-loop"
model = "anthropic/claude-sonnet-4-20250514"
`), 0644)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", parentServer.URL)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "parent-loop"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "The helper said: greetings!") {
		t.Errorf("expected final response from parent, got %q", output)
	}

	if parentCallCount != 2 {
		t.Errorf("expected parent server to be called 2 times, got %d", parentCallCount)
	}
}

func TestRun_ConversationLoop_MaxTurns(t *testing.T) {
	resetRunCmd(t)

	// Server always returns tool calls (never a final text response)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "msg_loop",
			"type": "message",
			"role": "assistant",
			"content": [
				{"type": "tool_use", "id": "toolu_loop", "name": "call_agent", "input": {"agent": "helper-turns", "task": "do something"}}
			],
			"model": "claude-sonnet-4-20250514",
			"stop_reason": "tool_use",
			"usage": {"input_tokens": 5, "output_tokens": 5}
		}`))
	}))
	defer server.Close()

	tmpDir := setupRunTestAgent(t, "loop-agent", `name = "loop-agent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["helper-turns"]
`)
	agentsDir := filepath.Join(tmpDir, "axe", "agents")
	os.WriteFile(filepath.Join(agentsDir, "helper-turns.toml"), []byte(`name = "helper-turns"
model = "anthropic/claude-sonnet-4-20250514"
`), 0644)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "loop-agent", "--timeout", "30"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for exceeding max turns, got nil")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 1 {
		t.Errorf("expected exit code 1, got %d", exitErr.Code)
	}
	if !strings.Contains(err.Error(), "maximum conversation turns") {
		t.Errorf("expected max turns error message, got: %v", err)
	}
}

func TestRun_SubAgent_Error_PropagatesAsToolResult(t *testing.T) {
	resetRunCmd(t)

	// Parent server: first returns tool call to nonexistent agent, second returns final text
	parentCallCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parentCallCount++
		w.Header().Set("Content-Type", "application/json")
		if parentCallCount == 1 {
			w.Write([]byte(`{
				"id": "msg_1",
				"type": "message",
				"role": "assistant",
				"content": [
					{"type": "tool_use", "id": "toolu_err", "name": "call_agent", "input": {"agent": "nonexistent", "task": "do something"}}
				],
				"model": "claude-sonnet-4-20250514",
				"stop_reason": "tool_use",
				"usage": {"input_tokens": 10, "output_tokens": 10}
			}`))
		} else {
			w.Write([]byte(`{
				"id": "msg_2",
				"type": "message",
				"role": "assistant",
				"content": [{"type": "text", "text": "The sub-agent failed, proceeding without it."}],
				"model": "claude-sonnet-4-20250514",
				"stop_reason": "end_turn",
				"usage": {"input_tokens": 20, "output_tokens": 10}
			}`))
		}
	}))
	defer server.Close()

	setupRunTestAgent(t, "err-parent", `name = "err-parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["nonexistent"]
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "err-parent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error (parent should handle sub-agent failure gracefully), got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "The sub-agent failed, proceeding without it.") {
		t.Errorf("expected parent to continue after sub-agent error, got %q", output)
	}

	if parentCallCount != 2 {
		t.Errorf("expected parent server to be called 2 times (tool call + final), got %d", parentCallCount)
	}
}

// --- Phase 8c: Parallel and Sequential Execution ---

func TestRun_ParallelToolCalls(t *testing.T) {
	resetRunCmd(t)

	// Track call counts and which agents were called
	var mu sync.Mutex
	callCount := 0
	subAgentCalls := make(map[string]bool)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := new(bytes.Buffer)
		body.ReadFrom(r.Body)
		bodyStr := body.String()
		w.Header().Set("Content-Type", "application/json")

		mu.Lock()
		callCount++
		currentCall := callCount
		mu.Unlock()

		// Check if this is a sub-agent call (has "Task:" in the message)
		if strings.Contains(bodyStr, "Task: task for agent-a") {
			mu.Lock()
			subAgentCalls["agent-a"] = true
			mu.Unlock()
			w.Write([]byte(`{
				"id": "msg_a", "type": "message", "role": "assistant",
				"content": [{"type": "text", "text": "result from a"}],
				"model": "claude-sonnet-4-20250514", "stop_reason": "end_turn",
				"usage": {"input_tokens": 5, "output_tokens": 3}
			}`))
			return
		}
		if strings.Contains(bodyStr, "Task: task for agent-b") {
			mu.Lock()
			subAgentCalls["agent-b"] = true
			mu.Unlock()
			w.Write([]byte(`{
				"id": "msg_b", "type": "message", "role": "assistant",
				"content": [{"type": "text", "text": "result from b"}],
				"model": "claude-sonnet-4-20250514", "stop_reason": "end_turn",
				"usage": {"input_tokens": 5, "output_tokens": 3}
			}`))
			return
		}

		// Parent calls
		if currentCall == 1 {
			// First parent call: return two tool calls
			w.Write([]byte(`{
				"id": "msg_p1", "type": "message", "role": "assistant",
				"content": [
					{"type": "tool_use", "id": "toolu_a", "name": "call_agent", "input": {"agent": "par-agent-a", "task": "task for agent-a"}},
					{"type": "tool_use", "id": "toolu_b", "name": "call_agent", "input": {"agent": "par-agent-b", "task": "task for agent-b"}}
				],
				"model": "claude-sonnet-4-20250514", "stop_reason": "tool_use",
				"usage": {"input_tokens": 10, "output_tokens": 20}
			}`))
		} else {
			// Second parent call: final response
			w.Write([]byte(`{
				"id": "msg_p2", "type": "message", "role": "assistant",
				"content": [{"type": "text", "text": "Both agents completed successfully."}],
				"model": "claude-sonnet-4-20250514", "stop_reason": "end_turn",
				"usage": {"input_tokens": 30, "output_tokens": 10}
			}`))
		}
	}))
	defer server.Close()

	tmpDir := setupRunTestAgent(t, "par-parent", `name = "par-parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["par-agent-a", "par-agent-b"]

[sub_agents_config]
parallel = true
`)
	agentsDir := filepath.Join(tmpDir, "axe", "agents")
	os.WriteFile(filepath.Join(agentsDir, "par-agent-a.toml"), []byte(`name = "par-agent-a"
model = "anthropic/claude-sonnet-4-20250514"
`), 0644)
	os.WriteFile(filepath.Join(agentsDir, "par-agent-b.toml"), []byte(`name = "par-agent-b"
model = "anthropic/claude-sonnet-4-20250514"
`), 0644)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "par-parent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Both agents completed successfully.") {
		t.Errorf("expected final response, got %q", output)
	}

	mu.Lock()
	defer mu.Unlock()
	if !subAgentCalls["agent-a"] {
		t.Error("expected sub-agent 'agent-a' to be called")
	}
	if !subAgentCalls["agent-b"] {
		t.Error("expected sub-agent 'agent-b' to be called")
	}
}

func TestRun_SequentialToolCalls(t *testing.T) {
	resetRunCmd(t)

	var mu sync.Mutex
	callCount := 0
	var callOrder []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := new(bytes.Buffer)
		body.ReadFrom(r.Body)
		bodyStr := body.String()
		w.Header().Set("Content-Type", "application/json")

		mu.Lock()
		callCount++
		currentCall := callCount
		mu.Unlock()

		if strings.Contains(bodyStr, "Task: seq task a") {
			mu.Lock()
			callOrder = append(callOrder, "a")
			mu.Unlock()
			w.Write([]byte(`{
				"id": "msg_a", "type": "message", "role": "assistant",
				"content": [{"type": "text", "text": "seq result a"}],
				"model": "claude-sonnet-4-20250514", "stop_reason": "end_turn",
				"usage": {"input_tokens": 5, "output_tokens": 3}
			}`))
			return
		}
		if strings.Contains(bodyStr, "Task: seq task b") {
			mu.Lock()
			callOrder = append(callOrder, "b")
			mu.Unlock()
			w.Write([]byte(`{
				"id": "msg_b", "type": "message", "role": "assistant",
				"content": [{"type": "text", "text": "seq result b"}],
				"model": "claude-sonnet-4-20250514", "stop_reason": "end_turn",
				"usage": {"input_tokens": 5, "output_tokens": 3}
			}`))
			return
		}

		if currentCall == 1 {
			w.Write([]byte(`{
				"id": "msg_p1", "type": "message", "role": "assistant",
				"content": [
					{"type": "tool_use", "id": "toolu_a", "name": "call_agent", "input": {"agent": "seq-agent-a", "task": "seq task a"}},
					{"type": "tool_use", "id": "toolu_b", "name": "call_agent", "input": {"agent": "seq-agent-b", "task": "seq task b"}}
				],
				"model": "claude-sonnet-4-20250514", "stop_reason": "tool_use",
				"usage": {"input_tokens": 10, "output_tokens": 20}
			}`))
		} else {
			w.Write([]byte(`{
				"id": "msg_p2", "type": "message", "role": "assistant",
				"content": [{"type": "text", "text": "Sequential done."}],
				"model": "claude-sonnet-4-20250514", "stop_reason": "end_turn",
				"usage": {"input_tokens": 30, "output_tokens": 10}
			}`))
		}
	}))
	defer server.Close()

	tmpDir := setupRunTestAgent(t, "seq-parent", `name = "seq-parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["seq-agent-a", "seq-agent-b"]

[sub_agents_config]
parallel = false
`)
	agentsDir := filepath.Join(tmpDir, "axe", "agents")
	os.WriteFile(filepath.Join(agentsDir, "seq-agent-a.toml"), []byte(`name = "seq-agent-a"
model = "anthropic/claude-sonnet-4-20250514"
`), 0644)
	os.WriteFile(filepath.Join(agentsDir, "seq-agent-b.toml"), []byte(`name = "seq-agent-b"
model = "anthropic/claude-sonnet-4-20250514"
`), 0644)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "seq-parent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Sequential done.") {
		t.Errorf("expected final response, got %q", output)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(callOrder) != 2 {
		t.Errorf("expected 2 sub-agent calls, got %d", len(callOrder))
	} else {
		// With sequential execution, a should come before b
		if callOrder[0] != "a" || callOrder[1] != "b" {
			t.Errorf("expected sequential order [a, b], got %v", callOrder)
		}
	}
}

// --- Phase 8d: Output Extensions (Dry-Run, JSON, Verbose) ---

func TestRun_DryRun_ShowsSubAgents(t *testing.T) {
	resetRunCmd(t)
	setupRunTestAgent(t, "dryrun-sub", `name = "dryrun-sub"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["helper-a", "helper-b"]

[sub_agents_config]
max_depth = 4
parallel = true
timeout = 60
`)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "dryrun-sub", "--dry-run"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "--- Sub-Agents ---") {
		t.Errorf("expected Sub-Agents section in dry-run output, got %q", output)
	}
	if !strings.Contains(output, "helper-a") {
		t.Errorf("expected agent name 'helper-a' in output, got %q", output)
	}
	if !strings.Contains(output, "helper-b") {
		t.Errorf("expected agent name 'helper-b' in output, got %q", output)
	}
	if !strings.Contains(output, "Max Depth:") {
		t.Errorf("expected 'Max Depth:' in output, got %q", output)
	}
	if !strings.Contains(output, "Parallel:") {
		t.Errorf("expected 'Parallel:' in output, got %q", output)
	}
	if !strings.Contains(output, "Timeout:") {
		t.Errorf("expected 'Timeout:' in output, got %q", output)
	}
}

func TestRun_DryRun_NoSubAgents(t *testing.T) {
	resetRunCmd(t)
	setupRunTestAgent(t, "dryrun-nosub", `name = "dryrun-nosub"
model = "anthropic/claude-sonnet-4-20250514"
`)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "dryrun-nosub", "--dry-run"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "--- Sub-Agents ---") {
		t.Errorf("expected Sub-Agents section in dry-run output, got %q", output)
	}
	if !strings.Contains(output, "(none)") {
		t.Errorf("expected '(none)' for empty sub-agents, got %q", output)
	}
}

func TestRun_JSON_IncludesToolCalls(t *testing.T) {
	resetRunCmd(t)

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := new(bytes.Buffer)
		body.ReadFrom(r.Body)
		bodyStr := body.String()
		w.Header().Set("Content-Type", "application/json")

		// Sub-agent call
		if strings.Contains(bodyStr, "Task: test task") {
			w.Write([]byte(`{
				"id": "msg_h", "type": "message", "role": "assistant",
				"content": [{"type": "text", "text": "sub-result"}],
				"model": "claude-sonnet-4-20250514", "stop_reason": "end_turn",
				"usage": {"input_tokens": 5, "output_tokens": 3}
			}`))
			return
		}

		callCount++
		if callCount == 1 {
			w.Write([]byte(`{
				"id": "msg_1", "type": "message", "role": "assistant",
				"content": [
					{"type": "tool_use", "id": "toolu_1", "name": "call_agent", "input": {"agent": "json-helper", "task": "test task"}}
				],
				"model": "claude-sonnet-4-20250514", "stop_reason": "tool_use",
				"usage": {"input_tokens": 10, "output_tokens": 15}
			}`))
		} else {
			w.Write([]byte(`{
				"id": "msg_2", "type": "message", "role": "assistant",
				"content": [{"type": "text", "text": "Final json result."}],
				"model": "claude-sonnet-4-20250514", "stop_reason": "end_turn",
				"usage": {"input_tokens": 20, "output_tokens": 10}
			}`))
		}
	}))
	defer server.Close()

	tmpDir := setupRunTestAgent(t, "json-parent", `name = "json-parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["json-helper"]
`)
	agentsDir := filepath.Join(tmpDir, "axe", "agents")
	os.WriteFile(filepath.Join(agentsDir, "json-helper.toml"), []byte(`name = "json-helper"
model = "anthropic/claude-sonnet-4-20250514"
`), 0644)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "json-parent", "--json"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, buf.String())
	}

	// Check tool_calls field exists and is > 0
	tc, ok := result["tool_calls"]
	if !ok {
		t.Fatalf("JSON output missing 'tool_calls' field: %v", result)
	}
	if tc.(float64) < 1 {
		t.Errorf("expected tool_calls >= 1, got %v", tc)
	}

	// Check cumulative token counts
	inputTokens := result["input_tokens"].(float64)
	if inputTokens < 20 {
		t.Errorf("expected cumulative input_tokens >= 20, got %v", inputTokens)
	}
}

func TestRun_Verbose_ConversationTurns(t *testing.T) {
	resetRunCmd(t)

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := new(bytes.Buffer)
		body.ReadFrom(r.Body)
		bodyStr := body.String()
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(bodyStr, "Task: verbose task") {
			w.Write([]byte(`{
				"id": "msg_h", "type": "message", "role": "assistant",
				"content": [{"type": "text", "text": "verbose sub-result"}],
				"model": "claude-sonnet-4-20250514", "stop_reason": "end_turn",
				"usage": {"input_tokens": 5, "output_tokens": 3}
			}`))
			return
		}

		callCount++
		if callCount == 1 {
			w.Write([]byte(`{
				"id": "msg_1", "type": "message", "role": "assistant",
				"content": [
					{"type": "tool_use", "id": "toolu_v", "name": "call_agent", "input": {"agent": "verbose-helper", "task": "verbose task"}}
				],
				"model": "claude-sonnet-4-20250514", "stop_reason": "tool_use",
				"usage": {"input_tokens": 10, "output_tokens": 15}
			}`))
		} else {
			w.Write([]byte(`{
				"id": "msg_2", "type": "message", "role": "assistant",
				"content": [{"type": "text", "text": "Verbose final."}],
				"model": "claude-sonnet-4-20250514", "stop_reason": "end_turn",
				"usage": {"input_tokens": 20, "output_tokens": 10}
			}`))
		}
	}))
	defer server.Close()

	tmpDir := setupRunTestAgent(t, "verbose-parent", `name = "verbose-parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["verbose-helper"]
`)
	agentsDir := filepath.Join(tmpDir, "axe", "agents")
	os.WriteFile(filepath.Join(agentsDir, "verbose-helper.toml"), []byte(`name = "verbose-helper"
model = "anthropic/claude-sonnet-4-20250514"
`), 0644)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "verbose-parent", "--verbose"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stderr := errBuf.String()
	// Should have turn-by-turn logs
	if !strings.Contains(stderr, "[turn 1]") {
		t.Errorf("expected '[turn 1]' in stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "[turn 2]") {
		t.Errorf("expected '[turn 2]' in stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "Sending request") {
		t.Errorf("expected 'Sending request' in stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "Received response") {
		t.Errorf("expected 'Received response' in stderr, got %q", stderr)
	}
}

// --- Review: Additional coverage tests ---

func TestRun_ParallelDefault_MultipleToolCalls(t *testing.T) {
	// Verify that when [sub_agents_config] is absent (Parallel is nil),
	// multiple tool calls still execute (parallel is the default).
	resetRunCmd(t)

	var mu sync.Mutex
	subAgentCalls := make(map[string]bool)
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := new(bytes.Buffer)
		body.ReadFrom(r.Body)
		bodyStr := body.String()
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(bodyStr, "Task: default-task-a") {
			mu.Lock()
			subAgentCalls["a"] = true
			mu.Unlock()
			w.Write([]byte(`{
				"id": "msg_a", "type": "message", "role": "assistant",
				"content": [{"type": "text", "text": "result a"}],
				"model": "claude-sonnet-4-20250514", "stop_reason": "end_turn",
				"usage": {"input_tokens": 5, "output_tokens": 3}
			}`))
			return
		}
		if strings.Contains(bodyStr, "Task: default-task-b") {
			mu.Lock()
			subAgentCalls["b"] = true
			mu.Unlock()
			w.Write([]byte(`{
				"id": "msg_b", "type": "message", "role": "assistant",
				"content": [{"type": "text", "text": "result b"}],
				"model": "claude-sonnet-4-20250514", "stop_reason": "end_turn",
				"usage": {"input_tokens": 5, "output_tokens": 3}
			}`))
			return
		}

		mu.Lock()
		callCount++
		cc := callCount
		mu.Unlock()

		if cc == 1 {
			w.Write([]byte(`{
				"id": "msg_p1", "type": "message", "role": "assistant",
				"content": [
					{"type": "tool_use", "id": "t_a", "name": "call_agent", "input": {"agent": "def-a", "task": "default-task-a"}},
					{"type": "tool_use", "id": "t_b", "name": "call_agent", "input": {"agent": "def-b", "task": "default-task-b"}}
				],
				"model": "claude-sonnet-4-20250514", "stop_reason": "tool_use",
				"usage": {"input_tokens": 10, "output_tokens": 20}
			}`))
		} else {
			w.Write([]byte(`{
				"id": "msg_p2", "type": "message", "role": "assistant",
				"content": [{"type": "text", "text": "Default parallel done."}],
				"model": "claude-sonnet-4-20250514", "stop_reason": "end_turn",
				"usage": {"input_tokens": 20, "output_tokens": 10}
			}`))
		}
	}))
	defer server.Close()

	// No [sub_agents_config] section - Parallel should default to true
	tmpDir := setupRunTestAgent(t, "def-parent", `name = "def-parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["def-a", "def-b"]
`)
	agentsDir := filepath.Join(tmpDir, "axe", "agents")
	os.WriteFile(filepath.Join(agentsDir, "def-a.toml"), []byte(`name = "def-a"
model = "anthropic/claude-sonnet-4-20250514"
`), 0644)
	os.WriteFile(filepath.Join(agentsDir, "def-b.toml"), []byte(`name = "def-b"
model = "anthropic/claude-sonnet-4-20250514"
`), 0644)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "def-parent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !subAgentCalls["a"] {
		t.Error("expected sub-agent 'a' to be called with default parallel=true")
	}
	if !subAgentCalls["b"] {
		t.Error("expected sub-agent 'b' to be called with default parallel=true")
	}
}

// --- Phase M6-4a: Memory Load into System Prompt ---

// startMockAnthropicServerCapture starts a mock Anthropic server that captures
// the request body and returns a configurable response.
func startMockAnthropicServerCapture(t *testing.T, capturedBody *string, mu *sync.Mutex, responseText string) *httptest.Server {
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
			"id": "msg_test",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "` + responseText + `"}],
			"model": "claude-sonnet-4-20250514",
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 10, "output_tokens": 5}
		}`))
	}))
}

func TestRun_MemoryDisabled_NoFileCreated(t *testing.T) {
	resetRunCmd(t)
	server := startMockAnthropicServer(t)
	defer server.Close()

	tmpDir := setupRunTestAgent(t, "mem-disabled", `name = "mem-disabled"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = false
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "mem-disabled"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify no memory file was created
	memoryDir := filepath.Join(tmpDir, "data", "axe", "memory")
	if _, err := os.Stat(memoryDir); !os.IsNotExist(err) {
		t.Errorf("expected memory directory to not exist, but it does (or other error): %v", err)
	}
}

func TestRun_MemoryEnabled_LoadsIntoPrompt(t *testing.T) {
	resetRunCmd(t)
	var capturedBody string
	var mu sync.Mutex
	server := startMockAnthropicServerCapture(t, &capturedBody, &mu, "memory response")
	defer server.Close()

	tmpDir := setupRunTestAgent(t, "mem-load", `name = "mem-load"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	// Pre-populate a memory file
	memoryDir := filepath.Join(tmpDir, "data", "axe", "memory")
	os.MkdirAll(memoryDir, 0755)
	memoryContent := "## 2026-02-28T10:00:00Z\n**Task:** previous task\n**Result:** previous result\n\n"
	os.WriteFile(filepath.Join(memoryDir, "mem-load.md"), []byte(memoryContent), 0644)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "mem-load"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	body := capturedBody
	mu.Unlock()

	// Verify the system prompt contains a Memory section with the pre-populated entries
	if !strings.Contains(body, "## Memory") {
		t.Errorf("expected '## Memory' in request body (system prompt), got %q", body)
	}
	if !strings.Contains(body, "previous task") {
		t.Errorf("expected 'previous task' in request body, got %q", body)
	}
	if !strings.Contains(body, "previous result") {
		t.Errorf("expected 'previous result' in request body, got %q", body)
	}
}

func TestRun_MemoryEnabled_LastN(t *testing.T) {
	resetRunCmd(t)
	var capturedBody string
	var mu sync.Mutex
	server := startMockAnthropicServerCapture(t, &capturedBody, &mu, "lastn response")
	defer server.Close()

	tmpDir := setupRunTestAgent(t, "mem-lastn", `name = "mem-lastn"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
last_n = 2
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	// Pre-populate a memory file with 5 entries
	memoryDir := filepath.Join(tmpDir, "data", "axe", "memory")
	os.MkdirAll(memoryDir, 0755)
	var memContent strings.Builder
	for i := 1; i <= 5; i++ {
		memContent.WriteString(fmt.Sprintf("## 2026-02-28T10:0%d:00Z\n**Task:** task %d\n**Result:** result %d\n\n", i, i, i))
	}
	os.WriteFile(filepath.Join(memoryDir, "mem-lastn.md"), []byte(memContent.String()), 0644)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "mem-lastn"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	body := capturedBody
	mu.Unlock()

	// Only the last 2 entries (task 4 and task 5) should be in the system prompt
	if !strings.Contains(body, "task 4") {
		t.Errorf("expected 'task 4' in request body, got %q", body)
	}
	if !strings.Contains(body, "task 5") {
		t.Errorf("expected 'task 5' in request body, got %q", body)
	}
	// Earlier entries should NOT be present
	if strings.Contains(body, "task 1") {
		t.Errorf("did not expect 'task 1' in request body (last_n=2), got %q", body)
	}
	if strings.Contains(body, "task 2") {
		t.Errorf("did not expect 'task 2' in request body (last_n=2), got %q", body)
	}
	if strings.Contains(body, "task 3") {
		t.Errorf("did not expect 'task 3' in request body (last_n=2), got %q", body)
	}
}

// --- Phase M6-4c: Max Entries Warning ---

func TestRun_MemoryEnabled_MaxEntriesWarning(t *testing.T) {
	resetRunCmd(t)
	server := startMockAnthropicServer(t)
	defer server.Close()

	tmpDir := setupRunTestAgent(t, "mem-maxwarn", `name = "mem-maxwarn"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
max_entries = 3
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	// Pre-populate memory file with 3 entries (meets max_entries)
	memoryDir := filepath.Join(tmpDir, "data", "axe", "memory")
	os.MkdirAll(memoryDir, 0755)
	var memContent strings.Builder
	for i := 1; i <= 3; i++ {
		memContent.WriteString(fmt.Sprintf("## 2026-02-28T10:0%d:00Z\n**Task:** task %d\n**Result:** result %d\n\n", i, i, i))
	}
	os.WriteFile(filepath.Join(memoryDir, "mem-maxwarn.md"), []byte(memContent.String()), 0644)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "mem-maxwarn"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "Warning:") {
		t.Errorf("expected warning in stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "mem-maxwarn") {
		t.Errorf("expected agent name in warning, got %q", stderr)
	}
	if !strings.Contains(stderr, "3 entries") {
		t.Errorf("expected entry count in warning, got %q", stderr)
	}
	if !strings.Contains(stderr, "max_entries: 3") {
		t.Errorf("expected max_entries value in warning, got %q", stderr)
	}
}

func TestRun_MemoryEnabled_MaxEntriesNoWarningWhenBelow(t *testing.T) {
	resetRunCmd(t)
	server := startMockAnthropicServer(t)
	defer server.Close()

	tmpDir := setupRunTestAgent(t, "mem-nowarn", `name = "mem-nowarn"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
max_entries = 10
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	// Pre-populate memory file with 3 entries (below max_entries=10)
	memoryDir := filepath.Join(tmpDir, "data", "axe", "memory")
	os.MkdirAll(memoryDir, 0755)
	var memContent strings.Builder
	for i := 1; i <= 3; i++ {
		memContent.WriteString(fmt.Sprintf("## 2026-02-28T10:0%d:00Z\n**Task:** task %d\n**Result:** result %d\n\n", i, i, i))
	}
	os.WriteFile(filepath.Join(memoryDir, "mem-nowarn.md"), []byte(memContent.String()), 0644)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "mem-nowarn"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stderr := errBuf.String()
	if strings.Contains(stderr, "Warning:") {
		t.Errorf("did not expect warning in stderr when below max_entries, got %q", stderr)
	}
}

// --- Phase M6-4e: Memory Append After Response ---

func TestRun_MemoryEnabled_AppendsEntry(t *testing.T) {
	resetRunCmd(t)
	server := startMockAnthropicServer(t)
	defer server.Close()

	tmpDir := setupRunTestAgent(t, "mem-append", `name = "mem-append"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "mem-append"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the memory file was created with an entry
	memoryFile := filepath.Join(tmpDir, "data", "axe", "memory", "mem-append.md")
	data, err := os.ReadFile(memoryFile)
	if err != nil {
		t.Fatalf("expected memory file to exist: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "## ") {
		t.Errorf("expected entry header in memory file, got %q", content)
	}
	if !strings.Contains(content, "**Task:**") {
		t.Errorf("expected Task line in memory file, got %q", content)
	}
	if !strings.Contains(content, "**Result:**") {
		t.Errorf("expected Result line in memory file, got %q", content)
	}
	if !strings.Contains(content, "Hello from mock") {
		t.Errorf("expected LLM response in memory file result, got %q", content)
	}
}

func TestRun_MemoryEnabled_APIError_NoEntryAppended(t *testing.T) {
	resetRunCmd(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		w.Write([]byte(`{"type": "error", "error": {"type": "server_error", "message": "Internal server error"}}`))
	}))
	defer server.Close()

	tmpDir := setupRunTestAgent(t, "mem-err", `name = "mem-err"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "mem-err"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for server error, got nil")
	}

	// Verify no memory file was created
	memoryFile := filepath.Join(tmpDir, "data", "axe", "memory", "mem-err.md")
	if _, err := os.Stat(memoryFile); !os.IsNotExist(err) {
		t.Errorf("expected no memory file after API error, but file exists or other error: %v", err)
	}
}

// --- Phase M6-4g: Dry-Run Memory Display ---

func TestRun_MemoryEnabled_DryRun(t *testing.T) {
	resetRunCmd(t)

	tmpDir := setupRunTestAgent(t, "mem-dryrun", `name = "mem-dryrun"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
`)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	// Pre-populate memory file
	memoryDir := filepath.Join(tmpDir, "data", "axe", "memory")
	os.MkdirAll(memoryDir, 0755)
	memoryContent := "## 2026-02-28T10:00:00Z\n**Task:** dry run task\n**Result:** dry run result\n\n"
	os.WriteFile(filepath.Join(memoryDir, "mem-dryrun.md"), []byte(memoryContent), 0644)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "mem-dryrun", "--dry-run"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "--- Memory ---") {
		t.Errorf("expected '--- Memory ---' section in dry-run output, got %q", output)
	}
	if !strings.Contains(output, "dry run task") {
		t.Errorf("expected memory entry content in dry-run output, got %q", output)
	}
	if !strings.Contains(output, "dry run result") {
		t.Errorf("expected memory entry result in dry-run output, got %q", output)
	}

	// Verify no new entry was appended
	data, _ := os.ReadFile(filepath.Join(memoryDir, "mem-dryrun.md"))
	if string(data) != memoryContent {
		t.Errorf("dry-run should not modify memory file; original=%q, got=%q", memoryContent, string(data))
	}
}

func TestRun_MemoryEnabled_DryRun_NoMemoryFile(t *testing.T) {
	resetRunCmd(t)

	tmpDir := setupRunTestAgent(t, "mem-dryrun-none", `name = "mem-dryrun-none"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
`)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "mem-dryrun-none", "--dry-run"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "--- Memory ---") {
		t.Errorf("expected '--- Memory ---' section in dry-run output, got %q", output)
	}
	if !strings.Contains(output, "(none)") {
		t.Errorf("expected '(none)' for empty memory in dry-run output, got %q", output)
	}
}

// --- Phase M6-4i: Verbose Memory Output ---

func TestRun_MemoryEnabled_Verbose(t *testing.T) {
	resetRunCmd(t)
	server := startMockAnthropicServer(t)
	defer server.Close()

	tmpDir := setupRunTestAgent(t, "mem-verbose", `name = "mem-verbose"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))

	// Pre-populate memory file with 3 entries
	memoryDir := filepath.Join(tmpDir, "data", "axe", "memory")
	os.MkdirAll(memoryDir, 0755)
	var memContent strings.Builder
	for i := 1; i <= 3; i++ {
		memContent.WriteString(fmt.Sprintf("## 2026-02-28T10:0%d:00Z\n**Task:** task %d\n**Result:** result %d\n\n", i, i, i))
	}
	os.WriteFile(filepath.Join(memoryDir, "mem-verbose.md"), []byte(memContent.String()), 0644)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "mem-verbose", "--verbose"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "Memory:") {
		t.Errorf("expected 'Memory:' in verbose stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "3 entries") {
		t.Errorf("expected '3 entries' in verbose stderr, got %q", stderr)
	}
}

// --- Phase M6-4k: Custom Path ---

func TestRun_MemoryEnabled_CustomPath(t *testing.T) {
	resetRunCmd(t)
	server := startMockAnthropicServer(t)
	defer server.Close()

	customDir := t.TempDir()
	customPath := filepath.Join(customDir, "custom-memory.md")

	setupRunTestAgent(t, "mem-custom", `name = "mem-custom"
model = "anthropic/claude-sonnet-4-20250514"

[memory]
enabled = true
path = "`+customPath+`"
`)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "mem-custom"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the custom path was used for writing
	data, err := os.ReadFile(customPath)
	if err != nil {
		t.Fatalf("expected memory file at custom path: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "**Task:**") {
		t.Errorf("expected Task line in memory file at custom path, got %q", content)
	}
	if !strings.Contains(content, "Hello from mock") {
		t.Errorf("expected LLM response in memory file at custom path, got %q", content)
	}
}

func TestRun_DryRun_NonAnthropicProvider(t *testing.T) {
	resetRunCmd(t)
	setupRunTestAgent(t, "dryrun-openai", `name = "dryrun-openai"
model = "openai/gpt-4o"
`)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "dryrun-openai", "--dry-run"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "openai/gpt-4o") {
		t.Errorf("expected 'openai/gpt-4o' in dry-run output, got %q", output)
	}
}
