package qwen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestQwenTokenStorage_SaveTokenToFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "qwen-token.json")
	ts := NewQwenTokenStorage(path)
	ts.AccessToken = "access"
	ts.Email = "test@example.com"

	if err := ts.SaveTokenToFile(path); err != nil {
		t.Fatalf("SaveTokenToFile failed: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected token file to exist: %v", err)
	}
}

func TestQwenTokenStorage_SaveTokenToFile_RejectsTraversalPath(t *testing.T) {
	t.Parallel()

	ts := NewQwenTokenStorage("../qwen-token.json")
	ts.AccessToken = "access"
	if err := ts.SaveTokenToFile("../qwen-token.json"); err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
}
