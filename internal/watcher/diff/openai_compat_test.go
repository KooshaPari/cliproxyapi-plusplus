package diff

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestDiffOpenAICompatibility(t *testing.T) {
	oldList := []config.OpenAICompatibility{
		{
			Name: "provider-a",
			APIKeyEntries: []config.OpenAICompatibilityAPIKey{
				{APIKey: "key-a"},
			},
			Models: []config.OpenAICompatibilityModel{
				{Name: "m1"},
			},
		},
	}
	newList := []config.OpenAICompatibility{
		{
			Name: "provider-a",
			APIKeyEntries: []config.OpenAICompatibilityAPIKey{
				{APIKey: "key-a"},
				{APIKey: "key-b"},
			},
			Models: []config.OpenAICompatibilityModel{
				{Name: "m1"},
				{Name: "m2"},
			},
			Headers: map[string]string{"X-Test": "1"},
		},
		{
			Name:          "provider-b",
			APIKeyEntries: []config.OpenAICompatibilityAPIKey{{APIKey: "key-b"}},
		},
	}

	changes := DiffOpenAICompatibility(oldList, newList)
	expectContains(t, changes, "provider added: provider-b (api-keys=1, models=0)")
	expectContains(t, changes, "provider updated: provider-a (api-keys 1 -> 2, models 1 -> 2, headers updated)")
}

func TestDiffOpenAICompatibility_RemovedAndUnchanged(t *testing.T) {
	oldList := []config.OpenAICompatibility{
		{
			Name:          "provider-a",
			APIKeyEntries: []config.OpenAICompatibilityAPIKey{{APIKey: "key-a"}},
			Models:        []config.OpenAICompatibilityModel{{Name: "m1"}},
		},
	}
	newList := []config.OpenAICompatibility{
		{
			Name:          "provider-a",
			APIKeyEntries: []config.OpenAICompatibilityAPIKey{{APIKey: "key-a"}},
			Models:        []config.OpenAICompatibilityModel{{Name: "m1"}},
		},
	}
	if changes := DiffOpenAICompatibility(oldList, newList); len(changes) != 0 {
		t.Fatalf("expected no changes, got %v", changes)
	}

	newList = nil
	changes := DiffOpenAICompatibility(oldList, newList)
	expectContains(t, changes, "provider removed: provider-a (api-keys=1, models=1)")
}

func TestOpenAICompatKeyFallbacks(t *testing.T) {
	entry := config.OpenAICompatibility{
		BaseURL: "http://base",
		Models:  []config.OpenAICompatibilityModel{{Alias: "alias-only"}},
	}
	key, label := openAICompatKey(entry, 0)
	if key != "base:http://base" || label != "http://base" {
		t.Fatalf("expected base key, got %s/%s", key, label)
	}

	entry.BaseURL = ""
	key, label = openAICompatKey(entry, 1)
	if key != "alias:alias-only" || label != "alias-only" {
		t.Fatalf("expected alias fallback, got %s/%s", key, label)
	}

	entry.Models = nil
	key, label = openAICompatKey(entry, 2)
	if key != "index:2" || label != "entry-3" {
		t.Fatalf("expected index fallback, got %s/%s", key, label)
	}
}

func TestCountOpenAIModelsSkipsBlanks(t *testing.T) {
	models := []config.OpenAICompatibilityModel{
		{Name: "m1"},
		{Name: ""},
		{Alias: ""},
		{Name: " "},
		{Alias: "a1"},
	}
	if got := countOpenAIModels(models); got != 2 {
		t.Fatalf("expected 2 counted models, got %d", got)
	}
}

func TestOpenAICompatKeyUsesModelNameWhenAliasEmpty(t *testing.T) {
	entry := config.OpenAICompatibility{
		Models: []config.OpenAICompatibilityModel{{Name: "model-name"}},
	}
	key, label := openAICompatKey(entry, 5)
	if key != "alias:model-name" || label != "model-name" {
		t.Fatalf("expected model-name fallback, got %s/%s", key, label)
	}
}
