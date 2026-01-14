// Package geminicli implements thinking configuration for Gemini CLI API format.
package geminicli

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/thinking"
	"github.com/tidwall/gjson"
)

func TestNewApplier(t *testing.T) {
	applier := NewApplier()
	if applier == nil {
		t.Fatal("NewApplier() returned nil")
	}
}

func TestApplierImplementsInterface(t *testing.T) {
	// Compile-time check: if Applier doesn't implement the interface, this won't compile
	var _ thinking.ProviderApplier = (*Applier)(nil)
}

// TestGeminiCLIApply tests the Gemini CLI thinking applier.
//
// Gemini CLI uses request.generationConfig.thinkingConfig.* path.
// Behavior mirrors Gemini applier but with different JSON path prefix.
func TestGeminiCLIApply(t *testing.T) {
	applier := NewApplier()
	tests := []struct {
		name                string
		model               string
		config              thinking.ThinkingConfig
		wantField           string
		wantValue           interface{}
		wantIncludeThoughts bool
	}{
		// Budget mode (no Levels)
		{"budget 8k", "gemini-cli-budget", thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 8192}, "thinkingBudget", 8192, true},
		{"budget zero", "gemini-cli-budget", thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 0}, "thinkingBudget", 0, false},
		{"none mode", "gemini-cli-budget", thinking.ThinkingConfig{Mode: thinking.ModeNone, Budget: 0}, "thinkingBudget", 0, false},
		{"auto mode", "gemini-cli-budget", thinking.ThinkingConfig{Mode: thinking.ModeAuto, Budget: -1}, "thinkingBudget", -1, true},

		// Level mode (has Levels)
		{"level high", "gemini-cli-level", thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelHigh}, "thinkingLevel", "high", true},
		{"level low", "gemini-cli-level", thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelLow}, "thinkingLevel", "low", true},
		{"level minimal", "gemini-cli-level", thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelMinimal}, "thinkingLevel", "minimal", true},
		// ModeAuto with Levels model still uses thinkingBudget=-1
		{"auto with levels", "gemini-cli-level", thinking.ThinkingConfig{Mode: thinking.ModeAuto, Budget: -1}, "thinkingBudget", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelInfo := buildGeminiCLIModelInfo(tt.model)
			result, err := applier.Apply([]byte(`{}`), tt.config, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			gotField := gjson.GetBytes(result, "request.generationConfig.thinkingConfig."+tt.wantField)
			switch want := tt.wantValue.(type) {
			case int:
				if int(gotField.Int()) != want {
					t.Fatalf("%s = %d, want %d", tt.wantField, gotField.Int(), want)
				}
			case string:
				if gotField.String() != want {
					t.Fatalf("%s = %q, want %q", tt.wantField, gotField.String(), want)
				}
			case bool:
				if gotField.Bool() != want {
					t.Fatalf("%s = %v, want %v", tt.wantField, gotField.Bool(), want)
				}
			default:
				t.Fatalf("unsupported wantValue type %T", tt.wantValue)
			}

			gotIncludeThoughts := gjson.GetBytes(result, "request.generationConfig.thinkingConfig.includeThoughts").Bool()
			if gotIncludeThoughts != tt.wantIncludeThoughts {
				t.Fatalf("includeThoughts = %v, want %v", gotIncludeThoughts, tt.wantIncludeThoughts)
			}
		})
	}
}

// TestGeminiCLIApplyModeNoneWithLevel tests ModeNone with Level model.
// When ModeNone is used with a model that has Levels, includeThoughts should be false.
func TestGeminiCLIApplyModeNoneWithLevel(t *testing.T) {
	applier := NewApplier()
	modelInfo := buildGeminiCLIModelInfo("gemini-cli-level")
	config := thinking.ThinkingConfig{Mode: thinking.ModeNone, Level: thinking.LevelLow}

	result, err := applier.Apply([]byte(`{}`), config, modelInfo)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	gotIncludeThoughts := gjson.GetBytes(result, "request.generationConfig.thinkingConfig.includeThoughts").Bool()
	if gotIncludeThoughts != false {
		t.Fatalf("includeThoughts = %v, want %v", gotIncludeThoughts, false)
	}

	gotLevel := gjson.GetBytes(result, "request.generationConfig.thinkingConfig.thinkingLevel").String()
	if gotLevel != "low" {
		t.Fatalf("thinkingLevel = %q, want %q", gotLevel, "low")
	}
}

