package config

import (
	"bytes"
	"encoding/json"
	"strings"

	log "github.com/sirupsen/logrus"
)

// SanitizePayloadRules validates raw JSON payload rule params and drops invalid rules.
func (cfg *Config) SanitizePayloadRules() {
	if cfg == nil {
		return
	}
	cfg.Payload.Default = sanitizePayloadRules(cfg.Payload.Default, "default")
	cfg.Payload.Override = sanitizePayloadRules(cfg.Payload.Override, "override")
	cfg.Payload.Filter = sanitizePayloadFilterRules(cfg.Payload.Filter, "filter")
	cfg.Payload.DefaultRaw = sanitizePayloadRawRules(cfg.Payload.DefaultRaw, "default-raw")
	cfg.Payload.OverrideRaw = sanitizePayloadRawRules(cfg.Payload.OverrideRaw, "override-raw")
}

func sanitizePayloadRules(rules []PayloadRule, section string) []PayloadRule {
	if len(rules) == 0 {
		return rules
	}
	out := make([]PayloadRule, 0, len(rules))
	for i := range rules {
		rule := rules[i]
		if len(rule.Params) == 0 {
			continue
		}
		invalid := false
		for path := range rule.Params {
			if payloadPathInvalid(path) {
				log.WithFields(log.Fields{
					"section":    section,
					"rule_index": i + 1,
					"param":      path,
				}).Warn("payload rule dropped: invalid parameter path")
				invalid = true
				break
			}
		}
		if invalid {
			continue
		}
		out = append(out, rule)
	}
	return out
}

func sanitizePayloadRawRules(rules []PayloadRule, section string) []PayloadRule {
	if len(rules) == 0 {
		return rules
	}
	out := make([]PayloadRule, 0, len(rules))
	for i := range rules {
		rule := rules[i]
		if len(rule.Params) == 0 {
			continue
		}
		invalid := false
		for path, value := range rule.Params {
			if payloadPathInvalid(path) {
				log.WithFields(log.Fields{
					"section":    section,
					"rule_index": i + 1,
					"param":      path,
				}).Warn("payload rule dropped: invalid parameter path")
				invalid = true
				break
			}
			raw, ok := payloadRawString(value)
			if !ok {
				continue
			}
			trimmed := bytes.TrimSpace(raw)
			if len(trimmed) == 0 || !json.Valid(trimmed) {
				log.WithFields(log.Fields{
					"section":    section,
					"rule_index": i + 1,
					"param":      path,
				}).Warn("payload rule dropped: invalid raw JSON")
				invalid = true
				break
			}
		}
		if invalid {
			continue
		}
		out = append(out, rule)
	}
	return out
}

func sanitizePayloadFilterRules(rules []PayloadFilterRule, section string) []PayloadFilterRule {
	if len(rules) == 0 {
		return rules
	}
	out := make([]PayloadFilterRule, 0, len(rules))
	for i := range rules {
		rule := rules[i]
		if len(rule.Params) == 0 {
			continue
		}
		invalid := false
		for _, path := range rule.Params {
			if payloadPathInvalid(path) {
				log.WithFields(log.Fields{
					"section":    section,
					"rule_index": i + 1,
					"param":      path,
				}).Warn("payload filter rule dropped: invalid parameter path")
				invalid = true
				break
			}
		}
		if invalid {
			continue
		}
		out = append(out, rule)
	}
	return out
}

func payloadPathInvalid(path string) bool {
	p := strings.TrimSpace(path)
	if p == "" {
		return true
	}
	return strings.HasPrefix(p, ".") || strings.HasSuffix(p, ".") || strings.Contains(p, "..")
}

func payloadRawString(value any) ([]byte, bool) {
	switch typed := value.(type) {
	case string:
		return []byte(typed), true
	case json.Number:
		return []byte(typed), true
	case []byte:
		return typed, true
	default:
		return nil, false
	}
}

