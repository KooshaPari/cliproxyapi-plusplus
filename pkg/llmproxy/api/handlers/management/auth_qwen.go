package management

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/auth/qwen"
	coreauth "github.com/kooshapari/CLIProxyAPI/v7/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

func (h *Handler) RequestQwenToken(c *gin.Context) {
	ctx := context.Background()

	fmt.Println("Initializing Qwen authentication...")

	state := fmt.Sprintf("gem-%d", time.Now().UnixNano())
	// Initialize Qwen auth service
	qwenAuth := qwen.NewQwenAuth(h.cfg, http.DefaultClient)

	// Generate authorization URL
	deviceFlow, err := qwenAuth.InitiateDeviceFlow(ctx)
	if err != nil {
		log.Errorf("Failed to generate authorization URL: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate authorization url"})
		return
	}
	authURL := deviceFlow.VerificationURIComplete

	RegisterOAuthSession(state, "qwen")

	go func() {
		fmt.Println("Waiting for authentication...")
		tokenData, errPollForToken := qwenAuth.PollForToken(deviceFlow.DeviceCode, deviceFlow.CodeVerifier)
		if errPollForToken != nil {
			SetOAuthSessionError(state, "Authentication failed")
			fmt.Printf("Authentication failed: %v\n", errPollForToken)
			return
		}

		// Create token storage
		tokenStorage := qwenAuth.CreateTokenStorage(tokenData)

		tokenStorage.Email = fmt.Sprintf("%d", time.Now().UnixMilli())
		record := &coreauth.Auth{
			ID:       fmt.Sprintf("qwen-%s.json", tokenStorage.Email),
			Provider: "qwen",
			FileName: fmt.Sprintf("qwen-%s.json", tokenStorage.Email),
			Storage:  tokenStorage,
			Metadata: map[string]any{"email": tokenStorage.Email},
		}

		if errComplete := h.saveAndCompleteAuth(ctx, state, "qwen", record, "Authentication successful! You can now use Qwen services through this CLI."); errComplete != nil {
			log.Errorf("Failed to complete qwen auth: %v", errComplete)
		}
	}()

	c.JSON(200, gin.H{"status": "ok", "url": authURL, "state": state})
}
