// Package executor provides HTTP request execution for various AI providers.
// This file contains Kiro-specific authentication handling logic.
package executor

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	kiroauth "github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/auth/kiro"
	"github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/config"
	"github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/util"
	cliproxyauth "github.com/kooshapari/CLIProxyAPI/v7/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

// kiroCredentials extracts access token and profile ARN from auth object.
func kiroCredentials(auth *cliproxyauth.Auth) (accessToken, profileArn string) {
	if auth == nil {
		return "", ""
	}
	if auth.Metadata != nil {
		if token, ok := auth.Metadata["access_token"].(string); ok {
			accessToken = token
		}
		if arn, ok := auth.Metadata["profile_arn"].(string); ok {
			profileArn = arn
		}
	}
	if accessToken == "" && auth.Attributes != nil {
		accessToken = auth.Attributes["access_token"]
		profileArn = auth.Attributes["profile_arn"]
	}
	if accessToken == "" && auth.Metadata != nil {
		if token, ok := auth.Metadata["accessToken"].(string); ok {
			accessToken = token
		}
		if arn, ok := auth.Metadata["profileArn"].(string); ok {
			profileArn = arn
		}
	}
	return accessToken, profileArn
}

// getTokenKey returns a unique key for rate limiting based on auth credentials.
func getTokenKey(auth *cliproxyauth.Auth) string {
	if auth != nil && auth.ID != "" {
		return auth.ID
	}
	accessToken, _ := kiroCredentials(auth)
	if len(accessToken) > 16 {
		return accessToken[:16]
	}
	return accessToken
}

// isIDCAuth checks if this auth object uses IDC authentication.
func isIDCAuth(auth *cliproxyauth.Auth) bool {
	if auth == nil || auth.Metadata == nil {
		return false
	}
	authMethod := getMetadataString(auth.Metadata, "auth_method", "authMethod")
	return strings.ToLower(authMethod) == "idc" ||
		strings.ToLower(authMethod) == "builder-id"
}

// applyDynamicFingerprint applies fingerprint-based headers to requests.
func applyDynamicFingerprint(req *http.Request, auth *cliproxyauth.Auth) {
	if isIDCAuth(auth) {
		tokenKey := getTokenKey(auth)
		fp := getGlobalFingerprintManager().GetFingerprint(tokenKey)
		req.Header.Set("User-Agent", fp.BuildUserAgent())
		req.Header.Set("X-Amz-User-Agent", fp.BuildAmzUserAgent())
		req.Header.Set("x-amzn-kiro-agent-mode", kiroIDEAgentModeVibe)
		log.Debugf("kiro: using dynamic fingerprint for token %s (SDK:%s, OS:%s/%s, Kiro:%s)",
			tokenKey[:8]+"...", fp.SDKVersion, fp.OSType, fp.OSVersion, fp.KiroVersion)
	} else {
		req.Header.Set("User-Agent", kiroUserAgent)
		req.Header.Set("X-Amz-User-Agent", kiroFullUserAgent)
	}
}

// PrepareRequest prepares the HTTP request before execution.
func (e *KiroExecutor) PrepareRequest(req *http.Request, auth *cliproxyauth.Auth) error {
	if req == nil {
		return nil
	}
	accessToken, _ := kiroCredentials(auth)
	if strings.TrimSpace(accessToken) == "" {
		return statusErr{code: http.StatusUnauthorized, msg: "missing access token"}
	}
	applyDynamicFingerprint(req, auth)
	req.Header.Set("Amz-Sdk-Request", "attempt=1; max=3")
	req.Header.Set("Amz-Sdk-Invocation-Id", uuid.New().String())
	req.Header.Set("Authorization", "Bearer "+accessToken)
	var attrs map[string]string
	if auth != nil {
		attrs = auth.Attributes
	}
	util.ApplyCustomHeadersFromAttrs(req, attrs)
	return nil
}

// HttpRequest injects Kiro credentials into the request and executes it.
func (e *KiroExecutor) HttpRequest(ctx context.Context, auth *cliproxyauth.Auth, req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("kiro executor: request is nil")
	}
	if ctx == nil {
		ctx = req.Context()
	}
	httpReq := req.WithContext(ctx)
	if errPrepare := e.PrepareRequest(httpReq, auth); errPrepare != nil {
		return nil, errPrepare
	}
	httpClient := newKiroHTTPClientWithPooling(ctx, e.cfg, auth, 0)
	return httpClient.Do(httpReq)
}

