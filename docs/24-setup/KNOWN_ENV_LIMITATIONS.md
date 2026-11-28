# Known Environment Limitations

This document records verified constraints of the current sandboxed development host (`Debian GNU/Linux 12 (bookworm)` on `Linux 4.19.0-gvisor x86_64`) and the required operational mitigations.

---

## 1. No Docker / No Docker Compose on This Host

- **Fact:** The `docker` binary is not installed, `/var/run/docker.sock` does not exist, and the gVisor sandbox kernel (`4.19.0-gvisor`) does not support running a nested Docker/containerd daemon.
- **Mitigation:** Run backing services directly as local OS processes via `make services-up` / `make services-down` (PostgreSQL 15 via `pg_ctl` as `appuser`, Redis 7 via `redis-server --daemonize yes`). Note that CI (e.g., GitHub Actions) runs in a different environment and may use Docker service containers where appropriate.

---

## 2. `dpkg` Dirty State (`postgresql-common` and `sysstat`)

- **Fact:** During the Prompt 2 probe, `dpkg` failed to configure `postgresql-common` and `sysstat` because `/var/log` is a restricted gVisor overlay mount where `dpkg` rename/unlink operations return `Operation not permitted`. Both packages remain unconfigured in `dpkg`, while the `postgresql-15` and `redis-server` binaries themselves unpacked and run cleanly.
- **Mitigation / Policy:**
  - Avoid running `apt` / `apt-get` except where strictly essential.
  - If `apt` is genuinely needed in the future, first run `dpkg --configure -a` and report the exact result before proceeding.
  - Do **not** remove or purge `postgresql-common` or `sysstat` without explicit approval.

---

## 3. EOL Go `1.19.8` (`apt`) Shadowing Verified Go `1.27.4`

- **Fact:** Debian's `golang-go` package installed Go `1.19.8` at `/usr/bin/go` (end-of-life). Verified Go `1.27.4` is installed at `/usr/local/go/bin/go`. The host's default shell `PATH` is:
  ```text
  /usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
  ```
  Under this default `PATH`, bare `go` resolves to `/usr/bin/go` (`1.19.8`) and `cargo`/`rustc` are not on `PATH`.
- **Mitigation / Policy:** `PATH` **must** resolve `/usr/local/go/bin/go` ahead of `/usr/bin/go`, and `$HOME/.cargo/bin` ahead of system paths. Add this exact line to any shell profile (`~/.bashrc`, `~/.profile`, or session init):
  ```bash
  export PATH="/usr/local/go/bin:$HOME/.cargo/bin:$PATH"
  ```
  The root `Makefile` also prepends these directories automatically and enforces precedence in `make env-check`.

---

## 4. Compute Envelope (`4` vCPU / `11 GiB` Available RAM)

- **Fact:** Host resources are `4` vCPUs (`nproc`) and `12 GiB` total RAM (`11 GiB` available, `0 B` swap) with `283 GiB` free disk.
- **Mitigation:** Rust release builds (`cargo build --release`) will be noticeably slower than on a high-core workstation. Do not attempt GPU-accelerated builds or unconstrained parallel builds; use `cargo check` for fast iteration and cap parallelism when needed (e.g., `CARGO_BUILD_JOBS=2`).

---

## 5. `/var/log` Restricted Overlay Mount

- **Fact:** `/var/log` is mounted as a gVisor host-forwarded overlay (`aname=/logs/0`) that rejects certain atomic file operations (`Operation not permitted`). System service wrappers (`service postgresql`, `service redis-server`) are unavailable.
- **Mitigation:** Never configure local development services to write logs, sockets, or PID files under `/var/log`. Store all local PostgreSQL and Redis runtime artifacts under `/app/applet/data/` (specifically `/app/applet/data/logs/`, `/app/applet/data/pgdata/`, `/app/applet/data/pgrun/`, and `/app/applet/data/redis/`), which is gitignored.

---

## 6. Network Behavior with `crates.io` (`HTTP 403` on `HEAD`)

- **Fact:** Sending an HTTP `HEAD` request (`curl -I https://crates.io`) returns `HTTP/2 403`. However, Cargo's sparse registry endpoint (`https://index.crates.io/config.json`) returns `HTTP/2 200`, and live crate downloads (`cargo check` with `serde`) succeed cleanly.
- **Mitigation:** Do not use `HEAD` probes against `https://crates.io` to test Rust registry connectivity. Use Cargo's default sparse index protocol (`index.crates.io`) or test via `curl -I https://index.crates.io/config.json`.

---

## 7. Ephemeral Container Root-FS & `git credential.helper store` (Local Sandbox Only)

- **Fact:** On idle container recycle, files under `/app/applet` persist, while `/app/applet/.git/`, `/root/.gitconfig`, `/root/.git-credentials`, and OS-level installs (`/usr/local/go`, `/root/.cargo`, `/usr/lib/postgresql/15/bin`, `/usr/bin/redis-server`) are wiped. To enable non-interactive pushes to the remote GitHub repository (now the source of truth), this sandbox configures `git config --global credential.helper store`, which writes credentials in **plaintext** to `~/.git-credentials`.
- **Mitigation / Policy:**
  - `credential.helper store` is a **local-sandbox-only** decision, acceptable solely because this container is single-tenant and its root filesystem is wiped on recycle.
  - **NEVER** use `git config --global credential.helper store` on a shared, persistent, or production host; use short-lived OAuth tokens, OS keychains, or SSH keys with passphrases instead.
  - Always rotate any Personal Access Token (PAT) pasted during interactive setup, and keep `*.git-credentials` in `.gitignore` as defense in depth.
  - Run `bash scripts/bootstrap-env.sh` at the start of any session after a container recycle.

---

## 8. Meilisearch Binary Download Requires Outbound Network (`make meili-up`)

- **Fact:** `meilisearch` is not pre-installed in the sandbox image and cannot be installed via `apt-get` or Docker. `make meili-up` checks `command -v meilisearch` first and otherwise downloads the official `meilisearch-linux-amd64` release binary from GitHub Releases into `./data/meili/meilisearch-bin` (gitignored under `data/`).
- **Mitigation:** Verified working in this sandbox (GitHub release asset download succeeds over HTTPS and persists under `/app/applet/data/meili/meilisearch-bin` across container restarts since `/app/applet` persists). In air-gapped environments without GitHub access, pre-place a `meilisearch` binary on `PATH` or at `./data/meili/meilisearch-bin` before running `make meili-up`.

