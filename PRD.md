# Product Requirements Document — cliproxyapi-plusplus

> Module path: `github.com/kooshapari/CLIProxyAPI/v7`
> Default port: `:8317`
> Primary language: Go 1.26+

---

## Overview

cliproxyapi-plusplus is an agent-native, multi-provider LLM proxy server. It exposes a single OpenAI-compatible HTTP API surface while transparently routing requests to heterogeneous upstream AI providers — including CLI-native tools like Claude Code, Gemini CLI, GitHub Copilot, AWS Kiro, and API-key providers like Anthropic, OpenAI, and Vertex AI.

The "plusplus" fork extends the mainline project with support for third-party providers maintained by community contributors: iFlow, Minimax, Kimi, Antigravity, Qwen, and others.

---

## E1: OpenAI-Compatible Proxy Surface

### E1.1: Chat Completions Endpoint
**As** a developer using an OpenAI-compatible tool (IDE extension, agent framework, etc.), **I want** to point my tool at the proxy and have all LLM calls transparently routed to configured backends **so that** I can use any supported model without changing my tooling.

**Acceptance Criteria:**
- `POST /v1/chat/completions` accepts OpenAI request format.
- Streaming (`stream: true`) and non-streaming responses are both supported.
- Multi-byte UTF-8 characters in SSE streams are correctly assembled before emission.
- Model name in the request is translated per-provider (e.g., `claude-3-5-sonnet` -> provider-specific model ID).
- The server is reachable on `:8317` by default; port is configurable via `config.yaml`.

### E1.2: Model Listing
**As** a developer, **I want** `GET /v1/models` to return all currently available models across active providers **so that** model-picker UIs populate correctly.

**Acceptance Criteria:**
- Returns combined model list from all authenticated/configured providers.
- Model IDs follow the naming scheme expected by callers (e.g., OpenAI-style IDs).

### E1.3: Provider-Aware Request Translation
**As** an agent author, **I want** request and response bodies to be translated between OpenAI format and each provider's native format **so that** provider differences are invisible to callers.

**Acceptance Criteria:**
- Translators exist for: Gemini, Gemini CLI, Claude (Anthropic native), Codex, Kiro, Antigravity, OpenAI-compatible, ACP.
- Thinking/extended thinking blocks from Claude are preserved and surfaced correctly.
- Tool call / function call format is mapped between OpenAI and each provider.

---

## E2: Multi-Provider Authentication

### E2.1: OAuth Browser Flows
**As** a developer, **I want** to authenticate with browser-based OAuth providers via a CLI flag **so that** I can start the proxy and route traffic within a few seconds.

**Acceptance Criteria:**
- `--login` triggers Google/Gemini OAuth.
- `--claude-login` triggers Anthropic Claude OAuth.
- `--codex-login` and `--codex-device-login` trigger OpenAI Codex OAuth (authorization-code and device-code flows respectively).
- `--qwen-login` triggers Qwen OAuth.
- `--kimi-login` triggers Kimi OAuth.
- `--antigravity-login` triggers Antigravity OAuth.
- `--iflow-login` triggers iFlow OAuth; `--iflow-cookie` triggers iFlow cookie-based auth.
- `--no-browser` suppresses automatic browser launch for headless environments.
- `--oauth-callback-port` overrides the local callback port when the default is taken.
- `--incognito` / `--no-incognito` controls browser profile isolation for multi-account support.

### E2.2: AWS Kiro Authentication
**As** a developer using AWS Builder ID or IAM Identity Center, **I want** multiple Kiro auth flows **so that** I can log in regardless of my AWS account type.

**Acceptance Criteria:**
- `--kiro-login` / `--kiro-google-login`: Google-backed Kiro OAuth, incognito by default.
- `--kiro-aws-login`: AWS Builder ID device code flow.
- `--kiro-aws-authcode`: AWS Builder ID authorization code flow (better UX than device flow).
- `--kiro-import`: Imports token from `~/.aws/sso/cache/kiro-auth-token.json` (IDE-generated token reuse).
- Background token refresh runs automatically after login to avoid mid-session expiry.

