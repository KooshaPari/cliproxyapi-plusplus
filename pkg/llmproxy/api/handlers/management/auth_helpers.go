package management

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	coreauth "github.com/kooshapari/CLIProxyAPI/v7/sdk/cliproxy/auth"
)

var lastRefreshKeys = []string{"last_refresh", "lastRefresh", "last_refreshed_at", "lastRefreshedAt"}

const (
	anthropicCallbackPort   = 54545
	geminiCallbackPort      = 8085
	codexCallbackPort       = 1455
	geminiCLIEndpoint       = "https://cloudcode-pa.googleapis.com"
	geminiCLIVersion        = "v1internal"
	geminiCLIUserAgent      = "google-api-nodejs-client/9.15.1"
	geminiCLIApiClient      = "gl-node/22.17.0"
	geminiCLIClientMetadata = "ideType=IDE_UNSPECIFIED,platform=PLATFORM_UNSPECIFIED,pluginType=GEMINI"
)

type callbackForwarder struct {
	provider string
	server   *http.Server
	done     chan struct{}
}

var (
	callbackForwardersMu sync.Mutex
	callbackForwarders   = make(map[int]*callbackForwarder)
)

func extractLastRefreshTimestamp(meta map[string]any) (time.Time, bool) {
	if len(meta) == 0 {
		return time.Time{}, false
	}
	for _, key := range lastRefreshKeys {
		if val, ok := meta[key]; ok {
			if ts, ok1 := parseLastRefreshValue(val); ok1 {
				return ts, true
			}
		}
	}
	return time.Time{}, false
}

func parseLastRefreshValue(v any) (time.Time, bool) {
	switch val := v.(type) {
	case string:
		s := strings.TrimSpace(val)
		if s == "" {
			return time.Time{}, false
		}
		layouts := []string{time.RFC3339, time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z07:00"}
		for _, layout := range layouts {
			if ts, err := time.Parse(layout, s); err == nil {
				return ts.UTC(), true
			}
		}
		if unix, err := strconv.ParseInt(s, 10, 64); err == nil && unix > 0 {
			return time.Unix(unix, 0).UTC(), true
		}
	case float64:
		if val <= 0 {
			return time.Time{}, false
		}
		return time.Unix(int64(val), 0).UTC(), true
	case int64:
		if val <= 0 {
			return time.Time{}, false
		}
		return time.Unix(val, 0).UTC(), true
	case int:
		if val <= 0 {
			return time.Time{}, false
		}
		return time.Unix(int64(val), 0).UTC(), true
	case json.Number:
		if i, err := val.Int64(); err == nil && i > 0 {
			return time.Unix(i, 0).UTC(), true
		}
	}
	return time.Time{}, false
}

func isWebUIRequest(c *gin.Context) bool {
	raw := strings.TrimSpace(c.Query("is_webui"))
	if raw == "" {
		return false
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func startCallbackForwarder(port int, provider, targetBase string) (*callbackForwarder, error) {
	targetURL, errTarget := validateCallbackForwarderTarget(targetBase)
	if errTarget != nil {
		return nil, fmt.Errorf("invalid callback target: %w", errTarget)
	}

	callbackForwardersMu.Lock()
	prev := callbackForwarders[port]
	if prev != nil {
		delete(callbackForwarders, port)
	}
	callbackForwardersMu.Unlock()

	if prev != nil {
		stopForwarderInstance(port, prev)
	}

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := *targetURL
		if raw := r.URL.RawQuery; raw != "" {
			if target.RawQuery != "" {
				target.RawQuery = target.RawQuery + "&" + raw
			} else {
				target.RawQuery = raw
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, target.String(), http.StatusFound)
	})

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
	}
	done := make(chan struct{})

	go func() {
		if errServe := srv.Serve(ln); errServe != nil && !errors.Is(errServe, http.ErrServerClosed) {
			log.WithError(errServe).Warnf("callback forwarder for %s stopped unexpectedly", provider)
		}
		close(done)
	}()

	forwarder := &callbackForwarder{
		provider: provider,
		server:   srv,
		done:     done,
	}

	callbackForwardersMu.Lock()
	callbackForwarders[port] = forwarder
	callbackForwardersMu.Unlock()

	log.Infof("callback forwarder for %s listening on %s", provider, addr)

	return forwarder, nil
}

func validateCallbackForwarderTarget(targetBase string) (*url.URL, error) {
	trimmed := strings.TrimSpace(targetBase)
	if trimmed == "" {
		return nil, fmt.Errorf("target cannot be empty")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("parse target: %w", err)
	}
	if !parsed.IsAbs() {
		return nil, fmt.Errorf("target must be absolute")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("target scheme %q is not allowed", parsed.Scheme)
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return nil, fmt.Errorf("target host is required")
	}
	if ip := net.ParseIP(host); ip != nil {
		if !ip.IsLoopback() {
			return nil, fmt.Errorf("target host must be loopback")
		}
		return parsed, nil
	}
	if host != "localhost" {
		return nil, fmt.Errorf("target host must be localhost or loopback")
	}
	return parsed, nil
}

func stopCallbackForwarderInstance(port int, forwarder *callbackForwarder) {
	if forwarder == nil {
		return
	}
	callbackForwardersMu.Lock()
	if current := callbackForwarders[port]; current == forwarder {
		delete(callbackForwarders, port)
	}
	callbackForwardersMu.Unlock()

	stopForwarderInstance(port, forwarder)
}

func stopForwarderInstance(port int, forwarder *callbackForwarder) {
	if forwarder == nil || forwarder.server == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := forwarder.server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.WithError(err).Warnf("failed to shut down callback forwarder on port %d", port)
	}

	select {
	case <-forwarder.done:
	case <-time.After(2 * time.Second):
	}

	log.Infof("callback forwarder on port %d stopped", port)
}

