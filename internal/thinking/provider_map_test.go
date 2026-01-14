// Package thinking_test provides external tests for the thinking package.
//
// This file uses package thinking_test (external) to allow importing provider
// subpackages, which triggers their init() functions to register appliers.
// This avoids import cycles that would occur if thinking package imported providers directly.
package thinking_test

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/thinking"

	// Blank imports to trigger provider init() registration
	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/thinking/provider/claude"
	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/thinking/provider/gemini"
	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/thinking/provider/geminicli"
	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/thinking/provider/iflow"
	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/thinking/provider/openai"
)

func TestProviderAppliersBasic(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		wantNil  bool
	}{
		{"gemini provider", "gemini", false},
		{"gemini-cli provider", "gemini-cli", false},
		{"claude provider", "claude", false},
		{"openai provider", "openai", false},
		{"iflow provider", "iflow", false},
		{"antigravity provider", "antigravity", false},
		{"unknown provider", "unknown", true},
		{"empty provider", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := thinking.GetProviderApplier(tt.provider)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("GetProviderApplier(%q) = %T, want nil", tt.provider, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("GetProviderApplier(%q) = nil, want non-nil", tt.provider)
			}
		})
	}
}
