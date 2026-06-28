package management

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/config"
	coreauth "github.com/kooshapari/CLIProxyAPI/v7/sdk/cliproxy/auth"
)

func isReadOnlyConfigWriteError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EROFS) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "read-only file system") ||
		strings.Contains(msg, "read-only filesystem") ||
		strings.Contains(msg, "read only file system")
}

func sanitizeOAuthCallbackPath(authDir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, `/\`) || filepath.Base(name) != name {
		return "", os.ErrInvalid
	}
	path := filepath.Join(authDir, name)
	absDir, err := filepath.Abs(authDir)
	if err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absDir, absPath)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", os.ErrInvalid
	}
	return absPath, nil
}

func (h *Handler) GetAmpCode(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ampcode": h.cfg.AmpCode}) }
func (h *Handler) GetAmpUpstreamURL(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"upstream-url": h.cfg.AmpCode.UpstreamURL})
}
func (h *Handler) PutAmpUpstreamURL(c *gin.Context) {
	var body struct {
		Value string `json:"value"`
	}
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	h.cfg.AmpCode.UpstreamURL = strings.TrimSpace(body.Value)
	h.persist(c)
}
func (h *Handler) DeleteAmpUpstreamURL(c *gin.Context) {
	h.cfg.AmpCode.UpstreamURL = ""
	h.persist(c)
}
func (h *Handler) GetAmpUpstreamAPIKey(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"upstream-api-key": h.cfg.AmpCode.UpstreamAPIKey})
}
func (h *Handler) PutAmpUpstreamAPIKey(c *gin.Context) {
	var body struct {
		Value string `json:"value"`
	}
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	h.cfg.AmpCode.UpstreamAPIKey = strings.TrimSpace(body.Value)
	h.persist(c)
}
func (h *Handler) DeleteAmpUpstreamAPIKey(c *gin.Context) {
	h.cfg.AmpCode.UpstreamAPIKey = ""
	h.persist(c)
}
func (h *Handler) GetAmpUpstreamAPIKeys(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"upstream-api-keys": h.cfg.AmpCode.UpstreamAPIKeys})
}
func (h *Handler) PutAmpUpstreamAPIKeys(c *gin.Context) {
	var body struct {
		Value []config.AmpUpstreamAPIKeyEntry `json:"value"`
	}
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	h.cfg.AmpCode.UpstreamAPIKeys = normalizeAmpUpstreamAPIKeys(body.Value)
	h.persist(c)
}
func (h *Handler) PatchAmpUpstreamAPIKeys(c *gin.Context) { h.PutAmpUpstreamAPIKeys(c) }
func (h *Handler) DeleteAmpUpstreamAPIKeys(c *gin.Context) {
	h.cfg.AmpCode.UpstreamAPIKeys = nil
	h.persist(c)
}
func (h *Handler) GetAmpRestrictManagementToLocalhost(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"restrict-management-to-localhost": h.cfg.AmpCode.RestrictManagementToLocalhost})
}
func (h *Handler) PutAmpRestrictManagementToLocalhost(c *gin.Context) {
	var body struct {
		Value bool `json:"value"`
	}
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	h.cfg.AmpCode.RestrictManagementToLocalhost = body.Value
	h.persist(c)
}
func (h *Handler) GetAmpModelMappings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"model-mappings": h.cfg.AmpCode.ModelMappings})
}
func (h *Handler) PutAmpModelMappings(c *gin.Context) {
	var body struct {
		Value []config.AmpModelMapping `json:"value"`
	}
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	h.cfg.AmpCode.ModelMappings = body.Value
	h.persist(c)
}

// PatchAmpModelMappings upserts the supplied mappings by their "from" field,
// updating existing entries and appending new ones while preserving any
// mappings that are not referenced in the request.
func (h *Handler) PatchAmpModelMappings(c *gin.Context) {
	var body struct {
		Value []config.AmpModelMapping `json:"value"`
	}
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	existing := h.cfg.AmpCode.ModelMappings
	index := make(map[string]int, len(existing))
	for i, mapping := range existing {
		index[mapping.From] = i
	}
	for _, mapping := range body.Value {
		if i, ok := index[mapping.From]; ok {
			existing[i] = mapping
			continue
		}
		index[mapping.From] = len(existing)
		existing = append(existing, mapping)
	}
	h.cfg.AmpCode.ModelMappings = existing
	h.persist(c)
}

// DeleteAmpModelMappings removes mappings named by their "from" field in the
// request body. A missing or empty body removes all mappings.
func (h *Handler) DeleteAmpModelMappings(c *gin.Context) {
	var body struct {
		Value []string `json:"value"`
	}
	// Ignore bind errors (e.g. empty body) and treat them as "delete all".
	_ = c.ShouldBindJSON(&body)
	if len(body.Value) == 0 {
		h.cfg.AmpCode.ModelMappings = nil
		h.persist(c)
		return
	}
	remove := make(map[string]struct{}, len(body.Value))
	for _, from := range body.Value {
		remove[from] = struct{}{}
	}
	filtered := make([]config.AmpModelMapping, 0, len(h.cfg.AmpCode.ModelMappings))
	for _, mapping := range h.cfg.AmpCode.ModelMappings {
		if _, ok := remove[mapping.From]; ok {
			continue
		}
		filtered = append(filtered, mapping)
	}
	h.cfg.AmpCode.ModelMappings = filtered
	h.persist(c)
}
func (h *Handler) GetAmpForceModelMappings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"force-model-mappings": h.cfg.AmpCode.ForceModelMappings})
}
func (h *Handler) PutAmpForceModelMappings(c *gin.Context) {
	value, ok := h.putBoolField(c)
	if !ok {
		return
	}
	h.cfg.AmpCode.ForceModelMappings = value
	h.persist(c)
}

// putBoolField binds a {"value": bool} body and requires the "value" field to be
// present. It writes a 400 response and returns ok=false when the body is
// invalid or the field is missing.
func (h *Handler) putBoolField(c *gin.Context) (bool, bool) {
	var body struct {
		Value *bool `json:"value"`
	}
	if c.ShouldBindJSON(&body) != nil || body.Value == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return false, false
	}
	return *body.Value, true
}

func normalizeAmpUpstreamAPIKeys(entries []config.AmpUpstreamAPIKeyEntry) []config.AmpUpstreamAPIKeyEntry {
	out := make([]config.AmpUpstreamAPIKeyEntry, 0, len(entries))
	for _, entry := range entries {
		key := strings.TrimSpace(entry.UpstreamAPIKey)
		if key == "" {
			continue
		}
		keys := make([]string, 0, len(entry.APIKeys))
		for _, apiKey := range entry.APIKeys {
			if trimmed := strings.TrimSpace(apiKey); trimmed != "" {
				keys = append(keys, trimmed)
			}
		}
		out = append(out, config.AmpUpstreamAPIKeyEntry{UpstreamAPIKey: key, APIKeys: keys})
	}
	return out
}

func (h *Handler) RequestGitLabPATToken(c *gin.Context) {
	var body struct {
		BaseURL             string `json:"base_url"`
		PersonalAccessToken string `json:"personal_access_token"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	baseURL := strings.TrimRight(strings.TrimSpace(body.BaseURL), "/")
	token := strings.TrimSpace(body.PersonalAccessToken)
	if baseURL == "" || token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing gitlab credentials"})
		return
	}
	client := &http.Client{Timeout: 15 * time.Second}
	user, err := gitLabGetJSON(client, baseURL+"/api/v4/user", token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	_, _ = gitLabGetJSON(client, baseURL+"/api/v4/personal_access_tokens/self", token)
	direct, err := gitLabGetJSON(client, baseURL+"/api/v4/code_suggestions/direct_access", token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	modelProvider, _ := direct["model_provider"].(string)
	modelName, _ := direct["model_name"].(string)
	if details, ok := direct["model_details"].(map[string]any); ok {
		if v, _ := details["model_provider"].(string); v != "" {
			modelProvider = v
		}
		if v, _ := details["model_name"].(string); v != "" {
			modelName = v
		}
	}
	label, _ := user["email"].(string)
	if label == "" {
		label, _ = user["username"].(string)
	}
	record := &coreauth.Auth{
		ID:       "gitlab-pat",
		Provider: "gitlab",
		Label:    label,
		Status:   coreauth.StatusActive,
		Attributes: map[string]string{
			coreauth.AttributeSourceBackend: coreauth.AuthSourceFile,
		},
		Metadata: map[string]any{
			"base_url":              baseURL,
			"auth_kind":             "personal_access_token",
			"personal_access_token": token,
			"gateway_base_url":      direct["base_url"],
			"duo_gateway_token":     direct["token"],
			"gateway_token":         direct["token"],
			"model_provider":        modelProvider,
			"model_name":            modelName,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if h.tokenStore != nil {
		if _, errSave := h.tokenStore.Save(c.Request.Context(), record); errSave != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": errSave.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "model_provider": modelProvider, "model_name": modelName})
}

func gitLabGetJSON(client *http.Client, url, token string) (map[string]any, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gitlab status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
