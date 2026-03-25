package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	kiroclaude "github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/translator/kiro/claude"
	clipproxyauth "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/cliproxy/auth"
	clipproxyexecutor "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/cliproxy/executor"
	sdktranslator "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/translator"
	log "github.com/sirupsen/logrus"
)

// Global state for web search tool description caching
// enabling the translator's convertClaudeToolsToKiro to read it when building Kiro requests.
var (
	toolDescMu      sync.Mutex
	toolDescFetched atomic.Bool
)

func fetchToolDescription(ctx context.Context, mcpEndpoint, authToken string, httpClient *http.Client, auth *cliproxyauth.Auth, authAttrs map[string]string) {
	// Fast path: already fetched successfully, no lock needed
	if toolDescFetched.Load() {
		return
	}

	toolDescMu.Lock()
	defer toolDescMu.Unlock()

	// Double-check after acquiring lock
	if toolDescFetched.Load() {
		return
	}

	handler := newWebSearchHandler(ctx, mcpEndpoint, authToken, httpClient, auth, authAttrs)
	reqBody := []byte(`{"id":"tools_list","jsonrpc":"2.0","method":"tools/list"}`)
	log.Debugf("kiro/websearch MCP tools/list request: %d bytes", len(reqBody))

	req, err := http.NewRequestWithContext(ctx, "POST", mcpEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		log.Warnf("kiro/websearch: failed to create tools/list request: %v", err)
		return
	}

	// Reuse same headers as callMcpAPI
	handler.setMcpHeaders(req)

	resp, err := handler.httpClient.Do(req)
	if err != nil {
		log.Warnf("kiro/websearch: tools/list request failed: %v", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Warnf("kiro/websearch: tools/list returned status %d", resp.StatusCode)
		return
	}
	log.Debugf("kiro/websearch MCP tools/list response: [%d] %d bytes", resp.StatusCode, len(body))

	// Parse: {"result":{"tools":[{"name":"web_search","description":"..."}]}}
	var result struct {
		Result *struct {
			Tools []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.Result == nil {
		log.Warnf("kiro/websearch: failed to parse tools/list response")
		return
	}

	for _, tool := range result.Result.Tools {
		if tool.Name == "web_search" && tool.Description != "" {
			kiroclaude.SetWebSearchDescription(tool.Description)
			toolDescFetched.Store(true) // success — no more fetches
			log.Infof("kiro/websearch: cached web_search description from tools/list (%d bytes)", len(tool.Description))
			return
		}
	}

	// web_search tool not found in response
	log.Warnf("kiro/websearch: web_search tool not found in tools/list response")
}

// webSearchHandler handles web search requests via Kiro MCP API
type webSearchHandler struct {
	ctx         context.Context
	mcpEndpoint string
	httpClient  *http.Client
	authToken   string
	auth        *cliproxyauth.Auth // for applyDynamicFingerprint
	authAttrs   map[string]string  // optional, for custom headers from auth.Attributes
}

// newWebSearchHandler creates a new webSearchHandler.
// If httpClient is nil, a default client with 30s timeout is used.
// Pass a shared pooled client (e.g. from getKiroPooledHTTPClient) for connection reuse.
func newWebSearchHandler(ctx context.Context, mcpEndpoint, authToken string, httpClient *http.Client, auth *cliproxyauth.Auth, authAttrs map[string]string) *webSearchHandler {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}
	return &webSearchHandler{
		ctx:         ctx,
		mcpEndpoint: mcpEndpoint,
		httpClient:  httpClient,
		authToken:   authToken,
		auth:        auth,
		authAttrs:   authAttrs,
	}
}

// setMcpHeaders sets standard MCP API headers on the request,
// aligned with the GAR request pattern.
func (h *webSearchHandler) setMcpHeaders(req *http.Request) {
	// 1. Content-Type & Accept (aligned with GAR)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")

	// 2. Kiro-specific headers (aligned with GAR)
	req.Header.Set("x-amzn-kiro-agent-mode", "vibe")
	req.Header.Set("x-amzn-codewhisperer-optout", "true")

	// 3. User-Agent: Reuse applyDynamicFingerprint for consistency
	applyDynamicFingerprint(req, h.auth)

	// 4. AWS SDK identifiers
	req.Header.Set("Amz-Sdk-Request", "attempt=1; max=3")
	req.Header.Set("Amz-Sdk-Invocation-Id", uuid.New().String())

	// 5. Authentication
	req.Header.Set("Authorization", "Bearer "+h.authToken)

	// 6. Custom headers from auth attributes
	util.ApplyCustomHeadersFromAttrs(req, h.authAttrs)
}

// mcpMaxRetries is the maximum number of retries for MCP API calls.
const mcpMaxRetries = 2

// callMcpAPI calls the Kiro MCP API with the given request.
// Includes retry logic with exponential backoff for retryable errors.
func (h *webSearchHandler) callMcpAPI(request *kiroclaude.McpRequest) (*kiroclaude.McpResponse, error) {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal MCP request: %w", err)
	}
	log.Debugf("kiro/websearch MCP request → %s (%d bytes)", h.mcpEndpoint, len(requestBody))

	var lastErr error
	for attempt := 0; attempt <= mcpMaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<attempt) * time.Second
			if backoff > 10*time.Second {
				backoff = 10 * time.Second
			}
			log.Warnf("kiro/websearch: MCP retry %d/%d after %v (last error: %v)", attempt, mcpMaxRetries, backoff, lastErr)
			select {
			case <-h.ctx.Done():
				return nil, h.ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(h.ctx, "POST", h.mcpEndpoint, bytes.NewReader(requestBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP request: %w", err)
		}

		h.setMcpHeaders(req)

		resp, err := h.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("MCP API request failed: %w", err)
			continue // network error → retry
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read MCP response: %w", err)
			continue // read error → retry
		}
		log.Debugf("kiro/websearch MCP response ← [%d] (%d bytes)", resp.StatusCode, len(body))

		// Retryable HTTP status codes (aligned with GAR: 502, 503, 504)
		if resp.StatusCode >= 502 && resp.StatusCode <= 504 {
			lastErr = fmt.Errorf("MCP API returned retryable status %d: %s", resp.StatusCode, string(body))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("MCP API returned status %d: %s", resp.StatusCode, string(body))
		}

		var mcpResponse kiroclaude.McpResponse
		if err := json.Unmarshal(body, &mcpResponse); err != nil {
			return nil, fmt.Errorf("failed to parse MCP response: %w", err)
		}

		if mcpResponse.Error != nil {
			code := -1
			if mcpResponse.Error.Code != nil {
				code = *mcpResponse.Error.Code
			}
			msg := "Unknown error"
			if mcpResponse.Error.Message != nil {
				msg = *mcpResponse.Error.Message
			}
			return nil, fmt.Errorf("MCP error %d: %s", code, msg)
		}

		return &mcpResponse, nil
	}

	return nil, lastErr
}