### E2.3: GitHub Copilot Authentication
**As** a GitHub Copilot subscriber, **I want** device-code authentication **so that** I can proxy Copilot-backed model requests.

**Acceptance Criteria:**
- `--github-copilot-login` initiates device code flow.
- Tokens are stored in the configured auth directory and refreshed automatically.

### E2.4: API Key Authentication
**As** an operator, **I want** to configure API keys for Gemini, Vertex AI (compat), Anthropic, Codex/OpenAI, and OpenAI-compatible providers in `config.yaml` **so that** the proxy manages key selection and rotation.

**Acceptance Criteria:**
- `--vertex-import <path>` imports a Vertex AI service account JSON key file.
- `--project_id <id>` sets the GCP project ID for Gemini/Vertex.
- Multiple API keys per provider are supported with round-robin or load-balanced selection.

---

## E3: Token Storage Backends

### E3.1: Local File Store (Default)
**As** a developer running locally, **I want** auth tokens stored in a local directory **so that** no external infrastructure is needed.

**Acceptance Criteria:**
- Default auth directory is `~/.cli-proxy-api` or a writable path derived from the working directory.
- `--config <path>` overrides the config file location.

### E3.2: PostgreSQL Store
**As** an operator deploying the proxy in a multi-instance cluster, **I want** tokens stored in PostgreSQL **so that** all instances share the same auth state.

**Acceptance Criteria:**
- Enabled by setting `PGSTORE_DSN` environment variable (or `pgstore_dsn`).
- Optional schema override: `PGSTORE_SCHEMA`.
- Local spool directory configurable via `PGSTORE_LOCAL_PATH`.
- Config file is bootstrapped from `config.example.yaml` on first start if none exists.

### E3.3: Git-Backed Store
**As** an operator who wants audit trails for token changes, **I want** tokens committed to a remote git repository **so that** all auth state changes are versioned.

**Acceptance Criteria:**
- Enabled by `GITSTORE_GIT_URL`, `GITSTORE_GIT_USERNAME`, `GITSTORE_GIT_TOKEN`.
- Local path configurable via `GITSTORE_LOCAL_PATH`.
- Config is committed on bootstrap; token writes are committed automatically.

### E3.4: Object Store (S3-Compatible)
**As** an operator using S3 or MinIO, **I want** tokens stored in an object bucket **so that** the proxy is stateless and horizontally scalable.

**Acceptance Criteria:**
- Enabled by `OBJECTSTORE_ENDPOINT`, `OBJECTSTORE_ACCESS_KEY`, `OBJECTSTORE_SECRET_KEY`, `OBJECTSTORE_BUCKET`.
- HTTP and HTTPS endpoints both supported; scheme auto-detected from URL prefix.
- Path-style addressing enabled.

---

## E4: Operational Tooling

### E4.1: Rate Limiting
**As** an operator, **I want** per-client and per-provider rate limiting **so that** quota is not exhausted by a single agent or user.

**Acceptance Criteria:**
- Configurable request rate limits in `config.yaml`.
- Cooldown management (`DisableCooling` flag) controls backoff behavior on upstream 429s.
- Rate limit violations return standard HTTP 429 responses.

### E4.2: Background Token Refresh
**As** a developer, **I want** token refresh to happen in the background before expiry **so that** in-flight requests are never interrupted.

**Acceptance Criteria:**
- Kiro's `RefreshManager` starts automatically when `--auth-dir` is set.
- Refresh lifecycle is properly torn down on server shutdown (via `defer kiro.StopGlobalRefreshManager()`).
- Each provider's refresh logic is independently managed via the refresh registry.

### E4.3: Usage Statistics
**As** an operator, **I want** usage statistics collection **so that** I can observe token consumption and request patterns.

**Acceptance Criteria:**
- `UsageStatisticsEnabled` config flag gates statistics collection.
- Statistics are accessible via the management API.

### E4.4: Management API
**As** an operator, **I want** a management HTTP API **so that** I can inspect configuration, view active sessions, and trigger reloads.

