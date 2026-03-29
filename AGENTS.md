# Project — AI Agent Instructions

This file guides Claude agents working on this project.

## Scope

- Full project ownership: implementation, testing, documentation, PR preparation
- Authority: create branches, commits, PRs, merge to main (via PR)

## Process

1. **Pre-work**: Check AgilePlus for specs (`agileplus list`)
2. **Implementation**: Work in feature branch at `repos/worktrees/<project>/<category>/<feature>`
3. **Testing**: Run quality checks locally before pushing
4. **PR prep**: Ensure all checks pass before requesting merge
5. **Integration**: Rebase/restack if needed, merge to main via PR

## Quality Standards

- All tests pass locally
- Lint: 0 errors
- Coverage: ≥80% (or project default)
- Documentation updated for new features

## Integration Points

- Coordinate with Phenotype shared packages
- Cross-repo updates tracked in AgilePlus

## Escalation

- Blocking issues: surface in AgilePlus worklog
- Design decisions: document in ADR files
- Architecture changes: reference governance framework
