# Functional Requirements — CLIProxyAPI++

## FR-PRX: Proxy Core

- **FR-PRX-001:** The system SHALL expose an OpenAI-compatible API on port 8317.
- **FR-PRX-002:** The system SHALL support model name conversion across providers.
- **FR-PRX-003:** The system SHALL handle UTF-8 streaming responses correctly.

## FR-AUTH: Authentication

- **FR-AUTH-001:** The system SHALL support GitHub Copilot OAuth login.
- **FR-AUTH-002:** The system SHALL support Kiro OAuth via web UI at `/v0/oauth/kiro`.
- **FR-AUTH-003:** Kiro auth SHALL support AWS Builder ID and Identity Center login.
- **FR-AUTH-004:** The system SHALL support token import from Kiro IDE.
- **FR-AUTH-005:** Background token refresh SHALL occur before token expiration.

## FR-RL: Rate Limiting

- **FR-RL-001:** The system SHALL enforce configurable request rate limits.
- **FR-RL-002:** The system SHALL implement smart cooldown on provider rate limits.

## FR-MON: Monitoring

- **FR-MON-001:** The system SHALL collect request metrics (count, latency, errors).
- **FR-MON-002:** The system SHALL provide real-time usage monitoring and quota tracking.

## FR-SEC: Security

- **FR-SEC-001:** The system SHALL generate device fingerprints for request attribution.

## FR-DEP: Deployment

- **FR-DEP-001:** The system SHALL support Docker deployment via docker-compose.
- **FR-DEP-002:** Configuration SHALL be via YAML file with volume mount support.

## FR-CFG: Configuration

- **FR-CFG-001:** The system SHALL read configuration from `config.yaml`.
- **FR-CFG-002:** The system SHALL support multiple provider configurations simultaneously.
