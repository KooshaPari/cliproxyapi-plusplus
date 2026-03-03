package management

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	sdkAuth "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/auth"
	coreauth "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/cliproxy/auth"
)

func (h *Handler) authIDForPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if h == nil || h.cfg == nil {
		return path
	}
	authDir := strings.TrimSpace(h.cfg.AuthDir)
	if authDir == "" {
		return path
	}
	if rel, err := filepath.Rel(authDir, path); err == nil && rel != "" {
		return rel
	}
	return path
}

func (h *Handler) resolveAuthPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("auth path is empty")
	}
	if h == nil || h.cfg == nil {
		return "", fmt.Errorf("handler configuration unavailable")
	}
	authDir := strings.TrimSpace(h.cfg.AuthDir)
	if authDir == "" {
		return "", fmt.Errorf("auth directory not configured")
	}
	cleanAuthDir, err := filepath.Abs(filepath.Clean(authDir))
	if err != nil {
		return "", fmt.Errorf("resolve auth dir: %w", err)
	}
	if resolvedDir, err := filepath.EvalSymlinks(cleanAuthDir); err == nil {
		cleanAuthDir = resolvedDir
	}
	cleanPath := filepath.Clean(path)
	absPath := cleanPath
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(cleanAuthDir, cleanPath)
	}
	absPath, err = filepath.Abs(absPath)
	if err != nil {
		return "", fmt.Errorf("resolve auth path: %w", err)
	}
	relPath, err := filepath.Rel(cleanAuthDir, absPath)
	if err != nil {
		return "", fmt.Errorf("resolve relative auth path: %w", err)
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("auth path escapes auth directory")
	}
	return absPath, nil
}

func (h *Handler) registerAuthFromFile(ctx context.Context, path string, data []byte) error {
	if h.authManager == nil {
		return nil
	}
	safePath, err := h.resolveAuthPath(path)
	if err != nil {
		return err
	}
	if data == nil {
		data, err = os.ReadFile(safePath)
		if err != nil {
			return fmt.Errorf("failed to read auth file: %w", err)
		}
	}
	metadata := make(map[string]any)
	if err := json.Unmarshal(data, &metadata); err != nil {
		return fmt.Errorf("invalid auth file: %w", err)
	}
	provider, _ := metadata["type"].(string)
	if provider == "" {
		provider = "unknown"
	}
	label := provider
	if email, ok := metadata["email"].(string); ok && email != "" {
		label = email
	}
	lastRefresh, hasLastRefresh := extractLastRefreshTimestamp(metadata)

	authID := h.authIDForPath(safePath)
	if authID == "" {
		authID = safePath
	}
	attr := map[string]string{
		"path":   safePath,
		"source": safePath,
	}
	auth := &coreauth.Auth{
		ID:         authID,
		Provider:   provider,
		FileName:   filepath.Base(safePath),
		Label:      label,
		Status:     coreauth.StatusActive,
		Attributes: attr,
		Metadata:   metadata,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if hasLastRefresh {
		auth.LastRefreshedAt = lastRefresh
	}
	if existing, ok := h.authManager.GetByID(authID); ok {
		auth.CreatedAt = existing.CreatedAt
		if !hasLastRefresh {
			auth.LastRefreshedAt = existing.LastRefreshedAt
		}
		auth.NextRefreshAfter = existing.NextRefreshAfter
		if len(auth.ModelStates) == 0 && len(existing.ModelStates) > 0 {
			auth.ModelStates = existing.ModelStates
		}
		auth.Runtime = existing.Runtime
		_, err = h.authManager.Update(ctx, auth)
		return err
	}
	_, err = h.authManager.Register(ctx, auth)
	return err
}

func (h *Handler) PatchAuthFileStatus(c *gin.Context) {
	if h.authManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "core auth manager unavailable"})
		return
	}

	var req struct {
		Name     string `json:"name"`
		Disabled *bool  `json:"disabled"`
		Enabled  *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if req.Disabled == nil && req.Enabled == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "disabled or enabled is required"})
		return
	}
	desiredDisabled := false
	if req.Disabled != nil {
		desiredDisabled = *req.Disabled
	} else {
		desiredDisabled = !*req.Enabled
	}

	ctx := c.Request.Context()

	targetAuth := h.findAuthByIdentifier(name)

	if targetAuth == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "auth file not found"})
		return
	}

	// Update disabled state
	targetAuth.Disabled = desiredDisabled
	if desiredDisabled {
		targetAuth.Status = coreauth.StatusDisabled
		targetAuth.StatusMessage = "disabled via management API"
	} else {
		targetAuth.Status = coreauth.StatusActive
		targetAuth.StatusMessage = ""
	}
	targetAuth.UpdatedAt = time.Now()

	if _, err := h.authManager.Update(ctx, targetAuth); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to update auth: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "disabled": desiredDisabled})
}

