// Package openai implements thinking configuration for OpenAI/Codex models.
package openai

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/thinking"
	"github.com/tidwall/gjson"
)

func buildOpenAIModelInfo(modelID string) *registry.ModelInfo {
	info := registry.LookupStaticModelInfo(modelID)
	if info != nil {
		return info
	}
	// Fallback with complete ThinkingSupport matching real OpenAI model capabilities
	return &registry.ModelInfo{
		ID: modelID,
		Thinking: &registry.ThinkingSupport{
			Min:         1024,
			Max:         32768,
			ZeroAllowed: true,
			Levels:      []string{"none", "low", "medium", "high", "xhigh"},
		},
	}
}

func TestNewApplier(t *testing.T) {
	applier := NewApplier()
	if applier == nil {
		t.Fatalf("expected non-nil applier")
	}
}

func TestApplierImplementsInterface(t *testing.T) {
	_, ok := interface{}(NewApplier()).(thinking.ProviderApplier)
	if !ok {
		t.Fatalf("expected Applier to implement thinking.ProviderApplier")
	}
}

func TestApplyNilModelInfo(t *testing.T) {
	applier := NewApplier()
	body := []byte(`{"model":"gpt-5.2"}`)
	config := thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelHigh}
	got, err := applier.Apply(body, config, nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	// nil modelInfo now applies compatible config
	if !gjson.GetBytes(got, "reasoning_effort").Exists() {
		t.Fatalf("expected reasoning_effort applied, got %s", string(got))
	}
}

func TestApplyMissingThinkingSupport(t *testing.T) {
	applier := NewApplier()
	modelInfo := &registry.ModelInfo{ID: "gpt-5.2"}
	body := []byte(`{"model":"gpt-5.2"}`)
	got, err := applier.Apply(body, thinking.ThinkingConfig{}, modelInfo)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("expected body unchanged, got %s", string(got))
	}
}

