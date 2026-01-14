// Package thinking provides unified thinking configuration processing logic.
package thinking

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
)

// TestClampBudget tests the ClampBudget function.
//
// ClampBudget applies range constraints to a budget value:
//   - budget < Min → clamp to Min (with Debug log)
//   - budget > Max → clamp to Max (with Debug log)
//   - Auto value (-1) passes through unchanged
func TestClampBudget(t *testing.T) {
	tests := []struct {
		name  string
		value int
		min   int
		max   int
		want  int
	}{
		// Within range - no clamping
		{"within range", 8192, 128, 32768, 8192},
		{"at min", 128, 128, 32768, 128},
		{"at max", 32768, 128, 32768, 32768},

		// Below min - clamp to min
		{"below min", 100, 128, 32768, 128},

		// Above max - clamp to max
		{"above max", 50000, 128, 32768, 32768},

		// Edge cases
		{"min equals max", 5000, 5000, 5000, 5000},
		{"zero min zero value", 0, 0, 100, 0},

		// Auto value (-1) - passes through
		{"auto value", -1, 128, 32768, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClampBudget(tt.value, tt.min, tt.max)
			if got != tt.want {
				t.Errorf("ClampBudget(%d, %d, %d) = %d, want %d",
					tt.value, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

// TestZeroAllowedBoundaryHandling tests ZeroAllowed=false edge cases.
//
// When ZeroAllowed=false and user requests 0, clamp to Min + log Warn.
func TestZeroAllowedBoundaryHandling(t *testing.T) {
	tests := []struct {
		name        string
		value       int
		min         int
		max         int
		zeroAllowed bool
		want        int
	}{
		// ZeroAllowed=true: 0 stays 0
		{"zero allowed - keep zero", 0, 128, 32768, true, 0},

		// ZeroAllowed=false: 0 clamps to min
		{"zero not allowed - clamp to min", 0, 128, 32768, false, 128},

		// ZeroAllowed=false but non-zero value: normal clamping
		{"zero not allowed - positive value", 8192, 1024, 100000, false, 8192},

		// Auto value (-1) always passes through
		{"auto value", -1, 128, 32768, false, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClampBudgetWithZeroCheck(tt.value, tt.min, tt.max, tt.zeroAllowed)
			if got != tt.want {
				t.Errorf("ClampBudgetWithZeroCheck(%d, %d, %d, %v) = %d, want %d",
					tt.value, tt.min, tt.max, tt.zeroAllowed, got, tt.want)
			}
		})
	}
}

// TestValidateConfigFramework verifies the ValidateConfig function framework.
// This test is merged into TestValidateConfig for consolidation.

// TestValidateConfigNotSupported verifies nil support handling.
// This test is merged into TestValidateConfig for consolidation.

// TestValidateConfigConversion verifies mode conversion based on capability.
// This test is merged into TestValidateConfig for consolidation.

// TestValidateConfigLevelSupport verifies level list validation.
// This test is merged into TestValidateConfig for consolidation.

// TestValidateConfigClamping verifies budget clamping behavior.
// This test is merged into TestValidateConfig for consolidation.

// TestValidateConfig is the comprehensive test for ValidateConfig function.
//
// ValidateConfig checks if a ThinkingConfig is valid for a given model.
// This test covers all validation scenarios including:
//   - Framework basics (nil support with ModeNone)
//   - Error cases (thinking not supported, level not supported, dynamic not allowed)
//   - Mode conversion (budget-only, level-only, hybrid)
//   - Budget clamping (to max, to min)
//   - ZeroAllowed boundary handling (ModeNone with ZeroAllowed=false)
//   - DynamicAllowed validation
//
// Depends on: Epic 5 Story 5-3 (config validity validation)
func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name       string
		config     ThinkingConfig
		support    *registry.ThinkingSupport
		wantMode   ThinkingMode
		wantBudget int
		wantLevel  ThinkingLevel
		wantErr    bool
		wantCode   ErrorCode
	}{
		// Framework basics
		{"nil support mode none", ThinkingConfig{Mode: ModeNone, Budget: 0}, nil, ModeNone, 0, "", false, ""},

		// Valid configs - no conversion needed
		{"budget-only keeps budget", ThinkingConfig{Mode: ModeBudget, Budget: 8192}, &registry.ThinkingSupport{Min: 1024, Max: 100000}, ModeBudget, 8192, "", false, ""},

		// Auto-conversion: Level → Budget
		{"budget-only converts level", ThinkingConfig{Mode: ModeLevel, Level: LevelHigh}, &registry.ThinkingSupport{Min: 1024, Max: 100000}, ModeBudget, 24576, "", false, ""},

		// Auto-conversion: Budget → Level
		{"level-only converts budget", ThinkingConfig{Mode: ModeBudget, Budget: 5000}, &registry.ThinkingSupport{Levels: []string{"low", "medium", "high"}}, ModeLevel, 0, LevelMedium, false, ""},

		// Hybrid preserves original format
		{"hybrid preserves level", ThinkingConfig{Mode: ModeLevel, Level: LevelLow}, &registry.ThinkingSupport{Min: 128, Max: 32768, Levels: []string{"low", "high"}}, ModeLevel, 0, LevelLow, false, ""},

		// Budget clamping
		{"budget clamped to max", ThinkingConfig{Mode: ModeBudget, Budget: 200000}, &registry.ThinkingSupport{Min: 1024, Max: 100000}, ModeBudget, 100000, "", false, ""},
		{"budget clamped to min", ThinkingConfig{Mode: ModeBudget, Budget: 100}, &registry.ThinkingSupport{Min: 1024, Max: 100000}, ModeBudget, 1024, "", false, ""},

		// Error: thinking not supported
		{"thinking not supported", ThinkingConfig{Mode: ModeBudget, Budget: 8192}, nil, 0, 0, "", true, ErrThinkingNotSupported},

		// Error: level not in list
		{"level not supported", ThinkingConfig{Mode: ModeLevel, Level: LevelXHigh}, &registry.ThinkingSupport{Levels: []string{"low", "medium", "high"}}, 0, 0, "", true, ErrLevelNotSupported},

		// Level case-insensitive
		{"level supported case-insensitive", ThinkingConfig{Mode: ModeLevel, Level: ThinkingLevel("HIGH")}, &registry.ThinkingSupport{Levels: []string{"low", "medium", "high"}}, ModeLevel, 0, ThinkingLevel("HIGH"), false, ""},

		// ModeAuto with DynamicAllowed
		{"auto with dynamic allowed", ThinkingConfig{Mode: ModeAuto, Budget: -1}, &registry.ThinkingSupport{Min: 128, Max: 32768, DynamicAllowed: true}, ModeAuto, -1, "", false, ""},

		// ModeAuto with DynamicAllowed=false - converts to mid-range (M3)
		{"auto with dynamic not allowed", ThinkingConfig{Mode: ModeAuto, Budget: -1}, &registry.ThinkingSupport{Min: 128, Max: 32768, DynamicAllowed: false}, ModeBudget, 16448, "", false, ""},

		// ModeNone with ZeroAllowed=true - stays as ModeNone
		{"mode none with zero allowed", ThinkingConfig{Mode: ModeNone, Budget: 0}, &registry.ThinkingSupport{Min: 1024, Max: 100000, ZeroAllowed: true}, ModeNone, 0, "", false, ""},

		// Budget=0 converts to ModeNone before clamping (M1)
		{"budget zero converts to none", ThinkingConfig{Mode: ModeBudget, Budget: 0}, &registry.ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: false}, ModeNone, 128, "", false, ""},

		// Level=none converts to ModeNone before clamping, then Level set to lowest
		{"level none converts to none", ThinkingConfig{Mode: ModeLevel, Level: LevelNone}, &registry.ThinkingSupport{Min: 128, Max: 32768, Levels: []string{"low", "high"}, ZeroAllowed: false}, ModeNone, 128, ThinkingLevel("low"), false, ""},
		{"level auto converts to auto", ThinkingConfig{Mode: ModeLevel, Level: LevelAuto}, &registry.ThinkingSupport{Min: 128, Max: 32768, Levels: []string{"low", "high"}, DynamicAllowed: true}, ModeAuto, -1, "", false, ""},
		// M1: Level=auto with DynamicAllowed=false - converts to mid-range budget
		{"level auto with dynamic not allowed", ThinkingConfig{Mode: ModeLevel, Level: LevelAuto}, &registry.ThinkingSupport{Min: 128, Max: 32768, Levels: []string{"low", "high"}, DynamicAllowed: false}, ModeBudget, 16448, "", false, ""},
		// M2: Level=auto on Budget-only model (no Levels)
		{"level auto on budget-only model", ThinkingConfig{Mode: ModeLevel, Level: LevelAuto}, &registry.ThinkingSupport{Min: 128, Max: 32768, DynamicAllowed: true}, ModeAuto, -1, "", false, ""},

		// ModeNone with ZeroAllowed=false - clamps to min but preserves ModeNone (M1)
		{"mode none with zero not allowed - preserve mode", ThinkingConfig{Mode: ModeNone, Budget: 0}, &registry.ThinkingSupport{Min: 1024, Max: 100000, ZeroAllowed: false}, ModeNone, 1024, "", false, ""},

		// ModeNone with clamped Budget > 0 and Levels: sets Level to lowest
		{"mode none clamped with levels", ThinkingConfig{Mode: ModeNone, Budget: 0}, &registry.ThinkingSupport{Min: 128, Max: 32768, Levels: []string{"low", "high"}, ZeroAllowed: false}, ModeNone, 128, ThinkingLevel("low"), false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateConfig(tt.config, tt.support)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ValidateConfig(%+v, support) error = nil, want %v", tt.config, tt.wantCode)
				}
				thinkingErr, ok := err.(*ThinkingError)
				if !ok {
					t.Fatalf("ValidateConfig(%+v, support) error type = %T, want *ThinkingError", tt.config, err)
				}
				if thinkingErr.Code != tt.wantCode {
					t.Errorf("ValidateConfig(%+v, support) code = %v, want %v", tt.config, thinkingErr.Code, tt.wantCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateConfig(%+v, support) returned error: %v", tt.config, err)
			}
			if got == nil {
				t.Fatalf("ValidateConfig(%+v, support) returned nil config", tt.config)
			}
			if got.Mode != tt.wantMode {
				t.Errorf("ValidateConfig(%+v, support) Mode = %v, want %v", tt.config, got.Mode, tt.wantMode)
			}
			if got.Budget != tt.wantBudget {
				t.Errorf("ValidateConfig(%+v, support) Budget = %d, want %d", tt.config, got.Budget, tt.wantBudget)
			}
			if got.Level != tt.wantLevel {
				t.Errorf("ValidateConfig(%+v, support) Level = %q, want %q", tt.config, got.Level, tt.wantLevel)
			}
		})
	}
}