**Acceptance Criteria:**
- Management API serves current config, provider status, and usage data.
- Protected by a password (set via `--password` flag or auto-generated in TUI standalone mode).
- `cliproxyctl` CLI provides human-friendly access to all management API operations.

### E4.5: Config Auto-Reload (File Watcher)
**As** an operator, **I want** configuration changes to take effect without restarting the server **so that** I can update provider settings in production.

**Acceptance Criteria:**
- `StartAutoUpdater` watches `config.yaml` and auth directory for changes.
- On change detection, the config is reloaded and affected services are reconfigured.
- Watcher is initialized before `StartService` and torn down on server exit.

### E4.6: Logging
**As** an operator, **I want** structured, configurable log output **so that** I can route logs to files or external systems.

**Acceptance Criteria:**
- Log level is configurable via config file (`util.SetLogLevel`).
- Log output can be directed to a file via `logging.ConfigureLogOutput`.
- Log format uses `LogFormatter` (JSON-compatible structured output).
- Log rotation via `lumberjack` when file output is configured.

---

## E5: Terminal UI

### E5.1: TUI Management Console
**As** a developer, **I want** a terminal UI to monitor and manage a running proxy **so that** I can inspect logs and config without HTTP calls.

**Acceptance Criteria:**
- `--tui` flag launches a Bubbletea-based management console.
- Without `--standalone`, TUI connects to an already-running proxy on the configured port.
- With `--standalone`, TUI starts an embedded proxy in-process, captures its output, and connects locally.

### E5.2: Standalone TUI Mode
**As** a local developer, **I want** the proxy and TUI to launch together with one command **so that** I don't need two terminal windows.

**Acceptance Criteria:**
- `--tui --standalone` starts the embedded server in a background goroutine.
- TUI polls the embedded server until ready (up to 3 seconds with exponential backoff).
- I/O (stdout/stderr) is redirected so the TUI can capture and display server logs.
- Server and TUI both shut down cleanly when the TUI exits.

---

## E6: SDK and Extensibility

### E6.1: Go SDK
**As** a Go application author, **I want** a typed SDK for the proxy service **so that** I can embed or control the proxy programmatically.

**Acceptance Criteria:**
- `sdk/cliproxy` package exposes `TokenClientProvider`, `APIKeyClientProvider`, and `WatcherFactory` interfaces.
- `sdk/auth` provides `RegisterTokenStore` for plugging in a custom token persistence backend.
- `sdk/translator` exposes request/response translation primitives.
- `sdk/watcher` provides file-watch utilities for config and auth directory monitoring.

### E6.2: Python SDK
**As** a Python application author, **I want** a Python SDK **so that** I can interact with the proxy from Python tooling without raw HTTP calls.

**Acceptance Criteria:**
- `sdk/python` package exists with documented public interface.

### E6.3: Cloud Deploy Mode
**As** a cloud operator, **I want** the server to start in a waiting state when no config is present **so that** it does not crash on cold start before configuration is injected.

**Acceptance Criteria:**
- `DEPLOY=cloud` environment variable activates cloud deploy mode.
- When config is missing or empty in cloud mode, the server enters a standby state and waits for shutdown signals instead of exiting.
- When a valid config file is detected in cloud mode, normal service start proceeds.

---

## E7: Deployment

### E7.1: Docker Compose Deployment
**As** an operator, **I want** a one-command Docker deployment **so that** the proxy is production-ready within minutes.

**Acceptance Criteria:**
- Official image: `eceasy/cli-proxy-api-plus:latest`.
- `docker-compose.yml` binds port 8317, mounts `config.yaml`, auth directory, and log directory.
- `restart: unless-stopped` policy for resilience.

### E7.2: Bare-Metal / Direct Binary
**As** a developer, **I want** to run the proxy as a single binary without Docker **so that** it integrates into any environment.

**Acceptance Criteria:**
- `go build ./cmd/server` produces a self-contained binary.
- `config.example.yaml` documents all configuration keys.
- `.env` file loading is supported for environment variable injection at startup.

### E7.3: Operational CLI (cliproxyctl)
**As** an operator, **I want** a dedicated CLI tool for management operations **so that** I don't need to curl the management API directly.

