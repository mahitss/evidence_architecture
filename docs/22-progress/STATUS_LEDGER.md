# Project Status Ledger

Single source of truth for prompt-by-prompt execution status, commits, and open `UNVERIFIED` / `BLOCKED` items. Updated on every prompt.

*Status values per charter:* `NOT STARTED` / `IN PROGRESS` / `BLOCKED` / `IMPLEMENTED` / `TESTED` / `VERIFIED` / `APPROVED`

---

## Prompt Ledger

| Prompt | Commit | Scope | Status | UNVERIFIED items |
|---|---|---|---|---|
| **P1** | *(pre-repo; moved in `5a1229a`)* | Phase 01 Reconnaissance (`docs/22-progress/RECON_REPORT.md`) | **VERIFIED** | None |
| **P2** | *(pre-repo; moved in `5a1229a`)* | Environment Capability Probe (`docs/22-progress/ENV_PROBE_REPORT.md`) | **VERIFIED** | Closed in P3/P4; note root-fs installs wiped on subsequent container recycle (see P5) |
| **P3** | `5a1229a` *(lost on container recycle — see Open UNVERIFIED #1)* | Phase A Version Truth Verification + Phase B Foundation Init (repo, monorepo skeleton, `.tool-versions`, `Makefile`, env docs) | **VERIFIED** *(historical — `.git/` wiped on container recycle before P5)* | `docs/` charter slugs (closed in P4); `make clean`/`db-reset` (closed in P4) |
| **P4** | `1ebb7ec` *(lost on container recycle — see Open UNVERIFIED #1)* | Audit Flags 1–5: branch `main`, Charter PART 06 `docs/` conformance, ADR-001..005, safe Make tests, `LOCAL DEV ONLY` warnings + `services-up` guards | **VERIFIED** *(historical — `.git/` wiped on container recycle before P5)* | Flag 1 `db-reset` guards (closed in P5); Flag 2 non-root path (`UNVERIFIED-IN-SANDBOX` in P5); Flag 3 `backend-go/README.md` link (closed in P5); Flag 4 `bun.lock` integrity (closed in P5); remote push (still open) |
| **P5** | **BLOCKED** *(no `.git/` after container recycle)* | Close Prompt 4 Flags 1–4 (`db-reset` guards + ADR-005 line refs, `ci_sim` non-root probe + ADR-003/LOCAL_DEV updates, `backend-go/README.md` link fix + stale-link grep, `bun.lock` frozen-lockfile check, `STATUS_LEDGER.md`) | **BLOCKED** *(Phases A, C, D, E verified; Phase B `UNVERIFIED-IN-SANDBOX`; Phase F blocked by missing `.git/`)* | See Open UNVERIFIED #1–#4 below |
| **P6 / Q1–Q5** | `41a8761` *(local `main` after 3rd recycle; synced via PO laptop)* | Phase 03 Requirements (`PRODUCT_REQUIREMENTS.md`, `EVIDENCE_INTEGRITY_SPEC.md`, `FEATURE_CATALOG.md`, `PERMISSION_MATRIX.md`, `USER_STORIES.md`, `REQUIREMENTS_STATUS.md`) + PO Q1–Q5 resolutions (`ADR-006`–`ADR-009` + Q5 `CHECK`) | **VERIFIED** | Remote sync handled from PO laptop |
| **P7** | `903b1dc` *(local `main`)* | Phase 06 Architecture (`SYSTEM_CONTEXT.md`, `HIGH_LEVEL_DESIGN.md`, `LOW_LEVEL_DESIGN.md`, `SEQUENCE_DIAGRAMS.md`, `DATABASE_DESIGN.md`, `openapi.yaml` [Redocly 0 errors/0 warnings], `SECURITY_ARCHITECTURE.md`, `ADR-010`–`ADR-015`, `ARCHITECTURE_STATUS.md`) | **VERIFIED** | Local commit only (PO pulls to laptop) |
| **P8a** | `0162029` *(local `main`)* | Phase 09 Foundation — DB Migrations (`000001_init`, `000002_seed_dev`), `scripts/install-migrate.sh`, `scripts/migrate.sh`, `scripts/test-migrations.sh`, `ADR-016`, `LOCAL_DEV.md` §7 | **VERIFIED** | Follow-ups 1 & 2 (T17 REJECTED review + T18 `prohibited_checklist_passed=FALSE`) closed in P8b |
| **P8b** | *(pending commit in Phase L)* | Phase 09 Continuation — `backend-go/` skeleton, migrations moved to `backend-go/migrations/` (`ADR-016` TODO DONE), `config`, `store` (`pgxpool`), Argon2id + HS256 JWT `auth`, middleware, `/health`, `/api/v1/admin/login`, `/api/v1/admin/cases/{id}`, unit + real-Postgres integration tests, `T17` & `T18` | **TESTED** | `staticcheck` not installed in sandbox (`go vet ./...`, `gofmt -l .`, `go test ./... -race -count=1`, and `scripts/test-migrations.sh` T1–T18 all passed); local commit only (not pushed) |

---

## Open UNVERIFIED & Active Blockers

1. **[CRITICAL BLOCKER — Ephemeral Sandbox `.git/` & Root-FS Wipe]:** Between P4 (`2026-11-26 10:26 PST`) and P5 (`2026-11-26 21:04 PST`), the sandbox container recycled (`2026-11-27 04:55 UTC`). Workspace files under `/app/applet` persisted, **but `/app/applet/.git/` (including commits `5a1229a` and `1ebb7ec`), `/root/.gitconfig`, `/usr/local/go`, `/root/.cargo`, `/usr/lib/postgresql/15/bin`, and `/usr/bin/redis-server` were wiped**.
   - *Impact on P5:* Phase F commit cannot produce "3 commits total on `main`" (`git status` returns `fatal: not a git repository: .git`), and Phase B live service startup could not run because `apt-get` is prohibited in P5.
   - *Required PO Decision:* (a) Authorize fresh `git init -b main` + immediate `origin` remote configuration so git history persists outside the sandbox, and (b) authorize a repeatable bootstrap script (`scripts/bootstrap-env.sh`) to restore Go `1.27.4`, Rust `1.100.0`, PostgreSQL `15.20`, and Redis `7.0.15` after container recycles.
2. **[UNVERIFIED-IN-SANDBOX — Non-root `Makefile` live service startup (Flag 2 / ADR-003)]:** Temporary user `ci_sim` (`uid=1001`) proved `run_as()` Branch 2 executes without `su`/`chown` and enforces guards/status/stop shell logic, but live PostgreSQL/Redis startup as `ci_sim` failed with `initdb: No such file or directory` / `redis-cli: command not found` due to container recycle + `Do NOT use apt-get`. Tracked as **MUST VERIFY ON FIRST CI RUN** in `docs/23-decisions/ADR-003-local-dev-direct-processes.md`.
3. **[UNVERIFIED — Remote git push & branch protection]:** No git remote (`origin`) has been configured yet (commits `41a8761`, `903b1dc`, `0162029`, and P8b are local on `main`).
4. **[UNVERIFIED — CI PATH-ordering verification (ADR-004)]:** No `.github/workflows/` CI pipeline exists yet to enforce Go `PATH` ordering in CI.
5. **[UNVERIFIED-IN-SANDBOX — `staticcheck` lint pass (P8b)]:** `staticcheck` binary is not installed in the sandbox environment; `go vet ./...` and `gofmt -l .` passed cleanly. Tracked for CI Go lint job.

---

## Closed Items

- **Flag 1 (`db-reset` host/port guards & ADR-005 parity — Prompt 5):** **VERIFIED** — `Makefile:204-212` refuses `PG_PORT=5433` (`exit: 2`) and `PGHOST=0.0.0.0` (`exit: 2`) before prompting; `echo "no" | make db-reset` aborts cleanly (`exit: 2`); ADR-005 §Consequences updated with exact `Makefile:88-104` and `Makefile:204-212` references.
- **Flag 3 (`backend-go/README.md` stale link — Prompt 5):** **VERIFIED** — Updated `docs/06-architecture/` -> `docs/05-architecture/`; repo-wide grep confirms zero unintended stale paths remain (only intentional historical mapping entries in `ADR-001`).
- **Flag 4 (`bun.lock` integrity — Prompt 5):** **VERIFIED** — `bun install --frozen-lockfile --dry-run` in `/tmp/bun-check` using `bun 1.4.4` exited `0`; lockfile is intact, install resolves cleanly without `package.json` changes, and `/tmp/bun-check` was deleted.
- **Follow-up 1 (`T17` — REJECTED legal review blocks publish — Prompt 8a audit, closed in P8b):** **VERIFIED** — Added `T17` in `scripts/test-migrations.sh`; publishing `test-case-rejected` linked to a `REJECTED` `legal_review_records` row is rejected by `fn_enforce_legal_review_before_publish()` (`no APPROVED legal_review_records row`), with clean row deletion afterward.
- **Follow-up 2 (`T18` — `prohibited_checklist_passed=FALSE` blocks publish — Prompt 8a audit, closed in P8b):** **VERIFIED** — Added `T18` (`T18a-check` + `T18b-trigger`) in `scripts/test-migrations.sh`; inserting `decision='APPROVED'` with `prohibited_checklist_passed=FALSE` is rejected by `chk_legal_review_approved_complete`, and publishing a case linked to a review with `prohibited_checklist_passed=FALSE` is rejected by `fn_enforce_legal_review_before_publish()`.
- **ADR-016 TODO (`migrations/` → `backend-go/migrations/` — closed in P8b):** **VERIFIED** — Moved via `git mv migrations backend-go/migrations`, updated `scripts/migrate.sh`, `scripts/test-migrations.sh`, `ADR-016`, and `LOCAL_DEV.md`; full `T1`–`T18` suite passed.
