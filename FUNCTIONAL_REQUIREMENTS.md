# Functional Requirements — CLIProxyAPI Plus

Traces to: PRD.md epics E1–E5.

---

## FR-PROXY: Core Proxy

| ID | Requirement | Status | Traces To |
|----|-------------|--------|-----------|
| FR-PROXY-001 | The system SHALL expose an HTTP endpoint at `/v1/chat/completions` accepting OpenAI-format JSON request bodies. | Implemented | E1.1 |
| FR-PROXY-002 | The system SHALL expose `/v1/models` returning available provider models in OpenAI list format. | Implemented | E1.1 |
| FR-PROXY-003 | The system SHALL support streaming responses via Server-Sent Events when `stream: true` is set. | Implemented | E1.1 |
| FR-PROXY-004 | The system SHALL return non-streaming responses with OpenAI-compatible `choices[0].message` structure. | Implemented | E1.1 |
| FR-PROXY-005 | The system SHALL fail with HTTP 400 and a descriptive error body when the request cannot be parsed. | Implemented | E1.1 |
| FR-PROXY-006 | The system SHALL select the upstream provider based on the `model` field or a configured default. | Implemented | E1.2 |

## FR-REGISTRY: Provider Registry

| ID | Requirement | Status | Traces To |
|----|-------------|--------|-----------|
| FR-REGISTRY-001 | The system SHALL register providers via a common interface defined in `internal/interfaces`. | Implemented | E1.2 |
| FR-REGISTRY-002 | The system SHALL load provider configuration from `config.yaml` at startup. | Implemented | E1.2 |
| FR-REGISTRY-003 | The system SHALL fail loudly (non-zero exit with message) if a required provider config key is missing. | Implemented | E1.2 |
| FR-REGISTRY-004 | The system SHALL support at minimum these providers: Anthropic Claude, Google Gemini, Cursor, GitLab Duo, Codex. | Partial | E1.2, E4.1 |

## FR-AUTH: Authentication

| ID | Requirement | Status | Traces To |
|----|-------------|--------|-----------|
| FR-AUTH-001 | The system SHALL support API-key authentication for providers using static bearer tokens. | Implemented | E1.3 |
| FR-AUTH-002 | The system SHALL support OAuth 2.0 flows for providers requiring browser-based auth. | Implemented | E1.3 |
| FR-AUTH-003 | The system SHALL store and refresh tokens via `internal/store`, not in plaintext config files. | Implemented | E1.3 |
| FR-AUTH-004 | The system SHALL reject requests with expired tokens with HTTP 401 and trigger a refresh flow. | Implemented | E1.3 |

## FR-SDK: Go SDK

| ID | Requirement | Status | Traces To |
|----|-------------|--------|-----------|
| FR-SDK-001 | The SDK SHALL provide a typed Go client for chat completions with streaming support. | Implemented | E2.1 |
| FR-SDK-002 | The SDK SHALL expose `api`, `auth`, `config`, `logging`, and `translator` sub-packages. | Implemented | E2.1 |
| FR-SDK-003 | The SDK documentation SHALL cover basic and advanced usage in `docs/sdk-usage.md` and `docs/sdk-advanced.md`. | Implemented | E2.1 |

## FR-WS: WebSocket Relay

| ID | Requirement | Status | Traces To |
|----|-------------|--------|-----------|
| FR-WS-001 | The system SHALL bridge WebSocket upstream responses to SSE for providers using WebSocket streaming. | Implemented | E2.2 |
| FR-WS-002 | The system SHALL close WebSocket connections gracefully and propagate upstream errors to the client. | Implemented | E2.2 |

## FR-OPS: Operational Features

| ID | Requirement | Status | Traces To |
|----|-------------|--------|-----------|
| FR-OPS-001 | The system SHALL track per-request token usage using tiktoken-go tokenizer. | Implemented | E3.1 |
| FR-OPS-002 | The system SHALL include token usage metadata in response bodies. | Implemented | E3.1 |
| FR-OPS-003 | The system SHALL cache responses with configurable TTL to avoid redundant upstream calls. | Implemented | E3.2 |
| FR-OPS-004 | The system SHALL hot-reload configuration on file change without dropping in-flight requests. | Implemented | E3.4 |
| FR-OPS-005 | The system SHALL display a TUI dashboard showing active requests, provider breakdown, and token counts. | Implemented | E3.3 |

## FR-DEPLOY: Deployment

| ID | Requirement | Status | Traces To |
|----|-------------|--------|-----------|
| FR-DEPLOY-001 | The system SHALL provide a Dockerfile producing a minimal Go binary image. | Implemented | E5.1 |
| FR-DEPLOY-002 | The system SHALL provide a goreleaser configuration for multi-platform binary releases. | Implemented | E5.2 |
| FR-DEPLOY-003 | Release binaries SHALL target linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64. | Planned | E5.2 |

---

## Traceability Matrix

| Epic | FRs |
|------|-----|
| E1.1 Core Proxy | FR-PROXY-001 to 006 |
| E1.2 Registry | FR-REGISTRY-001 to 004 |
| E1.3 Auth | FR-AUTH-001 to 004 |
| E2.1 SDK | FR-SDK-001 to 003 |
| E2.2 WebSocket | FR-WS-001 to 002 |
| E3.x Ops | FR-OPS-001 to 005 |
| E5.x Deployment | FR-DEPLOY-001 to 003 |
