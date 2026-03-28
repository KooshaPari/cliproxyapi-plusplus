package executor

import (
	"bufio"
	"bytes"
	"context"
	"net/http"

	cliproxyexecutor "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/cliproxy/executor"
	log "github.com/sirupsen/logrus"
)

// StreamLineProcessor is a callback function that processes a single line from the SSE stream.
// It receives the raw line bytes and returns any chunks to send to the client.
// If an error occurs, return an error; returning nil and false indicates success.
type StreamLineProcessor func(ctx context.Context, line []byte) ([]string, error)

// StreamErrorHandler is a callback for handling stream errors.
// It receives the scanner error and can perform cleanup/logging.
type StreamErrorHandler func(ctx context.Context, err error)

// ProcessSSEStream reads an SSE stream from an HTTP response and processes each line.
// It handles the boilerplate of:
// - Creating a channel for stream chunks
// - Spawning a goroutine to read lines
// - Calling a processor function for each line
// - Handling scanner errors
// - Ensuring proper cleanup
//
// The processor callback receives each non-empty line and should return
// a slice of strings to send as StreamChunk payloads. The provider is responsible
// for any SSE parsing (e.g., removing "data: " prefix) if needed.
//
// The optional errorHandler is called if the scanner encounters an error.
// If not provided, the error is logged and sent as a StreamChunk.Err.
func ProcessSSEStream(
	ctx context.Context,
	resp *http.Response,
	processor StreamLineProcessor,
	errorHandler StreamErrorHandler,
) *cliproxyexecutor.StreamResult {
	out := make(chan cliproxyexecutor.StreamChunk)

	go func() {
		defer close(out)
		defer func() {
			if errClose := resp.Body.Close(); errClose != nil {
				log.Errorf("response body close error: %v", errClose)
			}
		}()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(nil, 52_428_800) // 50MB buffer

		for scanner.Scan() {
			line := scanner.Bytes()

			// Call provider-specific processor
			chunks, err := processor(ctx, line)
			if err != nil {
				if errorHandler != nil {
					errorHandler(ctx, err)
				}
				out <- cliproxyexecutor.StreamChunk{Err: err}
				continue
			}

			// Send all chunks from processor
			for _, chunk := range chunks {
				out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk)}
			}
		}

		// Handle scanner error
		if errScan := scanner.Err(); errScan != nil {
			if errorHandler != nil {
				errorHandler(ctx, errScan)
			}
			out <- cliproxyexecutor.StreamChunk{Err: errScan}
		}
	}()

	return &cliproxyexecutor.StreamResult{
		Headers: resp.Header.Clone(),
		Chunks:  out,
	}
}

// ProcessSSEStreamWithFilter reads an SSE stream and filters lines before processing.
// It skips:
// - Empty lines
// - Lines that don't start with "data: " (if requireDataPrefix is true)
//
// This variant is useful for providers that emit mixed content or empty lines.
func ProcessSSEStreamWithFilter(
	ctx context.Context,
	resp *http.Response,
	processor StreamLineProcessor,
	errorHandler StreamErrorHandler,
	requireDataPrefix bool,
) *cliproxyexecutor.StreamResult {
	filteredProcessor := func(ctx context.Context, line []byte) ([]string, error) {
		// Skip empty lines
		if len(line) == 0 {
			return nil, nil
		}

		// Skip lines without "data: " prefix if required
		if requireDataPrefix && !bytes.HasPrefix(line, []byte("data:")) {
			return nil, nil
		}

		// Call original processor
		return processor(ctx, line)
	}

	return ProcessSSEStream(ctx, resp, filteredProcessor, errorHandler)
}

// LoggingErrorHandler returns an error handler that logs and publishes failures.
// Useful for providers that need to track usage on error.
func LoggingErrorHandler(ctx context.Context, err error) {
	log.Errorf("stream error: %v", err)
	recordAPIResponseError(ctx, nil, err)
}

// SimpleStreamProcessor wraps a line processor that doesn't need context.
// Useful when the processor is simple and doesn't need the full ctx/response context.
func SimpleStreamProcessor(
	processor func(line []byte) ([]string, error),
) StreamLineProcessor {
	return func(ctx context.Context, line []byte) ([]string, error) {
		return processor(line)
	}
}

// ChainProcessors combines multiple processors, running them in sequence.
// Each processor's output is passed to the next, and all chunks are collected.
// If any processor returns an error, the chain stops and returns that error.
func ChainProcessors(processors ...StreamLineProcessor) StreamLineProcessor {
	return func(ctx context.Context, line []byte) ([]string, error) {
		var allChunks []string

		for _, p := range processors {
			chunks, err := p(ctx, line)
			if err != nil {
				return nil, err
			}
			allChunks = append(allChunks, chunks...)
		}

		return allChunks, nil
	}
}
