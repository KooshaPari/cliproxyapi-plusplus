// Package thinking provides unified thinking configuration processing.
package thinking

import (
	"strings"
	"testing"
)

// TestParseSuffix tests the ParseSuffix function.
//
// ParseSuffix extracts thinking suffix from model name.
// Format: model-name(value) where value is the raw suffix content.
// This function only extracts; interpretation is done by other functions.
func TestParseSuffix(t *testing.T) {
	tests := []struct {
		name       string
		model      string
		wantModel  string
		wantSuffix bool
		wantRaw    string
	}{
		{"no suffix", "claude-sonnet-4-5", "claude-sonnet-4-5", false, ""},
		{"numeric suffix", "model(1000)", "model", true, "1000"},
		{"level suffix", "gpt-5(high)", "gpt-5", true, "high"},
		{"auto suffix", "gemini-2.5-pro(auto)", "gemini-2.5-pro", true, "auto"},
		{"none suffix", "model(none)", "model", true, "none"},
		{"complex model name", "gemini-2.5-flash-lite(8192)", "gemini-2.5-flash-lite", true, "8192"},
		{"alias with suffix", "g25p(1000)", "g25p", true, "1000"},
		{"empty suffix", "model()", "model", true, ""},
		{"nested parens", "model(a(b))", "model(a", true, "b)"},
		{"no model name", "(1000)", "", true, "1000"},
		{"unmatched open", "model(", "model(", false, ""},
		{"unmatched close", "model)", "model)", false, ""},
		{"paren not at end", "model(1000)extra", "model(1000)extra", false, ""},
		{"empty string", "", "", false, ""},
		{"large budget", "claude-opus(128000)", "claude-opus", true, "128000"},
		{"xhigh level", "gpt-5.2(xhigh)", "gpt-5.2", true, "xhigh"},
		{"minimal level", "model(minimal)", "model", true, "minimal"},
		{"medium level", "model(medium)", "model", true, "medium"},
		{"low level", "model(low)", "model", true, "low"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseSuffix(tt.model)
			if got.ModelName != tt.wantModel {
				t.Errorf("ModelName = %q, want %q", got.ModelName, tt.wantModel)
			}
			if got.HasSuffix != tt.wantSuffix {
				t.Errorf("HasSuffix = %v, want %v", got.HasSuffix, tt.wantSuffix)
			}
			if got.RawSuffix != tt.wantRaw {
				t.Errorf("RawSuffix = %q, want %q", got.RawSuffix, tt.wantRaw)
			}
		})
	}
}

// TestParseSuffixWithError tests invalid suffix error reporting.
func TestParseSuffixWithError(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		wantHasSuffix bool
	}{
		{"missing close paren", "model(abc", false},
		{"unmatched close paren", "model)", false},
		{"paren not at end", "model(1000)extra", false},
		{"no suffix", "gpt-5", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSuffixWithError(tt.model)
			if tt.name == "no suffix" {
				if err != nil {
					t.Fatalf("ParseSuffixWithError(%q) error = %v, want nil", tt.model, err)
				}
				if got.HasSuffix != tt.wantHasSuffix {
					t.Errorf("HasSuffix = %v, want %v", got.HasSuffix, tt.wantHasSuffix)
				}
				return
			}

			if err == nil {
				t.Fatalf("ParseSuffixWithError(%q) error = nil, want error", tt.model)
			}
			thinkingErr, ok := err.(*ThinkingError)
			if !ok {
				t.Fatalf("ParseSuffixWithError(%q) error type = %T, want *ThinkingError", tt.model, err)
			}
			if thinkingErr.Code != ErrInvalidSuffix {
				t.Errorf("error code = %v, want %v", thinkingErr.Code, ErrInvalidSuffix)
			}
			if !strings.Contains(thinkingErr.Message, tt.model) {
				t.Errorf("message %q does not include input %q", thinkingErr.Message, tt.model)
			}
			if got.HasSuffix != tt.wantHasSuffix {
				t.Errorf("HasSuffix = %v, want %v", got.HasSuffix, tt.wantHasSuffix)
			}
		})
	}
}

