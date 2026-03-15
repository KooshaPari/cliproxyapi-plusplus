package openai

import (
	"strings"

	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/registry"
)

const (
	openAIChatEndpoint      = "/chat/completions"
	openAIResponsesEndpoint = "/responses"
)

func resolveEndpointOverride(modelName, requestedEndpoint, _ string) (string, bool) {
	if modelName == "" {
		return "", false
	}

	reg := registry.GetGlobalRegistry()
	for _, provider := range reg.GetModelProviders(modelName) {
		if override, ok := resolveEndpointOverrideForInfo(reg.GetModelInfo(modelName, provider), requestedEndpoint); ok {
			return override, true
		}
	}

	return resolveEndpointOverrideForInfo(reg.GetModelInfo(modelName, ""), requestedEndpoint)
}

func resolveEndpointOverrideForInfo(info *registry.ModelInfo, requestedEndpoint string) (string, bool) {
	if info == nil || len(info.SupportedEndpoints) == 0 {
		return "", false
	}
	if endpointListContains(info.SupportedEndpoints, requestedEndpoint) {
		return "", false
	}
	if requestedEndpoint == openAIChatEndpoint && endpointListContains(info.SupportedEndpoints, openAIResponsesEndpoint) {
		return openAIResponsesEndpoint, true
	}
	if requestedEndpoint == openAIResponsesEndpoint && endpointListContains(info.SupportedEndpoints, openAIChatEndpoint) {
		return openAIChatEndpoint, true
	}
	return "", false
}

func endpointListContains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func shouldForceNonStreamingChatBridge(modelName string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(modelName)), "minimax")
}
