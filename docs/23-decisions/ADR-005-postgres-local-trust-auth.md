# ADR-005: Restrict PostgreSQL `--auth=trust` Strictly to Local Dev Sandbox

- **Status:** Accepted
- **Date:** 2026-11-26
- **Deciders:** Product Owner, Architect (MODE A)
- **Refs:** Audit Flag 3, ADR-003, `Makefile`, `docs/24-setup/LOCAL_DEV.md`

## Context

In Prompt 2 and Prompt 3, local PostgreSQL 15 cluster initialization (`initdb`) was configured with `--auth=trust` bound to `127.0.0.1:5432` to allow frictionless, zero-hardcoded-secret startup inside the single-tenant gVisor development sandbox via `make services-up`.

## Problem

PostgreSQL `trust` authentication grants full superuser access without credentials to any client able to reach the listening socket. If this configuration were ever copied into a staging, production, or network-reachable host, or bound to a non-loopback interface (`0.0.0.0`, LAN, or public IP), it would enable immediate unauthenticated data exfiltration, destructive schema tampering, and arbitrary SQL execution.

## Alternatives

1. **Rejected — Hardcode a dev password inside `Makefile` and configure `scram-sha-256` locally:**
   - *Reasoning:* Embedding static credentials in version-controlled `Makefile` scripts violates our no-hardcoded-secrets rule and trains contributors to reuse default passwords across environments.
2. **Rejected — Require every developer to manually create `.env` secrets and configure `pg_hba.conf` before `make services-up` can run:**
   - *Reasoning:* Adds unnecessary friction in an ephemeral, single-user gVisor sandbox where `127.0.0.1` is not network-routable, slowing down automated test/reset cycles without improving threat protection inside the isolated sandbox.
3. **Selected — Permit `--auth=trust` exclusively on loopback (`127.0.0.1:5432`) inside the local dev sandbox, protected by loud warnings and hard `Makefile` host/port guards, while mandating `scram-sha-256` + TLS for all deployed environments:**
   - *Reasoning:* Preserves zero-secret local ergonomics while establishing explicit programmatic guardrails and unambiguous security policy for every non-local environment.

## Selected

1. `--auth=trust` is permitted **ONLY** in the local development sandbox via `Makefile`.
2. Any deployed, shared, staging, or production environment **MUST** enforce `scram-sha-256` (or stronger, such as mTLS/IAM auth), TLS in transit, least-privilege application roles, and private network isolation.
3. `Makefile` enforces a hard pre-start guard in `services-up` refusing to launch PostgreSQL if `PG_HOST`/`PGHOST` is not `127.0.0.1` or `PG_PORT`/`PGPORT` is not `5432` (and refusing to launch Redis if `REDIS_HOST` is not `127.0.0.1` or `REDIS_PORT` is not `6379`).
4. Both `Makefile` and `docs/24-setup/LOCAL_DEV.md` carry prominent `LOCAL DEVELOPMENT ONLY` / `NEVER DO THIS` warnings detailing exact failure modes.

## Rationale

Defense in depth requires both clear documentation and executable guardrails. The `Makefile` guard ensures an accidental environment override (e.g., `PGHOST=0.0.0.0` or `PGHOST=prod-db.internal` or non-standard port forwarding) fails closed before any database process starts or any destructive command executes.

## Trade-offs

- Local developers cannot override `PG_PORT` or `PG_HOST` to run multiple parallel local clusters via the default `Makefile` without intentionally bypassing or updating this ADR.
- Local tests will not exercise password-based authentication handshakes against Postgres unless separately tested in CI against a `scram-sha-256` container.

## Risks

- A contributor copying `initdb --auth=trust` from `Makefile` into a deployment script or Dockerfile despite warnings.
- Local tools or processes running on the same container as `appuser`/`root` can connect to `127.0.0.1:5432` without a password.

## Consequences

- `Makefile` includes the top-level `LOCAL DEVELOPMENT ONLY` warning banner (`Makefile:3-11`), runtime host/port guards on `services-up` (`Makefile:88-104` covering `PG_HOST`/`PGHOST`, `PG_PORT`/`PGPORT`, `REDIS_HOST`, `REDIS_PORT`), and matching pre-prompt host/port guards on `db-reset` (`Makefile:204-212` covering `PG_HOST`/`PGHOST` and `PG_PORT`/`PGPORT`).
- `docs/24-setup/LOCAL_DEV.md` includes a dedicated `NEVER DO THIS` section explaining the exact failure modes (data exfiltration, unauthorized schema changes, credential theft).
- CI and staging/production infrastructure specs (`docs/10-security/`, `docs/13-infrastructure/`) MUST mandate `scram-sha-256` and reject `trust` in `pg_hba.conf`.

## Reversal Strategy

To eliminate `trust` auth even locally, generate a per-machine ephemeral password in gitignored `.env.local` (never committed), update `initdb` in `Makefile` to use `--auth-host=scram-sha-256 --pwfile=...`, update `LOCAL_DEV.md`, and mark this ADR Superseded.
