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
```

## Contributing

1. Create a worktree branch.
2. Implement and validate changes.
3. Open a PR with clear scope and migration notes.

## License

MIT License. See `LICENSE`.
