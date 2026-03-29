package management

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"

	coreauth "github.com/kooshapari/CLIProxyAPI/v7/sdk/cliproxy/auth"
)

func firstNonEmptyString(values ...*string) string {
	for _, v := range values {
		if v == nil {
			continue
		}
		if out := strings.TrimSpace(*v); out != "" {
			return out
		}
	}
	return ""
}

func stringValue(metadata map[string]any, key string) string {
	if len(metadata) == 0 || key == "" {
		return ""
	}
	if v, ok := metadata[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func int64Value(raw any) int64 {
	switch typed := raw.(type) {
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case uint:
		return int64(typed)
	case uint32:
		return int64(typed)
	case uint64:
		if typed > uint64(^uint64(0)>>1) {
			return 0
		}
		return int64(typed)
	case float32:
		return int64(typed)
	case float64:
		return int64(typed)
	case json.Number:
		if i, errParse := typed.Int64(); errParse == nil {
			return i
		}
	case string:
		if s := strings.TrimSpace(typed); s != "" {
			if i, errParse := json.Number(s).Int64(); errParse == nil {
				return i
			}
		}
	}
	return 0
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// headerContainsValue checks whether a header map contains a target value (case-insensitive key and value).
func headerContainsValue(headers map[string]string, targetKey, targetValue string) bool {
	if len(headers) == 0 {
		return false
	}
	for key, value := range headers {
		if !strings.EqualFold(strings.TrimSpace(key), strings.TrimSpace(targetKey)) {
			continue
		}
		if strings.Contains(strings.ToLower(value), strings.ToLower(strings.TrimSpace(targetValue))) {
			return true
		}
	}
	return false
}

func (h *Handler) authByIndex(authIndex string) *coreauth.Auth {
	authIndex = strings.TrimSpace(authIndex)
	if authIndex == "" || h == nil || h.authManager == nil {
		return nil
	}
	auths := h.authManager.List()
	for _, auth := range auths {
		if auth == nil {
			continue
		}
		auth.EnsureIndex()
		if auth.Index == authIndex {
			return auth
		}
	}
	return nil
}

func profileARNForAuth(auth *coreauth.Auth) string {
	if auth == nil {
		return ""
	}

	if v := strings.TrimSpace(auth.Attributes["profile_arn"]); v != "" {
		return v
	}
	if v := strings.TrimSpace(auth.Attributes["profileArn"]); v != "" {
		return v
	}

	metadata := auth.Metadata
	if len(metadata) == 0 {
		return ""
	}
	if v := stringValue(metadata, "profile_arn"); v != "" {
		return v
	}
	if v := stringValue(metadata, "profileArn"); v != "" {
		return v
	}

	if tokenRaw, ok := metadata["token"].(map[string]any); ok {
		if v := stringValue(tokenRaw, "profile_arn"); v != "" {
			return v
		}
		if v := stringValue(tokenRaw, "profileArn"); v != "" {
			return v
		}
	}

	return ""
}

func firstNonEmptyQuery(c *gin.Context, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(c.Query(key)); value != "" {
			return value
		}
	}
	return ""
}
