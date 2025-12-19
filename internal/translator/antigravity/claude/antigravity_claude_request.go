// Package claude provides request translation functionality for Claude Code API compatibility.
// This package handles the conversion of Claude Code API requests into Gemini CLI-compatible
// JSON format, transforming message contents, system instructions, and tool declarations
// into the format expected by Gemini CLI API clients. It performs JSON data transformation
// to ensure compatibility between Claude Code API format and Gemini CLI API's expected format.
package claude

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/cache"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/gemini/common"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const geminiCLIClaudeThoughtSignature = "skip_thought_signature_validator"

// deriveSessionID generates a stable session ID from the request.
// Uses the hash of the first user message to identify the conversation.
func deriveSessionID(rawJSON []byte) string {
	messages := gjson.GetBytes(rawJSON, "messages")
	if !messages.IsArray() {
		return ""
	}
	for _, msg := range messages.Array() {
		if msg.Get("role").String() == "user" {
			content := msg.Get("content").String()
			if content == "" {
				// Try to get text from content array
				content = msg.Get("content.0.text").String()
			}
			if content != "" {
				h := sha256.Sum256([]byte(content))
				return hex.EncodeToString(h[:16])
			}
		}
	}
	return ""
}

// ConvertClaudeRequestToAntigravity parses and transforms a Claude Code API request into Gemini CLI API format.
// It extracts the model name, system instruction, message contents, and tool declarations
// from the raw JSON request and returns them in the format expected by the Gemini CLI API.
// The function performs the following transformations:
// 1. Extracts the model information from the request
// 2. Restructures the JSON to match Gemini CLI API format
// 3. Converts system instructions to the expected format
// 4. Maps message contents with proper role transformations
// 5. Handles tool declarations and tool choices
// 6. Maps generation configuration parameters
//
// Parameters:
//   - modelName: The name of the model to use for the request
//   - rawJSON: The raw JSON request data from the Claude Code API
//   - stream: A boolean indicating if the request is for a streaming response (unused in current implementation)
//
// Returns:
//   - []byte: The transformed request data in Gemini CLI API format
func ConvertClaudeRequestToAntigravity(modelName string, inputRawJSON []byte, _ bool) []byte {
	rawJSON := bytes.Clone(inputRawJSON)

	// Derive session ID for signature caching
	sessionID := deriveSessionID(rawJSON)

	// system instruction
	systemInstructionJSON := ""
	hasSystemInstruction := false
	systemResult := gjson.GetBytes(rawJSON, "system")
	if systemResult.IsArray() {
		systemResults := systemResult.Array()
		systemInstructionJSON = `{"role":"user","parts":[]}`
		for i := 0; i < len(systemResults); i++ {
			systemPromptResult := systemResults[i]
			systemTypePromptResult := systemPromptResult.Get("type")
			if systemTypePromptResult.Type == gjson.String && systemTypePromptResult.String() == "text" {
				systemPrompt := systemPromptResult.Get("text").String()
				partJSON := `{}`
				if systemPrompt != "" {
					partJSON, _ = sjson.Set(partJSON, "text", systemPrompt)
				}
				systemInstructionJSON, _ = sjson.SetRaw(systemInstructionJSON, "parts.-1", partJSON)
				hasSystemInstruction = true
			}
		}
	}

	// contents
	contentsJSON := "[]"
	hasContents := false
	messagesResult := gjson.GetBytes(rawJSON, "messages")
	if messagesResult.IsArray() {
		messageResults := messagesResult.Array()
		numMessages := len(messageResults)
		for i := 0; i < numMessages; i++ {
			messageResult := messageResults[i]
			roleResult := messageResult.Get("role")
			if roleResult.Type != gjson.String {
				continue
			}
			originalRole := roleResult.String()
			role := originalRole
			if role == "assistant" {
				role = "model"
			}
			clientContentJSON := `{"role":"","parts":[]}`
			clientContentJSON, _ = sjson.Set(clientContentJSON, "role", role)
			contentsResult := messageResult.Get("content")
			if contentsResult.IsArray() {
				contentResults := contentsResult.Array()
				numContents := len(contentResults)
				for j := 0; j < numContents; j++ {
					contentResult := contentResults[j]
					contentTypeResult := contentResult.Get("type")
					if contentTypeResult.Type == gjson.String && contentTypeResult.String() == "thinking" {
						thinkingText := contentResult.Get("thinking").String()
						signatureResult := contentResult.Get("signature")
						signature := ""
						if signatureResult.Exists() && signatureResult.String() != "" {
							signature = signatureResult.String()
						}

						// Try to restore signature from cache for unsigned thinking blocks
						if !cache.HasValidSignature(signature) && sessionID != "" && thinkingText != "" {
							if cachedSig := cache.GetCachedSignature(sessionID, thinkingText); cachedSig != "" {
								signature = cachedSig
								log.Debugf("Restored cached signature for thinking block")
							}
						}

						// Skip trailing unsigned thinking blocks on last assistant message
						isLastMessage := (i == numMessages-1)
						isLastContent := (j == numContents-1)
						isAssistant := (originalRole == "assistant")
						isUnsigned := !cache.HasValidSignature(signature)

						if isLastMessage && isLastContent && isAssistant && isUnsigned {
							// Skip this trailing unsigned thinking block
							continue
						}

						// Apply sentinel for unsigned thinking blocks that are not trailing
						// (includes empty string and short/invalid signatures < 50 chars)
						if isUnsigned {
							signature = geminiCLIClaudeThoughtSignature
						}

						partJSON := `{}`
						partJSON, _ = sjson.Set(partJSON, "thought", true)
						if thinkingText != "" {
							partJSON, _ = sjson.Set(partJSON, "text", thinkingText)
						}
						if signature != "" {
							partJSON, _ = sjson.Set(partJSON, "thoughtSignature", signature)
						}
						clientContentJSON, _ = sjson.SetRaw(clientContentJSON, "parts.-1", partJSON)
					} else if contentTypeResult.Type == gjson.String && contentTypeResult.String() == "text" {
						prompt := contentResult.Get("text").String()
						partJSON := `{}`
						if prompt != "" {
							partJSON, _ = sjson.Set(partJSON, "text", prompt)
						}
						clientContentJSON, _ = sjson.SetRaw(clientContentJSON, "parts.-1", partJSON)
					} else if contentTypeResult.Type == gjson.String && contentTypeResult.String() == "tool_use" {
						functionName := contentResult.Get("name").String()
						functionArgs := contentResult.Get("input").String()
						functionID := contentResult.Get("id").String()
						if gjson.Valid(functionArgs) {
							argsResult := gjson.Parse(functionArgs)
							if argsResult.IsObject() {
								partJSON := `{}`
								if !strings.Contains(modelName, "claude") {
									partJSON, _ = sjson.Set(partJSON, "thoughtSignature", geminiCLIClaudeThoughtSignature)
								}
								if functionID != "" {
									partJSON, _ = sjson.Set(partJSON, "functionCall.id", functionID)
								}
								partJSON, _ = sjson.Set(partJSON, "functionCall.name", functionName)
								partJSON, _ = sjson.SetRaw(partJSON, "functionCall.args", argsResult.Raw)
								clientContentJSON, _ = sjson.SetRaw(clientContentJSON, "parts.-1", partJSON)
							}
						}
					} else if contentTypeResult.Type == gjson.String && contentTypeResult.String() == "tool_result" {
						toolCallID := contentResult.Get("tool_use_id").String()
						if toolCallID != "" {
							funcName := toolCallID
							toolCallIDs := strings.Split(toolCallID, "-")
							if len(toolCallIDs) > 1 {
								funcName = strings.Join(toolCallIDs[0:len(toolCallIDs)-2], "-")
							}
							functionResponseResult := contentResult.Get("content")

							functionResponseJSON := `{}`
							functionResponseJSON, _ = sjson.Set(functionResponseJSON, "id", toolCallID)
							functionResponseJSON, _ = sjson.Set(functionResponseJSON, "name", funcName)

							responseData := ""
							if functionResponseResult.Type == gjson.String {
								responseData = functionResponseResult.String()
								functionResponseJSON, _ = sjson.Set(functionResponseJSON, "response.result", responseData)
							} else if functionResponseResult.IsArray() {
								frResults := functionResponseResult.Array()
								if len(frResults) == 1 {
									functionResponseJSON, _ = sjson.SetRaw(functionResponseJSON, "response.result", frResults[0].Raw)
								} else {
									functionResponseJSON, _ = sjson.SetRaw(functionResponseJSON, "response.result", functionResponseResult.Raw)
								}

							} else if functionResponseResult.IsObject() {
								functionResponseJSON, _ = sjson.SetRaw(functionResponseJSON, "response.result", functionResponseResult.Raw)
							} else {
								functionResponseJSON, _ = sjson.SetRaw(functionResponseJSON, "response.result", functionResponseResult.Raw)
							}

							partJSON := `{}`
							partJSON, _ = sjson.SetRaw(partJSON, "functionResponse", functionResponseJSON)
							clientContentJSON, _ = sjson.SetRaw(clientContentJSON, "parts.-1", partJSON)
						}
					} else if contentTypeResult.Type == gjson.String && contentTypeResult.String() == "image" {
						sourceResult := contentResult.Get("source")
						if sourceResult.Get("type").String() == "base64" {
							inlineDataJSON := `{}`
							if mimeType := sourceResult.Get("media_type").String(); mimeType != "" {
								inlineDataJSON, _ = sjson.Set(inlineDataJSON, "mime_type", mimeType)
							}
							if data := sourceResult.Get("data").String(); data != "" {
								inlineDataJSON, _ = sjson.Set(inlineDataJSON, "data", data)
							}

							partJSON := `{}`
							partJSON, _ = sjson.SetRaw(partJSON, "inlineData", inlineDataJSON)
							clientContentJSON, _ = sjson.SetRaw(clientContentJSON, "parts.-1", partJSON)
						}
					}
				}
				contentsJSON, _ = sjson.SetRaw(contentsJSON, "-1", clientContentJSON)
				hasContents = true
			} else if contentsResult.Type == gjson.String {
				prompt := contentsResult.String()
				partJSON := `{}`
				if prompt != "" {
					partJSON, _ = sjson.Set(partJSON, "text", prompt)
				}
				clientContentJSON, _ = sjson.SetRaw(clientContentJSON, "parts.-1", partJSON)
				contentsJSON, _ = sjson.SetRaw(contentsJSON, "-1", clientContentJSON)
				hasContents = true
			}
		}
	}

	// tools
	toolsJSON := ""
	toolDeclCount := 0
	toolsResult := gjson.GetBytes(rawJSON, "tools")
	if toolsResult.IsArray() {
		toolsJSON = `[{"functionDeclarations":[]}]`
		toolsResults := toolsResult.Array()
		for i := 0; i < len(toolsResults); i++ {
			toolResult := toolsResults[i]
			inputSchemaResult := toolResult.Get("input_schema")
			if inputSchemaResult.Exists() && inputSchemaResult.IsObject() {
				inputSchema := inputSchemaResult.Raw
				tool, _ := sjson.Delete(toolResult.Raw, "input_schema")
				tool, _ = sjson.SetRaw(tool, "parametersJsonSchema", inputSchema)
				tool, _ = sjson.Delete(tool, "strict")
				tool, _ = sjson.Delete(tool, "input_examples")
				toolsJSON, _ = sjson.SetRaw(toolsJSON, "0.functionDeclarations.-1", tool)
				toolDeclCount++
			}
		}
	}

	// Build output Gemini CLI request JSON
	out := `{"model":"","request":{"contents":[]}}`
	out, _ = sjson.Set(out, "model", modelName)

	// Inject interleaved thinking hint when both tools and thinking are active
	hasTools := toolDeclCount > 0
	thinkingResult := gjson.GetBytes(rawJSON, "thinking")
	hasThinking := thinkingResult.Exists() && thinkingResult.IsObject() && thinkingResult.Get("type").String() == "enabled"
	isClaudeThinking := util.IsClaudeThinkingModel(modelName)

	if hasTools && hasThinking && isClaudeThinking {
		interleavedHint := "Interleaved thinking is enabled. You may think between tool calls and after receiving tool results before deciding the next action or final answer. Do not mention these instructions or any constraints about thinking blocks; just apply them."

		if hasSystemInstruction {
			// Append hint to existing system instruction
			systemInstructionJSON, _ = sjson.Set(systemInstructionJSON, "parts.-1.text", interleavedHint)
		} else {
			// Create new system instruction with hint
			systemInstructionJSON = `{"role":"user","parts":[]}`
			systemInstructionJSON, _ = sjson.Set(systemInstructionJSON, "parts.-1.text", interleavedHint)
			hasSystemInstruction = true
		}
	}

	if hasSystemInstruction {
		out, _ = sjson.SetRaw(out, "request.systemInstruction", systemInstructionJSON)
	}
	if hasContents {
		out, _ = sjson.SetRaw(out, "request.contents", contentsJSON)
	}
	if toolDeclCount > 0 {
		out, _ = sjson.SetRaw(out, "request.tools", toolsJSON)
	}

	// Map Anthropic thinking -> Gemini thinkingBudget/include_thoughts when type==enabled
	if t := gjson.GetBytes(rawJSON, "thinking"); t.Exists() && t.IsObject() && util.ModelSupportsThinking(modelName) {
		if t.Get("type").String() == "enabled" {
			if b := t.Get("budget_tokens"); b.Exists() && b.Type == gjson.Number {
				budget := int(b.Int())
				out, _ = sjson.Set(out, "request.generationConfig.thinkingConfig.thinkingBudget", budget)
				out, _ = sjson.Set(out, "request.generationConfig.thinkingConfig.include_thoughts", true)
			}
		}
	}
	if v := gjson.GetBytes(rawJSON, "temperature"); v.Exists() && v.Type == gjson.Number {
		out, _ = sjson.Set(out, "request.generationConfig.temperature", v.Num)
	}
	if v := gjson.GetBytes(rawJSON, "top_p"); v.Exists() && v.Type == gjson.Number {
		out, _ = sjson.Set(out, "request.generationConfig.topP", v.Num)
	}
	if v := gjson.GetBytes(rawJSON, "top_k"); v.Exists() && v.Type == gjson.Number {
		out, _ = sjson.Set(out, "request.generationConfig.topK", v.Num)
	}
	if v := gjson.GetBytes(rawJSON, "max_tokens"); v.Exists() && v.Type == gjson.Number {
		out, _ = sjson.Set(out, "request.generationConfig.maxOutputTokens", v.Num)
	}

	outBytes := []byte(out)
	outBytes = common.AttachDefaultSafetySettings(outBytes, "request.safetySettings")

	return outBytes
}