// SanitizeOAuthModelAlias normalizes OAuth model alias entries.
func (cfg *Config) SanitizeOAuthModelAlias() {
	if cfg == nil {
		return
	}

	// Inject default aliases for channels with built-in compatibility mappings.
	if cfg.OAuthModelAlias == nil {
		cfg.OAuthModelAlias = make(map[string][]OAuthModelAlias)
	}
	if _, hasKiro := cfg.OAuthModelAlias["kiro"]; !hasKiro {
		// Check case-insensitive too
		found := false
		for k := range cfg.OAuthModelAlias {
			if strings.EqualFold(strings.TrimSpace(k), "kiro") {
				found = true
				break
			}
		}
		if !found {
			cfg.OAuthModelAlias["kiro"] = defaultKiroAliases()
		}
	}
	if _, hasGitHubCopilot := cfg.OAuthModelAlias["github-copilot"]; !hasGitHubCopilot {
		// Check case-insensitive too
		found := false
		for k := range cfg.OAuthModelAlias {
			if strings.EqualFold(strings.TrimSpace(k), "github-copilot") {
				found = true
				break
			}
		}
		if !found {
			cfg.OAuthModelAlias["github-copilot"] = defaultGitHubCopilotAliases()
		}
	}

	if len(cfg.OAuthModelAlias) == 0 {
		return
	}
	out := make(map[string][]OAuthModelAlias, len(cfg.OAuthModelAlias))
	for rawChannel, aliases := range cfg.OAuthModelAlias {
		channel := strings.ToLower(strings.TrimSpace(rawChannel))
		if channel == "" {
			continue
		}
		// Preserve channels that were explicitly set to empty/nil
		if len(aliases) == 0 {
			out[channel] = nil
			continue
		}
		seenAlias := make(map[string]struct{}, len(aliases))
		clean := make([]OAuthModelAlias, 0, len(aliases))
		for _, entry := range aliases {
			name := strings.TrimSpace(entry.Name)
			alias := strings.TrimSpace(entry.Alias)
			if name == "" || alias == "" {
				continue
			}
			if strings.EqualFold(name, alias) {
				continue
			}
			// Dedupe by name+alias combination
			aliasKey := strings.ToLower(name) + ":" + strings.ToLower(alias)
			if _, ok := seenAlias[aliasKey]; ok {
				continue
			}
			seenAlias[aliasKey] = struct{}{}
			clean = append(clean, OAuthModelAlias{Name: name, Alias: alias, Fork: entry.Fork})
		}
		if len(clean) > 0 {
			out[channel] = clean
		}
	}
	cfg.OAuthModelAlias = out
}

// SanitizeOAuthUpstream normalizes OAuth upstream URL override keys/values.
func (cfg *Config) SanitizeOAuthUpstream() {
	if cfg == nil {
		return
	}
	if len(cfg.OAuthUpstream) == 0 {
		return
	}
	out := make(map[string]string, len(cfg.OAuthUpstream))
	for rawChannel, rawURL := range cfg.OAuthUpstream {
		channel := normalizeOAuthUpstreamChannel(rawChannel)
		if channel == "" {
			continue
		}
		baseURL := strings.TrimSpace(rawURL)
		if baseURL == "" {
			continue
		}
		out[channel] = strings.TrimRight(baseURL, "/")
	}
	cfg.OAuthUpstream = out
}

// OAuthUpstreamURL resolves the configured OAuth upstream override for a channel.
func (cfg *Config) OAuthUpstreamURL(channel string) string {
	if cfg == nil || len(cfg.OAuthUpstream) == 0 {
		return ""
	}
	key := normalizeOAuthUpstreamChannel(channel)
	if key == "" {
		return ""
	}
	return strings.TrimSpace(cfg.OAuthUpstream[key])
}

func normalizeOAuthUpstreamChannel(channel string) string {
	key := strings.TrimSpace(strings.ToLower(channel))
	if key == "" {
		return ""
	}
	key = strings.ReplaceAll(key, "_", "-")
	key = strings.ReplaceAll(key, " ", "-")
	key = strings.ReplaceAll(key, ".", "-")
	key = strings.ReplaceAll(key, "/", "-")
	key = strings.Trim(key, "-")
	key = strings.Join(strings.FieldsFunc(key, func(r rune) bool { return r == '-' }), "-")
	return key
}

// IsResponsesWebsocketEnabled returns true when the dedicated responses websocket
// route should be mounted. Default is enabled when unset.
func (cfg *Config) IsResponsesWebsocketEnabled() bool {
	if cfg == nil || cfg.ResponsesWebsocketEnabled == nil {
		return true
	}
	return *cfg.ResponsesWebsocketEnabled
}

// SanitizeOpenAICompatibility removes OpenAI-compatibility provider entries
// missing a BaseURL and trims whitespace.
func (cfg *Config) SanitizeOpenAICompatibility() {
	if cfg == nil || len(cfg.OpenAICompatibility) == 0 {
		return
	}
	out := make([]OpenAICompatibility, 0, len(cfg.OpenAICompatibility))
	for i := range cfg.OpenAICompatibility {
		e := cfg.OpenAICompatibility[i]
		e.Name = strings.TrimSpace(e.Name)
		e.Prefix = normalizeModelPrefix(e.Prefix)
		e.BaseURL = strings.TrimSpace(e.BaseURL)
		e.Headers = NormalizeHeaders(e.Headers)
		if e.BaseURL == "" {
			continue
		}
		out = append(out, e)
	}
	cfg.OpenAICompatibility = out
}

