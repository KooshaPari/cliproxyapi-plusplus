package claude

import (
	"net/http"
	"time"

	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/config"
	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/util"
)

// NewAnthropicHttpClient returns an HTTP client used by internal Claude auth flows.
func NewAnthropicHttpClient(cfg *config.SDKConfig) *http.Client {
	client := &http.Client{Timeout: 30 * time.Second}
	if cfg == nil {
		return client
	}
	return util.SetProxy(cfg, client)
}