// webSearchAuthAttrs extracts auth attributes for MCP calls.
// Used by handleWebSearch and handleWebSearchStream to pass custom headers.
func webSearchAuthAttrs(auth *cliproxyauth.Auth) map[string]string {
	if auth != nil {
		return auth.Attributes
	}
	return nil
}

const maxWebSearchIterations = 5

// handleWebSearchStream handles web_search requests:
// Step 1: tools/list (sync) → fetch/cache tool description
// Step 2+: MCP search → InjectToolResultsClaude → callKiroAndBuffer loop
// Note: We skip the "model decides to search" step because Claude Code already
// decided to use web_search. The Kiro tool description restricts non-coding
// topics, so asking the model again would cause it to refuse valid searches.
func (e *KiroExecutor) handleWebSearchStream(
	ctx context.Context,
	auth *cliproxyauth.Auth,
	req cliproxyexecutor.Request,
	opts cliproxyexecutor.Options,
	accessToken, profileArn string,
) (<-chan cliproxyexecutor.StreamChunk, error) {
	// Extract search query from Claude Code's web_search tool_use
	query := kiroclaude.ExtractSearchQuery(req.Payload)
	if query == "" {
		log.Warnf("kiro/websearch: failed to extract search query, falling back to normal flow")
		return e.callKiroDirectStream(ctx, auth, req, opts, accessToken, profileArn)
	}

	// Build MCP endpoint using shared region resolution (supports api_region + ProfileARN fallback)
	region := resolveKiroAPIRegion(auth)
	mcpEndpoint := kiroclaude.BuildMcpEndpoint(region)

	// ── Step 1: tools/list (SYNC) — cache tool description ──
	{
		authAttrs := webSearchAuthAttrs(auth)
		fetchToolDescription(ctx, mcpEndpoint, accessToken, newKiroHTTPClientWithPooling(ctx, e.cfg, auth, 30*time.Second), auth, authAttrs)
	}

	// Create output channel
	out := make(chan cliproxyexecutor.StreamChunk)

	// Usage reporting: track web search requests like normal streaming requests
	reporter := newUsageReporter(ctx, e.Identifier(), req.Model, auth)

	go func() {
		var wsErr error
		defer reporter.trackFailure(ctx, &wsErr)
		defer close(out)

		// Estimate input tokens using tokenizer (matching streamToChannel pattern)
		var totalUsage usage.Detail
		if enc, tokErr := getTokenizer(req.Model); tokErr == nil {
			if inp, e := countClaudeChatTokens(enc, req.Payload); e == nil && inp > 0 {
				totalUsage.InputTokens = inp
			} else {
				totalUsage.InputTokens = int64(len(req.Payload) / 4)
			}
		} else {
			totalUsage.InputTokens = int64(len(req.Payload) / 4)
		}
		if totalUsage.InputTokens == 0 && len(req.Payload) > 0 {
			totalUsage.InputTokens = 1
		}
		var accumulatedOutputLen int
		defer func() {
			if wsErr != nil {
				return // let trackFailure handle failure reporting
			}
			totalUsage.OutputTokens = int64(accumulatedOutputLen / 4)
			if accumulatedOutputLen > 0 && totalUsage.OutputTokens == 0 {
				totalUsage.OutputTokens = 1
			}
			reporter.publish(ctx, totalUsage)
		}()

		// Send message_start event to client (aligned with streamToChannel pattern)
		// Use payloadRequestedModel to return user's original model alias
		msgStart := kiroclaude.BuildClaudeMessageStartEvent(
			payloadRequestedModel(opts, req.Model),
			totalUsage.InputTokens,
		)
		select {
		case <-ctx.Done():
			return
		case out <- cliproxyexecutor.StreamChunk{Payload: append(msgStart, '\n', '\n')}:
		}

		// ── Step 2+: MCP search → InjectToolResultsClaude → callKiroAndBuffer loop ──
		contentBlockIndex := 0
		currentQuery := query

		// Replace web_search tool description with a minimal one that allows re-search.
		// The original tools/list description from Kiro restricts non-coding topics,
		// but we've already decided to search. We keep the tool so the model can
		// request additional searches when results are insufficient.
		simplifiedPayload, simplifyErr := kiroclaude.ReplaceWebSearchToolDescription(bytes.Clone(req.Payload))
		if simplifyErr != nil {
			log.Warnf("kiro/websearch: failed to simplify web_search tool: %v, using original payload", simplifyErr)
			simplifiedPayload = bytes.Clone(req.Payload)
		}

		currentClaudePayload := simplifiedPayload
		totalSearches := 0

		// Generate toolUseId for the first iteration (Claude Code already decided to search)
		currentToolUseId := fmt.Sprintf("srvtoolu_%s", kiroclaude.GenerateToolUseID())

		for iteration := 0; iteration < maxWebSearchIterations; iteration++ {
			log.Infof("kiro/websearch: search iteration %d/%d",
				iteration+1, maxWebSearchIterations)

			// MCP search
			_, mcpRequest := kiroclaude.CreateMcpRequest(currentQuery)

			authAttrs := webSearchAuthAttrs(auth)
			handler := newWebSearchHandler(ctx, mcpEndpoint, accessToken, newKiroHTTPClientWithPooling(ctx, e.cfg, auth, 30*time.Second), auth, authAttrs)
			mcpResponse, mcpErr := handler.callMcpAPI(mcpRequest)

			var searchResults *kiroclaude.WebSearchResults
			if mcpErr != nil {
				log.Warnf("kiro/websearch: MCP API call failed: %v, continuing with empty results", mcpErr)
			} else {
				searchResults = kiroclaude.ParseSearchResults(mcpResponse)
			}

			resultCount := 0
			if searchResults != nil {
				resultCount = len(searchResults.Results)
			}
			totalSearches++
			log.Infof("kiro/websearch: iteration %d — got %d search results", iteration+1, resultCount)

			// Send search indicator events to client
			searchEvents := kiroclaude.GenerateSearchIndicatorEvents(currentQuery, currentToolUseId, searchResults, contentBlockIndex)
			for _, event := range searchEvents {
				select {
				case <-ctx.Done():
					return
				case out <- cliproxyexecutor.StreamChunk{Payload: event}:
				}
			}
			contentBlockIndex += 2

			// Inject tool_use + tool_result into Claude payload, then call GAR
			var err error
			currentClaudePayload, err = kiroclaude.InjectToolResultsClaude(currentClaudePayload, currentToolUseId, currentQuery, searchResults)
			if err != nil {
				log.Warnf("kiro/websearch: failed to inject tool results: %v", err)
				wsErr = fmt.Errorf("failed to inject tool results: %w", err)
				e.sendFallbackText(ctx, out, contentBlockIndex, currentQuery, searchResults)
				return
			}

			// Call GAR with modified Claude payload (full translation pipeline)
			modifiedReq := req
			modifiedReq.Payload = currentClaudePayload
			kiroChunks, kiroErr := e.callKiroAndBuffer(ctx, auth, modifiedReq, opts, accessToken, profileArn)
			if kiroErr != nil {
				log.Warnf("kiro/websearch: Kiro API failed at iteration %d: %v", iteration+1, kiroErr)
				wsErr = fmt.Errorf("kiro API failed at iteration %d: %w", iteration+1, kiroErr)
				e.sendFallbackText(ctx, out, contentBlockIndex, currentQuery, searchResults)
				return
			}

			// Analyze response
			analysis := kiroclaude.AnalyzeBufferedStream(kiroChunks)
			log.Infof("kiro/websearch: iteration %d — stop_reason: %s, has_tool_use: %v",
				iteration+1, analysis.StopReason, analysis.HasWebSearchToolUse)

			if analysis.HasWebSearchToolUse && analysis.WebSearchQuery != "" && iteration+1 < maxWebSearchIterations {
				// Model wants another search
				filteredChunks := kiroclaude.FilterChunksForClient(kiroChunks, analysis.WebSearchToolUseIndex, contentBlockIndex)
				for _, chunk := range filteredChunks {
					select {
					case <-ctx.Done():
						return
					case out <- cliproxyexecutor.StreamChunk{Payload: chunk}:
					}
				}

				currentQuery = analysis.WebSearchQuery
				currentToolUseId = analysis.WebSearchToolUseId
				continue
			}

			// Model returned final response — stream to client
			for _, chunk := range kiroChunks {
				if contentBlockIndex > 0 && len(chunk) > 0 {
					adjusted, shouldForward := kiroclaude.AdjustSSEChunk(chunk, contentBlockIndex)
					if !shouldForward {
						continue
					}
					accumulatedOutputLen += len(adjusted)
					select {
					case <-ctx.Done():
						return
					case out <- cliproxyexecutor.StreamChunk{Payload: adjusted}:
					}
				} else {
					accumulatedOutputLen += len(chunk)
					select {
					case <-ctx.Done():
						return
					case out <- cliproxyexecutor.StreamChunk{Payload: chunk}:
					}
				}
			}
			log.Infof("kiro/websearch: completed after %d search iteration(s), total searches: %d", iteration+1, totalSearches)
			return
		}

		log.Warnf("kiro/websearch: reached max iterations (%d), stopping search loop", maxWebSearchIterations)
	}()

	return out, nil
}

