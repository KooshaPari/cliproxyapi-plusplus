package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	cliproxyexecutor "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/cliproxy/executor"
)

// RegisterExecutor registers a provider executor with the manager.
func (m *Manager) RegisterExecutor(exec ProviderExecutor) {
	if m == nil || exec == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executors[exec.Identifier()] = exec
}

// Register adds a new auth record to the manager and persists it via the store.
func (m *Manager) Register(ctx context.Context, auth *Auth) (string, error) {
	if m == nil {
		return "", fmt.Errorf("manager is nil")
	}
	if auth == nil {
		return "", fmt.Errorf("auth is nil")
	}
	if auth.ID == "" {
		return "", fmt.Errorf("auth ID is required")
	}
	now := time.Now()
	if auth.CreatedAt.IsZero() {
		auth.CreatedAt = now
	}
	auth.UpdatedAt = now
	auth.EnsureIndex()

	m.mu.Lock()
	m.auths[auth.ID] = auth
	m.mu.Unlock()

	if m.store != nil {
		if id, err := m.store.Save(ctx, auth); err != nil {
			return id, err
		}
	}

	m.hook.OnAuthRegistered(ctx, auth)
	return auth.ID, nil
}

// Update replaces an existing auth record in the manager and persists it.
func (m *Manager) Update(ctx context.Context, auth *Auth) (string, error) {
	if m == nil {
		return "", fmt.Errorf("manager is nil")
	}
	if auth == nil {
		return "", fmt.Errorf("auth is nil")
	}
	auth.UpdatedAt = time.Now()
	auth.EnsureIndex()

	m.mu.Lock()
	m.auths[auth.ID] = auth
	m.mu.Unlock()

	if m.store != nil {
		if id, err := m.store.Save(ctx, auth); err != nil {
			return id, err
		}
	}

	m.hook.OnAuthUpdated(ctx, auth)
	return auth.ID, nil
}

// List returns a snapshot of all registered auth records.
func (m *Manager) List() []*Auth {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Auth, 0, len(m.auths))
	for _, a := range m.auths {
		result = append(result, a)
	}
	return result
}

// GetByID returns the auth record with the given ID and a boolean indicating whether it was found.
func (m *Manager) GetByID(id string) (*Auth, bool) {
	if m == nil {
		return nil, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.auths[id]
	return a, ok
}

// Execute selects an auth candidate matching the requested providers and executes a
// non-streaming request through the corresponding provider executor.
func (m *Manager) Execute(ctx context.Context, providers []string, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	auth, exec, err := m.selectAuthAndExecutor(ctx, providers, req.Model, opts)
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}
	resp, execErr := exec.Execute(ctx, auth, req, opts)
	m.recordResult(ctx, auth, req.Model, execErr)
	return resp, execErr
}

// ExecuteCount selects an auth candidate and executes a token counting request.
func (m *Manager) ExecuteCount(ctx context.Context, providers []string, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	auth, exec, err := m.selectAuthAndExecutor(ctx, providers, req.Model, opts)
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}
	resp, execErr := exec.CountTokens(ctx, auth, req, opts)
	m.recordResult(ctx, auth, req.Model, execErr)
	return resp, execErr
}

