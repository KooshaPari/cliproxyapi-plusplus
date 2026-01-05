package codex

import (
	"fmt"
	"strings"
	"unicode"
)

// CredentialFileName returns the filename used to persist Codex OAuth credentials.
// When planType is available (e.g. "plus", "team"), it is appended after the email
// as a suffix to disambiguate subscriptions.
func CredentialFileName(email, planType string, includeProviderPrefix bool) string {
	email = strings.TrimSpace(email)
	plan := normalizePlanTypeForFilename(planType)

	prefix := ""
	if includeProviderPrefix {
		prefix = "codex-"
	}

	if plan == "" {
		return fmt.Sprintf("%s%s.json", prefix, email)
	}
	return fmt.Sprintf("%s%s-%s.json", prefix, email, plan)
}

func normalizePlanTypeForFilename(planType string) string {
	planType = strings.TrimSpace(planType)
	if planType == "" {
		return ""
	}

	parts := strings.FieldsFunc(planType, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(parts) == 0 {
		return ""
	}

	for i, part := range parts {
		parts[i] = titleToken(part)
	}
	return strings.Join(parts, "-")
}

func titleToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	lower := strings.ToLower(token)
	runes := []rune(lower)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
