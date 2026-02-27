package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
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

func TestRunE_ErrorDoesNotPrintUsage(t *testing.T) {
	resetRunCmd(t)
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	agentsDir := filepath.Join(tmpDir, "axe", "agents")
	os.MkdirAll(agentsDir, 0755)

	tests := []struct {
		name string
		args []string
	}{
		{"args_validation_error", []string{"run"}},
		{"runE_error", []string{"run", "nonexistent"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetRunCmd(t)
			errBuf := new(bytes.Buffer)
			rootCmd.SetOut(new(bytes.Buffer))
			rootCmd.SetErr(errBuf)
			rootCmd.SetArgs(tc.args)

			err := rootCmd.Execute()
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			stderr := errBuf.String()
			// Cobra should NOT print usage/help text to stderr on errors
			if strings.Contains(stderr, "Usage:") {
				t.Errorf("stderr should not contain usage text, got:\n%s", stderr)
			}
			if strings.Contains(stderr, "Available Commands:") {
				t.Errorf("stderr should not contain available commands, got:\n%s", stderr)
			}
		})
	}
}

func TestRunE_ErrorNotPrintedByCobra(t *testing.T) {
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run"}) // missing required arg

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing args, got nil")
	}

	stderr := errBuf.String()
	// With SilenceErrors, Cobra must not write the error to stderr itself.
	// The error string from ExactArgs would contain "accepts 1 arg(s)".
	if strings.Contains(stderr, "accepts 1 arg(s)") {
		t.Errorf("Cobra should not print the error to stderr (duplicate); got:\n%s", stderr)
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
