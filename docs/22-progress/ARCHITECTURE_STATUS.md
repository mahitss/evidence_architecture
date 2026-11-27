# Phase 06 Architecture Status & Readiness Summary

- **Document Path:** `docs/22-progress/ARCHITECTURE_STATUS.md`
- **Phase:** Phase 06 — Architecture (Charter PART 05)
- **Date:** 2026-11-27
- **Status:** Complete — Ready for PO Audit prior to Prompt 8 (Foundation & Scaffolding)

---

## 1. Charter PART 06 Domain Coverage Matrix

| Charter Domain Directory | Status at Phase 06 | Key Artifacts / Notes |
|---|---|---|
| `docs/00-overview/` | Documented (Baseline) | `README.md` |
| `docs/01-product/` | **Documented (Phase 03)** | `PRODUCT_REQUIREMENTS.md`, `VISION.md`, `PERSONAS.md`, `ROADMAP.md`, `BACKLOG.md` |
| `docs/02-requirements/` | **Documented (Phase 03 + Q5)** | `EVIDENCE_INTEGRITY_SPEC.md`, `FEATURE_CATALOG.md`, `PERMISSION_MATRIX.md`, `USER_STORIES.md` |
| `docs/03-research/` | Deferred | Case-level primary-source research dossiers assembled separately per case (Cases 1–10). |
| `docs/04-design/` | Deferred | Mobile PWA & Admin SPA wireframes / UI design system (prior to frontend implementation). |
| `docs/05-architecture/` | **Documented (Phase 06)** | `SYSTEM_CONTEXT.md`, `HIGH_LEVEL_DESIGN.md`, `LOW_LEVEL_DESIGN.md`, `SEQUENCE_DIAGRAMS.md`, `DOMAIN_MODEL.md` |
| `docs/06-frontend/` | Deferred | Component specs for `mobile-web` PWA & `admin-web` SPA (`ADR-006`, `ADR-014`) — Phase 08/frontend sprint. |
| `docs/07-backend/` | Deferred | Go/Rust service implementation guides — scaffolded in Prompt 8 based on `LOW_LEVEL_DESIGN.md`. |
| `docs/08-database/` | **Documented (Phase 06)** | `DATABASE_DESIGN.md` (full ERD, Q5 `CHECK`, legal-review trigger, append-only `audit_log`, migration strategy). |
| `docs/09-api/` | **Documented (Phase 06)** | `openapi.yaml` (OpenAPI 3.1 covering all 13 v1.0 endpoints, error/pagination envelopes, rate limits, idempotency). |
| `docs/10-security/` | **Documented (Phase 06)** | `SECURITY_ARCHITECTURE.md` (trust boundaries, STRIDE T1–T8, JWT rotation/TTL/rate-limit policy). |
| `docs/11-privacy/` | **N/A at MVP (Deferred to v1.1)** | Consumer user accounts deferred (`ADR-007`), EU/EEA geoblocked (`ADR-008`); full DPDP/GDPR privacy pack gated on v1.1 counsel. |
| `docs/12-ai-ml/` | **N/A at MVP** | Explicit Non-Goal #4 (`PRODUCT_REQUIREMENTS.md`): zero AI-generated case narratives or conclusions. |
| `docs/13-infrastructure/` | Deferred (Interface Defined) | `DEPLOYMENT.md`, `INGESTION.md` baselines exist; cloud/VPS & S3 provider selection explicitly deferred (`ADR-013`). |
| `docs/14-devops/` | Deferred | GitHub Actions CI workflows (using Docker service containers for Postgres, Redis, Meilisearch) created in Prompt 8. |
| `docs/15-testing/` | Deferred | Test pyramid & automated RBAC/Q5/kill-switch verification suite implemented alongside Prompt 8+. |
| `docs/16-performance/` | Deferred | Targets set (`<= 500ms` search p95, `<= 2.5s` mobile LCP, `<= 60s` kill-switch); load benchmarks post-MVP skeleton. |
| `docs/17-observability/` | Deferred | Structured zero-PII JSON logging & `/healthz` readiness probes specified in `LOW_LEVEL_DESIGN.md`; metrics dashboards post-MVP. |
| `docs/18-reliability/` | Deferred | Failure modes & backup/restore strategy documented in `HIGH_LEVEL_DESIGN.md` / `LOW_LEVEL_DESIGN.md`; drills pre-staging. |
| `docs/19-compliance/` | Baseline / External Gate | `COMPLIANCE.md` baseline; mandatory US media/First Amendment counsel sign-off required before Case 1 publishes (`REQ-027`). |
| `docs/20-operations/` | Deferred | Solo-founder runbooks (retractions, backups, GeoIP refresh, JWT rotation) written prior to staging launch. |
| `docs/21-governance/` | Documented (Baseline) | `RISKS.md` + risk register in §4 below. |
| `docs/22-progress/` | **Documented (Phase 01–06)** | `RECON_REPORT.md`, `ENV_PROBE_REPORT.md`, `PO_SCOPING_DECISIONS.md`, `REQUIREMENTS_STATUS.md`, `ARCHITECTURE_STATUS.md`, `STATUS_LEDGER.md`. |
| `docs/23-decisions/` | **Documented (`ADR-001`–`ADR-015`)** | Complete 15-ADR decision trail covering environment, product scoping, and Phase 06 architecture. |
| `docs/24-setup/` | **Documented (Phase 02)** | `LOCAL_DEV.md`, `KNOWN_ENV_LIMITATIONS.md`. |
| `docs/25-toolchain/` | **Documented (Phase 02)** | Pinned versions in `.tool-versions` and `scripts/bootstrap-env.sh`. |

