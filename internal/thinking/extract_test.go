// Package thinking provides unified thinking configuration processing logic.
package thinking

import "testing"

func TestExtractThinkingConfig(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		provider string
		want     ThinkingConfig
	}{
		{"claude budget", `{"thinking":{"budget_tokens":16384}}`, "claude", ThinkingConfig{Mode: ModeBudget, Budget: 16384}},
		{"claude disabled type", `{"thinking":{"type":"disabled"}}`, "claude", ThinkingConfig{Mode: ModeNone, Budget: 0}},
		{"claude auto budget", `{"thinking":{"budget_tokens":-1}}`, "claude", ThinkingConfig{Mode: ModeAuto, Budget: -1}},
		{"claude enabled type without budget", `{"thinking":{"type":"enabled"}}`, "claude", ThinkingConfig{Mode: ModeAuto, Budget: -1}},
		{"claude enabled type with budget", `{"thinking":{"type":"enabled","budget_tokens":8192}}`, "claude", ThinkingConfig{Mode: ModeBudget, Budget: 8192}},
		{"claude disabled type overrides budget", `{"thinking":{"type":"disabled","budget_tokens":8192}}`, "claude", ThinkingConfig{Mode: ModeNone, Budget: 0}},
		{"gemini budget", `{"generationConfig":{"thinkingConfig":{"thinkingBudget":8192}}}`, "gemini", ThinkingConfig{Mode: ModeBudget, Budget: 8192}},
		{"gemini level", `{"generationConfig":{"thinkingConfig":{"thinkingLevel":"high"}}}`, "gemini", ThinkingConfig{Mode: ModeLevel, Level: LevelHigh}},
		{"gemini cli auto", `{"request":{"generationConfig":{"thinkingConfig":{"thinkingLevel":"auto"}}}}`, "gemini-cli", ThinkingConfig{Mode: ModeAuto, Budget: -1}},
		{"openai level", `{"reasoning_effort":"medium"}`, "openai", ThinkingConfig{Mode: ModeLevel, Level: LevelMedium}},
		{"openai none", `{"reasoning_effort":"none"}`, "openai", ThinkingConfig{Mode: ModeNone, Budget: 0}},
		{"codex effort high", `{"reasoning":{"effort":"high"}}`, "codex", ThinkingConfig{Mode: ModeLevel, Level: LevelHigh}},
		{"codex effort none", `{"reasoning":{"effort":"none"}}`, "codex", ThinkingConfig{Mode: ModeNone, Budget: 0}},
		{"iflow enable", `{"chat_template_kwargs":{"enable_thinking":true}}`, "iflow", ThinkingConfig{Mode: ModeBudget, Budget: 1}},
		{"iflow disable", `{"reasoning_split":false}`, "iflow", ThinkingConfig{Mode: ModeNone, Budget: 0}},
		{"unknown provider", `{"thinking":{"budget_tokens":123}}`, "unknown", ThinkingConfig{}},
		{"invalid json", `{"thinking":`, "claude", ThinkingConfig{}},
		{"empty body", "", "claude", ThinkingConfig{}},
		{"no config", `{}`, "gemini", ThinkingConfig{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractThinkingConfig([]byte(tt.body), tt.provider)
			if got != tt.want {
				t.Fatalf("extractThinkingConfig() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