// TestGeminiCLIApplyInvalidBody tests Apply behavior with invalid body inputs.
func TestGeminiCLIApplyInvalidBody(t *testing.T) {
	applier := NewApplier()
	modelInfo := buildGeminiCLIModelInfo("gemini-cli-budget")
	config := thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 8192}

	tests := []struct {
		name string
		body []byte
	}{
		{"nil body", nil},
		{"empty body", []byte{}},
		{"invalid json", []byte("{\"not json\"")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := applier.Apply(tt.body, config, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			gotBudget := int(gjson.GetBytes(result, "request.generationConfig.thinkingConfig.thinkingBudget").Int())
			if gotBudget != 8192 {
				t.Fatalf("thinkingBudget = %d, want %d", gotBudget, 8192)
			}

			gotIncludeThoughts := gjson.GetBytes(result, "request.generationConfig.thinkingConfig.includeThoughts").Bool()
			if !gotIncludeThoughts {
				t.Fatalf("includeThoughts = %v, want %v", gotIncludeThoughts, true)
			}
		})
	}
}

// TestGeminiCLIApplyConflictingFields tests that conflicting fields are removed.
//
// When applying Budget format, any existing thinkingLevel should be removed.
// When applying Level format, any existing thinkingBudget should be removed.
func TestGeminiCLIApplyConflictingFields(t *testing.T) {
	applier := NewApplier()

	tests := []struct {
		name         string
		model        string
		config       thinking.ThinkingConfig
		existingBody string
		wantField    string // expected field to exist
		wantNoField  string // expected field to NOT exist
	}{
		// Budget format should remove existing thinkingLevel
		{
			"budget removes level",
			"gemini-cli-budget",
			thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 8192},
			`{"request":{"generationConfig":{"thinkingConfig":{"thinkingLevel":"high"}}}}`,
			"thinkingBudget",
			"thinkingLevel",
		},
		// Level format should remove existing thinkingBudget
		{
			"level removes budget",
			"gemini-cli-level",
			thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelHigh},
			`{"request":{"generationConfig":{"thinkingConfig":{"thinkingBudget":8192}}}}`,
			"thinkingLevel",
			"thinkingBudget",
		},
		// ModeAuto uses budget format, should remove thinkingLevel
		{
			"auto removes level",
			"gemini-cli-level",
			thinking.ThinkingConfig{Mode: thinking.ModeAuto, Budget: -1},
			`{"request":{"generationConfig":{"thinkingConfig":{"thinkingLevel":"high"}}}}`,
			"thinkingBudget",
			"thinkingLevel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelInfo := buildGeminiCLIModelInfo(tt.model)
			result, err := applier.Apply([]byte(tt.existingBody), tt.config, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			// Verify expected field exists
			wantPath := "request.generationConfig.thinkingConfig." + tt.wantField
			if !gjson.GetBytes(result, wantPath).Exists() {
				t.Fatalf("%s should exist in result: %s", tt.wantField, string(result))
			}

			// Verify conflicting field was removed
			noPath := "request.generationConfig.thinkingConfig." + tt.wantNoField
			if gjson.GetBytes(result, noPath).Exists() {
				t.Fatalf("%s should NOT exist in result: %s", tt.wantNoField, string(result))
			}
		})
	}
}

// TestGeminiCLIApplyThinkingNotSupported tests passthrough handling when modelInfo.Thinking is nil.
func TestGeminiCLIApplyThinkingNotSupported(t *testing.T) {
	applier := NewApplier()
	config := thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 8192}
	body := []byte(`{"request":{"generationConfig":{"thinkingConfig":{"thinkingBudget":8192}}}}`)

	// Model with nil Thinking support
	modelInfo := &registry.ModelInfo{ID: "gemini-cli-unknown", Thinking: nil}

	got, err := applier.Apply(body, config, modelInfo)
	if err != nil {
		t.Fatalf("Apply() expected nil error for nil Thinking, got %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("expected body unchanged, got %s", string(got))
	}
}

// TestGeminiCLIApplyNilModelInfo tests Apply behavior when modelInfo is nil.
func TestGeminiCLIApplyNilModelInfo(t *testing.T) {
	applier := NewApplier()
	config := thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 8192}
	body := []byte(`{"existing": "data"}`)

	result, err := applier.Apply(body, config, nil)
	if err != nil {
		t.Fatalf("Apply() with nil modelInfo should not error, got: %v", err)
	}
	// nil modelInfo now applies compatible config
	if !gjson.GetBytes(result, "request.generationConfig.thinkingConfig.thinkingBudget").Exists() {
		t.Fatalf("Apply() with nil modelInfo should apply thinking config, got: %s", result)
	}
}

