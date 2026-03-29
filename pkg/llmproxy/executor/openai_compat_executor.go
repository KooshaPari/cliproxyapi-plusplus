package executor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/config"
	"github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/thinking"
	"github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/util"
	cliproxyauth "github.com/kooshapari/CLIProxyAPI/v7/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/kooshapari/CLIProxyAPI/v7/sdk/cliproxy/executor"
	sdktranslator "github.com/kooshapari/CLIProxyAPI/v7/sdk/translator"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// OpenAICompatExecutor implements a stateless executor for OpenAI-compatible providers.
// It performs request/response translation and executes against the provider base URL
// using per-auth credentials (API key) and per-auth HTTP transport (proxy) from context.
type OpenAICompatExecutor struct {
	provider string
	cfg      *config.Config
}

// NewOpenAICompatExecutor creates an executor bound to a provider key (e.g., "openrouter").
func NewOpenAICompatExecutor(provider string, cfg *config.Config) *OpenAICompatExecutor {
	return &OpenAICompatExecutor{provider: provider, cfg: cfg}
}

// Identifier implements cliproxyauth.ProviderExecutor.
func (e *OpenAICompatExecutor) Identifier() string { return e.provider }

// PrepareRequest injects OpenAI-compatible credentials into the outgoing HTTP request.
func (e *OpenAICompatExecutor) PrepareRequest(req *http.Request, auth *cliproxyauth.Auth) error {
	if req == nil {
		return nil
	}
	_, apiKey := e.resolveCredentials(auth)
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	var attrs map[string]string
	if auth != nil {
		attrs = auth.Attributes
	}
	util.ApplyCustomHeadersFromAttrs(req, attrs)
	return nil
}

// HttpRequest injects OpenAI-compatible credentials into the request and executes it.
func (e *OpenAICompatExecutor) HttpRequest(ctx context.Context, auth *cliproxyauth.Auth, req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("openai compat executor: request is nil")
	}
	if ctx == nil {
		ctx = req.Context()
	}
	httpReq := req.WithContext(ctx)
	if err := e.PrepareRequest(httpReq, auth); err != nil {
		return nil, err
	}
	httpClient := newProxyAwareHTTPClient(ctx, e.cfg, auth, 0)
	return httpClient.Do(httpReq)
}

func (e *OpenAICompatExecutor) Execute(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (resp cliproxyexecutor.Response, err error) {
	baseModel := thinking.ParseSuffix(req.Model).ModelName

	reporter := newUsageReporter(ctx, e.Identifier(), baseModel, auth)
	defer reporter.trackFailure(ctx, &err)

	baseURL, apiKey := e.resolveCredentials(auth)
	if baseURL == "" {
		err = statusErr{code: http.StatusUnauthorized, msg: "missing provider baseURL"}
		return
	}

	from := opts.SourceFormat
	to := sdktranslator.FromString("openai")
	endpoint := "/chat/completions"
	if opts.Alt == "responses/compact" {
		if e.cfg != nil && !e.cfg.IsResponsesCompactEnabled() {
			err = statusErr{code: http.StatusNotFound, msg: "/responses/compact disabled by config"}
			return
		}
		to = sdktranslator.FromString("openai-response")
		endpoint = "/responses/compact"
	}
	originalPayloadSource := req.Payload
	if len(opts.OriginalRequest) > 0 {
		originalPayloadSource = opts.OriginalRequest
	}
	originalPayload := originalPayloadSource
	originalTranslated := sdktranslator.TranslateRequest(from, to, baseModel, originalPayload, opts.Stream)
	translated := sdktranslator.TranslateRequest(from, to, baseModel, req.Payload, opts.Stream)
	requestedModel := payloadRequestedModel(opts, req.Model)
	translated = applyPayloadConfigWithRoot(e.cfg, baseModel, to.String(), "", translated, originalTranslated, requestedModel)
	if opts.Alt == "responses/compact" {
		if updated, errDelete := sjson.DeleteBytes(translated, "stream"); errDelete == nil {
			translated = updated
		}
	}

	translated, err = thinking.ApplyThinking(translated, req.Model, from.String(), to.String(), e.Identifier())
	if err != nil {
		return resp, err
	}

	url := strings.TrimSuffix(baseURL, "/") + endpoint
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(translated))
	if err != nil {
		return resp, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
	httpReq.Header.Set("User-Agent", "cli-proxy-openai-compat")
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
		Body:      translated,
		Provider:  e.Identifier(),
		AuthID:    authID,
		AuthLabel: authLabel,
		AuthType:  authType,
		AuthValue: authValue,
	})

	httpResp, err := ExecuteHTTPRequest(ctx, e.cfg, auth, httpReq, "openai compat executor")
	if err != nil {
		return resp, err
	}
	defer func() {
		if errClose := httpResp.Body.Close(); errClose != nil {
			log.Errorf("openai compat executor: close response body error: %v", errClose)
		}
	}()
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		recordAPIResponseError(ctx, e.cfg, err)
		return resp, err
	}
	if err = validateOpenAICompatJSON(body); err != nil {
		reporter.publishFailure(ctx)
		return resp, err
	}
	appendAPIResponseChunk(ctx, e.cfg, body)
	reporter.publish(ctx, parseOpenAIUsage(body))
	// Ensure we at least record the request even if upstream doesn't return usage
	reporter.ensurePublished(ctx)
	// Translate response back to source format when needed
	var param any
	out := sdktranslator.TranslateNonStream(ctx, to, from, req.Model, opts.OriginalRequest, translated, body, &param)
	resp = cliproxyexecutor.Response{Payload: out, Headers: httpResp.Header.Clone()}
	return resp, nil
}

