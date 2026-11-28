SHELL := /bin/bash
.SHELLFLAGS := -euo pipefail -c
# =========================================================
# LOCAL DEVELOPMENT ONLY
# This Makefile runs Postgres (port 5432) and Redis (port 6379)
# on 127.0.0.1 with trust authentication. NEVER expose these
# ports outside localhost. NEVER use this configuration in
# staging, production, or any shared environment. For any
# non-local environment, use scram-sha-256 auth, TLS, and
# network isolation.
# =========================================================
.DEFAULT_GOAL := help

# run_as() execution model (used by services-up, services-down, services-status, db-reset):
#   - Branch 1: current user is root (id -u == 0) -> su to $(SVC_USER) to run command
#   - Branch 2: current user is non-root (id -u != 0) -> run command directly as current user (CI parity)

# Ensure verified Go 1.27.4 and rustup Cargo/Rustc resolve ahead of system /usr/bin
export PATH := /usr/local/go/bin:$(HOME)/.cargo/bin:$(PATH)

DATA_DIR    := $(CURDIR)/data
PG_BIN      := /usr/lib/postgresql/15/bin
PG_DATA     := $(DATA_DIR)/pgdata
PG_RUN      := $(DATA_DIR)/pgrun
PG_LOG      := $(DATA_DIR)/logs/postgres.log
PG_HOST     ?= 127.0.0.1
PG_PORT     ?= 5432
PG_USER     ?= postgres
PG_DB       ?= truecrime_dev

REDIS_DATA  := $(DATA_DIR)/redis
REDIS_LOG   := $(DATA_DIR)/logs/redis.log
REDIS_PID   := $(DATA_DIR)/redis/redis.pid
REDIS_HOST  ?= 127.0.0.1
REDIS_PORT  ?= 6379

MEILI_DIR     := $(DATA_DIR)/meili
MEILI_BIN     := $(MEILI_DIR)/meilisearch-bin
MEILI_DB      := $(MEILI_DIR)/data.ms
MEILI_PID     := $(MEILI_DIR)/meilisearch.pid
MEILI_LOG     := $(DATA_DIR)/logs/meilisearch.log
MEILI_HOST    ?= 127.0.0.1
MEILI_PORT    ?= 7700
MEILI_VERSION ?= v1.12.8

SVC_USER    ?= appuser

.PHONY: help env-check services-up services-down services-status meili-up meili-down db-reset clean

help: ## List available Makefile targets
	@echo "True Crime Investigation App — Local Dev Makefile"
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  help             List available targets (default)"
	@echo "  env-check        Print PATH ordering, tool versions, and verify Go/Cargo precedence"
	@echo "  services-up      Start local PostgreSQL and Redis as $(SVC_USER) using ./data/"
	@echo "  services-down    Stop local PostgreSQL and Redis gracefully"
	@echo "  services-status  Check PostgreSQL (pg_ctl status) and Redis (redis-cli ping)"
	@echo "  meili-up         Download (if needed) and start local Meilisearch on 127.0.0.1:7700"
	@echo "  meili-down       Stop local Meilisearch gracefully"
	@echo "  db-reset         Drop and recreate local dev database ($(PG_DB)) with confirmation"
	@echo "  clean            Remove build artifacts (preserves ./data/)"

