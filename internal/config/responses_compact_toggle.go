package config

// IsResponsesCompactEnabled reports whether /v1/responses/compact is enabled.
// Default is true when config or toggle is unset.
func (c *Config) IsResponsesCompactEnabled() bool {
	if c == nil || c.ResponsesCompactEnabled == nil {
		return true
	}
	return *c.ResponsesCompactEnabled
}

