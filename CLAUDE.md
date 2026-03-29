# cliproxyapi-plusplus — Project Instructions

## Governance

This project follows Phenotype governance standards.

## Branch Discipline

- Feature branches: `repos/worktrees/<project>/<category>/<branch>`
- Canonical repository tracks `main` only
- Return to `main` for merge/integration checkpoints

## Work Requirements

1. Check for AgilePlus spec before implementing
2. Create spec for new work: `agileplus specify --title "<feature>" --description "<desc>"`
3. Update work package status: `agileplus status <feature-id> --wp <wp-id> --state <state>`
4. No code without corresponding AgilePlus spec

## Release Channels

- `releases/alpha` — Development releases
- `releases/beta` — Pre-production releases
- `releases/stable` — Production-ready releases

## Quality Gates

From project root:
- `go vet ./...` — Run Go vet
- `go test ./...` — Run test suite
- `task quality` — Full quality check (linter, tests, coverage)

## Reference

- AgilePlus: `/Users/kooshapari/CodeProjects/Phenotype/repos/AgilePlus`
- Global instructions: `~/.claude/CLAUDE.md`
- Phenotype workspace: `/Users/kooshapari/CodeProjects/Phenotype/CLAUDE.md`
