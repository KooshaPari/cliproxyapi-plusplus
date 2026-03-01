// Package openai provides HTTP handlers for OpenAIResponses API endpoints.
// This package implements the OpenAIResponses-compatible API interface, including model listing
// and chat completion functionality. It supports both streaming and non-streaming responses,
// and manages a pool of clients to interact with backend services.
// The handlers translate OpenAIResponses API requests to the appropriate backend format and
// convert responses back to OpenAIResponses-compatible format.
package openai

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	. "github.com/router-for-me/CLIProxyAPI/v6/pkg/llmproxy/constant"
	"github.com/router-for-me/CLIProxyAPI/v6/pkg/llmproxy/interfaces"
	"github.com/router-for-me/CLIProxyAPI/v6/pkg/llmproxy/registry"
	responsesconverter "github.com/router-for-me/CLIProxyAPI/v6/pkg/llmproxy/translator/openai/openai/responses"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/api/handlers"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// OpenAIResponsesAPIHandler contains the handlers for OpenAIResponses API endpoints.
// It holds a pool of clients to interact with the backend service.
type OpenAIResponsesAPIHandler struct {
	*handlers.BaseAPIHandler
}

// NewOpenAIResponsesAPIHandler creates a new OpenAIResponses API handlers instance.
// It takes an BaseAPIHandler instance as input and returns an OpenAIResponsesAPIHandler.
//
// Parameters:
//   - apiHandlers: The base API handlers instance
//
// Returns:
//   - *OpenAIResponsesAPIHandler: A new OpenAIResponses API handlers instance
func NewOpenAIResponsesAPIHandler(apiHandlers *handlers.BaseAPIHandler) *OpenAIResponsesAPIHandler {
	return &OpenAIResponsesAPIHandler{
		BaseAPIHandler: apiHandlers,
	}
}

// HandlerType returns the identifier for this handler implementation.
func (h *OpenAIResponsesAPIHandler) HandlerType() string {
	return OpenaiResponse
}

// Models returns the OpenAIResponses-compatible model metadata supported by this handler.
func (h *OpenAIResponsesAPIHandler) Models() []map[string]any {
	// Get dynamic models from the global registry
	modelRegistry := registry.GetGlobalRegistry()
	return modelRegistry.GetAvailableModels("openai")
}

// OpenAIResponsesModels handles the /v1/models endpoint.
// It returns a list of available AI models with their capabilities
// and specifications in OpenAIResponses-compatible format.
func (h *OpenAIResponsesAPIHandler) OpenAIResponsesModels(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   h.Models(),
	})
}

// Responses handles the /v1/responses endpoint.
// It determines whether the request is for a streaming or non-streaming response
// and calls the appropriate handler based on the model provider.
//
// Parameters:
//   - c: The Gin context containing the HTTP request and response
func (h *OpenAIResponsesAPIHandler) Responses(c *gin.Context) {
	rawJSON, err := c.GetRawData()
	// If data retrieval fails, return a 400 Bad Request error.
	if err != nil {
		c.JSON(http.StatusBadRequest, handlers.ErrorResponse{
			Error: handlers.ErrorDetail{
				Message: fmt.Sprintf("Invalid request: %v", err),
				Type:    "invalid_request_error",
			},
		})
		return
	}

	// Check if the client requested a streaming response.
	streamResult := gjson.GetBytes(rawJSON, "stream")
	stream := streamResult.Type == gjson.True

	modelName := gjson.GetBytes(rawJSON, "model").String()
	if strings.Contains(strings.ToLower(strings.TrimSpace(modelName)), "minimax") {
		chatJSON := responsesconverter.ConvertOpenAIResponsesRequestToOpenAIChatCompletions(modelName, rawJSON, stream)
		if stream {
			h.handleStreamingResponseViaChat(c, rawJSON, chatJSON)
		} else {
			h.handleNonStreamingResponseViaChat(c, rawJSON, chatJSON)
		}
		return
	}
	if overrideEndpoint, ok := resolveEndpointOverride(modelName, openAIResponsesEndpoint); ok && overrideEndpoint == openAIChatEndpoint {
		chatJSON := responsesconverter.ConvertOpenAIResponsesRequestToOpenAIChatCompletions(modelName, rawJSON, stream)
		stream = gjson.GetBytes(chatJSON, "stream").Bool()
		if stream {
			h.handleStreamingResponseViaChat(c, rawJSON, chatJSON)
		} else {
			h.handleNonStreamingResponseViaChat(c, rawJSON, chatJSON)
		}
		return
	}

	if stream {
		h.handleStreamingResponse(c, rawJSON)
	} else {
		h.handleNonStreamingResponse(c, rawJSON)
	}

}

