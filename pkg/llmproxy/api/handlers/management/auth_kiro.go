package management

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	kiroauth "github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/auth/kiro"
	coreauth "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

func (h *Handler) RequestKiroToken(c *gin.Context) {
	ctx := context.Background()

	// Get the login method from query parameter (default: aws for device code flow)
	method := strings.ToLower(strings.TrimSpace(c.Query("method")))
	if method == "" {
		method = "aws"
	}

	fmt.Println("Initializing Kiro authentication...")

	state := fmt.Sprintf("kiro-%d", time.Now().UnixNano())

	switch method {
	case "aws", "builder-id":
		RegisterOAuthSession(state, "kiro")

		// AWS Builder ID uses device code flow (no callback needed)
		go func() {
			ssoClient := kiroauth.NewSSOOIDCClient(h.cfg)

			// Step 1: Register client
			fmt.Println("Registering client...")
			regResp, errRegister := ssoClient.RegisterClient(ctx)
			if errRegister != nil {
				log.Errorf("Failed to register client: %v", errRegister)
				SetOAuthSessionError(state, "Failed to register client")
				return
			}

			// Step 2: Start device authorization
			fmt.Println("Starting device authorization...")
			authResp, errAuth := ssoClient.StartDeviceAuthorization(ctx, regResp.ClientID, regResp.ClientSecret)
			if errAuth != nil {
				log.Errorf("Failed to start device auth: %v", errAuth)
				SetOAuthSessionError(state, "Failed to start device authorization")
				return
			}

			// Store the verification URL for the frontend to display.
			// Using "|" as separator because URLs contain ":".
			SetOAuthSessionError(state, "device_code|"+authResp.VerificationURIComplete+"|"+authResp.UserCode)

			// Step 3: Poll for token
			fmt.Println("Waiting for authorization...")
			interval := 5 * time.Second
			if authResp.Interval > 0 {
				interval = time.Duration(authResp.Interval) * time.Second
			}
			deadline := time.Now().Add(time.Duration(authResp.ExpiresIn) * time.Second)

			for time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					SetOAuthSessionError(state, "Authorization cancelled")
					return
				case <-time.After(interval):
					tokenResp, errToken := ssoClient.CreateToken(ctx, regResp.ClientID, regResp.ClientSecret, authResp.DeviceCode)
					if errToken != nil {
						errStr := errToken.Error()
						if strings.Contains(errStr, "authorization_pending") {
							continue
						}
						if strings.Contains(errStr, "slow_down") {
							interval += 5 * time.Second
							continue
						}
						log.Errorf("Token creation failed: %v", errToken)
						SetOAuthSessionError(state, "Token creation failed")
						return
					}

					// Success! Save the token
					expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
					email := kiroauth.ExtractEmailFromJWT(tokenResp.AccessToken)

					idPart := kiroauth.SanitizeEmailForFilename(email)
					if idPart == "" {
						idPart = fmt.Sprintf("%d", time.Now().UnixNano()%100000)
					}

					now := time.Now()
					fileName := fmt.Sprintf("kiro-aws-%s.json", idPart)

					record := &coreauth.Auth{
						ID:       fileName,
						Provider: "kiro",
						FileName: fileName,
						Metadata: map[string]any{
							"type":          "kiro",
							"access_token":  tokenResp.AccessToken,
							"refresh_token": tokenResp.RefreshToken,
							"expires_at":    expiresAt.Format(time.RFC3339),
							"auth_method":   "builder-id",
							"provider":      "AWS",
							"client_id":     regResp.ClientID,
							"client_secret": regResp.ClientSecret,
							"email":         email,
							"last_refresh":  now.Format(time.RFC3339),
						},
					}

					successMsg := "Authentication successful!"
					if email != "" {
						successMsg += fmt.Sprintf(" Authenticated as: %s.", email)
					}
					if errComplete := h.saveAndCompleteAuth(ctx, state, "kiro", record, successMsg); errComplete != nil {
						log.Errorf("Failed to complete kiro aws auth: %v", errComplete)
					}
					return
				}
			}

			SetOAuthSessionError(state, "Authorization timed out")
		}()

		// Return immediately with the state for polling
		c.JSON(http.StatusOK, gin.H{"status": "ok", "state": state, "method": "device_code"})

	case "google", "github":
		RegisterOAuthSession(state, "kiro")

		// Social auth uses protocol handler - for WEB UI we use a callback forwarder
		provider := "Google"
		if method == "github" {
			provider = "Github"
		}

		cleanup, errSetup := h.setupCallbackForwarder(c, kiroCallbackPort, "kiro", "/kiro/callback")
		if errSetup != nil {
			log.WithError(errSetup).Error("failed to setup kiro callback forwarder")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "callback server unavailable"})
			return
		}

		go func() {
			defer cleanup()

			socialClient := kiroauth.NewSocialAuthClient(h.cfg)

			// Generate PKCE codes
			codeVerifier, codeChallenge, errPKCE := generateKiroPKCE()
			if errPKCE != nil {
				log.Errorf("Failed to generate PKCE: %v", errPKCE)
				SetOAuthSessionError(state, "Failed to generate PKCE")
				return
			}

			// Build login URL
			authURL := fmt.Sprintf("%s/login?idp=%s&redirect_uri=%s&code_challenge=%s&code_challenge_method=S256&state=%s&prompt=select_account",
				"https://prod.us-east-1.auth.desktop.kiro.dev",
				provider,
				url.QueryEscape(kiroauth.KiroRedirectURI),
				codeChallenge,
				state,
			)

			// Store auth URL for frontend.
			// Using "|" as separator because URLs contain ":".
			SetOAuthSessionError(state, "auth_url|"+authURL)

			// Wait for callback file
			resultMap, errWait := h.waitForOAuthCallback(state, "kiro", 5*time.Minute)
			if errWait != nil {
				log.Error("oauth flow timed out or cancelled")
				SetOAuthSessionError(state, "OAuth flow timed out")
				return
			}
			if errStr := resultMap["error"]; errStr != "" {
				log.Errorf("Authentication failed: %s", errStr)
				SetOAuthSessionError(state, "Authentication failed")
				return
			}
			if resultMap["state"] != state {
				log.Errorf("State mismatch")
				SetOAuthSessionError(state, "State mismatch")
				return
			}
			code := resultMap["code"]
			if code == "" {
				log.Error("No authorization code received")
				SetOAuthSessionError(state, "No authorization code received")
				return
			}

			// Exchange code for tokens
			tokenReq := &kiroauth.CreateTokenRequest{
				Code:         code,
				CodeVerifier: codeVerifier,
				RedirectURI:  kiroauth.KiroRedirectURI,
			}

			tokenResp, errToken := socialClient.CreateToken(ctx, tokenReq)
			if errToken != nil {
				log.Errorf("Failed to exchange code for tokens: %v", errToken)
				SetOAuthSessionError(state, "Failed to exchange code for tokens")
				return
			}

			// Save the token
			expiresIn := tokenResp.ExpiresIn
			if expiresIn <= 0 {
				expiresIn = 3600
			}
			expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
			email := kiroauth.ExtractEmailFromJWT(tokenResp.AccessToken)

			idPart := kiroauth.SanitizeEmailForFilename(email)
			if idPart == "" {
				idPart = fmt.Sprintf("%d", time.Now().UnixNano()%100000)
			}

			now := time.Now()
			fileName := fmt.Sprintf("kiro-%s-%s.json", strings.ToLower(provider), idPart)

			record := &coreauth.Auth{
				ID:       fileName,
				Provider: "kiro",
				FileName: fileName,
				Metadata: map[string]any{
					"type":          "kiro",
					"access_token":  tokenResp.AccessToken,
					"refresh_token": tokenResp.RefreshToken,
					"profile_arn":   tokenResp.ProfileArn,
					"expires_at":    expiresAt.Format(time.RFC3339),
					"auth_method":   "social",
					"provider":      provider,
					"email":         email,
					"last_refresh":  now.Format(time.RFC3339),
				},
			}

			successMsg := "Authentication successful!"
			if email != "" {
				successMsg += fmt.Sprintf(" Authenticated as: %s.", email)
			}
			if errComplete := h.saveAndCompleteAuth(ctx, state, "kiro", record, successMsg); errComplete != nil {
				log.Errorf("Failed to complete kiro social auth: %v", errComplete)
			}
		}()

		c.JSON(http.StatusOK, gin.H{"status": "ok", "state": state, "method": "social"})

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid method, use 'aws', 'google', or 'github'"})
	}
}

func generateKiroPKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, errRead := io.ReadFull(rand.Reader, b); errRead != nil {
		return "", "", fmt.Errorf("failed to generate random bytes: %w", errRead)
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)

	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])

	return verifier, challenge, nil
}