**Acceptance Criteria:**
- `cliproxyctl` binary built from `cmd/cliproxyctl`.
- Supports all management API operations.
- Connects to a running proxy on the configured address.

---

## E8: AmpCode Management Sub-System

### E8.1: AmpCode Upstream Configuration
**As** an operator deploying the proxy for an AmpCode-compatible upstream, **I want** a dedicated REST API group for managing upstream connection parameters **so that** I can update the target URL and credentials without restarting the server.

**Acceptance Criteria:**
- `GET /v0/management/ampcode/upstream-url` returns the currently configured upstream base URL.
- `PUT /v0/management/ampcode/upstream-url` replaces the upstream URL; the change takes effect immediately for subsequent requests.
- `DELETE /v0/management/ampcode/upstream-url` clears the upstream URL, disabling AmpCode routing.
- `GET /v0/management/ampcode/upstream-api-key` returns the active single API key (masked or as configured).
- `PUT /v0/management/ampcode/upstream-api-key` replaces the single API key.
- `DELETE /v0/management/ampcode/upstream-api-key` removes the single API key.
- `GET /v0/management/ampcode/restrict-management-to-localhost` returns the current localhost-restriction flag.
- `PUT /v0/management/ampcode/restrict-management-to-localhost` sets or clears the flag; when set, management endpoints reject non-loopback callers.

### E8.2: AmpCode Upstream API Key Pool
**As** an operator with multiple upstream API keys, **I want** a pooled key list **so that** the proxy can load-balance or round-robin across keys.

**Acceptance Criteria:**
- `GET /v0/management/ampcode/upstream-api-keys` returns the list of all pooled keys.
- `PUT /v0/management/ampcode/upstream-api-keys` replaces the entire pool.
- `DELETE /v0/management/ampcode/upstream-api-keys` clears the pool.
- When the pool is non-empty, keys are selected per-request (round-robin or random).

### E8.3: AmpCode Model Mappings
**As** an operator, **I want** to define model name translation rules for the AmpCode upstream **so that** callers using standard model IDs are transparently mapped to upstream model identifiers.

**Acceptance Criteria:**
- `GET /v0/management/ampcode/model-mappings` returns the full mapping table.
- `PUT /v0/management/ampcode/model-mappings` replaces the entire mapping table atomically.
- `DELETE /v0/management/ampcode/model-mappings` clears all mappings.
- `GET /v0/management/ampcode/force-model-mappings` returns the force-map flag; when true, all requests use the mapping table even if an exact upstream model is specified.
- `PUT /v0/management/ampcode/force-model-mappings` sets or clears the force-map flag.
- `DELETE /v0/management/ampcode/force-model-mappings` resets the flag to its default (false).
- All mapping changes persist across server restarts via the active token/config storage backend.

---

## E9: BoardSync Release Tooling

### E9.1: GitHub Issue and PR Aggregation
**As** a maintainer of both `cliproxyapi-plusplus` and the upstream `cliproxyapi` repository, **I want** a dedicated binary that aggregates issues and pull requests from both repositories **so that** I can generate unified release changelogs without manual cross-repo correlation.

**Acceptance Criteria:**
- `boardsync` binary is built from `cmd/boardsync` and is included in the release artifact set.
- The binary fetches open and closed issues and PRs from `kooshapari/cliproxyapi-plusplus` and `kooshapari/cliproxyapi` via the `gh` CLI.
- Duplicate items (same title and number existing in both repos) are deduplicated before output.
- Output is written as JSON (default) or CSV (via flag) to stdout or a specified file.
- The binary targets up to 2000 aggregate items per run; limit is configurable via flag.
- `gh` CLI must be authenticated and available in `PATH`; the binary fails with a clear error if it is not.

---

## Non-Goals

- cliproxyapi-plusplus does not train or fine-tune models.
- It does not provide model hosting; it only proxies to existing provider endpoints.
- It does not provide a web dashboard UI (the TUI is terminal-only).
- Third-party provider support is community-maintained; upstream provides no support for those providers.
- The AmpCode management API does not validate upstream model availability; it only stores and applies configuration.
