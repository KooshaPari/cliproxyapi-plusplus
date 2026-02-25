package access

import (
	"context"
	"errors"
	"net/http"
	"sync"
)

// Manager coordinates authentication providers.
type Manager struct {
	mu        sync.RWMutex
	providers []Provider
}

// NewManager constructs an empty manager.
func NewManager() *Manager {
	return &Manager{}
}

// SetProviders replaces the active provider list.
func (m *Manager) SetProviders(providers []Provider) {
	if m == nil {
		return
	}
	cloned := make([]Provider, len(providers))
	copy(cloned, providers)
	m.mu.Lock()
	m.providers = cloned
	m.mu.Unlock()
}

// Providers returns a snapshot of the active providers.
func (m *Manager) Providers() []Provider {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	snapshot := make([]Provider, len(m.providers))
	copy(snapshot, m.providers)
	return snapshot
}

// Authenticate evaluates providers until one succeeds.
func (m *Manager) Authenticate(ctx context.Context, r *http.Request) (*Result, error) {
	if m == nil {
		return nil, nil
	}
	providers := m.Providers()
	if len(providers) == 0 {
		return nil, nil
	}

	var (
		missing bool
		invalid bool
	)

	for _, provider := range providers {
		if provider == nil {
			continue
		}
		res, err := provider.Authenticate(ctx, r)
		if err == nil {
			return res, nil
		}
		if errors.Is(err, ErrNotHandled) {
			continue
		}
		if errors.Is(err, ErrNoCredentials) {
			missing = true
			continue
		}
		if errors.Is(err, ErrInvalidCredential) {
			invalid = true
			continue
		}
		return nil, err
	}

	if invalid {
		return nil, ErrInvalidCredential
	}
	if missing {
		return nil, ErrNoCredentials
	}
	return nil, ErrNoCredentials
}