func (h *OpenAIResponsesAPIHandler) Compact(c *gin.Context) {
	rawJSON, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, handlers.ErrorResponse{
			Error: handlers.ErrorDetail{
				Message: fmt.Sprintf("Invalid request: %v", err),
				Type:    "invalid_request_error",
			},
		})
		return
	}

	streamResult := gjson.GetBytes(rawJSON, "stream")
	if streamResult.Type == gjson.True {
		c.JSON(http.StatusBadRequest, handlers.ErrorResponse{
			Error: handlers.ErrorDetail{
				Message: "Streaming not supported for compact responses",
				Type:    "invalid_request_error",
			},
		})
		return
	}
	if streamResult.Exists() {
		if updated, err := sjson.DeleteBytes(rawJSON, "stream"); err == nil {
			rawJSON = updated
		}
	}

	c.Header("Content-Type", "application/json")
	modelName := gjson.GetBytes(rawJSON, "model").String()
	cliCtx, cliCancel := h.GetContextWithCancel(h, c, context.Background())
	stopKeepAlive := h.StartNonStreamingKeepAlive(c, cliCtx)
	resp, upstreamHeaders, errMsg := h.ExecuteWithAuthManager(cliCtx, h.HandlerType(), modelName, rawJSON, "responses/compact")
	stopKeepAlive()
	if errMsg != nil {
		h.WriteErrorResponse(c, errMsg)
		cliCancel(errMsg.Error)
		return
	}
	handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)
	_, _ = c.Writer.Write(resp)
	cliCancel()
}

// handleNonStreamingResponse handles non-streaming chat completion responses
// for Gemini models. It selects a client from the pool, sends the request, and
// aggregates the response before sending it back to the client in OpenAIResponses format.
//
// Parameters:
//   - c: The Gin context containing the HTTP request and response
//   - rawJSON: The raw JSON bytes of the OpenAIResponses-compatible request
func (h *OpenAIResponsesAPIHandler) handleNonStreamingResponse(c *gin.Context, rawJSON []byte) {
	c.Header("Content-Type", "application/json")

	modelName := gjson.GetBytes(rawJSON, "model").String()
	cliCtx, cliCancel := h.GetContextWithCancel(h, c, context.Background())
	stopKeepAlive := h.StartNonStreamingKeepAlive(c, cliCtx)

	resp, upstreamHeaders, errMsg := h.ExecuteWithAuthManager(cliCtx, h.HandlerType(), modelName, rawJSON, "")
	stopKeepAlive()
	if errMsg != nil {
		h.WriteErrorResponse(c, errMsg)
		cliCancel(errMsg.Error)
		return
	}
	handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)
	_, _ = c.Writer.Write(resp)
	cliCancel()
}

func (h *OpenAIResponsesAPIHandler) handleNonStreamingResponseViaChat(c *gin.Context, originalResponsesJSON, chatJSON []byte) {
	c.Header("Content-Type", "application/json")

	modelName := gjson.GetBytes(chatJSON, "model").String()
	cliCtx, cliCancel := h.GetContextWithCancel(h, c, context.Background())
	resp, upstreamHeaders, errMsg := h.ExecuteWithAuthManager(cliCtx, OpenAI, modelName, chatJSON, "")
	if errMsg != nil {
		h.WriteErrorResponse(c, errMsg)
		cliCancel(errMsg.Error)
		return
	}
	if providerErr := extractCompatProviderError(resp); providerErr != nil {
		h.WriteErrorResponse(c, providerErr)
		cliCancel(providerErr.Error)
		return
	}
	handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)
	var param any
	converted := responsesconverter.ConvertOpenAIChatCompletionsResponseToOpenAIResponsesNonStream(cliCtx, modelName, originalResponsesJSON, originalResponsesJSON, resp, &param)
	if converted == "" {
		h.WriteErrorResponse(c, &interfaces.ErrorMessage{
			StatusCode: http.StatusInternalServerError,
			Error:      fmt.Errorf("failed to convert chat completion response to responses format"),
		})
		cliCancel(fmt.Errorf("response conversion failed"))
		return
	}
	_, _ = c.Writer.Write([]byte(converted))
	cliCancel()
}

