#!/usr/bin/env bash
# =============================================================================
# scripts/bootstrap-env.sh — Ephemeral Sandbox Toolchain & Service Bootstrap
#
# PURPOSE:
#   Restores pinned development toolchains and local backing-service binaries
#   after a sandbox container recycle wipes the root filesystem outside
#   /app/applet:
#     - Go 1.27.4         (/usr/local/go/bin/go)
#     - Rust 1.100.0      ($HOME/.cargo/bin/{rustc,cargo})
#     - PostgreSQL 15     (/usr/lib/postgresql/15/bin/{postgres,initdb,pg_ctl}, psql)
#     - Redis 7           (/usr/bin/redis-server, redis-cli)
#
# IDEMPOTENCY:
#   Safe to run multiple times in the same session. Each ensure_* function
#   checks whether the expected pinned binary/version is already installed and
#   skips re-downloading or reinstalling when present.
#
# PRIVILEGE / SUDO POLICY:
#   Does NOT invoke sudo when already running as root (uid=0, standard in this
#   sandbox). Uses sudo ONLY when invoked by a non-root user for system-level
#   steps that write to /usr/local or run apt-get (ensure_go, ensure_postgres,
#   ensure_redis).
#
# WARNING — LOCAL SANDBOX ONLY:
#   This script is tailored to the ephemeral Debian 12 gVisor sandbox (including
#   tolerating dpkg's known /var/log overlay error on postgresql-common). NEVER
#   use this script to provision staging, production, or shared hosts.
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

GO_VERSION="1.27.4"
RUST_VERSION="1.100.0"
APT_UPDATED=0

# --- Colors & logging helpers ------------------------------------------------
if [[ -t 1 ]]; then
  C_BLUE='\033[0;34m'
  C_YELLOW='\033[0;33m'
  C_RED='\033[0;31m'
  C_RESET='\033[0m'
else
  C_BLUE=''
  C_YELLOW=''
  C_RED=''
  C_RESET=''
fi

info()  { printf '%b[INFO]%b  %s\n' "$C_BLUE" "$C_RESET" "$*"; }
warn()  { printf '%b[WARN]%b  %s\n' "$C_YELLOW" "$C_RESET" "$*" >&2; }
error() { printf '%b[ERROR]%b %s\n' "$C_RED" "$C_RESET" "$*" >&2; }

run_privileged() {
  if [[ "$(id -u)" -eq 0 ]]; then
    "$@"
  else
    # Only used when non-root for system paths (/usr/local, apt-get)
    sudo "$@"
  fi
}

apt_update_once() {
  if [[ "$APT_UPDATED" -eq 0 ]]; then
    info "Updating apt package lists..."
    run_privileged apt-get update -qq
    APT_UPDATED=1
  fi
}

# --- Toolchain & service installers ------------------------------------------

ensure_go() {
  if [[ -x /usr/local/go/bin/go ]] && /usr/local/go/bin/go version 2>/dev/null | grep -q "go${GO_VERSION} "; then
    info "Go ${GO_VERSION} already installed at /usr/local/go/bin/go — skipping."
    return 0
  fi

  info "Installing Go ${GO_VERSION} into /usr/local/go..."
  local tarball="/tmp/go.tar.gz"
  rm -f "$tarball"
  curl -fsSL -o "$tarball" "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz"
  run_privileged rm -rf /usr/local/go
  run_privileged tar -C /usr/local -xzf "$tarball"
  rm -f "$tarball"

  if [[ ! -x /usr/local/go/bin/go ]] || ! /usr/local/go/bin/go version | grep -q "go${GO_VERSION} "; then
    error "Go ${GO_VERSION} installation verification failed."
    exit 1
  fi
  info "Installed $(/usr/local/go/bin/go version)."
}

ensure_rust() {
  local cargo_bin="${CARGO_HOME:-$HOME/.cargo}/bin"
  if [[ -x "${cargo_bin}/rustc" ]] && "${cargo_bin}/rustc" --version 2>/dev/null | grep -q "rustc ${RUST_VERSION} "; then
    info "Rust ${RUST_VERSION} already installed at ${cargo_bin}/rustc — skipping."
    return 0
  fi

  info "Installing Rust ${RUST_VERSION} via rustup..."
  curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain "${RUST_VERSION}"

  if [[ ! -x "${cargo_bin}/rustc" ]] || ! "${cargo_bin}/rustc" --version | grep -q "rustc ${RUST_VERSION} "; then
    error "Rust ${RUST_VERSION} installation verification failed."
    exit 1
  fi
  info "Installed $("${cargo_bin}/rustc" --version)."
}

ensure_postgres() {
  if [[ -x /usr/lib/postgresql/15/bin/postgres ]] && [[ -x /usr/lib/postgresql/15/bin/initdb ]] && [[ -x /usr/lib/postgresql/15/bin/pg_ctl ]] && command -v psql >/dev/null 2>&1; then
    info "PostgreSQL 15 binaries already installed — skipping."
    return 0
  fi

  apt_update_once
  info "Installing PostgreSQL 15 binaries (postgresql-15, postgresql-client-15)..."
  warn "Note: postgresql-common may exit non-zero on /var/log overlay restriction; verifying binaries explicitly."
  run_privileged env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends postgresql-15 postgresql-client-15 || true

  if [[ ! -x /usr/lib/postgresql/15/bin/postgres ]] || [[ ! -x /usr/lib/postgresql/15/bin/initdb ]] || [[ ! -x /usr/lib/postgresql/15/bin/pg_ctl ]]; then
    error "PostgreSQL 15 binaries missing under /usr/lib/postgresql/15/bin after install."
    exit 1
  fi
  if ! command -v psql >/dev/null 2>&1; then
    error "psql client binary missing after PostgreSQL install."
    exit 1
  fi
  info "Verified $(/usr/lib/postgresql/15/bin/postgres --version)."
}

ensure_redis() {
  if [[ -x /usr/bin/redis-server ]] && command -v redis-cli >/dev/null 2>&1; then
    info "Redis server binary already installed at /usr/bin/redis-server — skipping."
    return 0
  fi

  apt_update_once
  info "Installing Redis server binary (redis-server)..."
  run_privileged env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends redis-server || true

  if [[ ! -x /usr/bin/redis-server ]] || ! command -v redis-cli >/dev/null 2>&1; then
    error "Redis binaries (/usr/bin/redis-server or redis-cli) missing after install."
    exit 1
  fi
  info "Verified $(/usr/bin/redis-server --version)."
}

export_path() {
  local path_line='export PATH="/usr/local/go/bin:$HOME/.cargo/bin:$PATH"'
  printf '%s\n' "$path_line" > "$ROOT_DIR/.env.path"
  info "Wrote PATH exports to $ROOT_DIR/.env.path (gitignored)."
  info "For interactive shells, run: source \"$ROOT_DIR/.env.path\""
  printf '  %s\n' "$path_line"
}

main() {
  info "Bootstrapping local sandbox environment for $ROOT_DIR..."
  ensure_go
  ensure_rust
  ensure_postgres
  ensure_redis
  export_path

  export PATH="/usr/local/go/bin:$HOME/.cargo/bin:$PATH"
  info "Running make env-check from $ROOT_DIR..."
  make -C "$ROOT_DIR" env-check
  info "Bootstrap complete — environment verified."
}

main "$@"
