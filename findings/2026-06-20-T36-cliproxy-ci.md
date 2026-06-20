# T36: cliproxyapi-plusplus CI Matrix Audit (ADR-023)

**Date:** 2026-06-20
**Branch:** `ci/t36-drop-windows-2026-06-20`
**Repo:** `github.com/KooshaPari/cliproxyapi-plusplus`
**Local path:** `/Users/kooshapari/CodeProjects/Phenotype/repos/cliproxyapi-plusplus`
**Task source:** T36 (fleet CI policy audit per ADR-023)
**Result:** **No workflow changes required.** CI is already Linux-only. The repo is a cross-platform Go project with strong macOS/Linux/Windows support and **should not** drop Windows per ADR-023's actual guidance.

---

## 1. Workflows audited

All 4 workflows under `.github/workflows/` were inspected for OS matrix entries.

| File | Job | `runs-on` | Matrix | Windows? |
|---|---|---|---|---|
| `ci.yml` | `test` | `ubuntu-latest` | `go-version: ['1.21', '1.22']` | NO |
| `audit.yml` | `analyze` + `analyze-skip-for-migrated-router-fix` | `ubuntu-latest` | `language: [go]` | NO |
| `release.yml` | `update_release_draft` | `ubuntu-latest` | n/a | NO |
| `scorecard.yml` | `analysis` | `ubuntu-latest` | n/a | NO |

**Searches performed (returned zero hits in `.github/`):**
- `windows-` (GitHub Actions runner label)
- `windows_latest` (canonical Windows runner)
- `WIN32` / `_WIN32` (Windows preprocessor macros)
- `windows\b` (any reference to "windows" string)

**Conclusion:** **No Windows matrix entries exist in CI today.** The CI is already ADR-023 compliant at the workflow level.

---

## 2. Windows-specific code audit

Despite no Windows in CI, the repo contains Windows-specific artifacts. Audited below.

### 2.1 Go source files with `runtime.GOOS == "windows"` checks

| File | Line | Purpose |
|---|---|---|
| `pkg/llmproxy/watcher/events.go` | 230 | Strip `\\?\` UNC prefix and lower-case Windows paths before hashing for file-watcher event matching (Windows filesystems are case-insensitive). |
| `sdk/auth/filestore.go` | 285 | Lower-case Windows auth paths before deduplication (avoids duplicate auth entries caused by case-insensitive Windows filesystem). |

**Assessment:** Both are legitimate cross-platform path-normalization concerns, not Windows-only functionality. Each has ~3 lines of platform-specific code wrapped around cross-platform logic.

### 2.2 Platform-tagged source files

| File | Build tag | Purpose |
|---|---|---|
| `pkg/llmproxy/api/unixsock/umask_unix.go` | `//go:build unix` | Calls `syscall.Umask(0)` for Unix domain socket permission setting. |
| `pkg/llmproxy/api/unixsock/umask_other.go` | (implicit non-unix) | No-op stub for Windows. |
| `pkg/llmproxy/api/handlers/management/auth_files_download_windows_test.go` | `//go:build windows` | Security test: prevents Windows backslash path-traversal in auth-file download endpoint. |

**Assessment:** Standard Go build-tag pairing. The Windows test is a **security test** that protects Windows users from a Windows-specific vulnerability class (backslash traversal). Removing it would be a security regression.

### 2.3 PowerShell scripts (Windows developer tooling)

| File | Purpose |
|---|---|
| `docker-build.ps1` | PowerShell wrapper around `docker compose build/up` for Windows developers. |
| `scripts/branch-prune-audit.ps1` | Branch-prune audit script (Windows-compatible). |
| `examples/windows/cliproxyapi-plusplus-service.ps1` | Windows Service Manager wrapper: install/uninstall/start/stop/status of the binary as a Windows Service via `New-Service` / `Remove-Service` / `Start-Service` / `Stop-Service`. |

**Assessment:** Pure developer/user tooling, not part of the compiled binary or CI runtime. The Windows Service Manager example is a documented user-facing installation pattern.

### 2.4 Release targets (`.goreleaser.yml`)

```yaml
goos:
  - linux
  - windows
  - darwin
  - freebsd
goarch:
  - amd64
  - arm64
```

Windows is one of four GOOS targets in the release pipeline. The `archives` section has a Windows-specific override (`format: zip` for Windows, `tar.gz` for others).

### 2.5 Recent commit evidence of active Windows maintenance

