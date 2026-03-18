package cmd

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrswab/axe/internal/testutil"
)

func TestMain(m *testing.M) {
	code := m.Run()
	testutil.CleanupBinary()
	os.Exit(code)
}

// setupSmokeEnv creates an isolated XDG directory structure and returns an env
// map suitable for passing to runAxe. Unlike testutil.SetupXDGDirs, this does
// not call t.Setenv — it returns a map for the child process environment.
func setupSmokeEnv(t *testing.T) (configDir, dataDir string, env map[string]string) {
	t.Helper()

	root := t.TempDir()

	configHome := filepath.Join(root, "config")
	dataHome := filepath.Join(root, "data")

	configDir = filepath.Join(configHome, "axe")
	dataDir = filepath.Join(dataHome, "axe")

	dirs := []string{
		filepath.Join(configDir, "agents"),
		filepath.Join(configDir, "skills"),
		dataDir,
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("failed to create directory %s: %v", d, err)
		}
	}

	env = map[string]string{
		"XDG_CONFIG_HOME": configHome,
		"XDG_DATA_HOME":   dataHome,
	}

	return configDir, dataDir, env
}

// stripAPIKeys blanks out known provider API key env vars so the child process
// does not inherit real keys from the host machine.
func stripAPIKeys(env map[string]string) map[string]string {
	env["OPENAI_API_KEY"] = ""
	env["ANTHROPIC_API_KEY"] = ""
	env["OLLAMA_API_KEY"] = ""
	return env
}

// runAxe executes the compiled axe binary with the given args and env overrides.
// Returns stdout, stderr, and the process exit code.
func runAxe(t *testing.T, env map[string]string, stdinData string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	binPath := testutil.BuildBinary(t)

	cmd := exec.Command(binPath, args...)

	// Build child environment: start with host env, then apply overrides.
	if env != nil {
		childEnv := os.Environ()
		for k, v := range env {
			childEnv = replaceOrAppendEnv(childEnv, k, v)
		}
		cmd.Env = childEnv
	}

	if stdinData != "" {
		cmd.Stdin = strings.NewReader(stdinData)
	}

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return outBuf.String(), errBuf.String(), exitErr.ExitCode()
		}
		t.Fatalf("runAxe: unexpected error running binary: %v", err)
	}

	return outBuf.String(), errBuf.String(), 0
}

// replaceOrAppendEnv sets key=value in an environ slice, replacing an existing
// entry for the same key or appending a new one.
func replaceOrAppendEnv(environ []string, key, value string) []string {
	prefix := key + "="
	for i, entry := range environ {
		if strings.HasPrefix(entry, prefix) {
			environ[i] = prefix + value
			return environ
		}
	}
	return append(environ, prefix+value)
}

// ---------------------------------------------------------------------------
// Smoke Tests
// ---------------------------------------------------------------------------

func TestSmoke_Version(t *testing.T) {
	t.Parallel()

	stdout, stderr, exitCode := runAxe(t, nil, "", "version")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", exitCode, stderr)
	}
	if !strings.Contains(stdout, "axe version ") {
		t.Errorf("stdout does not contain 'axe version ': %q", stdout)
	}
	if stdout == "" || !strings.HasSuffix(stdout, "\n") {
		t.Errorf("stdout should be non-empty and end with newline: %q", stdout)
	}
	if stderr != "" {
		t.Errorf("expected empty stderr, got: %q", stderr)
	}
}

func TestSmoke_ConfigPath(t *testing.T) {
	t.Parallel()

	configDir, _, env := setupSmokeEnv(t)

	stdout, stderr, exitCode := runAxe(t, env, "", "config", "path")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", exitCode, stderr)
	}

	trimmed := strings.TrimSpace(stdout)
	if trimmed == "" {
		t.Fatal("stdout is empty")
	}
	if !strings.HasSuffix(trimmed, string(filepath.Separator)+"axe") {
		t.Errorf("stdout should end with /axe: %q", trimmed)
	}
	if trimmed != configDir {
		t.Errorf("stdout %q does not match expected configDir %q", trimmed, configDir)
	}
	if stderr != "" {
		t.Errorf("expected empty stderr, got: %q", stderr)
	}
}

