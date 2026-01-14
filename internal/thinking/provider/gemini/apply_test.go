// Package gemini implements thinking configuration for Gemini models.
package gemini

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

// parseConfigFromSuffix parses a raw suffix into a ThinkingConfig.
// This helper reduces code duplication in end-to-end tests (L1 fix).
func parseConfigFromSuffix(rawSuffix string) (thinking.ThinkingConfig, bool) {
	if budget, ok := thinking.ParseNumericSuffix(rawSuffix); ok {
		return thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: budget}, true
	}
	if level, ok := thinking.ParseLevelSuffix(rawSuffix); ok {
		return thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: level}, true
	}
	if mode, ok := thinking.ParseSpecialSuffix(rawSuffix); ok {
		config := thinking.ThinkingConfig{Mode: mode}
		if mode == thinking.ModeAuto {
			config.Budget = -1
		}
		return config, true
	}
	return thinking.ThinkingConfig{}, false
}

func TestApplierImplementsInterface(t *testing.T) {
	// Compile-time check: if Applier doesn't implement the interface, this won't compile
	var _ thinking.ProviderApplier = (*Applier)(nil)
}

// TestGeminiApply tests the Gemini thinking applier.
//
// Gemini-specific behavior:
//   - Gemini 2.5: thinkingBudget format (numeric)
//   - Gemini 3.x: thinkingLevel format (string)
//   - Flash series: ZeroAllowed=true
//   - Pro series: ZeroAllowed=false, Min=128
//   - CRITICAL: When budget=0/none, set includeThoughts=false
//
// Depends on: Epic 7 Story 7-2, 7-3
func TestGeminiApply(t *testing.T) {
	applier := NewApplier()
	tests := []struct {
		name                string
		model               string
		config              thinking.ThinkingConfig
		wantField           string
		wantValue           interface{}
		wantIncludeThoughts bool // CRITICAL: includeThoughts field
	}{
		// Gemini 2.5 Flash (ZeroAllowed=true)
		{"flash budget 8k", "gemini-2.5-flash", thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 8192}, "thinkingBudget", 8192, true},
		{"flash zero", "gemini-2.5-flash", thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 0}, "thinkingBudget", 0, false},
		{"flash none", "gemini-2.5-flash", thinking.ThinkingConfig{Mode: thinking.ModeNone, Budget: 0}, "thinkingBudget", 0, false},

		// Gemini 2.5 Pro (ZeroAllowed=false, Min=128)
		{"pro budget 8k", "gemini-2.5-pro", thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 8192}, "thinkingBudget", 8192, true},
		{"pro zero - clamp", "gemini-2.5-pro", thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 0}, "thinkingBudget", 128, false},
		{"pro none - clamp", "gemini-2.5-pro", thinking.ThinkingConfig{Mode: thinking.ModeNone, Budget: 0}, "thinkingBudget", 128, false},
		{"pro below min", "gemini-2.5-pro", thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 50}, "thinkingBudget", 128, true},
		{"pro above max", "gemini-2.5-pro", thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 50000}, "thinkingBudget", 32768, true},
		{"pro auto", "gemini-2.5-pro", thinking.ThinkingConfig{Mode: thinking.ModeAuto, Budget: -1}, "thinkingBudget", -1, true},

		// Gemini 3 Pro (Level mode, ZeroAllowed=false)
		{"g3-pro high", "gemini-3-pro-preview", thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelHigh}, "thinkingLevel", "high", true},
		{"g3-pro low", "gemini-3-pro-preview", thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelLow}, "thinkingLevel", "low", true},
		{"g3-pro auto", "gemini-3-pro-preview", thinking.ThinkingConfig{Mode: thinking.ModeAuto, Budget: -1}, "thinkingBudget", -1, true},

		// Gemini 3 Flash (Level mode, minimal is lowest)
		{"g3-flash high", "gemini-3-flash-preview", thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelHigh}, "thinkingLevel", "high", true},
		{"g3-flash medium", "gemini-3-flash-preview", thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelMedium}, "thinkingLevel", "medium", true},
		{"g3-flash minimal", "gemini-3-flash-preview", thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelMinimal}, "thinkingLevel", "minimal", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelInfo := buildGeminiModelInfo(tt.model)
			normalized, err := thinking.ValidateConfig(tt.config, modelInfo.Thinking)
			if err != nil {
				t.Fatalf("ValidateConfig() error = %v", err)
			}

			result, err := applier.Apply([]byte(`{}`), *normalized, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			gotField := gjson.GetBytes(result, "generationConfig.thinkingConfig."+tt.wantField)
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

			gotIncludeThoughts := gjson.GetBytes(result, "generationConfig.thinkingConfig.includeThoughts").Bool()
			if gotIncludeThoughts != tt.wantIncludeThoughts {
				t.Fatalf("includeThoughts = %v, want %v", gotIncludeThoughts, tt.wantIncludeThoughts)
			}
		})
	}
}

