# Architecture Decision Records — CLIProxyAPI Plus

---

## ADR-001: Go as Implementation Language

**Status**: Accepted
**Date**: 2024-Q4

### Context
CLIProxyAPI Plus is a high-throughput HTTP proxy requiring low latency, concurrent request handling, and a single deployable binary.

### Decision
Use Go (1.26+) as the sole implementation language.

### Rationale
- Native goroutine concurrency maps directly to concurrent proxy request handling.
- Single-binary distribution simplifies deployment (Docker, bare metal, CI runners).
- Strong stdlib for HTTP/WebSocket without heavy framework dependencies.
- Mainline CLIProxyAPI is Go — staying aligned avoids fork divergence.

### Alternatives Considered
- **Rust**: Higher performance ceiling but steeper contributor ramp-up and longer compile cycles.
- **Node.js/TypeScript**: Familiar to web devs but lacks Go's goroutine model for high-concurrency proxy work.

---

## ADR-002: gin as HTTP Framework

**Status**: Accepted
**Date**: 2024-Q4

### Context
Need an HTTP router that supports middleware chains, SSE streaming, and has low overhead.

### Decision
Use `github.com/gin-gonic/gin v1.10.1`.

### Rationale
- Fastest commonly-used Go HTTP router with middleware support.
- First-class SSE and streaming response support.
- Large ecosystem; familiar to Go contributors.
- Context-based request handling aligns with provider translator pattern.

### Alternatives Considered
- **net/http stdlib**: Less routing ergonomics; more boilerplate for middleware.
- **echo**: Comparable performance; gin chosen for wider contributor familiarity.
- **fiber**: Fasthttp-based; incompatible with standard `net/http` idioms used in SDK.

---

## ADR-003: Provider Plugin Architecture via Interface Registry

**Status**: Accepted
**Date**: 2024-Q4

### Context
CLIProxyAPI Plus must support multiple AI providers (Claude, Gemini, Cursor, GitLab Duo, Codex) without coupling core routing logic to provider-specific code.

### Decision
Define a common provider interface in `internal/interfaces`; register concrete implementations in `internal/registry` at startup from config.

### Rationale
- Open/Closed Principle: new providers added without modifying routing or auth layers.
- Community contributors can implement a single interface to add providers.
- Registry pattern allows runtime provider selection based on model name.

### Consequences
- All provider translators must implement the interface contract — breaking changes to the interface require all providers to update.
- Provider registry must be initialized before request routing begins.

---

## ADR-004: tiktoken-go for Token Counting

**Status**: Accepted
**Date**: 2025-Q1

### Context
Usage tracking requires accurate per-request token counts compatible with OpenAI's tokenization scheme.

### Decision
Use `github.com/tiktoken-go/tokenizer v0.7.0` for token counting.

### Rationale
- Go port of OpenAI's tiktoken — produces identical counts to the Python reference.
- Required for accurate billing attribution and quota enforcement.
- Avoids HTTP round-trips to count tokens server-side.

---

## ADR-005: Bubble Tea for TUI Dashboard

**Status**: Accepted
**Date**: 2025-Q1

### Context
Operators need a terminal-based monitoring dashboard for real-time proxy activity.

### Decision
Use `github.com/charmbracelet/bubbletea v1.3.10` with `lipgloss v1.1.0` for TUI rendering.

### Rationale
- Elm-Architecture TUI model (bubbletea) provides predictable state management for dynamic dashboards.
- lipgloss provides rich terminal styling without custom ANSI codes.
- Charm toolchain is the current Go TUI standard and has strong community support.
- Consistent with other Phenotype org tools using Charm stack.

---

## ADR-006: fsnotify for Hot Config Reload

**Status**: Accepted
**Date**: 2025-Q1

### Context
Production deployments require config changes (new API keys, provider toggles) without service restarts.

### Decision
Use `github.com/fsnotify/fsnotify v1.9.0` for config file watching.

### Rationale
- Cross-platform (Linux inotify, macOS kqueue, Windows ReadDirectoryChangesW).
- Minimal API surface: just watch a path and receive `Event` structs.
- In-flight requests complete against the old config; new requests use the reloaded config.

---

## ADR-007: Third-Party Provider Separation (Plus vs Mainline)

**Status**: Accepted
**Date**: 2024-Q4

### Context
The mainline CLIProxyAPI only supports first-party providers. Community wants to add providers without creating merge conflicts with mainline.

### Decision
Maintain CLIProxyAPI Plus as a separate repository that tracks mainline via periodic sync, with all third-party provider code isolated in `auths/` and `internal/registry/`.

### Rationale
- Clean separation of maintenance responsibility: mainline team does not review third-party provider code.
- Plus can release independently on community cadence.
- Providers added in Plus that gain wide adoption can be proposed back to mainline.

### Consequences
- Plus must periodically rebase/merge from mainline — merge conflicts possible at integration points.
- Breaking interface changes in mainline require Plus to update all third-party providers.
