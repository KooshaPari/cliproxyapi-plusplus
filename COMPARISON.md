# Comparison Matrix

## Feature Comparison

This document compares **cliproxyapi-plusplus** with similar tools in the OpenAI-compatible proxy and LLM routing space.

| Repository | Purpose | Key Features | Language/Framework | Maturity | Comparison |
|------------|---------|--------------|-------------------|----------|------------|
| **cliproxyapi-plusplus (this fork)** | Agent-native multi-provider proxy | Extended providers, OpenAI-compatible, Unified auth | Go | Stable | Plus fork with third-party support |
| [litellm](https://github.com/BerriAI/litellm) | LLM proxy | 100+ providers, OpenAI-compatible | Python | Stable | Industry standard |
| [portkey](https://github.com/PortKey-AI/openapi) | LLM gateway | Observability, Retries, Load balancing | Python | Stable | Enterprise-focused |
| [go-openai-proxy](https://github.com/linjiao/go-openai-proxy) | Simple proxy | Basic routing, OpenAI-compatible | Go | Beta | Minimal alternative |
| [localai](https://github.com/mudler/LocalAI) | Local LLM proxy | Self-hosted, OpenAI-compatible | Go | Stable | Local models focus |
| [ollama](https://github.com/ollama/ollama) | Local LLM runner | Local inference, Simple API | Go | Stable | Local model runner |
| [textgen-webui](https://github.com/oobabooga/text-generation-webui) | WebUI for LLMs | Web interface, Extensions | Python | Stable | Web-based local |

## Detailed Feature Comparison

### Provider Support

| Provider | cliproxyapi++ | LiteLLM | Portkey | LocalAI |
|----------|--------------|---------|---------|---------|
| OpenAI | ✅ | ✅ | ✅ | ✅ |
| Anthropic | ✅ | ✅ | ✅ | ✅ |
| Google Gemini | ✅ | ✅ | ✅ | ❌ |
| Azure OpenAI | ✅ | ✅ | ✅ | ✅ |
| AWS Bedrock | ✅ | ✅ | ✅ | ✅ |
| Kiro | ✅ | ❌ | ❌ | ❌ |
| Copilot | ✅ | ❌ | ❌ | ❌ |
| Local Models | ❌ | ✅ | ✅ | ✅ |

### Security & Rate Limiting

| Feature | cliproxyapi++ | LiteLLM | Portkey | localai |
|---------|--------------|---------|---------|---------|
| Rate Limiting | ✅ | ✅ | ✅ | ✅ |
| Quota Controls | ✅ | ✅ | ✅ | ❌ |
| Cooldown Periods | ✅ | ❌ | ✅ | ❌ |
| Auth Flows | ✅ | ✅ | ✅ | ✅ |
| Secrets Management | ✅ | ✅ | ✅ | ✅ |

### Operations & Observability

| Feature | cliproxyapi++ | LiteLLM | Portkey |
|---------|--------------|---------|---------|
| Management API | ✅ | ❌ | ✅ |
| Diagnostics | ✅ | ❌ | ✅ |
| Request Logging | ✅ | ✅ | ✅ |
| Cost Tracking | ✅ | ✅ | ✅ |
| CI Policy Checks | ✅ | ❌ | ❌ |

## Fork Enhancements

This fork extends the mainline with:

| Feature | Mainline | This Fork |
|---------|----------|-----------|
| Third-party Providers | Limited | Extended (Kiro, Copilot, etc.) |
| Provider Support | Basic | Full (auth, routing, conversion) |
| Community Contributions | ❌ | ✅ (community-maintained) |

## Unique Value Proposition

cliproxyapi-plusplus provides:

1. **Agent-Native**: Designed for CLI agent integration
2. **Extended Providers**: Supports Kiro, Copilot, and more
3. **Unified Auth**: Single auth flow for heterogeneous providers
4. **Provider-Aware Routing**: Intelligent routing with model conversion

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     cliproxyapi-plusplus                     │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              OpenAI-Compatible API                    │  │
│  └──────────────────────────────────────────────────────┘  │
│              │                    │                       │
│  ┌───────────▼───┐    ┌──────────▼────────┐              │
│  │  Auth Layer   │    │  Routing Engine   │              │
│  └───────────────┘    └──────────────────┘              │
│              │                    │                       │
│  ┌───────────┴────────────────────┴────────────────┐     │
│  │              Translator Layer                   │     │
│  │         (Provider-specific conversion)          │     │
│  └────────────────────────────────────────────────┘     │
│              │            │            │                 │
│     ┌────────┘            │            └────────┐        │
│     ▼                     ▼                     ▼        │
│  OpenAI    Google/Anthropic    Third-party     ...      │
└─────────────────────────────────────────────────────────────┘
```

## When to Use What

| Use Case | Recommended Tool |
|----------|-----------------|
| Third-party providers (Kiro, Copilot) | cliproxyapi++ |
| Quick setup, many providers | LiteLLM |
| Enterprise observability | Portkey |
| Local model serving | LocalAI, Ollama |
| Python-based ecosystem | LiteLLM |

## References

- Mainline: [KooshaPari/cliproxyapi-plusplus](https://github.com/kooshapari/cliproxyapi-plusplus)
- LiteLLM: [BerriAI/litellm](https://github.com/BerriAI/litellm)
- Portkey: [PortKey-AI/openapi](https://github.com/PortKey-AI/openapi)
- LocalAI: [mudler/LocalAI](https://github.com/mudler/LocalAI)
