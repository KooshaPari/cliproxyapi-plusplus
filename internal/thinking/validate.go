// Package thinking provides unified thinking configuration processing logic.
package thinking

import (
	"fmt"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	log "github.com/sirupsen/logrus"
)

// ClampBudget clamps a budget value to the specified range [min, max].
//
// This function ensures budget values stay within model-supported bounds.
// When clamping occurs, a Debug-level log is recorded.
//
// Special handling:
//   - Auto value (-1) passes through without clamping
//   - Values below min are clamped to min
//   - Values above max are clamped to max
//
// Parameters:
//   - value: The budget value to clamp
//   - min: Minimum allowed budget (inclusive)
//   - max: Maximum allowed budget (inclusive)
//
// Returns:
//   - The clamped budget value (min ≤ result ≤ max, or -1 for auto)
//
// Logging:
//   - Debug level when value is clamped (either to min or max)
//   - Fields: original_value, clamped_to, min, max
func ClampBudget(value, min, max int) int {
	// Auto value (-1) passes through without clamping
	if value == -1 {
		return value
	}

	// Clamp to min if below
	if value < min {
		logClamp(value, min, min, max)
		return min
	}

	// Clamp to max if above
	if value > max {
		logClamp(value, max, min, max)
		return max
	}

	// Within range, return original
	return value
}

// ClampBudgetWithZeroCheck clamps a budget value to the specified range [min, max]
// while honoring the ZeroAllowed constraint.
//
// This function extends ClampBudget with ZeroAllowed boundary handling.
// When zeroAllowed is false and value is 0, the value is clamped to min and logged.
//
// Parameters:
//   - value: The budget value to clamp
//   - min: Minimum allowed budget (inclusive)
//   - max: Maximum allowed budget (inclusive)
//   - zeroAllowed: Whether 0 (thinking disabled) is allowed
//
// Returns:
//   - The clamped budget value (min ≤ result ≤ max, or -1 for auto)
//
// Logging:
//   - Warn level when zeroAllowed=false and value=0 (zero not allowed for model)
//   - Fields: original_value, clamped_to, reason
func ClampBudgetWithZeroCheck(value, min, max int, zeroAllowed bool) int {
	if value == 0 {
		if zeroAllowed {
			return 0
		}
		log.WithFields(log.Fields{
			"clamped_to": min,
			"min":        min,
			"max":        max,
		}).Warn("thinking: budget zero not allowed")
		return min
	}

	return ClampBudget(value, min, max)
}

