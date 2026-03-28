// Package misc provides miscellaneous utilities for the llmproxy package.
package misc

import (
	"os"
	"path/filepath"
)

// LogLoadingCredentials logs a message when loading credentials.
// This is a placeholder to satisfy interface expectations.
func LogLoadingCredentials(path string) {}

// GetAuthDir returns the configured auth directory or default location.
func GetAuthDir() string {
	if dir := os.Getenv("CLIPROXY_AUTH_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cliproxy"
	}
	return filepath.Join(home, ".cliproxy", "auth")
}

// LogSavingCredentials logs a message when saving credentials.
// This is a placeholder to satisfy interface expectations.
func LogSavingCredentials(path string) {}

// LogCredentialSeparator is used for logging credential separators.
func LogCredentialSeparator() {}
