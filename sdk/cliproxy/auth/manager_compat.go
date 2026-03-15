package auth

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"

	internalconfig "github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/config"
	cliproxyexecutor "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/cliproxy/executor"
)

func (m *Manager) Register(ctx context.Context, auth *Auth) (*Auth, error) {
	if m == nil || auth == nil {
		return nil, &Error{Code: "invalid_auth", Message: "auth is required", HTTPStatus: http.StatusBadRequest}
	}
	now := time.Now().UTC()
	next := auth.Clone()
	if next.ID == "" {
		return nil, &Error{Code: "invalid_auth", Message: "auth id is required", HTTPStatus: http.StatusBadRequest}
	}
	if next.Provider == "" {
		return nil, &Error{Code: "invalid_auth", Message: "auth provider is required", HTTPStatus: http.StatusBadRequest}
	}
	if next.CreatedAt.IsZero() {
		next.CreatedAt = now
	}
	next.UpdatedAt = now
	if next.Status == "" {
		if next.Disabled {
			next.Status = StatusDisabled
		} else {
			next.Status = StatusActive
		}
	}
	next.EnsureIndex()

	m.mu.Lock()
	m.auths[next.ID] = next.Clone()
	m.mu.Unlock()

	if m.store != nil && !shouldSkipPersist(ctx) {
		if _, err := m.store.Save(ctx, next.Clone()); err != nil {
			return nil, err
		}
	}
	m.hook.OnAuthRegistered(ctx, next.Clone())
	return next.Clone(), nil
}

func (m *Manager) Update(ctx context.Context, auth *Auth) (*Auth, error) {
	if m == nil || auth == nil {
		return nil, &Error{Code: "invalid_auth", Message: "auth is required", HTTPStatus: http.StatusBadRequest}
	}
	now := time.Now().UTC()
	next := auth.Clone()
	if next.ID == "" {
		return nil, &Error{Code: "invalid_auth", Message: "auth id is required", HTTPStatus: http.StatusBadRequest}
	}

	m.mu.RLock()
	current := m.auths[next.ID]
	m.mu.RUnlock()
	if current != nil && next.CreatedAt.IsZero() {
		next.CreatedAt = current.CreatedAt
	}
	if next.CreatedAt.IsZero() {
		next.CreatedAt = now
	}
	next.UpdatedAt = now
	next.EnsureIndex()
	updateAggregatedAvailability(next, now)

	m.mu.Lock()
	m.auths[next.ID] = next.Clone()
	m.mu.Unlock()

	if m.store != nil && !shouldSkipPersist(ctx) {
		if _, err := m.store.Save(ctx, next.Clone()); err != nil {
			return nil, err
		}
	}
	m.hook.OnAuthUpdated(ctx, next.Clone())
	return next.Clone(), nil
}

func (m *Manager) GetByID(id string) (*Auth, bool) {
	if m == nil {
		return nil, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	auth, ok := m.auths[id]
	if !ok || auth == nil {
		return nil, false
	}
	return auth.Clone(), true
}

func (m *Manager) Load(ctx context.Context) error {
	if m == nil || m.store == nil {
		return nil
	}
	auths, err := m.store.List(ctx)
	if err != nil {
		return err
	}
	next := make(map[string]*Auth, len(auths))
	for _, auth := range auths {
		if auth == nil || strings.TrimSpace(auth.ID) == "" {
			continue
		}
		cloned := auth.Clone()
		cloned.EnsureIndex()
		next[cloned.ID] = cloned
	}
	m.mu.Lock()
	m.auths = next
	m.mu.Unlock()
	return nil
}

func (m *Manager) List() []*Auth {
	if m == nil {
		return nil
	}
	if m.store != nil {
		items, err := m.store.List(context.Background())
		if err == nil {
			return items
		}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]*Auth, 0, len(m.auths))
	for _, auth := range m.auths {
		if auth != nil {
			items = append(items, auth.Clone())
		}
	}
	slices.SortFunc(items, func(a, b *Auth) int {
		return strings.Compare(a.ID, b.ID)
	})
	return items
}

func (m *Manager) StartAutoRefresh(ctx context.Context, _ time.Duration) {
	if m == nil {
		return
	}
	m.StopAutoRefresh()
	if ctx == nil {
		ctx = context.Background()
	}
	refreshCtx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	m.refreshCancel = cancel
	m.mu.Unlock()
	go func() {
		<-refreshCtx.Done()
	}()
}

