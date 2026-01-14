// Package thinking provides unified thinking configuration processing logic.
package thinking

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
)

// TestConvertLevelToBudget tests the ConvertLevelToBudget function.
//
// ConvertLevelToBudget converts a thinking level to a budget value.
// This is a semantic conversion - it does NOT apply clamping.
//
// Level → Budget mapping:
//   - none    → 0
//   - auto    → -1
//   - minimal → 512
//   - low     → 1024
//   - medium  → 8192
//   - high    → 24576
//   - xhigh   → 32768
func TestConvertLevelToBudget(t *testing.T) {
	tests := []struct {
		name   string
		level  string
		want   int
		wantOK bool
	}{
		// Standard levels
		{"none", "none", 0, true},
		{"auto", "auto", -1, true},
		{"minimal", "minimal", 512, true},
		{"low", "low", 1024, true},
		{"medium", "medium", 8192, true},
		{"high", "high", 24576, true},
		{"xhigh", "xhigh", 32768, true},

		// Case insensitive
		{"case insensitive HIGH", "HIGH", 24576, true},
		{"case insensitive High", "High", 24576, true},
		{"case insensitive NONE", "NONE", 0, true},
		{"case insensitive Auto", "Auto", -1, true},

		// Invalid levels
		{"invalid ultra", "ultra", 0, false},
		{"invalid maximum", "maximum", 0, false},
		{"empty string", "", 0, false},
		{"whitespace", " ", 0, false},
		{"numeric string", "1000", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			budget, ok := ConvertLevelToBudget(tt.level)
			if ok != tt.wantOK {
				t.Errorf("ConvertLevelToBudget(%q) ok = %v, want %v", tt.level, ok, tt.wantOK)
			}
			if budget != tt.want {
				t.Errorf("ConvertLevelToBudget(%q) = %d, want %d", tt.level, budget, tt.want)
			}
		})
	}
}

// TestConvertBudgetToLevel tests the ConvertBudgetToLevel function.
//
// ConvertBudgetToLevel converts a budget value to the nearest level.
// Uses threshold-based mapping for range conversion.
//
// Budget → Level thresholds:
//   - -1       → auto
//   - 0        → none
//   - 1-512    → minimal
//   - 513-1024 → low
//   - 1025-8192 → medium
//   - 8193-24576 → high
//   - 24577+   → xhigh
//
// Depends on: Epic 4 Story 4-2 (budget to level conversion)
func TestConvertBudgetToLevel(t *testing.T) {
	tests := []struct {
		name   string
		budget int
		want   string
		wantOK bool
	}{
		// Special values
		{"auto", -1, "auto", true},
		{"none", 0, "none", true},

		// Invalid negative values
		{"invalid negative -2", -2, "", false},
		{"invalid negative -100", -100, "", false},
		{"invalid negative extreme", -999999, "", false},

		// Minimal range (1-512)
		{"minimal min", 1, "minimal", true},
		{"minimal mid", 256, "minimal", true},
		{"minimal max", 512, "minimal", true},

		// Low range (513-1024)
		{"low start", 513, "low", true},
		{"low boundary", 1024, "low", true},

		// Medium range (1025-8192)
		{"medium start", 1025, "medium", true},
		{"medium mid", 4096, "medium", true},
		{"medium boundary", 8192, "medium", true},

		// High range (8193-24576)
		{"high start", 8193, "high", true},
		{"high mid", 16384, "high", true},
		{"high boundary", 24576, "high", true},

		// XHigh range (24577+)
		{"xhigh start", 24577, "xhigh", true},
		{"xhigh mid", 32768, "xhigh", true},
		{"xhigh large", 100000, "xhigh", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, ok := ConvertBudgetToLevel(tt.budget)
			if ok != tt.wantOK {
				t.Errorf("ConvertBudgetToLevel(%d) ok = %v, want %v", tt.budget, ok, tt.wantOK)
			}
			if level != tt.want {
				t.Errorf("ConvertBudgetToLevel(%d) = %q, want %q", tt.budget, level, tt.want)
			}
		})
	}
}

// TestConvertMixedFormat tests mixed format handling.
//
// Tests scenarios where both level and budget might be present,
// or where format conversion requires special handling.
//
// Depends on: Epic 4 Story 4-3 (mixed format handling)
func TestConvertMixedFormat(t *testing.T) {
	tests := []struct {
		name        string
		inputBudget int
		inputLevel  string
		wantMode    ThinkingMode
		wantBudget  int
		wantLevel   ThinkingLevel
	}{
		// Level takes precedence when both present
		{"level and budget - level wins", 8192, "high", ModeLevel, 0, LevelHigh},
		{"level and zero budget", 0, "high", ModeLevel, 0, LevelHigh},

		// Budget only
		{"budget only", 16384, "", ModeBudget, 16384, ""},

		// Level only
		{"level only", 0, "medium", ModeLevel, 0, LevelMedium},

		// Neither (default)
		{"neither", 0, "", ModeNone, 0, ""},

		// Special values
		{"auto level", 0, "auto", ModeAuto, -1, LevelAuto},
		{"none level", 0, "none", ModeNone, 0, LevelNone},
		{"auto budget", -1, "", ModeAuto, -1, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeMixedConfig(tt.inputBudget, tt.inputLevel)
			if got.Mode != tt.wantMode {
				t.Errorf("normalizeMixedConfig(%d, %q) Mode = %v, want %v", tt.inputBudget, tt.inputLevel, got.Mode, tt.wantMode)
			}
			if got.Budget != tt.wantBudget {
				t.Errorf("normalizeMixedConfig(%d, %q) Budget = %d, want %d", tt.inputBudget, tt.inputLevel, got.Budget, tt.wantBudget)
			}
			if got.Level != tt.wantLevel {
				t.Errorf("normalizeMixedConfig(%d, %q) Level = %q, want %q", tt.inputBudget, tt.inputLevel, got.Level, tt.wantLevel)
			}
		})
	}
}

