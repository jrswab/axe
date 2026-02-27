package xdg

import (
	"fmt"
	"os"
	"path/filepath"
)

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
