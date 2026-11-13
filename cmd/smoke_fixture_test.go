package cmd

import (
	"strings"
	"testing"
)

func TestSmokeSubagentFixtureContract(t *testing.T) {
	t.Parallel()

	_, _, env := setupSmokeEnv(t)
	stripAPIKeys(env)

	stdout, stderr, exitCode := runAxe(t, env, "", "run",
		"--agents-dir", "../.github/smoke-agents", "cfg-subagents",
		"--dry-run", "-p", "Call your sub-agent now.")

	if exitCode != 0 {
		t.Fatalf("dry-run exited %d; stdout: %s; stderr: %s", exitCode, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got: %q", stderr)
	}

	prompt := extractSection(stdout, "--- System Prompt ---")
	if prompt == "" {
		t.Fatalf("dry-run is missing System Prompt section; stdout: %s", stdout)
	}
	for _, instruction := range []string{"cfg-sub-child", "reply with only: sub-agent-ok"} {
		if !strings.Contains(prompt, instruction) {
			t.Errorf("actual cfg-subagents System Prompt must contain %q; got:\n%s", instruction, prompt)
		}
	}
}
