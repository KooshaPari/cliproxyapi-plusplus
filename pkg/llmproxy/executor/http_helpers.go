package executor

import (
	"context"
	"io"
	"net/http"

	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/config"
	cliproxyauth "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

// HTTPExecutionResult contains the response and associated metadata from an HTTP request.
type HTTPExecutionResult struct {
	Response *http.Response
}

// ExecuteHTTPRequest is a helper that wraps the common HTTP execution pattern:
// - Creates a proxy-aware HTTP client
// - Executes the request
// - Checks the status code
// - Handles error responses (reads body, logs, appends to response tracking, closes)
// - Records response metadata
//
// On success, the caller must manage the response body and close it.
// On error, the response body is automatically closed and an error is returned with the error details.
//
// Parameters:
//   - ctx: The context for the request
//   - cfg: The application configuration
//   - auth: The authentication information
//   - httpReq: The prepared HTTP request
//   - logPrefix: Optional prefix for logging errors (e.g., "claude executor")
//
// Returns:
//   - *http.Response: The HTTP response if status is 2xx, nil on error
//   - error: An error with status code and error message if status is not 2xx
//   - bool: true if error occurred and was handled
func ExecuteHTTPRequest(
	ctx context.Context,
	cfg *config.Config,
	auth *cliproxyauth.Auth,
	httpReq *http.Request,
	logPrefix string,
) (*http.Response, error) {
	// Create proxy-aware HTTP client
	httpClient := newProxyAwareHTTPClient(ctx, cfg, auth, 0)

	// Execute request
	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		// Network/connection error
		recordAPIResponseError(ctx, cfg, err)
		return nil, err
	}

	// Record response metadata
	recordAPIResponseMetadata(ctx, cfg, httpResp.StatusCode, httpResp.Header.Clone())

	// Check status code
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		// Error status - read body and close
		b, _ := io.ReadAll(httpResp.Body)
		appendAPIResponseChunk(ctx, cfg, b)
		logWithRequestID(ctx).Debugf("request error, error status: %d, error message: %s",
			httpResp.StatusCode, summarizeErrorBody(httpResp.Header.Get("Content-Type"), b))

		if errClose := httpResp.Body.Close(); errClose != nil {
			log.Errorf("%s: close response body error: %v", logPrefix, errClose)
		}

		return nil, statusErr{code: httpResp.StatusCode, msg: string(b)}
	}

	// Success - caller is responsible for closing the response body
	return httpResp, nil
}

// ExecuteHTTPRequestForStreaming is similar to ExecuteHTTPRequest but specifically
// for streaming responses. The main difference is that it doesn't close the response
// body on success - the streaming code needs to read from it.
//
// On error, the response body is automatically closed.
//
// Parameters:
//   - ctx: The context for the request
//   - cfg: The application configuration
//   - auth: The authentication information
//   - httpReq: The prepared HTTP request
//   - logPrefix: Optional prefix for logging errors (e.g., "claude executor")
//
// Returns:
//   - *http.Response: The HTTP response if status is 2xx, nil on error
//   - error: An error with status code and error message if status is not 2xx
func ExecuteHTTPRequestForStreaming(
	ctx context.Context,
	cfg *config.Config,
	auth *cliproxyauth.Auth,
	httpReq *http.Request,
	logPrefix string,
) (*http.Response, error) {
	// Create proxy-aware HTTP client
	httpClient := newProxyAwareHTTPClient(ctx, cfg, auth, 0)

	// Execute request
	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		// Network/connection error
		recordAPIResponseError(ctx, cfg, err)
		return nil, err
	}

	// Record response metadata
	recordAPIResponseMetadata(ctx, cfg, httpResp.StatusCode, httpResp.Header.Clone())

	// Check status code
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		// Error status - read body and close
		data, readErr := io.ReadAll(httpResp.Body)

		if errClose := httpResp.Body.Close(); errClose != nil {
			log.Errorf("%s: close response body error: %v", logPrefix, errClose)
		}

		if readErr != nil {
			recordAPIResponseError(ctx, cfg, readErr)
			return nil, readErr
		}

		appendAPIResponseChunk(ctx, cfg, data)
		logWithRequestID(ctx).Debugf("request error, error status: %d, error message: %s",
			httpResp.StatusCode, summarizeErrorBody(httpResp.Header.Get("Content-Type"), data))

		return nil, statusErr{code: httpResp.StatusCode, msg: string(data)}
	}

	// Success - caller is responsible for closing the response body (streaming goroutine needs it)
	return httpResp, nil
}
