// Package iflow implements thinking configuration for iFlow models (GLM, MiniMax).
package iflow

import (
	"bytes"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/thinking"
	"github.com/tidwall/gjson"
)

func TestNewApplier(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applier := NewApplier()
			if applier == nil {
				t.Fatalf("expected non-nil applier")
			}
		})
	}
}

func TestApplierImplementsInterface(t *testing.T) {
	tests := []struct {
		name    string
		applier thinking.ProviderApplier
	}{
		{"default", NewApplier()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.applier == nil {
				t.Fatalf("expected thinking.ProviderApplier implementation")
			}
		})
	}
}

func TestApplyNilModelInfo(t *testing.T) {
	applier := NewApplier()

	tests := []struct {
		name string
		body []byte
	}{
		{"nil body", nil},
		{"empty body", []byte{}},
		{"json body", []byte(`{"model":"glm-4.6"}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applier.Apply(tt.body, thinking.ThinkingConfig{}, nil)
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if !bytes.Equal(got, tt.body) {
				t.Fatalf("expected body unchanged, got %s", string(got))
			}
		})
	}
}

func TestApplyMissingThinkingSupport(t *testing.T) {
	applier := NewApplier()

	tests := []struct {
		name    string
		modelID string
	}{
		{"model id", "glm-4.6"},
		{"empty model id", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelInfo := &registry.ModelInfo{ID: tt.modelID}
			body := []byte(`{"model":"` + tt.modelID + `"}`)
			got, err := applier.Apply(body, thinking.ThinkingConfig{}, modelInfo)
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if string(got) != string(body) {
				t.Fatalf("expected body unchanged, got %s", string(got))
			}
		})
	}
}

func TestConfigToBoolean(t *testing.T) {
	tests := []struct {
		name   string
		config thinking.ThinkingConfig
		want   bool
	}{
		{"mode none", thinking.ThinkingConfig{Mode: thinking.ModeNone}, false},
		{"mode auto", thinking.ThinkingConfig{Mode: thinking.ModeAuto}, true},
		{"budget zero", thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 0}, false},
		{"budget positive", thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 1000}, true},
		{"level none", thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelNone}, false},
		{"level minimal", thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelMinimal}, true},
		{"level low", thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelLow}, true},
		{"level medium", thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelMedium}, true},
		{"level high", thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelHigh}, true},
		{"level xhigh", thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelXHigh}, true},
		{"zero value config", thinking.ThinkingConfig{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := configToBoolean(tt.config); got != tt.want {
				t.Fatalf("configToBoolean(%+v) = %v, want %v", tt.config, got, tt.want)
			}
		})
	}
}

func TestApplyGLM(t *testing.T) {
	applier := NewApplier()

	tests := []struct {
		name         string
		modelID      string
		body         []byte
		config       thinking.ThinkingConfig
		wantEnable   bool
		wantPreserve string
	}{
		{"mode none", "glm-4.6", []byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeNone}, false, ""},
		{"level none", "glm-4.7", []byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelNone}, false, ""},
		{"mode auto", "glm-4.6", []byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeAuto}, true, ""},
		{"level minimal", "glm-4.6", []byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelMinimal}, true, ""},
		{"level low", "glm-4.7", []byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelLow}, true, ""},
		{"level medium", "glm-4.6", []byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelMedium}, true, ""},
		{"level high", "GLM-4.6", []byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelHigh}, true, ""},
		{"level xhigh", "glm-z1-preview", []byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelXHigh}, true, ""},
		{"budget zero", "glm-4.6", []byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 0}, false, ""},
		{"budget 1000", "glm-4.6", []byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 1000}, true, ""},
		{"preserve fields", "glm-4.6", []byte(`{"model":"glm-4.6","extra":{"keep":true}}`), thinking.ThinkingConfig{Mode: thinking.ModeAuto}, true, "glm-4.6"},
		{"empty body", "glm-4.6", nil, thinking.ThinkingConfig{Mode: thinking.ModeAuto}, true, ""},
		{"malformed json", "glm-4.6", []byte(`{invalid`), thinking.ThinkingConfig{Mode: thinking.ModeAuto}, true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelInfo := &registry.ModelInfo{
				ID:       tt.modelID,
				Thinking: &registry.ThinkingSupport{},
			}
			got, err := applier.Apply(tt.body, tt.config, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if !gjson.ValidBytes(got) {
				t.Fatalf("expected valid JSON, got %s", string(got))
			}

			enableResult := gjson.GetBytes(got, "chat_template_kwargs.enable_thinking")
			if !enableResult.Exists() {
				t.Fatalf("enable_thinking missing")
			}
			gotEnable := enableResult.Bool()
			if gotEnable != tt.wantEnable {
				t.Fatalf("enable_thinking = %v, want %v", gotEnable, tt.wantEnable)
			}

			// clear_thinking only set when enable_thinking=true
			clearResult := gjson.GetBytes(got, "chat_template_kwargs.clear_thinking")
			if tt.wantEnable {
				if !clearResult.Exists() {
					t.Fatalf("clear_thinking missing when enable_thinking=true")
				}
				if clearResult.Bool() {
					t.Fatalf("clear_thinking = %v, want false", clearResult.Bool())
				}
			} else {
				if clearResult.Exists() {
					t.Fatalf("clear_thinking should not exist when enable_thinking=false")
				}
			}

			if tt.wantPreserve != "" {
				gotModel := gjson.GetBytes(got, "model").String()
				if gotModel != tt.wantPreserve {
					t.Fatalf("model = %q, want %q", gotModel, tt.wantPreserve)
				}
				if !gjson.GetBytes(got, "extra.keep").Bool() {
					t.Fatalf("expected extra.keep preserved")
				}
			}
		})
	}
}

func TestApplyMiniMax(t *testing.T) {
	applier := NewApplier()

	tests := []struct {
		name      string
		modelID   string
		body      []byte
		config    thinking.ThinkingConfig
		wantSplit bool
		wantModel string
		wantKeep  bool
	}{
		{"mode none", "minimax-m2", []byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeNone}, false, "", false},
		{"level none", "minimax-m2.1", []byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelNone}, false, "", false},
		{"mode auto", "minimax-m2", []byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeAuto}, true, "", false},
		{"level high", "MINIMAX-M2", []byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelHigh}, true, "", false},
		{"level low", "minimax-m2.1", []byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelLow}, true, "", false},
		{"level minimal", "minimax-m2", []byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelMinimal}, true, "", false},
		{"level medium", "minimax-m2", []byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelMedium}, true, "", false},
		{"level xhigh", "minimax-m2", []byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelXHigh}, true, "", false},
		{"budget zero", "minimax-m2", []byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 0}, false, "", false},
		{"budget 1000", "minimax-m2.1", []byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeBudget, Budget: 1000}, true, "", false},
		{"unknown level", "minimax-m2", []byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: "unknown"}, true, "", false},
		{"preserve fields", "minimax-m2", []byte(`{"model":"minimax-m2","extra":{"keep":true}}`), thinking.ThinkingConfig{Mode: thinking.ModeAuto}, true, "minimax-m2", true},
		{"empty body", "minimax-m2", nil, thinking.ThinkingConfig{Mode: thinking.ModeAuto}, true, "", false},
		{"malformed json", "minimax-m2", []byte(`{invalid`), thinking.ThinkingConfig{Mode: thinking.ModeAuto}, true, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelInfo := &registry.ModelInfo{
				ID:       tt.modelID,
				Thinking: &registry.ThinkingSupport{},
			}
			got, err := applier.Apply(tt.body, tt.config, modelInfo)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if !gjson.ValidBytes(got) {
				t.Fatalf("expected valid JSON, got %s", string(got))
			}

			splitResult := gjson.GetBytes(got, "reasoning_split")
			if !splitResult.Exists() {
				t.Fatalf("reasoning_split missing")
			}
			// Verify JSON type is boolean, not string
			if splitResult.Type != gjson.True && splitResult.Type != gjson.False {
				t.Fatalf("reasoning_split should be boolean, got type %v", splitResult.Type)
			}
			gotSplit := splitResult.Bool()
			if gotSplit != tt.wantSplit {
				t.Fatalf("reasoning_split = %v, want %v", gotSplit, tt.wantSplit)
			}

			if tt.wantModel != "" {
				gotModel := gjson.GetBytes(got, "model").String()
				if gotModel != tt.wantModel {
					t.Fatalf("model = %q, want %q", gotModel, tt.wantModel)
				}
				if tt.wantKeep && !gjson.GetBytes(got, "extra.keep").Bool() {
					t.Fatalf("expected extra.keep preserved")
				}
			}
		})
	}
}

// TestIsGLMModel tests the GLM model detection.
//
// Depends on: Epic 9 Story 9-1
func TestIsGLMModel(t *testing.T) {
	tests := []struct {
		name    string
		model   string
		wantGLM bool
	}{
		{"glm-4.6", "glm-4.6", true},
		{"glm-z1-preview", "glm-z1-preview", true},
		{"glm uppercase", "GLM-4.7", true},
		{"minimax-01", "minimax-01", false},
		{"gpt-5.2", "gpt-5.2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isGLMModel(tt.model); got != tt.wantGLM {
				t.Fatalf("isGLMModel(%q) = %v, want %v", tt.model, got, tt.wantGLM)
			}
		})
	}
}

// TestIsMiniMaxModel tests the MiniMax model detection.
//
// Depends on: Epic 9 Story 9-1
func TestIsMiniMaxModel(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		wantMiniMax bool
	}{
		{"minimax-01", "minimax-01", true},
		{"minimax uppercase", "MINIMAX-M2", true},
		{"glm-4.6", "glm-4.6", false},
		{"gpt-5.2", "gpt-5.2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMiniMaxModel(tt.model); got != tt.wantMiniMax {
				t.Fatalf("isMiniMaxModel(%q) = %v, want %v", tt.model, got, tt.wantMiniMax)
			}
		})
	}
}
