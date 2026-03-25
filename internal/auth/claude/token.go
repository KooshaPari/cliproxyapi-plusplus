// Package claude provides authentication and token management functionality
// for Anthropic's Claude AI services. It handles OAuth2 token storage, serialization,
// and retrieval for maintaining authenticated sessions with the Claude API.
package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/misc"
)

func sanitizeTokenFilePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("token file path is empty")
	}
	return filepath.Clean(trimmed), nil
}

// ClaudeTokenStorage stores OAuth2 token information for Anthropic Claude API authentication.
// It extends the shared BaseTokenStorage with Claude-specific functionality,
// maintaining compatibility with the existing auth system.
type ClaudeTokenStorage struct {
	*auth.BaseTokenStorage
}

// NewClaudeTokenStorage creates a new Claude token storage with the given file path.
//
// Parameters:
//   - filePath: The full path where the token file should be saved/loaded
//
// Returns:
//   - *ClaudeTokenStorage: A new Claude token storage instance
func NewClaudeTokenStorage(filePath string) *ClaudeTokenStorage {
	return &ClaudeTokenStorage{
		BaseTokenStorage: auth.NewBaseTokenStorage(filePath),
	}
}

// SaveTokenToFile serializes the Claude token storage to a JSON file.
// This method wraps the base implementation to provide logging compatibility
// with the existing system.
//
// Parameters:
//   - authFilePath: The full path where the token file should be saved
//
// Returns:
//   - error: An error if the operation fails, nil otherwise
func (ts *ClaudeTokenStorage) SaveTokenToFile(authFilePath string) error {
	ts.Type = "claude"

	safePath, err := sanitizeTokenFilePath(authFilePath)
	if err != nil {
		return fmt.Errorf("invalid token file path: %w", err)
	}

	misc.LogSavingCredentials(safePath)

	// Create directory structure if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(safePath), 0700); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Create the token file
	f, err := os.Create(safePath)
	if err != nil {
		return fmt.Errorf("failed to create token file: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	// Merge metadata using helper
	data, errMerge := misc.MergeMetadata(ts, ts.Metadata)
	if errMerge != nil {
		return fmt.Errorf("failed to merge metadata: %w", errMerge)
	}

	// Encode and write the token data as JSON
	if err = json.NewEncoder(f).Encode(data); err != nil {
		return fmt.Errorf("failed to write token to file: %w", err)
	}
	return nil
}
