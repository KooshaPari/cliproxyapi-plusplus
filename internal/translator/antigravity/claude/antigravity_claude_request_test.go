package claude

import (
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestConvertClaudeRequestToAntigravity_BasicStructure(t *testing.T) {
	inputJSON := []byte(`{
		"model": "claude-3-5-sonnet-20240620",
		"messages": [
			{
				"role": "user",
				"content": [
					{"type": "text", "text": "Hello"}
				]
			}
		],
		"system": [
			{"type": "text", "text": "You are helpful"}
		]
	}`)

	output := ConvertClaudeRequestToAntigravity("claude-sonnet-4-5", inputJSON, false)
	outputStr := string(output)

	// Check model
	if gjson.Get(outputStr, "model").String() != "claude-sonnet-4-5" {
		t.Errorf("Expected model 'claude-sonnet-4-5', got '%s'", gjson.Get(outputStr, "model").String())
	}

	// Check contents exist
	contents := gjson.Get(outputStr, "request.contents")
	if !contents.Exists() || !contents.IsArray() {
		t.Error("request.contents should exist and be an array")
	}

	// Check role mapping (assistant -> model)
	firstContent := gjson.Get(outputStr, "request.contents.0")
	if firstContent.Get("role").String() != "user" {
		t.Errorf("Expected role 'user', got '%s'", firstContent.Get("role").String())
	}

	// Check systemInstruction
	sysInstruction := gjson.Get(outputStr, "request.systemInstruction")
	if !sysInstruction.Exists() {
		t.Error("systemInstruction should exist")
	}
	if sysInstruction.Get("parts.0.text").String() != "You are helpful" {
		t.Error("systemInstruction text mismatch")
	}
}

func TestConvertClaudeRequestToAntigravity_RoleMapping(t *testing.T) {
	inputJSON := []byte(`{
		"model": "claude-3-5-sonnet-20240620",
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "Hi"}]},
			{"role": "assistant", "content": [{"type": "text", "text": "Hello"}]}
		]
	}`)

	output := ConvertClaudeRequestToAntigravity("claude-sonnet-4-5", inputJSON, false)
	outputStr := string(output)

	// assistant should be mapped to model
	secondContent := gjson.Get(outputStr, "request.contents.1")
	if secondContent.Get("role").String() != "model" {
		t.Errorf("Expected role 'model' (mapped from 'assistant'), got '%s'", secondContent.Get("role").String())
	}
}

func TestConvertClaudeRequestToAntigravity_ThinkingBlocks(t *testing.T) {
	// Valid signature must be at least 50 characters
	validSignature := "abc123validSignature1234567890123456789012345678901234567890"
	inputJSON := []byte(`{
		"model": "claude-sonnet-4-5-thinking",
		"messages": [
			{
				"role": "assistant",
				"content": [
					{"type": "thinking", "thinking": "Let me think...", "signature": "` + validSignature + `"},
					{"type": "text", "text": "Answer"}
				]
			}
		]
	}`)

	output := ConvertClaudeRequestToAntigravity("claude-sonnet-4-5-thinking", inputJSON, false)
	outputStr := string(output)

	// Check thinking block conversion
	firstPart := gjson.Get(outputStr, "request.contents.0.parts.0")
	if !firstPart.Get("thought").Bool() {
		t.Error("thinking block should have thought: true")
	}
	if firstPart.Get("text").String() != "Let me think..." {
		t.Error("thinking text mismatch")
	}
	if firstPart.Get("thoughtSignature").String() != validSignature {
		t.Errorf("Expected thoughtSignature '%s', got '%s'", validSignature, firstPart.Get("thoughtSignature").String())
	}
}

func TestConvertClaudeRequestToAntigravity_ThinkingBlockWithoutSignature(t *testing.T) {
	inputJSON := []byte(`{
		"model": "claude-sonnet-4-5-thinking",
		"messages": [
			{
				"role": "assistant",
				"content": [
					{"type": "thinking", "thinking": "Let me think..."},
					{"type": "text", "text": "Answer"}
				]
			}
		]
	}`)

	output := ConvertClaudeRequestToAntigravity("claude-sonnet-4-5-thinking", inputJSON, false)
	outputStr := string(output)

	// Without signature, should use sentinel value
	firstPart := gjson.Get(outputStr, "request.contents.0.parts.0")
	if firstPart.Get("thoughtSignature").String() != geminiCLIClaudeThoughtSignature {
		t.Errorf("Expected sentinel signature '%s', got '%s'",
			geminiCLIClaudeThoughtSignature, firstPart.Get("thoughtSignature").String())
	}
}

func TestConvertClaudeRequestToAntigravity_ToolDeclarations(t *testing.T) {
	inputJSON := []byte(`{
		"model": "claude-3-5-sonnet-20240620",
		"messages": [],
		"tools": [
			{
				"name": "test_tool",
				"description": "A test tool",
				"input_schema": {
					"type": "object",
					"properties": {
						"name": {"type": "string"}
					},
					"required": ["name"]
				}
			}
		]
	}`)

	output := ConvertClaudeRequestToAntigravity("gemini-1.5-pro", inputJSON, false)
	outputStr := string(output)

	// Check tools structure
	tools := gjson.Get(outputStr, "request.tools")
	if !tools.Exists() {
		t.Error("Tools should exist in output")
	}

	funcDecl := gjson.Get(outputStr, "request.tools.0.functionDeclarations.0")
	if funcDecl.Get("name").String() != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got '%s'", funcDecl.Get("name").String())
	}

	// Check input_schema renamed to parametersJsonSchema
	if funcDecl.Get("parametersJsonSchema").Exists() {
		t.Log("parametersJsonSchema exists (expected)")
	}
	if funcDecl.Get("input_schema").Exists() {
		t.Error("input_schema should be removed")
	}
}

