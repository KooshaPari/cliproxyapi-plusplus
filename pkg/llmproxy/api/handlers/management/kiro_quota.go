package management

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	kiroauth "github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/auth/kiro"
	coreauth "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/cliproxy/auth"
)

type kiroUsageChecker interface {
	CheckUsageByAccessToken(ctx context.Context, accessToken, profileArn string) (*kiroauth.UsageQuotaResponse, error)
}

type kiroQuotaResponse struct {
	AuthIndex       string                       `json:"auth_index,omitempty"`
	ProfileARN      string                       `json:"profile_arn"`
	RemainingQuota  float64                      `json:"remaining_quota"`
	UsagePercentage float64                      `json:"usage_percentage"`
	QuotaExhausted  bool                         `json:"quota_exhausted"`
	Usage           *kiroauth.UsageQuotaResponse `json:"usage"`
}

// GetKiroQuota fetches Kiro quota information from CodeWhisperer usage API.
//
// Endpoint:
//
//	GET /v0/management/kiro-quota
//
// Query Parameters (optional):
//   - auth_index: The credential "auth_index" from GET /v0/management/auth-files.
//     If omitted, uses the first available Kiro credential.
func (h *Handler) GetKiroQuota(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "management config unavailable"})
		return
	}
	h.getKiroQuotaWithChecker(c, kiroauth.NewUsageChecker(h.cfg))
}

func (h *Handler) getKiroQuotaWithChecker(c *gin.Context, checker kiroUsageChecker) {
	authIndex := firstNonEmptyQuery(c, "auth_index", "authIndex", "AuthIndex", "index", "auth_id", "auth-id")

	auth := h.findKiroAuth(authIndex)
	if auth == nil {
		if authIndex != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no kiro credential found", "auth_index": authIndex})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "no kiro credential found"})
		return
	}
	auth.EnsureIndex()

	token, tokenErr := h.resolveTokenForAuth(c.Request.Context(), auth)
	if tokenErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to resolve kiro token", "auth_index": auth.Index, "detail": tokenErr.Error()})
		return
	}
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kiro token not found", "auth_index": auth.Index})
		return
	}

	profileARN := profileARNForAuth(auth)
	if profileARN == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kiro profile arn not found", "auth_index": auth.Index})
		return
	}

	usage, err := checker.CheckUsageByAccessToken(c.Request.Context(), token, profileARN)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "kiro quota request failed", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, kiroQuotaResponse{
		AuthIndex:       auth.Index,
		ProfileARN:      profileARN,
		RemainingQuota:  kiroauth.GetRemainingQuota(usage),
		UsagePercentage: kiroauth.GetUsagePercentage(usage),
		QuotaExhausted:  kiroauth.IsQuotaExhausted(usage),
		Usage:           usage,
	})
}

// findKiroAuth locates a Kiro credential by auth_index or returns the first available one.
func (h *Handler) findKiroAuth(authIndex string) *coreauth.Auth {
	if h == nil || h.authManager == nil {
		return nil
	}

	auths := h.authManager.List()
	var firstKiro *coreauth.Auth

	for _, auth := range auths {
		if auth == nil {
			continue
		}
		if strings.ToLower(strings.TrimSpace(auth.Provider)) != "kiro" {
			continue
		}

		if firstKiro == nil {
			firstKiro = auth
		}

		if authIndex != "" {
			auth.EnsureIndex()
			if auth.Index == authIndex || auth.ID == authIndex || auth.FileName == authIndex {
				return auth
			}
		}
	}

	return firstKiro
}
