# Architecture Decision Records — CLIProxyAPI++

## ADR-001 | Go HTTP Proxy Server | Adopted

**Status:** Adopted

**Context:** Need a high-performance LLM API proxy that can handle concurrent streaming connections with minimal latency overhead.

**Decision:** Use Go with net/http for the core proxy server on port 8317, implementing OpenAI-compatible endpoints that translate to multiple backend providers.

**Consequences:**
- Excellent concurrency via goroutines for streaming connections
- Single binary deployment simplifies Docker and bare-metal installs
- Strong stdlib HTTP support reduces external dependencies

---

## ADR-002 | Provider Abstraction Layer | Adopted

**Status:** Adopted

**Context:** The proxy must support multiple LLM providers (GitHub Copilot, Kiro/AWS) with different auth flows and API formats while exposing a unified OpenAI-compatible interface.

**Decision:** Implement a provider abstraction with pluggable auth adapters in `auths/`, model name conversion, and per-provider request/response translation.

**Consequences:**
- Adding new providers requires only a new auth adapter and model mapping
- Model name converter handles provider-specific naming transparently
- Each provider manages its own token lifecycle independently

---

## ADR-003 | OAuth Web UI for Kiro/AWS | Adopted

**Status:** Adopted

**Context:** Kiro authentication requires AWS Builder ID or Identity Center flows that involve browser-based OAuth redirects, unlike Copilot's device code flow.

**Decision:** Embed a web UI at `/v0/oauth/kiro` that handles the full OAuth PKCE flow, token import from IDE, and background refresh.

**Consequences:**
- Users can authenticate via browser without CLI interaction
- Token import supports copying tokens from Kiro IDE directly
- Background goroutine handles token refresh before expiry

---

## ADR-004 | Docker-First Deployment | Adopted

**Status:** Adopted

**Context:** Users need a simple, reproducible deployment that works across platforms with minimal configuration.

**Decision:** Provide `Dockerfile` and `docker-compose.yml` with YAML config via volume mount (`config.yaml`).

**Consequences:**
- Single `docker-compose up` for deployment
- Config changes require only volume-mounted YAML edits
- Multi-provider configuration in a single config file

---

## ADR-005 | Rate Limiting with Smart Cooldown | Adopted

**Status:** Adopted

**Context:** Provider APIs enforce rate limits; aggressive retries waste quota and risk bans.

**Decision:** Implement a rate limiter with smart cooldown manager that tracks per-provider limits and backs off exponentially.

**Consequences:**
- Prevents provider API ban from excessive requests
- Cooldown periods auto-adjust based on response headers
- Metrics module tracks usage for observability

---

## ADR-006 | Plus Fork Strategy | Adopted

**Status:** Adopted

**Context:** CLIProxyAPI++ is an enhanced fork adding multi-provider support, Kiro auth, and observability to the original CLIProxyAPI.

**Decision:** Maintain as independent "plus-plus" fork with additive features, preserving compatibility with upstream API contract.

**Consequences:**
- Upstream changes can be cherry-picked when compatible
- New features (Kiro, metrics, fingerprint) are additive, not breaking
- Original Copilot-only flow remains functional
