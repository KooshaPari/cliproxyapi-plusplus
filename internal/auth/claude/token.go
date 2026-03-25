// Package claude provides authentication and token management functionality
// for Anthropic's Claude AI services. It handles OAuth2 token storage, serialization,
// and retrieval for maintaining authenticated sessions with the Claude API.
package claude

import (
<<<<<<< HEAD
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

=======
	"github.com/KooshaPari/phenotype-go-auth"
>>>>>>> origin/main
	"github.com/kooshapari/cliproxyapi-plusplus/v6/internal/misc"
)

// ClaudeTokenStorage stores OAuth2 token information for Anthropic Claude API authentication.
<<<<<<< HEAD
// It maintains compatibility with the existing auth system while adding Claude-specific fields
// for managing access tokens, refresh tokens, and user account information.
type ClaudeTokenStorage struct {
	// IDToken is the JWT ID token containing user claims and identity information.
	IDToken string `json:"id_token"`

	// AccessToken is the OAuth2 access token used for authenticating API requests.
	AccessToken string `json:"access_token"`

	// RefreshToken is used to obtain new access tokens when the current one expires.
	RefreshToken string `json:"refresh_token"`

	// LastRefresh is the timestamp of the last token refresh operation.
	LastRefresh string `json:"last_refresh"`

	// Email is the Anthropic account email address associated with this token.
	Email string `json:"email"`

	// Type indicates the authentication provider type, always "claude" for this storage.
	Type string `json:"type"`

	// Expire is the timestamp when the current access token expires.
	Expire string `json:"expired"`

	// Metadata holds arbitrary key-value pairs injected via hooks.
	// It is not exported to JSON directly to allow flattening during serialization.
	Metadata map[string]any `json:"-"`
}

// SetMetadata allows external callers to inject metadata into the storage before saving.
func (ts *ClaudeTokenStorage) SetMetadata(meta map[string]any) {
	ts.Metadata = meta
}

// SaveTokenToFile serializes the Claude token storage to a JSON file.
// This method creates the necessary directory structure and writes the token
// data in JSON format to the specified file path for persistent storage.
// It merges any injected metadata into the top-level JSON object.
=======
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
>>>>>>> origin/main
//
// Parameters:
//   - authFilePath: The full path where the token file should be saved
//
// Returns:
//   - error: An error if the operation fails, nil otherwise
func (ts *ClaudeTokenStorage) SaveTokenToFile(authFilePath string) error {
	misc.LogSavingCredentials(authFilePath)
	ts.Type = "claude"

<<<<<<< HEAD
	// Create directory structure if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(authFilePath), 0700); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Create the token file
	f, err := os.Create(authFilePath)
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
=======
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
>>>>>>> origin/main
}