func (m *Manager) StopAutoRefresh() {
	if m == nil {
		return
	}
	m.mu.Lock()
	cancel := m.refreshCancel
	m.refreshCancel = nil
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (m *Manager) RegisterExecutor(executor ProviderExecutor) {
	if m == nil || executor == nil {
		return
	}
	key := strings.ToLower(strings.TrimSpace(executor.Identifier()))
	if key == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if replaced, ok := m.executors[key]; ok {
		if closer, okCloser := replaced.(ExecutionSessionCloser); okCloser {
			closer.CloseExecutionSession(CloseAllExecutionSessionsID)
		}
	}
	m.executors[key] = executor
}

func (m *Manager) Executor(provider string) (ProviderExecutor, bool) {
	if m == nil {
		return nil, false
	}
	key := strings.ToLower(strings.TrimSpace(provider))
	m.mu.RLock()
	defer m.mu.RUnlock()
	executor, ok := m.executors[key]
	return executor, ok
}

func (m *Manager) CloseExecutionSession(sessionID string) {
	if m == nil || sessionID == "" {
		return
	}
	m.mu.RLock()
	executors := make([]ProviderExecutor, 0, len(m.executors))
	for _, executor := range m.executors {
		executors = append(executors, executor)
	}
	m.mu.RUnlock()
	for _, executor := range executors {
		if closer, ok := executor.(ExecutionSessionCloser); ok {
			closer.CloseExecutionSession(sessionID)
		}
	}
}

func (m *Manager) SetRetryConfig(requestRetry int, maxRetryInterval time.Duration) {
	if m == nil {
		return
	}
	if requestRetry < 0 {
		requestRetry = 0
	}
	if maxRetryInterval < 0 {
		maxRetryInterval = 0
	}
	m.requestRetry.Store(int32(requestRetry))
	m.maxRetryInterval.Store(int64(maxRetryInterval))
}

func (m *Manager) retrySettings() (int, time.Duration) {
	if m == nil {
		return 0, 0
	}
	return int(m.requestRetry.Load()), time.Duration(m.maxRetryInterval.Load())
}

func (m *Manager) shouldRetryAfterError(err *Error, attempt int, providers []string, model string, maxWait time.Duration) (time.Duration, bool) {
	if m == nil || err == nil {
		return 0, false
	}
	now := time.Now()
	allowedRetries, _ := m.retrySettings()
	if override, ok := m.requestRetryOverride(providers); ok {
		allowedRetries = override
	}
	if allowedRetries <= 0 || attempt >= allowedRetries {
		return 0, false
	}
	wait := m.earliestRetryAfter(now, providers, model)
	if wait <= 0 {
		wait = time.Second
	}
	if maxWait > 0 && wait > maxWait {
		wait = maxWait
	}
	return wait, true
}

func (m *Manager) requestRetryOverride(providers []string) (int, bool) {
	if m == nil {
		return 0, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, provider := range providers {
		for _, auth := range m.auths {
			if auth == nil || !strings.EqualFold(auth.Provider, provider) || auth.Metadata == nil {
				continue
			}
			if raw, ok := auth.Metadata["request_retry"]; ok {
				switch v := raw.(type) {
				case int:
					return v, true
				case int32:
					return int(v), true
				case int64:
					return int(v), true
				case float64:
					return int(v), true
				}
			}
		}
	}
	return 0, false
}

func (m *Manager) earliestRetryAfter(now time.Time, providers []string, model string) time.Duration {
	if m == nil {
		return 0
	}
	var earliest time.Time
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, provider := range providers {
		for _, auth := range m.auths {
			if auth == nil || !strings.EqualFold(auth.Provider, provider) {
				continue
			}
			if blocked, _, next := isAuthBlockedForModel(auth, model, now); blocked && !next.IsZero() {
				if earliest.IsZero() || next.Before(earliest) {
					earliest = next
				}
			}
		}
	}
	if earliest.IsZero() {
		return 0
	}
	return earliest.Sub(now)
}

func (m *Manager) MarkResult(ctx context.Context, result Result) {
	if m == nil || result.AuthID == "" {
		return
	}
	now := time.Now().UTC()
	m.mu.Lock()
	auth := m.auths[result.AuthID]
	if auth == nil {
		m.mu.Unlock()
		return
	}
	if auth.ModelStates == nil {
		auth.ModelStates = make(map[string]*ModelState)
	}
	modelKey := canonicalModelKey(result.Model)
	if modelKey == "" {
		modelKey = strings.TrimSpace(result.Model)
	}
	state := auth.ModelStates[modelKey]
	if state == nil {
		state = &ModelState{}
		auth.ModelStates[modelKey] = state
	}
	if result.Success {
		*state = ModelState{Status: StatusActive, UpdatedAt: now}
	} else {
		state.Status = StatusError
		state.Unavailable = true
		state.UpdatedAt = now
		if result.Error != nil {
			state.LastError = result.Error
			state.StatusMessage = result.Error.Message
		}
		disableCooling, hasDisableCooling := auth.DisableCoolingOverride()
		hasAlternative := m.hasAlternativeAuthLocked(auth.ID, auth.Provider)
		if (hasDisableCooling && disableCooling) || (!hasDisableCooling && isQuotaCooldownDisabled()) {
			state.NextRetryAfter = time.Time{}
			state.Unavailable = false
		} else if !hasAlternative {
			state.NextRetryAfter = time.Time{}
			state.Unavailable = false
		} else if result.RetryAfter != nil && *result.RetryAfter > 0 {
			state.NextRetryAfter = now.Add(*result.RetryAfter)
		} else {
			state.NextRetryAfter = now.Add(quotaBackoffBase)
		}
	}
	updateAggregatedAvailability(auth, now)
	updated := auth.Clone()
	m.mu.Unlock()
	m.hook.OnResult(ctx, result)
	m.hook.OnAuthUpdated(ctx, updated)
}

func (m *Manager) hasAlternativeAuthLocked(currentID, provider string) bool {
	for id, auth := range m.auths {
		if id == currentID || auth == nil {
			continue
		}
		if strings.EqualFold(auth.Provider, provider) {
			return true
		}
	}
	return false
}

func updateAggregatedAvailability(auth *Auth, now time.Time) {
	if auth == nil {
		return
	}
	auth.Unavailable = false
	auth.NextRetryAfter = time.Time{}
	if len(auth.ModelStates) == 0 {
		return
	}
	for _, state := range auth.ModelStates {
		if state == nil || !state.Unavailable || state.NextRetryAfter.IsZero() || !state.NextRetryAfter.After(now) {
			continue
		}
		auth.Unavailable = true
		if auth.NextRetryAfter.IsZero() || state.NextRetryAfter.Before(auth.NextRetryAfter) {
			auth.NextRetryAfter = state.NextRetryAfter
		}
	}
}

func (m *Manager) Execute(ctx context.Context, providers []string, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	auth, executor, err := m.selectAuthAndExecutor(ctx, providers, req.Model, opts)
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}
	req.Model = m.applyAPIKeyModelAlias(auth, req.Model)
	resp, execErr := executor.Execute(ctx, auth, req, opts)
	m.recordExecutionResult(ctx, auth, req.Model, execErr, nil)
	return resp, execErr
}

func (m *Manager) ExecuteCount(ctx context.Context, providers []string, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	auth, executor, err := m.selectAuthAndExecutor(ctx, providers, req.Model, opts)
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}
	req.Model = m.applyAPIKeyModelAlias(auth, req.Model)
	resp, execErr := executor.CountTokens(ctx, auth, req, opts)
	m.recordExecutionResult(ctx, auth, req.Model, execErr, nil)
	return resp, execErr
}

