package tool

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateCommand_AbsolutePathOutsideWorkdir(t *testing.T) {
	workdir := t.TempDir()

	tests := []struct {
		name    string
		command string
	}{
		{"absolute file path", "cat /etc/passwd"},
		{"absolute directory path", "ls /tmp"},
		{"absolute output path", "echo hello > /tmp/out"},
		{"env var with absolute path", "FOO=/etc/passwd cmd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCommand(workdir, tt.command)
			if err == nil {
				t.Errorf("expected error for command %q, got nil", tt.command)
				return
			}
			if !strings.Contains(err.Error(), "absolute path") {
				t.Errorf("error should contain 'absolute path', got: %v", err)
			}
			// Check that the offending path is mentioned
			foundPath := false
			for _, part := range strings.Split(tt.command, " ") {
				// Check direct paths
				if strings.HasPrefix(part, "/") {
					if strings.Contains(err.Error(), part) {
						foundPath = true
						break
					}
				}
				// Check env var assignments like FOO=/etc/passwd
				if strings.Contains(part, "=") {
					parts := strings.SplitN(part, "=", 2)
					if len(parts) == 2 && strings.HasPrefix(parts[1], "/") {
						if strings.Contains(err.Error(), parts[1]) {
							foundPath = true
							break
						}
					}
				}
			}
			if !foundPath {
				t.Errorf("error should mention the offending path, got: %v", err)
			}
		})
	}
}

func TestValidateCommand_AbsolutePathInsideWorkdir(t *testing.T) {
	workdir := t.TempDir()

	tests := []struct {
		name    string
		command string
	}{
		{"file in workdir", fmt.Sprintf("cat %s/file.txt", workdir)},
		{"workdir itself", fmt.Sprintf("ls %s", workdir)},
		{"nested in workdir", fmt.Sprintf("ls %s", filepath.Join(workdir, "sub", "deep"))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCommand(workdir, tt.command)
			if err != nil {
				t.Errorf("expected nil error for command %q, got: %v", tt.command, err)
			}
		})
	}
}

func TestValidateCommand_ParentTraversalEscape(t *testing.T) {
	workdir := t.TempDir()

	tests := []struct {
		name    string
		command string
	}{
		{"double parent traversal", "cat ../../etc/passwd"},
		{"triple parent traversal", "cat ../../../etc/shadow"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCommand(workdir, tt.command)
			if err == nil {
				t.Errorf("expected error for command %q, got nil", tt.command)
				return
			}
			if !strings.Contains(err.Error(), "parent traversal") {
				t.Errorf("error should contain 'parent traversal', got: %v", err)
			}
		})
	}
}

func TestValidateCommand_ParentTraversalInside(t *testing.T) {
	workdir := t.TempDir()

	err := validateCommand(workdir, "cat subdir/../file.txt")
	if err != nil {
		t.Errorf("expected nil error for safe parent traversal, got: %v", err)
	}
}

func TestValidateCommand_TildeExpansion(t *testing.T) {
	workdir := t.TempDir()

	tests := []struct {
		name    string
		command string
	}{
		{"tilde at start", "cat ~/secrets"},
		{"tilde as directory", "ls ~/"},
		{"env var with tilde", "FOO=~/bar cmd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCommand(workdir, tt.command)
			if err == nil {
				t.Errorf("expected error for command %q, got nil", tt.command)
				return
			}
			if !strings.Contains(err.Error(), "home directory") && !strings.Contains(err.Error(), "tilde") {
				t.Errorf("error should contain 'home directory' or 'tilde', got: %v", err)
			}
		})
	}
}

func TestValidateCommand_TildeInFilename(t *testing.T) {
	workdir := t.TempDir()

	tests := []struct {
		name    string
		command string
	}{
		{"file with tilde in name", "cat file~backup"},
		{"echo with tilde", "echo hello~world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCommand(workdir, tt.command)
			if err != nil {
				t.Errorf("expected nil error for command %q, got: %v", tt.command, err)
			}
		})
	}
}

func TestValidateCommand_DoubleDotInFilename(t *testing.T) {
	workdir := t.TempDir()

	err := validateCommand(workdir, "cat file..bak")
	if err != nil {
		t.Errorf("expected nil error for filename with double dots, got: %v", err)
	}
}

