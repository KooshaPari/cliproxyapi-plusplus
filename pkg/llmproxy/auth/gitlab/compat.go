package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/config"
)

const (
	DefaultCallbackPort = 54545
	DefaultBaseURL      = "https://gitlab.com"
)

type PKCECodes struct {
	CodeVerifier  string
	CodeChallenge string
}

type OAuthResult struct {
	Code  string
	State string
	Error string
}

type OAuthServer struct {
	port int
}

func NewOAuthServer(port int) *OAuthServer {
	return &OAuthServer{port: port}
}

func (s *OAuthServer) Start() error { return nil }

func (s *OAuthServer) Stop(context.Context) error { return nil }

func (s *OAuthServer) WaitForCallback(time.Duration) (*OAuthResult, error) {
	return nil, fmt.Errorf("gitlab oauth callback server not implemented")
}

type AuthClient struct {
	cfg *config.Config
}

func NewAuthClient(cfg *config.Config) *AuthClient {
	return &AuthClient{cfg: cfg}
}

func RedirectURL(port int) string {
	return fmt.Sprintf("http://localhost:%d/callback", port)
}

func GeneratePKCECodes() (*PKCECodes, error) {
	return &PKCECodes{}, nil
}

func NormalizeBaseURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return DefaultBaseURL
	}
	return strings.TrimRight(baseURL, "/")
}

type TokenResponse struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Scope        string
	ExpiresIn    int64
	ExpiresAt    int64
}

func TokenExpiry(now time.Time, token *TokenResponse) time.Time {
	if token == nil {
		return time.Time{}
	}
	if token.ExpiresAt > 0 {
		return time.Unix(token.ExpiresAt, 0).UTC()
	}
	if token.ExpiresIn > 0 {
		return now.Add(time.Duration(token.ExpiresIn) * time.Second).UTC()
	}
	return time.Time{}
}

type DirectAccessResponse struct {
	BaseURL      string
	Token        string
	ExpiresAt    int64
	Headers      map[string]string
	ModelDetails *DirectAccessModelDetails
}

type DirectAccessModelDetails struct {
	ModelProvider string
	ModelName     string
}

type User struct {
	Username    string
	Email       string
	PublicEmail string
	Name        string
}

func (c *AuthClient) GenerateAuthURL(baseURL, clientID, redirectURI, state string, pkce *PKCECodes) (string, error) {
	return NormalizeBaseURL(baseURL) + "/oauth/authorize", nil
}

func (c *AuthClient) ExchangeCodeForTokens(context.Context, string, string, string, string, string, string) (*TokenResponse, error) {
	return nil, fmt.Errorf("gitlab oauth token exchange not implemented")
}

func (c *AuthClient) RefreshTokens(context.Context, string, string, string, string) (*TokenResponse, error) {
	return nil, fmt.Errorf("gitlab oauth token refresh not implemented")
}

func (c *AuthClient) GetCurrentUser(ctx context.Context, baseURL, token string) (*User, error) {
	var user struct {
		Username    string `json:"username"`
		Email       string `json:"email"`
		PublicEmail string `json:"public_email"`
		Name        string `json:"name"`
	}
	if err := c.getJSON(ctx, baseURL, token, "/api/v4/user", &user); err != nil {
		return nil, err
	}
	return &User{
		Username:    strings.TrimSpace(user.Username),
		Email:       strings.TrimSpace(user.Email),
		PublicEmail: strings.TrimSpace(user.PublicEmail),
		Name:        strings.TrimSpace(user.Name),
	}, nil
}

func (c *AuthClient) FetchDirectAccess(ctx context.Context, baseURL, token string) (*DirectAccessResponse, error) {
	var direct struct {
		BaseURL      string            `json:"base_url"`
		Token        string            `json:"token"`
		ExpiresAt    int64             `json:"expires_at"`
		Headers      map[string]string `json:"headers"`
		ModelDetails *struct {
			ModelProvider string `json:"model_provider"`
			ModelName     string `json:"model_name"`
		} `json:"model_details"`
	}
	if err := c.getJSON(ctx, baseURL, token, "/api/v4/code_suggestions/direct_access", &direct); err != nil {
		return nil, err
	}
	out := &DirectAccessResponse{
		BaseURL:   strings.TrimSpace(direct.BaseURL),
		Token:     strings.TrimSpace(direct.Token),
		ExpiresAt: direct.ExpiresAt,
		Headers:   direct.Headers,
	}
	if direct.ModelDetails != nil {
		out.ModelDetails = &DirectAccessModelDetails{
			ModelProvider: strings.TrimSpace(direct.ModelDetails.ModelProvider),
			ModelName:     strings.TrimSpace(direct.ModelDetails.ModelName),
		}
	}
	return out, nil
}

func (c *AuthClient) GetPersonalAccessTokenSelf(ctx context.Context, baseURL, token string) (map[string]any, error) {
	var payload map[string]any
	if err := c.getJSON(ctx, baseURL, token, "/api/v4/personal_access_tokens/self", &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *AuthClient) getJSON(ctx context.Context, baseURL, token, path string, target any) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("gitlab auth: token is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, NormalizeBaseURL(baseURL)+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gitlab request %s failed with status %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode gitlab response %s: %w", path, err)
	}
	return nil
}

type DiscoveredModel struct {
	ModelName     string
	ModelProvider string
}

func ExtractDiscoveredModels(map[string]any) []DiscoveredModel {
	return nil
}
