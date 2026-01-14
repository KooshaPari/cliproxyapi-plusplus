// Package thinking_test provides external tests for the thinking package.
package thinking_test

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/thinking"
	"github.com/tidwall/gjson"

	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/thinking/provider/claude"
	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/thinking/provider/gemini"
	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/thinking/provider/geminicli"
	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/thinking/provider/iflow"
	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/thinking/provider/openai"
)

// registerTestModels sets up test models in the registry and returns a cleanup function.
func registerTestModels(t *testing.T) func() {
	t.Helper()
	reg := registry.GetGlobalRegistry()

	testModels := []*registry.ModelInfo{
		geminiBudgetModel(),
		geminiLevelModel(),
		claudeBudgetModel(),
		openAILevelModel(),
		iFlowModel(),
		{ID: "claude-3"},
		{ID: "gemini-2.5-pro-strip"},
		{ID: "glm-4.6-strip"},
	}

	clientID := "test-thinking-models"
	reg.RegisterClient(clientID, "test", testModels)

	return func() {
		reg.UnregisterClient(clientID)
	}
}

// TestApplyThinking tests the main ApplyThinking entry point.
//
// ApplyThinking is the unified entry point for applying thinking configuration.
// It routes to the appropriate provider-specific applier based on model.
//
// Depends on: Epic 10 Story 10-2 (apply-thinking main entry)
func TestApplyThinking(t *testing.T) {
	cleanup := registerTestModels(t)
	defer cleanup()

	tests := []struct {
		name     string
		body     string
		model    string
		provider string
		check    string
	}{
		{"gemini budget", `{"generationConfig":{"thinkingConfig":{"thinkingBudget":8192}}}`, "gemini-2.5-pro-test", "gemini", "geminiBudget"},
		{"gemini level", `{"generationConfig":{"thinkingConfig":{"thinkingLevel":"high"}}}`, "gemini-3-pro-preview-test", "gemini", "geminiLevel"},
		{"gemini-cli budget", `{"request":{"generationConfig":{"thinkingConfig":{"thinkingBudget":8192}}}}`, "gemini-2.5-pro-test", "gemini-cli", "geminiCliBudget"},
		{"antigravity budget", `{"request":{"generationConfig":{"thinkingConfig":{"thinkingBudget":8192}}}}`, "gemini-2.5-pro-test", "antigravity", "geminiCliBudget"},
		{"claude budget", `{"thinking":{"budget_tokens":16384}}`, "claude-sonnet-4-5-test", "claude", "claudeBudget"},
		{"claude enabled type auto", `{"thinking":{"type":"enabled"}}`, "claude-sonnet-4-5-test", "claude", "claudeAuto"},
		{"openai level", `{"reasoning_effort":"high"}`, "gpt-5.2-test", "openai", "openaiLevel"},
		{"iflow enable", `{"chat_template_kwargs":{"enable_thinking":true}}`, "glm-4.6-test", "iflow", "iflowEnable"},
		{"unknown provider passthrough", `{"a":1}`, "gemini-2.5-pro-test", "unknown", "passthrough"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := thinking.ApplyThinking([]byte(tt.body), tt.model, tt.provider)
			if err != nil {
				t.Fatalf("ApplyThinking() error = %v", err)
			}
			assertApplyThinkingCheck(t, tt.check, tt.body, got)
		})
	}
}

func TestApplyThinkingErrors(t *testing.T) {
	cleanup := registerTestModels(t)
	defer cleanup()

	tests := []struct {
		name     string
		body     string
		model    string
		provider string
	}{
		{"unsupported level openai", `{"reasoning_effort":"ultra"}`, "gpt-5.2-test", "openai"},
		{"unsupported level gemini", `{"generationConfig":{"thinkingConfig":{"thinkingLevel":"ultra"}}}`, "gemini-3-pro-preview-test", "gemini"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := thinking.ApplyThinking([]byte(tt.body), tt.model, tt.provider)
			if err == nil {
				t.Fatalf("ApplyThinking() error = nil, want error")
			}
			// On validation error, ApplyThinking returns original body (defensive programming)
			if string(got) != tt.body {
				t.Fatalf("ApplyThinking() body = %s, want original body %s", string(got), tt.body)
			}
		})
	}
}

