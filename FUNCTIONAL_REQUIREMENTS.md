# Functional Requirements — cliproxyapi-plusplus

> Traces to: PRD.md epics E1–E9
> Module path: `github.com/kooshapari/CLIProxyAPI/v7`

---

## FR-PRX: Proxy Core

- **FR-PRX-001:** The system SHALL expose an OpenAI-compatible HTTP API on port 8317 (configurable via `config.yaml`). Traces to E1.1.
- **FR-PRX-002:** The system SHALL implement `POST /v1/chat/completions` accepting the OpenAI request schema. Traces to E1.1.
- **FR-PRX-003:** The system SHALL implement `GET /v1/models` returning a combined model list from all active providers. Traces to E1.2.
- **FR-PRX-004:** The system SHALL support streaming responses via SSE (`stream: true`). Traces to E1.1.
- **FR-PRX-005:** The system SHALL correctly assemble multi-byte UTF-8 characters split across SSE chunks before emitting them to the caller. Traces to E1.1.
- **FR-PRX-006:** The system SHALL translate model names in incoming requests to each provider's native model ID. Traces to E1.3.
- **FR-PRX-007:** The system SHALL translate OpenAI-format tool call payloads to each provider's native format and back. Traces to E1.3.
- **FR-PRX-008:** The system SHALL preserve extended thinking / reasoning blocks when translating Claude Anthropic responses. Traces to E1.3.

Translators verified in codebase: `pkg/llmproxy/translator/` — gemini, gemini-cli, claude, codex, kiro, antigravity, openai, acp.

---

## FR-AUTH: Authentication

### OAuth Browser Flows

- **FR-AUTH-001:** `--login` SHALL initiate Google/Gemini OAuth authentication. Traces to E2.1.
- **FR-AUTH-002:** `--claude-login` SHALL initiate Anthropic Claude OAuth authentication. Traces to E2.1.
- **FR-AUTH-003:** `--codex-login` SHALL initiate OpenAI Codex OAuth (authorization-code flow). Traces to E2.1.
- **FR-AUTH-004:** `--codex-device-login` SHALL initiate OpenAI Codex device-code flow. Traces to E2.1.
- **FR-AUTH-005:** `--qwen-login` SHALL initiate Qwen OAuth authentication. Traces to E2.1.
- **FR-AUTH-006:** `--kimi-login` SHALL initiate Kimi OAuth authentication. Traces to E2.1.
- **FR-AUTH-007:** `--antigravity-login` SHALL initiate Antigravity OAuth authentication. Traces to E2.1.
- **FR-AUTH-008:** `--iflow-login` SHALL initiate iFlow OAuth authentication. Traces to E2.1.
- **FR-AUTH-009:** `--iflow-cookie` SHALL initiate iFlow cookie-based authentication. Traces to E2.1.
- **FR-AUTH-010:** `--no-browser` SHALL suppress automatic browser launch for headless environments. Traces to E2.1.
- **FR-AUTH-011:** `--oauth-callback-port <port>` SHALL override the local OAuth callback port. Traces to E2.1.
- **FR-AUTH-012:** `--incognito` SHALL force browser launch in incognito/private mode. Traces to E2.1.
- **FR-AUTH-013:** `--no-incognito` SHALL force browser launch in non-incognito mode. Traces to E2.1.
- **FR-AUTH-014:** `--incognito` and `--no-incognito` SHALL be mutually exclusive; providing both SHALL produce a clear startup error. Traces to E2.1. Verified in `validateKiroIncognitoFlags`.

### AWS Kiro Authentication

- **FR-AUTH-015:** `--kiro-login` / `--kiro-google-login` SHALL initiate Kiro Google-backed OAuth and default to incognito mode. Traces to E2.2.
- **FR-AUTH-016:** `--kiro-aws-login` SHALL initiate Kiro AWS Builder ID device code flow. Traces to E2.2.
- **FR-AUTH-017:** `--kiro-aws-authcode` SHALL initiate Kiro AWS Builder ID authorization code flow. Traces to E2.2.
- **FR-AUTH-018:** `--kiro-import` SHALL import an existing Kiro token from `~/.aws/sso/cache/kiro-auth-token.json`. Traces to E2.2.
- **FR-AUTH-019:** The Kiro token refresh manager SHALL start automatically when `AuthDir` is set and SHALL be torn down on server shutdown. Traces to E2.2. Verified: `kiro.InitializeAndStart` + `defer kiro.StopGlobalRefreshManager()`.