// TestValidationErrorMessages tests error message formatting.
//
// Error messages should:
//   - Be lowercase
//   - Have no trailing period
//   - Include context with %s/%d
//
// Depends on: Epic 5 Story 5-4 (validation error messages)
func TestValidationErrorMessages(t *testing.T) {
	tests := []struct {
		name         string
		getErr       func() error
		wantCode     ErrorCode
		wantContains string
	}{
		{"invalid suffix", func() error {
			_, err := ParseSuffixWithError("model(abc")
			return err
		}, ErrInvalidSuffix, "model(abc"},
		{"level not supported", func() error {
			_, err := ValidateConfig(ThinkingConfig{Mode: ModeLevel, Level: LevelXHigh}, &registry.ThinkingSupport{Levels: []string{"low", "medium", "high"}})
			return err
		}, ErrLevelNotSupported, "valid levels: low, medium, high"},
		{"thinking not supported", func() error {
			_, err := ValidateConfig(ThinkingConfig{Mode: ModeBudget, Budget: 1024}, nil)
			return err
		}, ErrThinkingNotSupported, "thinking not supported for this model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.getErr()
			if err == nil {
				t.Fatalf("error = nil, want ThinkingError")
			}
			thinkingErr, ok := err.(*ThinkingError)
			if !ok {
				t.Fatalf("error type = %T, want *ThinkingError", err)
			}
			if thinkingErr.Code != tt.wantCode {
				t.Errorf("code = %v, want %v", thinkingErr.Code, tt.wantCode)
			}
			if thinkingErr.Message == "" {
				t.Fatalf("message is empty")
			}
			first, _ := utf8.DecodeRuneInString(thinkingErr.Message)
			if unicode.IsLetter(first) && !unicode.IsLower(first) {
				t.Errorf("message does not start with lowercase: %q", thinkingErr.Message)
			}
			if strings.HasSuffix(thinkingErr.Message, ".") {
				t.Errorf("message has trailing period: %q", thinkingErr.Message)
			}
			if !strings.Contains(thinkingErr.Message, tt.wantContains) {
				t.Errorf("message %q does not contain %q", thinkingErr.Message, tt.wantContains)
			}
		})
	}
}

