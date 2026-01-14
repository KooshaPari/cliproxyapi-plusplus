// Package thinking provides unified thinking configuration processing logic.
package thinking

import "testing"

// TestThinkingErrorError tests the Error() method of ThinkingError.
//
// Error() returns the message directly without code prefix.
// Use Code field for programmatic error handling.
func TestThinkingErrorError(t *testing.T) {
	tests := []struct {
		name     string
		err      *ThinkingError
		wantMsg  string
		wantCode ErrorCode
	}{
		{"invalid suffix format", NewThinkingError(ErrInvalidSuffix, "invalid suffix format: model(abc"), "invalid suffix format: model(abc", ErrInvalidSuffix},
		{"unknown level", NewThinkingError(ErrUnknownLevel, "unknown level: ultra"), "unknown level: ultra", ErrUnknownLevel},
		{"level not supported", NewThinkingError(ErrLevelNotSupported, "level \"xhigh\" not supported, valid levels: low, medium, high"), "level \"xhigh\" not supported, valid levels: low, medium, high", ErrLevelNotSupported},
		{"thinking not supported", NewThinkingErrorWithModel(ErrThinkingNotSupported, "thinking not supported for this model", "claude-haiku"), "thinking not supported for this model", ErrThinkingNotSupported},
		{"provider mismatch", NewThinkingError(ErrProviderMismatch, "provider mismatch: expected claude, got gemini"), "provider mismatch: expected claude, got gemini", ErrProviderMismatch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
			if tt.err.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", tt.err.Code, tt.wantCode)
			}
		})
	}
}
