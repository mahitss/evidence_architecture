# Developer Scripts (`scripts/`)

## `bootstrap-env.sh`

### What It Does
`scripts/bootstrap-env.sh` idempotently restores the pinned local development toolchains and backing-service binaries required by this monorepo:

- **Go `1.27.4`** -> `/usr/local/go/bin/go` (direct official tarball download)
- **Rust `1.100.0`** -> `$HOME/.cargo/bin/{rustc,cargo}` (via `rustup`)
- **PostgreSQL 15** -> `/usr/lib/postgresql/15/bin/{postgres,initdb,pg_ctl}` and `psql` (via `apt-get`, tolerating the known gVisor `/var/log` `postgresql-common` configure error and verifying binaries directly)
- **Redis 7** -> `/usr/bin/redis-server` and `redis-cli` (via `apt-get`, verified directly)
- Writes gitignored `.env.path` (`export PATH="/usr/local/go/bin:$HOME/.cargo/bin:$PATH"`) and executes `make env-check` to verify `PATH` ordering and versions.

### When to Run It
Run at the **start of every session after a sandbox container recycle** (when the container root filesystem outside `/app/applet` has been reset). It is safe to re-run at any time—already-installed pinned versions are skipped.

```bash
bash scripts/bootstrap-env.sh
source .env.path
```

### Assumptions & Prerequisites
- Debian 12 (`bookworm`) `amd64` host with `apt-get`, `curl`, `tar`, `make`, and `/usr/bin/cc` present.
- Outbound HTTPS network access to `go.dev`, `sh.rustup.rs`, `static.rust-lang.org`, and Debian apt mirrors.
- **Sudo policy:** When executed as `root` (default in this sandbox), `sudo` is never invoked. When executed as a non-root user, `sudo` is invoked only for system-wide operations (`/usr/local/go` extraction and `apt-get`).

### Local-Sandbox-Only Warning
> **LOCAL SANDBOX ONLY:** `bootstrap-env.sh` is designed strictly for this ephemeral gVisor development sandbox (including `|| true` workarounds for the `/var/log` overlay restriction). **NEVER** use this script to provision staging, production, CI containers, or shared developer hosts.
