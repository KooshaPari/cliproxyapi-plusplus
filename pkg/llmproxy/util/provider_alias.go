package util

import "strings"

// NormalizeProviderAlias maps legacy/alternate provider names to their canonical keys.
func NormalizeProviderAlias(provider string) string {
	normalized := strings.ToLower(strings.TrimSpace(provider))
	switch normalized {
	case "github-copilot":
		return "copilot"
	case "githubcopilot":
		return "copilot"
	case "ampcode":
		return "amp"
	case "amp-code":
		return "amp"
	case "kilo-code":
		return "kilo"
	case "kilocode":
		return "kilo"
	case "roo-code":
		return "roo"
	case "roocode":
		return "roo"
	case "droid":
		return "gemini"
	case "droid-cli":
		return "gemini"
	case "droidcli":
		return "gemini"
	case "factoryapi":
		return "factory-api"
	case "openai-compatible":
		return "factory-api"
	default:
		return normalized
	}
}
