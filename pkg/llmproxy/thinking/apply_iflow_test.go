package thinking

import "testing"

func TestExtractIFlowConfig_ReasoningEffort(t *testing.T) {
	tests := []struct {
		name string
		body string
		want ThinkingConfig
	}{
		{
			name: "nested reasoning.effort maps to level",
			body: `{"reasoning":{"effort":"high"}}`,
			want: ThinkingConfig{Mode: ModeLevel, Level: LevelHigh},
		},
		{
			name: "literal reasoning.effort key maps to level",
			body: `{"reasoning.effort":"high"}`,
			want: ThinkingConfig{Mode: ModeLevel, Level: LevelHigh},
		},
		{
			name: "none maps to disabled",
			body: `{"reasoning.effort":"none"}`,
			want: ThinkingConfig{Mode: ModeNone, Budget: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractIFlowConfig([]byte(tt.body))
			if got != tt.want {
				t.Fatalf("got=%+v want=%+v", got, tt.want)
			}
		})
	}
}

func TestExtractIFlowConfig_ReasoningObjectEffort(t *testing.T) {
	tests := []struct {
		name string
		body string
		want ThinkingConfig
	}{
		{
			name: "reasoning object effort maps to level",
			body: `{"reasoning":{"effort":"medium"}}`,
			want: ThinkingConfig{Mode: ModeLevel, Level: LevelMedium},
		},
		{
			name: "empty effort falls back to empty config",
			body: `{"reasoning":{"effort":""}}`,
			want: ThinkingConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractIFlowConfig([]byte(tt.body))
			if got != tt.want {
				t.Fatalf("got=%+v want=%+v", got, tt.want)
			}
		})
	}
}
