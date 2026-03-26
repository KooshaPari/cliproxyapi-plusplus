# Architecture Decision Records — cliproxyapi-plusplus

> Module path: `github.com/kooshapari/CLIProxyAPI/v7`

---

## ADR-001 | Go with net/http and Gin for the Proxy Server | Adopted

**Status:** Adopted

**Context:**
The proxy must handle many concurrent SSE streaming connections from AI agents and IDE tools with minimal latency added. Binary distribution and minimal runtime dependencies are required for easy deployment alongside CLI tools.

**Decision:**
Use Go 1.26+ as the implementation language with Gin (`github.com/gin-gonic/gin`) as the HTTP framework for the API surface, and stdlib `net/http` for lower-level proxy mechanics.

**Code evidence:**
- `go.mod`: `github.com/gin-gonic/gin v1.12.0`
- `cmd/server/main.go`: primary server entrypoint

**Consequences:**
- Goroutine-per-connection model handles thousands of concurrent SSE streams efficiently.
- Single statically-linked binary simplifies Docker images and bare-metal installs.
- Gin provides typed request binding and middleware chaining for the management API layer.
- Trade-off: Go's build times are longer than scripted languages; mitigated by `air.toml` for development hot-reload.

---

## ADR-002 | Provider Abstraction with Per-Provider Auth Adapters | Adopted

**Status:** Adopted

**Context:**
The proxy must support providers with fundamentally different auth flows: OAuth PKCE, device code, API keys, browser-based cookie capture, and AWS IAM Identity Center. A monolithic auth handler would be unextensible and hard to test.

**Decision:**
Each provider has a dedicated auth adapter under `pkg/llmproxy/auth/<provider>/`. The adapters implement a shared interface that handles token acquisition, storage, refresh, and revocation. New providers require only a new adapter directory.

**Code evidence:**
- `pkg/llmproxy/auth/`: claude/, codex/, copilot/, gemini/, iflow/, kilo/, kimi/, kiro/, qwen/, vertex/, antigravity/
- `internal/auth/base/token_storage.go`: shared base types
- `pkg/llmproxy/refresh_registry.go`: per-provider refresh lifecycle registry

**Consequences:**
- Third-party provider support (Kiro, iFlow, Kimi, Antigravity) can be added without touching core routing.
- Each provider's token lifecycle is independently managed; failures in one provider's refresh do not affect others.
- Token storage is abstracted via `sdk/auth.RegisterTokenStore`, allowing backends to be swapped at startup.

---

## ADR-003 | Pluggable Token Storage Backends | Adopted

**Status:** Adopted

**Context:**
The proxy is deployed in multiple environments: developer laptops (local file system), cloud VMs (no persistent disk), multi-instance deployments (shared state required), and audit-heavy environments (change history required).

**Decision:**
Token storage is pluggable via a registered backend. Four backends are implemented: local file (default), PostgreSQL (`PGSTORE_DSN`), git-backed (`GITSTORE_GIT_URL`), and S3-compatible object store (`OBJECTSTORE_ENDPOINT`). All backends implement a common interface registered via `sdkAuth.RegisterTokenStore`.

**Code evidence:**
- `pkg/llmproxy/store/`: PostgresStore, GitTokenStore, ObjectTokenStore
- `cmd/server/main.go`: backend selection and `sdkAuth.RegisterTokenStore(...)` calls
- `sdk/auth/`: `RegisterTokenStore`, `NewFileTokenStore`

**Consequences:**
- Multi-instance deployments can share token state via PostgreSQL or S3 without coordination.
- Git-backed storage provides an audit trail of token changes at no additional infrastructure cost.
- The file-based default requires zero configuration for local development.
- Trade-off: PostgreSQL and git backends add I/O latency on token read/write; acceptable since tokens are read at startup and on refresh, not per-request.

---

## ADR-004 | Provider-Specific Request/Response Translators | Adopted

**Status:** Adopted

**Context:**
Each upstream provider uses a different request and response schema. Callers use the OpenAI format. Translating inside each request handler would create deep coupling and duplicate the translation logic across providers.

**Decision:**
A dedicated translator package under `pkg/llmproxy/translator/` is created per provider. Each translator converts OpenAI `ChatCompletionRequest` to the provider's native format and converts the native response back to OpenAI `ChatCompletionResponse`. Translators are registered in `sdk/translator` and selected at request routing time.

**Code evidence:**
- `pkg/llmproxy/translator/`: gemini/, gemini-cli/, claude/, codex/, kiro/, antigravity/, openai/, acp/, translator/ (registry)
- `sdk/translator/`: public translator interface

**Consequences:**
- Adding a new provider requires only a new translator; the routing pipeline is unchanged.
- Thinking blocks (Claude extended reasoning) are handled in the Claude translator without leaking into generic types.
- Tool call schema differences (OpenAI vs Anthropic vs Gemini) are isolated per translator.

---

## ADR-005 | Config File Watching for Zero-Downtime Reloads | Adopted

**Status:** Adopted

**Context:**
Operators frequently update API keys or add provider configurations without wanting to restart the proxy (which would interrupt active SSE streams).

**Decision:**
`managementasset.StartAutoUpdater` uses `github.com/fsnotify/fsnotify` to watch `config.yaml` and the auth directory. When changes are detected, the affected provider clients are reloaded in-place without restarting the server.

**Code evidence:**
- `go.mod`: `github.com/fsnotify/fsnotify v1.9.0`
- `cmd/server/main.go`: `managementasset.StartAutoUpdater(context.Background(), configFilePath)`
- `pkg/llmproxy/watcher/`: watcher implementation with diff/synthesizer sub-packages

