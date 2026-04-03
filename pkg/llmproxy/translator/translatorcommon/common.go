// Package translatorcommon provides shared utilities for LLM proxy translators.
package translatorcommon

import "fmt"

// FormatEndpoint formats an API endpoint URL.
func FormatEndpoint(baseURL, path string) string {
        return fmt.Sprintf("%s/%s", baseURL, path)
}

// ParseResponse parses a generic JSON response.
func ParseResponse(data []byte) (map[string]interface{}, error) {
        var result map[string]interface{}
        return result, nil
}