// ExecuteStream selects an auth candidate and executes a streaming request.
func (m *Manager) ExecuteStream(ctx context.Context, providers []string, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (*cliproxyexecutor.StreamResult, error) {
	auth, exec, err := m.selectAuthAndExecutor(ctx, providers, req.Model, opts)
	if err != nil {
		return nil, err
	}
	result, execErr := exec.ExecuteStream(ctx, auth, req, opts)
	if execErr != nil {
		m.recordResult(ctx, auth, req.Model, execErr)
		return nil, execErr
	}
	return result, nil
}

// selectAuthAndExecutor picks a matching auth and its provider executor.
func (m *Manager) selectAuthAndExecutor(ctx context.Context, providers []string, model string, opts cliproxyexecutor.Options) (*Auth, ProviderExecutor, error) {
	if m == nil {
		return nil, nil, fmt.Errorf("manager is nil")
	}
	m.mu.RLock()
	candidates := m.filterCandidates(providers)
	selector := m.selector
	executors := m.executors
	m.mu.RUnlock()

	if len(candidates) == 0 {
		return nil, nil, fmt.Errorf("no auth candidates for providers %v", providers)
	}

	auth, err := selector.Pick(ctx, strings.Join(providers, ","), model, opts, candidates)
	if err != nil {
		return nil, nil, fmt.Errorf("selector pick failed: %w", err)
	}
	if auth == nil {
		return nil, nil, fmt.Errorf("no auth selected for model %s", model)
	}

	exec, ok := executors[strings.ToLower(auth.Provider)]
	if !ok {
		return nil, nil, fmt.Errorf("no executor registered for provider %q", auth.Provider)
	}
	return auth, exec, nil
}

// filterCandidates returns auths matching the requested provider list.
func (m *Manager) filterCandidates(providers []string) []*Auth {
	providerSet := make(map[string]bool, len(providers))
	for _, p := range providers {
		providerSet[strings.ToLower(p)] = true
	}
	var result []*Auth
	for _, a := range m.auths {
		if a.Disabled || a.Unavailable {
			continue
		}
		if providerSet[strings.ToLower(a.Provider)] {
			result = append(result, a)
		}
	}
	return result
}

// CloseExecutionSession notifies all registered executors that support session
// lifecycle management to release resources associated with the given session ID.
func (m *Manager) CloseExecutionSession(sessionID string) {
	if m == nil || sessionID == "" {
		return
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, exec := range m.executors {
		if closer, ok := exec.(ExecutionSessionCloser); ok {
			closer.CloseExecutionSession(sessionID)
		}
	}
}

// recordResult tracks execution outcomes on the auth.
func (m *Manager) recordResult(ctx context.Context, auth *Auth, model string, err error) {
	if auth == nil {
		return
	}
	result := Result{
		AuthID:   auth.ID,
		Provider: auth.Provider,
		Model:    model,
		Success:  err == nil,
	}
	if err != nil {
		result.Error = &Error{Message: err.Error()}
	}
	m.hook.OnResult(ctx, result)
}

// SetRetryConfig updates the request retry count and maximum retry interval.
func (m *Manager) SetRetryConfig(retryCount int, maxInterval time.Duration) {
	if m == nil {
		return
	}
	m.requestRetry.Store(int32(retryCount))
	m.maxRetryInterval.Store(int64(maxInterval))
}

// SetQuotaCooldownDisabled globally disables quota cooldown backoff.
func SetQuotaCooldownDisabled(disabled bool) {
	quotaCooldownDisabled.Store(disabled)
}

// Executor returns the provider executor registered for the given provider key.
func (m *Manager) Executor(provider string) (ProviderExecutor, bool) {
	if m == nil {
		return nil, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	exec, ok := m.executors[strings.ToLower(provider)]
	return exec, ok
}

// Load reads all auth records from the backing store into memory.
func (m *Manager) Load(ctx context.Context) error {
	if m == nil {
		return fmt.Errorf("manager is nil")
	}
	if m.store == nil {
		return nil
	}
	auths, err := m.store.List(ctx)
	if err != nil {
		return fmt.Errorf("loading auth store: %w", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range auths {
		a.EnsureIndex()
		m.auths[a.ID] = a
	}
	return nil
}

// StartAutoRefresh begins a background goroutine that periodically refreshes
// auth credentials using registered provider executors.
func (m *Manager) StartAutoRefresh(ctx context.Context, interval time.Duration) {
	if m == nil {
		return
	}
	m.StopAutoRefresh()
	ctx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	m.refreshCancel = cancel
	m.mu.Unlock()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.refreshAll(ctx)
			}
		}
	}()
}

// StopAutoRefresh cancels the background auto-refresh goroutine.
func (m *Manager) StopAutoRefresh() {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.refreshCancel != nil {
		m.refreshCancel()
		m.refreshCancel = nil
	}
	m.mu.Unlock()
}

// refreshAll iterates over all auth records and attempts to refresh those
// that have a registered executor supporting refresh.
func (m *Manager) refreshAll(ctx context.Context) {
	m.mu.RLock()
	var toRefresh []*Auth
	for _, a := range m.auths {
		if a.Disabled {
			continue
		}
		toRefresh = append(toRefresh, a)
	}
	executors := m.executors
	m.mu.RUnlock()

	for _, a := range toRefresh {
		exec, ok := executors[strings.ToLower(a.Provider)]
		if !ok {
			continue
		}
		updated, err := exec.Refresh(ctx, a)
		if err != nil {
			continue
		}
		if updated != nil {
			m.mu.Lock()
			m.auths[a.ID] = updated
			m.mu.Unlock()
			if m.store != nil {
				_, _ = m.store.Save(ctx, updated)
			}
			m.hook.OnAuthUpdated(ctx, updated)
		}
	}
}
