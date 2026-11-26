# Prompt 2 — Environment Capability Probe (`ENV_PROBE_REPORT.md`)

**Date:** 2026-11-26  
**Mode:** MODE A — ARCHITECT (Capability Probe — No Application Code / No Scaffolding)  
**Working Directory:** `/app/applet`  
**Execution User:** `root` (`uid=0(root) gid=0(root) groups=0(root)`); `sudo` not installed / not required.

---

## 9. Summary Table

| Tool / Service | Status | Evidence |
|---|---|---|
| **git** | **WORKING** | `git version 2.34.1`; `git init` succeeded in `/app/applet`; `git status --short` exited `0` after `git config --global --add safe.directory /app/applet` |
| **go** | **WORKING** | `apt-get install -y golang-go` installed `go1.19.8`; direct tarball from `go.dev/dl/go1.27.4.linux-amd64.tar.gz` installed `/usr/local/go/bin/go` (`go version go1.27.4 linux/amd64`); `go run ./hello.go` printed `hello from go probe` (exit `0`) |
| **rust** | **WORKING** | `rustup` installed `rustc 1.100.0` & `cargo 1.100.0` (`$HOME/.cargo/bin`, C linker `/usr/bin/cc` present); `cargo run` printed `Hello, world!` (0.40s, exit `0`); `cargo check` with `serde` (`derive`) downloaded 7 crates and passed (7.40s, exit `0`) |
| **postgres** | **WORKING** | `PostgreSQL 15.20` binaries installed (`/usr/lib/postgresql/15/bin/{initdb,pg_ctl,postgres}`, `/usr/bin/psql`); direct cluster start as `appuser` succeeded (`server started`) and `psql -h 127.0.0.1 -p 5432 -U postgres -c 'SELECT version();'` returned 1 row (exit `0`). *Caveat: `postgresql-common` / `service postgresql` failed during `dpkg` unpack due to `/var/log` mount restrictions; must run directly via `pg_ctl` as `appuser`.* |
| **redis** | **WORKING** | `Redis server v=7.0.15` (`redis-server`, `redis-cli`) installed; `redis-server --daemonize yes` started cleanly on port `6379`; `redis-cli ping` returned `PONG` (exit `0`). *Caveat: `service redis-server` not configured; must launch `redis-server` directly.* |
| **docker** | **BLOCKED** | `docker` binary not installed; `/var/run/docker.sock` absent; sandboxed gVisor kernel (`Linux 4.19.0-gvisor`). Plan must run PostgreSQL and Redis directly as local processes (no Docker/Compose). |
| **node** | **WORKING** | Pre-installed `node v22.23.5` / `npm 10.9.9`; existing React+Vite+TS+Tailwind starter in `/app/applet/src` untouched. |

