package util

import (
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
)

// ModelSupportsThinking reports whether the given model has Thinking capability
// according to the model registry metadata (provider-agnostic).
//
// Deprecated: Use thinking.ApplyThinking with modelInfo.Thinking check.
func ModelSupportsThinking(model string) bool {
	if model == "" {
		return false
	}
	// First check the global dynamic registry
	if info := registry.GetGlobalRegistry().GetModelInfo(model); info != nil {
		return info.Thinking != nil
	}
	// Fallback: check static model definitions
	if info := registry.LookupStaticModelInfo(model); info != nil {
		return info.Thinking != nil
	}
	// Fallback: check Antigravity static config
	if cfg := registry.GetAntigravityModelConfig()[model]; cfg != nil {
		return cfg.Thinking != nil
	}
	return false
}

// NormalizeThinkingBudget clamps the requested thinking budget to the
// supported range for the specified model using registry metadata only.
// If the model is unknown or has no Thinking metadata, returns the original budget.
// For dynamic (-1), returns -1 if DynamicAllowed; otherwise approximates mid-range
// or min (0 if zero is allowed and mid <= 0).
//
// Deprecated: Use thinking.ValidateConfig for budget normalization.
func NormalizeThinkingBudget(model string, budget int) int {
	if budget == -1 { // dynamic
		if found, minBudget, maxBudget, zeroAllowed, dynamicAllowed := thinkingRangeFromRegistry(model); found {
			if dynamicAllowed {
				return -1
			}
			mid := (minBudget + maxBudget) / 2
			if mid <= 0 && zeroAllowed {
				return 0
			}
			if mid <= 0 {
				return minBudget
			}
			return mid
		}
		return -1
	}
	if found, minBudget, maxBudget, zeroAllowed, _ := thinkingRangeFromRegistry(model); found {
		if budget == 0 {
			if zeroAllowed {
				return 0
			}
			return minBudget
		}
		if budget < minBudget {
			return minBudget
		}
		if budget > maxBudget {
			return maxBudget
		}
		return budget
	}
	return budget
}

// thinkingRangeFromRegistry attempts to read thinking ranges from the model registry.
func thinkingRangeFromRegistry(model string) (found bool, min int, max int, zeroAllowed bool, dynamicAllowed bool) {
	if model == "" {
		return false, 0, 0, false, false
	}
	// First check global dynamic registry
	if info := registry.GetGlobalRegistry().GetModelInfo(model); info != nil && info.Thinking != nil {
		return true, info.Thinking.Min, info.Thinking.Max, info.Thinking.ZeroAllowed, info.Thinking.DynamicAllowed
	}
	// Fallback: check static model definitions
	if info := registry.LookupStaticModelInfo(model); info != nil && info.Thinking != nil {
		return true, info.Thinking.Min, info.Thinking.Max, info.Thinking.ZeroAllowed, info.Thinking.DynamicAllowed
	}
	// Fallback: check Antigravity static config
	if cfg := registry.GetAntigravityModelConfig()[model]; cfg != nil && cfg.Thinking != nil {
		return true, cfg.Thinking.Min, cfg.Thinking.Max, cfg.Thinking.ZeroAllowed, cfg.Thinking.DynamicAllowed
	}
	return false, 0, 0, false, false
}

// ThinkingLevelToBudget maps a Gemini thinkingLevel to a numeric thinking budget (tokens).
//
// Mappings:
//   - "minimal" -> 512
//   - "low"     -> 1024
//   - "medium"  -> 8192
//   - "high"    -> 32768
//
// Returns false when the level is empty or unsupported.
//
// Deprecated: Use thinking.ConvertLevelToBudget instead.
func ThinkingLevelToBudget(level string) (int, bool) {
	if level == "" {
		return 0, false
	}
	normalized := strings.ToLower(strings.TrimSpace(level))
	switch normalized {
	case "minimal":
		return 512, true
	case "low":
		return 1024, true
	case "medium":
		return 8192, true
	case "high":
		return 32768, true
	default:
		return 0, false
	}
}