func (m *Manager) ExecuteStream(ctx context.Context, providers []string, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (*cliproxyexecutor.StreamResult, error) {
	auth, executor, err := m.selectAuthAndExecutor(ctx, providers, req.Model, opts)
	if err != nil {
		return nil, err
	}
	req.Model = m.applyAPIKeyModelAlias(auth, req.Model)
	return executor.ExecuteStream(ctx, auth, req, opts)
}

func (m *Manager) selectAuthAndExecutor(ctx context.Context, providers []string, model string, opts cliproxyexecutor.Options) (*Auth, ProviderExecutor, error) {
	if m == nil {
		return nil, nil, &Error{Code: "manager_nil", Message: "auth manager is nil", HTTPStatus: http.StatusInternalServerError}
	}
	now := time.Now()
	pinnedAuthID := ""
	var selectedAuthCallback func(string)
	if opts.Metadata != nil {
		if rawPinned, ok := opts.Metadata[cliproxyexecutor.PinnedAuthMetadataKey].(string); ok {
			pinnedAuthID = strings.TrimSpace(rawPinned)
		}
		if cb, ok := opts.Metadata[cliproxyexecutor.SelectedAuthCallbackMetadataKey].(func(string)); ok {
			selectedAuthCallback = cb
		}
	}
	var candidates []*Auth
	var executor ProviderExecutor
	m.mu.RLock()
	for _, provider := range providers {
		exec := m.executors[strings.ToLower(strings.TrimSpace(provider))]
		if exec == nil {
			continue
		}
		for _, auth := range m.auths {
			if auth == nil || !strings.EqualFold(auth.Provider, provider) {
				continue
			}
			if pinnedAuthID != "" && auth.ID != pinnedAuthID {
				continue
			}
			candidates = append(candidates, auth.Clone())
		}
		if executor == nil {
			executor = exec
		}
	}
	selector := m.selector
	m.mu.RUnlock()
	if len(candidates) == 0 || executor == nil {
		return nil, nil, &Error{Code: "auth_not_found", Message: "no auth candidates", HTTPStatus: http.StatusServiceUnavailable, Retryable: true}
	}
	auth, err := selector.Pick(ctx, providers[0], model, opts, candidates)
	if err != nil {
		return nil, nil, err
	}
	updateAggregatedAvailability(auth, now)
	if selectedAuthCallback != nil && auth != nil {
		selectedAuthCallback(auth.ID)
	}
	return auth, executor, nil
}

func (m *Manager) recordExecutionResult(ctx context.Context, auth *Auth, model string, execErr error, retryAfter *time.Duration) {
	if auth == nil {
		return
	}
	if execErr == nil {
		m.MarkResult(ctx, Result{AuthID: auth.ID, Provider: auth.Provider, Model: model, Success: true})
		return
	}
	appErr := &Error{Message: execErr.Error(), Retryable: true, HTTPStatus: statusCodeFromError(execErr)}
	m.MarkResult(ctx, Result{AuthID: auth.ID, Provider: auth.Provider, Model: model, Success: false, RetryAfter: retryAfter, Error: appErr})
}

func statusCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	if withStatus, ok := err.(interface{ StatusCode() int }); ok {
		return withStatus.StatusCode()
	}
	return http.StatusInternalServerError
}

