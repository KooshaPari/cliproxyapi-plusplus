# AgilePlus Methodology Specification

## Overview

AgilePlus is the development methodology employed by the Kush ecosystem for building and operating cliproxyapi++, a multi-provider LLM proxy service. It combines agile iterative delivery with structured governance, spec-first planning, and architectural discipline.

---

## Core Principles

### 1. Spec-First Development

Every feature begins with a specification document before any code is written.

- Requirements are captured in `docs/plans/` as markdown specs
- API contracts are defined before implementation
- Data models are specified before database changes
- Breaking changes require spec revision and approval

### 2. Source-to-Solution Traceability

All work items must be traceable from source request to implemented solution.

| Field | Purpose |
|-------|---------|
| `Board ID` | Unique identifier (e.g., `CP2K-0418`) |
| `Source Kind` | GitHub issue, PR, external ticket, etc. |
| `Source URL` | Link to original request |
| `Status` | `proposed` → `in_progress` → `blocked` → `done` |

### 3. Wave-Based Execution

Work is organized into waves for coordinated delivery:

- **Wave 1**: Core infrastructure and breaking changes
- **Wave 2**: Feature development and API additions  
- **Wave 3**: Polish, optimization, and documentation

### 4. Quality Gates

| Gate | Tool | Threshold |
|------|------|-----------|
| Tests | Go test | 80% coverage |
| Lint | golangci-lint | 0 errors |
| Security | trivy | 0 critical |
| Format | go fmt, go vet | Pass |

---

## Workflow Lifecycle

```
Source Request → Board Item → Spec → Implementation → PR → Review → Merge → Done
```

### 1. Planning Phase

1. Source requests are ingested from GitHub Issues/PRs/Discussions
2. Items are mapped to the execution board with required fields
3. Strategic items (architecture, DX, ops) are added proactively
4. Waves are assigned based on priority and dependencies

### 2. Implementation Phase

1. Agent picks up item from board, sets status to `in_progress`
2. Specification is written or updated in `docs/plans/`
3. Code follows TDD + BDD + SDD cycle
4. Architecture respects Hexagonal + Clean + SOLID boundaries

### 3. Review Phase

1. PR is opened with links to board item and spec
2. Bot review is requested (CodeRabbit or Gemini Code Assist)
3. Rate-limit governance: one retrigger per bot per 15 minutes
4. Review feedback is addressed in same branch

### 4. Completion Phase

1. PR merged, status set to `done`
2. Board item updated with:
   - PR URL
   - Merged commit SHA
   - Released version (if applicable)
   - Docs page updated (if applicable)

---

## Architecture Constraints

### Hexagonal + Clean Architecture

```
Client → Handler → Service → Repository → Provider
```

- Business logic lives in core domain, isolated from infrastructure
- Ports and adapters define interface boundaries
- Dependencies point inward, never outward

### Explicit Failure Mode

- Required dependencies must fail clearly when unavailable
- Silent degradation is forbidden
- Error messages must be actionable

### Low-Latency Local Paths

- Hot paths (request routing, auth) are deterministic and low-latency
- Distributed workflow logic is placed behind durable orchestration boundaries
- Correlation IDs enable distributed tracing

---

## Governance Overlay

The Phenotype Governance Overlay v1 enforces additional constraints:

| Requirement | Enforcement |
|-------------|-------------|
| TDD + BDD + SDD | All feature/workflow changes |
| Hexagonal + Clean + SOLID | Architecture review |
| Policy gating | Agent and workflow actions |
| Auditability | All actions have traceable IDs |
| Explicit failures | No silent degradation |

---

## File Organization

```
docs/plans/              # Spec documents for features
docs/planning/           # Execution boards and wave tracking
.github/workflows/       # CI/CD pipelines
cmd/                     # Executable entry points
pkg/                     # Core packages (hexagonal boundaries)
```

---

## Bot Review Retrigger Governance

When requesting bot reviews:

1. Check latest PR comments for existing trigger markers
2. Wait 15 minutes between retriggers for same bot
3. If rate-limited, queue retry for 15 minutes or bot-provided time
4. After two consecutive rate-limits, stop auto-retries

Required tracking marker in PR comments:
```
bot-review-trigger: <bot> <iso8601-time> <reason>
```

---

## References

- [Board Workflow](./planning/board-workflow.md)
- [Execution Board](./planning/CLIPROXYAPI_2000_ITEM_EXECUTION_BOARD_2026-02-22.md)
- [Phenotype Governance Overlay](../AGENTS.md#phenotype-governance-overlay-v1)
- [Kush Ecosystem](../AGENTS.md#kush-ecosystem)

---

*Last updated: 2026-03-31*
