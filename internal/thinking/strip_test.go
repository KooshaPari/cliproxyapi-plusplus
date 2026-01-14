// Package thinking_test provides tests for thinking config stripping.
package thinking_test

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/thinking"
	"github.com/tidwall/gjson"
)

func TestStripThinkingConfig(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		provider  string
		stripped  []string
		preserved []string
	}{
		{"claude thinking", `{"thinking":{"budget_tokens":8192},"model":"claude-3"}`, "claude", []string{"thinking"}, []string{"model"}},
		{"gemini thinkingConfig", `{"generationConfig":{"thinkingConfig":{"thinkingBudget":8192},"temperature":0.7}}`, "gemini", []string{"generationConfig.thinkingConfig"}, []string{"generationConfig.temperature"}},
		{"gemini-cli thinkingConfig", `{"request":{"generationConfig":{"thinkingConfig":{"thinkingBudget":8192},"temperature":0.7}}}`, "gemini-cli", []string{"request.generationConfig.thinkingConfig"}, []string{"request.generationConfig.temperature"}},
		{"antigravity thinkingConfig", `{"request":{"generationConfig":{"thinkingConfig":{"thinkingBudget":4096},"maxTokens":1024}}}`, "antigravity", []string{"request.generationConfig.thinkingConfig"}, []string{"request.generationConfig.maxTokens"}},
		{"openai reasoning_effort", `{"reasoning_effort":"high","model":"gpt-5"}`, "openai", []string{"reasoning_effort"}, []string{"model"}},
		{"iflow glm", `{"chat_template_kwargs":{"enable_thinking":true,"clear_thinking":false,"other":"value"}}`, "iflow", []string{"chat_template_kwargs.enable_thinking", "chat_template_kwargs.clear_thinking"}, []string{"chat_template_kwargs.other"}},
		{"iflow minimax", `{"reasoning_split":true,"model":"minimax"}`, "iflow", []string{"reasoning_split"}, []string{"model"}},
		{"iflow both formats", `{"chat_template_kwargs":{"enable_thinking":true,"clear_thinking":false},"reasoning_split":true,"model":"mixed"}`, "iflow", []string{"chat_template_kwargs.enable_thinking", "chat_template_kwargs.clear_thinking", "reasoning_split"}, []string{"model"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := thinking.StripThinkingConfig([]byte(tt.body), tt.provider)

			for _, path := range tt.stripped {
				if gjson.GetBytes(got, path).Exists() {
					t.Fatalf("expected %s to be stripped, got %s", path, string(got))
				}
			}
			for _, path := range tt.preserved {
				if !gjson.GetBytes(got, path).Exists() {
					t.Fatalf("expected %s to be preserved, got %s", path, string(got))
				}
			}
		})
	}
}

func TestStripThinkingConfigPassthrough(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		provider string
	}{
		{"empty body", ``, "claude"},
		{"invalid json", `{not valid`, "claude"},
		{"unknown provider", `{"thinking":{"budget_tokens":8192}}`, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := thinking.StripThinkingConfig([]byte(tt.body), tt.provider)
			if string(got) != tt.body {
				t.Fatalf("StripThinkingConfig() = %s, want passthrough %s", string(got), tt.body)
			}
		})
	}
}
