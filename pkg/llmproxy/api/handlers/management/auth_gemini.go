package management

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/auth/base"
	geminiAuth "github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/auth/gemini"
	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/interfaces"
	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/util"
	coreauth "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func (h *Handler) RequestGeminiCLIToken(c *gin.Context) {
	ctx := context.Background()
	proxyHTTPClient := util.SetProxy(&h.cfg.SDKConfig, &http.Client{})
	ctx = context.WithValue(ctx, oauth2.HTTPClient, proxyHTTPClient)

	// Optional project ID from query
	projectID := c.Query("project_id")

	fmt.Println("Initializing Google authentication...")

	// OAuth2 configuration using exported constants from pkg/llmproxy/auth/gemini
	conf := &oauth2.Config{
		ClientID:     geminiAuth.ClientID,
		ClientSecret: geminiAuth.ClientSecret,
		RedirectURL:  fmt.Sprintf("http://localhost:%d/oauth2callback", geminiAuth.DefaultCallbackPort),
		Scopes:       geminiAuth.Scopes,
		Endpoint:     google.Endpoint,
	}

	// Build authorization URL and return it immediately
	state := fmt.Sprintf("gem-%d", time.Now().UnixNano())
	authURL := conf.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))

	RegisterOAuthSession(state, "gemini")

	cleanup, errSetup := h.setupCallbackForwarder(c, geminiCallbackPort, "gemini", "/google/callback")
	if errSetup != nil {
		log.WithError(errSetup).Error("failed to setup gemini callback forwarder")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "callback server unavailable"})
		return
	}

	go func() {
		defer cleanup()

		fmt.Println("Waiting for authentication callback...")
		resultMap, errWait := h.waitForOAuthCallback(state, "gemini", 5*time.Minute)
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
		authCode := resultMap["code"]
		if authCode == "" {
			log.Errorf("Authentication failed: code not found")
			SetOAuthSessionError(state, "Authentication failed: code not found")
			return
		}

		// Exchange authorization code for token
		token, err := conf.Exchange(ctx, authCode)
		if err != nil {
			log.Errorf("Failed to exchange token: %v", err)
			SetOAuthSessionError(state, "Failed to exchange token")
			return
		}

		requestedProjectID := strings.TrimSpace(projectID)

		// Create token storage (mirrors pkg/llmproxy/auth/gemini createTokenStorage)
		authHTTPClient := conf.Client(ctx, token)
		req, errNewRequest := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v1/userinfo?alt=json", nil)
		if errNewRequest != nil {
			log.Errorf("Could not get user info: %v", errNewRequest)
			SetOAuthSessionError(state, "Could not get user info")
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))

		resp, errDo := authHTTPClient.Do(req)
		if errDo != nil {
			log.Errorf("Failed to execute request: %v", errDo)
			SetOAuthSessionError(state, "Failed to execute request")
			return
		}
		defer func() {
			if errClose := resp.Body.Close(); errClose != nil {
				log.Printf("warn: failed to close response body: %v", errClose)
			}
		}()

		bodyBytes, _ := io.ReadAll(resp.Body)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			log.Errorf("Get user info request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
			SetOAuthSessionError(state, fmt.Sprintf("Get user info request failed with status %d", resp.StatusCode))
			return
		}

		email := gjson.GetBytes(bodyBytes, "email").String()
		if email != "" {
			fmt.Printf("Authenticated user email: %s\n", email)
		} else {
			fmt.Println("Failed to get user email from token")
		}

		// Marshal/unmarshal oauth2.Token to generic map and enrich fields
		var ifToken map[string]any
		jsonData, _ := json.Marshal(token)
		if errUnmarshal := json.Unmarshal(jsonData, &ifToken); errUnmarshal != nil {
			log.Errorf("Failed to unmarshal token: %v", errUnmarshal)
			SetOAuthSessionError(state, "Failed to unmarshal token")
			return
		}

		ifToken["token_uri"] = "https://oauth2.googleapis.com/token"
		ifToken["client_id"] = geminiAuth.ClientID
		ifToken["client_secret"] = geminiAuth.ClientSecret
		ifToken["scopes"] = geminiAuth.Scopes
		ifToken["universe_domain"] = "googleapis.com"

		ts := geminiAuth.GeminiTokenStorage{
			BaseTokenStorage: base.BaseTokenStorage{
				Email: email,
			},
			Token:     ifToken,
			ProjectID: requestedProjectID,
			Auto:      requestedProjectID == "",
		}
		ts.Email = email

		// Initialize authenticated HTTP client via GeminiAuth to honor proxy settings
		gemAuth := geminiAuth.NewGeminiAuth()
		gemClient, errGetClient := gemAuth.GetAuthenticatedClient(ctx, &ts, h.cfg, &geminiAuth.WebLoginOptions{
			NoBrowser: true,
		})
		if errGetClient != nil {
			log.Errorf("failed to get authenticated client: %v", errGetClient)
			SetOAuthSessionError(state, "Failed to get authenticated client")
			return
		}
		fmt.Println("Authentication successful.")

		if strings.EqualFold(requestedProjectID, "ALL") {
			ts.Auto = false
			projects, errAll := onboardAllGeminiProjects(ctx, gemClient, &ts)
			if errAll != nil {
				log.Errorf("Failed to complete Gemini CLI onboarding: %v", errAll)
				SetOAuthSessionError(state, "Failed to complete Gemini CLI onboarding")
				return
			}
			if errVerify := ensureGeminiProjectsEnabled(ctx, gemClient, projects); errVerify != nil {
				log.Errorf("Failed to verify Cloud AI API status: %v", errVerify)
				SetOAuthSessionError(state, "Failed to verify Cloud AI API status")
				return
			}
			ts.ProjectID = strings.Join(projects, ",")
			ts.Checked = true
		} else if strings.EqualFold(requestedProjectID, "GOOGLE_ONE") {
			ts.Auto = false
			if errSetup := performGeminiCLISetup(ctx, gemClient, &ts, ""); errSetup != nil {
				log.Errorf("Google One auto-discovery failed: %v", errSetup)
				SetOAuthSessionError(state, "Google One auto-discovery failed")
				return
			}
			if strings.TrimSpace(ts.ProjectID) == "" {
				log.Error("Google One auto-discovery returned empty project ID")
				SetOAuthSessionError(state, "Google One auto-discovery returned empty project ID")
				return
			}
			isChecked, errCheck := checkCloudAPIIsEnabled(ctx, gemClient, ts.ProjectID)
			if errCheck != nil {
				log.Errorf("Failed to verify Cloud AI API status: %v", errCheck)
				SetOAuthSessionError(state, "Failed to verify Cloud AI API status")
				return
			}
			ts.Checked = isChecked
			if !isChecked {
				log.Error("Cloud AI API is not enabled for the auto-discovered project")
				SetOAuthSessionError(state, "Cloud AI API not enabled")
				return
			}
		} else {
			if errEnsure := ensureGeminiProjectAndOnboard(ctx, gemClient, &ts, requestedProjectID); errEnsure != nil {
				log.Errorf("Failed to complete Gemini CLI onboarding: %v", errEnsure)
				SetOAuthSessionError(state, "Failed to complete Gemini CLI onboarding")
				return
			}

			if strings.TrimSpace(ts.ProjectID) == "" {
				log.Error("Onboarding did not return a project ID")
				SetOAuthSessionError(state, "Failed to resolve project ID")
				return
			}

			isChecked, errCheck := checkCloudAPIIsEnabled(ctx, gemClient, ts.ProjectID)
			if errCheck != nil {
				log.Errorf("Failed to verify Cloud AI API status: %v", errCheck)
				SetOAuthSessionError(state, "Failed to verify Cloud AI API status")
				return
			}
			ts.Checked = isChecked
			if !isChecked {
				log.Error("Cloud AI API is not enabled for the selected project")
				SetOAuthSessionError(state, "Cloud AI API not enabled")
				return
			}
		}

		recordMetadata := map[string]any{
			"email":      ts.Email,
			"project_id": ts.ProjectID,
			"auto":       ts.Auto,
			"checked":    ts.Checked,
		}

		fileName := geminiAuth.CredentialFileName(ts.Email, ts.ProjectID, true)
		record := &coreauth.Auth{
			ID:       fileName,
			Provider: "gemini",
			FileName: fileName,
			Storage:  &ts,
			Metadata: recordMetadata,
		}

		if errComplete := h.saveAndCompleteAuth(ctx, state, "gemini", record, "You can now use Gemini CLI services through this CLI;"); errComplete != nil {
			log.Errorf("Failed to complete gemini auth: %v", errComplete)
		}
	}()

	c.JSON(200, gin.H{"status": "ok", "url": authURL, "state": state})
}

