// Package thinking provides unified thinking configuration processing logic.
package thinking

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	"github.com/tidwall/gjson"
)

// setupTestModels registers test models in the global registry for testing.
// This is required because ApplyThinking now looks up models by name.
func setupTestModels(t *testing.T) func() {
	t.Helper()
	reg := registry.GetGlobalRegistry()

	// Register test models via RegisterClient (the correct API)
	clientID := "test-thinking-client"
	testModels := []*registry.ModelInfo{
		{ID: "test-thinking-model", Thinking: &registry.ThinkingSupport{Min: 1, Max: 10}},
		{ID: "test-no-thinking", Type: "gemini"},
		{ID: "gpt-5.2-test", Thinking: &registry.ThinkingSupport{Min: 128, Max: 32768, Levels: []string{"low", "medium", "high"}}},
	}

	reg.RegisterClient(clientID, "test", testModels)

	// Return cleanup function
	return func() {
		reg.UnregisterClient(clientID)
	}
}

func TestApplyThinkingPassthrough(t *testing.T) {
	cleanup := setupTestModels(t)
	defer cleanup()

	tests := []struct {
		name     string
		body     string
		model    string
		provider string
	}{
		{"unknown provider", `{"a":1}`, "test-thinking-model", "unknown"},
		{"unknown model", `{"a":1}`, "nonexistent-model", "gemini"},
		{"nil thinking support", `{"a":1}`, "test-no-thinking", "gemini"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ApplyThinking([]byte(tt.body), tt.model, tt.provider)
			if err != nil {
				t.Fatalf("ApplyThinking() error = %v", err)
			}
			if string(got) != tt.body {
				t.Fatalf("ApplyThinking() = %s, want %s", string(got), tt.body)
			}
		})
	}
}

func TestApplyThinkingValidationError(t *testing.T) {
	cleanup := setupTestModels(t)
	defer cleanup()

	tests := []struct {
		name     string
		body     string
		model    string
		provider string
	}{
		{"unsupported level", `{"reasoning_effort":"ultra"}`, "gpt-5.2-test", "openai"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ApplyThinking([]byte(tt.body), tt.model, tt.provider)
			if err == nil {
				t.Fatalf("ApplyThinking() error = nil, want error")
			}
			// On validation error, ApplyThinking returns original body (defensive programming)
			if string(got) != tt.body {
				t.Fatalf("ApplyThinking() body = %s, want original body %s", string(got), tt.body)
			}
		})
	}
}

func TestApplyThinkingSuffixPriority(t *testing.T) {
	cleanup := setupTestModels(t)
	defer cleanup()

	// Register a model that supports thinking with budget
	reg := registry.GetGlobalRegistry()
	suffixClientID := "test-suffix-client"
	testModels := []*registry.ModelInfo{
		{
			ID:       "gemini-2.5-pro-suffix-test",
			Thinking: &registry.ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: true},
		},
	}
	reg.RegisterClient(suffixClientID, "gemini", testModels)
	defer reg.UnregisterClient(suffixClientID)

	tests := []struct {
		name          string
		body          string
		model         string
		provider      string
		checkPath     string
		expectedValue int
	}{
		{
			"suffix overrides body config",
			`{"generationConfig":{"thinkingConfig":{"thinkingBudget":1000}}}`,
			"gemini-2.5-pro-suffix-test(8192)",
			"gemini",
			"generationConfig.thinkingConfig.thinkingBudget",
			8192,
		},
		{
			"suffix none disables thinking",
			`{"generationConfig":{"thinkingConfig":{"thinkingBudget":1000}}}`,
			"gemini-2.5-pro-suffix-test(none)",
			"gemini",
			"generationConfig.thinkingConfig.thinkingBudget",
			0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ApplyThinking([]byte(tt.body), tt.model, tt.provider)
			if err != nil {
				t.Fatalf("ApplyThinking() error = %v", err)
			}

			// Use gjson to check the value
			result := int(gjson.GetBytes(got, tt.checkPath).Int())
			if result != tt.expectedValue {
				t.Fatalf("ApplyThinking() %s = %v, want %v", tt.checkPath, result, tt.expectedValue)
			}
		})
	}
}
