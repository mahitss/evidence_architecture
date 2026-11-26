# Local development Makefile — True Crime Investigation App
# NOTE: All defaults below (trust auth, localhost ports, local data dirs) are
# strictly for LOCAL DEVELOPMENT ONLY and are NOT for production use.

SHELL := /bin/bash
.SHELLFLAGS := -euo pipefail -c
.DEFAULT_GOAL := help

# Ensure verified Go 1.27.4 and rustup Cargo/Rustc resolve ahead of system /usr/bin
export PATH := /usr/local/go/bin:$(HOME)/.cargo/bin:$(PATH)

DATA_DIR    := $(CURDIR)/data
PG_BIN      := /usr/lib/postgresql/15/bin
PG_DATA     := $(DATA_DIR)/pgdata
PG_RUN      := $(DATA_DIR)/pgrun
PG_LOG      := $(DATA_DIR)/logs/postgres.log
PG_PORT     ?= 5432
PG_USER     ?= postgres
PG_DB       ?= truecrime_dev

REDIS_DATA  := $(DATA_DIR)/redis
REDIS_LOG   := $(DATA_DIR)/logs/redis.log
REDIS_PID   := $(DATA_DIR)/redis/redis.pid
REDIS_PORT  ?= 6379

SVC_USER    ?= appuser

.PHONY: help env-check services-up services-down services-status db-reset clean

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
	@echo "==> Ensuring local data and log directories under $(DATA_DIR)..."
	@mkdir -p "$(PG_DATA)" "$(PG_RUN)" "$(DATA_DIR)/logs" "$(REDIS_DATA)"
	@if [[ "$$(id -u)" -eq 0 ]]; then \
		chown -R "$(SVC_USER):$(SVC_USER)" "$(DATA_DIR)"; \
	fi
	@echo "==> Starting PostgreSQL 15 on 127.0.0.1:$(PG_PORT)..."
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
		run_as "'$(PG_BIN)/pg_ctl' -D '$(PG_DATA)' -l '$(PG_LOG)' -o '-k $(PG_RUN) -h 127.0.0.1 -p $(PG_PORT)' start"; \
	fi; \
	if ! run_as "psql -h 127.0.0.1 -p '$(PG_PORT)' -U '$(PG_USER)' -tAc \"SELECT 1 FROM pg_database WHERE datname='$(PG_DB)'\"" | grep -qx 1; then \
		echo "Creating local dev database '$(PG_DB)'..."; \
		run_as "'$(PG_BIN)/createdb' -h 127.0.0.1 -p '$(PG_PORT)' -U '$(PG_USER)' '$(PG_DB)'"; \
	fi
	@echo "==> Starting Redis 7 on 127.0.0.1:$(REDIS_PORT)..."
	@if redis-cli -h 127.0.0.1 -p "$(REDIS_PORT)" ping >/dev/null 2>&1; then \
		echo "Redis is already running."; \
	else \
		if [[ "$$(id -u)" -eq 0 ]]; then \
			su -s /bin/bash "$(SVC_USER)" -c "redis-server --daemonize yes --bind 127.0.0.1 --port '$(REDIS_PORT)' --dir '$(REDIS_DATA)' --pidfile '$(REDIS_PID)' --logfile '$(REDIS_LOG)'"; \
		else \
			redis-server --daemonize yes --bind 127.0.0.1 --port "$(REDIS_PORT)" --dir "$(REDIS_DATA)" --pidfile "$(REDIS_PID)" --logfile "$(REDIS_LOG)"; \
		fi; \
		for i in 1 2 3 4 5 6 7 8 9 10; do \
			if redis-cli -h 127.0.0.1 -p "$(REDIS_PORT)" ping >/dev/null 2>&1; then break; fi; \
			sleep 0.2; \
		done; \
		redis-cli -h 127.0.0.1 -p "$(REDIS_PORT)" ping | grep -qx PONG; \
	fi
	@echo "OK: Local PostgreSQL ($(PG_PORT)) and Redis ($(REDIS_PORT)) are up."

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

db-reset: ## Drop and recreate local dev database ($(PG_DB)) with confirmation prompt
	@echo "==> Requesting confirmation to reset local dev database '$(PG_DB)'..."
	@read -r -p "This will DROP and RECREATE local dev database '$(PG_DB)'. Type 'yes' to continue: " ans; \
	if [[ "$$ans" != "yes" ]]; then \
		echo "Aborted database reset." >&2; \
		exit 1; \
	fi
	@echo "==> Dropping and recreating '$(PG_DB)' on 127.0.0.1:$(PG_PORT)..."
	@run_as() { \
		if [[ "$$(id -u)" -eq 0 ]]; then \
			su -s /bin/bash "$(SVC_USER)" -c "$$1"; \
		else \
			bash -c "$$1"; \
		fi; \
	}; \
	run_as "'$(PG_BIN)/dropdb' --if-exists -h 127.0.0.1 -p '$(PG_PORT)' -U '$(PG_USER)' '$(PG_DB)'"; \
	run_as "'$(PG_BIN)/createdb' -h 127.0.0.1 -p '$(PG_PORT)' -U '$(PG_USER)' '$(PG_DB)'"
	@echo "OK: Local dev database '$(PG_DB)' has been reset."

clean: ## Remove build artifacts without deleting ./data/
	@echo "==> Removing build artifacts (preserving $(DATA_DIR))..."
	@rm -rf dist build .next coverage \
		backend-go/bin backend-go/dist \
		processor-rust/target \
		admin-web/dist admin-web/build \
		mobile-web/dist mobile-web/build
	@echo "OK: Build artifacts cleaned; $(DATA_DIR) untouched."