func TestSmoke_ConfigInit(t *testing.T) {
	t.Parallel()

	configDir, _, env := setupSmokeEnv(t)

	// First invocation
	stdout, stderr, exitCode := runAxe(t, env, "", "config", "init")

	if exitCode != 0 {
		t.Fatalf("first init: expected exit code 0, got %d; stderr: %s", exitCode, stderr)
	}
	if strings.TrimSpace(stdout) != configDir {
		t.Errorf("first init: stdout %q does not match configDir %q", strings.TrimSpace(stdout), configDir)
	}
	if stderr != "" {
		t.Errorf("first init: expected empty stderr, got: %q", stderr)
	}

	// Verify files/dirs created
	agentsDir := filepath.Join(configDir, "agents")
	if info, err := os.Stat(agentsDir); err != nil || !info.IsDir() {
		t.Errorf("agents/ directory not created: %v", err)
	}

	skillMD := filepath.Join(configDir, "skills", "sample", "SKILL.md")
	skillData, err := os.ReadFile(skillMD)
	if err != nil {
		t.Fatalf("skills/sample/SKILL.md not created: %v", err)
	}
	if len(skillData) == 0 {
		t.Error("skills/sample/SKILL.md is empty")
	}

	configTOML := filepath.Join(configDir, "config.toml")
	configData, err := os.ReadFile(configTOML)
	if err != nil {
		t.Fatalf("config.toml not created: %v", err)
	}
	if len(configData) == 0 {
		t.Error("config.toml is empty")
	}

	// Idempotency: second invocation
	_, stderr2, exitCode2 := runAxe(t, env, "", "config", "init")

	if exitCode2 != 0 {
		t.Fatalf("second init: expected exit code 0, got %d; stderr: %s", exitCode2, stderr2)
	}

	// Files must be byte-identical
	skillData2, err := os.ReadFile(skillMD)
	if err != nil {
		t.Fatalf("second init: failed to read SKILL.md: %v", err)
	}
	if !bytes.Equal(skillData, skillData2) {
		t.Error("second init overwrote SKILL.md")
	}

	configData2, err := os.ReadFile(configTOML)
	if err != nil {
		t.Fatalf("second init: failed to read config.toml: %v", err)
	}
	if !bytes.Equal(configData, configData2) {
		t.Error("second init overwrote config.toml")
	}
}

func TestSmoke_RunNonexistentAgent(t *testing.T) {
	t.Parallel()

	_, _, env := setupSmokeEnv(t)
	stripAPIKeys(env)

	stdout, stderr, exitCode := runAxe(t, env, "", "run", "nonexistent-agent")

	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d; stderr: %s", exitCode, stderr)
	}
	if stderr == "" {
		t.Error("expected non-empty stderr")
	}
	if !strings.Contains(stderr, "nonexistent-agent") {
		t.Errorf("stderr does not reference agent name: %q", stderr)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got: %q", stdout)
	}
}

