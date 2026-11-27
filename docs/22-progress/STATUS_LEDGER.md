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

---

## Open UNVERIFIED & Active Blockers

1. **[CRITICAL BLOCKER — Ephemeral Sandbox `.git/` & Root-FS Wipe]:** Between P4 (`2026-11-26 10:26 PST`) and P5 (`2026-11-26 21:04 PST`), the sandbox container recycled (`2026-11-27 04:55 UTC`). Workspace files under `/app/applet` persisted, **but `/app/applet/.git/` (including commits `5a1229a` and `1ebb7ec`), `/root/.gitconfig`, `/usr/local/go`, `/root/.cargo`, `/usr/lib/postgresql/15/bin`, and `/usr/bin/redis-server` were wiped**.
   - *Impact on P5:* Phase F commit cannot produce "3 commits total on `main`" (`git status` returns `fatal: not a git repository: .git`), and Phase B live service startup could not run because `apt-get` is prohibited in P5.
   - *Required PO Decision:* (a) Authorize fresh `git init -b main` + immediate `origin` remote configuration so git history persists outside the sandbox, and (b) authorize a repeatable bootstrap script (`scripts/bootstrap-env.sh`) to restore Go `1.27.4`, Rust `1.100.0`, PostgreSQL `15.20`, and Redis `7.0.15` after container recycles.
2. **[UNVERIFIED-IN-SANDBOX — Non-root `Makefile` live service startup (Flag 2 / ADR-003)]:** Temporary user `ci_sim` (`uid=1001`) proved `run_as()` Branch 2 executes without `su`/`chown` and enforces guards/status/stop shell logic, but live PostgreSQL/Redis startup as `ci_sim` failed with `initdb: No such file or directory` / `redis-cli: command not found` due to container recycle + `Do NOT use apt-get`. Tracked as **MUST VERIFY ON FIRST CI RUN** in `docs/23-decisions/ADR-003-local-dev-direct-processes.md`.
3. **[UNVERIFIED — Remote git push & branch protection]:** No git remote (`origin`) has been configured yet.
4. **[UNVERIFIED — CI PATH-ordering verification (ADR-004)]:** No `.github/workflows/` CI pipeline exists yet to enforce Go `PATH` ordering in CI.

---

## Closed Items (Verified in Prompt 5)

- **Flag 1 (`db-reset` host/port guards & ADR-005 parity):** **VERIFIED** — `Makefile:204-212` refuses `PG_PORT=5433` (`exit: 2`) and `PGHOST=0.0.0.0` (`exit: 2`) before prompting; `echo "no" | make db-reset` aborts cleanly (`exit: 2`); ADR-005 §Consequences updated with exact `Makefile:88-104` and `Makefile:204-212` references.
- **Flag 3 (`backend-go/README.md` stale link):** **VERIFIED** — Updated `docs/06-architecture/` -> `docs/05-architecture/`; repo-wide grep confirms zero unintended stale paths remain (only intentional historical mapping entries in `ADR-001`).
- **Flag 4 (`bun.lock` integrity):** **VERIFIED** — `bun install --frozen-lockfile --dry-run` in `/tmp/bun-check` using `bun 1.4.4` exited `0`; lockfile is intact, install resolves cleanly without `package.json` changes, and `/tmp/bun-check` was deleted.
