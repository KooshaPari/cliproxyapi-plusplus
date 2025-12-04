// Package amp implements the Amp CLI routing module, providing OAuth-based
// integration with Amp CLI for ChatGPT and Anthropic subscriptions.
package amp

import (
	"fmt"
	"net/http/httputil"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/api/modules"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	sdkaccess "github.com/router-for-me/CLIProxyAPI/v6/sdk/access"
	log "github.com/sirupsen/logrus"
)

// Option configures the AmpModule.
type Option func(*AmpModule)

// AmpModule implements the RouteModuleV2 interface for Amp CLI integration.
// It provides:
//   - Reverse proxy to Amp control plane for OAuth/management
//   - Provider-specific route aliases (/api/provider/{provider}/...)
//   - Automatic gzip decompression for misconfigured upstreams
//   - Model mapping for routing unavailable models to alternatives
type AmpModule struct {
	secretSource    SecretSource
	proxy           *httputil.ReverseProxy
	accessManager   *sdkaccess.Manager
	authMiddleware_ gin.HandlerFunc
	modelMapper     *DefaultModelMapper
	enabled         bool
	registerOnce    sync.Once

	// configMu protects lastConfig for partial reload comparison
	configMu   sync.RWMutex
	lastConfig *config.AmpCode
}

// New creates a new Amp routing module with the given options.
// This is the preferred constructor using the Option pattern.
//
// Example:
//
//	ampModule := amp.New(
//	    amp.WithAccessManager(accessManager),
//	    amp.WithAuthMiddleware(authMiddleware),
//	    amp.WithSecretSource(customSecret),
//	)
func New(opts ...Option) *AmpModule {
	m := &AmpModule{
		secretSource: nil, // Will be created on demand if not provided
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// NewLegacy creates a new Amp routing module using the legacy constructor signature.
// This is provided for backwards compatibility.
//
// DEPRECATED: Use New with options instead.
func NewLegacy(accessManager *sdkaccess.Manager, authMiddleware gin.HandlerFunc) *AmpModule {
	return New(
		WithAccessManager(accessManager),
		WithAuthMiddleware(authMiddleware),
	)
}

// WithSecretSource sets a custom secret source for the module.
func WithSecretSource(source SecretSource) Option {
	return func(m *AmpModule) {
		m.secretSource = source
	}
}

// WithAccessManager sets the access manager for the module.
func WithAccessManager(am *sdkaccess.Manager) Option {
	return func(m *AmpModule) {
		m.accessManager = am
	}
}

// WithAuthMiddleware sets the authentication middleware for provider routes.
func WithAuthMiddleware(middleware gin.HandlerFunc) Option {
	return func(m *AmpModule) {
		m.authMiddleware_ = middleware
	}
}

// Name returns the module identifier
func (m *AmpModule) Name() string {
	return "amp-routing"
}

// Register sets up Amp routes if configured.
// This implements the RouteModuleV2 interface with Context.
// Routes are registered only once via sync.Once for idempotent behavior.
func (m *AmpModule) Register(ctx modules.Context) error {
	settings := ctx.Config.AmpCode
	upstreamURL := strings.TrimSpace(settings.UpstreamURL)

	// Determine auth middleware (from module or context)
	auth := m.getAuthMiddleware(ctx)

	// Use registerOnce to ensure routes are only registered once
	var regErr error
	m.registerOnce.Do(func() {
		// Initialize model mapper from config (for routing unavailable models to alternatives)
		m.modelMapper = NewModelMapper(settings.ModelMappings)

		// Store initial config for partial reload comparison
		settingsCopy := settings
		m.lastConfig = &settingsCopy

		// Always register provider aliases - these work without an upstream
		m.registerProviderAliases(ctx.Engine, ctx.BaseHandler, auth)

		// If no upstream URL, skip proxy routes but provider aliases are still available
		if upstreamURL == "" {
			log.Debug("amp upstream proxy disabled (no upstream URL configured)")
			log.Debug("amp provider alias routes registered")
			m.enabled = false
			return
		}

		// Create secret source with precedence: config > env > file
		// Cache secrets for 5 minutes to reduce file I/O
		if m.secretSource == nil {
			m.secretSource = NewMultiSourceSecret(settings.UpstreamAPIKey, 0 /* default 5min */)
		}

		// Create reverse proxy with gzip handling via ModifyResponse
		proxy, err := createReverseProxy(upstreamURL, m.secretSource)
		if err != nil {
			regErr = fmt.Errorf("failed to create amp proxy: %w", err)
			return
		}

		m.proxy = proxy
		m.enabled = true

		// Register management proxy routes (requires upstream)
		// Restrict to localhost by default for security (prevents drive-by browser attacks)
		handler := proxyHandler(proxy)
		m.registerManagementRoutes(ctx.Engine, ctx.BaseHandler, handler, settings.RestrictManagementToLocalhost)

		log.Infof("amp upstream proxy enabled for: %s", upstreamURL)
		log.Debug("amp provider alias routes registered")
	})

	return regErr
}

// getAuthMiddleware returns the authentication middleware, preferring the
// module's configured middleware, then the context middleware, then a fallback.
func (m *AmpModule) getAuthMiddleware(ctx modules.Context) gin.HandlerFunc {
	if m.authMiddleware_ != nil {
		return m.authMiddleware_
	}
	if ctx.AuthMiddleware != nil {
		return ctx.AuthMiddleware
	}
	// Fallback: no authentication (should not happen in production)
	log.Warn("amp module: no auth middleware provided, allowing all requests")
	return func(c *gin.Context) {
		c.Next()
	}
}

// OnConfigUpdated handles configuration updates with partial reload support.
// Only updates components that have actually changed to avoid unnecessary work.
// URL changes still require restart (logged as warning).
func (m *AmpModule) OnConfigUpdated(cfg *config.Config) error {
	newSettings := cfg.AmpCode

	// Get previous config for comparison
	m.configMu.RLock()
	oldSettings := m.lastConfig
	m.configMu.RUnlock()

	// Track what changed for logging
	var changes []string

	// Check model mappings change
	modelMappingsChanged := m.hasModelMappingsChanged(oldSettings, &newSettings)
	if modelMappingsChanged {
		if m.modelMapper != nil {
			m.modelMapper.UpdateMappings(newSettings.ModelMappings)
			changes = append(changes, "model-mappings")
			if m.enabled {
				log.Infof("amp config partial reload: model mappings updated (%d entries)", len(newSettings.ModelMappings))
			}
		} else if m.enabled {
			log.Warnf("amp model mapper not initialized, skipping model mapping update")
		}
	}

	if m.enabled {
		// Check upstream URL change (requires restart)
		newUpstreamURL := strings.TrimSpace(newSettings.UpstreamURL)
		oldUpstreamURL := ""
		if oldSettings != nil {
			oldUpstreamURL = strings.TrimSpace(oldSettings.UpstreamURL)
		}

		if newUpstreamURL == "" && oldUpstreamURL != "" {
			log.Warn("amp upstream URL removed from config, restart required to disable")
		} else if newUpstreamURL != oldUpstreamURL {
			changes = append(changes, "upstream-url(restart required)")
			log.Warnf("amp config: upstream-url changed (%s -> %s), restart required", oldUpstreamURL, newUpstreamURL)
		}

		// Check API key change
		apiKeyChanged := m.hasAPIKeyChanged(oldSettings, &newSettings)
		if apiKeyChanged {
			if m.secretSource != nil {
				if ms, ok := m.secretSource.(*MultiSourceSecret); ok {
					ms.UpdateExplicitKey(newSettings.UpstreamAPIKey)
					ms.InvalidateCache()
					changes = append(changes, "upstream-api-key")
					log.Debug("amp config partial reload: secret cache invalidated")
				}
			}
		}

		// Check restrict-management-to-localhost change (requires restart)
		if oldSettings != nil && oldSettings.RestrictManagementToLocalhost != newSettings.RestrictManagementToLocalhost {
			changes = append(changes, "restrict-management-to-localhost(restart required)")
			log.Warnf("amp config: restrict-management-to-localhost changed (%t -> %t), restart required",
				oldSettings.RestrictManagementToLocalhost, newSettings.RestrictManagementToLocalhost)
		}
	}

	// Store current config for next comparison
	m.configMu.Lock()
	settingsCopy := newSettings // copy struct
	m.lastConfig = &settingsCopy
	m.configMu.Unlock()

	// Log summary if any changes detected
	if len(changes) > 0 {
		log.Debugf("amp config partial reload completed: %v", changes)
	} else {
		log.Debug("amp config checked: no changes detected")
	}

	return nil
}

// hasModelMappingsChanged compares old and new model mappings.
func (m *AmpModule) hasModelMappingsChanged(old *config.AmpCode, new *config.AmpCode) bool {
	if old == nil {
		return len(new.ModelMappings) > 0
	}

	if len(old.ModelMappings) != len(new.ModelMappings) {
		return true
	}

	// Build map for efficient comparison
	oldMap := make(map[string]string, len(old.ModelMappings))
	for _, mapping := range old.ModelMappings {
		oldMap[strings.TrimSpace(mapping.From)] = strings.TrimSpace(mapping.To)
	}

	for _, mapping := range new.ModelMappings {
		from := strings.TrimSpace(mapping.From)
		to := strings.TrimSpace(mapping.To)
		if oldTo, exists := oldMap[from]; !exists || oldTo != to {
			return true
		}
	}

	return false
}

// hasAPIKeyChanged compares old and new API keys.
func (m *AmpModule) hasAPIKeyChanged(old *config.AmpCode, new *config.AmpCode) bool {
	oldKey := ""
	if old != nil {
		oldKey = strings.TrimSpace(old.UpstreamAPIKey)
	}
	newKey := strings.TrimSpace(new.UpstreamAPIKey)
	return oldKey != newKey
}

// GetModelMapper returns the model mapper instance (for testing/debugging).
func (m *AmpModule) GetModelMapper() *DefaultModelMapper {
	return m.modelMapper
}
