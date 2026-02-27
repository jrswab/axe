package xdg

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetConfigDir_WithXDGEnvSet(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	got, err := GetConfigDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := filepath.Join(tmpDir, "axe")
	if got != want {
		t.Errorf("GetConfigDir() = %q, want %q", got, want)
	}
}

func TestGetConfigDir_WithoutXDGEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	got, err := GetConfigDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// os.UserConfigDir() returns platform-specific defaults
	userCfg, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("os.UserConfigDir() failed: %v", err)
	}

	want := filepath.Join(userCfg, "axe")
	if got != want {
		t.Errorf("GetConfigDir() = %q, want %q", got, want)
	}
}

func TestGetConfigDir_UsesFilepathJoin(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	got, err := GetConfigDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the separator is OS-appropriate
	if runtime.GOOS == "windows" {
		if filepath.Separator != '\\' {
			t.Skip("not on windows")
		}
	}

	// The path should use filepath.Join, which uses the OS separator
	if got != filepath.Join(tmpDir, "axe") {
		t.Errorf("path does not use OS-appropriate separators: %q", got)
	}
}

func TestGetConfigDir_Deterministic(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	first, err := GetConfigDir()
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}

	second, err := GetConfigDir()
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}

	if first != second {
		t.Errorf("GetConfigDir() not deterministic: first=%q, second=%q", first, second)
	}
}