func (e *OpenAICompatExecutor) ExecuteStream(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (_ *cliproxyexecutor.StreamResult, err error) {
	baseModel := thinking.ParseSuffix(req.Model).ModelName

	reporter := newUsageReporter(ctx, e.Identifier(), baseModel, auth)
	defer reporter.trackFailure(ctx, &err)

	baseURL, apiKey := e.resolveCredentials(auth)
	if baseURL == "" {
		err = statusErr{code: http.StatusUnauthorized, msg: "missing provider baseURL"}
		return nil, err
	}

	from := opts.SourceFormat
	to := sdktranslator.FromString("openai")
	originalPayloadSource := req.Payload
	if len(opts.OriginalRequest) > 0 {
		originalPayloadSource = opts.OriginalRequest
	}
	originalPayload := originalPayloadSource
	originalTranslated := sdktranslator.TranslateRequest(from, to, baseModel, originalPayload, true)
	translated := sdktranslator.TranslateRequest(from, to, baseModel, req.Payload, true)
	requestedModel := payloadRequestedModel(opts, req.Model)
	translated = applyPayloadConfigWithRoot(e.cfg, baseModel, to.String(), "", translated, originalTranslated, requestedModel)

	translated, err = thinking.ApplyThinking(translated, req.Model, from.String(), to.String(), e.Identifier())
	if err != nil {
		return nil, err
	}

	// Request usage data in the final streaming chunk so that token statistics
	// are captured even when the upstream is an OpenAI-compatible provider.
	translated, _ = sjson.SetBytes(translated, "stream_options.include_usage", true)

	url := strings.TrimSuffix(baseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(translated))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
	httpReq.Header.Set("User-Agent", "cli-proxy-openai-compat")
	var attrs map[string]string
	if auth != nil {
		attrs = auth.Attributes
	}
	util.ApplyCustomHeadersFromAttrs(httpReq, attrs)
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
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
		Body:      translated,
		Provider:  e.Identifier(),
		AuthID:    authID,
		AuthLabel: authLabel,
		AuthType:  authType,
		AuthValue: authValue,
	})

	httpResp, err := ExecuteHTTPRequestForStreaming(ctx, e.cfg, auth, httpReq, "openai compat executor")
	if err != nil {
		if shouldFallbackOpenAICompatStream(err, from) {
			return e.executeStreamViaNonStreamFallback(ctx, auth, req, opts)
		}
		return nil, err
	}
	out := make(chan cliproxyexecutor.StreamChunk)
	go func() {
		defer close(out)
		defer func() {
			if errClose := httpResp.Body.Close(); errClose != nil {
				log.Errorf("openai compat executor: close response body error: %v", errClose)
			}
		}()
		scanner := bufio.NewScanner(httpResp.Body)
		scanner.Buffer(nil, 52_428_800) // 50MB
		var param any
		for scanner.Scan() {
			line := scanner.Bytes()
			appendAPIResponseChunk(ctx, e.cfg, line)
			if err := validateOpenAICompatJSON(bytes.Clone(line)); err != nil {
				reporter.publishFailure(ctx)
				out <- cliproxyexecutor.StreamChunk{Err: err}
				return
			}
			if detail, ok := parseOpenAIStreamUsage(line); ok {
				reporter.publish(ctx, detail)
			}
			if len(line) == 0 {
				continue
			}

			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}

			// OpenAI-compatible streams are SSE: lines typically prefixed with "data: ".
			// Pass through translator; it yields one or more chunks for the target schema.
			chunks := sdktranslator.TranslateStream(ctx, to, from, req.Model, opts.OriginalRequest, translated, bytes.Clone(line), &param)
			for i := range chunks {
				out <- cliproxyexecutor.StreamChunk{Payload: chunks[i]}
			}
		}
		if errScan := scanner.Err(); errScan != nil {
			recordAPIResponseError(ctx, e.cfg, errScan)
			reporter.publishFailure(ctx)
			out <- cliproxyexecutor.StreamChunk{Err: errScan}
		}
		// Ensure we record the request if no usage chunk was ever seen
		reporter.ensurePublished(ctx)
	}()
	return &cliproxyexecutor.StreamResult{Headers: httpResp.Header.Clone(), Chunks: out}, nil
}

