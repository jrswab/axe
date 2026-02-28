package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
