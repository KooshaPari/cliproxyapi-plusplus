package iflow

import (
	"testing"

	"github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/registry"
	"github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/thinking"
	"github.com/tidwall/gjson"
)

func TestApplyMiniMaxStripsReasoningEffort(t *testing.T) {
	a := NewApplier()
	body := []byte(`{"reasoning_effort":"high","foo":"bar"}`)
	cfg := thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelHigh}
	model := &registry.ModelInfo{
		ID:       "minimax-m2.5",
		Thinking: &registry.ThinkingSupport{Levels: []string{"low", "medium", "high"}},
	}

	out, err := a.Apply(body, cfg, model)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res := gjson.ParseBytes(out)
	if !res.Get("reasoning_split").Bool() {
		t.Fatalf("expected reasoning_split=true for non-none config")
	}
	if res.Get("reasoning_effort").Exists() {
		t.Fatalf("expected reasoning_effort to be removed")
	}
}

func TestApplyMiniMaxSetsReasoningSplitFalseForModeNone(t *testing.T) {
	a := NewApplier()
	body := []byte(`{"reasoning_effort":"high","foo":"bar"}`)
	cfg := thinking.ThinkingConfig{Mode: thinking.ModeNone}
	model := &registry.ModelInfo{
		ID:       "minimax-m2",
		Thinking: &registry.ThinkingSupport{Levels: []string{"none", "low", "high"}},
	}

	out, err := a.Apply(body, cfg, model)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res := gjson.ParseBytes(out)
	if res.Get("reasoning_split").Bool() {
		t.Fatalf("expected reasoning_split=false for ModeNone")
	}
}

func TestApplyMiniMaxStripsReasoningVariantsAndLegacyFields(t *testing.T) {
	a := NewApplier()
	body := []byte(`{
		"reasoning_split":true,
		"reasoning_effort":"high",
		"reasoning":{"effort":"medium","summary":{"text":"legacy"}},
		"variant":"low",
		"foo":"bar"
	}`)
	cfg := thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelLow}
	model := &registry.ModelInfo{
		ID:       "minimax-m2.5",
		Thinking: &registry.ThinkingSupport{Levels: []string{"low", "medium", "high"}},
	}

	out, err := a.Apply(body, cfg, model)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res := gjson.ParseBytes(out)
	if !res.Get("reasoning_split").Bool() {
		t.Fatalf("expected reasoning_split=true")
	}
	if res.Get("reasoning_effort").Exists() {
		t.Fatalf("expected reasoning_effort to be removed")
	}
	if res.Get("reasoning").Exists() {
		t.Fatalf("expected reasoning object to be removed")
	}
	if res.Get("reasoning.effort").Exists() {
		t.Fatalf("expected reasoning.effort to be removed")
	}
	if res.Get("variant").Exists() {
		t.Fatalf("expected variant to be removed")
	}
	if res.Get("foo").String() != "bar" {
		t.Fatalf("expected unrelated fields to be preserved")
	}
}
