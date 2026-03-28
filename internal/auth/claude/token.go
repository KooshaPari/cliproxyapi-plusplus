// Package claude provides authentication and token management functionality
// for Anthropic's Claude AI services. It handles OAuth2 token storage, serialization,
// and retrieval for maintaining authenticated sessions with the Claude API.
package claude

import (
	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/auth/base"
)

// ClaudeTokenStorage stores OAuth2 token information for Anthropic Claude API authentication.
// It embeds the shared BaseTokenStorage with Claude-specific functionality.
type ClaudeTokenStorage struct {
	base.BaseTokenStorage
}

// NewClaudeTokenStorage creates a new Claude token storage with the given file path.
func NewClaudeTokenStorage(filePath string) *ClaudeTokenStorage {
	return &ClaudeTokenStorage{}
}

// SaveTokenToFile serializes the Claude token storage to a JSON file.
func (ts *ClaudeTokenStorage) SaveTokenToFile(authFilePath string) error {
	ts.Type = "claude"
	return ts.BaseTokenStorage.Save(authFilePath, ts)
}
