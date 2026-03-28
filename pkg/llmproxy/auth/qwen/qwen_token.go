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
)

// QwenTokenStorage stores OAuth2 token information for Alibaba Qwen API authentication.
// It maintains compatibility with the existing auth system while adding Qwen-specific fields
// for managing access tokens, refresh tokens, and user account information.
type QwenTokenStorage struct {
	base.BaseTokenStorage

	// LastRefresh is the timestamp of the last token refresh operation.
	LastRefresh string `json:"last_refresh"`
	// ResourceURL is the base URL for API requests.
	ResourceURL string `json:"resource_url"`
	// Expire is the timestamp when the current access token expires.
	Expire string `json:"expired"`
}

// NewQwenTokenStorage creates a new QwenTokenStorage instance with the given file path.
func NewQwenTokenStorage(filePath string) *QwenTokenStorage {
	return &QwenTokenStorage{
		BaseTokenStorage: base.BaseTokenStorage{FilePath: filePath},
	}
}

// SaveTokenToFile serializes the Qwen token storage to a JSON file.
func (ts *QwenTokenStorage) SaveTokenToFile(authFilePath string) error {
	if ts == nil {
		return fmt.Errorf("qwen token: storage is nil")
	}
	if _, err := cleanTokenFilePath(authFilePath, "qwen token"); err != nil {
		return err
	}
	ts.Type = "qwen"
	if err := ts.Save(authFilePath, ts); err != nil {
		return fmt.Errorf("qwen token: %w", err)
	}
	return nil
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