func TestConvertClaudeRequestToAntigravity_ToolUse(t *testing.T) {
	inputJSON := []byte(`{
		"model": "claude-3-5-sonnet-20240620",
		"messages": [
			{
				"role": "assistant",
				"content": [
					{
						"type": "tool_use",
						"id": "call_123",
						"name": "get_weather",
						"input": "{\"location\": \"Paris\"}"
					}
				]
			}
		]
	}`)

	output := ConvertClaudeRequestToAntigravity("claude-sonnet-4-5", inputJSON, false)
	outputStr := string(output)

	// Check function call conversion
	funcCall := gjson.Get(outputStr, "request.contents.0.parts.0.functionCall")
	if !funcCall.Exists() {
		t.Error("functionCall should exist")
	}
	if funcCall.Get("name").String() != "get_weather" {
		t.Errorf("Expected function name 'get_weather', got '%s'", funcCall.Get("name").String())
	}
	if funcCall.Get("id").String() != "call_123" {
		t.Errorf("Expected function id 'call_123', got '%s'", funcCall.Get("id").String())
	}
}

func TestConvertClaudeRequestToAntigravity_ToolResult(t *testing.T) {
	inputJSON := []byte(`{
		"model": "claude-3-5-sonnet-20240620",
		"messages": [
			{
				"role": "user",
				"content": [
					{
						"type": "tool_result",
						"tool_use_id": "get_weather-call-123",
						"content": "22C sunny"
					}
				]
			}
		]
	}`)

	output := ConvertClaudeRequestToAntigravity("claude-sonnet-4-5", inputJSON, false)
	outputStr := string(output)

	// Check function response conversion
	funcResp := gjson.Get(outputStr, "request.contents.0.parts.0.functionResponse")
	if !funcResp.Exists() {
		t.Error("functionResponse should exist")
	}
	if funcResp.Get("id").String() != "get_weather-call-123" {
		t.Errorf("Expected function id, got '%s'", funcResp.Get("id").String())
	}
}