func TestSmoke_RunDryRun(t *testing.T) {
	t.Parallel()

	configDir, _, env := setupSmokeEnv(t)
	testutil.SeedFixtureAgents(t, "testdata/agents", filepath.Join(configDir, "agents"))
	stripAPIKeys(env)

	stdout, stderr, exitCode := runAxe(t, env, "", "run", "basic", "--dry-run")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", exitCode, stderr)
	}

	checks := []string{
		"=== Dry Run ===",
		"--- System Prompt ---",
		"--- Stdin ---",
	}
	for _, check := range checks {
		if !strings.Contains(stdout, check) {
			t.Errorf("stdout does not contain %q", check)
		}
	}

	// Model line uses alignment spacing: "Model:    openai/gpt-4o"
	if !strings.Contains(stdout, "openai/gpt-4o") {
		t.Errorf("stdout does not contain model 'openai/gpt-4o': %q", stdout)
	}

	// Verify "(none)" appears in the Stdin section (no stdin was piped)
	stdinIdx := strings.Index(stdout, "--- Stdin ---")
	if stdinIdx >= 0 {
		afterStdin := stdout[stdinIdx:]
		nextSection := strings.Index(afterStdin[len("--- Stdin ---"):], "---")
		var stdinSection string
		if nextSection >= 0 {
			stdinSection = afterStdin[:len("--- Stdin ---")+nextSection]
		} else {
			stdinSection = afterStdin
		}
		if !strings.Contains(stdinSection, "(none)") {
			t.Errorf("Stdin section should contain '(none)' when no stdin is piped: %q", stdinSection)
		}
	}

	if stderr != "" {
		t.Errorf("expected empty stderr, got: %q", stderr)
	}
}

func TestSmoke_BadModelFormat(t *testing.T) {
	t.Parallel()

	configDir, _, env := setupSmokeEnv(t)
	testutil.SeedFixtureAgents(t, "testdata/agents", filepath.Join(configDir, "agents"))
	stripAPIKeys(env)

	stdout, stderr, exitCode := runAxe(t, env, "", "run", "basic", "--model", "no-slash-here")

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d; stderr: %s", exitCode, stderr)
	}
	if !strings.Contains(stderr, "invalid model format") {
		t.Errorf("stderr does not contain 'invalid model format': %q", stderr)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got: %q", stdout)
	}
}

func TestSmoke_MissingAPIKey(t *testing.T) {
	t.Parallel()

	configDir, _, env := setupSmokeEnv(t)
	testutil.SeedFixtureAgents(t, "testdata/agents", filepath.Join(configDir, "agents"))
	stripAPIKeys(env)

	stdout, stderr, exitCode := runAxe(t, env, "", "run", "basic")

	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d; stderr: %s", exitCode, stderr)
	}
	if !strings.Contains(stderr, "API key") {
		t.Errorf("stderr does not contain 'API key': %q", stderr)
	}
	if !strings.Contains(stderr, "OPENAI_API_KEY") {
		t.Errorf("stderr does not contain 'OPENAI_API_KEY': %q", stderr)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got: %q", stdout)
	}
}

func TestSmoke_PipedStdinInDryRun(t *testing.T) {
	t.Parallel()

	configDir, _, env := setupSmokeEnv(t)
	testutil.SeedFixtureAgents(t, "testdata/agents", filepath.Join(configDir, "agents"))
	stripAPIKeys(env)

	stdout, stderr, exitCode := runAxe(t, env, "custom user input from stdin", "run", "basic", "--dry-run")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", exitCode, stderr)
	}

	checks := []string{
		"=== Dry Run ===",
		"--- Stdin ---",
		"custom user input from stdin",
	}
	for _, check := range checks {
		if !strings.Contains(stdout, check) {
			t.Errorf("stdout does not contain %q", check)
		}
	}

	// Verify "(none)" does NOT appear in the Stdin section
	stdinIdx := strings.Index(stdout, "--- Stdin ---")
	if stdinIdx >= 0 {
		afterStdin := stdout[stdinIdx:]
		nextSection := strings.Index(afterStdin[len("--- Stdin ---"):], "---")
		var stdinSection string
		if nextSection >= 0 {
			stdinSection = afterStdin[:len("--- Stdin ---")+nextSection]
		} else {
			stdinSection = afterStdin
		}
		if strings.Contains(stdinSection, "(none)") {
			t.Errorf("Stdin section should NOT contain '(none)' when stdin is piped: %q", stdinSection)
		}
	} else {
		t.Error("stdout does not contain '--- Stdin ---' section")
	}

	if stderr != "" {
		t.Errorf("expected empty stderr, got: %q", stderr)
	}
}
