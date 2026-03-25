package responses

import (
<<<<<<< HEAD
	antigravitygemini "github.com/router-for-me/CLIProxyAPI/v6/pkg/llmproxy/translator/antigravity/gemini"
	geminiopenai "github.com/router-for-me/CLIProxyAPI/v6/pkg/llmproxy/translator/gemini/openai/responses"
=======
	antigravitygemini "github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/translator/antigravity/gemini"
	geminiopenai "github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/translator/gemini/openai/responses"
>>>>>>> origin/main
)

func ConvertOpenAIResponsesRequestToAntigravity(modelName string, inputRawJSON []byte, stream bool) []byte {
	rawJSON := inputRawJSON
	rawJSON = geminiopenai.ConvertOpenAIResponsesRequestToGemini(modelName, rawJSON, stream)
	return antigravitygemini.ConvertGeminiRequestToAntigravity(modelName, rawJSON, stream)
}