// TestApplyLevel tests Apply with ModeLevel (unit test, no ValidateConfig).
func TestApplyLevel(t *testing.T) {
	applier := NewApplier()
	modelInfo := buildOpenAIModelInfo("gpt-5.2")

	tests := []struct {
		name  string
		level thinking.ThinkingLevel
		want  string
	}{
		{"high", thinking.LevelHigh, "high"},
		{"medium", thinking.LevelMedium, "medium"},
		{"low", thinking.LevelLow, "low"},
		{"xhigh", thinking.LevelXHigh, "xhigh"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := applier.Apply([]byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: tt.level}, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if got := gjson.GetBytes(result, "reasoning_effort").String(); got != tt.want {
				t.Fatalf("reasoning_effort = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestApplyModeNone tests Apply with ModeNone (unit test).
func TestApplyModeNone(t *testing.T) {
	applier := NewApplier()

	tests := []struct {
		name      string
		config    thinking.ThinkingConfig
		modelInfo *registry.ModelInfo
		want      string
	}{
		{"zero allowed", thinking.ThinkingConfig{Mode: thinking.ModeNone, Budget: 0}, &registry.ModelInfo{ID: "gpt-5.2", Thinking: &registry.ThinkingSupport{ZeroAllowed: true, Levels: []string{"none", "low"}}}, "none"},
		{"clamped to level", thinking.ThinkingConfig{Mode: thinking.ModeNone, Budget: 128, Level: thinking.LevelLow}, &registry.ModelInfo{ID: "gpt-5", Thinking: &registry.ThinkingSupport{Levels: []string{"minimal", "low"}}}, "low"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := applier.Apply([]byte(`{}`), tt.config, tt.modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if got := gjson.GetBytes(result, "reasoning_effort").String(); got != tt.want {
				t.Fatalf("reasoning_effort = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestApplyPassthrough tests that unsupported modes pass through unchanged.
func TestApplyPassthrough(t *testing.T) {
	applier := NewApplier()
	modelInfo := buildOpenAIModelInfo("gpt-5.2")

	tests := []struct {
		name   string
		config thinking.ThinkingConfig
	}{
		{"mode auto", thinking.ThinkingConfig{Mode: thinking.ModeAuto}},
		{"mode budget", thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 8192}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(`{"model":"gpt-5.2"}`)
			result, err := applier.Apply(body, tt.config, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if string(result) != string(body) {
				t.Fatalf("Apply() result = %s, want %s", string(result), string(body))
			}
		})
	}
}

// TestApplyInvalidBody tests Apply with invalid body input.
func TestApplyInvalidBody(t *testing.T) {
	applier := NewApplier()
	modelInfo := buildOpenAIModelInfo("gpt-5.2")

	tests := []struct {
		name string
		body []byte
	}{
		{"nil body", nil},
		{"empty body", []byte{}},
		{"invalid json", []byte(`{"not json"`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := applier.Apply(tt.body, thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelHigh}, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if !gjson.ValidBytes(result) {
				t.Fatalf("Apply() result is not valid JSON: %s", string(result))
			}
			if got := gjson.GetBytes(result, "reasoning_effort").String(); got != "high" {
				t.Fatalf("reasoning_effort = %q, want %q", got, "high")
			}
		})
	}
}

// TestApplyPreservesFields tests that existing body fields are preserved.
func TestApplyPreservesFields(t *testing.T) {
	applier := NewApplier()
	modelInfo := buildOpenAIModelInfo("gpt-5.2")

	body := []byte(`{"model":"gpt-5.2","messages":[]}`)
	result, err := applier.Apply(body, thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelLow}, modelInfo)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got := gjson.GetBytes(result, "model").String(); got != "gpt-5.2" {
		t.Fatalf("model = %q, want %q", got, "gpt-5.2")
	}
	if !gjson.GetBytes(result, "messages").Exists() {
		t.Fatalf("messages missing from result: %s", string(result))
	}
	if got := gjson.GetBytes(result, "reasoning_effort").String(); got != "low" {
		t.Fatalf("reasoning_effort = %q, want %q", got, "low")
	}
}

// TestHasLevel tests the hasLevel helper function.
func TestHasLevel(t *testing.T) {
	tests := []struct {
		name   string
		levels []string
		target string
		want   bool
	}{
		{"exact match", []string{"low", "medium", "high"}, "medium", true},
		{"case insensitive", []string{"low", "medium", "high"}, "MEDIUM", true},
		{"with spaces", []string{"low", " medium ", "high"}, "medium", true},
		{"not found", []string{"low", "medium", "high"}, "xhigh", false},
		{"empty levels", []string{}, "medium", false},
		{"none level", []string{"none", "low", "medium"}, "none", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasLevel(tt.levels, tt.target); got != tt.want {
				t.Fatalf("hasLevel(%v, %q) = %v, want %v", tt.levels, tt.target, got, tt.want)
			}
		})
	}
}

// --- End-to-End Tests (ValidateConfig → Apply) ---

// TestE2EApply tests the full flow: ValidateConfig → Apply.
func TestE2EApply(t *testing.T) {
	tests := []struct {
		name   string
		model  string
		config thinking.ThinkingConfig
		want   string
	}{
		{"level high", "gpt-5.2", thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelHigh}, "high"},
		{"level medium", "gpt-5.2", thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelMedium}, "medium"},
		{"level low", "gpt-5.2", thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelLow}, "low"},
		{"level xhigh", "gpt-5.2", thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelXHigh}, "xhigh"},
		{"mode none", "gpt-5.2", thinking.ThinkingConfig{Mode: thinking.ModeNone, Budget: 0}, "none"},
		{"budget to level", "gpt-5.2", thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 8192}, "medium"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelInfo := buildOpenAIModelInfo(tt.model)
			normalized, err := thinking.ValidateConfig(tt.config, modelInfo.Thinking)
			if err != nil {
				t.Fatalf("ValidateConfig() error = %v", err)
			}

			applier := NewApplier()
			result, err := applier.Apply([]byte(`{}`), *normalized, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if got := gjson.GetBytes(result, "reasoning_effort").String(); got != tt.want {
				t.Fatalf("reasoning_effort = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestE2EApplyOutputFormat tests the full flow with exact JSON output verification.
func TestE2EApplyOutputFormat(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		config   thinking.ThinkingConfig
		wantJSON string
	}{
		{"level high", "gpt-5.2", thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelHigh}, `{"reasoning_effort":"high"}`},
		{"level none", "gpt-5.2", thinking.ThinkingConfig{Mode: thinking.ModeNone, Budget: 0}, `{"reasoning_effort":"none"}`},
		{"budget converted", "gpt-5.2", thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 8192}, `{"reasoning_effort":"medium"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelInfo := buildOpenAIModelInfo(tt.model)
			normalized, err := thinking.ValidateConfig(tt.config, modelInfo.Thinking)
			if err != nil {
				t.Fatalf("ValidateConfig() error = %v", err)
			}

			applier := NewApplier()
			result, err := applier.Apply([]byte(`{}`), *normalized, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if string(result) != tt.wantJSON {
				t.Fatalf("Apply() result = %s, want %s", string(result), tt.wantJSON)
			}
		})
	}
}

// TestE2EApplyWithExistingBody tests the full flow with existing body fields.
func TestE2EApplyWithExistingBody(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		config     thinking.ThinkingConfig
		wantEffort string
		wantModel  string
	}{
		{"empty body", `{}`, thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelHigh}, "high", ""},
		{"preserve fields", `{"model":"gpt-5.2","messages":[]}`, thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelMedium}, "medium", "gpt-5.2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelInfo := buildOpenAIModelInfo("gpt-5.2")
			normalized, err := thinking.ValidateConfig(tt.config, modelInfo.Thinking)
			if err != nil {
				t.Fatalf("ValidateConfig() error = %v", err)
			}

			applier := NewApplier()
			result, err := applier.Apply([]byte(tt.body), *normalized, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if got := gjson.GetBytes(result, "reasoning_effort").String(); got != tt.wantEffort {
				t.Fatalf("reasoning_effort = %q, want %q", got, tt.wantEffort)
			}
			if tt.wantModel != "" {
				if got := gjson.GetBytes(result, "model").String(); got != tt.wantModel {
					t.Fatalf("model = %q, want %q", got, tt.wantModel)
				}
			}
		})
	}
}