func TestApplyThinkingStripOnUnsupportedModel(t *testing.T) {
	cleanup := registerTestModels(t)
	defer cleanup()

	tests := []struct {
		name      string
		body      string
		model     string
		provider  string
		stripped  []string
		preserved []string
	}{
		{"claude strip", `{"thinking":{"budget_tokens":8192},"model":"claude-3"}`, "claude-3", "claude", []string{"thinking"}, []string{"model"}},
		{"gemini strip", `{"generationConfig":{"thinkingConfig":{"thinkingBudget":8192},"temperature":0.7}}`, "gemini-2.5-pro-strip", "gemini", []string{"generationConfig.thinkingConfig"}, []string{"generationConfig.temperature"}},
		{"iflow strip", `{"chat_template_kwargs":{"enable_thinking":true,"clear_thinking":false,"other":"value"}}`, "glm-4.6-strip", "iflow", []string{"chat_template_kwargs.enable_thinking", "chat_template_kwargs.clear_thinking"}, []string{"chat_template_kwargs.other"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := thinking.ApplyThinking([]byte(tt.body), tt.model, tt.provider)
			if err != nil {
				t.Fatalf("ApplyThinking() error = %v", err)
			}

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

func TestIsUserDefinedModel(t *testing.T) {
	tests := []struct {
		name      string
		modelInfo *registry.ModelInfo
		want      bool
	}{
		{"nil modelInfo", nil, false},
		{"not user-defined no flag", &registry.ModelInfo{ID: "test"}, false},
		{"not user-defined with type", &registry.ModelInfo{ID: "test", Type: "openai"}, false},
		{"user-defined with flag", &registry.ModelInfo{ID: "test", Type: "openai", UserDefined: true}, true},
		{"user-defined flag only", &registry.ModelInfo{ID: "test", UserDefined: true}, true},
		{"has thinking not user-defined", &registry.ModelInfo{ID: "test", Type: "openai", Thinking: &registry.ThinkingSupport{Min: 1024}}, false},
		{"has thinking with user-defined flag", &registry.ModelInfo{ID: "test", Type: "openai", Thinking: &registry.ThinkingSupport{Min: 1024}, UserDefined: true}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := thinking.IsUserDefinedModel(tt.modelInfo); got != tt.want {
				t.Fatalf("IsUserDefinedModel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyThinking_UserDefinedModel(t *testing.T) {
	// Register user-defined test models
	reg := registry.GetGlobalRegistry()
	userDefinedModels := []*registry.ModelInfo{
		{ID: "custom-gpt", Type: "openai", UserDefined: true},
		{ID: "or-claude", Type: "openai", UserDefined: true},
		{ID: "custom-gemini", Type: "gemini", UserDefined: true},
		{ID: "vertex-flash", Type: "gemini", UserDefined: true},
		{ID: "cli-gemini", Type: "gemini", UserDefined: true},
		{ID: "ag-gemini", Type: "gemini", UserDefined: true},
		{ID: "custom-claude", Type: "claude", UserDefined: true},
		{ID: "unknown"},
	}
	clientID := "test-user-defined-models"
	reg.RegisterClient(clientID, "test", userDefinedModels)
	defer reg.UnregisterClient(clientID)

	tests := []struct {
		name     string
		body     string
		model    string
		provider string
		check    string
	}{
		{
			"openai user-defined with reasoning_effort",
			`{"model":"custom-gpt","reasoning_effort":"high"}`,
			"custom-gpt",
			"openai",
			"openaiCompatible",
		},
		{
			"openai-compatibility model with reasoning_effort",
			`{"model":"or-claude","reasoning_effort":"high"}`,
			"or-claude",
			"openai",
			"openaiCompatible",
		},
		{
			"gemini user-defined with thinkingBudget",
			`{"model":"custom-gemini","generationConfig":{"thinkingConfig":{"thinkingBudget":8192}}}`,
			"custom-gemini",
			"gemini",
			"geminiCompatibleBudget",
		},
		{
			"vertex user-defined with thinkingBudget",
			`{"model":"vertex-flash","generationConfig":{"thinkingConfig":{"thinkingBudget":16384}}}`,
			"vertex-flash",
			"gemini",
			"geminiCompatibleBudget16384",
		},
		{
			"gemini-cli user-defined with thinkingBudget",
			`{"model":"cli-gemini","request":{"generationConfig":{"thinkingConfig":{"thinkingBudget":8192}}}}`,
			"cli-gemini",
			"gemini-cli",
			"geminiCliCompatibleBudget",
		},
		{
			"antigravity user-defined with thinkingBudget",
			`{"model":"ag-gemini","request":{"generationConfig":{"thinkingConfig":{"thinkingBudget":8192}}}}`,
			"ag-gemini",
			"antigravity",
			"geminiCliCompatibleBudget",
		},
		{
			"claude user-defined with thinking",
			`{"model":"custom-claude","thinking":{"type":"enabled","budget_tokens":8192}}`,
			"custom-claude",
			"claude",
			"claudeCompatibleBudget",
		},
		{
			"user-defined model no config",
			`{"model":"custom-gpt","messages":[]}`,
			"custom-gpt",
			"openai",
			"passthrough",
		},
		{
			"non-user-defined model strips config",
			`{"model":"unknown","reasoning_effort":"high"}`,
			"unknown",
			"openai",
			"stripReasoning",
		},
		{
			"user-defined model unknown provider",
			`{"model":"custom-gpt","reasoning_effort":"high"}`,
			"custom-gpt",
			"unknown",
			"passthrough",
		},
		{
			"unknown model passthrough",
			`{"model":"nonexistent","reasoning_effort":"high"}`,
			"nonexistent",
			"openai",
			"passthrough",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := thinking.ApplyThinking([]byte(tt.body), tt.model, tt.provider)
			if err != nil {
				t.Fatalf("ApplyThinking() error = %v", err)
			}
			assertCompatibleModelCheck(t, tt.check, tt.body, got)
		})
	}
}

// TestApplyThinkingSuffixPriority tests suffix priority over body config.
func TestApplyThinkingSuffixPriority(t *testing.T) {
	// Register test model
	reg := registry.GetGlobalRegistry()
	testModels := []*registry.ModelInfo{
		{
			ID:       "gemini-suffix-test",
			Thinking: &registry.ThinkingSupport{Min: 128, Max: 32768, ZeroAllowed: true},
		},
	}
	clientID := "test-suffix-priority"
	reg.RegisterClient(clientID, "gemini", testModels)
	defer reg.UnregisterClient(clientID)

	tests := []struct {
		name          string
		body          string
		model         string
		provider      string
		checkPath     string
		expectedValue int
	}{
		{
			"suffix overrides body budget",
			`{"generationConfig":{"thinkingConfig":{"thinkingBudget":1000}}}`,
			"gemini-suffix-test(8192)",
			"gemini",
			"generationConfig.thinkingConfig.thinkingBudget",
			8192,
		},
		{
			"suffix none sets budget to 0",
			`{"generationConfig":{"thinkingConfig":{"thinkingBudget":1000}}}`,
			"gemini-suffix-test(none)",
			"gemini",
			"generationConfig.thinkingConfig.thinkingBudget",
			0,
		},
		{
			"no suffix uses body config",
			`{"generationConfig":{"thinkingConfig":{"thinkingBudget":5000}}}`,
			"gemini-suffix-test",
			"gemini",
			"generationConfig.thinkingConfig.thinkingBudget",
			5000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := thinking.ApplyThinking([]byte(tt.body), tt.model, tt.provider)
			if err != nil {
				t.Fatalf("ApplyThinking() error = %v", err)
			}

			result := int(gjson.GetBytes(got, tt.checkPath).Int())
			if result != tt.expectedValue {
				t.Fatalf("ApplyThinking() %s = %v, want %v\nbody: %s", tt.checkPath, result, tt.expectedValue, string(got))
			}
		})
	}
}

func assertApplyThinkingCheck(t *testing.T, checkName, input string, body []byte) {
	t.Helper()

	switch checkName {
	case "geminiBudget":
		assertJSONInt(t, body, "generationConfig.thinkingConfig.thinkingBudget", 8192)
		assertJSONBool(t, body, "generationConfig.thinkingConfig.includeThoughts", true)
	case "geminiLevel":
		assertJSONString(t, body, "generationConfig.thinkingConfig.thinkingLevel", "high")
		assertJSONBool(t, body, "generationConfig.thinkingConfig.includeThoughts", true)
	case "geminiCliBudget":
		assertJSONInt(t, body, "request.generationConfig.thinkingConfig.thinkingBudget", 8192)
		assertJSONBool(t, body, "request.generationConfig.thinkingConfig.includeThoughts", true)
	case "claudeBudget":
		assertJSONString(t, body, "thinking.type", "enabled")
		assertJSONInt(t, body, "thinking.budget_tokens", 16384)
	case "claudeAuto":
		// When type=enabled without budget, auto mode is applied using mid-range budget
		assertJSONString(t, body, "thinking.type", "enabled")
		// Budget should be mid-range: (1024 + 128000) / 2 = 64512
		assertJSONInt(t, body, "thinking.budget_tokens", 64512)
	case "openaiLevel":
		assertJSONString(t, body, "reasoning_effort", "high")
	case "iflowEnable":
		assertJSONBool(t, body, "chat_template_kwargs.enable_thinking", true)
		assertJSONBool(t, body, "chat_template_kwargs.clear_thinking", false)
	case "passthrough":
		if string(body) != input {
			t.Fatalf("ApplyThinking() = %s, want %s", string(body), input)
		}
	default:
		t.Fatalf("unknown check: %s", checkName)
	}
}

func assertCompatibleModelCheck(t *testing.T, checkName, input string, body []byte) {
	t.Helper()

	switch checkName {
	case "openaiCompatible":
		assertJSONString(t, body, "reasoning_effort", "high")
	case "geminiCompatibleBudget":
		assertJSONInt(t, body, "generationConfig.thinkingConfig.thinkingBudget", 8192)
		assertJSONBool(t, body, "generationConfig.thinkingConfig.includeThoughts", true)
	case "geminiCompatibleBudget16384":
		assertJSONInt(t, body, "generationConfig.thinkingConfig.thinkingBudget", 16384)
		assertJSONBool(t, body, "generationConfig.thinkingConfig.includeThoughts", true)
	case "geminiCliCompatibleBudget":
		assertJSONInt(t, body, "request.generationConfig.thinkingConfig.thinkingBudget", 8192)
		assertJSONBool(t, body, "request.generationConfig.thinkingConfig.includeThoughts", true)
	case "claudeCompatibleBudget":
		assertJSONString(t, body, "thinking.type", "enabled")
		assertJSONInt(t, body, "thinking.budget_tokens", 8192)
	case "stripReasoning":
		if gjson.GetBytes(body, "reasoning_effort").Exists() {
			t.Fatalf("expected reasoning_effort to be stripped, got %s", string(body))
		}
	case "passthrough":
		if string(body) != input {
			t.Fatalf("ApplyThinking() = %s, want %s", string(body), input)
		}
	default:
		t.Fatalf("unknown check: %s", checkName)
	}
}

func assertJSONString(t *testing.T, body []byte, path, want string) {
	t.Helper()
	value := gjson.GetBytes(body, path)
	if !value.Exists() {
		t.Fatalf("expected %s to exist", path)
	}
	if value.String() != want {
		t.Fatalf("value at %s = %s, want %s", path, value.String(), want)
	}
}

func assertJSONInt(t *testing.T, body []byte, path string, want int) {
	t.Helper()
	value := gjson.GetBytes(body, path)
	if !value.Exists() {
		t.Fatalf("expected %s to exist", path)
	}
	if int(value.Int()) != want {
		t.Fatalf("value at %s = %d, want %d", path, value.Int(), want)
	}
}

func assertJSONBool(t *testing.T, body []byte, path string, want bool) {
	t.Helper()
	value := gjson.GetBytes(body, path)
	if !value.Exists() {
		t.Fatalf("expected %s to exist", path)
	}
	if value.Bool() != want {
		t.Fatalf("value at %s = %t, want %t", path, value.Bool(), want)
	}
}

func geminiBudgetModel() *registry.ModelInfo {
	return &registry.ModelInfo{
		ID: "gemini-2.5-pro-test",
		Thinking: &registry.ThinkingSupport{
			Min:         128,
			Max:         32768,
			ZeroAllowed: true,
		},
	}
}

func geminiLevelModel() *registry.ModelInfo {
	return &registry.ModelInfo{
		ID: "gemini-3-pro-preview-test",
		Thinking: &registry.ThinkingSupport{
			Min:    128,
			Max:    32768,
			Levels: []string{"minimal", "low", "medium", "high"},
		},
	}
}

func claudeBudgetModel() *registry.ModelInfo {
	return &registry.ModelInfo{
		ID: "claude-sonnet-4-5-test",
		Thinking: &registry.ThinkingSupport{
			Min:         1024,
			Max:         128000,
			ZeroAllowed: true,
		},
	}
}

func openAILevelModel() *registry.ModelInfo {
	return &registry.ModelInfo{
		ID: "gpt-5.2-test",
		Thinking: &registry.ThinkingSupport{
			Min:         128,
			Max:         32768,
			ZeroAllowed: true,
			Levels:      []string{"low", "medium", "high"},
		},
	}
}

func iFlowModel() *registry.ModelInfo {
	return &registry.ModelInfo{
		ID: "glm-4.6-test",
		Thinking: &registry.ThinkingSupport{
			Min:         1,
			Max:         10,
			ZeroAllowed: true,
		},
	}
}
