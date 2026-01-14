// Package claude implements thinking configuration for Claude models.
package claude

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/thinking"
	"github.com/tidwall/gjson"
)

// =============================================================================
// Unit Tests: Applier Creation and Interface
// =============================================================================

func TestNewApplier(t *testing.T) {
	applier := NewApplier()
	if applier == nil {
		t.Fatal("NewApplier() returned nil")
	}
}

func TestApplierImplementsInterface(t *testing.T) {
	var _ thinking.ProviderApplier = (*Applier)(nil)
}

// =============================================================================
// Unit Tests: Budget and Disable Logic (Pre-validated Config)
// =============================================================================

// TestClaudeApplyBudgetAndNone tests budget values and disable modes.
// NOTE: These tests assume config has been pre-validated by ValidateConfig.
// Apply trusts the input and does not perform clamping.
func TestClaudeApplyBudgetAndNone(t *testing.T) {
	applier := NewApplier()
	modelInfo := buildClaudeModelInfo()

	tests := []struct {
		name         string
		config       thinking.ThinkingConfig
		wantType     string
		wantBudget   int
		wantBudgetOK bool
	}{
		// Valid pre-validated budget values
		{"budget 16k", thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 16384}, "enabled", 16384, true},
		{"budget min", thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 1024}, "enabled", 1024, true},
		{"budget max", thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 128000}, "enabled", 128000, true},
		{"budget mid", thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 50000}, "enabled", 50000, true},
		// Disable cases
		{"budget zero disables", thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 0}, "disabled", 0, false},
		{"mode none disables", thinking.ThinkingConfig{Mode: thinking.ModeNone, Budget: 0}, "disabled", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := applier.Apply([]byte(`{}`), tt.config, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			thinkingType := gjson.GetBytes(result, "thinking.type").String()
			if thinkingType != tt.wantType {
				t.Fatalf("thinking.type = %q, want %q", thinkingType, tt.wantType)
			}

			budgetValue := gjson.GetBytes(result, "thinking.budget_tokens")
			if budgetValue.Exists() != tt.wantBudgetOK {
				t.Fatalf("thinking.budget_tokens exists = %v, want %v", budgetValue.Exists(), tt.wantBudgetOK)
			}
			if tt.wantBudgetOK {
				if got := int(budgetValue.Int()); got != tt.wantBudget {
					t.Fatalf("thinking.budget_tokens = %d, want %d", got, tt.wantBudget)
				}
			}
		})
	}
}

// TestClaudeApplyPassthroughBudget tests that Apply trusts pre-validated budget values.
// It does NOT perform clamping - that's ValidateConfig's responsibility.
func TestClaudeApplyPassthroughBudget(t *testing.T) {
	applier := NewApplier()
	modelInfo := buildClaudeModelInfo()

	tests := []struct {
		name       string
		config     thinking.ThinkingConfig
		wantBudget int
	}{
		// Apply should pass through the budget value as-is
		// (ValidateConfig would have clamped these, but Apply trusts the input)
		{"passes through any budget", thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 500}, 500},
		{"passes through large budget", thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 200000}, 200000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := applier.Apply([]byte(`{}`), tt.config, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			if got := int(gjson.GetBytes(result, "thinking.budget_tokens").Int()); got != tt.wantBudget {
				t.Fatalf("thinking.budget_tokens = %d, want %d (passthrough)", got, tt.wantBudget)
			}
		})
	}
}

// =============================================================================
// Unit Tests: Mode Passthrough (Strict Layering)
// =============================================================================