### GitHub Copilot Authentication

- **FR-AUTH-020:** `--github-copilot-login` SHALL initiate GitHub Copilot device code authentication. Traces to E2.3.

### API Key and Service Account Authentication

- **FR-AUTH-021:** `--vertex-import <path>` SHALL import a Vertex AI service account key JSON file. Traces to E2.4.
- **FR-AUTH-022:** `--project_id <id>` SHALL set the GCP project ID for Gemini/Vertex. Traces to E2.4.
- **FR-AUTH-023:** The system SHALL load multiple API keys per provider from `config.yaml` and make them available for request routing. Traces to E2.4.

---

## FR-STORE: Token Storage Backends

- **FR-STORE-001:** The system SHALL default to a local file-based token store when no store environment variables are set. Traces to E3.1. Code path: `sdkAuth.NewFileTokenStore()`.
- **FR-STORE-002:** Setting `PGSTORE_DSN` SHALL activate a PostgreSQL-backed token store. Traces to E3.2.
- **FR-STORE-003:** The PostgreSQL store SHALL accept `PGSTORE_SCHEMA` and `PGSTORE_LOCAL_PATH` as optional overrides. Traces to E3.2.
- **FR-STORE-004:** Setting `GITSTORE_GIT_URL` SHALL activate a git-backed token store. Traces to E3.3. Requires `GITSTORE_GIT_USERNAME` and `GITSTORE_GIT_TOKEN`.
- **FR-STORE-005:** The git store SHALL commit initial config from `config.example.yaml` if no config file exists in the repo. Traces to E3.3.
- **FR-STORE-006:** Setting `OBJECTSTORE_ENDPOINT` SHALL activate an S3-compatible object token store. Traces to E3.4. Requires `OBJECTSTORE_ACCESS_KEY`, `OBJECTSTORE_SECRET_KEY`, `OBJECTSTORE_BUCKET`.
- **FR-STORE-007:** The object store SHALL support both `http://` and `https://` endpoint URLs with auto-detected TLS. Traces to E3.4.
- **FR-STORE-008:** All token store backends SHALL register via `sdkAuth.RegisterTokenStore` so all service components share the same persistence instance. Traces to E3.1–E3.4.
- **FR-STORE-009:** The system SHALL include a SQLite-backed token store as an embedded single-file alternative to PostgreSQL. Traces to E3.2. Dependency: `modernc.org/sqlite v1.46.1` (confirmed in `go.mod`).

---

## FR-RL: Rate Limiting

- **FR-RL-001:** The system SHALL enforce configurable per-client request rate limits via `pkg/llmproxy/ratelimit`. Traces to E4.1.
- **FR-RL-002:** When `DisableCooling` is false in config, the system SHALL implement smart backoff on upstream 429 responses. Traces to E4.1. Verified: `coreauth.SetQuotaCooldownDisabled(cfg.DisableCooling)`.
- **FR-RL-003:** Rate limit violations SHALL return HTTP 429 to the caller. Traces to E4.1.

---

## FR-CFG: Configuration

- **FR-CFG-001:** The system SHALL read configuration from a YAML file at the path given by `--config`, or from a default location derived from the working directory. Traces to E3.1.
- **FR-CFG-002:** `DEPLOY=cloud` SHALL activate cloud deploy mode, causing the server to wait in standby when no config is present rather than exiting. Traces to E6.3.
- **FR-CFG-003:** Config files SHALL be validated strictly at startup via `validateConfigFileStrict`; malformed configs SHALL fail with a clear error before the server starts. Traces to E4.2.
- **FR-CFG-004:** `.env` files in the working directory SHALL be auto-loaded at startup. Traces to E7.2.
- **FR-CFG-005:** The system SHALL support multiple simultaneous provider configurations in a single `config.yaml`. Traces to E2.4.

---

## FR-WATCH: Config Auto-Reload

- **FR-WATCH-001:** `managementasset.StartAutoUpdater` SHALL watch `config.yaml` and the auth directory for file changes. Traces to E4.5.
- **FR-WATCH-002:** On detecting a config or auth change, the watcher SHALL reload affected provider clients without requiring a server restart. Traces to E4.5.
- **FR-WATCH-003:** The watcher SHALL be initialized before `cmd.StartService` and outlive the service lifecycle. Traces to E4.5.

