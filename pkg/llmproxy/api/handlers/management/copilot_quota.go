package management

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	coreauth "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/cliproxy/auth"
)

// QuotaDetail represents quota information for a specific resource type
type QuotaDetail struct {
	Entitlement      float64 `json:"entitlement"`
	OverageCount     float64 `json:"overage_count"`
	OveragePermitted bool    `json:"overage_permitted"`
	PercentRemaining float64 `json:"percent_remaining"`
	QuotaID          string  `json:"quota_id"`
	QuotaRemaining   float64 `json:"quota_remaining"`
	Remaining        float64 `json:"remaining"`
	Unlimited        bool    `json:"unlimited"`
}

// QuotaSnapshots contains quota details for different resource types
type QuotaSnapshots struct {
	Chat                QuotaDetail `json:"chat"`
	Completions         QuotaDetail `json:"completions"`
	PremiumInteractions QuotaDetail `json:"premium_interactions"`
}

// CopilotUsageResponse represents the GitHub Copilot usage information
type CopilotUsageResponse struct {
	AccessTypeSKU         string         `json:"access_type_sku"`
	AnalyticsTrackingID   string         `json:"analytics_tracking_id"`
	AssignedDate          string         `json:"assigned_date"`
	CanSignupForLimited   bool           `json:"can_signup_for_limited"`
	ChatEnabled           bool           `json:"chat_enabled"`
	CopilotPlan           string         `json:"copilot_plan"`
	OrganizationLoginList []interface{}  `json:"organization_login_list"`
	OrganizationList      []interface{}  `json:"organization_list"`
	QuotaResetDate        string         `json:"quota_reset_date"`
	QuotaSnapshots        QuotaSnapshots `json:"quota_snapshots"`
}

// GetCopilotQuota fetches GitHub Copilot quota information from the /copilot_pkg/llmproxy/user endpoint.
//
// Endpoint:
//
//	GET /v0/management/copilot-quota
//
// Query Parameters (optional):
//   - auth_index: The credential "auth_index" from GET /v0/management/auth-files.
//     If omitted, uses the first available GitHub Copilot credential.
//
// Response:
//
//	Returns the CopilotUsageResponse with quota_snapshots containing detailed quota information
//	for chat, completions, and premium_interactions.
//
// Example:
//
//	curl -sS -X GET "http://127.0.0.1:8317/v0/management/copilot-quota?auth_index=<AUTH_INDEX>" \
//	  -H "Authorization: Bearer <MANAGEMENT_KEY>"
func (h *Handler) GetCopilotQuota(c *gin.Context) {
	authIndex := strings.TrimSpace(c.Query("auth_index"))
	if authIndex == "" {
		authIndex = strings.TrimSpace(c.Query("authIndex"))
	}
	if authIndex == "" {
		authIndex = strings.TrimSpace(c.Query("AuthIndex"))
	}

	auth := h.findCopilotAuth(authIndex)
	if auth == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no github copilot credential found"})
		return
	}

	token, tokenErr := h.resolveTokenForAuth(c.Request.Context(), auth)
	if tokenErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to refresh copilot token"})
		return
	}
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "copilot token not found"})
		return
	}

	apiURL := "https://api.github.com/copilot_pkg/llmproxy/user"
	req, errNewRequest := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, apiURL, nil)
	if errNewRequest != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build request"})
		return
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "cliproxyapi++")
	req.Header.Set("Accept", "application/json")

	httpClient := &http.Client{
		Timeout:   defaultAPICallTimeout,
		Transport: h.apiCallTransport(auth),
	}

	resp, errDo := httpClient.Do(req)
	if errDo != nil {
		log.WithError(errDo).Debug("copilot quota request failed")
		c.JSON(http.StatusBadGateway, gin.H{"error": "request failed"})
		return
	}
	defer func() {
		if errClose := resp.Body.Close(); errClose != nil {
			log.Errorf("response body close error: %v", errClose)
		}
	}()

	respBody, errReadAll := io.ReadAll(resp.Body)
	if errReadAll != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to read response"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{
			"error":       "github api request failed",
			"status_code": resp.StatusCode,
			"body":        string(respBody),
		})
		return
	}

	var usage CopilotUsageResponse
	if errUnmarshal := json.Unmarshal(respBody, &usage); errUnmarshal != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse response"})
		return
	}

	c.JSON(http.StatusOK, usage)
}