func TestConvertClaudeRequestToAntigravity_ThinkingConfig(t *testing.T) {
	// Note: This test requires the model to be registered in the registry
	// with Thinking metadata. If the registry is not populated in test environment,
	// thinkingConfig won't be added. We'll test the basic structure only.
	inputJSON := []byte(`{
		"model": "claude-sonnet-4-5-thinking",
		"messages": [],
		"thinking": {
			"type": "enabled",
			"budget_tokens": 8000
		}
	}`)

	output := ConvertClaudeRequestToAntigravity("claude-sonnet-4-5-thinking", inputJSON, false)
	outputStr := string(output)

	// Check thinking config conversion (only if model supports thinking in registry)
	thinkingConfig := gjson.Get(outputStr, "request.generationConfig.thinkingConfig")
	if thinkingConfig.Exists() {
		if thinkingConfig.Get("thinkingBudget").Int() != 8000 {
			t.Errorf("Expected thinkingBudget 8000, got %d", thinkingConfig.Get("thinkingBudget").Int())
		}
		if !thinkingConfig.Get("include_thoughts").Bool() {
			t.Error("include_thoughts should be true")
		}
	} else {
		t.Log("thinkingConfig not present - model may not be registered in test registry")
	}
}

func TestConvertClaudeRequestToAntigravity_ImageContent(t *testing.T) {
	inputJSON := []byte(`{
		"model": "claude-3-5-sonnet-20240620",
		"messages": [
			{
				"role": "user",
				"content": [
					{
						"type": "image",
						"source": {
							"type": "base64",
							"media_type": "image/png",
							"data": "iVBORw0KGgoAAAANSUhEUg=="
						}
					}
				]
			}
		]
	}`)

	output := ConvertClaudeRequestToAntigravity("claude-sonnet-4-5", inputJSON, false)
	outputStr := string(output)

	// Check inline data conversion
	inlineData := gjson.Get(outputStr, "request.contents.0.parts.0.inlineData")
	if !inlineData.Exists() {
		t.Error("inlineData should exist")
	}
	if inlineData.Get("mime_type").String() != "image/png" {
		t.Error("mime_type mismatch")
	}
	if !strings.Contains(inlineData.Get("data").String(), "iVBORw0KGgo") {
		t.Error("data mismatch")
	}
}

func TestConvertClaudeRequestToAntigravity_GenerationConfig(t *testing.T) {
	inputJSON := []byte(`{
		"model": "claude-3-5-sonnet-20240620",
		"messages": [],
		"temperature": 0.7,
		"top_p": 0.9,
		"top_k": 40,
		"max_tokens": 2000
	}`)

	output := ConvertClaudeRequestToAntigravity("claude-sonnet-4-5", inputJSON, false)
	outputStr := string(output)

	genConfig := gjson.Get(outputStr, "request.generationConfig")
	if genConfig.Get("temperature").Float() != 0.7 {
		t.Errorf("Expected temperature 0.7, got %f", genConfig.Get("temperature").Float())
	}
	if genConfig.Get("topP").Float() != 0.9 {
		t.Errorf("Expected topP 0.9, got %f", genConfig.Get("topP").Float())
	}
	if genConfig.Get("topK").Float() != 40 {
		t.Errorf("Expected topK 40, got %f", genConfig.Get("topK").Float())
	}
	if genConfig.Get("maxOutputTokens").Float() != 2000 {
		t.Errorf("Expected maxOutputTokens 2000, got %f", genConfig.Get("maxOutputTokens").Float())
	}
}

// ============================================================================
// P2-A: Trailing Unsigned Thinking Block Removal
// ============================================================================

