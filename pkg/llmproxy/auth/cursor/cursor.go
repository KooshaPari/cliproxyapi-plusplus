package cursor

import (
	"time"

	cursorauth "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/cursor"
)

type AuthParams = cursorauth.AuthParams
type TokenPair = cursorauth.TokenPair

const (
	CursorLoginURL   = cursorauth.CursorLoginURL
	CursorPollURL    = cursorauth.CursorPollURL
	CursorRefreshURL = cursorauth.CursorRefreshURL
)

func GeneratePKCE() (string, string, error)    { return cursorauth.GeneratePKCE() }
func GenerateAuthParams() (*AuthParams, error) { return cursorauth.GenerateAuthParams() }
func PollForAuth(ctx time.Time, uuid, verifier string) (*TokenPair, error) {
	return cursorauth.PollForAuth(ctx, uuid, verifier)
}
func RefreshToken(ctx time.Time, refreshToken string) (*TokenPair, error) {
	return cursorauth.RefreshToken(ctx, refreshToken)
}
func ParseJWTSub(token string) string       { return cursorauth.ParseJWTSub(token) }
func SubToShortHash(sub string) string      { return cursorauth.SubToShortHash(sub) }
func GetTokenExpiry(token string) time.Time { return cursorauth.GetTokenExpiry(token) }