func shouldFallbackOpenAICompatStream(err error, from sdktranslator.Format) bool {
	if from != sdktranslator.FromString("openai-response") {
		return false
	}
	var status statusErr
	return errors.As(err, &status) && status.code == http.StatusNotAcceptable
}

func (e *OpenAICompatExecutor) executeStreamViaNonStreamFallback(
	ctx context.Context,
	auth *cliproxyauth.Auth,
	req cliproxyexecutor.Request,
	opts cliproxyexecutor.Options,
) (*cliproxyexecutor.StreamResult, error) {
	fallbackReq := req
	if updated, err := sjson.SetBytes(fallbackReq.Payload, "stream", false); err == nil {
		fallbackReq.Payload = updated
	}

	fallbackOpts := opts
	fallbackOpts.Stream = false
	if updated, err := sjson.SetBytes(fallbackOpts.OriginalRequest, "stream", false); err == nil {
		fallbackOpts.OriginalRequest = updated
	}

	resp, err := e.Execute(ctx, auth, fallbackReq, fallbackOpts)
	if err != nil {
		return nil, err
	}

	payload, err := synthesizeOpenAIResponsesCompletionEvent(resp.Payload)
	if err != nil {
		return nil, err
	}

	headers := resp.Headers.Clone()
	if headers == nil {
		headers = make(http.Header)
	}
	headers.Set("Content-Type", "text/event-stream")
	headers.Del("Content-Length")

	out := make(chan cliproxyexecutor.StreamChunk, 1)
	out <- cliproxyexecutor.StreamChunk{Payload: payload}
	close(out)
	return &cliproxyexecutor.StreamResult{Headers: headers, Chunks: out}, nil
}

