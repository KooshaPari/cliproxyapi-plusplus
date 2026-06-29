# Canonical Contracts Reference

cliproxy (Go) conforms to the provider-model schemas published at:

**<https://github.com/KooshaPari/phenotype-contracts>**

Pinned SHA (main at time of this commit):
`cc8f34ed34a3f1ae2ba7edd6810a902e51738693`

## Schemas

| Schema file | Description |
|---|---|
| `provider-models/oauth-refresh-policy.schema.json` | OAuth2 token refresh lead policy (per-provider parameter) |
| `provider-models/resilience-policy.schema.json` | Retry/backoff parameter set + retryable-error taxonomy + SSE terminal-marker rules |
| `provider-models/provider-model.schema.json` | Provider model shape (SseStopRule lives here) |

## How cliproxy conforms

cliproxy does **not** import the contracts repo as a Go dependency (the schemas
are language-neutral JSON Schema). Instead, conformance is validated at test
time by `sdk/auth/contract_conformance_test.go`.

### OAuth refresh-lead policy

The contract defines `default_refresh_lead_seconds = 300` (5 min) and
explicitly permits per-provider overrides via `per_provider_refresh_lead_seconds`.
cliproxy implements `RefreshLead() *time.Duration` per authenticator:

| Provider | cliproxy lead | Notes |
|---|---|---|
| antigravity | 5 min | matches contract default |
| claude | 4 h | per-provider override — legitimate |
| codebuddy | 24 h | per-provider override — explicitly cited as example in schema |
| codex | 5 days (120 h) | per-provider override — very long token lifetime |
| cursor | 10 min | per-provider override |
| gemini | nil (disabled) | API-key auth; no expiry window needed |
| github-copilot | nil (disabled) | token doesn't expire in OAuth sense |
| gitlab | 5 min | matches contract default |
| iflow | 24 h | per-provider override |
| kilo | nil (disabled) | device-flow; no refresh lead |
| kimi | 5 min | matches contract default |
| kiro | 20 min | per-provider override |
| qwen | 3 h | per-provider override |
| xai | 5 min | matches contract default (via pkg/llmproxy/auth/xai.RefreshLead) |

Nil leads indicate authenticators that return nil from `RefreshLead()`, meaning
no proactive refresh is scheduled — this is a valid implementation choice for
providers whose tokens do not expire on a predictable schedule.

### Resilience policy — retryable HTTP status codes

Contract canonical set: `[408, 429, 500, 502, 503, 504, 520, 522, 524, 529]`

cliproxy's conductor (`sdk/cliproxy/auth/conductor.go`) handles:
- `429` — rate limit, with Retry-After observation
- `408, 500, 502, 503, 504` — transient server/gateway errors

**Divergence (documented):** cliproxy does not currently retry on Cloudflare
codes `520, 522, 524`. `529` is handled in the Claude handler
(`sdk/api/handlers/claude/code_handlers.go`) as `overloaded_error` but is
classified rather than retried in the conductor. These are noted divergences
from the contract's canonical set; they do not indicate defects but should be
tracked for future alignment.

### SSE terminal markers

cliproxy recognises `[DONE]` as the SSE terminal marker in all streaming
handlers (`sdk/api/handlers/`). This aligns with the contract's
`SseStopRule` reference in `provider-model.schema.json` and the
`is_sse_terminal_reference` noted in `resilience-policy.schema.json`.

## Updating

When the contracts repo is updated, re-pin the SHA here and re-run:

```sh
go test ./sdk/auth/ -run TestContractConformance -v
```
