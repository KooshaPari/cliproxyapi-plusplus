// Package qwen provides authentication and token management functionality
// for Alibaba's Qwen AI services. It handles OAuth2 token storage, serialization,
// and retrieval for maintaining authenticated sessions with the Qwen API.
package qwen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/misc"
)

// BaseTokenStorage provides common token storage functionality shared across providers.
type BaseTokenStorage struct {
	FilePath     string `json:"-"`
	Type         string `json:"type"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token,omitempty"`
	LastRefresh  string `json:"last_refresh,omitempty"`
	Expire       string `json:"expired,omitempty"`
}

// NewBaseTokenStorage creates a new BaseTokenStorage with the given file path.
func NewBaseTokenStorage(filePath string) *BaseTokenStorage {
	return &BaseTokenStorage{FilePath: filePath}
}

// Save writes the token storage to its file path as JSON.
func (b *BaseTokenStorage) Save() error {
	if b.FilePath == "" {
		return fmt.Errorf("base token storage: file path is empty")
	}
	cleanPath := filepath.Clean(b.FilePath)
	dir := filepath.Dir(cleanPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	f, err := os.Create(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to create token file: %w", err)
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(b); err != nil {
		return fmt.Errorf("failed to write token to file: %w", err)
	}
	return nil
}

// QwenTokenStorage extends BaseTokenStorage with Qwen-specific fields for managing
// access tokens, refresh tokens, and user account information.
type QwenTokenStorage struct {
	*BaseTokenStorage

	// ResourceURL is the base URL for API requests.
	ResourceURL string `json:"resource_url"`

	// Email is the account email address associated with this token.
	Email string `json:"email"`
}

// NewQwenTokenStorage creates a new QwenTokenStorage instance with the given file path.
func NewQwenTokenStorage(filePath string) *QwenTokenStorage {
	return &QwenTokenStorage{
		BaseTokenStorage: NewBaseTokenStorage(filePath),
	}
}

// SaveTokenToFile serializes the Qwen token storage to a JSON file.
func (ts *QwenTokenStorage) SaveTokenToFile(authFilePath string) error {
	misc.LogSavingCredentials(authFilePath)
	if ts.BaseTokenStorage == nil {
		return fmt.Errorf("qwen token: base token storage is nil")
	}

	cleaned, err := cleanTokenFilePath(authFilePath, "qwen token")
	if err != nil {
		return err
	}

	ts.BaseTokenStorage.FilePath = cleaned
	ts.BaseTokenStorage.Type = "qwen"
	return ts.BaseTokenStorage.Save()
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