// TestClaudeApplyModePassthrough tests that non-Budget/None modes pass through unchanged.
// Apply expects ValidateConfig to have already converted Level/Auto to Budget.
func TestClaudeApplyModePassthrough(t *testing.T) {
	applier := NewApplier()
	modelInfo := buildClaudeModelInfo()

	tests := []struct {
		name   string
		config thinking.ThinkingConfig
		body   string
	}{
		{"ModeLevel passes through", thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: "high"}, `{"model":"test"}`},
		{"ModeAuto passes through", thinking.ThinkingConfig{Mode: thinking.ModeAuto, Budget: -1}, `{"model":"test"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := applier.Apply([]byte(tt.body), tt.config, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			// Should return body unchanged
			if string(result) != tt.body {
				t.Fatalf("Apply() = %s, want %s (passthrough)", string(result), tt.body)
			}
		})
	}
}

// =============================================================================
// Unit Tests: Output Format
// =============================================================================

// TestClaudeApplyOutputFormat tests the exact JSON output format.
//
// Claude expects:
//
//	{
//	  "thinking": {
//	    "type": "enabled",
//	    "budget_tokens": 16384
//	  }
//	}
func TestClaudeApplyOutputFormat(t *testing.T) {
	tests := []struct {
		name     string
		config   thinking.ThinkingConfig
		wantJSON string
	}{
		{
			"enabled with budget",
			thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 16384},
			`{"thinking":{"type":"enabled","budget_tokens":16384}}`,
		},
		{
			"disabled",
			thinking.ThinkingConfig{Mode: thinking.ModeNone, Budget: 0},
			`{"thinking":{"type":"disabled"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applier := NewApplier()
			modelInfo := buildClaudeModelInfo()

			result, err := applier.Apply([]byte(`{}`), tt.config, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if string(result) != tt.wantJSON {
				t.Fatalf("Apply() = %s, want %s", result, tt.wantJSON)
			}
		})
	}
}

// =============================================================================
// Unit Tests: Body Merging
// =============================================================================

// TestClaudeApplyWithExistingBody tests applying config to existing request body.
func TestClaudeApplyWithExistingBody(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		config   thinking.ThinkingConfig
		wantBody string
	}{
		{
			"add to empty body",
			`{}`,
			thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 16384},
			`{"thinking":{"type":"enabled","budget_tokens":16384}}`,
		},
		{
			"preserve existing fields",
			`{"model":"claude-sonnet-4-5","messages":[]}`,
			thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 8192},
			`{"model":"claude-sonnet-4-5","messages":[],"thinking":{"type":"enabled","budget_tokens":8192}}`,
		},
		{
			"override existing thinking",
			`{"thinking":{"type":"enabled","budget_tokens":1000}}`,
			thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 16384},
			`{"thinking":{"type":"enabled","budget_tokens":16384}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applier := NewApplier()
			modelInfo := buildClaudeModelInfo()

			result, err := applier.Apply([]byte(tt.body), tt.config, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if string(result) != tt.wantBody {
				t.Fatalf("Apply() = %s, want %s", result, tt.wantBody)
			}
		})
	}
}

// TestClaudeApplyWithNilBody tests handling of nil/empty body.
func TestClaudeApplyWithNilBody(t *testing.T) {
	applier := NewApplier()
	modelInfo := buildClaudeModelInfo()

	tests := []struct {
		name       string
		body       []byte
		wantBudget int
	}{
		{"nil body", nil, 16384},
		{"empty body", []byte{}, 16384},
		{"empty object", []byte(`{}`), 16384},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 16384}
			result, err := applier.Apply(tt.body, config, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			if got := gjson.GetBytes(result, "thinking.type").String(); got != "enabled" {
				t.Fatalf("thinking.type = %q, want %q", got, "enabled")
			}
			if got := int(gjson.GetBytes(result, "thinking.budget_tokens").Int()); got != tt.wantBudget {
				t.Fatalf("thinking.budget_tokens = %d, want %d", got, tt.wantBudget)
			}
		})
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

func buildClaudeModelInfo() *registry.ModelInfo {
	return &registry.ModelInfo{
		ID: "claude-sonnet-4-5",
		Thinking: &registry.ThinkingSupport{
			Min:            1024,
			Max:            128000,
			ZeroAllowed:    true,
			DynamicAllowed: false,
		},
	}
}