func (h *Handler) managementCallbackURL(path string) (string, error) {
	if h == nil || h.cfg == nil || h.cfg.Port <= 0 {
		return "", fmt.Errorf("server port is not configured")
	}
	path = normalizeManagementCallbackPath(path)
	scheme := "http"
	if h.cfg.TLS.Enable {
		scheme = "https"
	}
	return fmt.Sprintf("%s://127.0.0.1:%d%s", scheme, h.cfg.Port, path), nil
}

func normalizeManagementCallbackPath(rawPath string) string {
	normalized := strings.TrimSpace(rawPath)
	normalized = strings.ReplaceAll(normalized, "\\", "/")
	if idx := strings.IndexAny(normalized, "?#"); idx >= 0 {
		normalized = normalized[:idx]
	}
	if normalized == "" {
		return "/"
	}
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}
	normalized = path.Clean(normalized)
	// Security: Verify cleaned path is safe (no open redirect)
	if normalized == "." || normalized == "" {
		return "/"
	}
	// Prevent open redirect attacks (e.g., //evil.com or http://...)
	if strings.Contains(normalized, "//") || strings.Contains(normalized, ":/") {
		return "/"
	}
	// Security: Ensure path doesn't start with // or \ (could be interpreted as URL)
	if len(normalized) >= 2 && (normalized[1] == '/' || normalized[1] == '\\') {
		return "/"
	}
	if !strings.HasPrefix(normalized, "/") {
		return "/" + normalized
	}
	return normalized
}

// waitForOAuthCallback polls the auth directory for an OAuth callback file written by the
// callback route handler. The file name follows the convention:
//
//	.oauth-<provider>-<state>.oauth
//
// It polls every 500 ms until timeout elapses or the OAuth session is no longer pending.
// On success it returns the decoded key/value pairs from the JSON file and removes the file.
func (h *Handler) waitForOAuthCallback(state, provider string, timeout time.Duration) (map[string]string, error) {
	waitFile := filepath.Join(h.cfg.AuthDir, fmt.Sprintf(".oauth-%s-%s.oauth", provider, state))
	deadline := time.Now().Add(timeout)
	for {
		if !IsOAuthSessionPending(state, provider) {
			return nil, errOAuthSessionNotPending
		}
		if time.Now().After(deadline) {
			SetOAuthSessionError(state, "Timeout waiting for OAuth callback")
			return nil, fmt.Errorf("timeout waiting for OAuth callback")
		}
		data, errRead := os.ReadFile(waitFile)
		if errRead == nil {
			var m map[string]string
			_ = json.Unmarshal(data, &m)
			_ = os.Remove(waitFile)
			return m, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// setupCallbackForwarder starts a callback forwarder when the request comes from the
// web UI. It returns a cleanup function that stops the forwarder; the caller should
// defer the returned func. If the request is not a web UI request the returned cleanup
// func is a no-op and forwarder is nil.
func (h *Handler) setupCallbackForwarder(c *gin.Context, port int, provider, callbackPath string) (cleanup func(), err error) {
	noop := func() {}
	if !isWebUIRequest(c) {
		return noop, nil
	}
	targetURL, errTarget := h.managementCallbackURL(callbackPath)
	if errTarget != nil {
		return noop, fmt.Errorf("failed to compute %s callback target: %w", provider, errTarget)
	}
	forwarder, errStart := startCallbackForwarder(port, provider, targetURL)
	if errStart != nil {
		return noop, fmt.Errorf("failed to start %s callback forwarder: %w", provider, errStart)
	}
	return func() { stopCallbackForwarderInstance(port, forwarder) }, nil
}

// saveAndCompleteAuth persists the token record, prints a success message, then marks the
// OAuth session complete by state and by provider. It is the final step shared by every
// OAuth provider handler.
func (h *Handler) saveAndCompleteAuth(ctx context.Context, state, provider string, record *coreauth.Auth, successMsg string) error {
	savedPath, errSave := h.saveTokenRecord(ctx, record)
	if errSave != nil {
		SetOAuthSessionError(state, "Failed to save authentication tokens")
		return fmt.Errorf("failed to save authentication tokens: %w", errSave)
	}
	fmt.Printf("%s Token saved to %s\n", successMsg, savedPath)
	CompleteOAuthSession(state)
	CompleteOAuthSessionsByProvider(provider)
	return nil
}
