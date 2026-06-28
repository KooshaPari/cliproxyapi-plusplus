// Package registry provides model definitions for various AI service providers.
package registry

// GetKiloModels returns the Kilo model definitions
func GetKiloModels() []*ModelInfo {
	return []*ModelInfo{
		// --- Base Models ---
		{
			ID:                  "kilo/auto",
			Object:              "model",
			Created:             1732752000,
			OwnedBy:             "kilo",
			Type:                "kilo",
			DisplayName:         "Kilo Auto",
			Description:         "Automatic model selection by Kilo",
			ContextLength:       200000,
			MaxCompletionTokens: 64000,
			Thinking:            &ThinkingSupport{Min: 1024, Max: 32000, ZeroAllowed: true, DynamicAllowed: true},
		},
	}
}

// GetKiroModels returns the Kiro model definitions.
func GetKiroModels() []*ModelInfo {
	return []*ModelInfo{
		{
			ID:                  "kiro-claude-opus-4-6",
			Object:              "model",
			Created:             1764547200,
			OwnedBy:             "aws",
			Type:                "kiro",
			DisplayName:         "Kiro Claude Opus 4.6",
			Description:         "Kiro default Claude Opus model.",
			ContextLength:       DefaultKiroContextLength,
			MaxCompletionTokens: DefaultKiroMaxCompletionTokens,
			Thinking:            cloneThinkingSupport(DefaultKiroThinkingSupport),
		},
	}
}

// GetCursorModels returns the Cursor model definitions.
func GetCursorModels() []*ModelInfo {
	return []*ModelInfo{
		{
			ID:                  "default",
			Object:              "model",
			Created:             1732752000,
			OwnedBy:             "cursor",
			Type:                "cursor",
			DisplayName:         "Cursor Default",
			Description:         "Cursor default model selection.",
			ContextLength:       200000,
			MaxCompletionTokens: 64000,
		},
	}
}
