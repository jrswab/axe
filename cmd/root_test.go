package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestHelpCommand_DisplaysCommands(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Must list available commands
	if !strings.Contains(output, "version") {
		t.Error("help output missing 'version' command")
	}
	if !strings.Contains(output, "config") {
		t.Error("help output missing 'config' command")
	}
}

func TestHelpCommand_DisplaysUsageExamples(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Must show usage examples (defined in rootCmd.Example)
	if !strings.Contains(output, "axe version") {
		t.Error("help output missing usage example for 'axe version'")
	}
	if !strings.Contains(output, "axe config path") {
		t.Error("help output missing usage example for 'axe config path'")
	}
}

func TestRootCommand_Description(t *testing.T) {
	if rootCmd.Short == "" {
		t.Error("root command missing short description")
	}
	if rootCmd.Long == "" {
		t.Error("root command missing long description")
	}
}

func TestExitCodeFromError_ExitError(t *testing.T) {
	err := &ExitError{Code: 2, Err: errors.New("config error")}
	got := exitCodeFromError(err)
	if got != 2 {
		t.Errorf("exitCodeFromError(ExitError{Code:2}) = %d, want 2", got)
	}
}

func TestExitCodeFromError_DefaultExitCode(t *testing.T) {
	err := errors.New("generic error")
	got := exitCodeFromError(err)
	if got != 1 {
		t.Errorf("exitCodeFromError(generic) = %d, want 1", got)
	}
}
