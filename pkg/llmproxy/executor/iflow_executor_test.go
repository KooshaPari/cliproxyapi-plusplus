package executor

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/thinking"
	cliproxyauth "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/cliproxy/executor"
	sdktranslator "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/translator"
)

func TestIFlowExecutorParseSuffix(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		wantBase  string
		wantLevel string
	}{
		{"no suffix", "glm-4", "glm-4", ""},
		{"glm with suffix", "glm-4.1-flash(high)", "glm-4.1-flash", "high"},
		{"minimax no suffix", "minimax-m2", "minimax-m2", ""},
		{"minimax with suffix", "minimax-m2.1(medium)", "minimax-m2.1", "medium"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := thinking.ParseSuffix(tt.model)
			if result.ModelName != tt.wantBase {
				t.Errorf("ParseSuffix(%q).ModelName = %q, want %q", tt.model, result.ModelName, tt.wantBase)
			}
		})
	}
}

func TestClassifyIFlowRefreshError(t *testing.T) {
	t.Run("maps server busy to 503", func(t *testing.T) {
		err := classifyIFlowRefreshError(errors.New("iflow token: provider rejected token request (code=500 message=server busy)"))
		se, ok := err.(interface{ StatusCode() int })
		if !ok {
			t.Fatalf("expected status error type, got %T", err)
		}
		if got := se.StatusCode(); got != http.StatusServiceUnavailable {
			t.Fatalf("status code = %d, want %d", got, http.StatusServiceUnavailable)
		}
	})

	t.Run("non server busy unchanged", func(t *testing.T) {
		in := errors.New("iflow token: provider rejected token request (code=400 message=invalid_grant)")
		out := classifyIFlowRefreshError(in)
		if !errors.Is(out, in) {
			t.Fatalf("expected original error to be preserved")
		}
	})

	t.Run("maps provider 429 to 429", func(t *testing.T) {
		err := classifyIFlowRefreshError(errors.New("iflow token: provider rejected token request (code=429 message=rate limit exceeded)"))
		se, ok := err.(interface{ StatusCode() int })
		if !ok {
			t.Fatalf("expected status error type, got %T", err)
		}
		if got := se.StatusCode(); got != http.StatusTooManyRequests {
			t.Fatalf("status code = %d, want %d", got, http.StatusTooManyRequests)
		}
	})

	t.Run("maps provider 503 to 503", func(t *testing.T) {
		err := classifyIFlowRefreshError(errors.New("iflow token: provider rejected token request (code=503 message=service unavailable)"))
		se, ok := err.(interface{ StatusCode() int })
		if !ok {
			t.Fatalf("expected status error type, got %T", err)
		}
		if got := se.StatusCode(); got != http.StatusServiceUnavailable {
			t.Fatalf("status code = %d, want %d", got, http.StatusServiceUnavailable)
		}
	})
}

func TestDetectIFlowProviderError(t *testing.T) {
	t.Run("ignores normal chat completion payload", func(t *testing.T) {
		err := detectIFlowProviderError([]byte(`{"id":"chatcmpl_1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"}}]}`))
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("captures embedded token expiry envelope", func(t *testing.T) {
		err := detectIFlowProviderError([]byte(`{"status":"439","msg":"Your API Token has expired.","body":null}`))
		if err == nil {
			t.Fatal("expected provider error")
		}
		if !err.Refreshable {
			t.Fatal("expected provider error to be refreshable")
		}
		if got := err.StatusCode(); got != http.StatusUnauthorized {
			t.Fatalf("status code = %d, want %d", got, http.StatusUnauthorized)
		}
	})
}

func TestPreserveReasoningContentInMessages(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  []byte // nil means output should equal input
	}{
		{
			"non-glm model passthrough",
			[]byte(`{"model":"gpt-4","messages":[]}`),
			nil,
		},
		{
			"glm model with empty messages",
			[]byte(`{"model":"glm-4","messages":[]}`),
			nil,
		},
		{
			"glm model preserves existing reasoning_content",
			[]byte(`{"model":"glm-4","messages":[{"role":"assistant","content":"hi","reasoning_content":"thinking..."}]}`),
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := preserveReasoningContentInMessages(tt.input)
			want := tt.want
			if want == nil {
				want = tt.input
			}
			if string(got) != string(want) {
				t.Errorf("preserveReasoningContentInMessages() = %s, want %s", got, want)
			}
		})
	}
}

