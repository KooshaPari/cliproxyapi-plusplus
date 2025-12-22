package chat_completions

import (
	"os"
	"strings"
	"testing"

	responsesconverter "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/openai/openai/responses"
	"github.com/tidwall/gjson"
)

func TestResponsesPayloadToolsArePreserved(t *testing.T) {
	data, err := os.ReadFile("../../../../../error1.log")
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	var requestLine string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "{\"user\"") {
			requestLine = trimmed
			break
		}
	}
	if requestLine == "" {
		t.Fatalf("failed to extract request body from log")
	}

	raw := []byte(requestLine)
	chatPayload := responsesconverter.ConvertOpenAIResponsesRequestToOpenAIChatCompletions("gpt-5.1-codex-max(xhigh)", raw, true)
	codexPayload := ConvertOpenAIRequestToCodex("gpt-5.1-codex-max(xhigh)", chatPayload, true)

	tools := gjson.GetBytes(codexPayload, "tools")
	if !tools.IsArray() || len(tools.Array()) == 0 {
		t.Fatalf("expected tools array, got: %s", tools.Raw)
	}
	for i, tool := range tools.Array() {
		if name := strings.TrimSpace(tool.Get("name").String()); name == "" {
			t.Fatalf("tool %d missing name after conversion: %s", i, tool.Raw)
		}
	}
}
