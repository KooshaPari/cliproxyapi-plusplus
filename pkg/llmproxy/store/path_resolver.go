package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveManagedAuthPath(baseDir, value, scope string, addJSONSuffix bool) (string, error) {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		return "", fmt.Errorf("%s: auth directory not configured", scope)
	}
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("%s: resolve auth directory: %w", scope, err)
	}
	raw := strings.TrimSpace(value)
	if raw == "" {
		return "", fmt.Errorf("%s: auth path is empty", scope)
	}
	if hasTraversalComponent(raw) {
		return "", fmt.Errorf("%s: auth path %s escapes outside managed directory", scope, value)
	}
	clean := filepath.Clean(filepath.FromSlash(raw))
	if clean == "." || clean == ".." {
		return "", fmt.Errorf("%s: auth path is invalid", scope)
	}
	if addJSONSuffix && !strings.HasSuffix(strings.ToLower(clean), ".json") {
		clean += ".json"
	}
	path := clean
	if !filepath.IsAbs(path) {
		path = filepath.Join(baseAbs, path)
	}
	path = filepath.Clean(path)
	rel, err := filepath.Rel(baseAbs, path)
	if err != nil {
		return "", fmt.Errorf("%s: resolve auth path: %w", scope, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("%s: auth path %s escapes outside managed directory", scope, value)
	}
	return path, nil
}

func hasTraversalComponent(path string) bool {
	normalized := strings.ReplaceAll(path, "\\", "/")
	for _, component := range strings.Split(normalized, "/") {
		if component == ".." {
			return true
		}
	}
	return false
}
