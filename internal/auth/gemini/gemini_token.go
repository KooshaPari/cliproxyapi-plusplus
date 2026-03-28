// Package gemini provides authentication and token management functionality
// for Google's Gemini AI services. It handles OAuth2 token storage, serialization.
package gemini

import (
	"fmt"
	"strings"

	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/auth/base"
)

// GeminiTokenStorage stores OAuth2 token information for Google Gemini API authentication.
// It embeds the shared BaseTokenStorage with Gemini-specific fields.
type GeminiTokenStorage struct {
	base.BaseTokenStorage

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
func NewGeminiTokenStorage(filePath string) *GeminiTokenStorage {
	return &GeminiTokenStorage{}
}

// SaveTokenToFile serializes the Gemini token storage to a JSON file.
func (ts *GeminiTokenStorage) SaveTokenToFile(authFilePath string) error {
	ts.Type = "gemini"
	return ts.BaseTokenStorage.Save(authFilePath, ts)
}

// CredentialFileName returns the filename used to persist Gemini CLI credentials.
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
