package iflow

import (
<<<<<<< HEAD
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/misc"
=======
	"fmt"

	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/auth/base"
>>>>>>> origin/main
)

// IFlowTokenStorage persists iFlow OAuth credentials alongside the derived API key.
type IFlowTokenStorage struct {
<<<<<<< HEAD
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	LastRefresh  string `json:"last_refresh"`
	Expire       string `json:"expired"`
	APIKey       string `json:"api_key"`
	Email        string `json:"email"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	Cookie       string `json:"cookie"`
	Type         string `json:"type"`
=======
	base.BaseTokenStorage

	LastRefresh string `json:"last_refresh"`
	Expire      string `json:"expired"`
	APIKey      string `json:"api_key"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Cookie      string `json:"cookie"`
>>>>>>> origin/main
}

// SaveTokenToFile serialises the token storage to disk.
func (ts *IFlowTokenStorage) SaveTokenToFile(authFilePath string) error {
<<<<<<< HEAD
	safePath, err := misc.ResolveSafeFilePath(authFilePath)
	if err != nil {
		return fmt.Errorf("invalid token file path: %w", err)
	}
	misc.LogSavingCredentials(safePath)
	ts.Type = "iflow"
	if err = os.MkdirAll(filepath.Dir(safePath), 0o700); err != nil {
		return fmt.Errorf("iflow token: create directory failed: %w", err)
	}

	f, err := os.Create(safePath)
	if err != nil {
		return fmt.Errorf("iflow token: create file failed: %w", err)
	}
	defer func() { _ = f.Close() }()

	if err = json.NewEncoder(f).Encode(ts); err != nil {
		return fmt.Errorf("iflow token: encode token failed: %w", err)
=======
	ts.Type = "iflow"
	if err := ts.Save(authFilePath, ts); err != nil {
		return fmt.Errorf("iflow token: %w", err)
>>>>>>> origin/main
	}
	return nil
}
