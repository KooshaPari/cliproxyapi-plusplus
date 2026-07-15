package util

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/config"
)

const DefaultAuthDir = config.DefaultAuthDir

func ResolveAuthDirOrDefault(authDir string) (string, error) {
	authDir = strings.TrimSpace(authDir)
	if authDir == "" {
		authDir = DefaultAuthDir
	}
	if strings.HasPrefix(authDir, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if home != "" {
			remainder := strings.TrimPrefix(authDir, "~")
			remainder = strings.TrimLeft(remainder, "/\\")
			if remainder == "" {
				return filepath.Clean(home), nil
			}
			normalized := strings.ReplaceAll(remainder, "\\", "/")
			return filepath.Clean(filepath.Join(home, filepath.FromSlash(normalized))), nil
		}
	}
	return filepath.Clean(authDir), nil
}