// TestParseSuffixNumeric tests numeric suffix parsing.
//
// ParseNumericSuffix parses raw suffix content as integer budget.
// Only non-negative integers are valid. Negative numbers return ok=false.
func TestParseSuffixNumeric(t *testing.T) {
	tests := []struct {
		name       string
		rawSuffix  string
		wantBudget int
		wantOK     bool
	}{
		{"small budget", "512", 512, true},
		{"standard budget", "8192", 8192, true},
		{"large budget", "100000", 100000, true},
		{"max int32", "2147483647", 2147483647, true},
		{"max int64", "9223372036854775807", 9223372036854775807, true},
		{"zero", "0", 0, true},
		{"negative one", "-1", 0, false},
		{"negative", "-100", 0, false},
		{"int64 overflow", "9223372036854775808", 0, false},
		{"large overflow", "99999999999999999999", 0, false},
		{"not a number", "abc", 0, false},
		{"level string", "high", 0, false},
		{"float", "1.5", 0, false},
		{"empty", "", 0, false},
		{"leading zero", "08192", 8192, true},
		{"whitespace", "  8192  ", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			budget, ok := ParseNumericSuffix(tt.rawSuffix)
			if budget != tt.wantBudget {
				t.Errorf("budget = %d, want %d", budget, tt.wantBudget)
			}
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
		})
	}
}

// TestParseSuffixLevel tests level suffix parsing.
//
// ParseLevelSuffix parses raw suffix content as discrete thinking level.
// Only effort levels (minimal, low, medium, high, xhigh) are valid.
// Special values (none, auto) return ok=false - use ParseSpecialSuffix instead.
func TestParseSuffixLevel(t *testing.T) {
	tests := []struct {
		name      string
		rawSuffix string
		wantLevel ThinkingLevel
		wantOK    bool
	}{
		{"minimal", "minimal", LevelMinimal, true},
		{"low", "low", LevelLow, true},
		{"medium", "medium", LevelMedium, true},
		{"high", "high", LevelHigh, true},
		{"xhigh", "xhigh", LevelXHigh, true},
		{"case HIGH", "HIGH", LevelHigh, true},
		{"case High", "High", LevelHigh, true},
		{"case hIgH", "hIgH", LevelHigh, true},
		{"case MINIMAL", "MINIMAL", LevelMinimal, true},
		{"case XHigh", "XHigh", LevelXHigh, true},
		{"none special", "none", "", false},
		{"auto special", "auto", "", false},
		{"unknown ultra", "ultra", "", false},
		{"unknown maximum", "maximum", "", false},
		{"unknown invalid", "invalid", "", false},
		{"numeric", "8192", "", false},
		{"numeric zero", "0", "", false},
		{"empty", "", "", false},
		{"whitespace", "  high  ", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, ok := ParseLevelSuffix(tt.rawSuffix)
			if level != tt.wantLevel {
				t.Errorf("level = %q, want %q", level, tt.wantLevel)
			}
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
		})
	}
}

// TestParseSuffixSpecialValues tests special value suffix parsing.
//
// Depends on: Epic 3 Story 3-4 (special value suffix parsing)
func TestParseSuffixSpecialValues(t *testing.T) {
	tests := []struct {
		name      string
		rawSuffix string
		wantMode  ThinkingMode
		wantOK    bool
	}{
		{"none", "none", ModeNone, true},
		{"auto", "auto", ModeAuto, true},
		{"negative one", "-1", ModeAuto, true},
		{"case NONE", "NONE", ModeNone, true},
		{"case Auto", "Auto", ModeAuto, true},
		{"case aUtO", "aUtO", ModeAuto, true},
		{"case NoNe", "NoNe", ModeNone, true},
		{"empty", "", ModeBudget, false},
		{"level high", "high", ModeBudget, false},
		{"numeric", "8192", ModeBudget, false},
		{"negative other", "-2", ModeBudget, false},
		{"whitespace", "  none  ", ModeBudget, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, ok := ParseSpecialSuffix(tt.rawSuffix)
			if mode != tt.wantMode {
				t.Errorf("mode = %q, want %q", mode, tt.wantMode)
			}
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
		})
	}
}