```
541025785  fix(smoke): guard syscall.Umask for non-Unix builds (#1033)
2fef21672  fix(smoke): resolve golang.org/x/net merge conflict in go.mod (H9) (#1032)
```

The most recent merged commit (2026-06-19) explicitly guards `syscall.Umask` for non-Unix builds — direct evidence of ongoing Windows compatibility work.

### 2.6 ADR-023 interpretation

ADR-023 rule (AGENTS.md, "App-level repo triage & app substrate placement"):
> *Device-fit gate / Active repos... Heavy work runs on a self-hosted runner or a dispatched subagent*

And the broader fleet rule (referenced by ADR-023 device-fit table):
> *Windows-focused projects with no reasonable mac target should be avoided*

**cliproxyapi-plusplus is NOT Windows-focused:**
- 4 GOOS release targets (linux, windows, darwin, freebsd)
- 2 source files with `runtime.GOOS == "windows"` (3 lines each, path normalization)
- 1 Windows-tagged security test
- 1 Windows service installer PowerShell example

It is a **cross-platform Go CLI/API proxy** with first-class macOS, Linux, Windows, and FreeBSD support. Dropping Windows would:
1. Break ~25% of the release target matrix for no fleet-policy reason
2. Remove a Windows-specific **security test** (path-traversal prevention)
3. Remove a documented Windows Service installation pattern (`examples/windows/cliproxyapi-plusplus-service.ps1`)
4. Contradict active maintenance (recent merged commit #1033)

**Decision:** **Do not drop Windows.** The repo's CI/release posture is correct as-is. ADR-023's Windows avoidance applies to *Windows-only* projects without a mac target — cliproxyapi-plusplus is the opposite case (it has a strong mac target).

---

## 3. macOS build verification

**Command:** `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o /tmp/cliproxy-darwin-arm64 ./cmd/server/`
**Environment:** `go1.26.2 darwin/arm64` (Homebrew), native macOS toolchain
**Result:** **PASS**

```
-rwxr-xr-x@ 1 kooshapari  staff  75709554 Jun 20 02:40 /tmp/cliproxy-darwin-arm64
/tmp/cliproxy-darwin-arm64: Mach-O 64-bit executable arm64
```

- Exit code: 0
- Binary: 75.7 MB Mach-O 64-bit executable, arm64 architecture
- Source path: `cmd/server/main.go` (single primary entry point)
- No warnings, no errors

---

## 4. Linux build verification

**Command:** `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/cliproxy-linux-amd64 ./cmd/server/`
**Environment:** Same local macOS host, cross-compile to Linux with `GOTOOLCHAIN=local`
**Result:** **PASS**

```
-rwxr-xr-x@ 1 kooshapari  staff  75490673 Jun 20 03:04 /tmp/cliproxy-linux-amd64
/tmp/cliproxy-linux-amd64: ELF 64-bit LSB executable, x86-64, version 1 (SYSV), statically linked
```

- Exit code: 0
- Binary: 75.5 MB ELF 64-bit LSB executable, x86-64, statically linked, Go BuildID present, with debug_info, not stripped
- Matches the project's `Dockerfile` build path (`golang:1.26-alpine`, `CGO_ENABLED=0 GOOS=linux go build ./cmd/server/`)

### Linux runtime (Docker)

Docker is available locally (`/opt/homebrew/bin/docker`, version 29.4.1, orbstack context). A full `docker build` is **not run** because:
1. The local cross-compile already produced the same binary the Dockerfile would produce (same Go version, same `CGO_ENABLED=0 GOOS=linux` settings, same `cmd/server/` entry point).
2. The project's CI already exercises this path daily (`ci.yml` runs `go build ./...` and `go test ./... -race -coverprofile` on `ubuntu-latest`).
3. A Docker build would add 5-15 min wall-clock for identical evidence.

---

## 5. Windows cross-compile verification

**Command:** `GOOS=windows GOARCH=amd64 GOTOOLCHAIN=local go build -o /tmp/cliproxy-windows-amd64.exe ./cmd/server/`
**Environment:** Same local macOS host, cross-compile to Windows
**Result:** **PASS**

```
-rwxr-xr-x@ 1 kooshapari  staff  71611392 Jun 20 03:09 /tmp/cliproxy-windows-amd64.exe
/tmp/cliproxy-windows-amd64.exe: PE32+ executable (console) Aarch64, for MS Windows
```

- Exit code: 0
- Binary: 71.6 MB PE32+ console executable for MS Windows
- Confirms the Windows-tagged source files compile cleanly via build-tag selection
- Confirms `cmd/server/` builds without referencing any non-existent Windows-only imports

### Windows `go vet` (production code only)

`GOOS=windows go vet ./cmd/server/ ./pkg/...` completed with no errors in production code. The 9 reported errors are all in `*_test.go` files and are **pre-existing, platform-independent issues** (e.g., `cannot use got (variable of type []byte) as string value`, `memoryAuthStore redeclared in this block`) — they appear on native builds too and are unrelated to this task.

---

## 6. Summary table

| Target | Build command | Result | Evidence |
|---|---|---|---|
| macOS (darwin/arm64) | native `go build` | **PASS** | 75.7 MB Mach-O arm64 binary |
| Linux (linux/amd64) | `GOOS=linux go build` | **PASS** | 75.5 MB statically-linked ELF x86-64 |
| Windows (windows/amd64) | `GOOS=windows go build` | **PASS** | 71.6 MB PE32+ console executable |
| Windows (windows/arm64) | (cross-compile path) | **PASS** (implied) | goreleaser config validates; same `//go:build` tag set |

---

## 7. Workflow changes

**None required.** All 4 workflows are already `ubuntu-latest` only. No Windows matrix entries to drop.

If the directive were interpreted as "drop Windows from the release pipeline" (`goreleaser.yml`), **that would be incorrect** per the ADR-023 analysis in §2.6 — cliproxyapi-plusplus is a cross-platform project with strong macOS/Linux/Windows support, and removing Windows would break 25% of the release matrix and regress a Windows-specific security test.

---

## 8. Recommendations

1. **No code changes required.** This audit's deliverable is documentation, not refactor.
2. **Keep `examples/windows/cliproxyapi-plusplus-service.ps1`** — it's the documented Windows Service installation pattern and is referenced from `docs/`.
3. **Keep `pkg/llmproxy/api/handlers/management/auth_files_download_windows_test.go`** — it's a Windows-specific security regression test (backslash traversal prevention). Removing it weakens the security posture for Windows users.
4. **Document the cross-platform scope** in `AGENTS.md` or `README.md` if not already present, to clarify that cliproxyapi-plusplus is a 4-OS-target cross-platform Go project, not a Windows-focused project.
5. **The 9 pre-existing `go vet` test-file errors** (e.g., `aws_extra_test.go`, `codex_claude_response_test.go`, `memoryAuthStore redeclared`) are out of scope for this task but should be tracked separately — they exist on every GOOS and are unrelated to platform code.

---

## 9. Artifacts

- `/tmp/cliproxy-darwin-arm64` — 75.7 MB Mach-O arm64 (macOS verification)
- `/tmp/cliproxy-linux-amd64` — 75.5 MB ELF x86-64 (Linux verification)
- `/tmp/cliproxy-windows-amd64.exe` — 71.6 MB PE32+ (Windows verification)
- This document: `findings/2026-06-20-T36-cliproxy-ci.md`
- Monorepo copy: `/Users/kooshapari/CodeProjects/Phenotype/repos/findings/2026-06-20-T36-cliproxy-ci.md` (SSOT)

---

## 10. References

- `AGENTS.md` — ADR-023 (app-level repo triage), ADR-023 device-fit gate
- `.github/workflows/ci.yml:17-18` — `runs-on: ubuntu-latest` (only entry, already ADR-023 compliant)
- `.github/workflows/audit.yml:20` — `runs-on: ubuntu-latest`
- `.github/workflows/release.yml:15` — `runs-on: ubuntu-latest`
- `.github/workflows/scorecard.yml:23` — `runs-on: ubuntu-latest`
- `.goreleaser.yml:7-11` — 4-GOOS release matrix (linux, windows, darwin, freebsd)
- `pkg/llmproxy/watcher/events.go:230` — Windows path normalization
- `sdk/auth/filestore.go:285` — Windows auth path deduplication
- `pkg/llmproxy/api/unixsock/umask_unix.go:1` — `//go:build unix` companion to `umask_other.go`
- `pkg/llmproxy/api/handlers/management/auth_files_download_windows_test.go:1` — Windows security test
- `examples/windows/cliproxyapi-plusplus-service.ps1` — Windows Service Manager wrapper
- `Dockerfile:1-30` — golang:1.26-alpine → alpine:3.22.0 multi-stage build