func TestIFlowExecutorExecuteStreamFallsBackFrom406ForResponsesClients(t *testing.T) {
	requestCount := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		body, _ := io.ReadAll(r.Body)

		switch requestCount {
		case 1:
			if got := r.Header.Get("Accept"); got != "text/event-stream" {
				t.Fatalf("expected stream Accept header, got %q", got)
			}
			if !strings.Contains(string(body), `"stream":true`) {
				t.Fatalf("expected initial stream request, got %s", body)
			}
			http.Error(w, "status 406", http.StatusNotAcceptable)
		case 2:
			if strings.Contains(string(body), `"stream":true`) {
				t.Fatalf("expected fallback request to disable stream, got %s", body)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"chatcmpl_iflow","object":"chat.completion","created":1735689600,"model":"minimax-m2.5","choices":[{"index":0,"message":{"role":"assistant","content":"hi from iflow fallback"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`))
		default:
			t.Fatalf("unexpected upstream call %d", requestCount)
		}
	}))
	defer upstream.Close()

	executor := NewIFlowExecutor(nil)
	auth := &cliproxyauth.Auth{Attributes: map[string]string{
		"base_url": upstream.URL,
		"api_key":  "iflow-test",
	}}
	originalRequest := []byte(`{"model":"minimax-m2.5","stream":true,"input":[{"role":"user","content":"hi"}]}`)
	streamResult, err := executor.ExecuteStream(context.Background(), auth, cliproxyexecutor.Request{
		Model:   "minimax-m2.5",
		Payload: originalRequest,
	}, cliproxyexecutor.Options{
		SourceFormat:    sdktranslator.FromString("openai-response"),
		OriginalRequest: originalRequest,
		Stream:          true,
	})
	if err != nil {
		t.Fatalf("ExecuteStream returned unexpected error: %v", err)
	}

	var chunks [][]byte
	for chunk := range streamResult.Chunks {
		if chunk.Err != nil {
			t.Fatalf("unexpected stream error: %v", chunk.Err)
		}
		chunks = append(chunks, append([]byte(nil), chunk.Payload...))
	}

	if requestCount != 2 {
		t.Fatalf("expected 2 upstream calls, got %d", requestCount)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected one synthesized chunk, got %d", len(chunks))
	}
	got := string(chunks[0])
	if !strings.Contains(got, "event: response.completed") {
		t.Fatalf("expected response.completed SSE event, got %q", got)
	}
	if !strings.Contains(got, "hi from iflow fallback") {
		t.Fatalf("expected assistant text in synthesized payload, got %q", got)
	}
}

func TestIFlowExecutorExecuteRefreshesOnProviderExpiryEnvelope(t *testing.T) {
	requestCount := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		switch requestCount {
		case 1:
			_, _ = w.Write([]byte(`{"status":"439","msg":"Your API Token has expired.","body":null}`))
		case 2:
			_, _ = w.Write([]byte(`{"id":"chatcmpl_iflow","object":"chat.completion","created":1735689600,"model":"minimax-m2.5","choices":[{"index":0,"message":{"role":"assistant","content":"hi after refresh"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`))
		default:
			t.Fatalf("unexpected upstream call %d", requestCount)
		}
	}))
	defer upstream.Close()

	auth := &cliproxyauth.Auth{
		Attributes: map[string]string{
			"base_url": upstream.URL,
			"api_key":  "expired-key",
		},
		Metadata: map[string]any{
			"cookie":  "cookie",
			"email":   "user@example.com",
			"expired": "2000-01-01T00:00:00Z",
		},
	}

	executor := &IFlowExecutor{}
	originalRequest := []byte(`{"model":"minimax-m2.5","input":[{"role":"user","content":"hi"}]}`)
	resp, err := executor.execute(context.Background(), auth, cliproxyexecutor.Request{
		Model:   "minimax-m2.5",
		Payload: originalRequest,
	}, cliproxyexecutor.Options{
		SourceFormat:    sdktranslator.FromString("openai-response"),
		OriginalRequest: originalRequest,
	}, true)
	if err != nil {
		t.Fatalf("execute returned unexpected error: %v", err)
	}

	if requestCount != 2 {
		t.Fatalf("expected 2 upstream calls, got %d", requestCount)
	}
	if !strings.Contains(string(resp.Payload), `"hi after refresh"`) {
		t.Fatalf("expected translated payload to include refreshed content, got %s", resp.Payload)
	}
}

func TestIFlowExecutorExecuteStreamFallbackUnwrapsDataEnvelope(t *testing.T) {
	requestCount := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		body, _ := io.ReadAll(r.Body)

		switch requestCount {
		case 1:
			if got := r.Header.Get("Accept"); got != "text/event-stream" {
				t.Fatalf("expected stream Accept header, got %q", got)
			}
			if !strings.Contains(string(body), `"stream":true`) {
				t.Fatalf("expected initial stream request, got %s", body)
			}
			http.Error(w, "status 406", http.StatusNotAcceptable)
		case 2:
			if strings.Contains(string(body), `"stream":true`) {
				t.Fatalf("expected fallback request to disable stream, got %s", body)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":"chatcmpl_iflow","object":"chat.completion","created":1735689600,"model":"minimax-m2.5","choices":[{"index":0,"message":{"role":"assistant","content":"hello from wrapped iflow"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}}`))
		default:
			t.Fatalf("unexpected upstream call %d", requestCount)
		}
	}))
	defer upstream.Close()

	executor := NewIFlowExecutor(nil)
	auth := &cliproxyauth.Auth{Attributes: map[string]string{
		"base_url": upstream.URL,
		"api_key":  "iflow-test",
	}}
	originalRequest := []byte(`{"model":"minimax-m2.5","stream":true,"input":[{"role":"user","content":"hi"}]}`)
	streamResult, err := executor.ExecuteStream(context.Background(), auth, cliproxyexecutor.Request{
		Model:   "minimax-m2.5",
		Payload: originalRequest,
	}, cliproxyexecutor.Options{
		SourceFormat:    sdktranslator.FromString("openai-response"),
		OriginalRequest: originalRequest,
		Stream:          true,
	})
	if err != nil {
		t.Fatalf("ExecuteStream returned unexpected error: %v", err)
	}

	var chunks [][]byte
	for chunk := range streamResult.Chunks {
		if chunk.Err != nil {
			t.Fatalf("unexpected stream error: %v", chunk.Err)
		}
		chunks = append(chunks, append([]byte(nil), chunk.Payload...))
	}

	if requestCount != 2 {
		t.Fatalf("expected 2 upstream calls, got %d", requestCount)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected one synthesized chunk, got %d", len(chunks))
	}
	got := string(chunks[0])
	if !strings.Contains(got, "hello from wrapped iflow") {
		t.Fatalf("expected unwrapped assistant text in synthesized payload, got %q", got)
	}
}
