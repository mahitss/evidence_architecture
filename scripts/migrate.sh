#!/usr/bin/env bash
# =============================================================================
# scripts/migrate.sh — Local PostgreSQL Migration Runner (golang-migrate v4)
#
# Subcommands:
#   up [N]     Apply all pending migrations (or N steps if specified)
#   down [N]   Roll back N migrations (default: 1)
#   status     Show current migration version
#   force <V>  Force migration version V (for dirty-state recovery)
#   reset      Roll back all migrations and re-apply all (dev only, asks "yes")
#
# LOCAL DEV SAFETY GUARDS (mirrors Makefile / ADR-005):
#   - Refuses to execute if PGHOST, PG_HOST, or the host inside DATABASE_URL
#     is not 127.0.0.1.
#   - Refuses `reset` unless the user interactively types "yes".
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
MIGRATIONS_DIR="${MIGRATIONS_DIR:-${ROOT_DIR}/migrations}"

export PATH="/usr/local/go/bin:${HOME}/go/bin:${PATH}"

DEFAULT_DATABASE_URL="postgres://postgres@127.0.0.1:5432/truecrime_dev?sslmode=disable"
DATABASE_URL="${DATABASE_URL:-${DEFAULT_DATABASE_URL}}"

usage() {
  cat <<'EOF'
Usage: bash scripts/migrate.sh <subcommand> [args]

Subcommands:
  up [N]       Apply all pending migrations (or N migrations if N is given)
  down [N]     Roll back N migrations (default: 1)
  status       Show current migration version
  force <V>    Force migration version to V (for recovery)
  reset        Roll back all migrations then apply all (dev only; prompts for 'yes')
EOF
}

extract_url_host() {
  local url="$1"
  # Strip scheme (postgres:// or postgresql://)
  local rest="${url#*://}"
  # Strip credentials (user[:pass]@) if present
  if [[ "$rest" == *"@"* ]]; then
    rest="${rest#*@}"
  fi
  # Strip path/query (/db?...)
  rest="${rest%%/*}"
  rest="${rest%%\?*}"
  # Strip port (:5432)
  echo "${rest%%:*}"
}

enforce_loopback_guards() {
  echo "==> Enforcing local-dev-only loopback host guards (127.0.0.1)..."
  if [[ -n "${PGHOST:-}" && "${PGHOST}" != "127.0.0.1" ]]; then
    echo "ERROR: Refusing to run migrations when PGHOST='${PGHOST}' (must be 127.0.0.1)." >&2
    exit 1
  fi
  if [[ -n "${PG_HOST:-}" && "${PG_HOST}" != "127.0.0.1" ]]; then
    echo "ERROR: Refusing to run migrations when PG_HOST='${PG_HOST}' (must be 127.0.0.1)." >&2
    exit 1
  fi

  local url_host
  url_host="$(extract_url_host "${DATABASE_URL}")"
  if [[ "${url_host}" != "127.0.0.1" ]]; then
    echo "ERROR: Refusing to run migrations against non-loopback DATABASE_URL host '${url_host}' (must be 127.0.0.1)." >&2
    exit 1
  fi
  echo "OK: Loopback guard passed (host=127.0.0.1)."
}

ensure_migrate_bin() {
  if ! command -v migrate >/dev/null 2>&1; then
    echo "==> golang-migrate not found on PATH; running scripts/install-migrate.sh..."
    bash "${SCRIPT_DIR}/install-migrate.sh"
  fi
  MIGRATE_BIN="$(command -v migrate)"
  echo "==> Using migrate binary: ${MIGRATE_BIN}"
}

main() {
  if [[ $# -lt 1 ]]; then
    usage >&2
    exit 1
  fi

  local cmd="$1"
  shift

  case "$cmd" in
    -h|--help|help)
      usage
      exit 0
      ;;
  esac

  enforce_loopback_guards
  ensure_migrate_bin

  if [[ ! -d "${MIGRATIONS_DIR}" ]]; then
    echo "ERROR: Migrations directory not found: ${MIGRATIONS_DIR}" >&2
    exit 1
  fi

  echo "==> Migrations path: ${MIGRATIONS_DIR}"

  case "$cmd" in
    up)
      if [[ $# -ge 1 ]]; then
        local steps="$1"
        echo "==> Applying ${steps} migration(s) up..."
        "${MIGRATE_BIN}" -path "${MIGRATIONS_DIR}" -database "${DATABASE_URL}" up "${steps}"
      else
        echo "==> Applying all pending migrations up..."
        "${MIGRATE_BIN}" -path "${MIGRATIONS_DIR}" -database "${DATABASE_URL}" up
      fi
      echo "OK: migrate up completed."
      ;;
    down)
      local steps="${1:-1}"
      echo "==> Rolling back ${steps} migration(s)..."
      "${MIGRATE_BIN}" -path "${MIGRATIONS_DIR}" -database "${DATABASE_URL}" down "${steps}"
      echo "OK: migrate down ${steps} completed."
      ;;
    status)
      echo "==> Checking current migration version..."
      "${MIGRATE_BIN}" -path "${MIGRATIONS_DIR}" -database "${DATABASE_URL}" version
      ;;
    force)
      if [[ $# -lt 1 ]]; then
        echo "ERROR: 'force' requires a version argument V." >&2
        exit 1
      fi
      local ver="$1"
      echo "==> Forcing migration version to ${ver}..."
      "${MIGRATE_BIN}" -path "${MIGRATIONS_DIR}" -database "${DATABASE_URL}" force "${ver}"
      echo "OK: forced version to ${ver}."
      ;;
    reset)
      echo "==> Resetting all migrations on local dev database (PG_HOST=127.0.0.1)..."
      local ans=""
      read -r -p "This will roll back ALL migrations and re-apply them on 127.0.0.1. Type 'yes' to continue: " ans
      if [[ "${ans}" != "yes" ]]; then
        echo "Aborted migration reset." >&2
        exit 1
      fi
      echo "==> Step 1/2: Rolling back all migrations (down -all)..."
      "${MIGRATE_BIN}" -path "${MIGRATIONS_DIR}" -database "${DATABASE_URL}" down -all
      echo "==> Step 2/2: Re-applying all migrations (up)..."
      "${MIGRATE_BIN}" -path "${MIGRATIONS_DIR}" -database "${DATABASE_URL}" up
      echo "OK: migration reset completed."
      ;;
    *)
      echo "ERROR: Unknown subcommand '${cmd}'." >&2
      usage >&2
      exit 1
      ;;
  esac
}

main "$@"