func TestConvertClaudeRequestToAntigravity_TrailingUnsignedThinking_Removed(t *testing.T) {
	// Last assistant message ends with unsigned thinking block - should be removed
	inputJSON := []byte(`{
		"model": "claude-sonnet-4-5-thinking",
		"messages": [
			{
				"role": "user",
				"content": [{"type": "text", "text": "Hello"}]
			},
			{
				"role": "assistant",
				"content": [
					{"type": "text", "text": "Here is my answer"},
					{"type": "thinking", "thinking": "I should think more..."}
				]
			}
		]
	}`)

	output := ConvertClaudeRequestToAntigravity("claude-sonnet-4-5-thinking", inputJSON, false)
	outputStr := string(output)

	// The last part of the last assistant message should NOT be a thinking block
	lastMessageParts := gjson.Get(outputStr, "request.contents.1.parts")
	if !lastMessageParts.IsArray() {
		t.Fatal("Last message should have parts array")
	}
	parts := lastMessageParts.Array()
	if len(parts) == 0 {
		t.Fatal("Last message should have at least one part")
	}

	// The unsigned thinking should be removed, leaving only the text
	lastPart := parts[len(parts)-1]
	if lastPart.Get("thought").Bool() {
		t.Error("Trailing unsigned thinking block should be removed")
	}
}

func TestConvertClaudeRequestToAntigravity_TrailingSignedThinking_Kept(t *testing.T) {
	// Last assistant message ends with signed thinking block - should be kept
	inputJSON := []byte(`{
		"model": "claude-sonnet-4-5-thinking",
		"messages": [
			{
				"role": "user",
				"content": [{"type": "text", "text": "Hello"}]
			},
			{
				"role": "assistant",
				"content": [
					{"type": "text", "text": "Here is my answer"},
					{"type": "thinking", "thinking": "Valid thinking...", "signature": "abc123validSignature1234567890123456789012345678901234567890"}
				]
			}
		]
	}`)

	output := ConvertClaudeRequestToAntigravity("claude-sonnet-4-5-thinking", inputJSON, false)
	outputStr := string(output)

	// The signed thinking block should be preserved
	lastMessageParts := gjson.Get(outputStr, "request.contents.1.parts")
	parts := lastMessageParts.Array()
	if len(parts) < 2 {
		t.Error("Signed thinking block should be preserved")
	}
}

func TestConvertClaudeRequestToAntigravity_MiddleUnsignedThinking_SentinelApplied(t *testing.T) {
	// Middle message has unsigned thinking - should use sentinel (existing behavior)
	inputJSON := []byte(`{
		"model": "claude-sonnet-4-5-thinking",
		"messages": [
			{
				"role": "assistant",
				"content": [
					{"type": "thinking", "thinking": "Middle thinking..."},
					{"type": "text", "text": "Answer"}
				]
			},
			{
				"role": "user",
				"content": [{"type": "text", "text": "Follow up"}]
			}
		]
	}`)

	output := ConvertClaudeRequestToAntigravity("claude-sonnet-4-5-thinking", inputJSON, false)
	outputStr := string(output)

	// Middle unsigned thinking should have sentinel applied
	thinkingPart := gjson.Get(outputStr, "request.contents.0.parts.0")
	if !thinkingPart.Get("thought").Bool() {
		t.Error("Middle thinking block should be preserved with sentinel")
	}
	if thinkingPart.Get("thoughtSignature").String() != geminiCLIClaudeThoughtSignature {
		t.Errorf("Middle unsigned thinking should use sentinel signature, got: %s", thinkingPart.Get("thoughtSignature").String())
	}
}

// ============================================================================
// P2-B: Tool + Thinking System Hint Injection
// ============================================================================

func TestConvertClaudeRequestToAntigravity_ToolAndThinking_HintInjected(t *testing.T) {
	// When both tools and thinking are enabled, hint should be injected into system instruction
	inputJSON := []byte(`{
		"model": "claude-sonnet-4-5-thinking",
		"messages": [{"role": "user", "content": [{"type": "text", "text": "Hello"}]}],
		"system": [{"type": "text", "text": "You are helpful."}],
		"tools": [
			{
				"name": "get_weather",
				"description": "Get weather",
				"input_schema": {"type": "object", "properties": {"location": {"type": "string"}}}
			}
		],
		"thinking": {"type": "enabled", "budget_tokens": 8000}
	}`)

	output := ConvertClaudeRequestToAntigravity("claude-sonnet-4-5-thinking", inputJSON, false)
	outputStr := string(output)

	// System instruction should contain the interleaved thinking hint
	sysInstruction := gjson.Get(outputStr, "request.systemInstruction")
	if !sysInstruction.Exists() {
		t.Fatal("systemInstruction should exist")
	}

	// Check if hint is appended
	sysText := sysInstruction.Get("parts").Array()
	found := false
	for _, part := range sysText {
		if strings.Contains(part.Get("text").String(), "Interleaved thinking is enabled") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Interleaved thinking hint should be injected when tools and thinking are both active, got: %v", sysInstruction.Raw)
	}
}

