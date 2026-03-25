# GitHub ownership guard

Use this guard before any scripted GitHub mutation (issue/PR/comment operations):

```bash
./scripts/github-owned-guard.sh owner/repo
```

It returns non-zero for non-owned repos:

- allowed: `KooshaPari`
- allowed: `atoms-tech`

Example for a source URL:

```bash
./scripts/github-owned-guard.sh https://github.com/router-for-me/CLIProxyAPI/pull/1699
```

Example for current git origin:

```bash
./scripts/github-owned-guard.sh "$(git remote get-url origin | sed -E 's#https://github.com/##; s#git@github.com:##; s#\.git$##')"
```

If the command exits with code `2`, block the action and block the create/comment path in your workflow.
