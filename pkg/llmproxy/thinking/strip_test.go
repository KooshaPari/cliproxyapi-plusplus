package thinking

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestStripThinkingConfigIflow(t *testing.T) {
	body := []byte(`{
		"model":"minimax-m2.5",
		"reasoning":{"effort":"high"},
		"reasoning_effort":"high",
		"reasoning_split":true,
		"variant":"medium",
		"chat_template_kwargs":{"enable_thinking":true,"clear_thinking":false},
		"messages":[{"role":"user","content":"hi"}]
	}`)

	out := StripThinkingConfig(body, "iflow")
	res := gjson.ParseBytes(out)

	if res.Get("reasoning").Exists() {
		t.Fatalf("expected reasoning to be removed")
	}
	if res.Get("reasoning.effort").Exists() {
		t.Fatalf("expected reasoning.effort to be removed")
	}
	if res.Get("reasoning_split").Exists() {
		t.Fatalf("expected reasoning_split to be removed")
	}
	if res.Get("reasoning_effort").Exists() {
		t.Fatalf("expected reasoning_effort to be removed")
	}
	if res.Get("variant").Exists() {
		t.Fatalf("expected variant to be removed")
	}
	if res.Get("chat_template_kwargs").Exists() {
		t.Fatalf("expected chat_template_kwargs to be removed")
	}
	if res.Get("messages.0.content").String() != "hi" {
		t.Fatalf("expected unrelated messages to remain")
	}
}

func TestStripThinkingConfigIflowPreservesUnrelatedChatTemplateKwargs(t *testing.T) {
	body := []byte(`{
		"chat_template_kwargs":{
			"enable_thinking":true,
			"clear_thinking":false,
			"system_prompt":"keep-me"
		},
		"messages":[{"role":"user","content":"hi"}]
	}`)

	out := StripThinkingConfig(body, "iflow")
	res := gjson.ParseBytes(out)

	if res.Get("chat_template_kwargs.enable_thinking").Exists() {
		t.Fatalf("expected enable_thinking to be removed")
	}
	if res.Get("chat_template_kwargs.clear_thinking").Exists() {
		t.Fatalf("expected clear_thinking to be removed")
	}
	if res.Get("chat_template_kwargs.system_prompt").String() != "keep-me" {
		t.Fatalf("expected unrelated chat_template_kwargs fields to remain")
	}
	if res.Get("messages.0.content").String() != "hi" {
		t.Fatalf("expected unrelated messages to remain")
	}
}