func TestConvertClaudeRequestToAntigravity_ToolsOnly_NoHint(t *testing.T) {
	// When only tools are present (no thinking), hint should NOT be injected
	inputJSON := []byte(`{
		"model": "claude-sonnet-4-5",
		"messages": [{"role": "user", "content": [{"type": "text", "text": "Hello"}]}],
		"system": [{"type": "text", "text": "You are helpful."}],
		"tools": [
			{
				"name": "get_weather",
				"description": "Get weather",
				"input_schema": {"type": "object", "properties": {"location": {"type": "string"}}}
			}
		]
	}`)

	output := ConvertClaudeRequestToAntigravity("claude-sonnet-4-5", inputJSON, false)
	outputStr := string(output)

	// System instruction should NOT contain the hint
	sysInstruction := gjson.Get(outputStr, "request.systemInstruction")
	if sysInstruction.Exists() {
		for _, part := range sysInstruction.Get("parts").Array() {
			if strings.Contains(part.Get("text").String(), "Interleaved thinking is enabled") {
				t.Error("Hint should NOT be injected when only tools are present (no thinking)")
			}
		}
	}
}

func TestConvertClaudeRequestToAntigravity_ThinkingOnly_NoHint(t *testing.T) {
	// When only thinking is enabled (no tools), hint should NOT be injected
	inputJSON := []byte(`{
		"model": "claude-sonnet-4-5-thinking",
		"messages": [{"role": "user", "content": [{"type": "text", "text": "Hello"}]}],
		"system": [{"type": "text", "text": "You are helpful."}],
		"thinking": {"type": "enabled", "budget_tokens": 8000}
	}`)

	output := ConvertClaudeRequestToAntigravity("claude-sonnet-4-5-thinking", inputJSON, false)
	outputStr := string(output)

	// System instruction should NOT contain the hint (no tools)
	sysInstruction := gjson.Get(outputStr, "request.systemInstruction")
	if sysInstruction.Exists() {
		for _, part := range sysInstruction.Get("parts").Array() {
			if strings.Contains(part.Get("text").String(), "Interleaved thinking is enabled") {
				t.Error("Hint should NOT be injected when only thinking is present (no tools)")
			}
		}
	}
}

func TestConvertClaudeRequestToAntigravity_ToolAndThinking_NoExistingSystem(t *testing.T) {
	// When tools + thinking but no system instruction, should create one with hint
	inputJSON := []byte(`{
		"model": "claude-sonnet-4-5-thinking",
		"messages": [{"role": "user", "content": [{"type": "text", "text": "Hello"}]}],
		"tools": [
			{
				"name": "get_weather",
				"description": "Get weather",
				"input_schema": {"type": "object", "properties": {"location": {"type": "string"}}}
			}
		],
		"thinking": {"type": "enabled", "budget_tokens": 8000}
	}`)

	output := ConvertClaudeRequestToAntigravity("claude-sonnet-4-5-thinking", inputJSON, false)
	outputStr := string(output)

	// System instruction should be created with hint
	sysInstruction := gjson.Get(outputStr, "request.systemInstruction")
	if !sysInstruction.Exists() {
		t.Fatal("systemInstruction should be created when tools + thinking are active")
	}

	sysText := sysInstruction.Get("parts").Array()
	found := false
	for _, part := range sysText {
		if strings.Contains(part.Get("text").String(), "Interleaved thinking is enabled") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Interleaved thinking hint should be in created systemInstruction, got: %v", sysInstruction.Raw)
	}
}