env-check: ## Verify PATH ordering and pinned toolchain versions
	@echo "==> Checking effective PATH ordering..."
	@echo "PATH=$(PATH)"
	@GO_PATH="$$(command -v go || true)"; \
	CARGO_PATH="$$(command -v cargo || true)"; \
	RUSTC_PATH="$$(command -v rustc || true)"; \
	echo "Resolved go:    $${GO_PATH:-NOT FOUND}"; \
	echo "Resolved cargo: $${CARGO_PATH:-NOT FOUND}"; \
	echo "Resolved rustc: $${RUSTC_PATH:-NOT FOUND}"; \
	if [[ "$$GO_PATH" != "/usr/local/go/bin/go" ]]; then \
		echo "ERROR: Expected /usr/local/go/bin/go to precede /usr/bin/go, got '$${GO_PATH:-none}'" >&2; \
		exit 1; \
	fi; \
	if [[ "$$CARGO_PATH" != "$(HOME)/.cargo/bin/cargo" ]]; then \
		echo "ERROR: Expected $(HOME)/.cargo/bin/cargo to resolve first, got '$${CARGO_PATH:-none}'" >&2; \
		exit 1; \
	fi; \
	if [[ "$$RUSTC_PATH" != "$(HOME)/.cargo/bin/rustc" ]]; then \
		echo "ERROR: Expected $(HOME)/.cargo/bin/rustc to resolve first, got '$${RUSTC_PATH:-none}'" >&2; \
		exit 1; \
	fi; \
	echo "OK: /usr/local/go/bin precedes /usr/bin for go, and $(HOME)/.cargo/bin precedes for cargo/rustc."
	@echo ""
	@echo "==> Checking tool versions..."
	@git --version
	@go version
	@rustc --version
	@cargo --version
	@echo "node $$(node --version) / npm $$(npm --version)"
	@$(PG_BIN)/postgres --version
	@redis-server --version
	@echo "OK: All required local toolchains verified."

services-up: ## Start PostgreSQL and Redis directly as $(SVC_USER) (local dev only)
	@echo "==> Enforcing local-dev-only loopback and port guards..."
	@if [[ "$(PG_HOST)" != "127.0.0.1" ]] || [[ -n "$${PGHOST:-}" && "$${PGHOST}" != "127.0.0.1" ]]; then \
		echo "ERROR: Refusing to start PostgreSQL with trust auth on non-loopback host (PG_HOST='$(PG_HOST)', PGHOST='$${PGHOST:-unset}'). Must be 127.0.0.1." >&2; \
		exit 1; \
	fi
	@if [[ "$(PG_PORT)" != "5432" ]] || [[ -n "$${PGPORT:-}" && "$${PGPORT}" != "5432" ]]; then \
		echo "ERROR: Refusing to start PostgreSQL on non-standard local dev port (PG_PORT='$(PG_PORT)', PGPORT='$${PGPORT:-unset}'). Must be 5432." >&2; \
		exit 1; \
	fi
	@if [[ "$(REDIS_HOST)" != "127.0.0.1" ]]; then \
		echo "ERROR: Refusing to start unauthenticated Redis on non-loopback host (REDIS_HOST='$(REDIS_HOST)'). Must be 127.0.0.1." >&2; \
		exit 1; \
	fi
	@if [[ "$(REDIS_PORT)" != "6379" ]]; then \
		echo "ERROR: Refusing to start Redis on non-standard local dev port (REDIS_PORT='$(REDIS_PORT)'). Must be 6379." >&2; \
		exit 1; \
	fi
	@echo "==> Ensuring local data and log directories under $(DATA_DIR)..."
	@mkdir -p "$(PG_DATA)" "$(PG_RUN)" "$(DATA_DIR)/logs" "$(REDIS_DATA)"
	@if [[ "$$(id -u)" -eq 0 ]]; then \
		chown -R "$(SVC_USER):$(SVC_USER)" "$(DATA_DIR)"; \
	fi
	@echo "==> Starting PostgreSQL 15 on $(PG_HOST):$(PG_PORT)..."
	@run_as() { \
		if [[ "$$(id -u)" -eq 0 ]]; then \
			su -s /bin/bash "$(SVC_USER)" -c "$$1"; \
		else \
			bash -c "$$1"; \
		fi; \
	}; \
	if [[ ! -f "$(PG_DATA)/PG_VERSION" ]]; then \
		echo "Initializing new PostgreSQL data directory at $(PG_DATA)..."; \
		run_as "'$(PG_BIN)/initdb' -D '$(PG_DATA)' -U '$(PG_USER)' --auth=trust"; \
	fi; \
	if run_as "'$(PG_BIN)/pg_ctl' -D '$(PG_DATA)' status" >/dev/null 2>&1; then \
		echo "PostgreSQL is already running."; \
	else \
		run_as "'$(PG_BIN)/pg_ctl' -D '$(PG_DATA)' -l '$(PG_LOG)' -o '-k $(PG_RUN) -h $(PG_HOST) -p $(PG_PORT)' start"; \
	fi; \
	if ! run_as "psql -h '$(PG_HOST)' -p '$(PG_PORT)' -U '$(PG_USER)' -tAc \"SELECT 1 FROM pg_database WHERE datname='$(PG_DB)'\"" | grep -qx 1; then \
		echo "Creating local dev database '$(PG_DB)'..."; \
		run_as "'$(PG_BIN)/createdb' -h '$(PG_HOST)' -p '$(PG_PORT)' -U '$(PG_USER)' '$(PG_DB)'"; \
	fi
	@echo "==> Starting Redis 7 on $(REDIS_HOST):$(REDIS_PORT)..."
	@if redis-cli -h "$(REDIS_HOST)" -p "$(REDIS_PORT)" ping >/dev/null 2>&1; then \
		echo "Redis is already running."; \
	else \
		if [[ "$$(id -u)" -eq 0 ]]; then \
			su -s /bin/bash "$(SVC_USER)" -c "redis-server --daemonize yes --bind '$(REDIS_HOST)' --port '$(REDIS_PORT)' --dir '$(REDIS_DATA)' --pidfile '$(REDIS_PID)' --logfile '$(REDIS_LOG)'"; \
		else \
			redis-server --daemonize yes --bind "$(REDIS_HOST)" --port "$(REDIS_PORT)" --dir "$(REDIS_DATA)" --pidfile "$(REDIS_PID)" --logfile "$(REDIS_LOG)"; \
		fi; \
		for i in 1 2 3 4 5 6 7 8 9 10; do \
			if redis-cli -h "$(REDIS_HOST)" -p "$(REDIS_PORT)" ping >/dev/null 2>&1; then break; fi; \
			sleep 0.2; \
		done; \
		redis-cli -h "$(REDIS_HOST)" -p "$(REDIS_PORT)" ping | grep -qx PONG; \
	fi
	@echo "OK: Local PostgreSQL ($(PG_HOST):$(PG_PORT)) and Redis ($(REDIS_HOST):$(REDIS_PORT)) are up."

