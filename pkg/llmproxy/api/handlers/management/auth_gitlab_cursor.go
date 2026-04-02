package management

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	gitlabauth "github.com/kooshapari/CLIProxyAPI/v7/internal/auth/gitlab"
	cursorauth "github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/auth/cursor"
	sdkAuth "github.com/kooshapari/CLIProxyAPI/v7/sdk/auth"
	coreauth "github.com/kooshapari/CLIProxyAPI/v7/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

func (h *Handler) RequestGitLabPATToken(c *gin.Context) {
	var req struct {
		BaseURL             string `json:"base_url"`
		PersonalAccessToken string `json:"personal_access_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	record, err := sdkAuth.NewGitLabAuthenticator().Login(context.Background(), h.cfg, &sdkAuth.LoginOptions{
		NoBrowser: true,
		Metadata: map[string]string{
			"login_mode":            "pat",
			"base_url":              strings.TrimSpace(req.BaseURL),
			"personal_access_token": strings.TrimSpace(req.PersonalAccessToken),
		},
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "gitlab_pat_failed", "message": err.Error()})
		return
	}

	savedPath, err := h.saveTokenRecord(context.Background(), record)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save_failed", "message": err.Error()})
		return
	}

	resp := gin.H{
		"status": "ok",
		"path":   savedPath,
	}
	if record != nil && record.Metadata != nil {
		if v, ok := record.Metadata["model_provider"]; ok {
			resp["model_provider"] = v
		}
		if v, ok := record.Metadata["model_name"]; ok {
			resp["model_name"] = v
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) RequestGitLabToken(c *gin.Context) {
	ctx := context.Background()
	baseURL := strings.TrimSpace(c.Query("base_url"))
	if baseURL == "" {
		baseURL = gitlabauth.DefaultBaseURL
	}
	clientID := strings.TrimSpace(c.Query("client_id"))
	if clientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "client_id is required"})
		return
	}
	clientSecret := strings.TrimSpace(c.Query("client_secret"))

	pkceCodes, err := gitlabauth.GeneratePKCECodes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate pkce", "message": err.Error()})
		return
	}
	state := fmt.Sprintf("gitlab-%d", time.Now().UnixNano())
	redirectURI := gitlabauth.RedirectURL(gitlabauth.DefaultCallbackPort)
	authURL, err := gitlabauth.NewAuthClient(h.cfg).GenerateAuthURL(baseURL, clientID, redirectURI, state, pkceCodes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate auth url", "message": err.Error()})
		return
	}

	RegisterOAuthSession(state, "gitlab")
	cleanup, err := h.setupCallbackForwarder(c, gitlabauth.DefaultCallbackPort, "gitlab", "/oauth-callback")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "callback server unavailable", "message": err.Error()})
		return
	}

	go func() {
		defer cleanup()

		result, errWait := h.waitForOAuthCallback(state, "gitlab", 5*time.Minute)
		if errWait != nil {
			SetOAuthSessionError(state, "OAuth flow timed out")
			return
		}
		if errMsg := strings.TrimSpace(result["error"]); errMsg != "" {
			SetOAuthSessionError(state, errMsg)
			return
		}
		if result["state"] != state {
			SetOAuthSessionError(state, "State mismatch")
			return
		}
		code := strings.TrimSpace(result["code"])
		if code == "" {
			SetOAuthSessionError(state, "No authorization code received")
			return
		}

		client := gitlabauth.NewAuthClient(h.cfg)
		tokenResp, err := client.ExchangeCodeForTokens(ctx, baseURL, clientID, clientSecret, redirectURI, code, pkceCodes.CodeVerifier)
		if err != nil {
			SetOAuthSessionError(state, "Failed to exchange code for tokens")
			return
		}
		user, err := client.GetCurrentUser(ctx, baseURL, strings.TrimSpace(tokenResp.AccessToken))
		if err != nil {
			SetOAuthSessionError(state, "Failed to fetch user info")
			return
		}
		direct, err := client.FetchDirectAccess(ctx, baseURL, strings.TrimSpace(tokenResp.AccessToken))
		if err != nil {
			SetOAuthSessionError(state, "Failed to fetch direct access")
			return
		}

		identifier := sanitizeAuthFileName(strings.TrimSpace(user.Username))
		if identifier == "" {
			identifier = sanitizeAuthFileName(strings.TrimSpace(user.Email))
		}
		if identifier == "" {
			identifier = fmt.Sprintf("gitlab-%d", time.Now().Unix())
		}

		metadata := map[string]any{
			"type":            "gitlab",
			"auth_method":     "oauth",
			"auth_kind":       "oauth",
			"base_url":        gitlabauth.NormalizeBaseURL(baseURL),
			"access_token":    strings.TrimSpace(tokenResp.AccessToken),
			"refresh_token":   strings.TrimSpace(tokenResp.RefreshToken),
			"token_type":      strings.TrimSpace(tokenResp.TokenType),
			"scope":           strings.TrimSpace(tokenResp.Scope),
			"username":        strings.TrimSpace(user.Username),
			"name":            strings.TrimSpace(user.Name),
			"email":           strings.TrimSpace(user.Email),
			"public_email":    strings.TrimSpace(user.PublicEmail),
			"last_refresh":    time.Now().UTC().Format(time.RFC3339),
			"oauth_client_id": clientID,
		}
		if clientSecret != "" {
			metadata["oauth_client_secret"] = clientSecret
		}
		if direct != nil {
			if base := strings.TrimSpace(direct.BaseURL); base != "" {
				metadata["duo_gateway_base_url"] = base
			}
			if token := strings.TrimSpace(direct.Token); token != "" {
				metadata["duo_gateway_token"] = token
			}
			if direct.ExpiresAt > 0 {
				metadata["duo_gateway_expires_at"] = time.Unix(direct.ExpiresAt, 0).UTC().Format(time.RFC3339)
			}
			if len(direct.Headers) > 0 {
				metadata["duo_gateway_headers"] = direct.Headers
			}
			if direct.ModelDetails != nil {
				metadata["model_provider"] = strings.TrimSpace(direct.ModelDetails.ModelProvider)
				metadata["model_name"] = strings.TrimSpace(direct.ModelDetails.ModelName)
			}
		}

		record := &coreauth.Auth{
			ID:       fmt.Sprintf("gitlab-%s.json", identifier),
			Provider: "gitlab",
			FileName: fmt.Sprintf("gitlab-%s.json", identifier),
			Label:    strings.TrimSpace(user.Username),
			Metadata: metadata,
		}
		if errSave := h.saveAndCompleteAuth(ctx, state, "gitlab", record, "GitLab authentication successful!"); errSave != nil {
			log.WithError(errSave).Error("failed to complete gitlab auth")
		}
	}()

	c.JSON(http.StatusOK, gin.H{"status": "ok", "url": authURL, "state": state})
}

func (h *Handler) RequestCursorToken(c *gin.Context) {
	ctx := context.Background()
	authParams, err := cursorauth.GenerateAuthParams()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate auth params", "message": err.Error()})
		return
	}
	state := fmt.Sprintf("cursor-%d", time.Now().UnixNano())
	RegisterOAuthSession(state, "cursor")

	go func() {
		tokens, err := cursorauth.PollForAuth(ctx, authParams.UUID, authParams.Verifier)
		if err != nil {
			SetOAuthSessionError(state, "Authentication failed")
			return
		}
		expiresAt := cursorauth.GetTokenExpiry(tokens.AccessToken)
		sub := cursorauth.ParseJWTSub(tokens.AccessToken)
		subHash := cursorauth.SubToShortHash(sub)

		metadata := map[string]any{
			"type":          "cursor",
			"access_token":  tokens.AccessToken,
			"refresh_token": tokens.RefreshToken,
			"expires_at":    expiresAt.Format(time.RFC3339),
			"timestamp":     time.Now().UnixMilli(),
		}
		if sub != "" {
			metadata["sub"] = sub
		}

		fileName := cursorauth.CredentialFileName("", subHash)
		record := &coreauth.Auth{
			ID:       fileName,
			Provider: "cursor",
			FileName: fileName,
			Label:    cursorauth.DisplayLabel("", subHash),
			Metadata: metadata,
		}
		if errSave := h.saveAndCompleteAuth(ctx, state, "cursor", record, "Cursor authentication successful!"); errSave != nil {
			log.WithError(errSave).Error("failed to complete cursor auth")
		}
	}()

	c.JSON(http.StatusOK, gin.H{"status": "ok", "url": authParams.LoginURL, "state": state})
}

func sanitizeAuthFileName(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(raw))
	fallback := hex.EncodeToString(digest[:])[:8]
	replacer := strings.NewReplacer("@", "-", ".", "-", "/", "-", "\\", "-", " ", "-")
	cleaned := replacer.Replace(raw)
	cleaned = strings.Trim(cleaned, "-")
	if cleaned == "" {
		return fallback
	}
	return cleaned
}
