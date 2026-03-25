// Package gemini provides authentication and token management functionality
// for Google's Gemini AI services. It handles OAuth2 token storage, serialization,
// and retrieval for maintaining authenticated sessions with the Gemini API.
package gemini

import (
<<<<<<< HEAD
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/pkg/llmproxy/misc"
	log "github.com/sirupsen/logrus"
=======
	"fmt"
	"strings"

	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/auth/base"
>>>>>>> origin/main
)

// GeminiTokenStorage stores OAuth2 token information for Google Gemini API authentication.
// It maintains compatibility with the existing auth system while adding Gemini-specific fields
// for managing access tokens, refresh tokens, and user account information.
<<<<<<< HEAD
type GeminiTokenStorage struct {
=======
//
// Note: Gemini wraps its raw OAuth2 token inside the Token field (type any) rather than
// storing access/refresh tokens as top-level strings, so BaseTokenStorage.AccessToken and
// BaseTokenStorage.RefreshToken remain empty for this provider.
type GeminiTokenStorage struct {
	base.BaseTokenStorage

>>>>>>> origin/main
	// Token holds the raw OAuth2 token data, including access and refresh tokens.
	Token any `json:"token"`

	// ProjectID is the Google Cloud Project ID associated with this token.
	ProjectID string `json:"project_id"`

<<<<<<< HEAD
	// Email is the email address of the authenticated user.
	Email string `json:"email"`

=======
>>>>>>> origin/main
	// Auto indicates if the project ID was automatically selected.
	Auto bool `json:"auto"`

	// Checked indicates if the associated Cloud AI API has been verified as enabled.
	Checked bool `json:"checked"`
<<<<<<< HEAD

	// Type indicates the authentication provider type, always "gemini" for this storage.
	Type string `json:"type"`
=======
>>>>>>> origin/main
}

// SaveTokenToFile serializes the Gemini token storage to a JSON file.
// This method creates the necessary directory structure and writes the token
// data in JSON format to the specified file path for persistent storage.
//
// Parameters:
//   - authFilePath: The full path where the token file should be saved
//
// Returns:
//   - error: An error if the operation fails, nil otherwise
func (ts *GeminiTokenStorage) SaveTokenToFile(authFilePath string) error {
<<<<<<< HEAD
	safePath, err := misc.ResolveSafeFilePath(authFilePath)
	if err != nil {
		return fmt.Errorf("invalid token file path: %w", err)
	}
	misc.LogSavingCredentials(safePath)
	ts.Type = "gemini"
	if err = os.MkdirAll(filepath.Dir(safePath), 0700); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	f, err := os.Create(safePath)
	if err != nil {
		return fmt.Errorf("failed to create token file: %w", err)
	}
	defer func() {
		if errClose := f.Close(); errClose != nil {
			log.Errorf("failed to close file: %v", errClose)
		}
	}()

	if err = json.NewEncoder(f).Encode(ts); err != nil {
		return fmt.Errorf("failed to write token to file: %w", err)
=======
	ts.Type = "gemini"
	if err := ts.Save(authFilePath, ts); err != nil {
		return fmt.Errorf("gemini token: %w", err)
>>>>>>> origin/main
	}
	return nil
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