// ValidateConfig validates a thinking configuration against model capabilities.
//
// This function performs comprehensive validation:
//   - Checks if the model supports thinking
//   - Auto-converts between Budget and Level formats based on model capability
//   - Validates that requested level is in the model's supported levels list
//   - Clamps budget values to model's allowed range
//
// Parameters:
//   - config: The thinking configuration to validate
//   - support: Model's ThinkingSupport properties (nil means no thinking support)
//
// Returns:
//   - Normalized ThinkingConfig with clamped values
//   - ThinkingError if validation fails (ErrThinkingNotSupported, ErrLevelNotSupported, etc.)
//
// Auto-conversion behavior:
//   - Budget-only model + Level config → Level converted to Budget
//   - Level-only model + Budget config → Budget converted to Level
//   - Hybrid model → preserve original format
func ValidateConfig(config ThinkingConfig, support *registry.ThinkingSupport) (*ThinkingConfig, error) {
	normalized := config
	if support == nil {
		if config.Mode != ModeNone {
			return nil, NewThinkingErrorWithModel(ErrThinkingNotSupported, "thinking not supported for this model", "unknown")
		}
		return &normalized, nil
	}

	capability := detectModelCapability(&registry.ModelInfo{Thinking: support})
	switch capability {
	case CapabilityBudgetOnly:
		if normalized.Mode == ModeLevel {
			if normalized.Level == LevelAuto {
				break
			}
			budget, ok := ConvertLevelToBudget(string(normalized.Level))
			if !ok {
				return nil, NewThinkingError(ErrUnknownLevel, fmt.Sprintf("unknown level: %s", normalized.Level))
			}
			normalized.Mode = ModeBudget
			normalized.Budget = budget
			normalized.Level = ""
		}
	case CapabilityLevelOnly:
		if normalized.Mode == ModeBudget {
			level, ok := ConvertBudgetToLevel(normalized.Budget)
			if !ok {
				return nil, NewThinkingError(ErrUnknownLevel, fmt.Sprintf("budget %d cannot be converted to a valid level", normalized.Budget))
			}
			normalized.Mode = ModeLevel
			normalized.Level = ThinkingLevel(level)
			normalized.Budget = 0
		}
	case CapabilityHybrid:
	}

	if normalized.Mode == ModeLevel && normalized.Level == LevelNone {
		normalized.Mode = ModeNone
		normalized.Budget = 0
		normalized.Level = ""
	}
	if normalized.Mode == ModeLevel && normalized.Level == LevelAuto {
		normalized.Mode = ModeAuto
		normalized.Budget = -1
		normalized.Level = ""
	}
	if normalized.Mode == ModeBudget && normalized.Budget == 0 {
		normalized.Mode = ModeNone
		normalized.Level = ""
	}

	if len(support.Levels) > 0 && normalized.Mode == ModeLevel {
		if !isLevelSupported(string(normalized.Level), support.Levels) {
			validLevels := normalizeLevels(support.Levels)
			message := fmt.Sprintf("level %q not supported, valid levels: %s", strings.ToLower(string(normalized.Level)), strings.Join(validLevels, ", "))
			return nil, NewThinkingError(ErrLevelNotSupported, message)
		}
	}

	// Convert ModeAuto to mid-range if dynamic not allowed
	if normalized.Mode == ModeAuto && !support.DynamicAllowed {
		normalized = convertAutoToMidRange(normalized, support)
	}

	switch normalized.Mode {
	case ModeBudget, ModeAuto, ModeNone:
		clamped := ClampBudgetWithZeroCheck(normalized.Budget, support.Min, support.Max, support.ZeroAllowed)
		normalized.Budget = clamped
	}

	// ModeNone with clamped Budget > 0: set Level to lowest for Level-only/Hybrid models
	// This ensures Apply layer doesn't need to access support.Levels
	if normalized.Mode == ModeNone && normalized.Budget > 0 && len(support.Levels) > 0 {
		normalized.Level = ThinkingLevel(support.Levels[0])
	}

	return &normalized, nil
}

func isLevelSupported(level string, supported []string) bool {
	for _, candidate := range supported {
		if strings.EqualFold(level, strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}

func normalizeLevels(levels []string) []string {
	normalized := make([]string, 0, len(levels))
	for _, level := range levels {
		normalized = append(normalized, strings.ToLower(strings.TrimSpace(level)))
	}
	return normalized
}

// convertAutoToMidRange converts ModeAuto to a mid-range value when dynamic is not allowed.
//
// This function handles the case where a model does not support dynamic/auto thinking.
// The auto mode is silently converted to a fixed value based on model capability:
//   - Level-only models: convert to ModeLevel with LevelMedium
//   - Budget models: convert to ModeBudget with mid = (Min + Max) / 2
//
// Logging:
//   - Debug level when conversion occurs
//   - Fields: original_mode, clamped_to, reason
func convertAutoToMidRange(config ThinkingConfig, support *registry.ThinkingSupport) ThinkingConfig {
	// For level-only models (has Levels but no Min/Max range), use ModeLevel with medium
	if len(support.Levels) > 0 && support.Min == 0 && support.Max == 0 {
		config.Mode = ModeLevel
		config.Level = LevelMedium
		config.Budget = 0
		log.WithFields(log.Fields{
			"original_mode": "auto",
			"clamped_to":    string(LevelMedium),
			"reason":        "dynamic_not_allowed_level_only",
		}).Debug("thinking mode converted: dynamic not allowed, using medium level")
		return config
	}

	// For budget models, use mid-range budget
	mid := (support.Min + support.Max) / 2
	if mid <= 0 && support.ZeroAllowed {
		config.Mode = ModeNone
		config.Budget = 0
	} else if mid <= 0 {
		config.Mode = ModeBudget
		config.Budget = support.Min
	} else {
		config.Mode = ModeBudget
		config.Budget = mid
	}
	log.WithFields(log.Fields{
		"original_mode": "auto",
		"clamped_to":    config.Budget,
		"reason":        "dynamic_not_allowed",
	}).Debug("thinking mode converted: dynamic not allowed")
	return config
}

// logClamp logs a debug message when budget clamping occurs.
func logClamp(original, clampedTo, min, max int) {
	log.WithFields(log.Fields{
		"original_value": original,
		"min":            min,
		"max":            max,
		"clamped_to":     clampedTo,
	}).Debug("thinking: budget clamped")
}