// TestGeminiApplyEndToEndBudgetZero tests suffix parsing + validation + apply for budget=0.
//
// This test covers the complete flow from suffix parsing to Apply output:
//   - AC#1: ModeBudget+Budget=0 → ModeNone conversion
//   - AC#3: Gemini 3 ModeNone+Budget>0 → includeThoughts=false + thinkingLevel=low
//   - AC#4: Gemini 2.5 Pro (0) → clamped to 128 + includeThoughts=false
func TestGeminiApplyEndToEndBudgetZero(t *testing.T) {
	tests := []struct {
		name                string
		model               string
		wantModel           string
		wantField           string // "thinkingBudget" or "thinkingLevel"
		wantValue           interface{}
		wantIncludeThoughts bool
	}{
		// AC#4: Gemini 2.5 Pro - Budget format
		{"gemini-25-pro zero", "gemini-2.5-pro(0)", "gemini-2.5-pro", "thinkingBudget", 128, false},
		// AC#3: Gemini 3 Pro - Level format, ModeNone clamped to Budget=128, uses lowest level
		{"gemini-3-pro zero", "gemini-3-pro-preview(0)", "gemini-3-pro-preview", "thinkingLevel", "low", false},
		{"gemini-3-pro none", "gemini-3-pro-preview(none)", "gemini-3-pro-preview", "thinkingLevel", "low", false},
		// Gemini 3 Flash - Level format, lowest level is "minimal"
		{"gemini-3-flash zero", "gemini-3-flash-preview(0)", "gemini-3-flash-preview", "thinkingLevel", "minimal", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suffix := thinking.ParseSuffix(tt.model)
			if !suffix.HasSuffix {
				t.Fatalf("ParseSuffix(%q) HasSuffix = false, want true", tt.model)
			}
			if suffix.ModelName != tt.wantModel {
				t.Fatalf("ParseSuffix(%q) ModelName = %q, want %q", tt.model, suffix.ModelName, tt.wantModel)
			}

			// Parse suffix value using helper function (L1 fix)
			config, ok := parseConfigFromSuffix(suffix.RawSuffix)
			if !ok {
				t.Fatalf("ParseSuffix(%q) RawSuffix = %q is not a valid suffix", tt.model, suffix.RawSuffix)
			}

			modelInfo := buildGeminiModelInfo(suffix.ModelName)
			normalized, err := thinking.ValidateConfig(config, modelInfo.Thinking)
			if err != nil {
				t.Fatalf("ValidateConfig() error = %v", err)
			}

			applier := NewApplier()
			result, err := applier.Apply([]byte(`{}`), *normalized, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			// Verify the output field value
			gotField := gjson.GetBytes(result, "generationConfig.thinkingConfig."+tt.wantField)
			switch want := tt.wantValue.(type) {
			case int:
				if int(gotField.Int()) != want {
					t.Fatalf("%s = %d, want %d", tt.wantField, gotField.Int(), want)
				}
			case string:
				if gotField.String() != want {
					t.Fatalf("%s = %q, want %q", tt.wantField, gotField.String(), want)
				}
			}

			gotIncludeThoughts := gjson.GetBytes(result, "generationConfig.thinkingConfig.includeThoughts").Bool()
			if gotIncludeThoughts != tt.wantIncludeThoughts {
				t.Fatalf("includeThoughts = %v, want %v", gotIncludeThoughts, tt.wantIncludeThoughts)
			}
		})
	}
}