*(Note on Billing / Domain 21 per PO Scoping Decision #2: Explicitly **N/A at MVP and v1.0**, deferred until >= 1,000 MAU and >= 3 case drops.)*

---

## 2. Complete ADR Registry (`ADR-001` – `ADR-015`)

| ADR | File Path | One-Line Summary |
|---|---|---|
| **ADR-001** | `docs/23-decisions/ADR-001-docs-directory-structure.md` | Standardize `docs/` directory layout strictly to Charter PART 06 numbering. |
| **ADR-002** | `docs/23-decisions/ADR-002-additional-docs-directories.md` | Authorize `docs/24-setup/` and `docs/25-toolchain/` extensions for sandbox/toolchain runbooks. |
| **ADR-003** | `docs/23-decisions/ADR-003-local-dev-direct-processes.md` | Run PostgreSQL/Redis as direct local processes in the gVisor sandbox (no Docker locally; Docker service containers in CI). |
| **ADR-004** | `docs/23-decisions/ADR-004-go-toolchain-path-precedence.md` | Enforce `/usr/local/go/bin` ahead of `/usr/bin` so pinned Go 1.27.4 resolves reliably. |
| **ADR-005** | `docs/23-decisions/ADR-005-postgres-local-trust-auth.md` | Restrict PostgreSQL `--auth=trust` solely to `127.0.0.1:5432` local sandbox via Makefile guards; mandate `scram-sha-256` + TLS in deployed envs. |
| **ADR-006** | `docs/23-decisions/ADR-006-admin-web-unified-react-vite-spa.md` | Unify `admin-web` on React + Vite + Tailwind SPA (same stack as `mobile-web`; no Next.js). |
| **ADR-007** | `docs/23-decisions/ADR-007-defer-user-accounts-to-v1-1.md` | Defer consumer user accounts (`REQ-015`/`017`/`018`) to v1.1 gated on India DPDP counsel; v1.0 is anonymous-reading-only with internal admin auth. |
| **ADR-008** | `docs/23-decisions/ADR-008-geoblock-eu-eea-at-launch.md` | Geoblock EU/EEA visitors at IP level on the reader surface at launch (`HTTP 451` static notice) until GDPR counsel is funded. |
| **ADR-009** | `docs/23-decisions/ADR-009-defer-community-theories-to-v1-1.md` | Defer Community Theories (`REQ-019`–`022`) to v1.1 (schema-ready design only; no launch UI/queue; gated on moderation-policy legal review). |
| **ADR-010** | `docs/23-decisions/ADR-010-go-modular-monolith.md` | Build the backend as a Go modular monolith (`backend-go`) with strict internal packages plus one isolated Rust worker (not microservices). |
| **ADR-011** | `docs/23-decisions/ADR-011-redis-streams-job-queue.md` | Use Redis Streams (`evidence:jobs`, `evidence:results`, `evidence:dlq`) for Go↔Rust async document jobs (not Kafka, RabbitMQ, or SQS). |
| **ADR-012** | `docs/23-decisions/ADR-012-meilisearch-search-engine.md` | Use self-hosted Meilisearch for typo-tolerant search over OCR'd evidence while keeping PostgreSQL as sole durable source of truth. |
| **ADR-013** | `docs/23-decisions/ADR-013-s3-compatible-object-storage-interface.md` | Design evidence storage behind an S3-compatible `ObjectStore` interface and defer cloud provider selection until staging provisioning. |
| **ADR-014** | `docs/23-decisions/ADR-014-pwa-first-no-native-at-launch.md` | Ship installable offline-capable mobile-first PWA (`mobile-web`) as the sole reader client at launch; defer native iOS/Android builds. |
| **ADR-015** | `docs/23-decisions/ADR-015-anonymous-reader-with-geoblock-middleware.md` | Enforce anonymous public reading and EU/EEA geoblocking via dedicated server-side middleware in `backend-go` backed by proxy header sanitization. |

---

## 3. Top 5 Remaining Unknowns for PO Resolution Before Implementation

1. **UK Visitor Treatment Under `ADR-008` (UK GDPR):** `ADR-008` explicitly blocks the 30 EU/EEA states. Should the United Kingdom (`GB`) also be included in the launch geoblock list (because UK GDPR mirrors EU GDPR Art. 10/17 closely) until UK/EU counsel is funded?
2. **Local Sandbox Meilisearch & Tesseract OCR Binaries:** Prompt 8+ needs local Meilisearch (for search integration tests) and Tesseract OCR (for `processor-rust` scanned-PDF OCR). Should `scripts/bootstrap-env.sh` and `Makefile` be extended in Prompt 8 to install/start local `meilisearch` and `tesseract-ocr` binaries cleanly (consistent with `ADR-003`), or should local OCR tests use a pluggable stub when Tesseract is absent?
3. **S3-Compatible Provider & VPS Vendor Timing (`ADR-013`):** Confirm the target milestone (e.g., before staging deploy after Prompt 10) to author `ADR-016` choosing the specific VPS host and low/zero-egress S3 provider (e.g., Cloudflare R2 vs. Backblaze B2) within the `$300–$500` 6-month ceiling.
4. **Admin Origin Isolation Strategy:** Confirm whether production `admin-web` will be hosted on a separate subdomain (`admin.<domain>`) with reverse-proxy IP allowlisting/VPN restriction in addition to the 60-minute Admin JWT (`SECURITY_ARCHITECTURE.md` T2).
5. **US Media / First Amendment Legal Counsel Engagement Timeline:** Confirm when the `$1,000–$1,500` legal engagement begins so template disclaimers, the `correction_requests` wording (`REQ-030`), and Case 1 review do not become a late critical-path blocker before first publish (`REQ-027`).

---

## 4. Architecture Risk Register (Top 5 Highest Risks)

| Rank | Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|---|
| **R1** | **Third-Party Copyrighted PACER Exhibit Accidentally Published as `PUBLIC_DOMAIN`** | Medium | Critical (`$750–$150k` statutory damages / work) | Defense-in-depth: Q5 API validation (`HTTP 422`), PostgreSQL `CHECK (NOT (source_type = 'third_party_exhibit' AND license_status = 'PUBLIC_DOMAIN'))`, mandatory `licensing_record_id` for `LICENSED`, and `legal_review_records` publish gate. |
| **R2** | **Offline PWA Cache Retains a Retracted Case After Kill-Switch (`REQ-006` vs. `REQ-031`)** | Medium | High (ongoing defamation/takedown exposure) | Online PWA Service Worker checks lightweight case retraction manifest on every visit and immediately evicts non-`PUBLISHED` case packs; S3 pre-signed URLs expire in `<= 300s`. |
| **R3** | **PostgreSQL ↔ Meilisearch Index Drift During Retraction or Publish** | Medium | High (leaked or missing search hits) | Postgres is sole source of truth; `cases.search_sync_pending` triggers 5s retry on retractions / 30s reconciliation on publishes; live `GET /cases/{slug}` and `GET /evidence/{id}` always verify Postgres `status = 'PUBLISHED'`; CLI `cmd/reindex` rebuilds index deterministically. |
| **R4** | **Hostile PDF / OCR Decompression Bomb Exhausts Solo-Founder 4 GB VPS RAM** | Low–Med | High (service outage) | Process isolation in `processor-rust` (never parsed in Go), `25 MB` upload cap, `500`-page max, `>100:1` ratio abort, single-slot OCR Tokio semaphore + `RLIMIT_AS` memory cap + `90s` timeout + `evidence:dlq` isolation. |
| **R5** | **Container Recycle / Single-VPS Disk Failure Loses Data or Local State** | High (sandbox) / Low (VPS) | Critical | Remote GitHub repo (`mahitss/evidence_architecture`) as single source of truth for code/docs; automated encrypted daily `pg_dump` to off-box backup bucket + durable S3-compatible object storage for immutable evidence binaries (`ADR-013`). |

---

## 5. Definition of Ready for Prompt 8 (Foundation — Repo Scaffolding + DB Migration + Go API Skeleton)

Every item below **MUST** pass before transitioning from MODE A (Architect) to Prompt 8 implementation:

- [x] **Requirements Baseline Frozen:** `PRODUCT_REQUIREMENTS.md` (`32` testable `REQ-xxx`), `EVIDENCE_INTEGRITY_SPEC.md` (with Q5 rule), `FEATURE_CATALOG.md`, `PERMISSION_MATRIX.md`, `USER_STORIES.md`, and `REQUIREMENTS_STATUS.md` (0 open PO questions) exist and cross-reference cleanly.
- [x] **System & Component Architecture Approved:** `SYSTEM_CONTEXT.md` (C4 L1), `HIGH_LEVEL_DESIGN.md` (C4 L2), `LOW_LEVEL_DESIGN.md` (Go/Rust packages, Redis Streams, failure modes, concurrency), and `SEQUENCE_DIAGRAMS.md` (`SD-1`–`SD-5`) complete.
- [x] **Data Model & Invariants Finalized:** `DATABASE_DESIGN.md` defines all 8 v1.0 tables, column types, indexes, append-only `audit_log`, publication legal-review trigger, and verbatim Q5 `CHECK` constraint (`NOT (source_type = 'third_party_exhibit' AND license_status = 'PUBLIC_DOMAIN')`), with zero consumer-user or community tables at v1.0.
- [x] **API Contract Frozen:** `docs/09-api/openapi.yaml` (OpenAPI 3.1) defines all 13 v1.0 endpoints, error envelope, cursor pagination, rate limits, `Idempotency-Key` rules, and Q5 comment.
- [x] **Security Architecture Signed Off:** `SECURITY_ARCHITECTURE.md` covers trust boundaries, STRIDE `T1`–`T8`, JWT rotation/TTL/rate-limiting, and explicit MVP out-of-scope boundaries.
- [x] **All 15 ADRs Recorded:** `ADR-001` through `ADR-015` exist in `docs/23-decisions/` with rejected alternatives and reversal strategies.
- [ ] **PO Audit & Sign-Off of Phase 06 Commit:** PO reviews Phase 06 docs, pulls local commit to laptop / pushes to `origin/main`, and answers the 2 implementation-affecting unknowns in §3 (#1 UK geoblock inclusion; #2 local Meilisearch/Tesseract bootstrap strategy).
- [ ] **Local Sandbox Toolchains Verified:** `bash scripts/bootstrap-env.sh`, `make env-check`, and `make services-up` / `make services-status` green on the target Prompt 8 session before scaffolding `backend-go/` and migrations.
