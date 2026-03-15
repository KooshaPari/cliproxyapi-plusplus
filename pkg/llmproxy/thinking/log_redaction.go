package thinking

import (
	"strings"
)

const redactedLogValue = "[REDACTED]"

func redactLogText(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return redactedLogValue
}

func redactLogInt(_ int) string {
	return redactedLogValue
}

func redactLogMode(_ ThinkingMode) string {
	return redactedLogValue
}

func redactLogLevel(_ ThinkingLevel) string {
	return redactedLogValue
}