// TestGeminiApplyEndToEndAuto tests auto mode through both suffix parsing and direct config.
//
// This test covers:
//   - AC#2: Gemini 2.5 auto uses thinkingBudget=-1
//   - AC#3: Gemini 3 auto uses thinkingBudget=-1 (not thinkingLevel)
//   - Suffix parsing path: (auto) and (-1) suffixes
//   - Direct config path: ModeLevel + Level=auto → ModeAuto conversion
func TestGeminiApplyEndToEndAuto(t *testing.T) {
	tests := []struct {
		name                string
		model               string                   // model name (with suffix for parsing, or plain for direct config)
		directConfig        *thinking.ThinkingConfig // if not nil, use direct config instead of suffix parsing
		wantField           string
		wantValue           int
		wantIncludeThoughts bool
	}{
		// Suffix parsing path - Budget-only model (Gemini 2.5)
		{"suffix auto g25", "gemini-2.5-pro(auto)", nil, "thinkingBudget", -1, true},
		{"suffix -1 g25", "gemini-2.5-pro(-1)", nil, "thinkingBudget", -1, true},
		// Suffix parsing path - Hybrid model (Gemini 3)
		{"suffix auto g3", "gemini-3-pro-preview(auto)", nil, "thinkingBudget", -1, true},
		{"suffix -1 g3", "gemini-3-pro-preview(-1)", nil, "thinkingBudget", -1, true},
		// Direct config path - Level=auto → ModeAuto conversion
		{"direct level=auto g25", "gemini-2.5-pro", &thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelAuto}, "thinkingBudget", -1, true},
		{"direct level=auto g3", "gemini-3-pro-preview", &thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelAuto}, "thinkingBudget", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config thinking.ThinkingConfig
			var modelName string

			if tt.directConfig != nil {
				// Direct config path
				config = *tt.directConfig
				modelName = tt.model
			} else {
				// Suffix parsing path
				suffix := thinking.ParseSuffix(tt.model)
				if !suffix.HasSuffix {
					t.Fatalf("ParseSuffix(%q) HasSuffix = false", tt.model)
				}
				modelName = suffix.ModelName
				var ok bool
				config, ok = parseConfigFromSuffix(suffix.RawSuffix)
				if !ok {
					t.Fatalf("parseConfigFromSuffix(%q) failed", suffix.RawSuffix)
				}
			}

			modelInfo := buildGeminiModelInfo(modelName)
			normalized, err := thinking.ValidateConfig(config, modelInfo.Thinking)
			if err != nil {
				t.Fatalf("ValidateConfig() error = %v", err)
			}

			// Verify ModeAuto after validation
			if normalized.Mode != thinking.ModeAuto {
				t.Fatalf("ValidateConfig() Mode = %v, want ModeAuto", normalized.Mode)
			}

			applier := NewApplier()
			result, err := applier.Apply([]byte(`{}`), *normalized, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			gotField := gjson.GetBytes(result, "generationConfig.thinkingConfig."+tt.wantField)
			if int(gotField.Int()) != tt.wantValue {
				t.Fatalf("%s = %d, want %d", tt.wantField, gotField.Int(), tt.wantValue)
			}

			gotIncludeThoughts := gjson.GetBytes(result, "generationConfig.thinkingConfig.includeThoughts").Bool()
			if gotIncludeThoughts != tt.wantIncludeThoughts {
				t.Fatalf("includeThoughts = %v, want %v", gotIncludeThoughts, tt.wantIncludeThoughts)
			}
		})
	}
}

func TestGeminiApplyInvalidBody(t *testing.T) {
	applier := NewApplier()
	modelInfo := buildGeminiModelInfo("gemini-2.5-flash")
	config := thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 8192}
	normalized, err := thinking.ValidateConfig(config, modelInfo.Thinking)
	if err != nil {
		t.Fatalf("ValidateConfig() error = %v", err)
	}

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
			result, err := applier.Apply(tt.body, *normalized, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			gotBudget := int(gjson.GetBytes(result, "generationConfig.thinkingConfig.thinkingBudget").Int())
			if gotBudget != 8192 {
				t.Fatalf("thinkingBudget = %d, want %d", gotBudget, 8192)
			}

			gotIncludeThoughts := gjson.GetBytes(result, "generationConfig.thinkingConfig.includeThoughts").Bool()
			if !gotIncludeThoughts {
				t.Fatalf("includeThoughts = %v, want %v", gotIncludeThoughts, true)
			}
		})
	}
}

// TestGeminiApplyConflictingFields tests that conflicting fields are removed.
//
// When applying Budget format, any existing thinkingLevel should be removed.
// When applying Level format, any existing thinkingBudget should be removed.
func TestGeminiApplyConflictingFields(t *testing.T) {
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
			"gemini-2.5-pro",
			thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 8192},
			`{"generationConfig":{"thinkingConfig":{"thinkingLevel":"high"}}}`,
			"thinkingBudget",
			"thinkingLevel",
		},
		// Level format should remove existing thinkingBudget
		{
			"level removes budget",
			"gemini-3-pro-preview",
			thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelHigh},
			`{"generationConfig":{"thinkingConfig":{"thinkingBudget":8192}}}`,
			"thinkingLevel",
			"thinkingBudget",
		},
		// ModeAuto uses budget format, should remove thinkingLevel
		{
			"auto removes level",
			"gemini-3-pro-preview",
			thinking.ThinkingConfig{Mode: thinking.ModeAuto, Budget: -1},
			`{"generationConfig":{"thinkingConfig":{"thinkingLevel":"high"}}}`,
			"thinkingBudget",
			"thinkingLevel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelInfo := buildGeminiModelInfo(tt.model)
			result, err := applier.Apply([]byte(tt.existingBody), tt.config, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			// Verify expected field exists
			wantPath := "generationConfig.thinkingConfig." + tt.wantField
			if !gjson.GetBytes(result, wantPath).Exists() {
				t.Fatalf("%s should exist in result: %s", tt.wantField, string(result))
			}

			// Verify conflicting field was removed
			noPath := "generationConfig.thinkingConfig." + tt.wantNoField
			if gjson.GetBytes(result, noPath).Exists() {
				t.Fatalf("%s should NOT exist in result: %s", tt.wantNoField, string(result))
			}
		})
	}
}