func synthesizeOpenAIResponsesCompletionEvent(payload []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return nil, statusErr{code: http.StatusBadGateway, msg: "openai compat executor: empty non-stream fallback payload"}
	}
	if !json.Valid(trimmed) {
		return nil, statusErr{code: http.StatusBadGateway, msg: "openai compat executor: invalid non-stream fallback payload"}
	}
	root := gjson.ParseBytes(trimmed)
	if root.Get("object").String() != "chat.completion" {
		for _, path := range []string{"data", "result", "response", "data.response"} {
			candidate := root.Get(path)
			if candidate.Exists() && candidate.Get("object").String() == "chat.completion" {
				root = candidate
				break
			}
		}
	}
	if root.Get("object").String() == "chat.completion" {
		converted, err := convertChatCompletionToResponsesObject(trimmed)
		if err != nil {
			return nil, err
		}
		trimmed = converted
	}
	if gjson.GetBytes(trimmed, "object").String() != "response" {
		return nil, statusErr{code: http.StatusBadGateway, msg: "openai compat executor: fallback payload is not a responses object"}
	}

	responseID := gjson.GetBytes(trimmed, "id").String()
	if responseID == "" {
		responseID = "resp_fallback"
	}
	createdAt := gjson.GetBytes(trimmed, "created_at").Int()
	text := gjson.GetBytes(trimmed, "output.0.content.0.text").String()
	messageID := "msg_" + responseID + "_0"

	var events []string
	appendEvent := func(event, payload string) {
		events = append(events, "event: "+event+"\ndata: "+payload+"\n\n")
	}

	created := `{"type":"response.created","sequence_number":1,"response":{"id":"","object":"response","created_at":0,"status":"in_progress","background":false,"error":null,"output":[]}}`
	created, _ = sjson.Set(created, "response.id", responseID)
	created, _ = sjson.Set(created, "response.created_at", createdAt)
	appendEvent("response.created", created)

	inProgress := `{"type":"response.in_progress","sequence_number":2,"response":{"id":"","object":"response","created_at":0,"status":"in_progress"}}`
	inProgress, _ = sjson.Set(inProgress, "response.id", responseID)
	inProgress, _ = sjson.Set(inProgress, "response.created_at", createdAt)
	appendEvent("response.in_progress", inProgress)

	itemAdded := `{"type":"response.output_item.added","sequence_number":3,"output_index":0,"item":{"id":"","type":"message","status":"in_progress","content":[],"role":"assistant"}}`
	itemAdded, _ = sjson.Set(itemAdded, "item.id", messageID)
	appendEvent("response.output_item.added", itemAdded)

	partAdded := `{"type":"response.content_part.added","sequence_number":4,"item_id":"","output_index":0,"content_index":0,"part":{"type":"output_text","annotations":[],"logprobs":[],"text":""}}`
	partAdded, _ = sjson.Set(partAdded, "item_id", messageID)
	appendEvent("response.content_part.added", partAdded)

	if text != "" {
		textDelta := `{"type":"response.output_text.delta","sequence_number":5,"item_id":"","output_index":0,"content_index":0,"delta":"","logprobs":[]}`
		textDelta, _ = sjson.Set(textDelta, "item_id", messageID)
		textDelta, _ = sjson.Set(textDelta, "delta", text)
		appendEvent("response.output_text.delta", textDelta)
	}

	textDone := `{"type":"response.output_text.done","sequence_number":6,"item_id":"","output_index":0,"content_index":0,"text":"","logprobs":[]}`
	textDone, _ = sjson.Set(textDone, "item_id", messageID)
	textDone, _ = sjson.Set(textDone, "text", text)
	appendEvent("response.output_text.done", textDone)

	partDone := `{"type":"response.content_part.done","sequence_number":7,"item_id":"","output_index":0,"content_index":0,"part":{"type":"output_text","annotations":[],"logprobs":[],"text":""}}`
	partDone, _ = sjson.Set(partDone, "item_id", messageID)
	partDone, _ = sjson.Set(partDone, "part.text", text)
	appendEvent("response.content_part.done", partDone)

	itemDone := `{"type":"response.output_item.done","sequence_number":8,"output_index":0,"item":{"id":"","type":"message","status":"completed","content":[{"type":"output_text","annotations":[],"logprobs":[],"text":""}],"role":"assistant"}}`
	itemDone, _ = sjson.Set(itemDone, "item.id", messageID)
	itemDone, _ = sjson.Set(itemDone, "item.content.0.text", text)
	appendEvent("response.output_item.done", itemDone)

	completed := `{"type":"response.completed","sequence_number":0,"response":{"id":"","object":"response","created_at":0,"status":"completed","background":false,"error":null}}`
	var err error
	completed, err = sjson.Set(completed, "sequence_number", 9)
	if err != nil {
		return nil, fmt.Errorf("openai compat executor: set completion sequence: %w", err)
	}
	completed, err = sjson.SetRaw(completed, "response", string(trimmed))
	if err != nil {
		return nil, fmt.Errorf("openai compat executor: wrap non-stream fallback payload: %w", err)
	}
	appendEvent("response.completed", completed)
	return []byte(strings.Join(events, "")), nil
}

