// Package kilo provides authentication and token management functionality
// for Kilo AI services.
package kilo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/misc"
)

// KiloTokenStorage stores token information for Kilo AI authentication.
type KiloTokenStorage struct {
	// Type is the provider type for management UI recognition.
	Type string `json:"type"`

	// Token is the Kilo access token serialized as kilocodeToken.
	Token string `json:"kilocodeToken"`

	// OrganizationID is the Kilo organization ID.
	OrganizationID string `json:"kilocodeOrganizationId"`

	// Model is the default model to use.
	Model string `json:"kilocodeModel"`
}

// SaveTokenToFile serializes the Kilo token storage to a JSON file.
func (ts *KiloTokenStorage) SaveTokenToFile(authFilePath string) error {
	cleanPath, err := cleanTokenPath(authFilePath, "kilo token")
	if err != nil {
		return err
	}
	ts.Type = "kilo"

	dir := filepath.Dir(cleanPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(ts, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token storage: %w", err)
	}
	if err := os.WriteFile(cleanPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}
	return nil
}

// CredentialFileName returns the filename used to persist Kilo credentials.
func CredentialFileName(email string) string {
	return fmt.Sprintf("kilo-%s.json", strings.TrimSpace(email))
}

func cleanTokenPath(path, scope string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("%s: auth file path is empty", scope)
	}

	normalizedInput := filepath.FromSlash(trimmed)
	safe, err := misc.ResolveSafeFilePath(normalizedInput)
	if err != nil {
		return "", fmt.Errorf("%s: auth file path is invalid", scope)
	}

	baseDir, absPath, err := normalizePathWithinBase(safe)
	if err != nil {
		return "", fmt.Errorf("%s: auth file path is invalid", scope)
	}
	if err := denySymlinkPath(baseDir, absPath); err != nil {
		return "", fmt.Errorf("%s: auth file path is invalid", scope)
	}
	return absPath, nil
}

func normalizePathWithinBase(path string) (string, string, error) {
	cleanPath := filepath.Clean(path)
	if cleanPath == "." || cleanPath == ".." {
		return "", "", fmt.Errorf("path is invalid")
	}

	var (
		baseDir string
		absPath string
		err     error
	)

	if filepath.IsAbs(cleanPath) {
		absPath = filepath.Clean(cleanPath)
		baseDir = filepath.Clean(filepath.Dir(absPath))
	} else {
		baseDir, err = os.Getwd()
		if err != nil {
			return "", "", fmt.Errorf("resolve working directory: %w", err)
		}
		baseDir, err = filepath.Abs(baseDir)
		if err != nil {
			return "", "", fmt.Errorf("resolve base directory: %w", err)
		}
		absPath = filepath.Clean(filepath.Join(baseDir, cleanPath))
	}

	if !pathWithinBase(baseDir, absPath) {
		return "", "", fmt.Errorf("path escapes base directory")
	}
	return filepath.Clean(baseDir), filepath.Clean(absPath), nil
}

func pathWithinBase(baseDir, path string) bool {
	rel, err := filepath.Rel(baseDir, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func denySymlinkPath(baseDir, targetPath string) error {
	if !pathWithinBase(baseDir, targetPath) {
		return fmt.Errorf("path escapes base directory")
	}
	rel, err := filepath.Rel(baseDir, targetPath)
	if err != nil {
		return fmt.Errorf("resolve relative path: %w", err)
	}
	if rel == "." {
		return nil
	}

	current := filepath.Clean(baseDir)
	for _, component := range strings.Split(rel, string(os.PathSeparator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, errStat := os.Lstat(current)
		if errStat != nil {
			if os.IsNotExist(errStat) {
				return nil
			}
			return fmt.Errorf("stat path: %w", errStat)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink is not allowed in auth file path")
		}
	}
	return nil
}
