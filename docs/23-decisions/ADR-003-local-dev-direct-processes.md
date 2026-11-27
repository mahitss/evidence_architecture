# ADR-003: Run Local Dev Backing Services as Direct Processes (No Docker)

- **Status:** Accepted
- **Date:** 2026-11-26
- **Deciders:** Product Owner, Architect (MODE A)
- **Refs:** `docs/22-progress/ENV_PROBE_REPORT.md`, `docs/24-setup/KNOWN_ENV_LIMITATIONS.md`

## Context

The Prompt 2 environment probe verified that the primary development host runs Debian 12 inside a sandboxed gVisor kernel (`Linux 4.19.0-gvisor x86_64`) where `docker` is not installed, `/var/run/docker.sock` does not exist, and nested container daemons cannot run. Meanwhile, native PostgreSQL `15.20` and Redis `7.0.15` binaries are installed and verified running directly on localhost (`127.0.0.1:5432` and `127.0.0.1:6379`). Remote CI environments (e.g., GitHub Actions) run on standard Linux VMs where Docker service containers are available.

## Problem

We need a reliable, reproducible way to start, inspect, reset, and stop PostgreSQL and Redis during local development in this gVisor sandbox without blocking CI or future developer laptops from using containerized services.

## Alternatives

1. **Rejected — Require Docker Compose for all local development:**
   - *Reasoning:* Completely blocked on this host (`docker` unavailable under gVisor), which would immediately invalidate Option A and halt all local backend and integration testing.
2. **Rejected — Mock PostgreSQL and Redis or swap to embedded SQLite/in-memory stubs locally:**
   - *Reasoning:* Masks real PostgreSQL SQL dialect, migration, locking, and Redis protocol behaviors; increases production/dev divergence and creates false confidence in integration tests.
3. **Rejected — Depend on an always-on external cloud database for local dev:**
   - *Reasoning:* Introduces network latency, external credential management risk, cost, and shared-state contention during destructive `db-reset` cycles.
4. **Selected — Manage PostgreSQL 15 and Redis 7 as direct local processes behind a uniform `Makefile` interface (`services-up`, `services-down`, `services-status`), while allowing CI to use Docker service containers:**
   - *Reasoning:* Works natively today on gVisor using real PostgreSQL 15 and Redis 7 binaries, isolates runtime state under gitignored `./data/` (avoiding `/var/log` restrictions), and keeps the developer interface stable regardless of host runtime.

## Selected

Run PostgreSQL 15 (`initdb`/`pg_ctl` as `appuser`, data/sockets/logs in `./data/`) and Redis 7 (`redis-server --daemonize yes` as `appuser`, data/logs in `./data/`) directly via `Makefile` targets (`make services-up`, `make services-down`, `make services-status`, `make db-reset`) on this host. CI pipelines (GitHub Actions) may provision matching PostgreSQL 15 and Redis 7 versions via Docker service containers.

## Rationale

Real PostgreSQL 15 and Redis 7 processes guarantee protocol and SQL fidelity while keeping `Makefile` targets as a stable abstraction boundary. Storing runtime artifacts inside `./data/` avoids the gVisor `/var/log` mount restriction and keeps git working trees clean.

## Trade-offs

- Two service-provisioning mechanisms exist across environments: direct processes locally vs. Docker service containers in CI.
- Local developers must ensure minor-version parity (`.tool-versions`: PostgreSQL `15.20`, Redis `7.0.15`) with CI container image tags (`postgres:15`, `redis:7`).

## Risks

- Subtle configuration drift between direct `initdb` defaults and official Docker image entrypoints (e.g., extensions, locale, auth config).
- Orphaned background processes if a session terminates abnormally without running `make services-down`.

## Consequences

- Local development requires no Docker daemon; `make services-up` brings up both services in seconds.
- `Makefile` enforces loopback-only binding (`127.0.0.1`) and standard ports (`5432`, `6379`) to prevent accidental exposure (see ADR-005).
- `./data/` remains strictly gitignored.
- **MUST VERIFY ON FIRST CI RUN:** End-to-end non-root execution of `make services-up` / `make services-status` / `make services-down` (`run_as()` Branch 2, `id -u != 0`) is marked `UNVERIFIED-IN-SANDBOX` after Prompt 5. While `ci_sim` (`uid=1001`) proved the non-root shell path executes without `su` or `chown`, container idle recycle between Prompt 4 and Prompt 5 wiped `/usr/lib/postgresql/15/bin/*` and `redis-server` from the ephemeral root filesystem (and `apt-get` was prohibited in Prompt 5), preventing live service startup verification under `ci_sim`. Verify full non-root service lifecycle on the first CI run (or after toolchain re-bootstrap).

## Reversal Strategy

If development migrates exclusively to a Docker-capable host, add a `docker-compose.yml` matching PostgreSQL 15 / Redis 7, update `make services-up`/`services-down`/`services-status` to delegate to `docker compose` (or select automatically when Docker is detected), and supersede this ADR without changing developer workflows.
