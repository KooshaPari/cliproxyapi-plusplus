package util

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestThinkingUtilDeprecationComments(t *testing.T) {
	dir, err := thinkingSourceDir()
	if err != nil {
		t.Fatalf("resolve thinking source dir: %v", err)
	}

	// Test thinking.go deprecation comments
	t.Run("thinking.go", func(t *testing.T) {
		docs := parseFuncDocs(t, filepath.Join(dir, "thinking.go"))
		tests := []struct {
			funcName string
			want     string
		}{
			{"ModelSupportsThinking", "Deprecated: Use thinking.ApplyThinking with modelInfo.Thinking check."},
			{"NormalizeThinkingBudget", "Deprecated: Use thinking.ValidateConfig for budget normalization."},
			{"ThinkingEffortToBudget", "Deprecated: Use thinking.ConvertLevelToBudget instead."},
			{"ThinkingBudgetToEffort", "Deprecated: Use thinking.ConvertBudgetToLevel instead."},
			{"GetModelThinkingLevels", "Deprecated: Access modelInfo.Thinking.Levels directly."},
			{"ModelUsesThinkingLevels", "Deprecated: Check len(modelInfo.Thinking.Levels) > 0."},
			{"NormalizeReasoningEffortLevel", "Deprecated: Use thinking.ValidateConfig for level validation."},
			{"IsOpenAICompatibilityModel", "Deprecated: Check modelInfo.Type == \"openai-compatibility\"."},
			{"ThinkingLevelToBudget", "Deprecated: Use thinking.ConvertLevelToBudget instead."},
		}
		for _, tt := range tests {
			t.Run(tt.funcName, func(t *testing.T) {
				doc, ok := docs[tt.funcName]
				if !ok {
					t.Fatalf("missing function %q in thinking.go", tt.funcName)
				}
				if !strings.Contains(doc, tt.want) {
					t.Fatalf("missing deprecation note for %s: want %q, got %q", tt.funcName, tt.want, doc)
				}
			})
		}
	})

	// Test thinking_suffix.go deprecation comments
	t.Run("thinking_suffix.go", func(t *testing.T) {
		docs := parseFuncDocs(t, filepath.Join(dir, "thinking_suffix.go"))
		tests := []struct {
			funcName string
			want     string
		}{
			{"NormalizeThinkingModel", "Deprecated: Use thinking.ParseSuffix instead."},
			{"ThinkingFromMetadata", "Deprecated: Access ThinkingConfig fields directly."},
			{"ResolveThinkingConfigFromMetadata", "Deprecated: Use thinking.ApplyThinking instead."},
			{"ReasoningEffortFromMetadata", "Deprecated: Use thinking.ConvertBudgetToLevel instead."},
			{"ResolveOriginalModel", "Deprecated: Parse model suffix with thinking.ParseSuffix."},
		}
		for _, tt := range tests {
			t.Run(tt.funcName, func(t *testing.T) {
				doc, ok := docs[tt.funcName]
				if !ok {
					t.Fatalf("missing function %q in thinking_suffix.go", tt.funcName)
				}
				if !strings.Contains(doc, tt.want) {
					t.Fatalf("missing deprecation note for %s: want %q, got %q", tt.funcName, tt.want, doc)
				}
			})
		}
	})

	// Test thinking_text.go deprecation comments
	t.Run("thinking_text.go", func(t *testing.T) {
		docs := parseFuncDocs(t, filepath.Join(dir, "thinking_text.go"))
		tests := []struct {
			funcName string
			want     string
		}{
			{"GetThinkingText", "Deprecated: Use thinking package for thinking text extraction."},
			{"GetThinkingTextFromJSON", "Deprecated: Use thinking package for thinking text extraction."},
			{"SanitizeThinkingPart", "Deprecated: Use thinking package for thinking part sanitization."},
			{"StripCacheControl", "Deprecated: Use thinking package for cache control stripping."},
		}
		for _, tt := range tests {
			t.Run(tt.funcName, func(t *testing.T) {
				doc, ok := docs[tt.funcName]
				if !ok {
					t.Fatalf("missing function %q in thinking_text.go", tt.funcName)
				}
				if !strings.Contains(doc, tt.want) {
					t.Fatalf("missing deprecation note for %s: want %q, got %q", tt.funcName, tt.want, doc)
				}
			})
		}
	})
}

func parseFuncDocs(t *testing.T, path string) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	docs := map[string]string{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil {
			continue
		}
		if fn.Doc == nil {
			docs[fn.Name.Name] = ""
			continue
		}
		docs[fn.Name.Name] = fn.Doc.Text()
	}
	return docs
}

func thinkingSourceDir() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", os.ErrNotExist
	}
	return filepath.Dir(thisFile), nil
}
