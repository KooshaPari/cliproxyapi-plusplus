package auth

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBaseTokenStorageSaveRejectsTraversalPath(t *testing.T) {
	t.Parallel()

	ts := NewBaseTokenStorage("../secrets/token.json")
	ts.AccessToken = "token"
	ts.Type = "claude"

	err := ts.Save()
	if err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
	if !strings.Contains(err.Error(), "invalid token file path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBaseTokenStorageSaveAndLoadWithSafePath(t *testing.T) {
	t.Parallel()

	tokenPath := filepath.Join(t.TempDir(), "auth", "token.json")

	ts := NewBaseTokenStorage(tokenPath)
	ts.AccessToken = "token"
	ts.RefreshToken = "refresh"
	ts.Email = "test@example.com"
	ts.Type = "claude"

	if err := ts.Save(); err != nil {
		t.Fatalf("save token: %v", err)
	}

	loaded := NewBaseTokenStorage(tokenPath)
	if err := loaded.Load(); err != nil {
		t.Fatalf("load token: %v", err)
	}

	if loaded.AccessToken != ts.AccessToken {
		t.Fatalf("access token mismatch: got %q want %q", loaded.AccessToken, ts.AccessToken)
	}
	if loaded.RefreshToken != ts.RefreshToken {
		t.Fatalf("refresh token mismatch: got %q want %q", loaded.RefreshToken, ts.RefreshToken)
	}
	if loaded.Email != ts.Email {
		t.Fatalf("email mismatch: got %q want %q", loaded.Email, ts.Email)
	}
}