// SanitizeCodexKeys removes Codex API key entries missing a BaseURL.
func (cfg *Config) SanitizeCodexKeys() {
	if cfg == nil || len(cfg.CodexKey) == 0 {
		return
	}
	out := make([]CodexKey, 0, len(cfg.CodexKey))
	for i := range cfg.CodexKey {
		e := cfg.CodexKey[i]
		e.Prefix = normalizeModelPrefix(e.Prefix)
		e.BaseURL = strings.TrimSpace(e.BaseURL)
		e.Headers = NormalizeHeaders(e.Headers)
		e.ExcludedModels = NormalizeExcludedModels(e.ExcludedModels)
		if e.BaseURL == "" {
			continue
		}
		out = append(out, e)
	}
	cfg.CodexKey = out
}

// SanitizeClaudeKeys normalizes headers for Claude credentials.
func (cfg *Config) SanitizeClaudeKeys() {
	if cfg == nil || len(cfg.ClaudeKey) == 0 {
		return
	}
	for i := range cfg.ClaudeKey {
		entry := &cfg.ClaudeKey[i]
		entry.Prefix = normalizeModelPrefix(entry.Prefix)
		entry.Headers = NormalizeHeaders(entry.Headers)
		entry.ExcludedModels = NormalizeExcludedModels(entry.ExcludedModels)
	}
}

// SanitizeKiroKeys trims whitespace from Kiro credential fields.
func (cfg *Config) SanitizeKiroKeys() {
	if cfg == nil || len(cfg.KiroKey) == 0 {
		return
	}
	for i := range cfg.KiroKey {
		entry := &cfg.KiroKey[i]
		entry.TokenFile = strings.TrimSpace(entry.TokenFile)
		entry.AccessToken = strings.TrimSpace(entry.AccessToken)
		entry.RefreshToken = strings.TrimSpace(entry.RefreshToken)
		entry.ProfileArn = strings.TrimSpace(entry.ProfileArn)
		entry.Region = strings.TrimSpace(entry.Region)
		entry.ProxyURL = strings.TrimSpace(entry.ProxyURL)
		entry.PreferredEndpoint = strings.TrimSpace(entry.PreferredEndpoint)
	}
}

// SanitizeCursorKeys trims whitespace from Cursor credential fields.
func (cfg *Config) SanitizeCursorKeys() {
	if cfg == nil || len(cfg.CursorKey) == 0 {
		return
	}
	for i := range cfg.CursorKey {
		entry := &cfg.CursorKey[i]
		entry.TokenFile = strings.TrimSpace(entry.TokenFile)
		entry.CursorAPIURL = strings.TrimSpace(entry.CursorAPIURL)
		entry.AuthToken = strings.TrimSpace(entry.AuthToken)
		entry.ProxyURL = strings.TrimSpace(entry.ProxyURL)
	}
}

// SanitizeGeminiKeys deduplicates and normalizes Gemini credentials.
func (cfg *Config) SanitizeGeminiKeys() {
	if cfg == nil {
		return
	}

	seen := make(map[string]struct{}, len(cfg.GeminiKey))
	out := cfg.GeminiKey[:0]
	for i := range cfg.GeminiKey {
		entry := cfg.GeminiKey[i]
		entry.APIKey = strings.TrimSpace(entry.APIKey)
		if entry.APIKey == "" {
			continue
		}
		entry.Prefix = normalizeModelPrefix(entry.Prefix)
		entry.BaseURL = strings.TrimSpace(entry.BaseURL)
		entry.ProxyURL = strings.TrimSpace(entry.ProxyURL)
		entry.Headers = NormalizeHeaders(entry.Headers)
		entry.ExcludedModels = NormalizeExcludedModels(entry.ExcludedModels)
		if _, exists := seen[entry.APIKey]; exists {
			continue
		}
		seen[entry.APIKey] = struct{}{}
		out = append(out, entry)
	}
	cfg.GeminiKey = out
}

func normalizeModelPrefix(prefix string) string {
	trimmed := strings.TrimSpace(prefix)
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, "/") {
		return ""
	}
	return trimmed
}

// NormalizeHeaders normalizes HTTP header field names and values.
func NormalizeHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return headers
	}
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		normalized := strings.TrimSpace(k)
		value := strings.TrimSpace(v)
		if normalized == "" || value == "" {
			continue
		}
		out[normalized] = value
	}
	return out
}

// NormalizeExcludedModels normalizes excluded model names.
func NormalizeExcludedModels(models []string) []string {
	if len(models) == 0 {
		return models
	}
	seen := make(map[string]struct{}, len(models))
	out := make([]string, 0, len(models))
	for _, m := range models {
		normalized := strings.TrimSpace(m)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

// NormalizeOAuthExcludedModels normalizes per-channel excluded model maps.
func NormalizeOAuthExcludedModels(entries map[string][]string) map[string][]string {
	if len(entries) == 0 {
		return entries
	}
	out := make(map[string][]string, len(entries))
	for channel, models := range entries {
		normalized := strings.ToLower(strings.TrimSpace(channel))
		if normalized == "" {
			continue
		}
		out[normalized] = NormalizeExcludedModels(models)
	}
	return out
}
