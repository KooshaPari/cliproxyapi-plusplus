// Copyright 2025 Your Name
// SPDX-License-Identifier: Apache-2.0

package executor

import (
import (
	"testing"

	config "github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/config"
)
func TestPayloadModelRulesMatch_Unconditional(t *testing.T) {
	// Rule with empty Name is unconditional - matches any model
	rules := []config.PayloadModelRule{
		{Name: "", Protocol: ""},
	}

	// Unconditional rule should match any model
	if !payloadModelRulesMatch(rules, "", []string{"claude-3-5-sonnet"}) {
		t.Errorf("Expected unconditional rule to match claude-3-5-sonnet")
	}

	if !payloadModelRulesMatch(rules, "", []string{"gpt-4"}) {
		t.Errorf("Expected unconditional rule to match gpt-4")
	}

	if !payloadModelRulesMatch(rules, "", []string{}) {
		t.Errorf("Expected unconditional rule to match empty models")
	}
}

// Test that rules with empty Name but specific protocol only match that protocol
func TestPayloadModelRulesMatch_UnconditionalWithProtocol(t *testing.T) {
	// Rule with empty Name but specific protocol
	rules := []config.PayloadModelRule{
		{Name: "", Protocol: "gemini"},
	}

	// Should match when protocol matches
	if !payloadModelRulesMatch(rules, "gemini", []string{"any-model"}) {
		t.Errorf("Expected unconditional rule to match gemini protocol")
	}

	// Should NOT match when protocol doesn't match
	if payloadModelRulesMatch(rules, "openai", []string{"any-model"}) {
		t.Errorf("Expected unconditional rule to NOT match openai protocol")
	}
}

// Test conditional rules - rules with specific Name only match that model
func TestPayloadModelRulesMatch_Conditional(t *testing.T) {
	rules := []config.PayloadModelRule{
		{Name: "claude-3-5", Protocol: ""},
	}

	// Should match when model contains rule.Name
	if !payloadModelRulesMatch(rules, "", []string{"claude-3-5-sonnet"}) {
		t.Errorf("Expected claude-3-5-sonnet to match rule.Name claude-3-5")
	}

	// Should NOT match when model doesn't contain rule.Name
	if payloadModelRulesMatch(rules, "", []string{"gpt-4"}) {
		t.Errorf("Expected gpt-4 to NOT match rule.Name claude-3-5")
	}
}

// Test contains matching - "claude-3-5-sonnet" contains "claude-3-5"
func TestPayloadModelRulesMatch_ContainsMatch(t *testing.T) {
	rules := []config.PayloadModelRule{
		{Name: "claude-3", Protocol: ""},
	}

	// Contains: "claude-3-5" contains "claude-3" -> true
	if !payloadModelRulesMatch(rules, "", []string{"claude-3-5"}) {
		t.Errorf("Expected claude-3-5 to match rule containing claude-3")
	}

	// Contains: "claude-3" contains "claude-3" -> true
	if !payloadModelRulesMatch(rules, "", []string{"claude-3"}) {
		t.Errorf("Expected claude-3 to match rule containing claude-3")
	}

	// Not contained: "claude-3-5-sonnet" does NOT contain "claude-3-5" -> false
	if payloadModelRulesMatch(rules, "", []string{"claude-3-5-sonnet"}) {
		t.Errorf("Expected claude-3-5-sonnet to NOT match rule containing claude-3-5")
	}
}

// Test protocol matching
func TestPayloadModelRulesMatch_ProtocolMatching(t *testing.T) {
	rules := []config.PayloadModelRule{
		{Name: "claude-3", Protocol: "claude"},
	}

	// Should match when both model and protocol match
	if !payloadModelRulesMatch(rules, "claude", []string{"claude-3-5-sonnet"}) {
		t.Errorf("Expected match when model and protocol match")
	}

	// Should NOT match when protocol doesn't match
	if payloadModelRulesMatch(rules, "openai", []string{"claude-3-5-sonnet"}) {
		t.Errorf("Expected NO match when protocol doesn't match")
	}
}

// Test multiple models
func TestPayloadModelRulesMatch_MultipleModels(t *testing.T) {
	rules := []config.PayloadModelRule{
		{Name: "claude-3", Protocol: ""},
	}

	// Should match if any model matches
	if !payloadModelRulesMatch(rules, "", []string{"gpt-4", "claude-3-5-sonnet", "gemini"}) {
		t.Errorf("Expected match when any model contains claude-3")
	}

	// Should NOT match if no model contains the rule name
	if payloadModelRulesMatch(rules, "", []string{"gpt-4", "gemini"}) {
		t.Errorf("Expected NO match when no model contains claude-3")
	}
}

// Test multiple rules
func TestPayloadModelRulesMatch_MultipleRules(t *testing.T) {
	rules := []config.PayloadModelRule{
		{Name: "claude-3", Protocol: ""},
		{Name: "gpt-4", Protocol: ""},
	}

	// Should match if any rule matches
	if !payloadModelRulesMatch(rules, "", []string{"claude-3-5-sonnet"}) {
		t.Errorf("Expected match for claude rule")
	}

	if !payloadModelRulesMatch(rules, "", []string{"gpt-4-turbo"}) {
		t.Errorf("Expected match for gpt-4 rule")
	}
}

// Test empty rules
func TestPayloadModelRulesMatch_EmptyRules(t *testing.T) {
	rules := []config.PayloadModelRule{}

	if payloadModelRulesMatch(rules, "", []string{"claude-3-5-sonnet"}) {
		t.Errorf("Expected NO match for empty rules")
	}
}

// Test empty models with specific rule
func TestPayloadModelRulesMatch_EmptyModelsWithConditionalRule(t *testing.T) {
	rules := []config.PayloadModelRule{
		{Name: "claude-3", Protocol: ""},
	}

	// Empty models should NOT match conditional rule
	if payloadModelRulesMatch(rules, "", []string{}) {
		t.Errorf("Expected NO match for empty models with conditional rule")
	}
}

// Test suffix matching - model with @ suffix
func TestPayloadModelRulesMatch_SuffixMatching(t *testing.T) {
	rules := []config.PayloadModelRule{
		{Name: "claude-3-5", Protocol: ""},
	}

	// Model with @ suffix should still match
	if !payloadModelRulesMatch(rules, "", []string{"claude-3-5@fork"}) {
		t.Errorf("Expected claude-3-5@fork to match rule.Name claude-3-5")
	}
}

// Test exact match with @ suffix
func TestPayloadModelRulesMatch_ExactSuffix(t *testing.T) {
	rules := []config.PayloadModelRule{
		{Name: "claude-3-5@fork", Protocol: ""},
	}

	// Exact suffix match
	if !payloadModelRulesMatch(rules, "", []string{"claude-3-5@fork"}) {
		t.Errorf("Expected exact match claude-3-5@fork")
	}

	// Should NOT match without @ suffix
	if payloadModelRulesMatch(rules, "", []string{"claude-3-5"}) {
		t.Errorf("Expected NO match for claude-3-5 without @ suffix")
	}
}

// Test whitespace handling
func TestPayloadModelRulesMatch_Whitespace(t *testing.T) {
	rules := []config.PayloadModelRule{
		{Name: "  claude-3  ", Protocol: ""},
	}

	// Whitespace should be trimmed
	if !payloadModelRulesMatch(rules, "", []string{"claude-3"}) {
		t.Errorf("Expected whitespace trimmed to match")
	}
}
