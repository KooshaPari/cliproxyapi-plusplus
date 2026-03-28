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
	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/auth/codex"
	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/misc"
	coreauth "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

func (h *Handler) RequestCodexToken(c *gin.Context) {
	ctx := context.Background()

	fmt.Println("Initializing Codex authentication...")

	// Generate PKCE codes
	pkceCodes, err := codex.GeneratePKCECodes()
	if err != nil {
		log.Errorf("Failed to generate PKCE codes: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate PKCE codes"})
		return
	}

	// Generate random state parameter
	state, err := misc.GenerateRandomState()
	if err != nil {
		log.Errorf("Failed to generate state parameter: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate state parameter"})
		return
	}

	// Initialize Codex auth service
	openaiAuth := codex.NewCodexAuth(h.cfg)

	// Generate authorization URL
	authURL, err := openaiAuth.GenerateAuthURL(state, pkceCodes)
	if err != nil {
		log.Errorf("Failed to generate authorization URL: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate authorization url"})
		return
	}

	RegisterOAuthSession(state, "codex")

	cleanup, errSetup := h.setupCallbackForwarder(c, codexCallbackPort, "codex", "/codex/callback")
	if errSetup != nil {
		log.WithError(errSetup).Error("failed to setup codex callback forwarder")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "callback server unavailable"})
		return
	}

	go func() {
		defer cleanup()

		resultMap, errWait := h.waitForOAuthCallback(state, "codex", 5*time.Minute)
		if errWait != nil {
			authErr := codex.NewAuthenticationError(codex.ErrCallbackTimeout, errWait)
			log.Error(codex.GetUserFriendlyMessage(authErr))
			return
		}
		if errStr := resultMap["error"]; errStr != "" {
			oauthErr := codex.NewOAuthError(errStr, "", http.StatusBadRequest)
			log.Error(codex.GetUserFriendlyMessage(oauthErr))
			SetOAuthSessionError(state, "Bad Request")
			return
		}
		if resultMap["state"] != state {
			authErr := codex.NewAuthenticationError(codex.ErrInvalidState, fmt.Errorf("expected %s, got %s", state, resultMap["state"]))
			SetOAuthSessionError(state, "State code error")
			log.Error(codex.GetUserFriendlyMessage(authErr))
			return
		}

		code := resultMap["code"]

		log.Debug("Authorization code received, exchanging for tokens...")
		// Exchange code for tokens using internal auth service
		bundle, errExchange := openaiAuth.ExchangeCodeForTokens(ctx, code, pkceCodes)
		if errExchange != nil {
			authErr := codex.NewAuthenticationError(codex.ErrCodeExchangeFailed, errExchange)
			SetOAuthSessionError(state, "Failed to exchange authorization code for tokens")
			log.Errorf("Failed to exchange authorization code for tokens: %v", authErr)
			return
		}

		// Extract additional info for filename generation
		claims, _ := codex.ParseJWTToken(bundle.TokenData.IDToken)
		planType := ""
		hashAccountID := ""
		if claims != nil {
			planType = strings.TrimSpace(claims.CodexAuthInfo.ChatgptPlanType)
			if accountID := claims.GetAccountID(); accountID != "" {
				digest := sha256.Sum256([]byte(accountID))
				hashAccountID = hex.EncodeToString(digest[:])[:8]
			}
		}

		// Create token storage and persist
		tokenStorage := openaiAuth.CreateTokenStorage(bundle)
		fileName := codex.CredentialFileName(tokenStorage.Email, planType, hashAccountID, true)
		record := &coreauth.Auth{
			ID:       fileName,
			Provider: "codex",
			FileName: fileName,
			Storage:  tokenStorage,
			Metadata: map[string]any{
				"email":      tokenStorage.Email,
				"account_id": tokenStorage.AccountID,
			},
		}

		successMsg := "Authentication successful!"
		if bundle.APIKey != "" {
			successMsg += " API key obtained and saved."
		}
		successMsg += " You can now use Codex services through this CLI."
		if errComplete := h.saveAndCompleteAuth(ctx, state, "codex", record, successMsg); errComplete != nil {
			log.Errorf("Failed to complete codex auth: %v", errComplete)
		}
	}()

	c.JSON(200, gin.H{"status": "ok", "url": authURL, "state": state})
}
