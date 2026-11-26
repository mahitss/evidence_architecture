# Local Development Setup Guide

This guide walks from a fresh workspace/clone to verified toolchains and running local backing services (PostgreSQL 15 and Redis 7) on this host without Docker.

> **Security Note:** The local `Makefile` defaults (`trust` auth on `127.0.0.1:5432`, unauthenticated Redis on `127.0.0.1:6379`) are strictly for **local development only** and must never be used in staging or production.

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
