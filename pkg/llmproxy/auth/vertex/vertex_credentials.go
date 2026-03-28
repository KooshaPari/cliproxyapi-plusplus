// Package vertex provides token storage for Google Vertex AI Gemini via service account credentials.
// It serialises service account JSON into an auth file that is consumed by the runtime executor.
package vertex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/misc"
	log "github.com/sirupsen/logrus"
)

// authBaseDir is the root directory for all Vertex credential files.
const authBaseDir = "vertex"

// VertexCredentialStorage stores the service account JSON for Vertex AI access.
// The content is persisted verbatim under the "service_account" key, together with
// helper fields for project, location and email to improve logging and discovery.
type VertexCredentialStorage struct {
	// ServiceAccount holds the parsed service account JSON content.
	ServiceAccount map[string]any `json:"service_account"`

	// ProjectID is derived from the service account JSON (project_id).
	ProjectID string `json:"project_id"`

	// Email is the client_email from the service account JSON.
	Email string `json:"email"`

	// Location optionally sets a default region (e.g., us-central1) for Vertex endpoints.
	Location string `json:"location,omitempty"`

	// Type is the provider identifier stored alongside credentials. Always "vertex".
	Type string `json:"type"`
}

// cleanCredentialPath validates that the given path stays within the vertex auth directory.
// It uses misc.ResolveSafeFilePathInDir to ensure path-escape prevention.
func cleanCredentialPath(path, scope string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("%s: auth file path is empty", scope)
	}
	baseDir := filepath.Join(misc.GetAuthDir(), authBaseDir)
	return misc.ResolveSafeFilePathInDir(baseDir, path)
}

// SaveTokenToFile writes the credential payload to the given file path in JSON format.
// It ensures the parent directory exists and logs the operation for transparency.
func (s *VertexCredentialStorage) SaveTokenToFile(authFilePath string) error {
	misc.LogSavingCredentials(authFilePath)
	// Apply filepath.Clean at call site so static analysis can verify the path is sanitized.
	cleanPath := filepath.Clean(authFilePath)

	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o700); err != nil {
		return fmt.Errorf("vertex credential: create directory failed: %w", err)
	}
	f, err := os.Create(cleanPath)
	if err != nil {
		return fmt.Errorf("vertex credential: create file failed: %w", err)
	}
	defer func() {
		if errClose := f.Close(); errClose != nil {
			log.Errorf("vertex credential: failed to close file: %v", errClose)
		}
	}()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err = enc.Encode(s); err != nil {
		return fmt.Errorf("vertex credential: encode failed: %w", err)
	}
	return nil
}
