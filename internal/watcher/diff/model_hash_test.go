package diff

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestComputeOpenAICompatModelsHash_Deterministic(t *testing.T) {
	models := []config.OpenAICompatibilityModel{
		{Name: "gpt-4", Alias: "gpt4"},
		{Name: "gpt-3.5-turbo"},
	}
	hash1 := ComputeOpenAICompatModelsHash(models)
	hash2 := ComputeOpenAICompatModelsHash(models)
	if hash1 == "" {
		t.Fatal("hash should not be empty")
	}
	if hash1 != hash2 {
		t.Fatalf("hash should be deterministic, got %s vs %s", hash1, hash2)
	}
	changed := ComputeOpenAICompatModelsHash([]config.OpenAICompatibilityModel{{Name: "gpt-4"}, {Name: "gpt-4.1"}})
	if hash1 == changed {
		t.Fatal("hash should change when model list changes")
	}
}

func TestComputeVertexCompatModelsHash_DifferentInputs(t *testing.T) {
	models := []config.VertexCompatModel{{Name: "gemini-pro", Alias: "pro"}}
	hash1 := ComputeVertexCompatModelsHash(models)
	hash2 := ComputeVertexCompatModelsHash([]config.VertexCompatModel{{Name: "gemini-1.5-pro", Alias: "pro"}})
	if hash1 == "" || hash2 == "" {
		t.Fatal("hashes should not be empty for non-empty models")
	}
	if hash1 == hash2 {
		t.Fatal("hash should differ when model content differs")
	}
}

func TestComputeClaudeModelsHash_Empty(t *testing.T) {
	if got := ComputeClaudeModelsHash(nil); got != "" {
		t.Fatalf("expected empty hash for nil models, got %q", got)
	}
	if got := ComputeClaudeModelsHash([]config.ClaudeModel{}); got != "" {
		t.Fatalf("expected empty hash for empty slice, got %q", got)
	}
}

func TestComputeExcludedModelsHash_Normalizes(t *testing.T) {
	hash1 := ComputeExcludedModelsHash([]string{" A ", "b", "a"})
	hash2 := ComputeExcludedModelsHash([]string{"a", " b", "A"})
	if hash1 == "" || hash2 == "" {
		t.Fatal("hash should not be empty for non-empty input")
	}
	if hash1 != hash2 {
		t.Fatalf("hash should be order/space insensitive for same multiset, got %s vs %s", hash1, hash2)
	}
	hash3 := ComputeExcludedModelsHash([]string{"c"})
	if hash1 == hash3 {
		t.Fatal("hash should differ for different normalized sets")
	}
}

func TestComputeOpenAICompatModelsHash_Empty(t *testing.T) {
	if got := ComputeOpenAICompatModelsHash(nil); got != "" {
		t.Fatalf("expected empty hash for nil input, got %q", got)
	}
	if got := ComputeOpenAICompatModelsHash([]config.OpenAICompatibilityModel{}); got != "" {
		t.Fatalf("expected empty hash for empty slice, got %q", got)
	}
}

func TestComputeVertexCompatModelsHash_Empty(t *testing.T) {
	if got := ComputeVertexCompatModelsHash(nil); got != "" {
		t.Fatalf("expected empty hash for nil input, got %q", got)
	}
	if got := ComputeVertexCompatModelsHash([]config.VertexCompatModel{}); got != "" {
		t.Fatalf("expected empty hash for empty slice, got %q", got)
	}
}

func TestComputeExcludedModelsHash_Empty(t *testing.T) {
	if got := ComputeExcludedModelsHash(nil); got != "" {
		t.Fatalf("expected empty hash for nil input, got %q", got)
	}
	if got := ComputeExcludedModelsHash([]string{}); got != "" {
		t.Fatalf("expected empty hash for empty slice, got %q", got)
	}
	if got := ComputeExcludedModelsHash([]string{"  ", ""}); got != "" {
		t.Fatalf("expected empty hash for whitespace-only entries, got %q", got)
	}
}

func TestComputeClaudeModelsHash_Deterministic(t *testing.T) {
	models := []config.ClaudeModel{{Name: "a", Alias: "A"}, {Name: "b"}}
	h1 := ComputeClaudeModelsHash(models)
	h2 := ComputeClaudeModelsHash(models)
	if h1 == "" || h1 != h2 {
		t.Fatalf("expected deterministic hash, got %s / %s", h1, h2)
	}
	if h3 := ComputeClaudeModelsHash([]config.ClaudeModel{{Name: "a"}}); h3 == h1 {
		t.Fatalf("expected different hash when models change, got %s", h3)
	}
}