### Option A Viability & Blockers Assessment
- **No blockers invalidate Option A.** All core compilers/runtimes (`git`, `node`, `go 1.27.4`, `rust 1.100.0`) and backing data stores (`PostgreSQL 15.20`, `Redis 7.0.15`) run natively with full package registry access and generous compute resources (`4` vCPUs, `12Gi` RAM, `283G` free disk).
- **Operational constraints to account for in architecture & dev scripts:**
  1. **No Docker / Compose**: Start PostgreSQL (`pg_ctl` as `appuser` with data/socket/log dirs outside `/var/log`) and Redis (`redis-server --daemonize yes`) directly via a local dev script/Makefile.
  2. **`/var/log` gVisor mount restriction**: `/var/log` is a host-forwarded overlay (`aname=/logs/0`) where `dpkg` rename/unlink returns `Operation not permitted`. This broke `dpkg` configuration for `postgresql-common` and `sysstat` (leaving them half-installed in `dpkg`). Avoid `apt` packages that create/replace paths under `/var/log`, and keep service logs in `/tmp` or a workspace-local directory.
  3. **PATH precedence**: Prepend `/usr/local/go/bin` (for Go `1.27.4` over Debian's `/usr/bin/go` `1.19.8`) and `$HOME/.cargo/bin` (for `rustc`/`cargo`) in shell profiles/scripts.
  4. **Git ownership**: `/app/applet` is owned by `appuser:appuser` while commands run as `root`; `git config --global --add safe.directory /app/applet` is required (already applied).

---

## Pre-Install `apt-get` Dry-Run Findings (Executed Before Any Installs)

### Initial dry-run (before `apt-get update`)
```text
apt-get install -y --dry-run golang-go    -> E: Unable to locate package golang-go
apt-get install -y --dry-run postgresql   -> E: Unable to locate package postgresql
apt-get install -y --dry-run redis-server -> E: Unable to locate package redis-server
```
*(Container shipped with empty apt package lists.)*

### Dry-run after `apt-get update` (Debian 12 bookworm + Ubuntu 22.04 jammy sources)
- `apt-get install -y --dry-run golang-go`: resolved `golang-go (2:1.19~1)` / `golang-1.19-go (1.19.8-2)` + `pkg-config`.
- `apt-get install -y --dry-run postgresql`: resolved `postgresql (15+248+deb12u2)`, `postgresql-15 (15.20-0+deb12u1)`, `postgresql-client-15 (15.20-0+deb12u1)`, `postgresql-common`, `sysstat`, `psmisc`.
- `apt-get install -y --dry-run redis-server`: resolved `redis-server (5:7.0.15-1~deb12u10)`, `redis-tools`, `libjemalloc2`, `liblzf1`.
- `apt-get install -y --dry-run docker.io`: resolved `docker.io (29.1.3-0ubuntu3~22.04.2)` in apt metadata (not installed; cannot run daemon inside gVisor).

---

## Detailed Probe Results (Items 1–8)

### 1. GIT — `WORKING`

**Commands & Output:**
```text
$ git --version
git version 2.34.1

$ git init
Initialized empty Git repository in /app/applet/.git/

$ git status --short
fatal: detected dubious ownership in repository at '/app/applet'
To add an exception for this directory, call:
	git config --global --add safe.directory /app/applet

$ git config --global --add safe.directory /app/applet
$ git status --short
?? .env.example
?? .gitignore
?? RECON_REPORT.md
?? bun.lock
?? index.html
?? metadata.json
?? package.json
?? src/
?? tsconfig.json
?? vite.config.ts
(exit code: 0)
```

---

### 2. GO — `WORKING`

- **2a. `apt-get install -y golang-go`:** Succeeded (root user; non-fatal apt log unlink warnings on `/var/log/apt/eipp.log.xz`). Installed Debian's older Go:
  ```text
  $ /usr/bin/go version
  go version go1.19.8 linux/amd64
  ```
- **2b. Direct tarball download:** Queried `https://go.dev/VERSION?m=text` (`go1.27.4`), downloaded `https://go.dev/dl/go1.27.4.linux-amd64.tar.gz`, extracted to `/usr/local/go` (exit `0`).
- **2c. Version after install:**
  ```text
  $ PATH="/usr/local/go/bin:$PATH" which go && PATH="/usr/local/go/bin:$PATH" go version
  /usr/local/go/bin/go
  go version go1.27.4 linux/amd64
  ```
- **2d. Minimal hello-world (`go run ./hello.go` in `/tmp/probe-go`, cleaned up afterward):**
  ```text
  hello from go probe
  go run exit: 0
  ```

---

### 3. RUST — `WORKING`

- **Prerequisite check:** C linker `/usr/bin/cc` already present.
- **3a. `curl https://sh.rustup.rs -sSf | sh -s -- -y`:** Succeeded (exit `0`), installed `stable-x86_64-unknown-linux-gnu` under `$HOME/.cargo/bin`.
- **3b. Versions:**
  ```text
  rustc 1.100.0 (0ef1bc386 2026-11-09)
  cargo 1.100.0 (3acc79c0c 2026-11-05)
  ```
- **3c. Minimal hello-world (`cargo new probe && cd probe && cargo run` in `/tmp/probe-rust`):**
  ```text
  Compiling probe v0.1.0 (/tmp/probe-rust/probe)
  Finished `dev` profile [unoptimized + debuginfo] target(s) in 0.40s
  Running `target/debug/probe`
  Hello, world!
  cargo run exit: 0
  ```
- **3d. `cargo check` with `serde = { version = "1", features = ["derive"] }`:**
  ```text
  Updating crates.io index
  Locking 7 packages to highest Rust 1.100.0 compatible versions
    Adding proc-macro2 v1.0.107, quote v1.0.47, serde v1.0.229, serde_core v1.0.229,
           serde_derive v1.0.229, syn v3.0.10, unicode-ident v1.0.27
  Checking probe v0.1.0 (/tmp/probe-rust/probe)
  Finished `dev` profile [unoptimized + debuginfo] target(s) in 7.40s
  cargo check serde exit: 0
  ```
  *(Temporary `/tmp/probe-rust` directory removed after test.)*

---

### 4. POSTGRES — `WORKING` (Direct Process Mode)

- **Install attempt (`DEBIAN_FRONTEND=noninteractive apt-get install -y postgresql`):**
  Unpacked `postgresql-15` (`15.20-0+deb12u1`) and `postgresql-client-15`, but `dpkg` failed (exit `1`) while unpacking `postgresql-common` and `sysstat`:
  ```text
  dpkg: error processing archive .../41-sysstat_12.6.1-1_amd64.deb (--unpack):
   unable to install new version of './var/log/sysstat': Operation not permitted
  Errors were encountered while processing:
   .../11-postgresql-common_248+deb12u2_all.deb
   .../41-sysstat_12.6.1-1_amd64.deb
  ```
  Root cause confirmed via `mount`:
  ```text
  none on /var/log type overlay (rw,trans=fd,rfdno=11,wfdno=11,aname=/logs/0,...)
  ```
  Because `postgresql-common` did not configure, `service postgresql`, `pg_lsclusters`, `pg_ctlcluster`, and OS user `postgres` do not exist.
- **Installed binary version:**
  ```text
  $ /usr/lib/postgresql/15/bin/postgres --version
  postgres (PostgreSQL) 15.20 (Debian 15.20-0+deb12u1)
  $ psql --version
  psql (PostgreSQL) 15.20 (Debian 15.20-0+deb12u1)
  ```
- **Direct start & `psql` connection proof (run as `appuser` in `/tmp/pgprobe`, cleaned up afterward):**
  ```text
  $ su -s /bin/sh appuser -c "/usr/lib/postgresql/15/bin/initdb -D /tmp/pgprobe/data -U postgres --auth=trust"
  initdb exit: 0

  $ su -s /bin/sh appuser -c "/usr/lib/postgresql/15/bin/pg_ctl -D /tmp/pgprobe/data -l /tmp/pgprobe/pg.log -o '-k /tmp/pgprobe/run -p 5432' start"
  waiting for server to start.... done
  server started
  pg_ctl start exit: 0

  $ su -s /bin/sh appuser -c "psql -h 127.0.0.1 -p 5432 -U postgres -c 'SELECT version();'"
  PostgreSQL 15.20 (Debian 15.20-0+deb12u1) on x86_64-pc-linux-gnu, compiled by gcc (Debian 12.2.0-14+deb12u1) 12.2.0, 64-bit
  (1 row)
  psql connect exit: 0
  ```

---

### 5. REDIS — `WORKING` (Direct Daemon Mode)

- **Install attempt (`DEBIAN_FRONTEND=noninteractive apt-get install -y redis-server`):**
  Unpacked `libjemalloc2`, `liblzf1`, `redis-tools`, and `redis-server (5:7.0.15-1~deb12u10)`. (`dpkg` exited `1` only due to the pre-existing unconfigured `postgresql-common`/`sysstat` packages; Redis binaries themselves installed cleanly.)
- **Version:**
  ```text
  Redis server v=7.0.15 sha=00000000:0 malloc=jemalloc-5.3.0 bits=64 build=ae4d7c971a948f0
  ```
- **Start status & ping:** `service redis-server start` returned `unrecognized service`; launching directly via `redis-server --daemonize yes` succeeded on port `6379`:
  ```text
  $ redis-cli ping
  PONG
  redis-cli ping exit: 0

  $ redis-cli info server | grep -E '^(redis_version|os|process_id|tcp_port):'
  redis_version:7.0.15
  os:Linux 4.19.0-gvisor x86_64
  process_id:2505
  tcp_port:6379
  ```
  *(Stopped cleanly afterward via `redis-cli shutdown nosave`.)*

---

### 6. DOCKER — `BLOCKED`

- `command -v docker`: not installed.
- `/var/run/docker.sock`: `No such file or directory`.
- Kernel: `Linux 4.19.0-gvisor` (gVisor sandbox; nested Docker daemon/containerd is unsupported).
- **Architectural conclusion:** Do not rely on Docker or Docker Compose inside this environment; run PostgreSQL and Redis directly as local processes.

---

### 7. FILESYSTEM, RAM, AND CPU

```text
$ df -h .
Filesystem      Size  Used Avail Use% Mounted on
none            283G  239M  283G   1% /

$ free -h
               total        used        free      shared  buff/cache   available
Mem:            12Gi        99Mi        11Gi       240Mi       353Mi        11Gi
Swap:             0B          0B          0B

$ nproc
4
```

---

### 8. NETWORK REACHABILITY

| Endpoint | Command | Observed HTTP Status | Notes |
|---|---|---|---|
| `https://proxy.golang.org` | `curl -sSI https://proxy.golang.org` | `HTTP/2 200` | Reachable; Go module proxy available |
| `https://crates.io` | `curl -sSI https://crates.io` | `HTTP/2 403` | Bare HEAD to web root rejected (`403`) |
| `https://index.crates.io/config.json` | `curl -sSI https://index.crates.io/config.json` | `HTTP/2 200` | Cargo sparse index reachable; verified live `serde` download via `cargo check` |
| `https://registry.npmjs.org` | `curl -sSI https://registry.npmjs.org` | `HTTP/2 200` | Reachable |
| `https://go.dev/VERSION?m=text` | `curl -sSI https://go.dev/VERSION?m=text` | `HTTP/2 200` | Reachable (`go1.27.4`) |
