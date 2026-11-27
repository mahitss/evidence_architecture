#!/usr/bin/env bash
# =============================================================================
# scripts/install-migrate.sh — Install golang-migrate CLI with PostgreSQL driver
# Uses `go install` (NEVER apt-get, per KNOWN_ENV_LIMITATIONS.md §2 & ADR-016).
# =============================================================================
set -euo pipefail

export PATH="/usr/local/go/bin:${HOME}/go/bin:${PATH}"

if ! command -v go >/dev/null 2>&1; then
  echo "ERROR: go toolchain not found. Run 'bash scripts/bootstrap-env.sh' first." >&2
  exit 1
fi

MIGRATE_BIN="$(command -v migrate || true)"
if [[ -n "$MIGRATE_BIN" ]]; then
  echo "golang-migrate already installed at ${MIGRATE_BIN} ($("${MIGRATE_BIN}" -version 2>&1 || true))."
  exit 0
fi

echo "==> Installing golang-migrate (with postgres driver tag) via go install..."
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

MIGRATE_BIN="$(command -v migrate || true)"
if [[ -z "$MIGRATE_BIN" ]]; then
  echo "ERROR: migrate binary not found on PATH after go install (expected in $(go env GOPATH)/bin)." >&2
  exit 1
fi

echo "OK: Installed golang-migrate at ${MIGRATE_BIN} ($("${MIGRATE_BIN}" -version 2>&1 || true))."