---

## FR-MON: Monitoring and Usage

- **FR-MON-001:** When `UsageStatisticsEnabled` is true in config, the system SHALL collect per-request usage statistics. Traces to E4.3. Verified: `usage.SetStatisticsEnabled(cfg.UsageStatisticsEnabled)`.
- **FR-MON-002:** Usage statistics SHALL be accessible via the management API. Traces to E4.3.
- **FR-MON-003:** The management API SHALL expose current configuration, provider health, and usage metrics. Traces to E4.4.

---

## FR-MGMT: Management API and CLI

- **FR-MGMT-001:** The management API SHALL be protected by a password (configurable via `--password`). Traces to E4.4.
- **FR-MGMT-002:** In TUI standalone mode the system SHALL auto-generate a per-process management password if none is provided. Traces to E5.2.
- **FR-MGMT-003:** `cliproxyctl` SHALL provide CLI access to all management API operations. Traces to E4.4.

---

## FR-LOG: Logging

- **FR-LOG-001:** Log level SHALL be configurable via `config.yaml`. Traces to E4.6. Verified: `util.SetLogLevel(cfg)`.
- **FR-LOG-002:** Log output SHALL be redirectable to a file via `logging.ConfigureLogOutput`. Traces to E4.6.
- **FR-LOG-003:** All log output SHALL use `LogFormatter` (structured, parseable format). Traces to E4.6.
- **FR-LOG-004:** Log files SHALL support rotation via `lumberjack`. Traces to E4.6.

---

## FR-TUI: Terminal UI

- **FR-TUI-001:** `--tui` SHALL launch a Bubbletea-based terminal management console. Traces to E5.1.
- **FR-TUI-002:** Without `--standalone`, the TUI SHALL connect to an already-running proxy on the configured port and password. Traces to E5.1.
- **FR-TUI-003:** `--tui --standalone` SHALL start an embedded in-process proxy before launching the TUI client. Traces to E5.2.
- **FR-TUI-004:** In standalone mode the TUI SHALL poll the embedded server (up to 30 attempts with exponential backoff, max 1s) and exit with an error if the server does not become ready. Traces to E5.2.
- **FR-TUI-005:** In standalone mode all proxy stdout/stderr SHALL be redirected to the TUI log panel; original streams SHALL be restored on TUI exit. Traces to E5.2.
- **FR-TUI-006:** Both the embedded server and TUI SHALL shut down cleanly when the TUI exits. Traces to E5.2.

---

## FR-SDK: SDK and Extensibility

- **FR-SDK-001:** `sdk/cliproxy` SHALL expose `TokenClientProvider` and `APIKeyClientProvider` interfaces for programmatic client loading. Traces to E6.1.
- **FR-SDK-002:** `sdk/auth` SHALL expose `RegisterTokenStore(store)` for plugging in custom token backends. Traces to E6.1.
- **FR-SDK-003:** `sdk/translator` SHALL expose request/response translation primitives for each supported provider. Traces to E6.1.
- **FR-SDK-004:** `sdk/watcher` SHALL expose file-watch utilities reusable outside the main server. Traces to E6.1.
- **FR-SDK-005:** `sdk/python` SHALL provide a Python package for interacting with the proxy. Traces to E6.2.

---

## FR-DEP: Deployment

- **FR-DEP-001:** The project SHALL provide a `docker-compose.yml` that maps port 8317, mounts `config.yaml`, auth directory, and log directory with `restart: unless-stopped`. Traces to E7.1.
- **FR-DEP-002:** The official Docker image SHALL be `eceasy/cli-proxy-api-plus:latest`. Traces to E7.1.
- **FR-DEP-003:** `go build ./cmd/server` SHALL produce a single self-contained binary with no runtime dependencies beyond the OS. Traces to E7.2.
- **FR-DEP-004:** `config.example.yaml` SHALL document all configuration keys with default values and descriptions. Traces to E7.2.
- **FR-DEP-005:** `cmd/cliproxyctl` SHALL build a separate `cliproxyctl` binary. Traces to E7.3.
- **FR-DEP-006:** `cmd/releasebatch` SHALL build a release batch tooling binary. Traces to E7.2.