// TestClampingLogging tests that clamping produces correct log entries.
//
// Clamping behavior:
//   - Normal clamp (budget outside range) → Debug log
//   - ZeroAllowed=false + zero request → Warn log
//
// Depends on: Epic 5 Story 5-1, 5-2
func TestClampingLogging(t *testing.T) {
	tests := []struct {
		name         string
		useZeroCheck bool
		budget       int
		min          int
		max          int
		zeroAllowed  bool
		wantLevel    log.Level
		wantReason   string
		wantClamped  int
	}{
		{"above max - debug", false, 50000, 128, 32768, false, log.DebugLevel, "", 32768},
		{"below min - debug", false, 50, 128, 32768, false, log.DebugLevel, "", 128},
		{"zero not allowed - warn", true, 0, 128, 32768, false, log.WarnLevel, "zero_not_allowed", 128},
	}

	logger := log.StandardLogger()
	originalLevel := logger.GetLevel()
	logger.SetLevel(log.DebugLevel)
	hook := logtest.NewLocal(logger)
	t.Cleanup(func() {
		logger.SetLevel(originalLevel)
		hook.Reset()
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook.Reset()
			var got int
			if tt.useZeroCheck {
				got = ClampBudgetWithZeroCheck(tt.budget, tt.min, tt.max, tt.zeroAllowed)
			} else {
				got = ClampBudget(tt.budget, tt.min, tt.max)
			}
			if got != tt.wantClamped {
				t.Fatalf("clamped budget = %d, want %d", got, tt.wantClamped)
			}

			entry := hook.LastEntry()
			if entry == nil {
				t.Fatalf("no log entry captured")
			}
			if entry.Level != tt.wantLevel {
				t.Errorf("log level = %v, want %v", entry.Level, tt.wantLevel)
			}

			fields := []string{"original_value", "clamped_to", "min", "max"}
			for _, key := range fields {
				if _, ok := entry.Data[key]; !ok {
					t.Errorf("missing field %q", key)
				}
			}
			if tt.wantReason != "" {
				if value, ok := entry.Data["reason"]; !ok || value != tt.wantReason {
					t.Errorf("reason = %v, want %v", value, tt.wantReason)
				}
			}
		})
	}
}
