# Product Requirements Document — CLIProxyAPI++

## E1: Multi-Provider LLM Proxy

### E1.1: Provider Abstraction
**As** a developer, **I want** a unified API proxy supporting Claude, Copilot, Kiro, and other providers **so that** I can access any model through a single endpoint.

**Acceptance Criteria:**
- OpenAI-compatible API endpoint at `:8317`
- Provider-specific authentication (OAuth, API key)
- Model name conversion across providers

### E1.2: GitHub Copilot Support
**As** a developer, **I want** GitHub Copilot OAuth login **so that** I can proxy requests through my Copilot subscription.

### E1.3: Kiro (AWS CodeWhisperer) Support
**As** a developer, **I want** Kiro OAuth login via web UI **so that** I can proxy requests through AWS Builder ID or Identity Center.

**Acceptance Criteria:**
- Web-based OAuth at `/v0/oauth/kiro`
- AWS Builder ID and Identity Center login flows
- Token import from Kiro IDE

## E2: Enhanced Features

### E2.1: Rate Limiter
**As** an operator, **I want** built-in rate limiting **so that** API abuse is prevented.

### E2.2: Background Token Refresh
**As** a developer, **I want** automatic token refresh before expiration **so that** requests are never interrupted by expired tokens.

### E2.3: Metrics and Monitoring
**As** an operator, **I want** request metrics collection **so that** I can monitor usage and debug issues.

### E2.4: Device Fingerprint
**As** a security engineer, **I want** device fingerprint generation **so that** requests are tied to specific devices.

### E2.5: Cooldown Management
**As** an operator, **I want** smart cooldown for API rate limits **so that** the proxy backs off gracefully.

### E2.6: Usage Checker
**As** a developer, **I want** real-time usage monitoring **so that** I can track quota consumption.

### E2.7: UTF-8 Stream Processing
**As** a developer, **I want** improved streaming response handling **so that** multi-byte characters are processed correctly.

## E3: Deployment

### E3.1: Docker Deployment
**As** an operator, **I want** one-command Docker deployment **so that** the proxy is running in seconds.

**Acceptance Criteria:**
- `docker-compose.yml` with volume-mounted config
- Pre-built image at `eceasy/cli-proxy-api-plus:latest`