// TestGeminiApplyThinkingNotSupported tests passthrough handling when modelInfo.Thinking is nil.
func TestGeminiApplyThinkingNotSupported(t *testing.T) {
	applier := NewApplier()
	config := thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 8192}
	body := []byte(`{"generationConfig":{"thinkingConfig":{"thinkingBudget":8192}}}`)

	// Model with nil Thinking support
	modelInfo := &registry.ModelInfo{ID: "gemini-unknown", Thinking: nil}

	got, err := applier.Apply(body, config, modelInfo)
	if err != nil {
		t.Fatalf("Apply() expected nil error for nil Thinking, got %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("expected body unchanged, got %s", string(got))
	}
}

func buildGeminiModelInfo(modelID string) *registry.ModelInfo {
	support := &registry.ThinkingSupport{}
	switch modelID {
	case "gemini-2.5-pro":
		support.Min = 128
		support.Max = 32768
		support.ZeroAllowed = false
		support.DynamicAllowed = true
	case "gemini-2.5-flash", "gemini-2.5-flash-lite":
		support.Min = 0
		support.Max = 24576
		support.ZeroAllowed = true
		support.DynamicAllowed = true
	case "gemini-3-pro-preview":
		support.Min = 128
		support.Max = 32768
		support.ZeroAllowed = false
		support.DynamicAllowed = true
		support.Levels = []string{"low", "high"}
	case "gemini-3-flash-preview":
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

// TestGeminiApplyNilModelInfo tests Apply behavior when modelInfo is nil.
// Coverage: apply.go:56-58 (H1)
func TestGeminiApplyNilModelInfo(t *testing.T) {
	applier := NewApplier()
	config := thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 8192}
	body := []byte(`{"existing": "data"}`)

	result, err := applier.Apply(body, config, nil)
	if err != nil {
		t.Fatalf("Apply() with nil modelInfo should not error, got: %v", err)
	}
	// nil modelInfo now applies compatible config
	if !gjson.GetBytes(result, "generationConfig.thinkingConfig.thinkingBudget").Exists() {
		t.Fatalf("Apply() with nil modelInfo should apply thinking config, got: %s", result)
	}
}

// TestGeminiApplyEmptyModelID tests Apply when modelID is empty.
// Coverage: apply.go:61-63 (H2)
func TestGeminiApplyEmptyModelID(t *testing.T) {
	applier := NewApplier()
	config := thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 8192}
	modelInfo := &registry.ModelInfo{ID: "", Thinking: nil}
	body := []byte(`{"generationConfig":{"thinkingConfig":{"thinkingBudget":8192}}}`)

	got, err := applier.Apply(body, config, modelInfo)
	if err != nil {
		t.Fatalf("Apply() expected nil error, got %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("expected body unchanged, got %s", string(got))
	}
}

// TestGeminiApplyModeBudgetWithLevels tests that ModeBudget is applied with budget format
// even for models with Levels. The Apply layer handles ModeBudget by applying thinkingBudget.
// Coverage: apply.go:88-90
func TestGeminiApplyModeBudgetWithLevels(t *testing.T) {
	applier := NewApplier()
	modelInfo := buildGeminiModelInfo("gemini-3-flash-preview")
	config := thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 8192}
	body := []byte(`{"existing": "data"}`)

	result, err := applier.Apply(body, config, modelInfo)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	// ModeBudget applies budget format
	budget := gjson.GetBytes(result, "generationConfig.thinkingConfig.thinkingBudget").Int()
	if budget != 8192 {
		t.Fatalf("Apply() expected thinkingBudget=8192, got: %d", budget)
	}
}

// TestGeminiApplyUnsupportedMode tests behavior with unsupported Mode types.
// Coverage: apply.go:67-69 and 97-98 (H5, L2)
func TestGeminiApplyUnsupportedMode(t *testing.T) {
	applier := NewApplier()
	body := []byte(`{"existing": "data"}`)

	tests := []struct {
		name   string
		model  string
		config thinking.ThinkingConfig
	}{
		{"unknown mode with budget model", "gemini-2.5-pro", thinking.ThinkingConfig{Mode: thinking.ThinkingMode(99), Budget: 8192}},
		{"unknown mode with level model", "gemini-3-pro-preview", thinking.ThinkingConfig{Mode: thinking.ThinkingMode(99), Level: thinking.LevelHigh}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelInfo := buildGeminiModelInfo(tt.model)
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
