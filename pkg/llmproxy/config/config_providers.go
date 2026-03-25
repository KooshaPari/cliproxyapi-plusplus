package config

import (
	"strings"
)

// GetDedicatedProviders returns providers that have a dedicated config block.
func GetDedicatedProviders() []ProviderSpec {
	var out []ProviderSpec
	for _, p := range AllProviders {
		if p.YAMLKey != "" {
			out = append(out, p)
		}
	}
	return out
}

// GetPremadeProviders returns providers that can be injected from environment variables.
func GetPremadeProviders() []ProviderSpec {
	var out []ProviderSpec
	for _, p := range AllProviders {
		if len(p.EnvVars) > 0 {
			out = append(out, p)
		}
	}
	return out
}

// GetProviderByName looks up a provider by its name (case-insensitive).
func GetProviderByName(name string) (ProviderSpec, bool) {
	for _, p := range AllProviders {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return ProviderSpec{}, false
}
