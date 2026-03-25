package responses

import (
<<<<<<< HEAD
	geminicligemini "github.com/router-for-me/CLIProxyAPI/v6/pkg/llmproxy/translator/gemini-cli/gemini"
	geminiopenai "github.com/router-for-me/CLIProxyAPI/v6/pkg/llmproxy/translator/gemini/openai/responses"
=======
	geminicligemini "github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/translator/gemini-cli/gemini"
	geminiopenai "github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/translator/gemini/openai/responses"
>>>>>>> origin/main
)

func ConvertOpenAIResponsesRequestToGeminiCLI(modelName string, inputRawJSON []byte, stream bool) []byte {
	rawJSON := inputRawJSON
	rawJSON = geminiopenai.ConvertOpenAIResponsesRequestToGemini(modelName, rawJSON, stream)
	return geminicligemini.ConvertGeminiRequestToGeminiCLI(modelName, rawJSON, stream)
}