services-down: ## Stop local PostgreSQL and Redis gracefully
	@echo "==> Stopping PostgreSQL..."
	@run_as() { \
		if [[ "$$(id -u)" -eq 0 ]]; then \
			su -s /bin/bash "$(SVC_USER)" -c "$$1"; \
		else \
			bash -c "$$1"; \
		fi; \
	}; \
	if [[ -f "$(PG_DATA)/PG_VERSION" ]] && run_as "'$(PG_BIN)/pg_ctl' -D '$(PG_DATA)' status" >/dev/null 2>&1; then \
		run_as "'$(PG_BIN)/pg_ctl' -D '$(PG_DATA)' -m fast stop"; \
	else \
		echo "PostgreSQL is not running."; \
	fi
	@echo "==> Stopping Redis..."
	@if redis-cli -h 127.0.0.1 -p "$(REDIS_PORT)" ping >/dev/null 2>&1; then \
		redis-cli -h 127.0.0.1 -p "$(REDIS_PORT)" shutdown save; \
		for i in 1 2 3 4 5 6 7 8 9 10; do \
			if ! redis-cli -h 127.0.0.1 -p "$(REDIS_PORT)" ping >/dev/null 2>&1; then break; fi; \
			sleep 0.2; \
		done; \
		echo "Redis stopped."; \
	else \
		echo "Redis is not running."; \
	fi
	@echo "OK: Local services stopped."