func (m *Manager) applyAPIKeyModelAlias(auth *Auth, requestedModel string) string {
	if auth == nil {
		return requestedModel
	}
	if auth.Attributes != nil && strings.EqualFold(strings.TrimSpace(auth.Attributes["auth_kind"]), "oauth") {
		return requestedModel
	}
	if resolved := m.lookupAPIKeyUpstreamModel(auth.ID, requestedModel); resolved != "" {
		return resolved
	}
	return requestedModel
}

func (m *Manager) lookupAPIKeyUpstreamModel(authID, requestedModel string) string {
	if m == nil || authID == "" || strings.TrimSpace(requestedModel) == "" {
		return ""
	}
	auth, ok := m.GetByID(authID)
	if !ok || auth == nil {
		return ""
	}
	cfgAny := m.runtimeConfig.Load()
	cfg, _ := cfgAny.(*internalconfig.Config)
	if cfg == nil {
		return ""
	}
	apiKey := ""
	baseURL := ""
	if auth.Attributes != nil {
		apiKey = strings.TrimSpace(auth.Attributes["api_key"])
		baseURL = strings.TrimSpace(auth.Attributes["base_url"])
	}
	switch strings.ToLower(strings.TrimSpace(auth.Provider)) {
	case "gemini":
		for _, key := range cfg.GeminiKey {
			if strings.TrimSpace(key.APIKey) == apiKey && (baseURL == "" || strings.TrimSpace(key.BaseURL) == baseURL) {
				models := make([]modelAliasEntry, 0, len(key.Models))
				for _, model := range key.Models {
					models = append(models, model)
				}
				return resolveModelAliasFromConfigModels(requestedModel, models)
			}
		}
	case "claude":
		for _, key := range cfg.ClaudeKey {
			if strings.TrimSpace(key.APIKey) == apiKey && (baseURL == "" || strings.TrimSpace(key.BaseURL) == baseURL) {
				models := make([]modelAliasEntry, 0, len(key.Models))
				for _, model := range key.Models {
					models = append(models, model)
				}
				return resolveModelAliasFromConfigModels(requestedModel, models)
			}
		}
	case "codex":
		for _, key := range cfg.CodexKey {
			if strings.TrimSpace(key.APIKey) == apiKey && (baseURL == "" || strings.TrimSpace(key.BaseURL) == baseURL) {
				models := make([]modelAliasEntry, 0, len(key.Models))
				for _, model := range key.Models {
					models = append(models, model)
				}
				return resolveModelAliasFromConfigModels(requestedModel, models)
			}
		}
	}
	return ""
}
