package cmd

import (
	"strings"
	"testing"

	"github.com/jrswab/axe/internal/agent"
)

func TestSmokeSubagentFixtureContract(t *testing.T) {
	t.Parallel()

	const agentsDir = "../.github/smoke-agents"
	child, err := agent.Load("cfg-sub-child", []string{agentsDir})
	if err != nil {
		t.Fatalf("load actual smoke child fixture: %v", err)
	}
	_, _, env := setupSmokeEnv(t)
	stripAPIKeys(env)

	stdout, stderr, exitCode := runAxe(t, env, "", "run",
		"--agents-dir", agentsDir, "cfg-subagents",
		"--dry-run", "-p", "Call your sub-agent now.")

	if exitCode != 0 {
		t.Fatalf("dry-run exited %d; stdout: %s; stderr: %s", exitCode, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got: %q", stderr)
	}
	if child.Name != "cfg-sub-child" {
		t.Errorf("child fixture name = %q, want cfg-sub-child", child.Name)
	}
	if !strings.Contains(child.SystemPrompt, "When called, reply with only: sub-agent-ok") {
		t.Errorf("child fixture must retain its sentinel reply contract; got: %q", child.SystemPrompt)
	}

	subAgents := extractSection(stdout, "--- Sub-Agents ---")
	allowed, _, _ := strings.Cut(strings.TrimSpace(strings.TrimPrefix(subAgents, "--- Sub-Agents ---")), "\n")
	if allowed != child.Name {
		t.Errorf("parent allowed child list = %q, want only fixture name %q", allowed, child.Name)
	}

	prompt := extractSection(stdout, "--- System Prompt ---")
	if prompt == "" {
		t.Fatalf("dry-run is missing System Prompt section; stdout: %s", stdout)
	}
	for _, instruction := range []string{
		"You must always call the cfg-sub-child sub-agent immediately.",
		"Do not ask for clarification.",
		"Call the sub-agent with the task 'reply with only: sub-agent-ok'",
		"and report its exact reply.",
	} {
		if !strings.Contains(prompt, instruction) {
			t.Errorf("actual cfg-subagents System Prompt must contain %q; got:\n%s", instruction, prompt)
		}
	}
}
