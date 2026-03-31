# Kilo Gastown Methodology Specification

**Rig:** `1f1669fc-c16a-40de-869c-107f631a9935`  
**Town:** `78a8d430-a206-4a25-96c0-5cd9f5caf984`  
**Repository:** cliproxyapi++ (LLM Proxy with Multi-Provider Support)

---

## Overview

Kilo Gastown is an agent orchestration methodology that coordinates distributed AI agents across a rig to accomplish complex, multi-repo software engineering tasks. This document explains how Kilo mechanics apply to the cliproxyapi++ codebase.

---

## Core Concepts

### Convoys

A **convoy** is a grouping mechanism for related work across repos. Convoys enable parallel feature development while maintaining semantic relationships between beads (work items).

```
Convoy: "AgilePlus + Kilo Specs: cliproxyapi++"
├── Bead: Add Kilo Gastown methodology spec (this work)
├── Bead: Add methodology artifacts to thegent
├── Bead: Add methodology artifacts to agentapi++
└── ...
```

**Characteristics:**
- Convoys have a `feature_branch` metadata field for the shared branch name
- All repo worktrees join the same convoy branch
- Progress tracked via `gt_list_convoys`

### Beads

**Beads** are the fundamental work unit in Kilo Gastown. Each bead represents a discrete task assigned to an agent.

| Field | Purpose |
|-------|---------|
| `bead_id` | Unique identifier (UUID) |
| `type` | `issue`, `convoy`, `task`, `triage` |
| `status` | `open`, `in_progress`, `in_review`, `closed` |
| `assignee_agent_bead_id` | Which polecat is working this bead |
| `parent_bead_id` | Hierarchical grouping |
| `metadata` | Key-value pairs (convoy_id, feature_branch, etc.) |

**Bead Lifecycle:**
```
open → in_progress → in_review → closed
         ↑_____________↓ (rework loop)
```

### Polecats

**Polecats** are the working agents in a rig. Each polecat:
- Has a unique identity (e.g., `Polecat-27-polecat-1f1669fc@78a8d430`)
- Is assigned one or more beads via `current_hook_bead_id`
- Operates within a worktree (`.worktrees/` directory)
- Calls `gt_done` when a bead transitions to `in_review`

### Rigs

A **rig** is a coordinated group of agents working together on shared objectives:
- Rig ID: `1f1669fc-c16a-40de-869c-107f631a9935`
- Contains multiple polecats and towns
- Manages convoy lifecycle and agent dispatch

### Towns

A **town** is a logical subdivision within a rig:
- Town ID: `78a8d430-a206-4a25-96c0-5cd9f5caf984`
- Provides namespace isolation for agents and beads

---

## Delegation Mechanisms

### gt_sling / gt_sling_batch

Used to delegate work to other agents:

- `gt_sling`: Assigns a single bead to another agent
- `gt_sling_batch`: Assigns multiple beads in one operation

**Usage in cliproxyapi++:**
```bash
# Delegate a bead to another polecat
gt_sling --to-agent <agent_id> --bead <bead_id>
```

### gt_prime

Called at session start to retrieve:
- Agent identity and status
- Hooked (current) bead
- Undelivered mail
- All open beads in the rig

**Pattern:**
```bash
gt_prime  # Auto-injected on first message, refresh with explicit call
```

---

## Bead Coordination

### gt_bead_status

Inspect any bead's current state by ID:
```bash
gt_bead_status --bead-id <bead_id>
```

### gt_bead_close

Mark a bead as completed (after all work is done and merged):
```bash
gt_bead_close --bead-id <bead_id>
```

### gt_list_convoys

Track convoy progress across repos. Shows:
- Open convoys with their feature branches
- Bead counts per convoy
- `ready_to_land` flag when all beads are in_review/closed

---

## Merge Modes

When a bead's work is ready to land:

| Mode | Description |
|------|-------------|
| `in_review` | Work submitted to review queue; refinery handles merge |
| `closed` | Work fully completed and merged |

**Important:** Agents do NOT merge directly. They push their branch and call `gt_done`, which transitions the bead to `in_review` and submits to the refinery queue.

---

## Agent Workflow for cliproxyapi++

### Starting Work

1. Receive bead assignment (hooked via `current_hook_bead_id`)
2. Call `gt_prime` if needing context refresh
3. Review bead requirements
4. Create/checkout appropriate worktree

### During Work

1. Implement the feature or fix
2. Run quality gates: `task quality`
3. Commit frequently with descriptive messages
4. Push after each commit (worktree disk is ephemeral)

### Completing Work

1. Verify all pre-submission gates pass
2. Push branch
3. Call `gt_done --branch <branch_name>`
4. Bead transitions to `in_review`

### Error Handling

- If stuck after multiple attempts: `gt_escalate` with problem description
- If blocked: use `gt_mail_send` to coordinate with other agents
- If container restarts: recover from last `gt_checkpoint`

---

## Gastown Tool Reference

| Tool | Purpose |
|------|---------|
| `gt_prime` | Get full context at session start |
| `gt_bead_status` | Inspect bead state |
| `gt_bead_close` | Close a completed bead |
| `gt_done` | Push branch and transition bead to in_review |
| `gt_mail_send` | Send message to another agent |
| `gt_mail_check` | Read pending mail |
| `gt_escalate` | Create escalation bead for blockers |
| `gt_checkpoint` | Write crash-recovery data |
| `gt_status` | Emit dashboard status update |
| `gt_nudge` | Send real-time nudge to agent |
| `gt_mol_current` | Get current molecule step |
| `gt_mol_advance` | Complete molecule step and advance |
| `gt_triage_resolve` | Resolve a triage request |

---

## cliproxyapi++ Integration

### Repository Role

cliproxyapi++ is the LLM proxy component in the Kush ecosystem:

```
kush/
├── thegent/         # Agent orchestration
├── agentapi++/      # HTTP API for coding agents
├── cliproxy++/      # LLM proxy with multi-provider support (this repo)
├── tokenledger/     # Token and cost tracking
└── ...
```

### Methodology Application

1. **Convoy Participation**: cliproxyapi++ joins convoys like "AgilePlus + Kilo Specs" to implement cross-repo features

2. **Worktree Discipline**: 
   - All feature work happens in `.worktrees/convoy__*-<bead_id>/`
   - Primary checkout remains on `main`

3. **Phenotype Governance**:
   - TDD + BDD + SDD for all feature changes
   - Hexagonal + Clean + SOLID architecture boundaries
   - Explicit failures over silent degradation

### Bot Review Governance

For PR reviews in this repo:

| Bot | Retrigger Command |
|-----|-------------------|
| CodeRabbit | `@coderabbitai full review` |
| Gemini Code Assist | `@gemini-code-assist review` (fallback: `/gemini review`) |

**Rate-limit contract:**
- 1 retrigger per bot per PR every 15 minutes
- Check for existing trigger markers before re-triggering
- Track with `bot-review-trigger: <bot> <iso8601-time> <reason>`

---

## Quality Gates

Before calling `gt_done`, run:

```bash
task quality
```

**cliproxyapi++ specific gates:**
- `go fmt` pass
- `go vet` pass  
- `golangci-lint` zero errors
- 80% test coverage (CI gate)
- Zero critical security issues (trivy scan)

---

## References

- AGENTS.md: Agent guidance for this repository
- Phenotype Governance Overlay v1: TDD/BDD/SDD requirements
- Kush Ecosystem: Multi-repo system overview
