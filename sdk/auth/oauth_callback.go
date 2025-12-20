package auth

import (
	"fmt"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/misc"
)

func promptForOAuthCallback(prompt func(string) (string, error), provider string) (<-chan *misc.OAuthCallback, <-chan error) {
	if prompt == nil {
		return nil, nil
	}

	resultCh := make(chan *misc.OAuthCallback, 1)
	errCh := make(chan error, 1)

	go func() {
		label := provider
		if label == "" {
			label = "OAuth"
		}
		input, err := prompt(fmt.Sprintf("Paste the %s callback URL (or press Enter to keep waiting): ", label))
		if err != nil {
			errCh <- err
			return
		}

		parsed, err := misc.ParseOAuthCallback(input)
		if err != nil {
			errCh <- err
			return
		}
		if parsed == nil {
			return
		}

		resultCh <- parsed
	}()

	return resultCh, errCh
}
