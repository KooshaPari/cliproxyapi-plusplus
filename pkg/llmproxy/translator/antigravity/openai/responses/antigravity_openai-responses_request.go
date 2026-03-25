package responses

import (
	antigravitygemini "github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/translator/antigravity/gemini"
	geminiopenai "github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/translator/gemini/openai/responses"
)

func ConvertOpenAIResponsesRequestToAntigravity(modelName string, inputRawJSON []byte, stream bool) []byte {
	rawJSON := inputRawJSON
	rawJSON = geminiopenai.ConvertOpenAIResponsesRequestToGemini(modelName, rawJSON, stream)
	return antigravitygemini.ConvertGeminiRequestToAntigravity(modelName, rawJSON, stream)
}
