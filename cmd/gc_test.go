package cmd

import (
	"bytes"
	"errors"
	"strings"
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
