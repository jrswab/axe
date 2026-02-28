package xdg

import (
	"fmt"
	"os"
	"path/filepath"
)

// GetDataDir returns the Axe data directory path following the XDG Base
// Directory specification. It resolves to $XDG_DATA_HOME/axe if the
// environment variable is set and non-empty. Otherwise it falls back to
// $HOME/.local/share/axe. It does NOT create the directory.
func GetDataDir() (string, error) {
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		return filepath.Join(xdgData, "axe"), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("unable to determine data directory: %w", err)
	}

	return filepath.Join(homeDir, ".local", "share", "axe"), nil
}

// GetConfigDir returns the Axe configuration directory path following
// the XDG Base Directory specification. It resolves to $XDG_CONFIG_HOME/axe
// if the environment variable is set, otherwise falls back to
// os.UserConfigDir()/axe.
func GetConfigDir() (string, error) {
	if xdgHome := os.Getenv("XDG_CONFIG_HOME"); xdgHome != "" {
		return filepath.Join(xdgHome, "axe"), nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("unable to determine config directory: %w", err)
	}

	return filepath.Join(configDir, "axe"), nil
}
