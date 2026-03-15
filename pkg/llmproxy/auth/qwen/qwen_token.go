// Package qwen provides authentication and token management functionality
// for Alibaba's Qwen AI services. It handles OAuth2 token storage, serialization,
// and retrieval for maintaining authenticated sessions with the Qwen API.
package qwen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/auth/base"
	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/misc"
)

// QwenTokenStorage extends BaseTokenStorage with Qwen-specific fields for managing
// access tokens, refresh tokens, and user account information.
// It embeds auth.BaseTokenStorage to inherit shared token management functionality.
type QwenTokenStorage struct {
	base.BaseTokenStorage

	// LastRefresh is the RFC3339 timestamp of the last successful refresh.
	LastRefresh string `json:"last_refresh,omitempty"`

	// ResourceURL is the base URL for API requests.
	ResourceURL string `json:"resource_url"`

	// Expire is the RFC3339 timestamp when the token expires.
	Expire string `json:"expired,omitempty"`
}

// NewQwenTokenStorage creates a new QwenTokenStorage instance with the given file path.
// Parameters:
//   - filePath: The full path where the token file should be saved/loaded
//
// Returns:
//   - *QwenTokenStorage: A new QwenTokenStorage instance
func NewQwenTokenStorage(filePath string) *QwenTokenStorage {
	return &QwenTokenStorage{
		BaseTokenStorage: base.BaseTokenStorage{FilePath: filePath},
	}
}

// SaveTokenToFile serializes the Qwen token storage to a JSON file.
// This method creates the necessary directory structure and writes the token
// data in JSON format to the specified file path for persistent storage.
//
// Parameters:
//   - authFilePath: The full path where the token file should be saved
//
// Returns:
//   - error: An error if the operation fails, nil otherwise
func (ts *QwenTokenStorage) SaveTokenToFile(authFilePath string) error {
	misc.LogSavingCredentials(authFilePath)
	if _, err := cleanTokenFilePath(authFilePath, "qwen token"); err != nil {
		return err
	}

	ts.BaseTokenStorage.Type = "qwen"
	return ts.BaseTokenStorage.Save(authFilePath, ts)
}

func cleanTokenFilePath(path, scope string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("%s: auth file path is empty", scope)
	}
	clean := filepath.Clean(filepath.FromSlash(trimmed))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("%s: auth file path is invalid", scope)
	}
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("%s: resolve auth file path: %w", scope, err)
	}
	return filepath.Clean(abs), nil
}