// handleStreamingResponse handles streaming responses for Gemini models.
// It establishes a streaming connection with the backend service and forwards
// the response chunks to the client in real-time using Server-Sent Events.
//
// Parameters:
//   - c: The Gin context containing the HTTP request and response
//   - rawJSON: The raw JSON bytes of the OpenAIResponses-compatible request
func (h *OpenAIResponsesAPIHandler) handleStreamingResponse(c *gin.Context, rawJSON []byte) {
	// Get the http.Flusher interface to manually flush the response.
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, handlers.ErrorResponse{
			Error: handlers.ErrorDetail{
				Message: "Streaming not supported",
				Type:    "server_error",
			},
		})
		return
	}

	// New core execution path
	modelName := gjson.GetBytes(rawJSON, "model").String()
	cliCtx, cliCancel := h.GetContextWithCancel(h, c, context.Background())
	dataChan, upstreamHeaders, errChan := h.ExecuteStreamWithAuthManager(cliCtx, h.HandlerType(), modelName, rawJSON, "")

	setSSEHeaders := func() {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")
	}

	// Peek at the first chunk
	for {
		select {
		case <-c.Request.Context().Done():
			cliCancel(c.Request.Context().Err())
			return
		case errMsg, ok := <-errChan:
			if !ok {
				// Err channel closed cleanly; wait for data channel.
				errChan = nil
				continue
			}
			// Upstream failed immediately. Return proper error status and JSON.
			h.WriteErrorResponse(c, errMsg)
			if errMsg != nil {
				cliCancel(errMsg.Error)
			} else {
				cliCancel(nil)
			}
			return
		case chunk, ok := <-dataChan:
			if !ok {
				// Stream closed without data? Send headers and done.
				setSSEHeaders()
				handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)
				_, _ = c.Writer.Write([]byte("\n"))
				flusher.Flush()
				cliCancel(nil)
				return
			}

			// Success! Set headers.
			setSSEHeaders()
			handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)

			// Write first chunk logic (matching forwardResponsesStream)
			if bytes.HasPrefix(chunk, []byte("event:")) {
				_, _ = c.Writer.Write([]byte("\n"))
			}
			_, _ = c.Writer.Write(chunk)
			_, _ = c.Writer.Write([]byte("\n"))
			flusher.Flush()

			// Continue
			h.forwardResponsesStream(c, flusher, func(err error) { cliCancel(err) }, dataChan, errChan)
			return
		}
	}
}

func (h *OpenAIResponsesAPIHandler) handleStreamingResponseViaChat(c *gin.Context, originalResponsesJSON, chatJSON []byte) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, handlers.ErrorResponse{
			Error: handlers.ErrorDetail{
				Message: "Streaming not supported",
				Type:    "server_error",
			},
		})
		return
	}

	modelName := gjson.GetBytes(chatJSON, "model").String()
	if strings.Contains(strings.ToLower(strings.TrimSpace(modelName)), "minimax") {
		if updated, err := sjson.SetBytes(chatJSON, "stream", false); err == nil {
			chatJSON = updated
		}
		cliCtx, cliCancel := h.GetContextWithCancel(h, c, context.Background())
		resp, upstreamHeaders, errMsg := h.ExecuteWithAuthManager(cliCtx, OpenAI, modelName, chatJSON, "")
		if errMsg != nil {
			h.WriteErrorResponse(c, errMsg)
			cliCancel(errMsg.Error)
			return
		}
		if providerErr := extractCompatProviderError(resp); providerErr != nil {
			h.WriteErrorResponse(c, providerErr)
			cliCancel(providerErr.Error)
			return
		}
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)

		var param any
		converted := responsesconverter.ConvertOpenAIChatCompletionsResponseToOpenAIResponsesNonStream(
			cliCtx,
			modelName,
			originalResponsesJSON,
			originalResponsesJSON,
			resp,
			&param,
		)
		if converted == "" {
			h.WriteErrorResponse(c, &interfaces.ErrorMessage{
				StatusCode: http.StatusInternalServerError,
				Error:      fmt.Errorf("failed to convert chat completion response to responses format"),
			})
			cliCancel(fmt.Errorf("response conversion failed"))
			return
		}

		wrapped := wrapResponsesPayloadAsCompleted([]byte(converted))
		if len(wrapped) > 0 {
			writeSyntheticResponsesStreamFromCompleted(c, wrapped)
		}
		_, _ = fmt.Fprint(c.Writer, "data: [DONE]\n\n")
		flusher.Flush()
		cliCancel()
		return
	}

	cliCtx, cliCancel := h.GetContextWithCancel(h, c, context.Background())
	dataChan, upstreamHeaders, errChan := h.ExecuteStreamWithAuthManager(cliCtx, OpenAI, modelName, chatJSON, "")
	var param any

	setSSEHeaders := func() {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")
	}

	for {
		select {
		case <-c.Request.Context().Done():
			cliCancel(c.Request.Context().Err())
			return
		case errMsg, ok := <-errChan:
			if !ok {
				errChan = nil
				continue
			}
			h.WriteErrorResponse(c, errMsg)
			if errMsg != nil {
				cliCancel(errMsg.Error)
			} else {
				cliCancel(nil)
			}
			return
		case chunk, ok := <-dataChan:
			if !ok {
				setSSEHeaders()
				handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)
				_, _ = c.Writer.Write([]byte("\n"))
				flusher.Flush()
				cliCancel(nil)
				return
			}

			setSSEHeaders()
			handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)
			providerErr := writeChatAsResponsesChunk(c, cliCtx, modelName, originalResponsesJSON, chunk, &param)
			flusher.Flush()
			if providerErr {
				cliCancel(fmt.Errorf("provider error chunk"))
				return
			}

			h.forwardChatAsResponsesStream(c, flusher, func(err error) { cliCancel(err) }, dataChan, errChan, cliCtx, modelName, originalResponsesJSON, &param)
			return
		}
	}
}

