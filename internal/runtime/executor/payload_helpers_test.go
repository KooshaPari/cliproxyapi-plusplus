package executor

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestPayloadModelRulesMatch(t *testing.T) {
	tests := []struct {
		name     string
		rules    []config.PayloadModelRule
		protocol string
		models   []string
		want     bool
	}{
		{
			name:     "no rules",
			rules:    nil,
			protocol: "anthropic",
			models:   []string{"claude-3-5-sonnet"},
			want:     false,
		},
		{
			name:     "empty rules",
			rules:    []config.PayloadModelRule{},
			protocol: "anthropic",
			models:   []string{"claude-3-5-sonnet"},
			want:     false,
		},
		{
			name:     "unconditional rule matches",
			rules:    []config.PayloadModelRule{{Name: ""}},
			protocol: "anthropic",
			models:   []string{"claude-3-5-sonnet"},
			want:     true,
		},
		{
			name:     "specific model matches",
			rules:    []config.PayloadModelRule{{Name: "claude-3-5-sonnet"}},
			protocol: "anthropic",
			models:   []string{"claude-3-5-sonnet"},
			want:     true,
		},
		{
			name:     "specific model no match",
			rules:    []config.PayloadModelRule{{Name: "claude-3-5-sonnet"}},
			protocol: "anthropic",
			models:   []string{"other-model"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := payloadModelRulesMatch(tt.rules, tt.protocol, tt.models); got != tt.want {
				t.Errorf("payloadModelRulesMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}
