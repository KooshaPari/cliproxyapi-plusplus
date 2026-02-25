package access

import "errors"

var (
	// ErrNoCredentials indicates no recognizable credentials were supplied.
	ErrNoCredentials = errors.New("access: no credentials provided")
	// ErrInvalidCredential signals that supplied credentials were rejected by a provider.
	ErrInvalidCredential = errors.New("access: invalid credential")
	// ErrNotHandled tells the manager to continue trying other providers.
	ErrNotHandled = errors.New("access: not handled")
)