func writeChatAsResponsesChunk(c *gin.Context, ctx context.Context, modelName string, originalResponsesJSON, chunk []byte, param *any) bool {
	if providerErr := extractCompatProviderErrorFromSSEChunk(chunk); providerErr != nil {
		status := providerErr.StatusCode
		if status <= 0 {
			status = http.StatusBadGateway
		}
		errText := http.StatusText(status)
		if providerErr.Error != nil && providerErr.Error.Error() != "" {
			errText = providerErr.Error.Error()
		}
		body := handlers.BuildErrorResponseBody(status, errText)
		_, _ = fmt.Fprintf(c.Writer, "\nevent: error\ndata: %s\n\n", string(body))
		return true
	}
	outputs := responsesconverter.ConvertOpenAIChatCompletionsResponseToOpenAIResponses(ctx, modelName, originalResponsesJSON, originalResponsesJSON, chunk, param)
	for _, out := range outputs {
		if out == "" {
			continue
		}
		if bytes.HasPrefix([]byte(out), []byte("event:")) {
			_, _ = c.Writer.Write([]byte("\n"))
		}
		_, _ = c.Writer.Write([]byte(out))
		_, _ = c.Writer.Write([]byte("\n"))
	}
	return false
}

func (h *OpenAIResponsesAPIHandler) forwardChatAsResponsesStream(c *gin.Context, flusher http.Flusher, cancel func(error), data <-chan []byte, errs <-chan *interfaces.ErrorMessage, ctx context.Context, modelName string, originalResponsesJSON []byte, param *any) {
	h.ForwardStream(c, flusher, cancel, data, errs, handlers.StreamForwardOptions{
		WriteChunk: func(chunk []byte) {
			if providerErr := extractCompatProviderErrorFromSSEChunk(chunk); providerErr != nil {
				status := providerErr.StatusCode
				if status <= 0 {
					status = http.StatusBadGateway
				}
				errText := http.StatusText(status)
				if providerErr.Error != nil && providerErr.Error.Error() != "" {
					errText = providerErr.Error.Error()
				}
				body := handlers.BuildErrorResponseBody(status, errText)
				_, _ = fmt.Fprintf(c.Writer, "\nevent: error\ndata: %s\n\n", string(body))
				return
			}
			outputs := responsesconverter.ConvertOpenAIChatCompletionsResponseToOpenAIResponses(ctx, modelName, originalResponsesJSON, originalResponsesJSON, chunk, param)
			for _, out := range outputs {
				if out == "" {
					continue
				}
				if bytes.HasPrefix([]byte(out), []byte("event:")) {
					_, _ = c.Writer.Write([]byte("\n"))
				}
				_, _ = c.Writer.Write([]byte(out))
				_, _ = c.Writer.Write([]byte("\n"))
			}
		},
		WriteTerminalError: func(errMsg *interfaces.ErrorMessage) {
			if errMsg == nil {
				return
			}
			status := http.StatusInternalServerError
			if errMsg.StatusCode > 0 {
				status = errMsg.StatusCode
			}
			errText := http.StatusText(status)
			if errMsg.Error != nil && errMsg.Error.Error() != "" {
				errText = errMsg.Error.Error()
			}
			body := handlers.BuildErrorResponseBody(status, errText)
			_, _ = fmt.Fprintf(c.Writer, "\nevent: error\ndata: %s\n\n", string(body))
		},
		WriteDone: func() {
			_, _ = c.Writer.Write([]byte("\n"))
		},
	})
}

