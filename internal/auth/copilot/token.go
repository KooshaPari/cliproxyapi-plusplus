// Package copilot provides authentication and token management functionality
// for GitHub Copilot AI services. It handles OAuth2 device flow token storage,
// serialization, and retrieval for maintaining authenticated sessions with the Copilot API.
package copilot

import (
	"github.com/KooshaPari/phenotype-go-kit/pkg/auth"
	"github.com/kooshapari/cliproxyapi-plusplus/v6/internal/misc"
)

// CopilotTokenStorage stores OAuth2 token information for GitHub Copilot API authentication.
// It extends the shared BaseTokenStorage with Copilot-specific fields for managing
// GitHub user profile information.
type CopilotTokenStorage struct {
	*auth.BaseTokenStorage

	// TokenType is the type of token, typically "bearer".
	TokenType string `json:"token_type"`
	// Scope is the OAuth2 scope granted to the token.
	Scope string `json:"scope"`
	// ExpiresAt is the timestamp when the access token expires (if provided).
	ExpiresAt string `json:"expires_at,omitempty"`
	// Username is the GitHub username associated with this token.
	Username string `json:"username"`
	// Name is the GitHub display name associated with this token.
	Name string `json:"name,omitempty"`
}

// NewCopilotTokenStorage creates a new Copilot token storage with the given file path.
//
// Parameters:
//   - filePath: The full path where the token file should be saved/loaded
//
// Returns:
//   - *CopilotTokenStorage: A new Copilot token storage instance
func NewCopilotTokenStorage(filePath string) *CopilotTokenStorage {
	return &CopilotTokenStorage{
		BaseTokenStorage: auth.NewBaseTokenStorage(filePath),
	}
}

// CopilotTokenData holds the raw OAuth token response from GitHub.
type CopilotTokenData struct {
	// AccessToken is the OAuth2 access token.
	AccessToken string `json:"access_token"`
	// TokenType is the type of token, typically "bearer".
	TokenType string `json:"token_type"`
	// Scope is the OAuth2 scope granted to the token.
	Scope string `json:"scope"`
}

// CopilotAuthBundle bundles authentication data for storage.
type CopilotAuthBundle struct {
	// TokenData contains the OAuth token information.
	TokenData *CopilotTokenData
	// Username is the GitHub username.
	Username string
	// Email is the GitHub email address.
	Email string
	// Name is the GitHub display name.
	Name string
}

// DeviceCodeResponse represents GitHub's device code response.
type DeviceCodeResponse struct {
	// DeviceCode is the device verification code.
	DeviceCode string `json:"device_code"`
	// UserCode is the code the user must enter at the verification URI.
	UserCode string `json:"user_code"`
	// VerificationURI is the URL where the user should enter the code.
	VerificationURI string `json:"verification_uri"`
	// ExpiresIn is the number of seconds until the device code expires.
	ExpiresIn int `json:"expires_in"`
	// Interval is the minimum number of seconds to wait between polling requests.
	Interval int `json:"interval"`
}

// SaveTokenToFile serializes the Copilot token storage to a JSON file.
// This method wraps the base implementation to provide logging compatibility
// with the existing system.
//
// Parameters:
//   - authFilePath: The full path where the token file should be saved
//
// Returns:
//   - error: An error if the operation fails, nil otherwise
func (ts *CopilotTokenStorage) SaveTokenToFile(authFilePath string) error {
	misc.LogSavingCredentials(authFilePath)
	ts.Type = "github-copilot"

	// Create a new token storage with the file path and copy the fields
	base := auth.NewBaseTokenStorage(authFilePath)
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
