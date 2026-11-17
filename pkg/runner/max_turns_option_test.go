package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrswab/axe/internal/testutil"
)

func TestMaxTurnsOption_OverridesTOML(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, maxTurnsToolResponses(2))

	setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"test-agent",
		`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
max_turns = 5
tools = ["list_directory"]
`,
	)
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()
	_, err := Run(context.Background(), Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
		MaxTurns:  2,
	})
	if !IsRuntimeError(err) {
		t.Fatalf("expected RuntimeError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "exceeded maximum conversation turns (2)") {
		t.Errorf("expected overridden max turns in error, got: %v", err)
	}
	if got := mock.RequestCount(); got != 2 {
		t.Errorf("RequestCount = %d, want 2", got)
	}
}

func TestMaxTurnsOption_NegativeConfigError(t *testing.T) {
	tests := []struct {
		value  int
		dryRun bool
	}{
		{-1, false},
		{-1, true},
		{-5, false},
		{-5, true},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("value_%d_dry_run_%t", tt.value, tt.dryRun), func(t *testing.T) {
			mock := testutil.NewMockLLMServer(t, nil)
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
			_, err := Run(context.Background(), Options{
				AgentName: "test-agent",
				Stdout:    stdout,
				Stderr:    stderr,
				MaxTurns:  tt.value,
				DryRun:    tt.dryRun,
			})
			if got := mock.RequestCount(); got != 0 {
				t.Errorf("RequestCount = %d, want 0", got)
			}
			if !IsConfigError(err) {
				t.Fatalf("expected ConfigError, got %T: %v", err, err)
			}
			if !strings.Contains(err.Error(), "Options.MaxTurns") {
				t.Errorf("error message missing Options.MaxTurns: %v", err)
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("%d", tt.value)) {
				t.Errorf("error message missing negative value %d: %v", tt.value, err)
			}
		})
	}
}

func TestMaxTurnsOption_ZeroKeepsTOMLAndDefault(t *testing.T) {
	tests := []struct {
		name         string
		maxTurnsTOML string
		wantTurns    int
	}{
		{"toml_2", "max_turns = 2\n", 2},
		{"unset", "", 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := testutil.NewMockLLMServer(t, maxTurnsToolResponses(tt.wantTurns))
			setupTestEnv(t,
				`[providers.anthropic]
api_key = "fake-key"
`,
				"test-agent",
				`name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["list_directory"]
`+tt.maxTurnsTOML,
			)
			t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

			stdout, stderr := newMockStdoutStderr()
			_, err := Run(context.Background(), Options{
				AgentName: "test-agent",
				Stdout:    stdout,
				Stderr:    stderr,
				MaxTurns:  0,
			})
			if !IsRuntimeError(err) {
				t.Fatalf("expected RuntimeError, got %T: %v", err, err)
			}
			want := fmt.Sprintf("exceeded maximum conversation turns (%d)", tt.wantTurns)
			if !strings.Contains(err.Error(), want) {
				t.Errorf("expected %q in error, got: %v", want, err)
			}
			if got := mock.RequestCount(); got != tt.wantTurns {
				t.Errorf("RequestCount = %d, want %d", got, tt.wantTurns)
			}
		})
	}
}

func TestMaxTurnsOption_SubAgentNotInherited(t *testing.T) {
	responses := []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Delegating", []testutil.MockToolCall{
			{ID: "tool_1", Name: "call_agent", Input: map[string]string{"agent": "child-agent", "task": "say hello"}},
		}),
	}
	responses = append(responses, maxTurnsToolResponses(3)...)
	responses = append(responses,
		testutil.AnthropicResponse("child done"),
		testutil.AnthropicResponse("parent done"),
	)
	mock := testutil.NewMockLLMServer(t, responses)

	tmpDir := setupTestEnv(t,
		`[providers.anthropic]
api_key = "fake-key"
`,
		"parent-agent",
		`name = "parent-agent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["child-agent"]
tools = ["list_directory"]
`,
	)
	childPath := filepath.Join(tmpDir, "axe", "agents", "child-agent.toml")
	childTOML := `name = "child-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["list_directory"]
`
	if err := os.WriteFile(childPath, []byte(childTOML), 0644); err != nil {
		t.Fatalf("failed to write child agent: %v", err)
	}
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()
	_, err := Run(context.Background(), Options{
		AgentName: "parent-agent",
		Stdout:    stdout,
		Stderr:    stderr,
		MaxTurns:  2,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := mock.RequestCount(); got != 6 {
		t.Fatalf("RequestCount = %d, want 6", got)
	}
	if !strings.Contains(mock.Requests[5].Body, "child done") {
		t.Errorf("parent request missing child result: %s", mock.Requests[5].Body)
	}
	if strings.Contains(mock.Requests[5].Body, "sub-agent exceeded maximum conversation turns") {
		t.Errorf("parent request contains child turn-limit error: %s", mock.Requests[5].Body)
	}
	if !strings.Contains(mock.Requests[0].Body, "call_agent") {
		t.Errorf("parent request missing call_agent tool: %s", mock.Requests[0].Body)
	}
}

func maxTurnsToolResponses(count int) []testutil.MockLLMResponse {
	responses := make([]testutil.MockLLMResponse, count)
	for i := range responses {
		responses[i] = testutil.AnthropicToolUseResponse("Tool call", []testutil.MockToolCall{
			{ID: fmt.Sprintf("tool_%d", i), Name: "list_directory", Input: map[string]string{"path": "."}},
		})
	}
	return responses
}
