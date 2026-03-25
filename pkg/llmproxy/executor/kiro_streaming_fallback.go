package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"

	clipproxyexecutor "github.com/kooshapari/CLIProxyAPI/v7/sdk/cliproxy/executor"
	sdktranslator "github.com/kooshapari/CLIProxyAPI/v7/sdk/translator"
	log "github.com/sirupsen/logrus"
)

func (e *KiroExecutor) callKiroAndBuffer(
	ctx context.Context,
	auth *cliproxyauth.Auth,
	req cliproxyexecutor.Request,
	opts cliproxyexecutor.Options,
	accessToken, profileArn string,
) ([][]byte, error) {
	from := opts.SourceFormat
	to := sdktranslator.FromString("kiro")
	body := sdktranslator.TranslateRequest(from, to, req.Model, bytes.Clone(req.Payload), true)
	log.Debugf("kiro/websearch GAR request: %d bytes", len(body))

	kiroModelID := e.mapModelToKiro(req.Model)
	isAgentic, isChatOnly := determineAgenticMode(req.Model)
	effectiveProfileArn := getEffectiveProfileArnWithWarning(auth, profileArn)

	tokenKey := getTokenKey(auth)

	kiroStream, err := e.executeStreamWithRetry(
		ctx, auth, req, opts, accessToken, effectiveProfileArn,
		body, from, nil, kiroModelID, isAgentic, isChatOnly, tokenKey,
	)
	if err != nil {
		return nil, err
	}

	// Buffer all chunks
	var chunks [][]byte
	for chunk := range kiroStream {
		if chunk.Err != nil {
			return chunks, chunk.Err
		}
		if len(chunk.Payload) > 0 {
			chunks = append(chunks, bytes.Clone(chunk.Payload))
		}
	}

	log.Debugf("kiro/websearch GAR response: %d chunks buffered", len(chunks))

	return chunks, nil
}

// callKiroDirectStream creates a direct streaming channel to Kiro API without search.
func (e *KiroExecutor) callKiroDirectStream(
	ctx context.Context,
	auth *cliproxyauth.Auth,
	req cliproxyexecutor.Request,
	opts cliproxyexecutor.Options,
	accessToken, profileArn string,
) (<-chan cliproxyexecutor.StreamChunk, error) {
	from := opts.SourceFormat
	to := sdktranslator.FromString("kiro")
	body := sdktranslator.TranslateRequest(from, to, req.Model, bytes.Clone(req.Payload), true)

	kiroModelID := e.mapModelToKiro(req.Model)
	isAgentic, isChatOnly := determineAgenticMode(req.Model)
	effectiveProfileArn := getEffectiveProfileArnWithWarning(auth, profileArn)

	tokenKey := getTokenKey(auth)

	reporter := newUsageReporter(ctx, e.Identifier(), req.Model, auth)
	var streamErr error
	defer reporter.trackFailure(ctx, &streamErr)

	stream, streamErr := e.executeStreamWithRetry(
		ctx, auth, req, opts, accessToken, effectiveProfileArn,
		body, from, reporter, kiroModelID, isAgentic, isChatOnly, tokenKey,
	)
	return stream, streamErr
}

// sendFallbackText sends a simple text response when the Kiro API fails during the search loop.
// Delegates SSE event construction to kiroclaude.BuildFallbackTextEvents() for alignment
// with how streamToChannel() uses BuildClaude*Event() functions.
func (e *KiroExecutor) sendFallbackText(
	ctx context.Context,
	out chan<- cliproxyexecutor.StreamChunk,
	contentBlockIndex int,
	query string,
	searchResults *kiroclaude.WebSearchResults,
) {
	events := kiroclaude.BuildFallbackTextEvents(contentBlockIndex, query, searchResults)
	for _, event := range events {
		select {
		case <-ctx.Done():
			return
		case out <- cliproxyexecutor.StreamChunk{Payload: append(event, '\n', '\n')}:
		}
	}
}

// executeNonStreamFallback runs the standard non-streaming Execute path for a request.
// Used by handleWebSearch after injecting search results, or as a fallback.
func (e *KiroExecutor) executeNonStreamFallback(
	ctx context.Context,
	auth *cliproxyauth.Auth,
	req cliproxyexecutor.Request,
	opts cliproxyexecutor.Options,
	accessToken, profileArn string,
) (cliproxyexecutor.Response, error) {
	from := opts.SourceFormat
	to := sdktranslator.FromString("kiro")
	body := sdktranslator.TranslateRequest(from, to, req.Model, bytes.Clone(req.Payload), true)

	kiroModelID := e.mapModelToKiro(req.Model)
	isAgentic, isChatOnly := determineAgenticMode(req.Model)
	effectiveProfileArn := getEffectiveProfileArnWithWarning(auth, profileArn)
	tokenKey := getTokenKey(auth)

	reporter := newUsageReporter(ctx, e.Identifier(), req.Model, auth)
	var err error
	defer reporter.trackFailure(ctx, &err)

	resp, err := e.executeWithRetry(ctx, auth, req, opts, accessToken, effectiveProfileArn, body, from, to, reporter, kiroModelID, isAgentic, isChatOnly, tokenKey)
	return resp, err
}

func (e *KiroExecutor) CloseExecutionSession(sessionID string) {}