// findCopilotAuth locates a GitHub Copilot credential by auth_index or returns the first available one
func (h *Handler) findCopilotAuth(authIndex string) *coreauth.Auth {
	if h == nil || h.authManager == nil {
		return nil
	}

	auths := h.authManager.List()
	var firstCopilot *coreauth.Auth

	for _, auth := range auths {
		if auth == nil {
			continue
		}

		provider := strings.ToLower(strings.TrimSpace(auth.Provider))
		if provider != "copilot" && provider != "github" && provider != "github-copilot" {
			continue
		}

		if firstCopilot == nil {
			firstCopilot = auth
		}

		if authIndex != "" {
			auth.EnsureIndex()
			if auth.Index == authIndex {
				return auth
			}
		}
	}

	return firstCopilot
}

// enrichCopilotTokenResponse fetches quota information and adds it to the Copilot token response body
func (h *Handler) enrichCopilotTokenResponse(ctx context.Context, response apiCallResponse, auth *coreauth.Auth, originalURL string) apiCallResponse {
	if auth == nil || response.Body == "" {
		return response
	}

	// Parse the token response to check if it's enterprise (null limited_user_quotas)
	var tokenResp map[string]interface{}
	if err := json.Unmarshal([]byte(response.Body), &tokenResp); err != nil {
		log.WithError(err).Debug("enrichCopilotTokenResponse: failed to parse copilot token response")
		return response
	}

	// Get the GitHub token to call the copilot_pkg/llmproxy/user endpoint
	token, tokenErr := h.resolveTokenForAuth(ctx, auth)
	if tokenErr != nil {
		log.WithError(tokenErr).Debug("enrichCopilotTokenResponse: failed to resolve token")
		return response
	}
	if token == "" {
		return response
	}

	// Fetch quota information from /copilot_pkg/llmproxy/user
	// Derive the base URL from the original token request to support proxies and test servers
	quotaURL, errQuotaURL := copilotQuotaURLFromTokenURL(originalURL)
	if errQuotaURL != nil {
		log.WithError(errQuotaURL).Debug("enrichCopilotTokenResponse: rejected token URL for quota request")
		return response
	}
	parsedQuotaURL, errParseQuotaURL := url.Parse(quotaURL)
	if errParseQuotaURL != nil {
		return response
	}
	if errValidate := validateAPICallURL(parsedQuotaURL); errValidate != nil {
		return response
	}
	if errResolve := validateResolvedHostIPs(parsedQuotaURL.Hostname()); errResolve != nil {
		return response
	}

	req, errNewRequest := http.NewRequestWithContext(ctx, http.MethodGet, quotaURL, nil)
	if errNewRequest != nil {
		log.WithError(errNewRequest).Debug("enrichCopilotTokenResponse: failed to build request")
		return response
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "cliproxyapi++")
	req.Header.Set("Accept", "application/json")

	httpClient := &http.Client{
		Timeout:   defaultAPICallTimeout,
		Transport: h.apiCallTransport(auth),
	}

	quotaResp, errDo := httpClient.Do(req)
	if errDo != nil {
		log.WithError(errDo).Debug("enrichCopilotTokenResponse: quota fetch HTTP request failed")
		return response
	}

	defer func() {
		if errClose := quotaResp.Body.Close(); errClose != nil {
			log.Errorf("quota response body close error: %v", errClose)
		}
	}()

	if quotaResp.StatusCode != http.StatusOK {
		return response
	}

	quotaBody, errReadAll := io.ReadAll(quotaResp.Body)
	if errReadAll != nil {
		log.WithError(errReadAll).Debug("enrichCopilotTokenResponse: failed to read response")
		return response
	}

	// Parse the quota response
	var quotaData CopilotUsageResponse
	if err := json.Unmarshal(quotaBody, &quotaData); err != nil {
		log.WithError(err).Debug("enrichCopilotTokenResponse: failed to parse response")
		return response
	}

	// Check if this is an enterprise account by looking for quota_snapshots in the response
	// Enterprise accounts have quota_snapshots, non-enterprise have limited_user_quotas
	var quotaRaw map[string]interface{}
	if err := json.Unmarshal(quotaBody, &quotaRaw); err == nil {
		if _, hasQuotaSnapshots := quotaRaw["quota_snapshots"]; hasQuotaSnapshots {
			// Enterprise account - has quota_snapshots
			tokenResp["quota_snapshots"] = quotaData.QuotaSnapshots
			tokenResp["access_type_sku"] = quotaData.AccessTypeSKU
			tokenResp["copilot_plan"] = quotaData.CopilotPlan

			// Add quota reset date for enterprise (quota_reset_date_utc)
			if quotaResetDateUTC, ok := quotaRaw["quota_reset_date_utc"]; ok {
				tokenResp["quota_reset_date"] = quotaResetDateUTC
			} else if quotaData.QuotaResetDate != "" {
				tokenResp["quota_reset_date"] = quotaData.QuotaResetDate
			}
		} else {
			// Non-enterprise account - build quota from limited_user_quotas and monthly_quotas
			var quotaSnapshots QuotaSnapshots

			// Get monthly quotas (total entitlement) and limited_user_quotas (remaining)
			monthlyQuotas, hasMonthly := quotaRaw["monthly_quotas"].(map[string]interface{})
			limitedQuotas, hasLimited := quotaRaw["limited_user_quotas"].(map[string]interface{})

			// Process chat quota
			if hasMonthly && hasLimited {
				if chatTotal, ok := monthlyQuotas["chat"].(float64); ok {
					chatRemaining := chatTotal // default to full if no limited quota
					if chatLimited, ok := limitedQuotas["chat"].(float64); ok {
						chatRemaining = chatLimited
					}
					percentRemaining := 0.0
					if chatTotal > 0 {
						percentRemaining = (chatRemaining / chatTotal) * 100.0
					}
					quotaSnapshots.Chat = QuotaDetail{
						Entitlement:      chatTotal,
						Remaining:        chatRemaining,
						QuotaRemaining:   chatRemaining,
						PercentRemaining: percentRemaining,
						QuotaID:          "chat",
						Unlimited:        false,
					}
				}

				// Process completions quota
				if completionsTotal, ok := monthlyQuotas["completions"].(float64); ok {
					completionsRemaining := completionsTotal // default to full if no limited quota
					if completionsLimited, ok := limitedQuotas["completions"].(float64); ok {
						completionsRemaining = completionsLimited
					}
					percentRemaining := 0.0
					if completionsTotal > 0 {
						percentRemaining = (completionsRemaining / completionsTotal) * 100.0
					}
					quotaSnapshots.Completions = QuotaDetail{
						Entitlement:      completionsTotal,
						Remaining:        completionsRemaining,
						QuotaRemaining:   completionsRemaining,
						PercentRemaining: percentRemaining,
						QuotaID:          "completions",
						Unlimited:        false,
					}
				}
			}

			// Premium interactions don't exist for non-enterprise, leave as zero values
			quotaSnapshots.PremiumInteractions = QuotaDetail{
				QuotaID:   "premium_interactions",
				Unlimited: false,
			}

			// Add quota_snapshots to the token response
			tokenResp["quota_snapshots"] = quotaSnapshots
			tokenResp["access_type_sku"] = quotaData.AccessTypeSKU
			tokenResp["copilot_plan"] = quotaData.CopilotPlan

			// Add quota reset date for non-enterprise (limited_user_reset_date)
			if limitedResetDate, ok := quotaRaw["limited_user_reset_date"]; ok {
				tokenResp["quota_reset_date"] = limitedResetDate
			}
		}
	}

	// Re-serialize the enriched response
	enrichedBody, errMarshal := json.Marshal(tokenResp)
	if errMarshal != nil {
		log.WithError(errMarshal).Debug("failed to marshal enriched response")
		return response
	}

	response.Body = string(enrichedBody)

	return response
}
