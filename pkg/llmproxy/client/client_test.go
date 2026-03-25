package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/client"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newTestServer(t *testing.T, handler http.Handler) (*httptest.Server, *client.Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := client.New(
		client.WithBaseURL(srv.URL),
		client.WithTimeout(5*time.Second),
	)
	return srv, c
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

func TestHealth_OK(t *testing.T) {
	_, c := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ok"})
	}))

	if err := c.Health(context.Background()); err != nil {
		t.Fatalf("Health() unexpected error: %v", err)
	}
}

func TestHealth_Unreachable(t *testing.T) {
	// Point at a port nothing is listening on.
	c := client.New(
		client.WithBaseURL("http://127.0.0.1:1"),
		client.WithTimeout(500*time.Millisecond),
	)
	if err := c.Health(context.Background()); err == nil {
		t.Fatal("Health() expected error for unreachable server, got nil")
	}
}

func TestHealth_ServerError(t *testing.T) {
	_, c := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 503, map[string]any{
			"error": map[string]any{"message": "service unavailable", "code": 503},
		})
	}))
	if err := c.Health(context.Background()); err == nil {
		t.Fatal("Health() expected error for 503, got nil")
	}
}

// ---------------------------------------------------------------------------
// ListModels
// ---------------------------------------------------------------------------

func TestListModels_ProxyShape(t *testing.T) {
	// cliproxyapi++ normalised shape: {"models": [...]}
	_, c := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("x-models-etag", "abc123")
		writeJSON(w, 200, map[string]any{
			"models": []map[string]any{
				{"id": "anthropic/claude-opus-4-6", "object": "model", "owned_by": "anthropic"},
				{"id": "openai/gpt-4o", "object": "model", "owned_by": "openai"},
			},
		})
	}))

	resp, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() unexpected error: %v", err)
	}
	if len(resp.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(resp.Models))
	}
	if resp.Models[0].ID != "anthropic/claude-opus-4-6" {
		t.Errorf("unexpected first model ID: %s", resp.Models[0].ID)
	}
}

func TestListModels_OpenAIShape(t *testing.T) {
	// Raw upstream OpenAI shape: {"data": [...], "object": "list"}
	_, c := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{
			"object": "list",
			"data": []map[string]any{
				{"id": "gpt-4o", "object": "model", "owned_by": "openai"},
			},
		})
	}))

	resp, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() unexpected error: %v", err)
	}
	if len(resp.Models) != 1 || resp.Models[0].ID != "gpt-4o" {
		t.Errorf("unexpected models: %+v", resp.Models)
	}
}

func TestListModels_Error(t *testing.T) {
	_, c := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 401, map[string]any{
			"error": map[string]any{"message": "unauthorized", "code": 401},
		})
	}))

	_, err := c.ListModels(context.Background())
	if err == nil {
		t.Fatal("ListModels() expected error for 401, got nil")
	}
	if _, ok := err.(*client.APIError); !ok {
		t.Logf("error type: %T — not an *client.APIError, that is acceptable", err)
	}
}

// ---------------------------------------------------------------------------
// ChatCompletion
// ---------------------------------------------------------------------------

func TestChatCompletion_OK(t *testing.T) {
	_, c := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		// Decode and validate request body
		var body client.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", 400)
			return
		}
		if body.Stream {
			http.Error(w, "client must not set stream=true", 400)
			return
		}
		writeJSON(w, 200, map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 1700000000,
			"model":   body.Model,
			"choices": []map[string]any{
				{
					"index":         0,
					"message":       map[string]any{"role": "assistant", "content": "Hello!"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15,
			},
		})
	}))

	resp, err := c.ChatCompletion(context.Background(), client.ChatCompletionRequest{
		Model: "anthropic/claude-opus-4-6",
		Messages: []client.ChatMessage{
			{Role: "user", Content: "Say hi"},
		},
	})
	if err != nil {
		t.Fatalf("ChatCompletion() unexpected error: %v", err)
	}
	if len(resp.Choices) == 0 {
		t.Fatal("expected at least one choice")
	}
	if resp.Choices[0].Message.Content != "Hello!" {
		t.Errorf("unexpected content: %q", resp.Choices[0].Message.Content)
	}
}

