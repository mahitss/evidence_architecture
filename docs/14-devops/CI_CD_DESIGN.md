# CI/CD Design Specification

- **Document Path:** `docs/14-devops/CI_CD_DESIGN.md`
- **Phase:** Phase 09 — Foundation & Implementation (Follow-up `FU1-CI` closure)
- **Traceability:** `ADR-003` (local processes, no Docker), `ADR-004` (Go PATH precedence), `ADR-010` (Go modular monolith), `ADR-011` (Redis Streams), `ADR-017` (OCR strategy), `STATUS_LEDGER.md`
- **Status:** Active Specification (basis for future `.github/workflows/ci.yml`)

---

## 1. Purpose

This document defines the pinned toolchains, lint gates, test jobs, and environment parity checks that will be codified in `.github/workflows/ci.yml` once GitHub Actions CI is scaffolded.

---

## 2. Pinned Toolchains & Linters

| Component | Tool / Package | Pinned Version | Notes / Verification Requirement |
|---|---|---|---|
| **Go Toolchain** | `go` | `1.27.4` | Must precede any system `/usr/bin/go` via explicit `PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"` (`ADR-004`). |
| **Go Static Analysis (`FU1-CI`)** | `honnef.co/go/tools/cmd/staticcheck` | **`@v0.8.2`** (`staticcheck 2026.2.2`) | **Pin rationale:** `honnef.co/go/tools/cmd/staticcheck@2024.1.1` (`v0.5.1`) fails to compile under Go `1.27.4` due to an incompatibility in its pinned `golang.org/x/tools@v0.21.1` dependency (`internal/tokeninternal/tokeninternal.go:64:9: invalid array length -delta * delta`). **`@v0.8.2` compiles and runs cleanly on Go `1.27.4` and MUST be re-verified whenever Go is upgraded.** |
| **DB Migrations** | `github.com/golang-migrate/migrate/v4/cmd/migrate` | `@v4.20.1` (`-tags 'postgres'`) | Installed via `scripts/install-migrate.sh` (`ADR-016`). |
| **Rust Toolchain** | `rustc` / `cargo` / `clippy` / `rustfmt` | `1.100.0` | Enforced via `cargo fmt --check` and `cargo clippy -- -D warnings`. |
| **OCR Engine (CI Linux Runner)** | `tesseract-ocr` (`tesseract` CLI) | Distro package (`apt-get install -y tesseract-ocr`) | Installed in CI runner to verify live OCR test paths (`ADR-017`). Graceful degradation when absent is also unit-tested. |
| **OpenAPI Linter** | `@redocly/cli` | Latest compatible via `npx --yes @redocly/cli` | Must pass `lint docs/09-api/openapi.yaml` with `0` errors and `0` warnings. |
| **Backing Services (CI)** | PostgreSQL `15.x`, Redis `7.x` | `15.20` / `7.0.15` (or runner service containers / local processes) | First CI run MUST verify `Makefile` non-root `run_as()` Branch 2 (`ADR-003` open item). |

---

## 3. Planned `.github/workflows/ci.yml` Job Matrix

### Job 1: `contracts-and-docs`
1. `npx --yes @redocly/cli lint docs/09-api/openapi.yaml` (0 errors, 0 warnings).
2. Verify no forbidden secret patterns are committed.

### Job 2: `backend-go`
1. Configure Go `1.27.4` and verify `which go` precedes `/usr/bin/go` (`ADR-004`).
2. Install pinned `staticcheck`:
   ```bash
   go install honnef.co/go/tools/cmd/staticcheck@v0.8.2
   staticcheck -version   # expect: staticcheck 2026.2.2 (0.8.2)
   ```
3. Start local PostgreSQL 15 and Redis 7 (`make services-up` as non-root runner, verifying `ADR-003` Branch 2) and run `bash scripts/migrate.sh up` + `bash scripts/test-migrations.sh` (`T1`–`T18`).
4. Run Go verification gates inside `backend-go/`:
   ```bash
   test -z "$(gofmt -l .)"
   go vet ./...
   staticcheck ./...
   go test ./... -race -count=1
   go build ./cmd/api
   ```

### Job 3: `processor-rust`
1. Install `tesseract-ocr` (`sudo apt-get update && sudo apt-get install -y tesseract-ocr`) so OCR integration tests run unskipped in CI.
2. Start local PostgreSQL 15 and Redis 7 with migrations applied.
3. Run Rust verification gates inside `processor-rust/`:
   ```bash
   cargo fmt --check
   cargo clippy -- -D warnings
   cargo test --all -- --nocapture
   cargo build --release
   ```

---

## 4. Upgrade & Maintenance Policy

- **Go minor/major upgrades:** Re-test `honnef.co/go/tools/cmd/staticcheck@v0.8.2` against the target Go version in a feature branch; bump the pin and update §2 above if `staticcheck` requires a newer release.
- **No `//nolint` suppressions or weakened tests:** Any new linter warning must be resolved in code rather than suppressed.