func extractCompatProviderError(payload []byte) *interfaces.ErrorMessage {
	if len(bytes.TrimSpace(payload)) == 0 {
		return nil
	}

	errorNode := gjson.GetBytes(payload, "error")
	if errorNode.Exists() {
		message := strings.TrimSpace(errorNode.Get("message").String())
		if message == "" {
			message = strings.TrimSpace(errorNode.String())
		}
		if message == "" {
			message = "upstream provider error"
		}
		statusCode := int(errorNode.Get("status").Int())
		if statusCode <= 0 {
			statusCode = http.StatusBadGateway
		}
		statusCode = normalizeProviderStatus(statusCode, message)
		return &interfaces.ErrorMessage{StatusCode: statusCode, Error: fmt.Errorf("%s", message)}
	}

	msg := strings.TrimSpace(gjson.GetBytes(payload, "msg").String())
	statusRaw := strings.TrimSpace(gjson.GetBytes(payload, "status").String())
	if msg == "" || statusRaw == "" {
		return nil
	}
	statusCode, err := strconv.Atoi(statusRaw)
	if err != nil || statusCode < 400 || statusCode > 599 {
		statusCode = http.StatusBadGateway
	}
	statusCode = normalizeProviderStatus(statusCode, msg)
	return &interfaces.ErrorMessage{StatusCode: statusCode, Error: fmt.Errorf("%s", msg)}
}

func normalizeProviderStatus(statusCode int, message string) int {
	msg := strings.ToLower(strings.TrimSpace(message))
	if statusCode >= 100 && statusCode <= 599 && http.StatusText(statusCode) != "" {
		return statusCode
	}
	switch {
	case strings.Contains(msg, "invalid api key"),
		strings.Contains(msg, "unauthorized"),
		strings.Contains(msg, "authentication"),
		strings.Contains(msg, "auth failed"):
		return http.StatusUnauthorized
	case strings.Contains(msg, "forbidden"),
		strings.Contains(msg, "blocked"),
		strings.Contains(msg, "denied"):
		return http.StatusForbidden
	case strings.Contains(msg, "rate limit"),
		strings.Contains(msg, "too many requests"),
		strings.Contains(msg, "quota"):
		return http.StatusTooManyRequests
	default:
		return http.StatusBadGateway
	}
}

func extractCompatProviderErrorFromSSEChunk(chunk []byte) *interfaces.ErrorMessage {
	line := bytes.TrimSpace(chunk)
	if !bytes.HasPrefix(line, []byte("data:")) {
		return nil
	}
	payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
	if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
		return nil
	}
	return extractCompatProviderError(payload)
}

func (h *OpenAIResponsesAPIHandler) forwardResponsesStream(c *gin.Context, flusher http.Flusher, cancel func(error), data <-chan []byte, errs <-chan *interfaces.ErrorMessage) {
	h.ForwardStream(c, flusher, cancel, data, errs, handlers.StreamForwardOptions{
		WriteChunk: func(chunk []byte) {
			if writeSyntheticResponsesStreamFromChunk(c, chunk) {
				return
			}
			if bytes.HasPrefix(chunk, []byte("event:")) {
				_, _ = c.Writer.Write([]byte("\n"))
			}
			_, _ = c.Writer.Write(chunk)
			_, _ = c.Writer.Write([]byte("\n"))
		},
		WriteTerminalError: func(errMsg *interfaces.ErrorMessage) {
			if errMsg == nil {
				return
			}
			status := http.StatusInternalServerError
			if errMsg.StatusCode > 0 {
				status = errMsg.StatusCode
			}
			errText := http.StatusText(status)
			if errMsg.Error != nil && errMsg.Error.Error() != "" {
				errText = errMsg.Error.Error()
			}
			chunk := handlers.BuildOpenAIResponsesStreamErrorChunk(status, errText, 0)
			_, _ = fmt.Fprintf(c.Writer, "\nevent: error\ndata: %s\n\n", string(chunk))
		},
		WriteDone: func() {
			_, _ = c.Writer.Write([]byte("\n"))
		},
	})
}

func writeSyntheticResponsesStreamFromChunk(c *gin.Context, chunk []byte) bool {
	payloads := websocketJSONPayloadsFromChunk(chunk)
	if len(payloads) != 1 {
		return false
	}
	return writeSyntheticResponsesStreamFromCompleted(c, payloads[0])
}