services-status: ## Show PostgreSQL and Redis status (exits non-zero if either is down)
	@echo "==> Checking PostgreSQL status..."
	@PG_OK=0; REDIS_OK=0; \
	run_as() { \
		if [[ "$$(id -u)" -eq 0 ]]; then \
			su -s /bin/bash "$(SVC_USER)" -c "$$1"; \
		else \
			bash -c "$$1"; \
		fi; \
	}; \
	if [[ -f "$(PG_DATA)/PG_VERSION" ]] && run_as "'$(PG_BIN)/pg_ctl' -D '$(PG_DATA)' status"; then \
		PG_OK=1; \
	else \
		echo "PostgreSQL: STOPPED"; \
	fi; \
	echo "==> Checking Redis status..."; \
	if REDIS_PONG="$$(redis-cli -h 127.0.0.1 -p "$(REDIS_PORT)" ping 2>&1)"; then \
		echo "Redis (127.0.0.1:$(REDIS_PORT)): $$REDIS_PONG"; \
		if [[ "$$REDIS_PONG" == "PONG" ]]; then REDIS_OK=1; fi; \
	else \
		echo "Redis (127.0.0.1:$(REDIS_PORT)): STOPPED ($$REDIS_PONG)"; \
	fi; \
	if [[ "$$PG_OK" -ne 1 || "$$REDIS_OK" -ne 1 ]]; then \
		echo "ERROR: One or more services are not running." >&2; \
		exit 1; \
	fi; \
	echo "OK: Both PostgreSQL and Redis are running."

meili-up: ## Start local Meilisearch on 127.0.0.1:7700 (downloads binary if absent)
	@echo "==> Enforcing local-dev-only loopback and port guards for Meilisearch..."
	@if [[ "$(MEILI_HOST)" != "127.0.0.1" ]]; then \
		echo "ERROR: Refusing to start local Meilisearch on non-loopback host (MEILI_HOST='$(MEILI_HOST)'). Must be 127.0.0.1." >&2; \
		exit 1; \
	fi
	@if [[ "$(MEILI_PORT)" != "7700" ]]; then \
		echo "ERROR: Refusing to start local Meilisearch on non-standard local dev port (MEILI_PORT='$(MEILI_PORT)'). Must be 7700." >&2; \
		exit 1; \
	fi
	@mkdir -p "$(MEILI_DIR)" "$(DATA_DIR)/logs"
	@if curl -fsS "http://$(MEILI_HOST):$(MEILI_PORT)/health" >/dev/null 2>&1; then \
		echo "Meilisearch is already running on http://$(MEILI_HOST):$(MEILI_PORT)."; \
		exit 0; \
	fi
	@BIN="$$(command -v meilisearch || true)"; \
	if [[ -z "$$BIN" ]]; then \
		if [[ ! -x "$(MEILI_BIN)" ]]; then \
			echo "==> Downloading Meilisearch $(MEILI_VERSION) Linux amd64 binary to $(MEILI_BIN)..."; \
			curl -fsSL "https://github.com/meilisearch/meilisearch/releases/download/$(MEILI_VERSION)/meilisearch-linux-amd64" -o "$(MEILI_BIN).tmp"; \
			chmod +x "$(MEILI_BIN).tmp"; \
			mv "$(MEILI_BIN).tmp" "$(MEILI_BIN)"; \
		fi; \
		BIN="$(MEILI_BIN)"; \
	fi; \
	echo "==> Starting Meilisearch ($$BIN) on $(MEILI_HOST):$(MEILI_PORT)..."; \
	if [[ "$$(id -u)" -eq 0 ]]; then \
		chown -R "$(SVC_USER):$(SVC_USER)" "$(MEILI_DIR)" "$(DATA_DIR)/logs"; \
		su -s /bin/bash "$(SVC_USER)" -c "nohup '$$BIN' --http-addr '$(MEILI_HOST):$(MEILI_PORT)' --db-path '$(MEILI_DB)' --env development --no-analytics >>'$(MEILI_LOG)' 2>&1 & echo \$$! > '$(MEILI_PID)'"; \
	else \
		nohup "$$BIN" --http-addr "$(MEILI_HOST):$(MEILI_PORT)" --db-path "$(MEILI_DB)" --env development --no-analytics >>"$(MEILI_LOG)" 2>&1 & echo $$! > "$(MEILI_PID)"; \
	fi; \
	for i in $$(seq 1 30); do \
		if curl -fsS "http://$(MEILI_HOST):$(MEILI_PORT)/health" >/dev/null 2>&1; then \
			echo "OK: Local Meilisearch is up at http://$(MEILI_HOST):$(MEILI_PORT)."; \
			exit 0; \
		fi; \
		sleep 0.25; \
	done; \
	echo "ERROR: Meilisearch failed to become healthy within timeout. Check $(MEILI_LOG)." >&2; \
	exit 1