// TestParseSuffixAliasFormats tests alias model suffix parsing.
//
// This test validates that short model aliases (e.g., g25p, cs45) work correctly
// with all suffix types. Alias-to-canonical-model mapping is caller's responsibility.
func TestParseSuffixAliasFormats(t *testing.T) {
	tests := []struct {
		name        string        // test case description
		model       string        // input model string with optional suffix
		wantName    string        // expected ModelName after parsing
		wantSuffix  bool          // expected HasSuffix value
		wantRaw     string        // expected RawSuffix value
		checkBudget bool          // if true, verify ParseNumericSuffix result
		wantBudget  int           // expected budget (only when checkBudget=true)
		checkLevel  bool          // if true, verify ParseLevelSuffix result
		wantLevel   ThinkingLevel // expected level (only when checkLevel=true)
		checkMode   bool          // if true, verify ParseSpecialSuffix result
		wantMode    ThinkingMode  // expected mode (only when checkMode=true)
	}{
		// Alias + numeric suffix
		{"alias numeric g25p", "g25p(1000)", "g25p", true, "1000", true, 1000, false, "", false, 0},
		{"alias numeric cs45", "cs45(16384)", "cs45", true, "16384", true, 16384, false, "", false, 0},
		{"alias numeric g3f", "g3f(8192)", "g3f", true, "8192", true, 8192, false, "", false, 0},
		// Alias + level suffix
		{"alias level gpt52", "gpt52(high)", "gpt52", true, "high", false, 0, true, LevelHigh, false, 0},
		{"alias level g25f", "g25f(medium)", "g25f", true, "medium", false, 0, true, LevelMedium, false, 0},
		{"alias level cs4", "cs4(low)", "cs4", true, "low", false, 0, true, LevelLow, false, 0},
		// Alias + special suffix
		{"alias auto g3f", "g3f(auto)", "g3f", true, "auto", false, 0, false, "", true, ModeAuto},
		{"alias none claude", "claude(none)", "claude", true, "none", false, 0, false, "", true, ModeNone},
		{"alias -1 g25p", "g25p(-1)", "g25p", true, "-1", false, 0, false, "", true, ModeAuto},
		// Single char alias
		{"single char c", "c(1024)", "c", true, "1024", true, 1024, false, "", false, 0},
		{"single char g", "g(high)", "g", true, "high", false, 0, true, LevelHigh, false, 0},
		// Alias containing numbers
		{"alias with num gpt5", "gpt5(medium)", "gpt5", true, "medium", false, 0, true, LevelMedium, false, 0},
		{"alias with num g25", "g25(1000)", "g25", true, "1000", true, 1000, false, "", false, 0},
		// Edge cases
		{"no suffix", "g25p", "g25p", false, "", false, 0, false, "", false, 0},
		{"empty alias", "(1000)", "", true, "1000", true, 1000, false, "", false, 0},
		{"hyphen alias", "g-25-p(1000)", "g-25-p", true, "1000", true, 1000, false, "", false, 0},
		{"underscore alias", "g_25_p(high)", "g_25_p", true, "high", false, 0, true, LevelHigh, false, 0},
		{"nested parens", "g25p(test)(1000)", "g25p(test)", true, "1000", true, 1000, false, "", false, 0},
	}

	// ParseSuffix only extracts alias and suffix; mapping to canonical model is caller responsibility.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseSuffix(tt.model)

			if result.ModelName != tt.wantName {
				t.Errorf("ParseSuffix(%q).ModelName = %q, want %q", tt.model, result.ModelName, tt.wantName)
			}
			if result.HasSuffix != tt.wantSuffix {
				t.Errorf("ParseSuffix(%q).HasSuffix = %v, want %v", tt.model, result.HasSuffix, tt.wantSuffix)
			}
			if result.RawSuffix != tt.wantRaw {
				t.Errorf("ParseSuffix(%q).RawSuffix = %q, want %q", tt.model, result.RawSuffix, tt.wantRaw)
			}

			if result.HasSuffix {
				if tt.checkBudget {
					budget, ok := ParseNumericSuffix(result.RawSuffix)
					if !ok || budget != tt.wantBudget {
						t.Errorf("ParseNumericSuffix(%q) = (%d, %v), want (%d, true)",
							result.RawSuffix, budget, ok, tt.wantBudget)
					}
				}
				if tt.checkLevel {
					level, ok := ParseLevelSuffix(result.RawSuffix)
					if !ok || level != tt.wantLevel {
						t.Errorf("ParseLevelSuffix(%q) = (%q, %v), want (%q, true)",
							result.RawSuffix, level, ok, tt.wantLevel)
					}
				}
				if tt.checkMode {
					mode, ok := ParseSpecialSuffix(result.RawSuffix)
					if !ok || mode != tt.wantMode {
						t.Errorf("ParseSpecialSuffix(%q) = (%v, %v), want (%v, true)",
							result.RawSuffix, mode, ok, tt.wantMode)
					}
				}
			}
		})
	}
}
