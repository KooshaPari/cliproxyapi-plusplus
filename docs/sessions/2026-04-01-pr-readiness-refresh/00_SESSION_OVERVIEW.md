# PR Readiness Refresh

## Goal

Stabilize PR `#942` enough to move it out of branch-local merge debt and obvious CI wiring failures.

## Scope

- Resolve the lingering `docs/plans/KILO_GASTOWN_SPEC.md` merge residue in the checked-out branch.
- Replace deprecated or broken SAST workflow wiring with current pinned actions and direct tool invocation.
- Re-target custom Semgrep content away from Rust-only patterns so the ruleset matches this Go repository.

## Outcome

- The branch no longer carries an unmerged spec file.
- `SAST Quick Check` no longer references a missing action repo or a Rust-only lint job.
- Remaining blockers are pre-existing repo debt or external issues, not broken workflow scaffolding in this PR.
