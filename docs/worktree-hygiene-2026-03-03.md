# Worktree Hygiene Sweep (2026-03-03)

## Scope
- /Users/kooshapari/CodeProjects/Phenotype/repos/cliproxyapi++
- /Users/kooshapari/CodeProjects/Phenotype/repos/cliproxyapi-plusplus

## Canonical Safety
- `cliproxyapi++` canonical checkout is dirty and was not modified.
- `cliproxyapi-plusplus` canonical checkout is dirty and was not modified.
- All execution performed in dedicated worktree lanes.

## Active Lanes (High-Signal)
- `cliproxyapi++/PROJECT-wtrees/release-prep-fmt-20260303` -> `codex/release-prep-fmt-20260303` (fmt unblock lane)
- `cliproxyapi-plusplus-wtrees/blocker-triage-20260303` -> `chore/blocker-triage-canonical-dirty-20260303` (PR #839)
- `cliproxyapi-plusplus/PROJECT-wtrees/main-sync-20260303` -> `codex/main-sync-20260303` (safe sync/rebase lane)

## Sync/Rebase Status
- `origin/main` and `upstream/main` now resolve to the same commit: `c9d5e112`.
- No remaining rebase delta to apply in the clean sync lane.
- Safety backup branch created in canonical repo: `backup/main-pre-rebase-20260303-6d974368`.

## Hygiene Findings
- Large historical worktree footprint remains under `cliproxy-wtress/*` (migration era lanes).
- Multiple detached-head worktrees detected:
  - `cliproxy-wtress/migrated-ci-fix-feature-koosh-migrate-1672-fix-responses-json-corruption`
  - `cliproxy-wtress/pr-553`
  - `cliproxy-wtress/pr-554`
- Ambiguous remote-tracking config detected when targeting `origin/main` in one lane (`upstream_pre_airlock2` overlap).

## Cleanup Candidates (No Action Taken)
- Prune stale merged `tmp-pr-*` / `migrated-*` branches after verification.
- Resolve detached-head worktrees to named branches or remove them.
- Normalize remote fetch refspecs to prevent `origin/main` ambiguity.

## Residual Blockers
- `Taskfile.yml` parse error blocks task-based validation in blocker-triage lane:
  - `yaml: line 359: could not find expected ':'`
- `release:prep` in `cliproxyapi++` still blocked by missing report file:
  - `docs/reports/fragemented/OPEN_ITEMS_VALIDATION_2026-02-22.md`