func TestValidateCommand_RelativeCommands(t *testing.T) {
	workdir := t.TempDir()

	tests := []struct {
		name    string
		command string
	}{
		{"echo command", "echo hello"},
		{"ls command", "ls"},
		{"cat relative file", "cat file.txt"},
		{"grep recursive", "grep -r pattern ."},
		{"piped commands", "ls | grep foo"},
		{"redirect output", "echo hello > output.txt"},
		{"semicolon commands", "echo a; echo b"},
		{"and operator", "echo a && echo b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCommand(workdir, tt.command)
			if err != nil {
				t.Errorf("expected nil error for command %q, got: %v", tt.command, err)
			}
		})
	}
}

func TestValidateCommand_RootWorkdir(t *testing.T) {
	err := validateCommand("/", "cat /etc/passwd")
	if err != nil {
		t.Errorf("expected nil error for root workdir with absolute path, got: %v", err)
	}
}

func TestValidateCommand_ErrorFormat(t *testing.T) {
	workdir := t.TempDir()

	err := validateCommand(workdir, "cat /etc/passwd")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errStr := err.Error()

	// Check for pattern type (absolute path)
	if !strings.Contains(errStr, "absolute path") {
		t.Errorf("error should contain pattern type 'absolute path', got: %v", errStr)
	}

	// Check for offending token (/etc/passwd)
	if !strings.Contains(errStr, "/etc/passwd") {
		t.Errorf("error should contain offending token '/etc/passwd', got: %v", errStr)
	}

	// Check for workdir restriction message
	if !strings.Contains(errStr, "working directory") {
		t.Errorf("error should contain workdir restriction message, got: %v", errStr)
	}
}

// envContains checks if a specific environment entry exists in the slice
func envContains(env []string, entry string) bool {
	for _, e := range env {
		if e == entry {
			return true
		}
	}
	return false
}

// envHasPrefix checks if any environment entry starts with the given prefix
func envHasPrefix(env []string, prefix string) bool {
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return true
		}
	}
	return false
}

func TestSandboxEnv_HomeSetToWorkdir(t *testing.T) {
	workdir := t.TempDir()
	env := sandboxEnv(workdir)
	if !envContains(env, "HOME="+workdir) {
		t.Errorf("expected HOME=%q in env, got: %v", workdir, env)
	}
}

func TestSandboxEnv_TmpdirSetToWorkdir(t *testing.T) {
	workdir := t.TempDir()
	env := sandboxEnv(workdir)
	if !envContains(env, "TMPDIR="+workdir) {
		t.Errorf("expected TMPDIR=%q in env, got: %v", workdir, env)
	}
}

func TestSandboxEnv_PathInherited(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin")
	workdir := t.TempDir()
	env := sandboxEnv(workdir)
	if !envContains(env, "PATH=/usr/bin:/bin") {
		t.Errorf("expected PATH=/usr/bin:/bin in env, got: %v", env)
	}
}

func TestSandboxEnv_StripsNonAllowlisted(t *testing.T) {
	t.Setenv("SECRET_API_KEY", "leaked")
	t.Setenv("EDITOR", "vim")
	workdir := t.TempDir()
	env := sandboxEnv(workdir)
	if envHasPrefix(env, "SECRET_API_KEY=") {
		t.Errorf("SECRET_API_KEY should not be in env, got: %v", env)
	}
	if envHasPrefix(env, "EDITOR=") {
		t.Errorf("EDITOR should not be in env, got: %v", env)
	}
}

func TestValidateCommand_DevPathAllowed(t *testing.T) {
	workdir := t.TempDir()

	tests := []struct {
		name    string
		command string
	}{
		{"dev zero", "dd if=/dev/zero bs=1024 count=1"},
		{"dev null", "echo hello > /dev/null"},
		{"dev urandom", "head -c 16 /dev/urandom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCommand(workdir, tt.command)
			if err != nil {
				t.Errorf("expected nil error for command %q, got: %v", tt.command, err)
			}
		})
	}
}

func TestValidateCommand_CompoundCommandOneBad(t *testing.T) {
	workdir := t.TempDir()

	tests := []struct {
		name    string
		command string
	}{
		{"semicolon with bad path", "echo ok; cat /etc/passwd"},
		{"and with bad path", "echo ok && cat /etc/passwd"},
		{"pipe with bad path", "echo ok | tee /tmp/out"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCommand(workdir, tt.command)
			if err == nil {
				t.Errorf("expected error for command %q, got nil", tt.command)
			}
		})
	}
}

func TestValidateCommand_QuotedAbsolutePathRejected(t *testing.T) {
	workdir := t.TempDir()

	// Per spec: quoted paths are still detected. This is a known conservative
	// false positive — the path is a string argument, not a file access.
	err := validateCommand(workdir, `echo "/etc/passwd"`)
	if err == nil {
		t.Error("expected error for quoted absolute path (conservative rejection per spec), got nil")
	}
}