func TestChatCompletion_4xx(t *testing.T) {
	_, c := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 429, map[string]any{
			"error": map[string]any{"message": "rate limit exceeded", "code": 429},
		})
	}))

	_, err := c.ChatCompletion(context.Background(), client.ChatCompletionRequest{
		Model:    "any",
		Messages: []client.ChatMessage{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 429")
	}
}

// ---------------------------------------------------------------------------
// Responses
// ---------------------------------------------------------------------------

func TestResponses_OK(t *testing.T) {
	_, c := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/responses" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, 200, map[string]any{
			"id":     "resp_test",
			"object": "response",
			"output": []map[string]any{
				{"type": "message", "role": "assistant", "content": []map[string]any{
					{"type": "text", "text": "Hello from responses API"},
				}},
			},
		})
	}))

	out, err := c.Responses(context.Background(), client.ResponsesRequest{
		Model: "anthropic/claude-opus-4-6",
		Input: "Say hi",
	})
	if err != nil {
		t.Fatalf("Responses() unexpected error: %v", err)
	}
	if out["id"] != "resp_test" {
		t.Errorf("unexpected id: %v", out["id"])
	}
}

// ---------------------------------------------------------------------------
// Options
// ---------------------------------------------------------------------------

func TestWithAPIKey_SetsAuthorizationHeader(t *testing.T) {
	var gotAuth string
	_, c := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		writeJSON(w, 200, map[string]any{"models": []any{}})
	}))
	// Rebuild with API key
	_, c = newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		writeJSON(w, 200, map[string]any{"models": []any{}})
	}))
	_ = c // silence unused warning; we rebuild below

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		writeJSON(w, 200, map[string]any{"models": []any{}})
	}))
	t.Cleanup(srv.Close)

	c = client.New(
		client.WithBaseURL(srv.URL),
		client.WithAPIKey("sk-test-key"),
		client.WithTimeout(5*time.Second),
	)
	if _, err := c.ListModels(context.Background()); err != nil {
		t.Fatalf("ListModels() unexpected error: %v", err)
	}
	if gotAuth != "Bearer sk-test-key" {
		t.Errorf("expected 'Bearer sk-test-key', got %q", gotAuth)
	}
}

func TestBaseURL(t *testing.T) {
	c := client.New(client.WithBaseURL("http://localhost:9999"))
	if c.BaseURL() != "http://localhost:9999" {
		t.Errorf("BaseURL() = %q, want %q", c.BaseURL(), "http://localhost:9999")
	}
}

// ---------------------------------------------------------------------------
// Error type
// ---------------------------------------------------------------------------

func TestAPIError_Message(t *testing.T) {
	_, c := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 503, map[string]any{
			"error": map[string]any{
				"message": "service unavailable — no providers matched",
				"code":    503,
			},
		})
	}))

	_, err := c.ListModels(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*client.APIError)
	if !ok {
		t.Fatalf("expected *client.APIError, got %T", err)
	}
	if apiErr.StatusCode != 503 {
		t.Errorf("StatusCode = %d, want 503", apiErr.StatusCode)
	}
	if apiErr.Message == "" {
		t.Error("Message must not be empty")
	}
}

// ---------------------------------------------------------------------------
// Context cancellation
// ---------------------------------------------------------------------------

func TestContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until client cancels
		<-r.Context().Done()
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)

	c := client.New(client.WithBaseURL(srv.URL), client.WithTimeout(5*time.Second))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := c.Health(ctx); err == nil {
		t.Fatal("expected error due to context cancellation")
	}
}
