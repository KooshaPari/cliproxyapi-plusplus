# Product Requirements Document — CLIProxyAPI Plus

## Overview

CLIProxyAPI Plus is a Go-based API proxy server that extends the mainline [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) project with community-maintained third-party provider integrations. It provides an OpenAI-compatible API surface over diverse AI provider backends (Anthropic, Gemini, Cursor, GitLab Duo, Codex, etc.), enabling drop-in replacement for OpenAI clients.

```
Architecture Overview:

  Client (any OpenAI-compatible SDK)
         |
         v
  ┌──────────────────────────────┐
  │      CLIProxyAPI Plus        │
  │   (gin HTTP server, :8080)   │
  │                              │
  │  ┌────────┐  ┌────────────┐  │
  │  │  Auth  │  │  Registry  │  │
  │  │ Layer  │  │ (providers)│  │
  │  └────────┘  └────────────┘  │
  │  ┌────────┐  ┌────────────┐  │
  │  │Translator│ │   Cache   │  │
  │  │(req/resp)│ │   Layer   │  │
  │  └────────┘  └────────────┘  │
  └──────────────────────────────┘
         |
         v
  Third-Party AI Provider APIs
  (Claude, Gemini, Cursor, GitLab Duo, etc.)
```

---

## E1: Core Proxy Engine (Implemented)

### E1.1: OpenAI-Compatible HTTP API

As a developer using OpenAI SDK clients, I want a drop-in replacement endpoint that proxies to third-party providers so that I can switch backends without changing client code.

**Acceptance Criteria**:
- HTTP server on configurable port (default 8080) using gin framework.
- `/v1/chat/completions` endpoint accepts OpenAI-format requests.
- `/v1/models` endpoint returns available provider models.
- Streaming responses via Server-Sent Events (SSE) mirroring OpenAI streaming format.
- Non-streaming responses return OpenAI-compatible JSON body.
- Request/response translation via `internal/translator` per provider.

### E1.2: Provider Registry

As an operator, I want a pluggable provider registry so that new AI providers can be added without modifying core routing logic.

**Acceptance Criteria**:
- `internal/registry` contains provider registration and discovery.
- Each provider implements a common translator interface (`internal/interfaces`).
- Config-driven provider selection via `config.example.yaml`.
- Providers: Anthropic Claude, Google Gemini, Cursor, GitLab Duo, OpenAI Codex.

### E1.3: Authentication and Access Control

As an operator, I want per-provider authentication handling so that API keys and tokens are managed securely per backend.

**Acceptance Criteria**:
- `internal/auth` handles OAuth and API-key auth flows per provider.
- `auths/` directory stores auth-module implementations.
- Browser-based auth flow support via `internal/browser`.
- Token storage and refresh via `internal/store`.

---

## E2: SDK and Client Support (Implemented)

### E2.1: Go SDK

As a Go developer, I want a typed SDK client for CLIProxyAPI so that I can integrate programmatically without raw HTTP calls.

**Acceptance Criteria**:
- `sdk/` package provides typed Go client.
- SDK covers `api`, `auth`, `config`, `logging`, `translator` sub-packages.
- `sdk/cliproxy` binary for direct CLI usage.
- SDK documented in `docs/sdk-usage.md` and `docs/sdk-advanced.md`.

### E2.2: WebSocket Relay

As a provider that uses WebSocket-based streaming, I want a WebSocket relay so that streaming responses are normalized to SSE for the client.

**Acceptance Criteria**:
- `internal/wsrelay` handles WebSocket-to-SSE bridging.
- Gorilla WebSocket v1.5.3 used for upstream connections.
- Graceful connection close and error propagation.

---

## E3: Operational Features (Implemented)

### E3.1: Usage Tracking and Token Counting

As an operator, I want per-request token usage tracking so that I can monitor costs and enforce quotas.

**Acceptance Criteria**:
- `internal/usage` tracks token counts per request.
- tiktoken-go tokenizer used for accurate OpenAI-compatible token counting.
- Usage metadata attached to response objects.

### E3.2: Caching Layer

As an operator, I want response caching so that repeated identical requests do not incur upstream API costs.

**Acceptance Criteria**:
- `internal/cache` provides configurable response caching.
- Cache keyed on normalized request hash.
- TTL and eviction configurable via `config.yaml`.

### E3.3: TUI Dashboard

As an operator, I want a terminal UI dashboard so that I can monitor proxy activity, active connections, and usage in real time.

**Acceptance Criteria**:
- `internal/tui` implements Bubble Tea TUI.
- Dashboard shows: active requests, provider breakdown, error rates, token counts.
- lipgloss v1.1.0 for styled output; bubbletea v1.3.10 for TUI framework.

### E3.4: Config File Watcher

As an operator, I want hot-reload of configuration so that provider keys and settings can be updated without restarting the server.

**Acceptance Criteria**:
- `internal/watcher` uses fsnotify v1.9.0 for config file watching.
- On config change, providers are re-registered with new settings.
- No dropped requests during config reload.

---

## E4: Third-Party Provider Integrations (Community-Maintained)

### E4.1: GitLab Duo Integration

As a GitLab user, I want CLIProxyAPI Plus to proxy GitLab Duo AI completions through the OpenAI-compatible interface.

**Acceptance Criteria**:
- GitLab Duo provider registered in registry.
- OAuth flow for GitLab authentication.
- Completion and chat endpoints mapped.
- Parity documented in `docs/gitlab-duo.md`.

### E4.2: Thinking Mode Support

As a developer, I want support for extended-thinking modes on providers that support it so that reasoning-capable models can be used via the proxy.

**Acceptance Criteria**:
- `internal/thinking` module handles thinking-mode request transformation.
- Thinking tokens excluded from usage counts per provider documentation.
- Configurable thinking budget in config.

---

## E5: Deployment and Distribution (Planned)

### E5.1: Docker Distribution

As an operator, I want an official Docker image so that I can deploy CLIProxyAPI Plus in containerized environments.

**Acceptance Criteria**:
- `Dockerfile` builds a minimal Go binary image.
- `docker-compose.yml` for local development stack.
- `docker-build.sh` / `docker-build.ps1` for cross-platform image builds.
- Image published to Docker Hub or GitHub Container Registry on release.

### E5.2: Release Automation

As a maintainer, I want automated release builds so that binaries are published for all target platforms on tag push.

**Acceptance Criteria**:
- `.goreleaser.yml` configured for multi-platform builds (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64).
- GitHub Actions workflow triggers on semver tags.
- Checksums and release notes generated automatically.
