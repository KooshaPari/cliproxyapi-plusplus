package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	kiroauth "github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/auth/kiro"
	kiroclaude "github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/translator/kiro/claude"
	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/util"
	clipproxyauth "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/cliproxy/auth"
	clipproxyexecutor "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/cliproxy/executor"
	sdktranslator "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/translator"
	log "github.com/sirupsen/logrus"
)

// ExecuteStream handles streaming requests to Kiro API.
// Supports automatic token refresh on 401/403 errors and quota fallback on 429.
func (e *KiroExecutor) ExecuteStream(ctx context.Context, auth *clipproxyauth.Auth, req clipproxyexecutor.Request, opts clipproxyexecutor.Options) (_ *clipproxyexecutor.StreamResult, err error) {
	accessToken, profileArn := kiroCredentials(auth)
	if accessToken == "" {
		return nil, fmt.Errorf("kiro: access token not found in auth")
	}

	// Rate limiting: get token key for tracking
	tokenKey := getTokenKey(auth)
	rateLimiter := kiroauth.GetGlobalRateLimiter()
	cooldownMgr := kiroauth.GetGlobalCooldownManager()

	// Check if token is in cooldown period
	if cooldownMgr.IsInCooldown(tokenKey) {
		remaining := cooldownMgr.GetRemainingCooldown(tokenKey)
		reason := cooldownMgr.GetCooldownReason(tokenKey)
		log.Warnf("kiro: token %s is in cooldown (reason: %s), remaining: %v", tokenKey, reason, remaining)
		return nil, fmt.Errorf("kiro: token is in cooldown for %v (reason: %s)", remaining, reason)
	}

	// Wait for rate limiter before proceeding
	log.Debugf("kiro: stream waiting for rate limiter for token %s", tokenKey)
	rateLimiter.WaitForToken(tokenKey)
	log.Debugf("kiro: stream rate limiter cleared for token %s", tokenKey)

	// Check if token is expired before making request (covers both normal and web_search paths)
	if e.isTokenExpired(accessToken) {
		log.Infof("kiro: access token expired, attempting recovery before stream request")

		// 方案 B: 先尝试从文件重新加载 token（后台刷新器可能已更新文件）
		reloadedAuth, reloadErr := e.reloadAuthFromFile(auth)
		if reloadErr == nil && reloadedAuth != nil {
			// 文件中有更新的 token，使用它
			auth = reloadedAuth
			accessToken, profileArn = kiroCredentials(auth)
			log.Infof("kiro: recovered token from file (background refresh) for stream, expires_at: %v", auth.Metadata["expires_at"])
		} else {
			// 文件中的 token 也过期了，执行主动刷新
			log.Debugf("kiro: file reload failed (%v), attempting active refresh for stream", reloadErr)
			refreshedAuth, refreshErr := e.Refresh(ctx, auth)
			if refreshErr != nil {
				log.Warnf("kiro: pre-request token refresh failed: %v", refreshErr)
			} else if refreshedAuth != nil {
				auth = refreshedAuth
				// Persist the refreshed auth to file so subsequent requests use it
				if persistErr := e.persistRefreshedAuth(auth); persistErr != nil {
					log.Warnf("kiro: failed to persist refreshed auth: %v", persistErr)
				}
				accessToken, profileArn = kiroCredentials(auth)
				log.Infof("kiro: token refreshed successfully before stream request")
			}
		}
	}

	// Check for pure web_search request
	// Route to MCP endpoint instead of normal Kiro API
	if kiroclaude.HasWebSearchTool(req.Payload) {
		log.Infof("kiro: detected pure web_search request, routing to MCP endpoint")
		streamWebSearch, errWebSearch := e.handleWebSearchStream(ctx, auth, req, opts, accessToken, profileArn)
		if errWebSearch != nil {
			return nil, errWebSearch
		}
		return &clipproxyexecutor.StreamResult{Chunks: streamWebSearch}, nil
	}

	reporter := newUsageReporter(ctx, e.Identifier(), req.Model, auth)
	defer reporter.trackFailure(ctx, &err)

	from := opts.SourceFormat
	to := sdktranslator.FromString("kiro")
	body := sdktranslator.TranslateRequest(from, to, req.Model, bytes.Clone(req.Payload), true)

	kiroModelID := e.mapModelToKiro(req.Model)

	// Determine agentic mode and effective profile ARN using helper functions
	isAgentic, isChatOnly := determineAgenticMode(req.Model)
	effectiveProfileArn := getEffectiveProfileArnWithWarning(auth, profileArn)

	// Execute stream with retry on 401/403 and 429 (quota exhausted)
	// Note: currentOrigin and kiroPayload are built inside executeStreamWithRetry for each endpoint
	streamKiro, errStreamKiro := e.executeStreamWithRetry(ctx, auth, req, opts, accessToken, effectiveProfileArn, body, from, reporter, kiroModelID, isAgentic, isChatOnly, tokenKey)
	if errStreamKiro != nil {
		return nil, errStreamKiro
	}
	return &clipproxyexecutor.StreamResult{Chunks: streamKiro}, nil
}

