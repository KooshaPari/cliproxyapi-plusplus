<<<<<<< HEAD
# CLIProxyAPI Plus

English | [Chinese](README_CN.md)

This is the Plus version of [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI), adding support for third-party providers on top of the mainline project.

All third-party provider support is maintained by community contributors; CLIProxyAPI does not provide technical support. Please contact the corresponding community maintainer if you need assistance.

The Plus release stays in lockstep with the mainline features.

## Differences from the Mainline

- Added GitHub Copilot support (OAuth login), provided by [em4go](https://github.com/em4go/CLIProxyAPI/tree/feature/github-copilot-auth)
- Added Kiro (AWS CodeWhisperer) support (OAuth login), provided by [fuko2935](https://github.com/fuko2935/CLIProxyAPI/tree/feature/kiro-integration), [Ravens2121](https://github.com/Ravens2121/CLIProxyAPIPlus/)

## New Features (Plus Enhanced)

- **OAuth Web Authentication**: Browser-based OAuth login for Kiro with beautiful web UI
- **Rate Limiter**: Built-in request rate limiting to prevent API abuse
- **Background Token Refresh**: Automatic token refresh 10 minutes before expiration
- **Metrics & Monitoring**: Request metrics collection for monitoring and debugging
- **Device Fingerprint**: Device fingerprint generation for enhanced security
- **Cooldown Management**: Smart cooldown mechanism for API rate limits
- **Usage Checker**: Real-time usage monitoring and quota management
- **Model Converter**: Unified model name conversion across providers
- **UTF-8 Stream Processing**: Improved streaming response handling

## Kiro Authentication

### Web-based OAuth Login

Access the Kiro OAuth web interface at:

```
http://your-server:8080/v0/oauth/kiro
```

This provides a browser-based OAuth flow for Kiro (AWS CodeWhisperer) authentication with:
- AWS Builder ID login
- AWS Identity Center (IDC) login
- Token import from Kiro IDE

## Quick Deployment with Docker

### One-Command Deployment

```bash
# Create deployment directory
mkdir -p ~/cli-proxy && cd ~/cli-proxy

# Create docker-compose.yml
cat > docker-compose.yml << 'EOF'
services:
  cli-proxy-api:
    image: eceasy/cli-proxy-api-plus:latest
    container_name: cli-proxy-api-plus
    ports:
      - "8317:8317"
    volumes:
      - ./config.yaml:/CLIProxyAPI/config.yaml
      - ./auths:/root/.cli-proxy-api
      - ./logs:/CLIProxyAPI/logs
    restart: unless-stopped
EOF

# Download example config
curl -o config.yaml https://raw.githubusercontent.com/router-for-me/CLIProxyAPIPlus/main/config.example.yaml

# Pull and start
docker compose pull && docker compose up -d
```

### Configuration

Edit `config.yaml` before starting:

```yaml
# Basic configuration example
server:
  port: 8317

# Add your provider configurations here
```

### Update to Latest Version

```bash
cd ~/cli-proxy
docker compose pull && docker compose up -d
=======
# CLIProxyAPI++

Agent-native, multi-provider OpenAI-compatible proxy for production and local model routing.

## Table of Contents

- [Key Features](#key-features)
- [Architecture](#architecture)
- [Getting Started](#getting-started)
- [Operations and Security](#operations-and-security)
- [Testing and Quality](#testing-and-quality)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

## Key Features

- OpenAI-compatible request surface across heterogeneous providers.
- Unified auth and token handling for OpenAI, Anthropic, Gemini, Kiro, Copilot, and more.
- Provider-aware routing and model conversion.
- Built-in operational tooling for management APIs and diagnostics.

## Architecture

- `cmd/server`: primary API server entrypoint.
- `cmd/cliproxyctl`: operational CLI.
- `internal/`: runtime/auth/translator internals.
- `pkg/llmproxy/`: reusable proxy modules.
- `sdk/`: SDK-facing interfaces.

## Getting Started

### Prerequisites

- Go 1.24+
- Docker (optional)
- Provider credentials for target upstreams

### Quick Start

```bash
go build -o cliproxy ./cmd/server
./cliproxy --config config.yaml
```

### Docker Quick Start

```bash
docker run -p 8317:8317 eceasy/cli-proxy-api-plus:latest
```

## Operations and Security

- Rate limiting and quota/cooldown controls.
- Auth flows for provider-specific OAuth/API keys.
- CI policy checks and path guards.
- Governance and security docs under `docs/operations/` and `docs/reference/`.

## Testing and Quality

```bash
go test ./...
```

Quality gates are enforced via repo CI workflows (build/lint/path guards).

## Documentation

Primary docs root is `docs/` with a unified category IA:

- `docs/wiki/`
- `docs/development/`
- `docs/index/`
- `docs/api/`
- `docs/roadmap/`

VitePress docs commands:

```bash
cd docs
npm install
npm run docs:dev
npm run docs:build
>>>>>>> origin/main
```

## Contributing

<<<<<<< HEAD
This project only accepts pull requests that relate to third-party provider support. Any pull requests unrelated to third-party provider support will be rejected.

If you need to submit any non-third-party provider changes, please open them against the [mainline](https://github.com/router-for-me/CLIProxyAPI) repository.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
=======
1. Create a worktree branch.
2. Implement and validate changes.
3. Open a PR with clear scope and migration notes.

## License

MIT License. See `LICENSE`.
>>>>>>> origin/main
