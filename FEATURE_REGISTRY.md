# CLIPROXYAPI++ Feature Registry

This document catalogs all features, providers, and modules in the fork ecosystem.

---

## PART 1: PROVIDERS (Executors)

### Upstream Providers (Original)
| Provider | File | Description |
|----------|------|-------------|
| OpenAI | `openai_compat_executor.go` | Standard OpenAI API |
| Claude | `claude_executor.go` | Anthropic Claude API |
| Gemini | `gemini_executor.go` | Google Gemini API |
| Gemini Vertex | `gemini_vertex_executor.go` | Google Vertex AI |
| Gemini CLI | `gemini_cli_executor.go` | Google Gemini CLI |
| Codex | `codex_executor.go` | OpenAI Codex |
| Codex WebSocket | `codex_websockets_executor.go` | Codex WebSocket transport |
| GitHub Copilot | `github_copilot_executor.go` | GitHub Copilot |

### Fork Providers (New)
| Provider | File | Description |
|----------|------|-------------|
| **MiniMax/iflow** | `iflow_executor.go` | MiniMax via iflow |
| **Kiro** | `kiro_executor.go` | AWS Kiro (CodeWhisperer) |
| **Kimi** | `kimi_executor.go` | Kimi AI |
| **Qwen** | `qwen_executor.go` | Alibaba Qwen |
| **Kilo** | `kilo_executor.go` | Kilo AI |
| **AiStudio** | `aistudio_executor.go` | Google AI Studio |
| **Antigravity** | `antigravity_executor.go` | Custom provider |

### Registered Providers (providers.json)
```
minimax, roo, kilo, deepseek, groq, mistral, siliconflow, 
openrouter, together, fireworks, novita, zen, nim
```

---

## PART 2: CORE MODULES

### Authentication
| Module | Path | Description |
|--------|------|-------------|
| OAuth Web | `auth/kiro/oauth_web.go` | Kiro OAuth |
| OAuth SSO OIDC | `auth/kiro/sso_oidc.go` | Kiro SSO |
| Social Auth | `auth/kiro/social_auth.go` | Social login |
| Token (Kimi) | `auth/kimi/token.go` | Kimi tokens |
| Token (Qwen) | `auth/qwen/qwen_token.go` | Qwen tokens |
| Token (Copilot) | `auth/copilot/token.go` | Copilot tokens |
| Token (Claude) | `auth/claude/` | Anthropic OAuth |
| Token (Gemini) | `auth/gemini/gemini_token.go` | Gemini tokens |
| Token (Codex) | `auth/codex/token.go` | Codex auth |
| PKCE | `auth/*/pkce.go` | PKCE flows |

### Translation/Request Building
| Module | Path | Description |
|--------|------|-------------|
| Kiro→OpenAI | `translator/kiro/openai/` | Kiro request translation |
| Kiro→Claude | `translator/kiro/claude/` | Kiro Claude translation |
| Gemini→Claude | `translator/gemini/claude/` | Gemini Claude translation |
| Claude→OpenAI | `translator/claude/openai/` | Claude to OpenAI |
| Antigravity | `translator/antigravity/` | Antigravity translation |

### Thinking/Reasoning
| Module | Path | Description |
|--------|------|-------------|
| MiniMax Thinking | `thinking/provider/iflow/` | iflow reasoning |
| Generic Thinking | `thinking/apply.go` | Thinking config |

### Storage
| Module | Path | Description |
|--------|------|-------------|
| Cursor Storage | `cursorstorage/cursor_storage.go` | Cursor session storage |
| Auth Store | `store/` | Auth token storage |

### Routing
| Module | Path | Description |
|--------|------|-------------|
| Pareto Router | `registry/pareto_router.go` | Pareto distribution |
| Aliases | `registry/aliases.go` | Model alias routing |

### WebSocket
| Module | Path | Description |
|--------|------|-------------|
| WS Relay | `wsrelay/manager.go` | WebSocket management |
| WS Session | `wsrelay/session.go` | Session handling |

### Usage/Metrics
| Module | Path | Description |
|--------|------|-------------|
| Metrics | `usage/metrics.go` | Usage tracking |
| Privacy ZDR | `usage/privacy_zdr.go` | Privacy filtering |

### TUI/UI
| Module | Path | Description |
|--------|------|-------------|
| Auth Tab | `tui/auth_tab.go` | Auth UI |
| Config Tab | `tui/config_tab.go` | Config UI |
| Keys Tab | `tui/keys_tab.go` | API keys UI |
| OAuth Tab | `tui/oauth_tab.go` | OAuth UI |

### Command Line
| Module | Path | Description |
|--------|------|-------------|
| Cursor Login | `cmd/cursor_login.go` | Cursor auth |
| MiniMax Login | `cmd/minimax_login.go` | MiniMax auth |
| Generic API Key | `cmd/generic_apikey_login.go` | Generic auth |
| Setup | `cmd/setup.go` | Initial setup |

### API Modules
| Module | Path | Description |
|--------|------|-------------|
| AMP (Sourcegraph) | `api/modules/amp/` | Sourcegraph AMP |
| Rankings | `api/handlers/management/rankings.go` | Provider rankings |
| Provider Status | `api/handlers/management/provider_status.go` | Status |

### Advanced Features
| Module | Path | Description |
|--------|------|-------------|
| Synthesizer Config | `auth/synthesizer/` | Config synthesis |
| Diff Engine | `auth/diff/` | Config diffing |
| Watcher Synth | `watcher/synthesizer/` | Watch-based synthesis |
| Watcher Diff | `watcher/diff/` | Watch-based diff |

---

## PART 3: WORKTREES (In Progress)

### cliproxy-wtress (Migration Worktrees)
```
lane-7-process
migrated-chore-cliproxyctl-minimal2
migrated-chore-cpb-wave-c7-next-pr2
migrated-chore-merge-branches
canonical/main
... 70+ more
```

### cliproxyapi-plusplus-wtrees
```
merge-fix-20260227
upstream-feature-replay
pr585-fix
... 30+ more
```

---

## PART 4: RECOMMENDED RESTRUCTURE

### Phase 1: Clean Base Branch
Create `fork/main` from upstream + minimal import path changes only.

### Phase 2: Modular Feature Worktrees
| Worktree | Features |
|----------|----------|
| `feat/providers-minimax` | MiniMax/iflow executor, login, thinking |
| `feat/providers-kiro` | Kiro executor, OAuth, translation |
| `feat/providers-kimi` | Kimi executor, tokens |
| `feat/providers-cursor` | Cursor storage, login |
| `feat/routing-pareto` | Pareto router |
| `feat/routing-aliases` | Model aliases |
| `feat/amp-sourcegraph` | Sourcegraph AMP module |
| `feat/synth-config` | Synthesizer, Diff engine |
| `feat/sticky-sessions` | X-Session-Key, sticky routing |
| `feat/usage-metrics` | Metrics, privacy filtering |

### Phase 3: Merge Strategy
Each feature worktree merges cleanly into `fork/main` without import path chaos.

---

## PART 5: BUG FIXES TO PORT

| Issue | Fix | Status |
|-------|-----|--------|
| X-Session-Key forwarding | Added to handlers.go | ✅ DONE |
| Sticky round-robin | In upstream, check fork | ⚠️ NEEDS PORT |
| Antigravity backfill | In upstream, check fork | ⚠️ NEEDS PORT |
| Codex /v1/responses route | In upstream, check fork | ⚠️ NEEDS PORT |

---

*Last Updated: 2026-02-27*