// TestGeminiCLIApplyEmptyModelID tests Apply when modelID is empty.
func TestGeminiCLIApplyEmptyModelID(t *testing.T) {
	applier := NewApplier()
	config := thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 8192}
	modelInfo := &registry.ModelInfo{ID: "", Thinking: nil}
	body := []byte(`{"request":{"generationConfig":{"thinkingConfig":{"thinkingBudget":8192}}}}`)

	got, err := applier.Apply(body, config, modelInfo)
	if err != nil {
		t.Fatalf("Apply() expected nil error, got %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("expected body unchanged, got %s", string(got))
	}
}

// TestGeminiCLIApplyModeBudgetWithLevels tests that ModeBudget with Levels model passes through.
// Apply layer doesn't convert - upper layer should handle Budget→Level conversion.
func TestGeminiCLIApplyModeBudgetWithLevels(t *testing.T) {
	applier := NewApplier()
	modelInfo := buildGeminiCLIModelInfo("gemini-cli-level")
	config := thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 8192}
	body := []byte(`{"existing": "data"}`)

	result, err := applier.Apply(body, config, modelInfo)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	// ModeBudget applies budget format directly without conversion to levels
	if !gjson.GetBytes(result, "request.generationConfig.thinkingConfig.thinkingBudget").Exists() {
		t.Fatalf("Apply() ModeBudget should apply budget format, got: %s", result)
	}
}

// TestGeminiCLIApplyUnsupportedMode tests behavior with unsupported Mode types.
func TestGeminiCLIApplyUnsupportedMode(t *testing.T) {
	applier := NewApplier()
	body := []byte(`{"existing": "data"}`)

	tests := []struct {
		name   string
		model  string
		config thinking.ThinkingConfig
	}{
		{"unknown mode with budget model", "gemini-cli-budget", thinking.ThinkingConfig{Mode: thinking.ThinkingMode(99), Budget: 8192}},
		{"unknown mode with level model", "gemini-cli-level", thinking.ThinkingConfig{Mode: thinking.ThinkingMode(99), Level: thinking.LevelHigh}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelInfo := buildGeminiCLIModelInfo(tt.model)
			result, err := applier.Apply(body, tt.config, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			// Unsupported modes return original body unchanged
			if string(result) != string(body) {
				t.Fatalf("Apply() with unsupported mode should return original body, got: %s", result)
			}
		})
	}
}

// TestAntigravityUsesGeminiCLIFormat tests that antigravity provider uses gemini-cli format.
// Antigravity is registered with the same applier as gemini-cli.
func TestAntigravityUsesGeminiCLIFormat(t *testing.T) {
	applier := NewApplier()

	tests := []struct {
		name      string
		config    thinking.ThinkingConfig
		modelInfo *registry.ModelInfo
		wantField string
	}{
		{
			"claude model budget",
			thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 16384},
			&registry.ModelInfo{ID: "gemini-claude-sonnet-4-5-thinking", Thinking: &registry.ThinkingSupport{Min: 1024, Max: 200000}},
			"request.generationConfig.thinkingConfig.thinkingBudget",
		},
		{
			"opus model budget",
			thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 32768},
			&registry.ModelInfo{ID: "gemini-claude-opus-4-5-thinking", Thinking: &registry.ThinkingSupport{Min: 1024, Max: 200000}},
			"request.generationConfig.thinkingConfig.thinkingBudget",
		},
		{
			"model with levels",
			thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelHigh},
			&registry.ModelInfo{ID: "some-model-with-levels", Thinking: &registry.ThinkingSupport{Min: 1024, Max: 200000, Levels: []string{"low", "high"}}},
			"request.generationConfig.thinkingConfig.thinkingLevel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applier.Apply([]byte(`{}`), tt.config, tt.modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			if !gjson.GetBytes(got, tt.wantField).Exists() {
				t.Fatalf("expected field %s in output: %s", tt.wantField, string(got))
			}
		})
	}
}

func buildGeminiCLIModelInfo(modelID string) *registry.ModelInfo {
	support := &registry.ThinkingSupport{}
	switch modelID {
	case "gemini-cli-budget":
		support.Min = 0
		support.Max = 32768
		support.ZeroAllowed = true
		support.DynamicAllowed = true
	case "gemini-cli-level":
		support.Min = 128
		support.Max = 32768
		support.ZeroAllowed = false
		support.DynamicAllowed = true
		support.Levels = []string{"minimal", "low", "medium", "high"}
	default:
		// Unknown model - return nil Thinking to trigger error path
		return &registry.ModelInfo{ID: modelID, Thinking: nil}
	}
	return &registry.ModelInfo{
		ID:       modelID,
		Thinking: support,
	}
}