type projectSelectionRequiredError struct{}

func (e *projectSelectionRequiredError) Error() string {
	return "gemini cli: project selection required"
}

func ensureGeminiProjectAndOnboard(ctx context.Context, httpClient *http.Client, storage *geminiAuth.GeminiTokenStorage, requestedProject string) error {
	if storage == nil {
		return fmt.Errorf("gemini storage is nil")
	}

	trimmedRequest := strings.TrimSpace(requestedProject)
	if trimmedRequest == "" {
		projects, errProjects := fetchGCPProjects(ctx, httpClient)
		if errProjects != nil {
			return fmt.Errorf("fetch project list: %w", errProjects)
		}
		if len(projects) == 0 {
			return fmt.Errorf("no Google Cloud projects available for this account")
		}
		trimmedRequest = strings.TrimSpace(projects[0].ProjectID)
		if trimmedRequest == "" {
			return fmt.Errorf("resolved project id is empty")
		}
		storage.Auto = true
	} else {
		storage.Auto = false
	}

	if err := performGeminiCLISetup(ctx, httpClient, storage, trimmedRequest); err != nil {
		return err
	}

	if strings.TrimSpace(storage.ProjectID) == "" {
		storage.ProjectID = trimmedRequest
	}

	return nil
}

func onboardAllGeminiProjects(ctx context.Context, httpClient *http.Client, storage *geminiAuth.GeminiTokenStorage) ([]string, error) {
	projects, errProjects := fetchGCPProjects(ctx, httpClient)
	if errProjects != nil {
		return nil, fmt.Errorf("fetch project list: %w", errProjects)
	}
	if len(projects) == 0 {
		return nil, fmt.Errorf("no Google Cloud projects available for this account")
	}
	activated := make([]string, 0, len(projects))
	seen := make(map[string]struct{}, len(projects))
	for _, project := range projects {
		candidate := strings.TrimSpace(project.ProjectID)
		if candidate == "" {
			continue
		}
		if _, dup := seen[candidate]; dup {
			continue
		}
		if err := performGeminiCLISetup(ctx, httpClient, storage, candidate); err != nil {
			return nil, fmt.Errorf("onboard project %s: %w", candidate, err)
		}
		finalID := strings.TrimSpace(storage.ProjectID)
		if finalID == "" {
			finalID = candidate
		}
		activated = append(activated, finalID)
		seen[candidate] = struct{}{}
	}
	if len(activated) == 0 {
		return nil, fmt.Errorf("no Google Cloud projects available for this account")
	}
	return activated, nil
}

