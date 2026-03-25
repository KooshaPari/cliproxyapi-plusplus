package executor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	kiroclaude "github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/translator/kiro/claude"
	kirocommon "github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/translator/kiro/common"
	clipproxyexecutor "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/cliproxy/executor"
	"github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/cliproxy/usage"
	sdktranslator "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/translator"
	log "github.com/sirupsen/logrus"
)

// streamToChannel converts AWS Event Stream to channel-based streaming.
// Supports tool calling - emits tool_use content blocks when tools are used.
// Includes embedded [Called ...] tool call parsing and input buffering for toolUseEvent.
// Implements duplicate content filtering using lastContentEvent detection (based on AIClient-2-API).
// Extracts stop_reason from upstream events when available.
// thinkingEnabled controls whether <thinking> tags are parsed - only parse when request enabled thinking.
func (e *KiroExecutor) streamToChannel(ctx context.Context, body io.Reader, out chan<- cliproxyexecutor.StreamChunk, targetFormat sdktranslator.Format, model string, originalReq, claudeBody []byte, reporter *usageReporter, thinkingEnabled bool) {
	reader := bufio.NewReaderSize(body, 20*1024*1024) // 20MB buffer to match other providers
	var totalUsage usage.Detail
	var hasToolUses bool          // Track if any tool uses were emitted
	var hasTruncatedTools bool    // Track if any tool uses were truncated
	var upstreamStopReason string // Track stop_reason from upstream events

	// Tool use state tracking for input buffering and deduplication
	processedIDs := make(map[string]bool)
	var currentToolUse *kiroclaude.ToolUseState

	// NOTE: Duplicate content filtering removed - it was causing legitimate repeated
	// content (like consecutive newlines) to be incorrectly filtered out.
	// The previous implementation compared lastContentEvent == contentDelta which
	// is too aggressive for streaming scenarios.

	// Streaming token calculation - accumulate content for real-time token counting
	// Based on AIClient-2-API implementation
	var accumulatedContent strings.Builder
	accumulatedContent.Grow(4096) // Pre-allocate 4KB capacity to reduce reallocations

	// Real-time usage estimation state
	// These track when to send periodic usage updates during streaming
	var lastUsageUpdateLen int           // Last accumulated content length when usage was sent
	var lastUsageUpdateTime = time.Now() // Last time usage update was sent
	var lastReportedOutputTokens int64   // Last reported output token count

	// Upstream usage tracking - Kiro API returns credit usage and context percentage
	var upstreamCreditUsage float64       // Credit usage from upstream (e.g., 1.458)
	var upstreamContextPercentage float64 // Context usage percentage from upstream (e.g., 78.56)
	var hasUpstreamUsage bool             // Whether we received usage from upstream

	// Translator param for maintaining tool call state across streaming events
	// IMPORTANT: This must persist across all TranslateStream calls
	var translatorParam any

	// Thinking mode state tracking - tag-based parsing for <thinking> tags in content
	inThinkBlock := false                          // Whether we're currently inside a <thinking> block
	isThinkingBlockOpen := false                   // Track if thinking content block SSE event is open
	thinkingBlockIndex := -1                       // Index of the thinking content block
	var accumulatedThinkingContent strings.Builder // Accumulate thinking content for token counting

	// Buffer for handling partial tag matches at chunk boundaries
	var pendingContent strings.Builder // Buffer content that might be part of a tag

	// Pre-calculate input tokens from request if possible
	// Kiro uses Claude format, so try Claude format first, then OpenAI format, then fallback
	if enc, err := getTokenizer(model); err == nil {
		var inputTokens int64
		var countMethod string

		// Try Claude format first (Kiro uses Claude API format)
		if inp, err := countClaudeChatTokens(enc, claudeBody); err == nil && inp > 0 {
			inputTokens = inp
			countMethod = "claude"
		} else if inp, err := countOpenAIChatTokens(enc, originalReq); err == nil && inp > 0 {
			// Fallback to OpenAI format (for OpenAI-compatible requests)
			inputTokens = inp
			countMethod = "openai"
		} else {
			// Final fallback: estimate from raw request size (roughly 4 chars per token)
			inputTokens = int64(len(claudeBody) / 4)
			if inputTokens == 0 && len(claudeBody) > 0 {
				inputTokens = 1
			}
			countMethod = "estimate"
		}

		totalUsage.InputTokens = inputTokens
		log.Debugf("kiro: streamToChannel pre-calculated input tokens: %d (method: %s, claude body: %d bytes, original req: %d bytes)",
			totalUsage.InputTokens, countMethod, len(claudeBody), len(originalReq))
	}

	contentBlockIndex := -1
	messageStartSent := false
	isTextBlockOpen := false
	var outputLen int

	// Ensure usage is published even on early return
	defer func() {
		reporter.publish(ctx, totalUsage)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msg, eventErr := e.readEventStreamMessage(reader)
		if eventErr != nil {
			// Log the error
			log.Errorf("kiro: streamToChannel error: %v", eventErr)

			// Send error to channel for client notification
			out <- cliproxyexecutor.StreamChunk{Err: eventErr}
			return
		}
		if msg == nil {
			// Normal end of stream (EOF)
			// Flush any incomplete tool use before ending stream
			if currentToolUse != nil && !processedIDs[currentToolUse.ToolUseID] {
				log.Warnf("kiro: flushing incomplete tool use at EOF: %s (ID: %s)", currentToolUse.Name, currentToolUse.ToolUseID)
				fullInput := currentToolUse.InputBuffer.String()
				repairedJSON := kiroclaude.RepairJSON(fullInput)
				var finalInput map[string]interface{}
				if err := json.Unmarshal([]byte(repairedJSON), &finalInput); err != nil {
					log.Warnf("kiro: failed to parse incomplete tool input at EOF: %v", err)
					finalInput = make(map[string]interface{})
				}

				processedIDs[currentToolUse.ToolUseID] = true
				contentBlockIndex++

				// Send tool_use content block
				blockStart := kiroclaude.BuildClaudeContentBlockStartEvent(contentBlockIndex, "tool_use", currentToolUse.ToolUseID, currentToolUse.Name)
				sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStart, &translatorParam)
				for _, chunk := range sseData {
					if chunk != "" {
						out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
					}
				}

				// Send tool input as delta
				inputBytes, _ := json.Marshal(finalInput)
				inputDelta := kiroclaude.BuildClaudeInputJsonDeltaEvent(string(inputBytes), contentBlockIndex)
				sseData = sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, inputDelta, &translatorParam)
				for _, chunk := range sseData {
					if chunk != "" {
						out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
					}
				}

				// Close block
				blockStop := kiroclaude.BuildClaudeContentBlockStopEvent(contentBlockIndex)
				sseData = sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStop, &translatorParam)
				for _, chunk := range sseData {
					if chunk != "" {
						out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
					}
				}

				hasToolUses = true
				currentToolUse = nil
			}

			// DISABLED: Tag-based pending character flushing
			// This code block was used for tag-based thinking detection which has been
			// replaced by reasoningContentEvent handling. No pending tag chars to flush.
			// Original code preserved in git history.
			break
		}

		eventType := msg.EventType
		payload := msg.Payload
		if len(payload) == 0 {
			continue
		}
		appendAPIResponseChunk(ctx, e.cfg, payload)

		var event map[string]interface{}
		if err := json.Unmarshal(payload, &event); err != nil {
			log.Warnf("kiro: failed to unmarshal event payload: %v, raw: %s", err, string(payload))
			continue
		}

		// Check for error/exception events in the payload (Kiro API may return errors with HTTP 200)
		// These can appear as top-level fields or nested within the event
		if errType, hasErrType := event["_type"].(string); hasErrType {
			// AWS-style error: {"_type": "com.amazon.aws.codewhisperer#ValidationException", "message": "..."}
			errMsg := ""
			if msg, ok := event["message"].(string); ok {
				errMsg = msg
			}
			log.Errorf("kiro: received AWS error in stream: type=%s, message=%s", errType, errMsg)
			out <- cliproxyexecutor.StreamChunk{Err: fmt.Errorf("kiro API error: %s - %s", errType, errMsg)}
			return
		}
		if errType, hasErrType := event["type"].(string); hasErrType && (errType == "error" || errType == "exception") {
			// Generic error event
			errMsg := ""
			if msg, ok := event["message"].(string); ok {
				errMsg = msg
			} else if errObj, ok := event["error"].(map[string]interface{}); ok {
				if msg, ok := errObj["message"].(string); ok {
					errMsg = msg
				}
			}
			log.Errorf("kiro: received error event in stream: type=%s, message=%s", errType, errMsg)
			out <- cliproxyexecutor.StreamChunk{Err: fmt.Errorf("kiro API error: %s", errMsg)}
			return
		}

		// Extract stop_reason from various event formats (streaming)
		// Kiro/Amazon Q API may include stop_reason in different locations
		if sr := kirocommon.GetString(event, "stop_reason"); sr != "" {
			upstreamStopReason = sr
			log.Debugf("kiro: streamToChannel found stop_reason (top-level): %s", upstreamStopReason)
		}
		if sr := kirocommon.GetString(event, "stopReason"); sr != "" {
			upstreamStopReason = sr
			log.Debugf("kiro: streamToChannel found stopReason (top-level): %s", upstreamStopReason)
		}

		// Send message_start on first event
		if !messageStartSent {
			msgStart := kiroclaude.BuildClaudeMessageStartEvent(model, totalUsage.InputTokens)
			sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, msgStart, &translatorParam)
			for _, chunk := range sseData {
				if chunk != "" {
					out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
				}
			}
			messageStartSent = true
		}

		switch eventType {
		case "followupPromptEvent":
			// Filter out followupPrompt events - these are UI suggestions, not content
			log.Debugf("kiro: streamToChannel ignoring followupPrompt event")
			continue

		case "messageStopEvent", "message_stop":
			// Handle message stop events which may contain stop_reason
			if sr := kirocommon.GetString(event, "stop_reason"); sr != "" {
				upstreamStopReason = sr
				log.Debugf("kiro: streamToChannel found stop_reason in messageStopEvent: %s", upstreamStopReason)
			}
			if sr := kirocommon.GetString(event, "stopReason"); sr != "" {
				upstreamStopReason = sr
				log.Debugf("kiro: streamToChannel found stopReason in messageStopEvent: %s", upstreamStopReason)
			}

		case "meteringEvent":
			// Handle metering events from Kiro API (usage billing information)
			// Official format: { unit: string, unitPlural: string, usage: number }
			if metering, ok := event["meteringEvent"].(map[string]interface{}); ok {
				unit := ""
				if u, ok := metering["unit"].(string); ok {
					unit = u
				}
				usageVal := 0.0
				if u, ok := metering["usage"].(float64); ok {
					usageVal = u
				}
				upstreamCreditUsage = usageVal
				hasUpstreamUsage = true
				log.Infof("kiro: streamToChannel received meteringEvent: usage=%.4f %s", usageVal, unit)
			} else {
				// Try direct fields (event is meteringEvent itself)
				if unit, ok := event["unit"].(string); ok {
					if usage, ok := event["usage"].(float64); ok {
						upstreamCreditUsage = usage
						hasUpstreamUsage = true
						log.Infof("kiro: streamToChannel received meteringEvent (direct): usage=%.4f %s", usage, unit)
					}
				}
			}

		case "contextUsageEvent":
			// Handle context usage events from Kiro API
			// Format: {"contextUsageEvent": {"contextUsagePercentage": 0.53}}
			if ctxUsage, ok := event["contextUsageEvent"].(map[string]interface{}); ok {
				if ctxPct, ok := ctxUsage["contextUsagePercentage"].(float64); ok {
					upstreamContextPercentage = ctxPct
					log.Debugf("kiro: streamToChannel received contextUsageEvent: %.2f%%", ctxPct*100)
				}
			} else {
				// Try direct field (fallback)
				if ctxPct, ok := event["contextUsagePercentage"].(float64); ok {
					upstreamContextPercentage = ctxPct
					log.Debugf("kiro: streamToChannel received contextUsagePercentage (direct): %.2f%%", ctxPct*100)
				}
			}

		case "error", "exception", "internalServerException":
			// Handle error events from Kiro API stream
			errMsg := ""
			errType := eventType

			// Try to extract error message from various formats
			if msg, ok := event["message"].(string); ok {
				errMsg = msg
			} else if errObj, ok := event[eventType].(map[string]interface{}); ok {
				if msg, ok := errObj["message"].(string); ok {
					errMsg = msg
				}
				if t, ok := errObj["type"].(string); ok {
					errType = t
				}
			} else if errObj, ok := event["error"].(map[string]interface{}); ok {
				if msg, ok := errObj["message"].(string); ok {
					errMsg = msg
				}
			}

			log.Errorf("kiro: streamToChannel received error event: type=%s, message=%s", errType, errMsg)

			// Send error to the stream and exit
			if errMsg != "" {
				out <- cliproxyexecutor.StreamChunk{
					Err: fmt.Errorf("kiro API error (%s): %s", errType, errMsg),
				}
				return
			}

		case "invalidStateEvent":
			// Handle invalid state events - log and continue (non-fatal)
			errMsg := ""
			if msg, ok := event["message"].(string); ok {
				errMsg = msg
			} else if stateEvent, ok := event["invalidStateEvent"].(map[string]interface{}); ok {
				if msg, ok := stateEvent["message"].(string); ok {
					errMsg = msg
				}
			}
			log.Warnf("kiro: streamToChannel received invalidStateEvent: %s, continuing", errMsg)
			continue

		case "assistantResponseEvent":
			var contentDelta string
			var toolUses []map[string]interface{}

			if assistantResp, ok := event["assistantResponseEvent"].(map[string]interface{}); ok {
				if c, ok := assistantResp["content"].(string); ok {
					contentDelta = c
				}
				// Extract stop_reason from assistantResponseEvent
				if sr := kirocommon.GetString(assistantResp, "stop_reason"); sr != "" {
					upstreamStopReason = sr
					log.Debugf("kiro: streamToChannel found stop_reason in assistantResponseEvent: %s", upstreamStopReason)
				}
				if sr := kirocommon.GetString(assistantResp, "stopReason"); sr != "" {
					upstreamStopReason = sr
					log.Debugf("kiro: streamToChannel found stopReason in assistantResponseEvent: %s", upstreamStopReason)
				}
				// Extract tool uses from response
				if tus, ok := assistantResp["toolUses"].([]interface{}); ok {
					for _, tuRaw := range tus {
						if tu, ok := tuRaw.(map[string]interface{}); ok {
							toolUses = append(toolUses, tu)
						}
					}
				}
			}
			if contentDelta == "" {
				if c, ok := event["content"].(string); ok {
					contentDelta = c
				}
			}
			// Direct tool uses
			if tus, ok := event["toolUses"].([]interface{}); ok {
				for _, tuRaw := range tus {
					if tu, ok := tuRaw.(map[string]interface{}); ok {
						toolUses = append(toolUses, tu)
					}
				}
			}

			// Handle text content with thinking mode support
			if contentDelta != "" {
				// NOTE: Duplicate content filtering was removed because it incorrectly
				// filtered out legitimate repeated content (like consecutive newlines "\n\n").
				// Streaming naturally can have identical chunks that are valid content.

				outputLen += len(contentDelta)
				// Accumulate content for streaming token calculation
				accumulatedContent.WriteString(contentDelta)

				// Real-time usage estimation: Check if we should send a usage update
				// This helps clients track context usage during long thinking sessions
				shouldSendUsageUpdate := false
				if accumulatedContent.Len()-lastUsageUpdateLen >= usageUpdateCharThreshold {
					shouldSendUsageUpdate = true
				} else if time.Since(lastUsageUpdateTime) >= usageUpdateTimeInterval && accumulatedContent.Len() > lastUsageUpdateLen {
					shouldSendUsageUpdate = true
				}

				if shouldSendUsageUpdate {
					// Calculate current output tokens using tiktoken
					var currentOutputTokens int64
					if enc, encErr := getTokenizer(model); encErr == nil {
						if tokenCount, countErr := enc.Count(accumulatedContent.String()); countErr == nil {
							currentOutputTokens = int64(tokenCount)
						}
					}
					// Fallback to character estimation if tiktoken fails
					if currentOutputTokens == 0 {
						currentOutputTokens = int64(accumulatedContent.Len() / 4)
						if currentOutputTokens == 0 {
							currentOutputTokens = 1
						}
					}

					// Only send update if token count has changed significantly (at least 10 tokens)
					if currentOutputTokens > lastReportedOutputTokens+10 {
						// Send ping event with usage information
						// This is a non-blocking update that clients can optionally process
						pingEvent := kiroclaude.BuildClaudePingEventWithUsage(totalUsage.InputTokens, currentOutputTokens)
						sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, pingEvent, &translatorParam)
						for _, chunk := range sseData {
							if chunk != "" {
								out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
							}
						}

						lastReportedOutputTokens = currentOutputTokens
						log.Debugf("kiro: sent real-time usage update - input: %d, output: %d (accumulated: %d chars)",
							totalUsage.InputTokens, currentOutputTokens, accumulatedContent.Len())
					}

					lastUsageUpdateLen = accumulatedContent.Len()
					lastUsageUpdateTime = time.Now()
				}

				// TAG-BASED THINKING PARSING: Parse <thinking> tags from content
				// Combine pending content with new content for processing
				pendingContent.WriteString(contentDelta)
				processContent := pendingContent.String()
				pendingContent.Reset()

				// Process content looking for thinking tags
				for len(processContent) > 0 {
					if inThinkBlock {
						// We're inside a thinking block, look for </thinking>
						endIdx := strings.Index(processContent, kirocommon.ThinkingEndTag)
						if endIdx >= 0 {
							// Found end tag - emit thinking content before the tag
							thinkingText := processContent[:endIdx]
							if thinkingText != "" {
								// Ensure thinking block is open
								if !isThinkingBlockOpen {
									contentBlockIndex++
									thinkingBlockIndex = contentBlockIndex
									isThinkingBlockOpen = true
									blockStart := kiroclaude.BuildClaudeContentBlockStartEvent(thinkingBlockIndex, "thinking", "", "")
									sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStart, &translatorParam)
									for _, chunk := range sseData {
										if chunk != "" {
											out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
										}
									}
								}
								// Send thinking delta
								thinkingEvent := kiroclaude.BuildClaudeThinkingDeltaEvent(thinkingText, thinkingBlockIndex)
								sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, thinkingEvent, &translatorParam)
								for _, chunk := range sseData {
									if chunk != "" {
										out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
									}
								}
								accumulatedThinkingContent.WriteString(thinkingText)
							}
							// Close thinking block
							if isThinkingBlockOpen {
								blockStop := kiroclaude.BuildClaudeThinkingBlockStopEvent(thinkingBlockIndex)
								sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStop, &translatorParam)
								for _, chunk := range sseData {
									if chunk != "" {
										out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
									}
								}
								isThinkingBlockOpen = false
							}
							inThinkBlock = false
							processContent = processContent[endIdx+len(kirocommon.ThinkingEndTag):]
							log.Debugf("kiro: closed thinking block, remaining content: %d chars", len(processContent))
						} else {
							// No end tag found - check for partial match at end
							partialMatch := false
							for i := 1; i < len(kirocommon.ThinkingEndTag) && i <= len(processContent); i++ {
								if strings.HasSuffix(processContent, kirocommon.ThinkingEndTag[:i]) {
									// Possible partial tag at end, buffer it
									pendingContent.WriteString(processContent[len(processContent)-i:])
									processContent = processContent[:len(processContent)-i]
									partialMatch = true
									break
								}
							}
							if !partialMatch || len(processContent) > 0 {
								// Emit all as thinking content
								if processContent != "" {
									if !isThinkingBlockOpen {
										contentBlockIndex++
										thinkingBlockIndex = contentBlockIndex
										isThinkingBlockOpen = true
										blockStart := kiroclaude.BuildClaudeContentBlockStartEvent(thinkingBlockIndex, "thinking", "", "")
										sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStart, &translatorParam)
										for _, chunk := range sseData {
											if chunk != "" {
												out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
											}
										}
									}
									thinkingEvent := kiroclaude.BuildClaudeThinkingDeltaEvent(processContent, thinkingBlockIndex)
									sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, thinkingEvent, &translatorParam)
									for _, chunk := range sseData {
										if chunk != "" {
											out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
										}
									}
									accumulatedThinkingContent.WriteString(processContent)
								}
							}
							processContent = ""
						}
					} else {
						// Not in thinking block, look for <thinking>
						startIdx := strings.Index(processContent, kirocommon.ThinkingStartTag)
						if startIdx >= 0 {
							// Found start tag - emit text content before the tag
							textBefore := processContent[:startIdx]
							if textBefore != "" {
								// Close thinking block if open
								if isThinkingBlockOpen {
									blockStop := kiroclaude.BuildClaudeThinkingBlockStopEvent(thinkingBlockIndex)
									sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStop, &translatorParam)
									for _, chunk := range sseData {
										if chunk != "" {
											out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
										}
									}
									isThinkingBlockOpen = false
								}
								// Ensure text block is open
								if !isTextBlockOpen {
									contentBlockIndex++
									isTextBlockOpen = true
									blockStart := kiroclaude.BuildClaudeContentBlockStartEvent(contentBlockIndex, "text", "", "")
									sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStart, &translatorParam)
									for _, chunk := range sseData {
										if chunk != "" {
											out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
										}
									}
								}
								// Send text delta
								claudeEvent := kiroclaude.BuildClaudeStreamEvent(textBefore, contentBlockIndex)
								sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, claudeEvent, &translatorParam)
								for _, chunk := range sseData {
									if chunk != "" {
										out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
									}
								}
							}
							// Close text block before entering thinking
							if isTextBlockOpen {
								blockStop := kiroclaude.BuildClaudeContentBlockStopEvent(contentBlockIndex)
								sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStop, &translatorParam)
								for _, chunk := range sseData {
									if chunk != "" {
										out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
									}
								}
								isTextBlockOpen = false
							}
							inThinkBlock = true
							processContent = processContent[startIdx+len(kirocommon.ThinkingStartTag):]
							log.Debugf("kiro: entered thinking block")
						} else {
							// No start tag found - check for partial match at end
							partialMatch := false
							for i := 1; i < len(kirocommon.ThinkingStartTag) && i <= len(processContent); i++ {
								if strings.HasSuffix(processContent, kirocommon.ThinkingStartTag[:i]) {
									// Possible partial tag at end, buffer it
									pendingContent.WriteString(processContent[len(processContent)-i:])
									processContent = processContent[:len(processContent)-i]
									partialMatch = true
									break
								}
							}
							if !partialMatch || len(processContent) > 0 {
								// Emit all as text content
								if processContent != "" {
									if !isTextBlockOpen {
										contentBlockIndex++
										isTextBlockOpen = true
										blockStart := kiroclaude.BuildClaudeContentBlockStartEvent(contentBlockIndex, "text", "", "")
										sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStart, &translatorParam)
										for _, chunk := range sseData {
											if chunk != "" {
												out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
											}
										}
									}
									claudeEvent := kiroclaude.BuildClaudeStreamEvent(processContent, contentBlockIndex)
									sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, claudeEvent, &translatorParam)
									for _, chunk := range sseData {
										if chunk != "" {
											out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
										}
									}
								}
							}
							processContent = ""
						}
					}
				}
			}

			// Handle tool uses in response (with deduplication)
			for _, tu := range toolUses {
				toolUseID := kirocommon.GetString(tu, "toolUseId")
				toolName := kirocommon.GetString(tu, "name")

				// Check for duplicate
				if processedIDs[toolUseID] {
					log.Debugf("kiro: skipping duplicate tool use in stream: %s", toolUseID)
					continue
				}
				processedIDs[toolUseID] = true

				hasToolUses = true
				// Close text block if open before starting tool_use block
				if isTextBlockOpen && contentBlockIndex >= 0 {
					blockStop := kiroclaude.BuildClaudeContentBlockStopEvent(contentBlockIndex)
					sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStop, &translatorParam)
					for _, chunk := range sseData {
						if chunk != "" {
							out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
						}
					}
					isTextBlockOpen = false
				}

				// Emit tool_use content block
				contentBlockIndex++

				blockStart := kiroclaude.BuildClaudeContentBlockStartEvent(contentBlockIndex, "tool_use", toolUseID, toolName)
				sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStart, &translatorParam)
				for _, chunk := range sseData {
					if chunk != "" {
						out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
					}
				}

				// Send input_json_delta with the tool input
				if input, ok := tu["input"].(map[string]interface{}); ok {
					inputJSON, err := json.Marshal(input)
					if err != nil {
						log.Debugf("kiro: failed to marshal tool input: %v", err)
						// Don't continue - still need to close the block
					} else {
						inputDelta := kiroclaude.BuildClaudeInputJsonDeltaEvent(string(inputJSON), contentBlockIndex)
						sseData = sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, inputDelta, &translatorParam)
						for _, chunk := range sseData {
							if chunk != "" {
								out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
							}
						}
					}
				}

				// Close tool_use block (always close even if input marshal failed)
				blockStop := kiroclaude.BuildClaudeContentBlockStopEvent(contentBlockIndex)
				sseData = sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStop, &translatorParam)
				for _, chunk := range sseData {
					if chunk != "" {
						out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
					}
				}
			}

		case "reasoningContentEvent":
			// Handle official reasoningContentEvent from Kiro API
			// This replaces tag-based thinking detection with the proper event type
			// Official format: { text: string, signature?: string, redactedContent?: base64 }
			var thinkingText string
			var signature string

			if re, ok := event["reasoningContentEvent"].(map[string]interface{}); ok {
				if text, ok := re["text"].(string); ok {
					thinkingText = text
				}
				if sig, ok := re["signature"].(string); ok {
					signature = sig
					if len(sig) > 20 {
						log.Debugf("kiro: reasoningContentEvent has signature: %s...", sig[:20])
					} else {
						log.Debugf("kiro: reasoningContentEvent has signature: %s", sig)
					}
				}
			} else {
				// Try direct fields
				if text, ok := event["text"].(string); ok {
					thinkingText = text
				}
				if sig, ok := event["signature"].(string); ok {
					signature = sig
				}
			}

			if thinkingText != "" {
				// Close text block if open before starting thinking block
				if isTextBlockOpen && contentBlockIndex >= 0 {
					blockStop := kiroclaude.BuildClaudeContentBlockStopEvent(contentBlockIndex)
					sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStop, &translatorParam)
					for _, chunk := range sseData {
						if chunk != "" {
							out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
						}
					}
					isTextBlockOpen = false
				}

				// Start thinking block if not already open
				if !isThinkingBlockOpen {
					contentBlockIndex++
					thinkingBlockIndex = contentBlockIndex
					isThinkingBlockOpen = true
					blockStart := kiroclaude.BuildClaudeContentBlockStartEvent(thinkingBlockIndex, "thinking", "", "")
					sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStart, &translatorParam)
					for _, chunk := range sseData {
						if chunk != "" {
							out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
						}
					}
				}

				// Send thinking content
				thinkingEvent := kiroclaude.BuildClaudeThinkingDeltaEvent(thinkingText, thinkingBlockIndex)
				sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, thinkingEvent, &translatorParam)
				for _, chunk := range sseData {
					if chunk != "" {
						out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
					}
				}

				// Accumulate for token counting
				accumulatedThinkingContent.WriteString(thinkingText)
				log.Debugf("kiro: received reasoningContentEvent, text length: %d, has signature: %v", len(thinkingText), signature != "")
			}

			// Note: We don't close the thinking block here - it will be closed when we see
			// the next assistantResponseEvent or at the end of the stream
			_ = signature // Signature can be used for verification if needed

		case "toolUseEvent":
			// Handle dedicated tool use events with input buffering
			completedToolUses, newState := kiroclaude.ProcessToolUseEvent(event, currentToolUse, processedIDs)
			currentToolUse = newState

			// Emit completed tool uses
			for _, tu := range completedToolUses {
				// Check if this tool was truncated - emit with SOFT_LIMIT_REACHED marker
				if tu.IsTruncated {
					hasTruncatedTools = true
					log.Infof("kiro: streamToChannel emitting truncated tool with SOFT_LIMIT_REACHED: %s (ID: %s)", tu.Name, tu.ToolUseID)

					// Close text block if open
					if isTextBlockOpen && contentBlockIndex >= 0 {
						blockStop := kiroclaude.BuildClaudeContentBlockStopEvent(contentBlockIndex)
						sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStop, &translatorParam)
						for _, chunk := range sseData {
							if chunk != "" {
								out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
							}
						}
						isTextBlockOpen = false
					}

					contentBlockIndex++

					// Emit tool_use with SOFT_LIMIT_REACHED marker input
					blockStart := kiroclaude.BuildClaudeContentBlockStartEvent(contentBlockIndex, "tool_use", tu.ToolUseID, tu.Name)
					sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStart, &translatorParam)
					for _, chunk := range sseData {
						if chunk != "" {
							out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
						}
					}

					// Build SOFT_LIMIT_REACHED marker input
					markerInput := map[string]interface{}{
						"_status":  "SOFT_LIMIT_REACHED",
						"_message": "Tool output was truncated. Split content into smaller chunks (max 300 lines). Due to potential model hallucination, you MUST re-fetch the current working directory and generate the correct file_path.",
					}

					markerJSON, _ := json.Marshal(markerInput)
					inputDelta := kiroclaude.BuildClaudeInputJsonDeltaEvent(string(markerJSON), contentBlockIndex)
					sseData = sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, inputDelta, &translatorParam)
					for _, chunk := range sseData {
						if chunk != "" {
							out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
						}
					}

					// Close tool_use block
					blockStop := kiroclaude.BuildClaudeContentBlockStopEvent(contentBlockIndex)
					sseData = sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStop, &translatorParam)
					for _, chunk := range sseData {
						if chunk != "" {
							out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
						}
					}

					hasToolUses = true // Keep this so stop_reason = tool_use
					continue
				}

				hasToolUses = true

				// Close text block if open
				if isTextBlockOpen && contentBlockIndex >= 0 {
					blockStop := kiroclaude.BuildClaudeContentBlockStopEvent(contentBlockIndex)
					sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStop, &translatorParam)
					for _, chunk := range sseData {
						if chunk != "" {
							out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
						}
					}
					isTextBlockOpen = false
				}

				contentBlockIndex++

				blockStart := kiroclaude.BuildClaudeContentBlockStartEvent(contentBlockIndex, "tool_use", tu.ToolUseID, tu.Name)
				sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStart, &translatorParam)
				for _, chunk := range sseData {
					if chunk != "" {
						out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
					}
				}

				if tu.Input != nil {
					inputJSON, err := json.Marshal(tu.Input)
					if err != nil {
						log.Debugf("kiro: failed to marshal tool input in toolUseEvent: %v", err)
					} else {
						inputDelta := kiroclaude.BuildClaudeInputJsonDeltaEvent(string(inputJSON), contentBlockIndex)
						sseData = sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, inputDelta, &translatorParam)
						for _, chunk := range sseData {
							if chunk != "" {
								out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
							}
						}
					}
				}

				blockStop := kiroclaude.BuildClaudeContentBlockStopEvent(contentBlockIndex)
				sseData = sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStop, &translatorParam)
				for _, chunk := range sseData {
					if chunk != "" {
						out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
					}
				}
			}

		case "supplementaryWebLinksEvent":
			if inputTokens, ok := event["inputTokens"].(float64); ok {
				totalUsage.InputTokens = int64(inputTokens)
			}
			if outputTokens, ok := event["outputTokens"].(float64); ok {
				totalUsage.OutputTokens = int64(outputTokens)
			}

		case "messageMetadataEvent", "metadataEvent":
			// Handle message metadata events which contain token counts
			// Official format: { tokenUsage: { outputTokens, totalTokens, uncachedInputTokens, cacheReadInputTokens, cacheWriteInputTokens, contextUsagePercentage } }
			var metadata map[string]interface{}
			if m, ok := event["messageMetadataEvent"].(map[string]interface{}); ok {
				metadata = m
			} else if m, ok := event["metadataEvent"].(map[string]interface{}); ok {
				metadata = m
			} else {
				metadata = event // event itself might be the metadata
			}

			// Check for nested tokenUsage object (official format)
			if tokenUsage, ok := metadata["tokenUsage"].(map[string]interface{}); ok {
				// outputTokens - precise output token count
				if outputTokens, ok := tokenUsage["outputTokens"].(float64); ok {
					totalUsage.OutputTokens = int64(outputTokens)
					hasUpstreamUsage = true
					log.Infof("kiro: streamToChannel found precise outputTokens in tokenUsage: %d", totalUsage.OutputTokens)
				}
				// totalTokens - precise total token count
				if totalTokens, ok := tokenUsage["totalTokens"].(float64); ok {
					totalUsage.TotalTokens = int64(totalTokens)
					log.Infof("kiro: streamToChannel found precise totalTokens in tokenUsage: %d", totalUsage.TotalTokens)
				}
				// uncachedInputTokens - input tokens not from cache
				if uncachedInputTokens, ok := tokenUsage["uncachedInputTokens"].(float64); ok {
					totalUsage.InputTokens = int64(uncachedInputTokens)
					hasUpstreamUsage = true
					log.Infof("kiro: streamToChannel found uncachedInputTokens in tokenUsage: %d", totalUsage.InputTokens)
				}
				// cacheReadInputTokens - tokens read from cache
				if cacheReadTokens, ok := tokenUsage["cacheReadInputTokens"].(float64); ok {
					// Add to input tokens if we have uncached tokens, otherwise use as input
					if totalUsage.InputTokens > 0 {
						totalUsage.InputTokens += int64(cacheReadTokens)
					} else {
						totalUsage.InputTokens = int64(cacheReadTokens)
					}
					hasUpstreamUsage = true
					log.Debugf("kiro: streamToChannel found cacheReadInputTokens in tokenUsage: %d", int64(cacheReadTokens))
				}
				// contextUsagePercentage - can be used as fallback for input token estimation
				if ctxPct, ok := tokenUsage["contextUsagePercentage"].(float64); ok {
					upstreamContextPercentage = ctxPct
					log.Debugf("kiro: streamToChannel found contextUsagePercentage in tokenUsage: %.2f%%", ctxPct)
				}
			}

			// Fallback: check for direct fields in metadata (legacy format)
			if totalUsage.InputTokens == 0 {
				if inputTokens, ok := metadata["inputTokens"].(float64); ok {
					totalUsage.InputTokens = int64(inputTokens)
					hasUpstreamUsage = true
					log.Debugf("kiro: streamToChannel found inputTokens in messageMetadataEvent: %d", totalUsage.InputTokens)
				}
			}
			if totalUsage.OutputTokens == 0 {
				if outputTokens, ok := metadata["outputTokens"].(float64); ok {
					totalUsage.OutputTokens = int64(outputTokens)
					hasUpstreamUsage = true
					log.Debugf("kiro: streamToChannel found outputTokens in messageMetadataEvent: %d", totalUsage.OutputTokens)
				}
			}
			if totalUsage.TotalTokens == 0 {
				if totalTokens, ok := metadata["totalTokens"].(float64); ok {
					totalUsage.TotalTokens = int64(totalTokens)
					log.Debugf("kiro: streamToChannel found totalTokens in messageMetadataEvent: %d", totalUsage.TotalTokens)
				}
			}

		case "usageEvent", "usage":
			// Handle dedicated usage events
			if inputTokens, ok := event["inputTokens"].(float64); ok {
				totalUsage.InputTokens = int64(inputTokens)
				log.Debugf("kiro: streamToChannel found inputTokens in usageEvent: %d", totalUsage.InputTokens)
			}
			if outputTokens, ok := event["outputTokens"].(float64); ok {
				totalUsage.OutputTokens = int64(outputTokens)
				log.Debugf("kiro: streamToChannel found outputTokens in usageEvent: %d", totalUsage.OutputTokens)
			}
			if totalTokens, ok := event["totalTokens"].(float64); ok {
				totalUsage.TotalTokens = int64(totalTokens)
				log.Debugf("kiro: streamToChannel found totalTokens in usageEvent: %d", totalUsage.TotalTokens)
			}
			// Also check nested usage object
			if usageObj, ok := event["usage"].(map[string]interface{}); ok {
				if inputTokens, ok := usageObj["input_tokens"].(float64); ok {
					totalUsage.InputTokens = int64(inputTokens)
				} else if inputTokens, ok := usageObj["prompt_tokens"].(float64); ok {
					totalUsage.InputTokens = int64(inputTokens)
				}
				if outputTokens, ok := usageObj["output_tokens"].(float64); ok {
					totalUsage.OutputTokens = int64(outputTokens)
				} else if outputTokens, ok := usageObj["completion_tokens"].(float64); ok {
					totalUsage.OutputTokens = int64(outputTokens)
				}
				if totalTokens, ok := usageObj["total_tokens"].(float64); ok {
					totalUsage.TotalTokens = int64(totalTokens)
				}
				log.Debugf("kiro: streamToChannel found usage object: input=%d, output=%d, total=%d",
					totalUsage.InputTokens, totalUsage.OutputTokens, totalUsage.TotalTokens)
			}

		case "metricsEvent":
			// Handle metrics events which may contain usage data
			if metrics, ok := event["metricsEvent"].(map[string]interface{}); ok {
				if inputTokens, ok := metrics["inputTokens"].(float64); ok {
					totalUsage.InputTokens = int64(inputTokens)
				}
				if outputTokens, ok := metrics["outputTokens"].(float64); ok {
					totalUsage.OutputTokens = int64(outputTokens)
				}
				log.Debugf("kiro: streamToChannel found metricsEvent: input=%d, output=%d",
					totalUsage.InputTokens, totalUsage.OutputTokens)

			}
		default:
			// Check for upstream usage events from Kiro API
			// Format: {"unit":"credit","unitPlural":"credits","usage":1.458}
			if unit, ok := event["unit"].(string); ok && unit == "credit" {
				if usage, ok := event["usage"].(float64); ok {
					upstreamCreditUsage = usage
					hasUpstreamUsage = true
					log.Debugf("kiro: received upstream credit usage: %.4f", upstreamCreditUsage)
				}
			}
			// Format: {"contextUsagePercentage":78.56}
			if ctxPct, ok := event["contextUsagePercentage"].(float64); ok {
				upstreamContextPercentage = ctxPct
				log.Debugf("kiro: received upstream context usage: %.2f%%", upstreamContextPercentage)
			}

			// Check for token counts in unknown events
			if inputTokens, ok := event["inputTokens"].(float64); ok {
				totalUsage.InputTokens = int64(inputTokens)
				hasUpstreamUsage = true
				log.Debugf("kiro: streamToChannel found inputTokens in event %s: %d", eventType, totalUsage.InputTokens)
			}
			if outputTokens, ok := event["outputTokens"].(float64); ok {
				totalUsage.OutputTokens = int64(outputTokens)
				hasUpstreamUsage = true
				log.Debugf("kiro: streamToChannel found outputTokens in event %s: %d", eventType, totalUsage.OutputTokens)
			}
			if totalTokens, ok := event["totalTokens"].(float64); ok {
				totalUsage.TotalTokens = int64(totalTokens)
				log.Debugf("kiro: streamToChannel found totalTokens in event %s: %d", eventType, totalUsage.TotalTokens)
			}

			// Check for usage object in unknown events (OpenAI/Claude format)
			if usageObj, ok := event["usage"].(map[string]interface{}); ok {
				if inputTokens, ok := usageObj["input_tokens"].(float64); ok {
					totalUsage.InputTokens = int64(inputTokens)
					hasUpstreamUsage = true
				} else if inputTokens, ok := usageObj["prompt_tokens"].(float64); ok {
					totalUsage.InputTokens = int64(inputTokens)
					hasUpstreamUsage = true
				}
				if outputTokens, ok := usageObj["output_tokens"].(float64); ok {
					totalUsage.OutputTokens = int64(outputTokens)
					hasUpstreamUsage = true
				} else if outputTokens, ok := usageObj["completion_tokens"].(float64); ok {
					totalUsage.OutputTokens = int64(outputTokens)
					hasUpstreamUsage = true
				}
				if totalTokens, ok := usageObj["total_tokens"].(float64); ok {
					totalUsage.TotalTokens = int64(totalTokens)
				}
				log.Debugf("kiro: streamToChannel found usage object in event %s: input=%d, output=%d, total=%d",
					eventType, totalUsage.InputTokens, totalUsage.OutputTokens, totalUsage.TotalTokens)
			}

			// Log unknown event types for debugging (to discover new event formats)
			if eventType != "" {
				log.Debugf("kiro: streamToChannel unknown event type: %s, payload: %s", eventType, string(payload))
			}

		}

		// Check nested usage event
		if usageEvent, ok := event["supplementaryWebLinksEvent"].(map[string]interface{}); ok {
			if inputTokens, ok := usageEvent["inputTokens"].(float64); ok {
				totalUsage.InputTokens = int64(inputTokens)
			}
			if outputTokens, ok := usageEvent["outputTokens"].(float64); ok {
				totalUsage.OutputTokens = int64(outputTokens)
			}
		}

		// Check for direct token fields in any event (fallback)
		if totalUsage.InputTokens == 0 {
			if inputTokens, ok := event["inputTokens"].(float64); ok {
				totalUsage.InputTokens = int64(inputTokens)
				log.Debugf("kiro: streamToChannel found direct inputTokens: %d", totalUsage.InputTokens)
			}
		}
		if totalUsage.OutputTokens == 0 {
			if outputTokens, ok := event["outputTokens"].(float64); ok {
				totalUsage.OutputTokens = int64(outputTokens)
				log.Debugf("kiro: streamToChannel found direct outputTokens: %d", totalUsage.OutputTokens)
			}
		}

		// Check for usage object in any event (OpenAI format)
		if totalUsage.InputTokens == 0 || totalUsage.OutputTokens == 0 {
			if usageObj, ok := event["usage"].(map[string]interface{}); ok {
				if totalUsage.InputTokens == 0 {
					if inputTokens, ok := usageObj["input_tokens"].(float64); ok {
						totalUsage.InputTokens = int64(inputTokens)
					} else if inputTokens, ok := usageObj["prompt_tokens"].(float64); ok {
						totalUsage.InputTokens = int64(inputTokens)
					}
				}
				if totalUsage.OutputTokens == 0 {
					if outputTokens, ok := usageObj["output_tokens"].(float64); ok {
						totalUsage.OutputTokens = int64(outputTokens)
					} else if outputTokens, ok := usageObj["completion_tokens"].(float64); ok {
						totalUsage.OutputTokens = int64(outputTokens)
					}
				}
				if totalUsage.TotalTokens == 0 {
					if totalTokens, ok := usageObj["total_tokens"].(float64); ok {
						totalUsage.TotalTokens = int64(totalTokens)
					}
				}
				log.Debugf("kiro: streamToChannel found usage object (fallback): input=%d, output=%d, total=%d",
					totalUsage.InputTokens, totalUsage.OutputTokens, totalUsage.TotalTokens)
			}
		}
	}

	// Close content block if open
	if isTextBlockOpen && contentBlockIndex >= 0 {
		blockStop := kiroclaude.BuildClaudeContentBlockStopEvent(contentBlockIndex)
		sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, blockStop, &translatorParam)
		for _, chunk := range sseData {
			if chunk != "" {
				out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
			}
		}
	}

	// Streaming token calculation - calculate output tokens from accumulated content
	// Only use local estimation if server didn't provide usage (server-side usage takes priority)
	if totalUsage.OutputTokens == 0 && accumulatedContent.Len() > 0 {
		// Try to use tiktoken for accurate counting
		if enc, err := getTokenizer(model); err == nil {
			if tokenCount, countErr := enc.Count(accumulatedContent.String()); countErr == nil {
				totalUsage.OutputTokens = int64(tokenCount)
				log.Debugf("kiro: streamToChannel calculated output tokens using tiktoken: %d", totalUsage.OutputTokens)
			} else {
				// Fallback on count error: estimate from character count
				totalUsage.OutputTokens = int64(accumulatedContent.Len() / 4)
				if totalUsage.OutputTokens == 0 {
					totalUsage.OutputTokens = 1
				}
				log.Debugf("kiro: streamToChannel tiktoken count failed, estimated from chars: %d", totalUsage.OutputTokens)
			}
		} else {
			// Fallback: estimate from character count (roughly 4 chars per token)
			totalUsage.OutputTokens = int64(accumulatedContent.Len() / 4)
			if totalUsage.OutputTokens == 0 {
				totalUsage.OutputTokens = 1
			}
			log.Debugf("kiro: streamToChannel estimated output tokens from chars: %d (content len: %d)", totalUsage.OutputTokens, accumulatedContent.Len())
		}
	} else if totalUsage.OutputTokens == 0 && outputLen > 0 {
		// Legacy fallback using outputLen
		totalUsage.OutputTokens = int64(outputLen / 4)
		if totalUsage.OutputTokens == 0 {
			totalUsage.OutputTokens = 1
		}
	}

	// Use contextUsagePercentage to calculate more accurate input tokens
	// Kiro model has 200k max context, contextUsagePercentage represents the percentage used
	// Formula: input_tokens = contextUsagePercentage * 200000 / 100
	// Note: The effective input context is ~170k (200k - 30k reserved for output)
	if upstreamContextPercentage > 0 {
		// Calculate input tokens from context percentage
		// Using 200k as the base since that's what Kiro reports against
		calculatedInputTokens := int64(upstreamContextPercentage * 200000 / 100)

		// Only use calculated value if it's significantly different from local estimate
		// This provides more accurate token counts based on upstream data
		if calculatedInputTokens > 0 {
			localEstimate := totalUsage.InputTokens
			totalUsage.InputTokens = calculatedInputTokens
			log.Debugf("kiro: using contextUsagePercentage (%.2f%%) to calculate input tokens: %d (local estimate was: %d)",
				upstreamContextPercentage, calculatedInputTokens, localEstimate)
		}
	}

	totalUsage.TotalTokens = totalUsage.InputTokens + totalUsage.OutputTokens

	// Log upstream usage information if received
	if hasUpstreamUsage {
		log.Debugf("kiro: upstream usage - credits: %.4f, context: %.2f%%, final tokens - input: %d, output: %d, total: %d",
			upstreamCreditUsage, upstreamContextPercentage,
			totalUsage.InputTokens, totalUsage.OutputTokens, totalUsage.TotalTokens)
	}

	// Determine stop reason: prefer upstream, then detect tool_use, default to end_turn
	// SOFT_LIMIT_REACHED: Keep stop_reason = "tool_use" so Claude continues the loop
	stopReason := upstreamStopReason
	if hasTruncatedTools {
		// Log that we're using SOFT_LIMIT_REACHED approach
		log.Infof("kiro: streamToChannel using SOFT_LIMIT_REACHED - keeping stop_reason=tool_use for truncated tools")
	}
	if stopReason == "" {
		if hasToolUses {
			stopReason = "tool_use"
			log.Debugf("kiro: streamToChannel using fallback stop_reason: tool_use")
		} else {
			stopReason = "end_turn"
			log.Debugf("kiro: streamToChannel using fallback stop_reason: end_turn")
		}
	}

	// Log warning if response was truncated due to max_tokens
	if stopReason == "max_tokens" {
		log.Warnf("kiro: response truncated due to max_tokens limit (streamToChannel)")
	}

	// Send message_delta event
	msgDelta := kiroclaude.BuildClaudeMessageDeltaEvent(stopReason, totalUsage)
	sseData := sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, msgDelta, &translatorParam)
	for _, chunk := range sseData {
		if chunk != "" {
			out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
		}
	}

	// Send message_stop event separately
	msgStop := kiroclaude.BuildClaudeMessageStopOnlyEvent()
	sseData = sdktranslator.TranslateStream(ctx, sdktranslator.FromString("kiro"), targetFormat, model, originalReq, claudeBody, msgStop, &translatorParam)
	for _, chunk := range sseData {
		if chunk != "" {
			out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
		}
	}
	// reporter.publish is called via defer
}

// ══════════════════════════════════════════════════════════════════════════════
// Web Search Handler (MCP API)
// ══════════════════════════════════════════════════════════════════════════════

// fetchToolDescription caching:
// Uses a mutex + fetched flag to ensure only one goroutine fetches at a time,
// with automatic retry on failure:
// - On failure, fetched stays false so subsequent calls will retry
// - On success, fetched is set to true — subsequent calls skip immediately (mutex-free fast path)
// The cached description is stored in the translator package via kiroclaude.SetWebSearchDescription(),
// enabling the translator's convertClaudeToolsToKiro to read it when building Kiro requests.
var (
	toolDescMu      sync.Mutex
	toolDescFetched atomic.Bool
)

// fetchToolDescription calls MCP tools/list to get the web_search tool description
// and caches it. Safe to call concurrently — only one goroutine fetches at a time.
// If the fetch fails, subsequent calls will retry. On success, no further fetches occur.
// The httpClient parameter allows reusing a shared pooled HTTP client.