// handleWebSearch handles web_search requests for non-streaming Execute path.
// Performs MCP search synchronously, injects results into the request payload,
// then calls the normal non-streaming Kiro API path which returns a proper
// Claude JSON response (not SSE chunks).
func (e *KiroExecutor) handleWebSearch(
	ctx context.Context,
	auth *cliproxyauth.Auth,
	req cliproxyexecutor.Request,
	opts cliproxyexecutor.Options,
	accessToken, profileArn string,
) (cliproxyexecutor.Response, error) {
	// Extract search query from Claude Code's web_search tool_use
	query := kiroclaude.ExtractSearchQuery(req.Payload)
	if query == "" {
		log.Warnf("kiro/websearch: non-stream: failed to extract search query, falling back to normal Execute")
		// Fall through to normal non-streaming path
		return e.executeNonStreamFallback(ctx, auth, req, opts, accessToken, profileArn)
	}

	// Build MCP endpoint using shared region resolution (supports api_region + ProfileARN fallback)
	region := resolveKiroAPIRegion(auth)
	mcpEndpoint := kiroclaude.BuildMcpEndpoint(region)

	// Step 1: Fetch/cache tool description (sync)
	{
		authAttrs := webSearchAuthAttrs(auth)
		fetchToolDescription(ctx, mcpEndpoint, accessToken, newKiroHTTPClientWithPooling(ctx, e.cfg, auth, 30*time.Second), auth, authAttrs)
	}

	// Step 2: Perform MCP search
	_, mcpRequest := kiroclaude.CreateMcpRequest(query)

	authAttrs := webSearchAuthAttrs(auth)
	handler := newWebSearchHandler(ctx, mcpEndpoint, accessToken, newKiroHTTPClientWithPooling(ctx, e.cfg, auth, 30*time.Second), auth, authAttrs)
	mcpResponse, mcpErr := handler.callMcpAPI(mcpRequest)

	var searchResults *kiroclaude.WebSearchResults
	if mcpErr != nil {
		log.Warnf("kiro/websearch: non-stream: MCP API call failed: %v, continuing with empty results", mcpErr)
	} else {
		searchResults = kiroclaude.ParseSearchResults(mcpResponse)
	}

	resultCount := 0
	if searchResults != nil {
		resultCount = len(searchResults.Results)
	}
	log.Infof("kiro/websearch: non-stream: got %d search results", resultCount)

	// Step 3: Replace restrictive web_search tool description (align with streaming path)
	simplifiedPayload, simplifyErr := kiroclaude.ReplaceWebSearchToolDescription(bytes.Clone(req.Payload))
	if simplifyErr != nil {
		log.Warnf("kiro/websearch: non-stream: failed to simplify web_search tool: %v, using original payload", simplifyErr)
		simplifiedPayload = bytes.Clone(req.Payload)
	}

	// Step 4: Inject search tool_use + tool_result into Claude payload
	currentToolUseId := fmt.Sprintf("srvtoolu_%s", kiroclaude.GenerateToolUseID())
	modifiedPayload, err := kiroclaude.InjectToolResultsClaude(simplifiedPayload, currentToolUseId, query, searchResults)
	if err != nil {
		log.Warnf("kiro/websearch: non-stream: failed to inject tool results: %v, falling back", err)
		return e.executeNonStreamFallback(ctx, auth, req, opts, accessToken, profileArn)
	}

	// Step 5: Call Kiro API via the normal non-streaming path (executeWithRetry)
	// This path uses parseEventStream → BuildClaudeResponse → TranslateNonStream
	// to produce a proper Claude JSON response
	modifiedReq := req
	modifiedReq.Payload = modifiedPayload

	resp, err := e.executeNonStreamFallback(ctx, auth, modifiedReq, opts, accessToken, profileArn)
	if err != nil {
		return resp, err
	}

	// Step 6: Inject server_tool_use + web_search_tool_result into response
	// so Claude Code can display "Did X searches in Ys"
	indicators := []kiroclaude.SearchIndicator{
		{
			ToolUseID: currentToolUseId,
			Query:     query,
			Results:   searchResults,
		},
	}
	injectedPayload, injErr := kiroclaude.InjectSearchIndicatorsInResponse(resp.Payload, indicators)
	if injErr != nil {
		log.Warnf("kiro/websearch: non-stream: failed to inject search indicators: %v", injErr)
	} else {
		resp.Payload = injectedPayload
	}

	return resp, nil
}

// callKiroAndBuffer calls the Kiro API and buffers all response chunks.
// Returns the buffered chunks for analysis before forwarding to client.
// Usage reporting is NOT done here — the caller (handleWebSearchStream) manages its own reporter.