func writeSyntheticResponsesStreamFromCompleted(c *gin.Context, payload []byte) bool {
	completed := wrapResponsesPayloadAsCompleted(payload)
	if len(completed) == 0 {
		return false
	}
	if gjson.GetBytes(completed, "type").String() != "response.completed" {
		return false
	}

	text := extractResponsesOutputText(completed)
	if strings.TrimSpace(text) == "" {
		_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", string(completed))
		return true
	}

	responseID := strings.TrimSpace(gjson.GetBytes(completed, "response.id").String())
	if responseID == "" {
		responseID = strings.TrimSpace(gjson.GetBytes(completed, "response.response_id").String())
	}
	itemID := strings.TrimSpace(gjson.GetBytes(completed, "response.output.0.id").String())
	if itemID == "" && responseID != "" {
		itemID = "msg_" + responseID + "_0"
	}

	created := `{"type":"response.created","sequence_number":0,"response":{"id":"","object":"response","created_at":0,"status":"in_progress","background":false,"error":null,"output":[]}}`
	if responseID != "" {
		created, _ = sjson.Set(created, "response.id", responseID)
	}
	if model := strings.TrimSpace(gjson.GetBytes(completed, "response.model").String()); model != "" {
		created, _ = sjson.Set(created, "response.model", model)
	}
	if createdAt := gjson.GetBytes(completed, "response.created_at"); createdAt.Exists() {
		created, _ = sjson.Set(created, "response.created_at", createdAt.Int())
	}
	_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", created)

	itemAdded := `{"type":"response.output_item.added","sequence_number":1,"output_index":0,"item":{"id":"","type":"message","status":"in_progress","content":[],"role":"assistant"}}`
	if itemID != "" {
		itemAdded, _ = sjson.Set(itemAdded, "item.id", itemID)
	}
	_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", itemAdded)

	contentPartAdded := `{"type":"response.content_part.added","sequence_number":2,"item_id":"","output_index":0,"content_index":0,"part":{"type":"output_text","annotations":[],"logprobs":[],"text":""}}`
	if itemID != "" {
		contentPartAdded, _ = sjson.Set(contentPartAdded, "item_id", itemID)
	}
	_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", contentPartAdded)

	delta := `{"type":"response.output_text.delta","sequence_number":3,"item_id":"","output_index":0,"content_index":0,"delta":"","logprobs":[]}`
	if itemID != "" {
		delta, _ = sjson.Set(delta, "item_id", itemID)
	}
	delta, _ = sjson.Set(delta, "delta", text)
	_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", delta)

	done := `{"type":"response.output_text.done","sequence_number":4,"item_id":"","output_index":0,"content_index":0,"text":"","logprobs":[]}`
	if itemID != "" {
		done, _ = sjson.Set(done, "item_id", itemID)
	}
	done, _ = sjson.Set(done, "text", text)
	_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", done)

	contentPartDone := `{"type":"response.content_part.done","sequence_number":5,"item_id":"","output_index":0,"content_index":0,"part":{"type":"output_text","annotations":[],"logprobs":[],"text":""}}`
	if itemID != "" {
		contentPartDone, _ = sjson.Set(contentPartDone, "item_id", itemID)
	}
	contentPartDone, _ = sjson.Set(contentPartDone, "part.text", text)
	_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", contentPartDone)

	itemDone := `{"type":"response.output_item.done","sequence_number":6,"output_index":0,"item":{"id":"","type":"message","status":"completed","content":[{"type":"output_text","annotations":[],"logprobs":[],"text":""}],"role":"assistant"}}`
	if itemID != "" {
		itemDone, _ = sjson.Set(itemDone, "item.id", itemID)
	}
	itemDone, _ = sjson.Set(itemDone, "item.content.0.text", text)
	_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", itemDone)

	_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", string(completed))
	return true
}

func extractResponsesOutputText(completed []byte) string {
	if txt := strings.TrimSpace(gjson.GetBytes(completed, "response.output_text").String()); txt != "" {
		return txt
	}
	parts := make([]string, 0, 2)
	for _, item := range gjson.GetBytes(completed, "response.output").Array() {
		for _, content := range item.Get("content").Array() {
			contentType := strings.TrimSpace(content.Get("type").String())
			if contentType != "output_text" && contentType != "text" {
				continue
			}
			text := strings.TrimSpace(content.Get("text").String())
			if text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n")
}