func (h *Handler) findAuthByIdentifier(name string) *coreauth.Auth {
	name = strings.TrimSpace(name)
	if name == "" || h.authManager == nil {
		return nil
	}
	if auth, ok := h.authManager.GetByID(name); ok {
		return auth
	}
	for _, auth := range h.authManager.List() {
		if auth.FileName == name || filepath.Base(auth.FileName) == name {
			return auth
		}
		if pathVal, ok := auth.Attributes["path"]; ok && (pathVal == name || filepath.Base(pathVal) == name) {
			return auth
		}
		if sourceVal, ok := auth.Attributes["source"]; ok && (sourceVal == name || filepath.Base(sourceVal) == name) {
			return auth
		}
	}
	return nil
}

func (h *Handler) PatchAuthFileFields(c *gin.Context) {
	if h.authManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "core auth manager unavailable"})
		return
	}

	var req struct {
		Name     string  `json:"name"`
		Prefix   *string `json:"prefix"`
		ProxyURL *string `json:"proxy_url"`
		Priority *int    `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	ctx := c.Request.Context()

	targetAuth := h.findAuthByIdentifier(name)

	if targetAuth == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "auth file not found"})
		return
	}

	changed := false
	if req.Prefix != nil {
		targetAuth.Prefix = *req.Prefix
		changed = true
	}
	if req.ProxyURL != nil {
		targetAuth.ProxyURL = *req.ProxyURL
		changed = true
	}
	if req.Priority != nil {
		if targetAuth.Metadata == nil {
			targetAuth.Metadata = make(map[string]any)
		}
		if *req.Priority == 0 {
			delete(targetAuth.Metadata, "priority")
		} else {
			targetAuth.Metadata["priority"] = *req.Priority
		}
		changed = true
	}

	if !changed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}

	targetAuth.UpdatedAt = time.Now()

	if _, err := h.authManager.Update(ctx, targetAuth); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to update auth: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) disableAuth(ctx context.Context, id string) {
	if h == nil || h.authManager == nil {
		return
	}
	authID := h.authIDForPath(id)
	if authID == "" {
		authID = strings.TrimSpace(id)
	}
	if authID == "" {
		return
	}
	if auth, ok := h.authManager.GetByID(authID); ok {
		auth.Disabled = true
		auth.Status = coreauth.StatusDisabled
		auth.StatusMessage = "removed via management API"
		auth.UpdatedAt = time.Now()
		_, _ = h.authManager.Update(ctx, auth)
	}
}

func (h *Handler) deleteTokenRecord(ctx context.Context, path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("auth path is empty")
	}
	store := h.tokenStoreWithBaseDir()
	if store == nil {
		return fmt.Errorf("token store unavailable")
	}
	return store.Delete(ctx, path)
}

func (h *Handler) tokenStoreWithBaseDir() coreauth.Store {
	if h == nil {
		return nil
	}
	store := h.tokenStore
	if store == nil {
		store = sdkAuth.GetTokenStore()
		h.tokenStore = store
	}
	if h.cfg != nil {
		if dirSetter, ok := store.(interface{ SetBaseDir(string) }); ok {
			dirSetter.SetBaseDir(h.cfg.AuthDir)
		}
	}
	return store
}

func (h *Handler) saveTokenRecord(ctx context.Context, record *coreauth.Auth) (string, error) {
	if record == nil {
		return "", fmt.Errorf("token record is nil")
	}
	store := h.tokenStoreWithBaseDir()
	if store == nil {
		return "", fmt.Errorf("token store unavailable")
	}
	return store.Save(ctx, record)
}