meili-down: ## Stop local Meilisearch gracefully
	@echo "==> Stopping Meilisearch..."
	@if [[ -f "$(MEILI_PID)" ]]; then \
		PID="$$(cat "$(MEILI_PID)")"; \
		if kill -0 "$$PID" 2>/dev/null; then \
			kill "$$PID" 2>/dev/null || true; \
			for i in $$(seq 1 20); do \
				if ! kill -0 "$$PID" 2>/dev/null; then break; fi; \
				sleep 0.2; \
			done; \
			if kill -0 "$$PID" 2>/dev/null; then kill -9 "$$PID" 2>/dev/null || true; fi; \
			echo "Meilisearch stopped (PID $$PID)."; \
		else \
			echo "Meilisearch PID $$PID is not running."; \
		fi; \
		rm -f "$(MEILI_PID)"; \
	else \
		echo "Meilisearch is not running."; \
	fi
	@echo "OK: Meilisearch stopped."

db-reset: ## Drop and recreate local dev database ($(PG_DB)) with confirmation prompt
	@echo "==> Enforcing local-dev-only loopback and port guards for db-reset..."
	@if [[ "$(PG_HOST)" != "127.0.0.1" ]] || [[ -n "$${PGHOST:-}" && "$${PGHOST}" != "127.0.0.1" ]]; then \
		echo "ERROR: Refusing to reset PostgreSQL on non-loopback host (PG_HOST='$(PG_HOST)', PGHOST='$${PGHOST:-unset}'). Must be 127.0.0.1." >&2; \
		exit 1; \
	fi
	@if [[ "$(PG_PORT)" != "5432" ]] || [[ -n "$${PGPORT:-}" && "$${PGPORT}" != "5432" ]]; then \
		echo "ERROR: Refusing to reset PostgreSQL on non-standard local dev port (PG_PORT='$(PG_PORT)', PGPORT='$${PGPORT:-unset}'). Must be 5432." >&2; \
		exit 1; \
	fi
	@echo "==> Requesting confirmation to reset local dev database '$(PG_DB)'..."
	@read -r -p "This will DROP and RECREATE local dev database '$(PG_DB)'. Type 'yes' to continue: " ans; \
	if [[ "$$ans" != "yes" ]]; then \
		echo "Aborted database reset." >&2; \
		exit 1; \
	fi
	@echo "==> Dropping and recreating '$(PG_DB)' on $(PG_HOST):$(PG_PORT)..."
	@run_as() { \
		if [[ "$$(id -u)" -eq 0 ]]; then \
			su -s /bin/bash "$(SVC_USER)" -c "$$1"; \
		else \
			bash -c "$$1"; \
		fi; \
	}; \
	run_as "'$(PG_BIN)/dropdb' --if-exists -h '$(PG_HOST)' -p '$(PG_PORT)' -U '$(PG_USER)' '$(PG_DB)'"; \
	run_as "'$(PG_BIN)/createdb' -h '$(PG_HOST)' -p '$(PG_PORT)' -U '$(PG_USER)' '$(PG_DB)'"
	@echo "OK: Local dev database '$(PG_DB)' has been reset."

clean: ## Remove build artifacts without deleting ./data/
	@echo "==> Removing build artifacts (preserving $(DATA_DIR))..."
	@rm -rf dist build .next coverage \
		backend-go/bin backend-go/dist \
		processor-rust/target \
		admin-web/dist admin-web/build \
		mobile-web/dist mobile-web/build
	@echo "OK: Build artifacts cleaned; $(DATA_DIR) untouched."
