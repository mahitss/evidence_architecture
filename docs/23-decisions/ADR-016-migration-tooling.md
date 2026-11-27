# ADR-016: Use `golang-migrate` CLI for Versioned SQL Migrations & Lock Phase 09 Schema Resolutions

- **Status:** Accepted
- **Date:** 2026-11-27
- **Deciders:** Product Owner, Implementer (MODE B)
- **Refs:** `docs/08-database/DATABASE_DESIGN.md`, `docs/02-requirements/EVIDENCE_INTEGRITY_SPEC.md`, `docs/24-setup/KNOWN_ENV_LIMITATIONS.md` (§2), `docs/24-setup/LOCAL_DEV.md`, `ADR-003`, `ADR-005`

## Context

Phase 09 (Foundation — Prompt 8a) introduces the PostgreSQL 15 core schema (`admin_users`, `cases`, `licensing_records`, `evidence_items`, `case_timeline_events`, `legal_review_records`, `audit_log`, `correction_requests`), mandatory legal/copyright `CHECK` constraints (including Q5), database triggers (`enforce_legal_review_before_publish`, append-only `audit_log`, immutable `legal_review_records`), and a deterministic dev seed migration—**before** `backend-go/` application code is scaffolded.

In addition, Phase A reconciliation between Prompt 8a and `docs/08-database/DATABASE_DESIGN.md` identified four schema/seed conflicts that were resolved by the Product Owner prior to writing migrations and must be permanently recorded here.

## Problem

We need a repeatable, version-tracked, forward-and-backward SQL migration tool that:
1. Installs cleanly in the Debian 12 gVisor sandbox without `apt-get` (avoiding dirty `dpkg` state per `KNOWN_ENV_LIMITATIONS.md §2`) and without Docker (`ADR-003`).
2. Works natively with raw PostgreSQL DDL (including `CHECK` constraints, partial indexes, and PL/pgSQL triggers).
3. Enforces explicit `up`/`down` reversibility and version tracking (`schema_migrations`).

## Alternatives

1. **Rejected — Raw `psql` Shell Scripts:**
   - *Reasoning:* No built-in `schema_migrations` version table, dirty-state protection, step-counted rollback (`down N`), or recovery (`force V`). Hand-rolling migration state tracking invites production drift.
2. **Rejected — Flyway:**
   - *Reasoning:* Requires a JVM runtime (hundreds of MB overhead on a small VPS and extra sandbox tooling) and restricts `undo` / rollback migrations to paid commercial tiers (violating `$0` paid tooling constraints).
3. **Rejected — Atlas (HCL / declarative schema):**
   - *Reasoning:* Introduces a custom DSL/cloud-oriented CLI ecosystem and hides exact SQL DDL for complex PL/pgSQL triggers (`enforce_legal_review_before_publish`, append-only `audit_log`) behind declarative diffing.
4. **Rejected — Embedded Go-Only Migration Runner at Prompt 8a:**
   - *Reasoning:* Prompt 8a prohibits writing Go application code or scaffolding `backend-go/`. Choosing `golang-migrate` CLI now preserves full compatibility with embedding `golang-migrate/migrate/v4` inside `backend-go` later if desired, without prematurely scaffolding Go packages.
5. **Selected — `golang-migrate` CLI (`v4`, installed via `go install -tags 'postgres'`):**
   - *Reasoning:* Single static Go binary installed via the verified Go 1.27.4 toolchain (`go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`), zero `apt-get` or JVM dependencies, first-class PostgreSQL DDL/trigger support, explicit `NNNNNN_name.{up,down}.sql` pairs, built-in `up`, `down`, `version`, and `force` commands.

## Selected

### 1. Migration Tooling & Directory Convention
- **Tool:** `golang-migrate` CLI (`v4`) compiled with `-tags 'postgres'` via `scripts/install-migrate.sh` (using `go install`, never `apt-get`).
- **Naming Convention:** Zero-padded 6-digit sequential pairs: `000001_init.up.sql` / `000001_init.down.sql`, `000002_seed_dev.up.sql` / `000002_seed_dev.down.sql`.
- **Wrapper Script:** `scripts/migrate.sh` exposing `up`, `down [N]`, `status`, `force <V>`, and `reset` with strict loopback (`127.0.0.1`) host guards mirroring `Makefile` (`ADR-005`).
- **TODO (Scaffolding Transition):** *"When `backend-go/` is scaffolded, move `migrations/` to `backend-go/migrations/` and update `scripts/migrate.sh` paths accordingly."*

