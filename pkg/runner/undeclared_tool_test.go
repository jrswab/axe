package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrswab/axe/internal/testutil"
)

func TestRun_UndeclaredBuiltinToolIsNotExecuted(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Writing", []testutil.MockToolCall{
			{ID: "call_1", Name: "write_file", Input: map[string]string{"path": "undeclared.txt", "content": "x"}},
		}),
		testutil.AnthropicResponse("Done"),
	})
	setupTestEnv(t, "[providers.anthropic]\napi_key = \"fake-key\"\n", "test-agent", `name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["read_file"]
`)
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())
	stdout, stderr := newMockStdoutStderr()
	workdir := t.TempDir()

	if _, err := Run(context.Background(), Options{
		AgentName: "test-agent",
		Workdir:   workdir,
		Stdin:     strings.NewReader(""),
		Stdout:    stdout,
		Stderr:    stderr,
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(workdir, "undeclared.txt")); !os.IsNotExist(err) {
		t.Errorf("write_file ran although the agent does not declare it (stat error: %v)", err)
	}
	if mock.RequestCount() != 2 {
		t.Fatalf("LLM request count = %d, want 2", mock.RequestCount())
	}
	if !strings.Contains(mock.Requests[1].Body, "unknown tool") {
		t.Error("the model did not receive an unknown tool result")
	}
}
