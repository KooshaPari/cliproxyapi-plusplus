// Package kilo provides authentication and token management functionality
// for Kilo AI services.
package kilo

import (
<<<<<<< HEAD
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/misc"
	log "github.com/sirupsen/logrus"
)

// KiloTokenStorage stores token information for Kilo AI authentication.
type KiloTokenStorage struct {
	// Token is the Kilo access token.
=======
	"fmt"

	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/auth/base"
)

// KiloTokenStorage stores token information for Kilo AI authentication.
//
// Note: Kilo uses a proprietary token format stored under the "kilocodeToken" JSON key
// rather than the standard "access_token" key, so BaseTokenStorage.AccessToken is not
// populated for this provider.  The Email and Type fields from BaseTokenStorage are used.
type KiloTokenStorage struct {
	base.BaseTokenStorage

	// Token is the Kilo access token (serialised as "kilocodeToken" for Kilo compatibility).
>>>>>>> origin/main
	Token string `json:"kilocodeToken"`

	// OrganizationID is the Kilo organization ID.
	OrganizationID string `json:"kilocodeOrganizationId"`

	// Model is the default model to use.
	Model string `json:"kilocodeModel"`
<<<<<<< HEAD

	// Email is the email address of the authenticated user.
	Email string `json:"email"`

	// Type indicates the authentication provider type, always "kilo" for this storage.
	Type string `json:"type"`
=======
>>>>>>> origin/main
}

// SaveTokenToFile serializes the Kilo token storage to a JSON file.
func (ts *KiloTokenStorage) SaveTokenToFile(authFilePath string) error {
<<<<<<< HEAD
	safePath, err := misc.ResolveSafeFilePath(authFilePath)
	if err != nil {
		return fmt.Errorf("invalid token file path: %w", err)
	}
	misc.LogSavingCredentials(safePath)
	ts.Type = "kilo"
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
	ts.Type = "kilo"
	if err := ts.Save(authFilePath, ts); err != nil {
		return fmt.Errorf("kilo token: %w", err)
>>>>>>> origin/main
	}
	return nil
}

// CredentialFileName returns the filename used to persist Kilo credentials.
func CredentialFileName(email string) string {
	return fmt.Sprintf("kilo-%s.json", email)
}
