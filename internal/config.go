// Package internal holds shared application paths.
package internal

import (
	"fmt"
	"os"
	"path/filepath"
)

// ConfigDir is the shared directory for config.yml, credentials.json, and sessions.
// "." uses the working directory for local development. Other relative values,
// such as ".justsay", are resolved under the user's home directory.
// Absolute values are used directly.
const ConfigDir = ".justsay"

func ConfigPath(name string) (string, error) {
	return configPath(ConfigDir, name)
}

func configPath(dir, name string) (string, error) {
	if dir == "." || filepath.IsAbs(dir) {
		return filepath.Join(dir, name), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve configuration home directory: %w", err)
	}
	return filepath.Join(home, dir, name), nil
}