**Consequences:**
- Active SSE streams are not interrupted when the config changes.
- There is a brief window where the old and new configs may coexist; mitigated by atomic config swap in the reload handler.
- Auth directory watching catches new tokens written by `--kiro-login` or `--github-copilot-login` without requiring a restart.

---

## ADR-006 | Bubbletea TUI for Operator UX | Adopted

**Status:** Adopted

**Context:**
Operators running the proxy locally want to see live logs, provider status, and the active config without opening a browser or running curl commands. A rich terminal UI provides this without adding web infrastructure.

**Decision:**
`github.com/charmbracelet/bubbletea` is used for the TUI. Two modes are supported: standalone (TUI starts an embedded server) and client-only (TUI connects to an already-running server). The TUI communicates with the proxy via the management API using a local HTTP client.

**Code evidence:**
- `go.mod`: `github.com/charmbracelet/bubbletea v1.3.10`, `github.com/charmbracelet/lipgloss v1.1.0`, `github.com/charmbracelet/bubbles v1.0.0`
- `cmd/server/main.go`: `--tui` and `--standalone` flag handling, `tui.Run(...)`, `tui.NewClient(...)`
- `pkg/llmproxy/tui/`: TUI implementation

**Consequences:**
- Operators get a first-class terminal experience without web infrastructure.
- Standalone mode allows the proxy and TUI to co-exist in a single terminal window.
- I/O redirection in standalone mode (capturing stdout/stderr for the log panel) is a subtle complexity; carefully restored on TUI exit.
- Trade-off: The TUI is not usable in pure log-streaming CI/CD environments; `--tui` is opt-in.

---

## ADR-007 | Kiro Incognito Mode Default | Adopted

**Status:** Adopted

**Context:**
Kiro authentication opens a browser for OAuth. Developers commonly have multiple AWS accounts (personal, work, client) and need to log into different Kiro identities without their existing browser sessions interfering.

**Decision:**
All Kiro login flags default to incognito mode (`cfg.IncognitoBrowser = true`). Users can override with `--no-incognito` to reuse their existing browser session. `--incognito` and `--no-incognito` are mutually exclusive and validated at startup.

**Code evidence:**
- `cmd/server/main.go`: `setKiroIncognitoMode(cfg, useIncognito, noIncognito)` called for all Kiro login paths
- `validateKiroIncognitoFlags`: returns error if both flags are set

**Consequences:**
- Multi-account Kiro workflows work correctly by default.
- Users with SSO or pre-authenticated browser sessions can opt out with `--no-incognito`.
- The default is unusual (most OAuth flows use the existing browser session); documented in `--help` output to avoid confusion.

---

## ADR-008 | Cloud Deploy Standby Mode | Adopted

**Status:** Adopted

**Context:**
In cloud deployments (Kubernetes, ECS, etc.) the container may start before configuration has been injected via a ConfigMap or secret. A hard exit on missing config would trigger crash loops.

**Decision:**
When `DEPLOY=cloud` is set and no valid config file is found (missing, empty, or port is 0), the server enters a standby state (`cmd.WaitForCloudDeploy()`) instead of exiting. It waits for OS shutdown signals and logs a clear message that configuration is pending.

**Code evidence:**
- `cmd/server/main.go`: `deployEnv := os.Getenv("DEPLOY")`, `isCloudDeploy = true` check, `cmd.WaitForCloudDeploy()` fallback

**Consequences:**
- Kubernetes deployments with late config injection do not crash-loop.
- The standby state is explicit and logged; operators know the server is waiting.
- Normal startup (when config is valid) is unaffected.

---

## ADR-009 | Single Token Store Registration per Process | Adopted

**Status:** Adopted

**Context:**
Multiple service components (auth adapters, config reloader, management API) all need to read and write tokens. Using separate store instances per component would cause cache inconsistencies and race conditions.

**Decision:**
`sdkAuth.RegisterTokenStore` is called exactly once at startup with the selected backend. All components obtain their token access through this single registered instance. The registration is backend-agnostic: the same interface is used regardless of whether the backend is a local file, PostgreSQL, git, or object store.

**Code evidence:**
- `cmd/server/main.go`: exactly one `sdkAuth.RegisterTokenStore(...)` call per startup path
- `sdk/auth/`: `RegisterTokenStore`, store interface, `NewFileTokenStore`

**Consequences:**
- Consistent token state across all service components.
- Backend can be changed by operators without modifying component code.
- A single registration point makes startup audit straightforward.

---

## ADR-010 | Strict Config Validation Before Service Start | Adopted

**Status:** Adopted

**Context:**
A misconfigured proxy (e.g., overlapping port settings, invalid provider credentials format) would fail at request time, often in ways that are hard to diagnose. Validating config eagerly at startup produces a clear, actionable error before any requests are served.

**Decision:**
`validateConfigFileStrict(configFilePath)` is called immediately after loading the config and before any service is started. If validation fails, the process exits with a descriptive error. Validation is skipped if no config file exists (first-run scenario).

**Code evidence:**
- `cmd/server/main.go`: `validateConfigFileStrict` call
- `cmd/server/config_validate.go`: strict validation logic
- `cmd/server/config_validate_test.go`: validation test coverage

**Consequences:**
- Misconfigured deployments fail fast with a clear error rather than serving bad requests silently.
- The validation is strict: unknown keys and type mismatches are rejected.
- First-run (no config) is handled gracefully; the example config documents all valid keys.
