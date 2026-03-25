// Package gemini provides authentication and token management functionality
// for Google's Gemini AI services. It handles OAuth2 token storage, serialization,
// and retrieval for maintaining authenticated sessions with the Gemini API.
package gemini

import (
	"fmt"
	"strings"

	"github.com/kooshapari/cliproxyapi-plusplus/v6/internal/misc"
	log "github.com/sirupsen/logrus"
)

// GeminiTokenStorage stores OAuth2 token information for Google Gemini API authentication.
// It extends the shared BaseTokenStorage with Gemini-specific fields for managing
// Google Cloud Project information.
type GeminiTokenStorage struct {
	*base.BaseTokenStorage

	// Token holds the raw OAuth2 token data, including access and refresh tokens.
	Token any `json:"token"`

	// ProjectID is the Google Cloud Project ID associated with this token.
	ProjectID string `json:"project_id"`

	// Auto indicates if the project ID was automatically selected.
	Auto bool `json:"auto"`

	// Checked indicates if the associated Cloud AI API has been verified as enabled.
	Checked bool `json:"checked"`
}

// NewGeminiTokenStorage creates a new Gemini token storage with the given file path.
//
// Parameters:
//   - filePath: The full path where the token file should be saved/loaded
//
// Returns:
//   - *GeminiTokenStorage: A new Gemini token storage instance
func NewGeminiTokenStorage(filePath string) *GeminiTokenStorage {
	return &GeminiTokenStorage{
		BaseTokenStorage: base.NewBaseTokenStorage(filePath),
	}
}

// SaveTokenToFile serializes the Gemini token storage to a JSON file.
// This method wraps the base implementation to provide logging compatibility
// with the existing system.
//
// Parameters:
//   - authFilePath: The full path where the token file should be saved
//
// Returns:
//   - error: An error if the operation fails, nil otherwise
func (ts *GeminiTokenStorage) SaveTokenToFile(authFilePath string) error {
	misc.LogSavingCredentials(authFilePath)
	ts.Type = "gemini"

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

// CredentialFileName returns the filename used to persist Gemini CLI credentials.
// When projectID represents multiple projects (comma-separated or literal ALL),
// the suffix is normalized to "all" and a "gemini-" prefix is enforced to keep
// web and CLI generated files consistent.
func CredentialFileName(email, projectID string, includeProviderPrefix bool) string {
	email = strings.TrimSpace(email)
	project := strings.TrimSpace(projectID)
	if strings.EqualFold(project, "all") || strings.Contains(project, ",") {
		return fmt.Sprintf("gemini-%s-all.json", email)
	}
	prefix := ""
	if includeProviderPrefix {
		prefix = "gemini-"
	}
	return fmt.Sprintf("%s%s-%s.json", prefix, email, project)
}
