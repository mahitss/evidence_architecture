# Local Development Setup Guide

This guide walks from a fresh workspace/clone to verified toolchains and running local backing services (PostgreSQL 15 and Redis 7) on this host without Docker.

---

## NEVER DO THIS

> **LOCAL DEVELOPMENT ONLY:** The `Makefile` runs PostgreSQL (port `5432`) with `--auth=trust` and Redis (port `6379`) without authentication on `127.0.0.1`.
>
> - **NEVER** expose port `5432` or port `6379` outside `127.0.0.1` (no `0.0.0.0` binding, no port forwarding to a LAN or public interface, no reverse proxying).
> - **NEVER** reuse this `Makefile` or `--auth=trust` configuration in staging, production, CI runners with untrusted tenants, or any shared environment.
> - **For any non-local environment**, you **MUST** enforce `scram-sha-256` (or stronger) authentication, TLS encryption in transit, least-privilege roles, Redis ACL/AUTH, and private network isolation (see `docs/23-decisions/ADR-005-postgres-local-trust-auth.md`).
>
> **Exact failure modes if these ports or trust-auth settings are exposed:**
> 1. **Data exfiltration:** Any network client reaching `5432` or `6379` gains immediate unauthenticated read access to all case records, evidence metadata, PII, and cached session/job payloads.
> 2. **Unauthorized schema changes & data destruction:** Because `--auth=trust` grants superuser access without a password, an attacker can drop databases, alter schemas, tamper with audit trails, or execute arbitrary PostgreSQL `COPY ... PROGRAM` commands.
> 3. **Credential & token theft:** Any stored user password hashes, API tokens, or Redis session keys can be dumped in full without triggering authentication failures.

---

## 1. Configure Shell `PATH`