// Refresh performs token refresh using appropriate OAuth2 flow.
func (e *KiroExecutor) Refresh(ctx context.Context, auth *cliproxyauth.Auth) (*cliproxyauth.Auth, error) {
	e.refreshMu.Lock()
	defer e.refreshMu.Unlock()

	var authID string
	if auth != nil {
		authID = auth.ID
	} else {
		authID = "<nil>"
	}
	log.Debugf("kiro executor: refresh called for auth %s", authID)
	if auth == nil {
		return nil, fmt.Errorf("kiro executor: auth is nil")
	}

	if auth.Metadata != nil {
		if lastRefresh, ok := auth.Metadata["last_refresh"].(string); ok {
			if refreshTime, err := time.Parse(time.RFC3339, lastRefresh); err == nil {
				if time.Since(refreshTime) < 30*time.Second {
					log.Debugf("kiro executor: token was recently refreshed by another goroutine, skipping")
					return auth, nil
				}
			}
		}
		if expiresAt, ok := auth.Metadata["expires_at"].(string); ok {
			if expTime, err := time.Parse(time.RFC3339, expiresAt); err == nil {
				if time.Until(expTime) > 20*time.Minute {
					log.Debugf("kiro executor: token is still valid (expires in %v), skipping refresh", time.Until(expTime))
					updated := auth.Clone()
					nextRefresh := expTime.Add(-20 * time.Minute)
					minNextRefresh := time.Now().Add(30 * time.Second)
					if nextRefresh.Before(minNextRefresh) {
						nextRefresh = minNextRefresh
					}
					updated.NextRefreshAfter = nextRefresh
					log.Debugf("kiro executor: setting NextRefreshAfter to %v (in %v)", nextRefresh.Format(time.RFC3339), time.Until(nextRefresh))
					return updated, nil
				}
			}
		}
	}

	var refreshToken string
	var clientID, clientSecret string
	var authMethod string
	var region, startURL string

	if auth.Metadata != nil {
		refreshToken = getMetadataString(auth.Metadata, "refresh_token", "refreshToken")
		clientID = getMetadataString(auth.Metadata, "client_id", "clientId")
		clientSecret = getMetadataString(auth.Metadata, "client_secret", "clientSecret")
		authMethod = strings.ToLower(getMetadataString(auth.Metadata, "auth_method", "authMethod"))
		region = getMetadataString(auth.Metadata, "region")
		startURL = getMetadataString(auth.Metadata, "start_url", "startUrl")
	}

	if refreshToken == "" {
		return nil, fmt.Errorf("kiro executor: refresh token not found")
	}

	var tokenData *kiroauth.KiroTokenData
	var err error

	ssoClient := kiroauth.NewSSOOIDCClient(e.cfg)

	switch {
	case clientID != "" && clientSecret != "" && authMethod == "idc" && region != "":
		log.Debugf("kiro executor: using SSO OIDC refresh for IDC (region=%s)", region)
		tokenData, err = ssoClient.RefreshTokenWithRegion(ctx, clientID, clientSecret, refreshToken, region, startURL)
	case clientID != "" && clientSecret != "" && authMethod == "builder-id":
		log.Debugf("kiro executor: using SSO OIDC refresh for AWS Builder ID")
		tokenData, err = ssoClient.RefreshToken(ctx, clientID, clientSecret, refreshToken)
	default:
		log.Debugf("kiro executor: using Kiro OAuth refresh endpoint")
		oauth := kiroauth.NewKiroOAuth(e.cfg)
		tokenData, err = oauth.RefreshToken(ctx, refreshToken)
	}

	if err != nil {
		return nil, fmt.Errorf("kiro executor: token refresh failed: %w", err)
	}

	updated := auth.Clone()
	now := time.Now()
	updated.UpdatedAt = now
	updated.LastRefreshedAt = now

	if updated.Metadata == nil {
		updated.Metadata = make(map[string]any)
	}
	updated.Metadata["access_token"] = tokenData.AccessToken
	updated.Metadata["refresh_token"] = tokenData.RefreshToken
	updated.Metadata["expires_at"] = tokenData.ExpiresAt
	updated.Metadata["last_refresh"] = now.Format(time.RFC3339)
	if tokenData.ProfileArn != "" {
		updated.Metadata["profile_arn"] = tokenData.ProfileArn
	}
	if tokenData.AuthMethod != "" {
		updated.Metadata["auth_method"] = tokenData.AuthMethod
	}
	if tokenData.Provider != "" {
		updated.Metadata["provider"] = tokenData.Provider
	}
	if tokenData.ClientID != "" {
		updated.Metadata["client_id"] = tokenData.ClientID
	}
	if tokenData.ClientSecret != "" {
		updated.Metadata["client_secret"] = tokenData.ClientSecret
	}
	if tokenData.Region != "" {
		updated.Metadata["region"] = tokenData.Region
	}
	if tokenData.StartURL != "" {
		updated.Metadata["start_url"] = tokenData.StartURL
	}

	if updated.Attributes == nil {
		updated.Attributes = make(map[string]string)
	}
	updated.Attributes["access_token"] = tokenData.AccessToken
	if tokenData.ProfileArn != "" {
		updated.Attributes["profile_arn"] = tokenData.ProfileArn
	}

	if expiresAt, parseErr := time.Parse(time.RFC3339, tokenData.ExpiresAt); parseErr == nil {
		updated.NextRefreshAfter = expiresAt.Add(-20 * time.Minute)
	}

	log.Infof("kiro executor: token refreshed successfully, expires at %s", tokenData.ExpiresAt)
	return updated, nil
}

