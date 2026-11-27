# ADR-010: Build Backend as a Go Modular Monolith (Not Microservices)

- **Status:** Accepted
- **Date:** 2026-11-27
- **Deciders:** Product Owner, Architect (MODE A)
- **Refs:** `docs/05-architecture/HIGH_LEVEL_DESIGN.md`, `docs/05-architecture/LOW_LEVEL_DESIGN.md`, `docs/22-progress/PO_SCOPING_DECISIONS.md` (#3, #4)

## Context

The True Crime Investigation Platform is operated by a solo founder plus one AI implementation agent with a `$300–$500` six-month infrastructure ceiling and zero 24/7 operational staff. The backend serves public case/evidence reads, search proxying, correction submissions, internal admin authoring, legal-review publication gating, and job orchestration with a separate Rust document processor (`processor-rust`).

## Problem

Decomposing the HTTP API layer into separate microservices (e.g., Case Service, Evidence Service, Legal Review Service, Search Service, Corrections Service) would introduce distributed transactions, inter-service RPC authentication, service discovery, multiple deployment pipelines, and memory overhead on a single small VPS—directly conflicting with the solo-founder and `$300–$500` infra constraints.

## Alternatives

1. **Rejected — Microservices Architecture (separate Go services per domain):**
   - *Reasoning:* Breaks single-transaction ACID invariants across `cases`, `evidence_items`, `legal_review_records`, and `audit_log` (forcing fragile sagas for the legal publication gate), multiplies operational failure points and VPS RAM usage, and slows iteration for a 1-person team with no distributed-tracing/service-mesh ops capacity.
2. **Rejected — Node.js / TypeScript Backend Monolith:**
   - *Reasoning:* Higher memory footprint under concurrent multipart PDF uploads and streaming SHA-256 computation, weaker static binary deployment ergonomics, and diverges from the verified Go 1.27.4 toolchain already established in Phase 02 (`ADR-004`).
3. **Selected — Go Modular Monolith (`backend-go`) with strict internal package boundaries + single isolated Rust worker (`processor-rust`):**
   - *Reasoning:* Preserves single-process deployment, ACID PostgreSQL transactions for legal publication and audit logging, low RAM/CPU overhead, and clean internal module boundaries (`cmd/`, `internal/handlers`, `internal/models`, `internal/store`, `internal/queue`, `internal/middleware`, `internal/config`) while isolating untrusted PDF/OCR execution inside Rust.

## Selected

1. All HTTP API routes (`/api/v1/*` and `/api/v1/admin/*`), domain rules, PostgreSQL transactions, Meilisearch sync, and Redis Streams orchestration live inside one compiled Go binary (`backend-go`).
2. Strict unidirectional package dependencies (`LOW_LEVEL_DESIGN.md §1`) enforce modularity so domain logic (`internal/models`) remains decoupled from HTTP and SQL.
3. Only CPU/memory-hazardous untrusted file parsing and OCR are separated into an independent OS process (`processor-rust`) communicating asynchronously via Redis Streams (`ADR-011`).

## Rationale

A modular monolith gives 100% of the code clarity and testability needed at 10–50 cases while keeping transactional consistency trivial (e.g., checking `legal_review_records`, updating `cases.status = 'PUBLISHED'`, and inserting into `audit_log` in one atomic SQL transaction) and running comfortably in `< 64 MB` RAM on a budget VPS.

## Trade-offs

- All HTTP endpoints share one OS process and deployment lifecycle (mitigated by graceful shutdown, strict per-request timeouts, and panic recovery middleware).
- Independent horizontal scaling of individual API domains (e.g., scaling `/search` separately from `/admin`) is not possible without extracting a service later.

## Risks

- Package boundaries could erode over time into tangled imports if not enforced.
- **Mitigation:** Acyclic dependency rules documented in `LOW_LEVEL_DESIGN.md` and enforced via Go compiler cycle checks plus code review.

## Consequences

- Single `backend-go` binary deployed alongside `processor-rust` on staging/production VPS.
- Zero service mesh, zero internal gRPC/mTLS overhead, and simple single-transaction integration testing against PostgreSQL.

## Reversal Strategy

Because `internal/models`, `internal/store`, and `internal/handlers` maintain clean module interfaces, any high-load read path (e.g., public search/catalog) can be extracted into a separate Go binary sharing the same PostgreSQL read replica without rewriting domain logic.
