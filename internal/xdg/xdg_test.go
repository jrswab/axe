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

func TestGetDataDir_XDGDataHomeSet(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	got, err := GetDataDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := filepath.Join(tmpDir, "axe")
	if got != want {
		t.Errorf("GetDataDir() = %q, want %q", got, want)
	}
}

func TestGetDataDir_XDGDataHomeEmpty(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")

	got, err := GetDataDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() failed: %v", err)
	}

	want := filepath.Join(homeDir, ".local", "share", "axe")
	if got != want {
		t.Errorf("GetDataDir() = %q, want %q", got, want)
	}
}

func TestGetDataDir_XDGDataHomeUnset(t *testing.T) {
	// Ensure XDG_DATA_HOME is not set
	t.Setenv("XDG_DATA_HOME", "")
	_ = os.Unsetenv("XDG_DATA_HOME")

	got, err := GetDataDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() failed: %v", err)
	}

	want := filepath.Join(homeDir, ".local", "share", "axe")
	if got != want {
		t.Errorf("GetDataDir() = %q, want %q", got, want)
	}
}

func TestGetDataDir_DoesNotCreateDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	got, err := GetDataDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The returned path should NOT exist on disk
	if _, err := os.Stat(got); err == nil {
		t.Errorf("GetDataDir() created directory %q, but should not create it", got)
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking path: %v", err)
	}
}

func TestGetCacheDir(t *testing.T) {
	tests := []struct {
		name       string
		envXDG     string
		unsetXDG   bool
		wantSuffix string
	}{
		{
			name:       "XDG_CACHE_HOME set",
			envXDG:     "",
			wantSuffix: "axe",
		},
		{
			name:       "XDG_CACHE_HOME empty string",
			envXDG:     "",
			wantSuffix: filepath.Join(".cache", "axe"),
		},
		{
			name:       "XDG_CACHE_HOME unset",
			envXDG:     "",
			unsetXDG:   true,
			wantSuffix: filepath.Join(".cache", "axe"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "XDG_CACHE_HOME set" {
				tmpDir := t.TempDir()
				t.Setenv("XDG_CACHE_HOME", tmpDir)
				got, err := GetCacheDir()
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				want := filepath.Join(tmpDir, "axe")
				if got != want {
					t.Errorf("GetCacheDir() = %q, want %q", got, want)
				}
				return
			}

			if tt.unsetXDG {
				t.Setenv("XDG_CACHE_HOME", "")
			} else {
				t.Setenv("XDG_CACHE_HOME", tt.envXDG)
			}

			homeDir, err := os.UserHomeDir()
			if err != nil {
				t.Fatalf("os.UserHomeDir() failed: %v", err)
			}

			got, err := GetCacheDir()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			want := filepath.Join(homeDir, tt.wantSuffix)
			if got != want {
				t.Errorf("GetCacheDir() = %q, want %q", got, want)
			}
		})
	}
}

func TestGetCacheDir_DoesNotCreateDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	got, err := GetCacheDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The returned path should NOT exist on disk
	if _, err := os.Stat(got); err == nil {
		t.Errorf("GetCacheDir() created directory %q, but should not create it", got)
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking path: %v", err)
	}
}
