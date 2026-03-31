# Kilo Gastown Methodology Specification for cliproxyapi++

**Rig ID:** `1f1669fc-c16a-40de-869c-107f631a9935`  
**Town:** `78a8d430-a206-4a25-96c0-5cd9f5caf984`

---

## Overview

Kilo Gastown is the agent orchestration methodology used in the Kush multi-repo ecosystem. This document explains how Kilo mechanics apply to `cliproxyapi++`, the LLM proxy layer with multi-provider support.

---

## Core Concepts

### Convoys

Convoys are logical grouping mechanisms for related work items (beads) that need to ship together across repos. A convoy ensures atomic delivery of coordinated changes.

**In cliproxyapi++:**
- Convoys coordinate multi-repo changes such as API contract updates, provider additions, or shared protocol changes
- Each convoy has a `feature_branch` metadata field tracking the coordinated branch across repos
- Convoys are tracked via `gt_list_convoys` for progress visibility

**Convoy lifecycle:**
```
open → in_progress → (ready_to_land) → merged
```

| Status | Meaning |
|--------|---------|
| `open` | Work not yet started |
| `in_progress` | Work underway |
| `ready_to_land` | CI gates passed, awaiting merge |
| `merged` | Changes landed on target branch |

### Beads

Beads are the atomic work items in the Kilo system. Each bead represents a unit of work that can be assigned to an agent.

**Bead types:**
- `issue` — Feature, bug fix, or task
- `convoy` — Coordinator bead for multi-repo work
- `triage` — Routing or escalation request

**In cliproxyapi++:**
- Issue beads track individual features (e.g., "Add Anthropic streaming support")
- Each bead has a `bead_id`, `status`, `priority`, and optional `parent_bead_id`
- Beads are assigned via `assignee_agent_bead_id`

**Bead lifecycle:**
```
open → in_progress → in_review → closed
```

| Status | Meaning |
|--------|---------|
| `open` | Queued, not yet started |
| `in_progress` | Agent is working on it |
| `in_review` | Submitted for review/merge |
| `closed` | Completed or rejected |

### Delegation: gt_sling and gt_sling_batch

**gt_sling** — Delegates a single bead to another agent.

**gt_sling_batch** — Delegates multiple beads to another agent in a single operation.

**In cliproxyapi++:**
- Used by orchestrating agents (e.g., TownDO or lead agents) to route work to specialized polecat agents
- Example: A "provider expansion" bead gets slung to an agent with relevant provider expertise
- Batch sling used when multiple related beads (e.g., provider + tests + docs) go to the same agent

### Merge Modes

Kilo supports different merge strategies for integrating bead work:

| Mode | Description |
|------|-------------|
| `squash` | All commits squashed into one (clean history) |
| `rebase` | Commits replayed on target (linear history) |
| `merge` | Full commit history preserved |

**In cliproxyapi++:**
- Default: `squash` for feature branches (clean main history)
- Exception: `rebase` for hotfixes requiring full audit trail
- Merge mode determined at convoy creation based on change type

### gt_list_convoys

The `gt_list_convoys` command provides progress visibility across all active convoys in the rig.

**Output includes:**
- Convoy ID and title
- Status (open, in_progress, ready_to_land)
- Child beads and their statuses
- Feature branch name

**In cliproxyapi++:**
- Use `gt_list_convoys` to track cross-cutting initiatives like "Add AWS Bedrock support" which may touch provider adapters, auth handlers, and routing logic simultaneously

---

## Agent Roles in cliproxyapi++

| Role | Function | Tools |
|------|----------|-------|
| **TownDO** | Orchestrator; creates and assigns beads, manages convoys | gt_prime, gt_sling, gt_sling_batch |
| **Polecat** | Worker agent; implements beads assigned to it | gt_done, gt_bead_close, gt_checkpoint |
| **Refinery** | Merge gate; validates and lands approved changes | gt_list_convoys, gt_bead_status |

### Polecat Workflow (cliproxyapi++)

1. **Prime** — Call `gt_prime` to get hooked bead and context
2. **Work** — Implement the bead requirement
3. **Checkpoint** — Call `gt_checkpoint` after significant milestones
4. **Verify** — Run lint/typecheck/tests before submission
5. **Done** — Push branch, call `gt_done` to submit for review

---

## Applying Kilo to cliproxyapi++ Development

### Feature Development Flow

```
TownDO creates bead
    ↓
Bead hooked to Polecat
    ↓
Polecat implements on feature branch
    ↓
Push + gt_done → in_review
    ↓
Refinery validates
    ↓
Merge to main
```

### Multi-Repo Coordinated Changes

For changes affecting multiple Kush repos (e.g., adding a new provider that also requires SDK updates):

```
TownDO creates convoy bead
    ↓
Child beads created for each repo (cliproxyapi++, thegent, agentapi++, etc.)
    ↓
All child beads slung to respective polecats
    ↓
Each polecat works independently on their feature branch
    ↓
All beads reach ready_to_land
    ↓
Refinery merges convoy atomically
```

### Bot Review Retrigger Governance

When requesting bot reviews (CodeRabbit, Gemini Code Assist):

1. Check latest PR comments for existing trigger markers
2. If rate-limited, queue retry for 15+ minutes later
3. After two consecutive rate-limit responses, stop auto-retries and post status
4. Required marker format: `bot-review-trigger: <bot> <iso8601-time> <reason>`

---

## Quality Gates

Before calling `gt_done`, polecats must verify:

| Gate | Command | Threshold |
|------|---------|-----------|
| Tests | `go test ./...` | 80% coverage |
| Lint | `golangci-lint run` | 0 errors |
| Vet | `go vet ./...` | 0 errors |
| Format | `go fmt ./...` | No diff |

---

## Related Documentation

- [cliproxyapi++ SPEC.md](./SPEC.md) — Technical architecture
- [cliproxyapi++ FEATURE_CHANGES_PLUSPLUS.md](./FEATURE_CHANGES_PLUSPLUS.md) — ++ vs baseline changes
- [Kush multi-repo system overview](../AGENTS.md)

---

**Document version:** 1.0  
**Last updated:** 2026-03-31