Add the verified Go (`1.27.4`) and Rust/Cargo (`1.100.0`) toolchains to the front of your `PATH` so they precede system binaries (including Debian's EOL `/usr/bin/go` `1.19.8`):

```bash
export PATH="/usr/local/go/bin:$HOME/.cargo/bin:$PATH"
```

Persist this line in `~/.bashrc` or `~/.profile` for interactive sessions.

---

## 2. Verify Toolchain & Environment

Run the environment check target from the repository root (`/app/applet`):

```bash
make env-check
```

This verifies:
- `/usr/local/go/bin/go` resolves ahead of `/usr/bin/go` (`go1.27.4`)
- `$HOME/.cargo/bin/cargo` and `$HOME/.cargo/bin/rustc` resolve first (`1.100.0`)
- `git`, `node`, `npm`, `postgres`, and `redis-server` are present and match `.tool-versions`

---

## 3. Start Local Backing Services

PostgreSQL and Redis run directly as local processes owned by `appuser`, storing all data, sockets, PIDs, and logs under `./data/` (which is gitignored):

```bash
make services-up
make services-status
```

Expected result:
- PostgreSQL 15 listening on `127.0.0.1:5432` with database `truecrime_dev` initialized (`data/pgdata/`, logs in `data/logs/postgres.log`)
- Redis 7 listening on `127.0.0.1:6379` responding `PONG` (`data/redis/`, logs in `data/logs/redis.log`)

---

## 4. Common Daily Commands

```bash
make help             # List all available targets
make services-status  # Verify PostgreSQL and Redis are healthy
make db-reset         # Drop and recreate local dev database (prompts for 'yes')
make services-down    # Gracefully stop PostgreSQL and Redis
make clean            # Remove build artifacts (preserves ./data/)
```

---

## 5. Troubleshooting Known Environment Limitations

See [`KNOWN_ENV_LIMITATIONS.md`](KNOWN_ENV_LIMITATIONS.md) for full details. Quick fixes:

1. **Docker commands fail / no Docker Compose:** Docker is unsupported under gVisor. Always use `make services-up` / `make services-down` locally instead of `docker compose`.
2. **`go version` shows `1.19.8` instead of `1.27.4`:** Your shell `PATH` did not prepend `/usr/local/go/bin`. Run `export PATH="/usr/local/go/bin:$HOME/.cargo/bin:$PATH"` and re-verify with `make env-check`.
3. **Service fails writing to `/var/log` or `service postgresql` says unrecognized service:** `/var/log` is a restricted gVisor overlay and `postgresql-common` is unconfigured. Never use `service postgresql` or `service redis-server`; always use `make services-up` which writes logs to `./data/logs/`.
4. **Slow or memory-heavy Rust builds (`4` vCPUs, `11 GiB` RAM, `0 B` swap):** Prefer `cargo check` during iteration and throttle parallel jobs if needed (`CARGO_BUILD_JOBS=2 cargo build`).
5. **`apt-get` reports unconfigured `postgresql-common` / `sysstat`:** Do not run `apt` unless essential. If required, run `dpkg --configure -a` first and report the outcome; do not purge packages without approval.
6. **`curl -I https://crates.io` returns `403`:** Expected. Cargo uses the sparse index (`https://index.crates.io/config.json`, which returns `200`).
7. **`git status` reports `dubious ownership in repository at '/app/applet'`:** Occurs when running as `root` inside an `appuser`-owned workspace. Fix with `git config --global --add safe.directory /app/applet`.

---

## 6. Running as non-root (CI parity)

The `Makefile` helper `run_as()` supports both container-root local shells and non-root CI runners (such as GitHub Actions `runner`):

- **Branch 1 (`id -u == 0`, root):** Drops privileges via `su -s /bin/bash $(SVC_USER) -c ...` and ensures `./data/` is owned by `$(SVC_USER)`.
- **Branch 2 (`id -u != 0`, non-root):** Skips `su` and `chown`, executing `bash -c ...` directly as the current user.

### Commands (non-root user)

```bash
export PATH="/usr/local/go/bin:$HOME/.cargo/bin:$PATH"
make env-check
make services-up
make services-status
make services-down
```

### Known Caveats

1. **`./data/` ownership:** When running as a non-root user, that user must have write permission to `./data/` (e.g., own the workspace checkout as in CI, or own `./data/`). If `./data/` was previously initialized as another user (e.g., `appuser`), remove or `chown` `./data/` before running `make services-up`.
2. **Sandbox verification status (`UNVERIFIED-IN-SANDBOX`):** In Prompt 5, testing with temporary user `ci_sim` (`uid=1001`) confirmed `run_as()` Branch 2 executes without invoking `su` or `chown` and that guards/status/stop shell logic run cleanly. However, live PostgreSQL/Redis startup as `ci_sim` could not complete because container idle recycle wiped `/usr/lib/postgresql/15/bin/*` and `redis-server` from the ephemeral root filesystem while `apt-get` was prohibited. Full live non-root service verification is tracked as **MUST VERIFY ON FIRST CI RUN** in `docs/23-decisions/ADR-003-local-dev-direct-processes.md`.
3. **Ephemeral root filesystem on container recycle:** On this sandbox host, files under `/app/applet` persist across idle restarts, whereas `.git/` and OS-level installs outside `/app/applet` (`/usr/local/go`, `/root/.cargo`, PostgreSQL, Redis, `/root/.gitconfig`) do not. Always push commits to a remote origin once configured and re-bootstrap toolchains after container recycles.

---

## 7. Database migrations

Database schema migrations are managed with **`golang-migrate` CLI (`v4`)** compiled with the PostgreSQL driver tag via `go install` (never `apt-get`; see `docs/23-decisions/ADR-016-migration-tooling.md`).

### Install `golang-migrate` (one-time per fresh sandbox)

```bash
bash scripts/install-migrate.sh
# Equivalent direct command:
# go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

Ensure `$HOME/go/bin` is on your `PATH` (handled automatically by `scripts/migrate.sh`):

```bash
export PATH="/usr/local/go/bin:$HOME/go/bin:$HOME/.cargo/bin:$PATH"
```

### Run Migrations Locally

Migrations live in `backend-go/migrations/` (moved from root `migrations/` per `ADR-016`). Start PostgreSQL first (`make services-up`), then use `scripts/migrate.sh` (defaults to `postgres://postgres@127.0.0.1:5432/truecrime_dev?sslmode=disable` and refuses any non-`127.0.0.1` host):

```bash
bash scripts/migrate.sh up          # Apply all pending migrations
bash scripts/migrate.sh up 1        # Apply next 1 migration step
bash scripts/migrate.sh status      # Print current migration version
bash scripts/migrate.sh down 1      # Roll back 1 migration (default: 1)
bash scripts/migrate.sh down 2      # Roll back 2 migrations
bash scripts/migrate.sh force <V>   # Force version V to recover dirty state
bash scripts/migrate.sh reset       # Roll back all then re-apply all (prompts for 'yes')
```

### Verify All Schema Constraints & Triggers (`T1`–`T18`)

Run the end-to-end migration and database invariant test suite (starts local services, verifies all 8 tables, tests Q5 `CHECK` constraint, `NOT NULL`, `chk_no_recreated_tier3_tier4`, bidirectional legal-review publish trigger including `T17` rejected review & `T18` `prohibited_checklist_passed=FALSE`, append-only `audit_log`, immutable `legal_review_records`, full rollback + re-apply, and stops local services):

```bash
bash scripts/test-migrations.sh
```

---

## 8. Running the Go API

`backend-go/` depends on PostgreSQL 15 running locally with schema migrations applied from `backend-go/migrations/`.

### Required Environment Variables

| Variable | Required | Default | Notes |
|---|---|---|---|
| `DATABASE_URL` | **Yes** | — | Must use `127.0.0.1` host locally (e.g. `postgres://postgres@127.0.0.1:5432/truecrime_dev?sslmode=disable`); startup refuses non-`127.0.0.1` hosts |
| `JWT_SECRET` | **Yes** | — | Minimum 32 characters (HS256 signing key; never commit real secrets) |
| `LISTEN_ADDR` | **Yes** | — | Bind address (e.g. `127.0.0.1:8080`) |
| `LOG_LEVEL` | No | `info` | Zerolog level (`debug`, `info`, `warn`, `error`) |
| `JWT_TTL` | No | `60m` | Admin Bearer JWT lifetime |
| `PORT` | No | `8080` | Optional port metadata (`1`–`65535`) |

### Start Locally

```bash
export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"
make services-up
bash scripts/migrate.sh up

cd backend-go
DATABASE_URL="postgres://postgres@127.0.0.1:5432/truecrime_dev?sslmode=disable" \
JWT_SECRET="replace-with-local-dev-secret-at-least-32-chars" \
LISTEN_ADDR="127.0.0.1:8080" \
go run cmd/api/main.go
```

### Expected Startup & Health Output

```text
{"level":"info","listen_addr":"127.0.0.1:8080","log_level":"info","time":"...","message":"starting backend-go api"}
{"level":"info","addr":"127.0.0.1:8080","version":"0.1.0","time":"...","message":"http_server_listening"}
```

Verify in a second terminal:

```bash
curl -s http://127.0.0.1:8080/health
# Expected: {"status":"ok","db":"ok","version":"0.1.0"}
```

### Run Unit & Integration Tests

```bash
make services-up
bash scripts/migrate.sh up
cd backend-go
go test ./... -race -count=1
go vet ./...
gofmt -l .
```


