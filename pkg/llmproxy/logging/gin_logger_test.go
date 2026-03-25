package logging

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGinLogrusRecoveryRepanicsErrAbortHandler(t *testing.T) {
	// Test the recovery logic directly: gin.CustomRecovery's internal recovery
	// handling varies across platforms (macOS vs Linux) and Go versions, so we
	// invoke the recovery callback that GinLogrusRecovery passes to
	// gin.CustomRecovery and verify it re-panics ErrAbortHandler.
	gin.SetMode(gin.TestMode)

	var repanicked bool
	var repanic interface{}

	func() {
		defer func() {
			if r := recover(); r != nil {
				repanicked = true
				repanic = r
			}
		}()
		// Simulate what gin.CustomRecovery does: call the recovery func
		// with the recovered value.
		ginLogrusRecoveryFunc(nil, http.ErrAbortHandler)
	}()

	if !repanicked {
		t.Fatalf("expected ginLogrusRecoveryFunc to re-panic http.ErrAbortHandler, but it did not")
	}
	err, ok := repanic.(error)
	if !ok {
		t.Fatalf("expected error panic, got %T", repanic)
	}
	if !errors.Is(err, http.ErrAbortHandler) {
		t.Fatalf("expected ErrAbortHandler, got %v", err)
	}
}

func TestGinLogrusRecoveryHandlesRegularPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(GinLogrusRecovery())
	engine.GET("/panic", func(c *gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
}

func TestGinLogrusLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(GinLogrusLogger())
	engine.GET("/v1/chat/completions", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	engine.GET("/skip", func(c *gin.Context) {
		SkipGinRequestLogging(c)
		c.String(http.StatusOK, "skipped")
	})

	// AI API path
	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	// Regular path
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	// Skipped path
	req = httptest.NewRequest(http.MethodGet, "/skip", nil)
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
}

func TestIsAIAPIPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/v1/chat/completions", true},
		{"/v1/messages", true},
		{"/other", false},
	}
	for _, tc := range cases {
		if got := isAIAPIPath(tc.path); got != tc.want {
			t.Errorf("isAIAPIPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