// executeStreamWithRetry performs the streaming HTTP request with automatic retry on auth errors.
// Supports automatic fallback between endpoints with different quotas:
// - Amazon Q endpoint (CLI origin) uses Amazon Q Developer quota
// - CodeWhisperer endpoint (AI_EDITOR origin) uses Kiro IDE quota
// Also supports multi-endpoint fallback similar to Antigravity implementation.
// tokenKey is used for rate limiting and cooldown tracking.
func (e *KiroExecutor) executeStreamWithRetry(ctx context.Context, auth *clipproxyauth.Auth, req clipproxyexecutor.Request, opts clipproxyexecutor.Options, accessToken, profileArn string, body []byte, from sdktranslator.Format, reporter *usageReporter, kiroModelID string, isAgentic, isChatOnly bool, tokenKey string) (<-chan clipproxyexecutor.StreamChunk, error) {
	var currentOrigin string
	maxRetries := 2 // Allow retries for token refresh + endpoint fallback
	rateLimiter := kiroauth.GetGlobalRateLimiter()
	cooldownMgr := kiroauth.GetGlobalCooldownManager()
	endpointConfigs := getKiroEndpointConfigs(auth)
	var last429Err error

	for endpointIdx := 0; endpointIdx < len(endpointConfigs); endpointIdx++ {
		endpointConfig := endpointConfigs[endpointIdx]
		url := endpointConfig.URL
		// Use this endpoint's compatible Origin (critical for avoiding 403 errors)
		currentOrigin = endpointConfig.Origin

		// Rebuild payload with the correct origin for this endpoint
		// Each endpoint requires its matching Origin value in the request body
		kiroPayload, thinkingEnabled := buildKiroPayloadForFormat(body, kiroModelID, profileArn, currentOrigin, isAgentic, isChatOnly, from, opts.Headers)

		log.Debugf("kiro: stream trying endpoint %d/%d: %s (Name: %s, Origin: %s)",
			endpointIdx+1, len(endpointConfigs), url, endpointConfig.Name, currentOrigin)

		for attempt := 0; attempt <= maxRetries; attempt++ {
			// Apply human-like delay before first streaming request (not on retries)
			// This mimics natural user behavior patterns
			// Note: Delay is NOT applied during streaming response - only before initial request
			if attempt == 0 && endpointIdx == 0 {
				kiroauth.ApplyHumanLikeDelay()
			}

			httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(kiroPayload))
			if err != nil {
				return nil, err
			}

			httpReq.Header.Set("Content-Type", kiroContentType)
			httpReq.Header.Set("Accept", kiroAcceptStream)
			// Only set X-Amz-Target if specified (Q endpoint doesn't require it)
			if endpointConfig.AmzTarget != "" {
				httpReq.Header.Set("X-Amz-Target", endpointConfig.AmzTarget)
			}
			// Kiro-specific headers
			httpReq.Header.Set("x-amzn-kiro-agent-mode", kiroIDEAgentModeVibe)
			httpReq.Header.Set("x-amzn-codewhisperer-optout", "true")

			// Apply dynamic fingerprint-based headers
			applyDynamicFingerprint(httpReq, auth)

			httpReq.Header.Set("Amz-Sdk-Request", "attempt=1; max=3")
			httpReq.Header.Set("Amz-Sdk-Invocation-Id", uuid.New().String())

			// Bearer token authentication for all auth types (Builder ID, IDC, social, etc.)
			httpReq.Header.Set("Authorization", "Bearer "+accessToken)

			var attrs map[string]string
			if auth != nil {
				attrs = auth.Attributes
			}
			util.ApplyCustomHeadersFromAttrs(httpReq, attrs)

			var authID, authLabel, authType, authValue string
			if auth != nil {
				authID = auth.ID
				authLabel = auth.Label
				authType, authValue = auth.AccountInfo()
			}
			recordAPIRequest(ctx, e.cfg, upstreamRequestLog{
				URL:       url,
				Method:    http.MethodPost,
				Headers:   httpReq.Header.Clone(),
				Body:      kiroPayload,
				Provider:  e.Identifier(),
				AuthID:    authID,
				AuthLabel: authLabel,
				AuthType:  authType,
				AuthValue: authValue,
			})

			httpClient := newKiroHTTPClientWithPooling(ctx, e.cfg, auth, 0)
			httpResp, err := httpClient.Do(httpReq)
			if err != nil {
				recordAPIResponseError(ctx, e.cfg, err)

				// Enhanced socket retry for streaming: Check if error is retryable (network timeout, connection reset, etc.)
				retryCfg := defaultRetryConfig()
				if isRetryableError(err) && attempt < retryCfg.MaxRetries {
					delay := calculateRetryDelay(attempt, retryCfg)
					logRetryAttempt(attempt, retryCfg.MaxRetries, fmt.Sprintf("stream socket error: %v", err), delay, endpointConfig.Name)
					time.Sleep(delay)
					continue
				}

				return nil, err
			}
			recordAPIResponseMetadata(ctx, e.cfg, httpResp.StatusCode, httpResp.Header.Clone())

			// Handle 429 errors (quota exhausted) - try next endpoint
			// Each endpoint has its own quota pool, so we can try different endpoints
			if httpResp.StatusCode == 429 {
				respBody, _ := io.ReadAll(httpResp.Body)
				_ = httpResp.Body.Close()
				appendAPIResponseChunk(ctx, e.cfg, respBody)

				// Record failure and set cooldown for 429
				rateLimiter.MarkTokenFailed(tokenKey)
				cooldownDuration := kiroauth.CalculateCooldownFor429(attempt)
				cooldownMgr.SetCooldown(tokenKey, cooldownDuration, kiroauth.CooldownReason429)
				log.Warnf("kiro: stream rate limit hit (429), token %s set to cooldown for %v", tokenKey, cooldownDuration)

				// Preserve last 429 so callers can correctly backoff when all endpoints are exhausted
				last429Err = statusErr{code: httpResp.StatusCode, msg: string(respBody)}

				log.Warnf("kiro: stream %s endpoint quota exhausted (429), will try next endpoint, body: %s",
					endpointConfig.Name, summarizeErrorBody(httpResp.Header.Get("Content-Type"), respBody))

				// Break inner retry loop to try next endpoint (which has different quota)
				break
			}

			// Handle 5xx server errors with exponential backoff retry
			// Enhanced: Use retryConfig for consistent retry behavior
			if httpResp.StatusCode >= 500 && httpResp.StatusCode < 600 {
				respBody, _ := io.ReadAll(httpResp.Body)
				_ = httpResp.Body.Close()
				appendAPIResponseChunk(ctx, e.cfg, respBody)

				retryCfg := defaultRetryConfig()
				// Check if this specific 5xx code is retryable (502, 503, 504)
				if isRetryableHTTPStatus(httpResp.StatusCode) && attempt < retryCfg.MaxRetries {
					delay := calculateRetryDelay(attempt, retryCfg)
					logRetryAttempt(attempt, retryCfg.MaxRetries, fmt.Sprintf("stream HTTP %d", httpResp.StatusCode), delay, endpointConfig.Name)
					time.Sleep(delay)
					continue
				} else if attempt < maxRetries {
					// Fallback for other 5xx errors (500, 501, etc.)
					backoff := time.Duration(1<<attempt) * time.Second
					if backoff > 30*time.Second {
						backoff = 30 * time.Second
					}
					log.Warnf("kiro: stream server error %d, retrying in %v (attempt %d/%d)", httpResp.StatusCode, backoff, attempt+1, maxRetries)
					time.Sleep(backoff)
					continue
				}
				log.Errorf("kiro: stream server error %d after %d retries", httpResp.StatusCode, maxRetries)
				return nil, statusErr{code: httpResp.StatusCode, msg: string(respBody)}
			}

			// Handle 400 errors - Credential/Validation issues
			// Do NOT switch endpoints - return error immediately
			if httpResp.StatusCode == 400 {
				respBody, _ := io.ReadAll(httpResp.Body)
				_ = httpResp.Body.Close()
				appendAPIResponseChunk(ctx, e.cfg, respBody)

				log.Warnf("kiro: received 400 error (attempt %d/%d), body: %s", attempt+1, maxRetries+1, summarizeErrorBody(httpResp.Header.Get("Content-Type"), respBody))

				// 400 errors indicate request validation issues - return immediately without retry
				return nil, statusErr{code: httpResp.StatusCode, msg: string(respBody)}
			}

			// Handle 401 errors with token refresh and retry
			// 401 = Unauthorized (token expired/invalid) - refresh token
			if httpResp.StatusCode == 401 {
				respBody, _ := io.ReadAll(httpResp.Body)
				_ = httpResp.Body.Close()
				appendAPIResponseChunk(ctx, e.cfg, respBody)

				log.Warnf("kiro: stream received 401 error, attempting token refresh")
				refreshedAuth, refreshErr := e.Refresh(ctx, auth)
				if refreshErr != nil {
					log.Errorf("kiro: token refresh failed: %v", refreshErr)
					return nil, statusErr{code: httpResp.StatusCode, msg: string(respBody)}
				}

				if refreshedAuth != nil {
					auth = refreshedAuth
					// Persist the refreshed auth to file so subsequent requests use it
					if persistErr := e.persistRefreshedAuth(auth); persistErr != nil {
						log.Warnf("kiro: failed to persist refreshed auth: %v", persistErr)
						// Continue anyway - the token is valid for this request
					}
					accessToken, profileArn = kiroCredentials(auth)
					// Rebuild payload with new profile ARN if changed
					kiroPayload, _ = buildKiroPayloadForFormat(body, kiroModelID, profileArn, currentOrigin, isAgentic, isChatOnly, from, opts.Headers)
					if attempt < maxRetries {
						log.Infof("kiro: token refreshed successfully, retrying stream request (attempt %d/%d)", attempt+1, maxRetries+1)
						continue
					}
					log.Infof("kiro: token refreshed successfully, no retries remaining")
				}

				log.Warnf("kiro stream error, status: 401, body: %s", string(respBody))
				return nil, statusErr{code: httpResp.StatusCode, msg: string(respBody)}
			}

			// Handle 402 errors - Monthly Limit Reached
			if httpResp.StatusCode == 402 {
				respBody, _ := io.ReadAll(httpResp.Body)
				_ = httpResp.Body.Close()
				appendAPIResponseChunk(ctx, e.cfg, respBody)

				log.Warnf("kiro: stream received 402 (monthly limit). Upstream body: %s", string(respBody))

				// Return upstream error body directly
				return nil, statusErr{code: httpResp.StatusCode, msg: string(respBody)}
			}

			// Handle 403 errors - Access Denied / Token Expired
			// Do NOT switch endpoints for 403 errors
			if httpResp.StatusCode == 403 {
				respBody, _ := io.ReadAll(httpResp.Body)
				_ = httpResp.Body.Close()
				appendAPIResponseChunk(ctx, e.cfg, respBody)

				// Log the 403 error details for debugging
				log.Warnf("kiro: stream received 403 error (attempt %d/%d), body: %s", attempt+1, maxRetries+1, string(respBody))

				respBodyStr := string(respBody)

				// Check for SUSPENDED status - return immediately without retry
				if strings.Contains(respBodyStr, "SUSPENDED") || strings.Contains(respBodyStr, "TEMPORARILY_SUSPENDED") {
					// Set long cooldown for suspended accounts
					rateLimiter.CheckAndMarkSuspended(tokenKey, respBodyStr)
					cooldownMgr.SetCooldown(tokenKey, kiroauth.LongCooldown, kiroauth.CooldownReasonSuspended)
					log.Errorf("kiro: stream account is suspended, token %s set to cooldown for %v", tokenKey, kiroauth.LongCooldown)
					return nil, statusErr{code: httpResp.StatusCode, msg: "account suspended: " + string(respBody)}
				}

				// Check if this looks like a token-related 403 (some APIs return 403 for expired tokens)
				isTokenRelated := strings.Contains(respBodyStr, "token") ||
					strings.Contains(respBodyStr, "expired") ||
					strings.Contains(respBodyStr, "invalid") ||
					strings.Contains(respBodyStr, "unauthorized")

				if isTokenRelated && attempt < maxRetries {
					log.Warnf("kiro: 403 appears token-related, attempting token refresh")
					refreshedAuth, refreshErr := e.Refresh(ctx, auth)
					if refreshErr != nil {
						log.Errorf("kiro: token refresh failed: %v", refreshErr)
						// Token refresh failed - return error immediately
						return nil, statusErr{code: httpResp.StatusCode, msg: string(respBody)}
					}
					if refreshedAuth != nil {
						auth = refreshedAuth
						// Persist the refreshed auth to file so subsequent requests use it
						if persistErr := e.persistRefreshedAuth(auth); persistErr != nil {
							log.Warnf("kiro: failed to persist refreshed auth: %v", persistErr)
							// Continue anyway - the token is valid for this request
						}
						accessToken, profileArn = kiroCredentials(auth)
						kiroPayload, _ = buildKiroPayloadForFormat(body, kiroModelID, profileArn, currentOrigin, isAgentic, isChatOnly, from, opts.Headers)
						log.Infof("kiro: token refreshed for 403, retrying stream request")
						continue
					}
				}

				// For non-token 403 or after max retries, return error immediately
				// Do NOT switch endpoints for 403 errors
				log.Warnf("kiro: 403 error, returning immediately (no endpoint switch)")
				return nil, statusErr{code: httpResp.StatusCode, msg: string(respBody)}
			}

			if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
				b, _ := io.ReadAll(httpResp.Body)
				appendAPIResponseChunk(ctx, e.cfg, b)
				log.Debugf("kiro stream error, status: %d, body: %s", httpResp.StatusCode, string(b))
				if errClose := httpResp.Body.Close(); errClose != nil {
					log.Errorf("response body close error: %v", errClose)
				}
				return nil, statusErr{code: httpResp.StatusCode, msg: string(b)}
			}

			out := make(chan clipproxyexecutor.StreamChunk)

			// Record success immediately since connection was established successfully
			// Streaming errors will be handled separately
			rateLimiter.MarkTokenSuccess(tokenKey)
			log.Debugf("kiro: stream request successful, token %s marked as success", tokenKey)

			go func(resp *http.Response, thinkingEnabled bool) {
				defer close(out)
				defer func() {
					if r := recover(); r != nil {
						log.Errorf("kiro: panic in stream handler: %v", r)
						out <- clipproxyexecutor.StreamChunk{Err: fmt.Errorf("internal error: %v", r)}
					}
				}()
				defer func() {
					if errClose := resp.Body.Close(); errClose != nil {
						log.Errorf("response body close error: %v", errClose)
					}
				}()

				// Kiro API always returns <thinking> tags regardless of request parameters
				// So we always enable thinking parsing for Kiro responses
				log.Debugf("kiro: stream thinkingEnabled = %v (always true for Kiro)", thinkingEnabled)

				e.streamToChannel(ctx, resp.Body, out, from, payloadRequestedModel(opts, req.Model), opts.OriginalRequest, body, reporter, thinkingEnabled)
			}(httpResp, thinkingEnabled)

			return out, nil
		}
		// Inner retry loop exhausted for this endpoint, try next endpoint
		// Note: This code is unreachable because all paths in the inner loop
		// either return or continue. Kept as comment for documentation.
	}

	// All endpoints exhausted
	if last429Err != nil {
		return nil, last429Err
	}
	return nil, fmt.Errorf("kiro: stream all endpoints exhausted")
}