func ensureGeminiProjectsEnabled(ctx context.Context, httpClient *http.Client, projectIDs []string) error {
	for _, pid := range projectIDs {
		trimmed := strings.TrimSpace(pid)
		if trimmed == "" {
			continue
		}
		isChecked, errCheck := checkCloudAPIIsEnabled(ctx, httpClient, trimmed)
		if errCheck != nil {
			return fmt.Errorf("project %s: %w", trimmed, errCheck)
		}
		if !isChecked {
			return fmt.Errorf("project %s: Cloud AI API not enabled", trimmed)
		}
	}
	return nil
}

func performGeminiCLISetup(ctx context.Context, httpClient *http.Client, storage *geminiAuth.GeminiTokenStorage, requestedProject string) error {
	metadata := map[string]string{
		"ideType":    "IDE_UNSPECIFIED",
		"platform":   "PLATFORM_UNSPECIFIED",
		"pluginType": "GEMINI",
	}

	trimmedRequest := strings.TrimSpace(requestedProject)
	explicitProject := trimmedRequest != ""

	loadReqBody := map[string]any{
		"metadata": metadata,
	}
	if explicitProject {
		loadReqBody["cloudaicompanionProject"] = trimmedRequest
	}

	var loadResp map[string]any
	if errLoad := callGeminiCLI(ctx, httpClient, "loadCodeAssist", loadReqBody, &loadResp); errLoad != nil {
		return fmt.Errorf("load code assist: %w", errLoad)
	}

	tierID := "legacy-tier"
	if tiers, okTiers := loadResp["allowedTiers"].([]any); okTiers {
		for _, rawTier := range tiers {
			tier, okTier := rawTier.(map[string]any)
			if !okTier {
				continue
			}
			if isDefault, okDefault := tier["isDefault"].(bool); okDefault && isDefault {
				if id, okID := tier["id"].(string); okID && strings.TrimSpace(id) != "" {
					tierID = strings.TrimSpace(id)
					break
				}
			}
		}
	}

	projectID := trimmedRequest
	if projectID == "" {
		if id, okProject := loadResp["cloudaicompanionProject"].(string); okProject {
			projectID = strings.TrimSpace(id)
		}
		if projectID == "" {
			if projectMap, okProject := loadResp["cloudaicompanionProject"].(map[string]any); okProject {
				if id, okID := projectMap["id"].(string); okID {
					projectID = strings.TrimSpace(id)
				}
			}
		}
	}
	if projectID == "" {
		// Auto-discovery: try onboardUser without specifying a project
		// to let Google auto-provision one (matches Gemini CLI headless behavior
		// and Antigravity's FetchProjectID pattern).
		autoOnboardReq := map[string]any{
			"tierId":   tierID,
			"metadata": metadata,
		}

		autoCtx, autoCancel := context.WithTimeout(ctx, 30*time.Second)
		defer autoCancel()
		for attempt := 1; ; attempt++ {
			var onboardResp map[string]any
			if errOnboard := callGeminiCLI(autoCtx, httpClient, "onboardUser", autoOnboardReq, &onboardResp); errOnboard != nil {
				return fmt.Errorf("auto-discovery onboardUser: %w", errOnboard)
			}

			if done, okDone := onboardResp["done"].(bool); okDone && done {
				if resp, okResp := onboardResp["response"].(map[string]any); okResp {
					switch v := resp["cloudaicompanionProject"].(type) {
					case string:
						projectID = strings.TrimSpace(v)
					case map[string]any:
						if id, okID := v["id"].(string); okID {
							projectID = strings.TrimSpace(id)
						}
					}
				}
				break
			}

			log.Debugf("Auto-discovery: onboarding in progress, attempt %d...", attempt)
			select {
			case <-autoCtx.Done():
				return &projectSelectionRequiredError{}
			case <-time.After(2 * time.Second):
			}
		}

		if projectID == "" {
			return &projectSelectionRequiredError{}
		}
		log.Infof("Auto-discovered project ID via onboarding: %s", projectID)
	}

	onboardReqBody := map[string]any{
		"tierId":                  tierID,
		"metadata":                metadata,
		"cloudaicompanionProject": projectID,
	}

	storage.ProjectID = projectID

	for {
		var onboardResp map[string]any
		if errOnboard := callGeminiCLI(ctx, httpClient, "onboardUser", onboardReqBody, &onboardResp); errOnboard != nil {
			return fmt.Errorf("onboard user: %w", errOnboard)
		}

		if done, okDone := onboardResp["done"].(bool); okDone && done {
			responseProjectID := ""
			if resp, okResp := onboardResp["response"].(map[string]any); okResp {
				switch projectValue := resp["cloudaicompanionProject"].(type) {
				case map[string]any:
					if id, okID := projectValue["id"].(string); okID {
						responseProjectID = strings.TrimSpace(id)
					}
				case string:
					responseProjectID = strings.TrimSpace(projectValue)
				}
			}

			finalProjectID := projectID
			if responseProjectID != "" {
				if explicitProject && !strings.EqualFold(responseProjectID, projectID) {
					// Check if this is a free user (gen-lang-client projects or free/legacy tier)
					isFreeUser := strings.HasPrefix(projectID, "gen-lang-client-") ||
						strings.EqualFold(tierID, "FREE") ||
						strings.EqualFold(tierID, "LEGACY")

					if isFreeUser {
						// For free users, use backend project ID for preview model access
						log.Infof("Gemini onboarding: frontend project %s maps to backend project %s", projectID, responseProjectID)
						log.Infof("Using backend project ID: %s (recommended for preview model access)", responseProjectID)
						finalProjectID = responseProjectID
					} else {
						// Pro users: keep requested project ID (original behavior)
						log.Warnf("Gemini onboarding returned project %s instead of requested %s; keeping requested project ID.", responseProjectID, projectID)
					}
				} else {
					finalProjectID = responseProjectID
				}
			}

			storage.ProjectID = strings.TrimSpace(finalProjectID)
			if storage.ProjectID == "" {
				storage.ProjectID = strings.TrimSpace(projectID)
			}
			if storage.ProjectID == "" {
				return fmt.Errorf("onboard user completed without project id")
			}
			log.Infof("Onboarding complete. Using Project ID: %s", storage.ProjectID)
			return nil
		}

		log.Println("Onboarding in progress, waiting 5 seconds...")
		time.Sleep(5 * time.Second)
	}
}

