package kimi

import (
	"strings"
	"testing"

	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/auth/base"
)

func TestKimiTokenStorage_SaveTokenToFile_RejectsTraversalPath(t *testing.T) {
	ts := &KimiTokenStorage{BaseTokenStorage: base.BaseTokenStorage{AccessToken: "token"}}
	badPath := t.TempDir() + "/../kimi-token.json"

	err := ts.SaveTokenToFile(badPath)
	if err == nil {
		t.Fatal("expected error for traversal path")
	}
	if !strings.Contains(err.Error(), "invalid file path") {
		t.Fatalf("expected invalid path error, got %v", err)
	}
}
