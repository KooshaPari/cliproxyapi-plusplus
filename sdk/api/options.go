// Package api exposes server option helpers for embedding CLIProxyAPI.
//
// It wraps internal server option types so external projects can configure the embedded
// HTTP server without importing internal packages.
package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/api"
	"github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/api/handlers"
	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/config"
	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/logging"
)

// ServerOption customises HTTP server construction.
type ServerOption = api.ServerOption

// WithMiddleware appends additional Gin middleware during server construction.
func WithMiddleware(mw ...gin.HandlerFunc) ServerOption { return api.WithMiddleware(mw...) }

// WithEngineConfigurator allows callers to mutate the Gin engine prior to middleware setup.
func WithEngineConfigurator(fn func(*gin.Engine)) ServerOption {
	return api.WithEngineConfigurator(fn)
}

// WithRouterConfigurator appends a callback after default routes are registered.
func WithRouterConfigurator(fn func(*gin.Engine, *handlers.BaseAPIHandler, *config.Config)) ServerOption {
	return api.WithRouterConfigurator(fn)
}

// WithLocalManagementPassword stores a runtime-only management password accepted for localhost requests.
func WithLocalManagementPassword(password string) ServerOption {
	return api.WithLocalManagementPassword(password)
}

// WithKeepAliveEndpoint enables a keep-alive endpoint with the provided timeout and callback.
func WithKeepAliveEndpoint(timeout time.Duration, onTimeout func()) ServerOption {
	return api.WithKeepAliveEndpoint(timeout, onTimeout)
}

// WithRequestLoggerFactory customises request logger creation.
func WithRequestLoggerFactory(factory func(*config.Config, string) logging.RequestLogger) ServerOption {
	return api.WithRequestLoggerFactory(factory)
}