func callGeminiCLI(ctx context.Context, httpClient *http.Client, endpoint string, body any, result any) error {
	endPointURL := fmt.Sprintf("%s/%s:%s", geminiCLIEndpoint, geminiCLIVersion, endpoint)
	if strings.HasPrefix(endpoint, "operations/") {
		endPointURL = fmt.Sprintf("%s/%s", geminiCLIEndpoint, endpoint)
	}

	var reader io.Reader
	if body != nil {
		rawBody, errMarshal := json.Marshal(body)
		if errMarshal != nil {
			return fmt.Errorf("marshal request body: %w", errMarshal)
		}
		reader = bytes.NewReader(rawBody)
	}

	req, errRequest := http.NewRequestWithContext(ctx, http.MethodPost, endPointURL, reader)
	if errRequest != nil {
		return fmt.Errorf("create request: %w", errRequest)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", geminiCLIUserAgent)
	req.Header.Set("X-Goog-Api-Client", geminiCLIApiClient)
	req.Header.Set("Client-Metadata", geminiCLIClientMetadata)

	resp, errDo := httpClient.Do(req)
	if errDo != nil {
		return fmt.Errorf("execute request: %w", errDo)
	}
	defer func() {
		if errClose := resp.Body.Close(); errClose != nil {
			log.Errorf("response body close error: %v", errClose)
		}
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("api request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	if result == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}

	if errDecode := json.NewDecoder(resp.Body).Decode(result); errDecode != nil {
		return fmt.Errorf("decode response body: %w", errDecode)
	}

	return nil
}

func fetchGCPProjects(ctx context.Context, httpClient *http.Client) ([]interfaces.GCPProjectProjects, error) {
	req, errRequest := http.NewRequestWithContext(ctx, http.MethodGet, "https://cloudresourcemanager.googleapis.com/v1/projects", nil)
	if errRequest != nil {
		return nil, fmt.Errorf("could not create project list request: %w", errRequest)
	}

	resp, errDo := httpClient.Do(req)
	if errDo != nil {
		return nil, fmt.Errorf("failed to execute project list request: %w", errDo)
	}
	defer func() {
		if errClose := resp.Body.Close(); errClose != nil {
			log.Errorf("response body close error: %v", errClose)
		}
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("project list request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var projects interfaces.GCPProject
	if errDecode := json.NewDecoder(resp.Body).Decode(&projects); errDecode != nil {
		return nil, fmt.Errorf("failed to unmarshal project list: %w", errDecode)
	}

	return projects.Projects, nil
}

func checkCloudAPIIsEnabled(ctx context.Context, httpClient *http.Client, projectID string) (bool, error) {
	serviceUsageURL := "https://serviceusage.googleapis.com"
	requiredServices := []string{
		"cloudaicompanion.googleapis.com",
	}
	for _, service := range requiredServices {
		checkURL := fmt.Sprintf("%s/v1/projects/%s/services/%s", serviceUsageURL, projectID, service)
		req, errRequest := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
		if errRequest != nil {
			return false, fmt.Errorf("failed to create request: %w", errRequest)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", geminiCLIUserAgent)
		resp, errDo := httpClient.Do(req)
		if errDo != nil {
			return false, fmt.Errorf("failed to execute request: %w", errDo)
		}

		if resp.StatusCode == http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			if gjson.GetBytes(bodyBytes, "state").String() == "ENABLED" {
				_ = resp.Body.Close()
				continue
			}
		}
		_ = resp.Body.Close()

		enableURL := fmt.Sprintf("%s/v1/projects/%s/services/%s:enable", serviceUsageURL, projectID, service)
		req, errRequest = http.NewRequestWithContext(ctx, http.MethodPost, enableURL, strings.NewReader("{}"))
		if errRequest != nil {
			return false, fmt.Errorf("failed to create request: %w", errRequest)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", geminiCLIUserAgent)
		resp, errDo = httpClient.Do(req)
		if errDo != nil {
			return false, fmt.Errorf("failed to execute request: %w", errDo)
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		errMessage := string(bodyBytes)
		errMessageResult := gjson.GetBytes(bodyBytes, "error.message")
		if errMessageResult.Exists() {
			errMessage = errMessageResult.String()
		}
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
			_ = resp.Body.Close()
			continue
		} else if resp.StatusCode == http.StatusBadRequest {
			_ = resp.Body.Close()
			if strings.Contains(strings.ToLower(errMessage), "already enabled") {
				continue
			}
		}
		_ = resp.Body.Close()
		return false, fmt.Errorf("project activation required: %s", errMessage)
	}
	return true, nil
}