func convertChatCompletionToResponsesObject(payload []byte) ([]byte, error) {
	root := gjson.ParseBytes(payload)
	if !root.Get("choices").Exists() {
		for _, path := range []string{"data", "result", "response", "data.response"} {
			candidate := root.Get(path)
			if candidate.Exists() && candidate.Get("choices").Exists() {
				root = candidate
				break
			}
		}
	}

	choice := root.Get("choices.0")
	if !choice.Exists() {
		return nil, statusErr{code: http.StatusBadGateway, msg: "openai compat executor: chat completion fallback missing choices"}
	}

	text := choice.Get("message.content").String()
	response := `{"id":"","object":"response","created_at":0,"status":"completed","output":[],"usage":{"input_tokens":0,"output_tokens":0,"total_tokens":0}}`
	var err error
	if response, err = sjson.Set(response, "id", root.Get("id").String()); err != nil {
		return nil, err
	}
	if response, err = sjson.Set(response, "created_at", root.Get("created").Int()); err != nil {
		return nil, err
	}
	if response, err = sjson.Set(response, "model", root.Get("model").String()); err != nil {
		return nil, err
	}
	if response, err = sjson.SetRaw(response, "output", `[{"type":"message","role":"assistant","content":[{"type":"output_text","text":""}]}]`); err != nil {
		return nil, err
	}
	if response, err = sjson.Set(response, "output.0.content.0.text", text); err != nil {
		return nil, err
	}
	if response, err = sjson.Set(response, "usage.input_tokens", root.Get("usage.prompt_tokens").Int()); err != nil {
		return nil, err
	}
	if response, err = sjson.Set(response, "usage.output_tokens", root.Get("usage.completion_tokens").Int()); err != nil {
		return nil, err
	}
	if response, err = sjson.Set(response, "usage.total_tokens", root.Get("usage.total_tokens").Int()); err != nil {
		return nil, err
	}
	return []byte(response), nil
}

func (e *OpenAICompatExecutor) CountTokens(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	baseModel := thinking.ParseSuffix(req.Model).ModelName

	from := opts.SourceFormat
	to := sdktranslator.FromString("openai")
	translated := sdktranslator.TranslateRequest(from, to, baseModel, req.Payload, false)

	modelForCounting := baseModel

	translated, err := thinking.ApplyThinking(translated, req.Model, from.String(), to.String(), e.Identifier())
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}

	enc, err := tokenizerForModel(modelForCounting)
	if err != nil {
		return cliproxyexecutor.Response{}, fmt.Errorf("openai compat executor: tokenizer init failed: %w", err)
	}

	count, err := countOpenAIChatTokens(enc, translated)
	if err != nil {
		return cliproxyexecutor.Response{}, fmt.Errorf("openai compat executor: token counting failed: %w", err)
	}

	usageJSON := buildOpenAIUsageJSON(count)
	translatedUsage := sdktranslator.TranslateTokenCount(ctx, to, from, count, usageJSON)
	return cliproxyexecutor.Response{Payload: translatedUsage}, nil
}

// Refresh is a no-op for API-key based compatibility providers.
func (e *OpenAICompatExecutor) Refresh(ctx context.Context, auth *cliproxyauth.Auth) (*cliproxyauth.Auth, error) {
	log.Debugf("openai compat executor: refresh called")
	_ = ctx
	return auth, nil
}

func (e *OpenAICompatExecutor) resolveCredentials(auth *cliproxyauth.Auth) (baseURL, apiKey string) {
	if auth == nil {
		return "", ""
	}
	if auth.Attributes != nil {
		baseURL = strings.TrimSpace(auth.Attributes["base_url"])
		apiKey = strings.TrimSpace(auth.Attributes["api_key"])
	}
	return
}

type statusErr struct {
	code       int
	msg        string
	retryAfter *time.Duration
}

func (e statusErr) Error() string {
	if e.msg != "" {
		return e.msg
	}
	return fmt.Sprintf("status %d", e.code)
}
func (e statusErr) StatusCode() int            { return e.code }
func (e statusErr) RetryAfter() *time.Duration { return e.retryAfter }

func validateOpenAICompatJSON(data []byte) error {
	line := bytes.TrimSpace(data)
	if len(line) == 0 {
		return nil
	}

	if bytes.HasPrefix(line, []byte("data:")) {
		payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			return nil
		}
		line = payload
	}

	if !json.Valid(line) {
		return statusErr{code: http.StatusBadRequest, msg: "invalid json in OpenAI-compatible response"}
	}

	return nil
}

func (e *OpenAICompatExecutor) CloseExecutionSession(sessionID string) {}
