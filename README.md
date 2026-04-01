# cliproxyapi-plusplus
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FKooshaPari%2Fcliproxyapi-plusplus.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2FKooshaPari%2Fcliproxyapi-plusplus?ref=badge_shield)


Agent-native, multi-provider OpenAI-compatible proxy for production and local model routing.

This is the Plus version of [cliproxyapi-plusplus](https://github.com/kooshapari/cliproxyapi-plusplus), adding support for third-party providers on top of the mainline project.

All third-party provider support is maintained by community contributors; cliproxyapi-plusplus does not provide technical support. Please contact the corresponding community maintainer if you need assistance.

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
curl -o config.yaml https://raw.githubusercontent.com/kooshapari/cliproxyapi-plusplus/main/config.example.yaml

# Pull and start
docker compose pull && docker compose up -d
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

- `docs/start-here.md` - Getting started guide
- `docs/provider-usage.md` - Provider configuration
- `docs/provider-quickstarts.md` - Per-provider guides
- `docs/api/` - API reference
- `docs/sdk-usage.md` - SDK guides

## Environment

```bash
cd docs
npm install
npm run docs:dev
npm run docs:build
```

---

This project only accepts pull requests that relate to third-party provider support. Any pull requests unrelated to third-party provider support will be rejected.

If you need to submit any non-third-party provider changes, please open them against the [mainline](https://github.com/kooshapari/cliproxyapi-plusplus) repository.

## License

MIT License. See `LICENSE`.


[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FKooshaPari%2Fcliproxyapi-plusplus.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2FKooshaPari%2Fcliproxyapi-plusplus?ref=badge_large)