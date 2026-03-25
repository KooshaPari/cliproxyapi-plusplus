package management

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/auth/antigravity"
	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/misc"
	coreauth "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

func (h *Handler) RequestAntigravityToken(c *gin.Context) {
	ctx := context.Background()

	fmt.Println("Initializing Antigravity authentication...")

	authSvc := antigravity.NewAntigravityAuth(h.cfg, nil)

	state, errState := misc.GenerateRandomState()
	if errState != nil {
		log.Errorf("Failed to generate state parameter: %v", errState)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate state parameter"})
		return
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/oauth-callback", antigravity.CallbackPort)
	authURL := authSvc.BuildAuthURL(state, redirectURI)

	RegisterOAuthSession(state, "antigravity")

	cleanup, errSetup := h.setupCallbackForwarder(c, antigravity.CallbackPort, "antigravity", "/antigravity/callback")
	if errSetup != nil {
		log.WithError(errSetup).Error("failed to setup antigravity callback forwarder")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "callback server unavailable"})
		return
	}

	go func() {
		defer cleanup()

		payload, errWait := h.waitForOAuthCallback(state, "antigravity", 5*time.Minute)
		if errWait != nil {
			log.Error("oauth flow timed out or cancelled")
			SetOAuthSessionError(state, "OAuth flow timed out")
			return
		}
		if errStr := strings.TrimSpace(payload["error"]); errStr != "" {
			log.Errorf("Authentication failed: %s", errStr)
			SetOAuthSessionError(state, "Authentication failed")
			return
		}
		if payloadState := strings.TrimSpace(payload["state"]); payloadState != "" && payloadState != state {
			log.Errorf("Authentication failed: state mismatch")
			SetOAuthSessionError(state, "Authentication failed: state mismatch")
			return
		}
		authCode := strings.TrimSpace(payload["code"])
		if authCode == "" {
			log.Error("Authentication failed: code not found")
			SetOAuthSessionError(state, "Authentication failed: code not found")
			return
		}

		tokenResp, errToken := authSvc.ExchangeCodeForTokens(ctx, authCode, redirectURI)
		if errToken != nil {
			log.Errorf("Failed to exchange token: %v", errToken)
			SetOAuthSessionError(state, "Failed to exchange token")
			return
		}

		accessToken := strings.TrimSpace(tokenResp.AccessToken)
		if accessToken == "" {
			log.Error("antigravity: token exchange returned empty access token")
			SetOAuthSessionError(state, "Failed to exchange token")
			return
		}

		email, errInfo := authSvc.FetchUserInfo(ctx, accessToken)
		if errInfo != nil {
			log.Errorf("Failed to fetch user info: %v", errInfo)
			SetOAuthSessionError(state, "Failed to fetch user info")
			return
		}
		email = strings.TrimSpace(email)
		if email == "" {
			log.Error("antigravity: user info returned empty email")
			SetOAuthSessionError(state, "Failed to fetch user info")
			return
		}

		projectID := ""
		if accessToken != "" {
			fetchedProjectID, errProject := authSvc.FetchProjectID(ctx, accessToken)
			if errProject != nil {
				log.Warnf("antigravity: failed to fetch project ID: %v", errProject)
			} else {
				projectID = fetchedProjectID
				log.Infof("antigravity: obtained project ID %s", projectID)
			}
		}

		now := time.Now()
		metadata := map[string]any{
			"type":          "antigravity",
			"access_token":  tokenResp.AccessToken,
			"refresh_token": tokenResp.RefreshToken,
			"expires_in":    tokenResp.ExpiresIn,
			"timestamp":     now.UnixMilli(),
			"expired":       now.Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Format(time.RFC3339),
		}
		if email != "" {
			metadata["email"] = email
		}
		if projectID != "" {
			metadata["project_id"] = projectID
		}

		fileName := antigravity.CredentialFileName(email)
		label := strings.TrimSpace(email)
		if label == "" {
			label = "antigravity"
		}

		record := &coreauth.Auth{
			ID:       fileName,
			Provider: "antigravity",
			FileName: fileName,
			Label:    label,
			Metadata: metadata,
		}

		successMsg := "Authentication successful!"
		if projectID != "" {
			successMsg += fmt.Sprintf(" Using GCP project: %s.", projectID)
		}
		successMsg += " You can now use Antigravity services through this CLI."
		if errComplete := h.saveAndCompleteAuth(ctx, state, "antigravity", record, successMsg); errComplete != nil {
			log.Errorf("Failed to complete antigravity auth: %v", errComplete)
		}
	}()

	c.JSON(200, gin.H{"status": "ok", "url": authURL, "state": state})
}
