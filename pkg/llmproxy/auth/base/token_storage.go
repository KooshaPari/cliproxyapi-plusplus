// Package base provides a shared foundation for OAuth2 token storage across all
// LLM proxy authentication providers. It centralises the common Save/Load/Clear
// file-I/O operations so that individual provider packages only need to embed
// BaseTokenStorage and add their own provider-specific fields.
package base

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/misc"
)

// BaseTokenStorage holds the fields and file-I/O methods that every provider
// token struct shares.  Provider-specific structs embed a *BaseTokenStorage
// (or a copy by value) and extend it with their own fields.
type BaseTokenStorage struct {
	// AccessToken is the OAuth2 bearer token used to authenticate API requests.
	AccessToken string `json:"access_token"`

	// RefreshToken is used to obtain a new access token when the current one expires.
	RefreshToken string `json:"refresh_token,omitempty"`

	// Email is the account e-mail address associated with this token.
	Email string `json:"email,omitempty"`

	// Type is the provider identifier (e.g. "claude", "codex", "kimi").
	// Each provider sets this before saving so that callers can identify
	// which authentication provider a credential file belongs to.
	Type string `json:"type"`

	// LastRefresh is the timestamp of the last token refresh operation.
	LastRefresh string `json:"last_refresh,omitempty"`

	// Expire is the timestamp when the current access token expires.
	Expire string `json:"expired,omitempty"`

	// FilePath is the on-disk path used by Save/Load/Clear.  It is not
	// serialised to JSON; it is populated at runtime from the caller-supplied
	// authFilePath argument.
	FilePath string `json:"-"`
}

// NewBaseTokenStorage creates a new BaseTokenStorage with the given file path.
func NewBaseTokenStorage(filePath string) *BaseTokenStorage {
	return &BaseTokenStorage{FilePath: filePath}
}

// GetAccessToken returns the OAuth2 access token.
func (b *BaseTokenStorage) GetAccessToken() string { return b.AccessToken }

// GetRefreshToken returns the OAuth2 refresh token.
func (b *BaseTokenStorage) GetRefreshToken() string { return b.RefreshToken }

// GetEmail returns the e-mail address associated with the token.
func (b *BaseTokenStorage) GetEmail() string { return b.Email }

// GetType returns the provider type string.
func (b *BaseTokenStorage) GetType() string { return b.Type }

// Save serialises v (the outer provider struct that embeds BaseTokenStorage)
// to the file at authFilePath using an atomic write (write to a temp file,
// then rename).  The directory is created if it does not already exist.
//
// v must be JSON-marshallable.  Passing the provider struct rather than
// BaseTokenStorage itself ensures that all provider-specific fields are
// persisted alongside the base fields.
func (b *BaseTokenStorage) Save(authFilePath string, v any) error {
	safePath, err := misc.ResolveSafeFilePath(authFilePath)
	if err != nil {
		return fmt.Errorf("base token storage: invalid file path: %w", err)
	}
	misc.LogSavingCredentials(safePath)

	if err = os.MkdirAll(filepath.Dir(safePath), 0o700); err != nil {
		return fmt.Errorf("base token storage: create directory: %w", err)
	}

	// Write to a temporary file in the same directory, then rename so that
	// a concurrent reader never observes a partially-written file.
	tmpFile, err := os.CreateTemp(filepath.Dir(safePath), ".tmp-token-*")
	if err != nil {
		return fmt.Errorf("base token storage: create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	writeErr := json.NewEncoder(tmpFile).Encode(v)
	closeErr := tmpFile.Close()

	if writeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("base token storage: encode token: %w", writeErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("base token storage: close temp file: %w", closeErr)
	}

	if err = os.Rename(tmpPath, safePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("base token storage: rename temp file: %w", err)
	}
	return nil
}

// Load reads the JSON file at authFilePath and unmarshals it into v.
// v should be a pointer to the outer provider struct so that all fields
// are populated.
func (b *BaseTokenStorage) Load(authFilePath string, v any) error {
	safePath, err := misc.ResolveSafeFilePath(authFilePath)
	if err != nil {
		return fmt.Errorf("base token storage: invalid file path: %w", err)
	}

	data, err := os.ReadFile(safePath)
	if err != nil {
		return fmt.Errorf("base token storage: read token file: %w", err)
	}

	if err = json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("base token storage: unmarshal token: %w", err)
	}
	return nil
}

// Clear removes the token file at authFilePath.  It returns nil if the file
// does not exist (idempotent delete).
func (b *BaseTokenStorage) Clear(authFilePath string) error {
	safePath, err := misc.ResolveSafeFilePath(authFilePath)
	if err != nil {
		return fmt.Errorf("base token storage: invalid file path: %w", err)
	}

	if err = os.Remove(safePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("base token storage: remove token file: %w", err)
	}
	return nil
}
