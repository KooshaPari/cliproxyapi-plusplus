// Package copilot provides authentication and token management functionality
// Package copilot provides authentication and token management functionality
// for GitHub Copilot AI services. It handles OAuth2 device flow token storage.
package copilot

import (
	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/auth/base"
)

// CopilotTokenStorage stores OAuth2 token information for GitHub Copilot API authentication.
// It embeds the shared BaseTokenStorage with Copilot-specific fields.
type CopilotTokenStorage struct {
	base.BaseTokenStorage

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
func NewCopilotTokenStorage(filePath string) *CopilotTokenStorage {
	return &CopilotTokenStorage{}
}

// CopilotTokenData holds the raw OAuth token response from GitHub.
type CopilotTokenData struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

// CopilotAuthBundle bundles authentication data for storage.
type CopilotAuthBundle struct {
	TokenData *CopilotTokenData
	Username  string
	Email     string
	Name      string
}

// DeviceCodeResponse represents GitHub's device code response.
type DeviceCodeResponse struct {
	DeviceCode       string `json:"device_code"`
	UserCode         string `json:"user_code"`
	VerificationURI  string `json:"verification_uri"`
	ExpiresIn        int    `json:"expires_in"`
	Interval         int    `json:"interval"`
}

// SaveTokenToFile serializes the Copilot token storage to a JSON file.
func (ts *CopilotTokenStorage) SaveTokenToFile(authFilePath string) error {
	ts.Type = "github-copilot"
	return ts.BaseTokenStorage.Save(authFilePath, ts)
}
