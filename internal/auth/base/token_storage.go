// Package base provides shared token storage types for provider auth modules.
// This package was inlined from phenotype-go-kit/pkg/auth to remove the
// external dependency on a local-only module that does not exist in CI.
package base

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// BaseTokenStorage holds common OAuth2 token fields shared across providers.
type BaseTokenStorage struct {
	// FilePath is the path where the token file is stored.
	FilePath string `json:"-"`

	// IDToken is the OIDC ID token (if applicable).
	IDToken string `json:"id_token,omitempty"`
	// AccessToken is the OAuth2 access token.
	AccessToken string `json:"access_token"`
	// RefreshToken is the OAuth2 refresh token.
	RefreshToken string `json:"refresh_token,omitempty"`
	// LastRefresh is the timestamp of the last token refresh.
	LastRefresh time.Time `json:"last_refresh,omitempty"`
	// Email is the email associated with the token.
	Email string `json:"email,omitempty"`
	// Type identifies the provider type.
	Type string `json:"type,omitempty"`
	// Expire is the token expiration timestamp.
	Expire time.Time `json:"expire,omitempty"`
	// Metadata holds arbitrary provider-specific key-value data.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// NewBaseTokenStorage creates a new BaseTokenStorage with the given file path.
func NewBaseTokenStorage(filePath string) *BaseTokenStorage {
	return &BaseTokenStorage{
		FilePath: filePath,
		Metadata: make(map[string]any),
	}
}

// SetMetadata replaces the metadata map.
func (b *BaseTokenStorage) SetMetadata(m map[string]any) {
	if m == nil {
		b.Metadata = make(map[string]any)
		return
	}
	b.Metadata = m
}

// Save serializes the token storage to its configured file path as JSON.
func (b *BaseTokenStorage) Save() error {
	dir := filepath.Dir(b.FilePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(b.FilePath, data, 0o600)
}