// persistRefreshedAuth persists a refreshed auth record to disk.
func (e *KiroExecutor) persistRefreshedAuth(auth *cliproxyauth.Auth) error {
	if auth == nil || auth.Metadata == nil {
		return fmt.Errorf("kiro executor: cannot persist nil auth or metadata")
	}

	var authPath string
	if auth.Attributes != nil {
		if p := strings.TrimSpace(auth.Attributes["path"]); p != "" {
			authPath = p
		}
	}
	if authPath == "" {
		fileName := strings.TrimSpace(auth.FileName)
		if fileName == "" {
			return fmt.Errorf("kiro executor: auth has no file path or filename")
		}
		if filepath.IsAbs(fileName) {
			authPath = fileName
		} else if e.cfg != nil && e.cfg.AuthDir != "" {
			authPath = filepath.Join(e.cfg.AuthDir, fileName)
		} else {
			return fmt.Errorf("kiro executor: cannot determine auth file path")
		}
	}

	raw, err := json.Marshal(auth.Metadata)
	if err != nil {
		return fmt.Errorf("kiro executor: marshal metadata failed: %w", err)
	}

	tmp := authPath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("kiro executor: write temp auth file failed: %w", err)
	}
	if err := os.Rename(tmp, authPath); err != nil {
		return fmt.Errorf("kiro executor: rename auth file failed: %w", err)
	}

	log.Debugf("kiro executor: persisted refreshed auth to %s", authPath)
	return nil
}

// reloadAuthFromFile reloads the auth object from its persistent storage.
func (e *KiroExecutor) reloadAuthFromFile(auth *cliproxyauth.Auth) (*cliproxyauth.Auth, error) {
	if auth == nil {
		return nil, fmt.Errorf("kiro executor: cannot reload nil auth")
	}

	var authPath string
	if auth.Attributes != nil {
		if p := strings.TrimSpace(auth.Attributes["path"]); p != "" {
			authPath = p
		}
	}
	if authPath == "" {
		fileName := strings.TrimSpace(auth.FileName)
		if fileName == "" {
			return nil, fmt.Errorf("kiro executor: auth has no file path or filename for reload")
		}
		if filepath.IsAbs(fileName) {
			authPath = fileName
		} else if e.cfg != nil && e.cfg.AuthDir != "" {
			authPath = filepath.Join(e.cfg.AuthDir, fileName)
		} else {
			return nil, fmt.Errorf("kiro executor: cannot determine auth file path for reload")
		}
	}

	raw, err := os.ReadFile(authPath)
	if err != nil {
		return nil, fmt.Errorf("kiro executor: read auth file failed: %w", err)
	}

	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return nil, fmt.Errorf("kiro executor: unmarshal auth metadata failed: %w", err)
	}

	reloaded := auth.Clone()
	reloaded.Metadata = metadata
	log.Debugf("kiro executor: reloaded auth from %s", authPath)
	return reloaded, nil
}

// isTokenExpired checks if the access token has expired by decoding JWT.
func (e *KiroExecutor) isTokenExpired(accessToken string) bool {
	if accessToken == "" {
		return true
	}

	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return false
	}

	payload := parts[1]
	switch len(payload) % 4 {
	case 1:
		payload += "==="
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}

	decoded, err := base64.RawStdEncoding.DecodeString(payload)
	if err != nil {
		log.Debugf("kiro: failed to decode JWT payload: %v", err)
		return false
	}

	var claims map[string]any
	if err := json.Unmarshal(decoded, &claims); err != nil {
		log.Debugf("kiro: failed to parse JWT claims: %v", err)
		return false
	}

	if exp, ok := claims["exp"]; ok {
		var expiresAt int64
		switch v := exp.(type) {
		case float64:
			expiresAt = int64(v)
		case int64:
			expiresAt = v
		default:
			return false
		}

		now := time.Now().Unix()
		if now > expiresAt {
			log.Debugf("kiro: token expired at %d (now: %d)", expiresAt, now)
			return true
		}
	}

	return false
}
