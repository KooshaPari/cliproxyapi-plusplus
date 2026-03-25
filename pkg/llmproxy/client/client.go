package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client is an HTTP client for the cliproxyapi++ proxy server.
//
// It covers:
//   - GET  /v1/models           — list available models
//   - POST /v1/chat/completions  — chat completions (non-streaming)
//   - POST /v1/responses         — OpenAI Responses API passthrough
//   - GET  /                     — health / reachability check
//
// Streaming variants are deliberately out of scope for this package; callers
// that need SSE should use [net/http] directly against [Client.BaseURL].
type Client struct {
	cfg  clientConfig
	http *http.Client
}

// New creates a Client with the given options.
//
// Defaults: base URL http://127.0.0.1:8318, timeout 120 s, no auth.
func New(opts ...Option) *Client {
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}
	cfg.baseURL = strings.TrimRight(cfg.baseURL, "/")
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.httpTimeout},
	}
}

// BaseURL returns the proxy base URL this client is configured against.
func (c *Client) BaseURL() string { return c.cfg.baseURL }

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (c *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("cliproxy/client: marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.cfg.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	// LLM API key (Bearer token for /v1/* routes)
	if c.cfg.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.apiKey)
	}
	return req, nil
}

func (c *Client) do(req *http.Request) ([]byte, int, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("cliproxy/client: HTTP %s %s: %w", req.Method, req.URL.Path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("cliproxy/client: read response body: %w", err)
	}
	return data, resp.StatusCode, nil
}

func (c *Client) doJSON(req *http.Request, out any) error {
	data, code, err := c.do(req)
	if err != nil {
		return err
	}
	if code >= 400 {
		return parseAPIError(code, data)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("cliproxy/client: decode response (HTTP %d): %w", code, err)
	}
	return nil
}

// parseAPIError extracts a structured error from a non-2xx response body.
// It mirrors the error shape produced by _make_error_body in the Python adapter.
func parseAPIError(code int, body []byte) *APIError {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
			Code    any    `json:"code"`
		} `json:"error"`
	}
	msg := strings.TrimSpace(string(body))
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Error.Message != "" {
		msg = envelope.Error.Message
	}
	if msg == "" {
		msg = fmt.Sprintf("proxy returned HTTP %d", code)
	}
	return &APIError{StatusCode: code, Message: msg, Code: envelope.Error.Code}
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// Health performs a lightweight GET / against the proxy and reports whether it
// is reachable.  A nil error means the server responded with HTTP 2xx.
func (c *Client) Health(ctx context.Context) error {
	req, err := c.newRequest(ctx, http.MethodGet, "/", nil)
	if err != nil {
		return err
	}
	_, code, err := c.do(req)
	if err != nil {
		return err
	}
	if code >= 400 {
		return fmt.Errorf("cliproxy/client: health check failed with HTTP %d", code)
	}
	return nil
}

// ListModels calls GET /v1/models and returns the normalised model list.
//
// cliproxyapi++ transforms the upstream OpenAI-compatible {"data":[...]} shape
// into {"models":[...]} for Codex compatibility.  This method handles both
// shapes transparently.
func (c *Client) ListModels(ctx context.Context) (*ModelsResponse, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/models", nil)
	if err != nil {
		return nil, err
	}

	// Use the underlying Do directly so we can read the response headers.
	httpResp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cliproxy/client: GET /v1/models: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	data, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("cliproxy/client: read /v1/models body: %w", err)
	}
	if httpResp.StatusCode >= 400 {
		return nil, parseAPIError(httpResp.StatusCode, data)
	}

	// The proxy normalises the response to {"models":[...]}.
	// Fall back to the raw OpenAI {"data":[...], "object":"list"} shape for
	// consumers that hit the upstream directly.
	var result ModelsResponse
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("cliproxy/client: decode /v1/models: %w", err)
	}

	if modelsJSON, ok := raw["models"]; ok {
		if err := json.Unmarshal(modelsJSON, &result.Models); err != nil {
			return nil, fmt.Errorf("cliproxy/client: decode models array: %w", err)
		}
	} else if dataJSON, ok := raw["data"]; ok {
		if err := json.Unmarshal(dataJSON, &result.Models); err != nil {
			return nil, fmt.Errorf("cliproxy/client: decode data array: %w", err)
		}
	}

	// Capture ETag from response header (set by the proxy for cache validation).
	result.ETag = httpResp.Header.Get("x-models-etag")

	return &result, nil
}

// ChatCompletion sends a non-streaming POST /v1/chat/completions request.
//
// For streaming completions use net/http directly; this package does not wrap
// SSE streams in order to avoid pulling in additional dependencies.
func (c *Client) ChatCompletion(ctx context.Context, r ChatCompletionRequest) (*ChatCompletionResponse, error) {
	r.Stream = false // enforce non-streaming
	req, err := c.newRequest(ctx, http.MethodPost, "/v1/chat/completions", r)
	if err != nil {
		return nil, err
	}
	var out ChatCompletionResponse
	if err := c.doJSON(req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Responses sends a non-streaming POST /v1/responses request (OpenAI Responses
// API).  The proxy transparently bridges this to /v1/chat/completions when the
// backend does not natively support the Responses endpoint.
//
// The raw decoded JSON is returned as map[string]any to remain forward-
// compatible as the Responses API schema evolves.
func (c *Client) Responses(ctx context.Context, r ResponsesRequest) (map[string]any, error) {
	r.Stream = false
	req, err := c.newRequest(ctx, http.MethodPost, "/v1/responses", r)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := c.doJSON(req, &out); err != nil {
		return nil, err
	}
	return out, nil
}