---

## FR-AMP: AmpCode Management Sub-System

- **FR-AMP-001:** `GET /v0/management/ampcode/upstream-url` SHALL return the currently configured AmpCode upstream base URL. Traces to E8.1.
- **FR-AMP-002:** `PUT /v0/management/ampcode/upstream-url` SHALL replace the upstream URL and apply the change immediately. Traces to E8.1.
- **FR-AMP-003:** `DELETE /v0/management/ampcode/upstream-url` SHALL clear the upstream URL and disable AmpCode routing. Traces to E8.1.
- **FR-AMP-004:** `GET /v0/management/ampcode/upstream-api-key` SHALL return the active single API key. Traces to E8.1.
- **FR-AMP-005:** `PUT /v0/management/ampcode/upstream-api-key` SHALL replace the single API key. Traces to E8.1.
- **FR-AMP-006:** `DELETE /v0/management/ampcode/upstream-api-key` SHALL remove the single API key. Traces to E8.1.
- **FR-AMP-007:** `GET /v0/management/ampcode/restrict-management-to-localhost` SHALL return the localhost-restriction flag. Traces to E8.1.
- **FR-AMP-008:** `PUT /v0/management/ampcode/restrict-management-to-localhost` SHALL set or clear the localhost-restriction flag; when set, management endpoints SHALL reject non-loopback callers. Traces to E8.1.
- **FR-AMP-009:** `GET /v0/management/ampcode/upstream-api-keys` SHALL return all pooled API keys. Traces to E8.2.
- **FR-AMP-010:** `PUT /v0/management/ampcode/upstream-api-keys` SHALL replace the entire API key pool atomically. Traces to E8.2.
- **FR-AMP-011:** `DELETE /v0/management/ampcode/upstream-api-keys` SHALL clear the API key pool. Traces to E8.2.
- **FR-AMP-012:** When the API key pool is non-empty, the system SHALL select keys per-request using round-robin or random strategy. Traces to E8.2.
- **FR-AMP-013:** `GET /v0/management/ampcode/model-mappings` SHALL return the full model name translation table. Traces to E8.3.
- **FR-AMP-014:** `PUT /v0/management/ampcode/model-mappings` SHALL replace the mapping table atomically. Traces to E8.3.
- **FR-AMP-015:** `DELETE /v0/management/ampcode/model-mappings` SHALL clear all model mappings. Traces to E8.3.
- **FR-AMP-016:** `GET /v0/management/ampcode/force-model-mappings` SHALL return the force-map flag. Traces to E8.3.
- **FR-AMP-017:** `PUT /v0/management/ampcode/force-model-mappings` SHALL set or clear the force-map flag. Traces to E8.3.
- **FR-AMP-018:** `DELETE /v0/management/ampcode/force-model-mappings` SHALL reset the force-map flag to false. Traces to E8.3.
- **FR-AMP-019:** All AmpCode management configuration changes SHALL persist across server restarts via the active storage backend. Traces to E8.1–E8.3.

Test coverage: `test/amp_management_test.go` covers all 19 FR-AMP items.

---

## FR-BSY: BoardSync Release Tooling

- **FR-BSY-001:** `boardsync` binary SHALL be buildable from `cmd/boardsync` and SHALL be included in the release artifact set. Traces to E9.1.
- **FR-BSY-002:** `boardsync` SHALL fetch issues and pull requests from `kooshapari/cliproxyapi-plusplus` and `kooshapari/cliproxyapi` via the `gh` CLI. Traces to E9.1.
- **FR-BSY-003:** `boardsync` SHALL deduplicate items that appear in both repositories (matched by title and number) before writing output. Traces to E9.1.
- **FR-BSY-004:** `boardsync` SHALL write aggregated results as JSON to stdout by default. Traces to E9.1.
- **FR-BSY-005:** `boardsync` SHALL support a `--csv` flag to emit output as CSV instead of JSON. Traces to E9.1.
- **FR-BSY-006:** `boardsync` SHALL support a `--limit` flag controlling the maximum number of items fetched (default 2000). Traces to E9.1.
- **FR-BSY-007:** `boardsync` SHALL fail with a clear, non-zero exit code and actionable error message if `gh` CLI is not present or not authenticated. Traces to E9.1.