### 2. PO-Approved Phase 09 Schema & Seed Resolutions (Locked In)
- **R1 (Conflict 1 — `Tier 3 × RECREATED` Block):** Keep `chk_no_recreated_tier3_tier4` (`CHECK (NOT (verification_tier IN (3, 4) AND license_status = 'RECREATED'))`) strictly enforced per `DATABASE_DESIGN.md` and `EVIDENCE_INTEGRITY_SPEC.md §3`. Seed the `RECREATED` dev evidence item with `verification_tier = 2` (never `3`).
- **R2 (Conflict 2 — `evidence_items.source_type` Enum):** Enforce the authoritative `DATABASE_DESIGN.md` enum (`gov_primary_record`, `court_opinion_or_order`, `third_party_exhibit`, `historical_public_domain`, `licensed_media`, `in_house_recreation`). Seed the 3 dev evidence items as:
  1. `gov_primary_record` + `PUBLIC_DOMAIN` + `verification_tier = 1`
  2. `third_party_exhibit` + `LICENSED` + `verification_tier = 2` + `licensing_record_id` FK (proves the legal path for third-party PACER exhibits alongside Q5)
  3. `in_house_recreation` + `RECREATED` + `verification_tier = 2` + non-empty `recreation_source_ids` and `recreation_author_note`
- **R3 (Conflict 3 — Fixed Dev Admin UUID):** Seed 1 internal `admin_users` row with fixed UUID `'00000000-0000-0000-0000-000000000001'`, email `'admin-test@truecrime.local'`, and an explicit non-authenticating dev-only placeholder hash (`dev seed — cannot authenticate in production`). Reference this UUID across `reviewer_id`, `created_by`, `updated_by`, and `recorded_by` to satisfy `UUID NOT NULL REFERENCES admin_users(id)`.
- **R4 (Conflict 4 — Migration Path & Publish-Gate Trigger):** Store initial migrations in root `migrations/000001_init.{up,down}.sql` until `backend-go/` is scaffolded (see TODO above). Implement the publish-gate trigger named `enforce_legal_review_before_publish` enforcing all five checks on `NEW.status = 'PUBLISHED'`:
  1. `NEW.published_legal_review_id IS NOT NULL`
  2. Matching row exists in `legal_review_records` (`id = NEW.published_legal_review_id AND case_id = NEW.id`)
  3. `reviewer_id IS NOT NULL`
  4. `reviewed_at IS NOT NULL`
  5. `decision = 'APPROVED' AND prohibited_checklist_passed = TRUE`
  *(Plus verifying non-empty `genuine_gaps` and cleared/completed evidence per `DATABASE_DESIGN.md`.)*

## Rationale

`golang-migrate` executes exact, auditable SQL without hiding PostgreSQL triggers or `CHECK` constraints behind an ORM or DSL, installs directly from the project's pinned Go toolchain, and enables automated forward → constraint-violation → backward → forward verification (`scripts/test-migrations.sh`). Locking in R1–R4 inside ADR-016 prevents future drift between `DATABASE_DESIGN.md`, `EVIDENCE_INTEGRITY_SPEC.md`, and migration SQL.

## Trade-offs

- Requires `go install -tags 'postgres'` after sandbox container recycles (automated cleanly by `scripts/install-migrate.sh` / `scripts/migrate.sh`).
- Pure SQL migrations require disciplined manual authoring of matching `.down.sql` scripts (verified automatically by `scripts/test-migrations.sh` T11–T13 and CI).

## Risks

- Running `migrate` without `-tags 'postgres'` produces a binary lacking the PostgreSQL driver.
- **Mitigation:** `scripts/install-migrate.sh` hardcodes `-tags 'postgres'`, and `scripts/migrate.sh` invokes it automatically when `migrate` is absent.
- Accidentally applying `000002_seed_dev.up.sql` in production.
- **Mitigation:** `scripts/migrate.sh` enforces `127.0.0.1` loopback guard for local dev; production deployment scripts (Prompt 10+) must separate or guard dev-only seed migrations.

## Consequences

- `migrations/000001_init.{up,down}.sql` and `migrations/000002_seed_dev.{up,down}.sql` establish the authoritative Phase 09 schema baseline.
- `docs/24-setup/LOCAL_DEV.md` documents local migration commands under "Database migrations".

## Reversal Strategy

Because all migrations are plain standard PostgreSQL SQL files, switching to `goose`, `dbmate`, or an embedded Go runner later requires only re-mapping the version header/table without rewriting any SQL DDL.
