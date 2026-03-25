// Package claude provides authentication and token management functionality
// for Anthropic's Claude AI services. It handles OAuth2 token storage, serialization,
// and retrieval for maintaining authenticated sessions with the Claude API.
package claude

import (
	"github.com/kooshapari/cliproxyapi-plusplus/v6/internal/auth/base"
	"github.com/kooshapari/cliproxyapi-plusplus/v6/internal/misc"
)

// ClaudeTokenStorage stores OAuth2 token information for Anthropic Claude API authentication.
// It extends the shared BaseTokenStorage with Claude-specific functionality,
// maintaining compatibility with the existing auth system.
type ClaudeTokenStorage struct {
	*base.BaseTokenStorage
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
		BaseTokenStorage: base.NewBaseTokenStorage(filePath),
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
	misc.LogSavingCredentials(authFilePath)
	ts.Type = "claude"

	// Create a new token storage with the file path and copy the fields
	base := base.NewBaseTokenStorage(authFilePath)
	base.IDToken = ts.IDToken
	base.AccessToken = ts.AccessToken
	base.RefreshToken = ts.RefreshToken
	base.LastRefresh = ts.LastRefresh
	base.Email = ts.Email
	base.Type = ts.Type
	base.Expire = ts.Expire
	base.SetMetadata(ts.Metadata)

	return base.Save()
}
