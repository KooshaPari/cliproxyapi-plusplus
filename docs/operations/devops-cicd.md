# DevOps and CI/CD

This repository uses a shared Phenotype DevOps helper surface for checks and push fallback behavior.

## Local Delivery Helpers

Run these repository-root commands:

- `task devops:status`
  - Show branch, remote, and status for fast handoff
- `task devops:check`
  - Run shared preflight checks and local quality probes
- `task devops:check:ci`
  - Include CI-oriented checks and policies
- `task devops:check:ci-summary`
  - Same as CI check but emit a machine-readable summary
- `task devops:push`
  - Push with primary remote-first then fallback on failure
- `task devops:push:origin`
  - Push to fallback remote only (primary skipped)

## Cross-Project Reuse and Pattern

These helpers are part of a shared pattern used by sibling repositories:

- `thegent`
  - Uses `scripts/push-thegent-with-fallback.sh`
  - `Taskfile.yml` task group: `devops:*`
- `portage`
  - Uses `scripts/push-portage-with-fallback.sh`
  - `Taskfile.yml` task group: `devops:*`
- `heliosCLI`
  - Uses `scripts/push-helioscli-with-fallback.sh`
  - `justfile` task group: `devops-*`

The concrete implementation is centralized in `../agent-devops-setups` and reused via env overrides:

- `PHENOTYPE_DEVOPS_REPO_ROOT`
- `PHENOTYPE_DEVOPS_PUSH_HELPER`
- `PHENOTYPE_DEVOPS_CHECKER_HELPER`

## Fallback policy in practice

`repo-push-fallback.sh` prefers the project’s configured push remote first. If push fails because
branch divergence or transient network issues, it falls back to the Airlock-style remote.
Use `task devops:push` in normal operations and `task devops:push:origin` for forced fallback testing.