// TestNormalizeForModel tests model-aware format normalization.
func TestNormalizeForModel(t *testing.T) {
	budgetOnlyModel := &registry.ModelInfo{
		Thinking: &registry.ThinkingSupport{
			Min: 1024,
			Max: 128000,
		},
	}
	levelOnlyModel := &registry.ModelInfo{
		Thinking: &registry.ThinkingSupport{
			Levels: []string{"low", "medium", "high"},
		},
	}
	hybridModel := &registry.ModelInfo{
		Thinking: &registry.ThinkingSupport{
			Min:    128,
			Max:    32768,
			Levels: []string{"minimal", "low", "medium", "high"},
		},
	}

	tests := []struct {
		name    string
		config  ThinkingConfig
		model   *registry.ModelInfo
		want    ThinkingConfig
		wantErr bool
	}{
		{"budget-only keeps budget", ThinkingConfig{Mode: ModeBudget, Budget: 8192}, budgetOnlyModel, ThinkingConfig{Mode: ModeBudget, Budget: 8192}, false},
		{"budget-only converts level", ThinkingConfig{Mode: ModeLevel, Level: LevelHigh}, budgetOnlyModel, ThinkingConfig{Mode: ModeBudget, Budget: 24576}, false},
		{"level-only converts budget", ThinkingConfig{Mode: ModeBudget, Budget: 8192}, levelOnlyModel, ThinkingConfig{Mode: ModeLevel, Level: LevelMedium}, false},
		{"level-only keeps level", ThinkingConfig{Mode: ModeLevel, Level: LevelLow}, levelOnlyModel, ThinkingConfig{Mode: ModeLevel, Level: LevelLow}, false},
		{"hybrid keeps budget", ThinkingConfig{Mode: ModeBudget, Budget: 16384}, hybridModel, ThinkingConfig{Mode: ModeBudget, Budget: 16384}, false},
		{"hybrid keeps level", ThinkingConfig{Mode: ModeLevel, Level: LevelMinimal}, hybridModel, ThinkingConfig{Mode: ModeLevel, Level: LevelMinimal}, false},
		{"auto passthrough", ThinkingConfig{Mode: ModeAuto, Budget: -1}, levelOnlyModel, ThinkingConfig{Mode: ModeAuto, Budget: -1}, false},
		{"none passthrough", ThinkingConfig{Mode: ModeNone, Budget: 0}, budgetOnlyModel, ThinkingConfig{Mode: ModeNone, Budget: 0}, false},
		{"invalid level", ThinkingConfig{Mode: ModeLevel, Level: "ultra"}, budgetOnlyModel, ThinkingConfig{}, true},
		{"invalid budget", ThinkingConfig{Mode: ModeBudget, Budget: -2}, levelOnlyModel, ThinkingConfig{}, true},
		{"nil modelInfo passthrough budget", ThinkingConfig{Mode: ModeBudget, Budget: 8192}, nil, ThinkingConfig{Mode: ModeBudget, Budget: 8192}, false},
		{"nil modelInfo passthrough level", ThinkingConfig{Mode: ModeLevel, Level: LevelHigh}, nil, ThinkingConfig{Mode: ModeLevel, Level: LevelHigh}, false},
		{"nil thinking degrades to none", ThinkingConfig{Mode: ModeBudget, Budget: 4096}, &registry.ModelInfo{}, ThinkingConfig{Mode: ModeNone, Budget: 0}, false},
		{"nil thinking level degrades to none", ThinkingConfig{Mode: ModeLevel, Level: LevelHigh}, &registry.ModelInfo{}, ThinkingConfig{Mode: ModeNone, Budget: 0}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeForModel(&tt.config, tt.model)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NormalizeForModel(%+v) error = %v, wantErr %v", tt.config, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got == nil {
				t.Fatalf("NormalizeForModel(%+v) returned nil config", tt.config)
			}
			if got.Mode != tt.want.Mode {
				t.Errorf("NormalizeForModel(%+v) Mode = %v, want %v", tt.config, got.Mode, tt.want.Mode)
			}
			if got.Budget != tt.want.Budget {
				t.Errorf("NormalizeForModel(%+v) Budget = %d, want %d", tt.config, got.Budget, tt.want.Budget)
			}
			if got.Level != tt.want.Level {
				t.Errorf("NormalizeForModel(%+v) Level = %q, want %q", tt.config, got.Level, tt.want.Level)
			}
		})
	}
}

// TestLevelToBudgetRoundTrip tests level → budget → level round trip.
//
// Verifies that converting level to budget and back produces consistent results.
//
// Depends on: Epic 4 Story 4-1, 4-2
func TestLevelToBudgetRoundTrip(t *testing.T) {
	levels := []string{"none", "auto", "minimal", "low", "medium", "high", "xhigh"}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			budget, ok := ConvertLevelToBudget(level)
			if !ok {
				t.Fatalf("ConvertLevelToBudget(%q) returned ok=false", level)
			}
			resultLevel, ok := ConvertBudgetToLevel(budget)
			if !ok {
				t.Fatalf("ConvertBudgetToLevel(%d) returned ok=false", budget)
			}
			if resultLevel != level {
				t.Errorf("round trip: %q → %d → %q, want %q", level, budget, resultLevel, level)
			}
		})
	}
}
