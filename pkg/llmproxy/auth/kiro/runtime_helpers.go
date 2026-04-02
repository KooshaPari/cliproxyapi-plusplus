package kiro

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

const (
	DefaultKiroRegion       = "us-east-1"
	pathGetUsageLimits      = "getUsageLimits"
	pathListAvailableModels = "listAvailableModels"
)

type ProfileARN struct {
	Raw          string
	Partition    string
	Service      string
	Region       string
	AccountID    string
	ResourceType string
	ResourceID   string
}

func ParseProfileARN(arn string) *ProfileARN {
	if arn == "" {
		return nil
	}
	parts := strings.Split(arn, ":")
	if len(parts) < 6 || parts[0] != "arn" || parts[1] == "" || parts[2] != "codewhisperer" {
		return nil
	}
	region := strings.TrimSpace(parts[3])
	if region == "" || !strings.Contains(region, "-") {
		return nil
	}
	resource := strings.Join(parts[5:], ":")
	resourceType := resource
	resourceID := ""
	if idx := strings.Index(resource, "/"); idx > 0 {
		resourceType = resource[:idx]
		resourceID = resource[idx+1:]
	}
	return &ProfileARN{
		Raw:          arn,
		Partition:    parts[1],
		Service:      parts[2],
		Region:       region,
		AccountID:    parts[4],
		ResourceType: resourceType,
		ResourceID:   resourceID,
	}
}

func ExtractRegionFromProfileArn(profileArn string) string {
	parsed := ParseProfileARN(profileArn)
	if parsed == nil {
		return ""
	}
	return parsed.Region
}

func GetKiroAPIEndpoint(region string) string {
	if region == "" {
		region = DefaultKiroRegion
	}
	return "https://q." + region + ".amazonaws.com"
}

func GetKiroAPIEndpointFromProfileArn(profileArn string) string {
	return GetKiroAPIEndpoint(ExtractRegionFromProfileArn(profileArn))
}

func buildURL(endpoint, path string, queryParams map[string]string) string {
	fullURL := fmt.Sprintf("%s/%s", endpoint, path)
	if len(queryParams) == 0 {
		return fullURL
	}
	values := url.Values{}
	for key, value := range queryParams {
		if strings.TrimSpace(value) == "" {
			continue
		}
		values.Set(key, value)
	}
	if encoded := values.Encode(); encoded != "" {
		fullURL += "?" + encoded
	}
	return fullURL
}

func GenerateAccountKey(seed string) string {
	hash := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(hash[:8])
}

func GetAccountKey(clientID, refreshToken string) string {
	if clientID != "" {
		return GenerateAccountKey(clientID)
	}
	if refreshToken != "" {
		return GenerateAccountKey(refreshToken)
	}
	return GenerateAccountKey(uuid.New().String())
}

func setRuntimeHeaders(req *http.Request, accessToken string, accountKey string) {
	fp := GlobalFingerprintManager().GetFingerprint(accountKey)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("x-amz-user-agent", fp.BuildAmzUserAgent())
	req.Header.Set("User-Agent", fp.BuildUserAgent())
	req.Header.Set("amz-sdk-invocation-id", uuid.New().String())
	req.Header.Set("amz-sdk-request", "attempt=1; max=1")
}
