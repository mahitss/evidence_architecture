# ADR-013: Design to an S3-Compatible Object Storage Interface (Defer Cloud Provider Selection)

- **Status:** Accepted
- **Date:** 2026-11-27
- **Deciders:** Product Owner, Architect (MODE A)
- **Refs:** `docs/05-architecture/HIGH_LEVEL_DESIGN.md`, `docs/05-architecture/LOW_LEVEL_DESIGN.md`, `docs/08-database/DATABASE_DESIGN.md`, `docs/22-progress/PO_SCOPING_DECISIONS.md` (#4)

## Context

Evidence items consist of immutable primary-source PDFs, archival scans, images, and recreated visuals up to `25 MB` each (plus archived written license proofs referenced by `licensing_records.proof_object_key`). The Product Owner has a `$300–$500` six-month infrastructure budget and has explicitly directed that the **cloud/object-storage vendor choice be deferred** until staging/production provisioning while designing the architecture cleanly against a standard S3-compatible interface.

## Problem

Hardcoding a specific cloud vendor's proprietary SDK or storing binary PDFs directly inside PostgreSQL or an ephemeral VPS disk would either cause premature vendor lock-in, bloat database backups and RAM, or create severe data-loss risk on VPS rebuild.

## Alternatives

1. **Rejected — Store Evidence Binaries as `BYTEA` Inside PostgreSQL:**
   - *Reasoning:* Bloats PostgreSQL storage, shared buffers, WAL logs, and daily `pg_dump` backups; forces Go to proxy all multi-megabyte PDF downloads through DB connections; degrades query performance on a small VPS.
2. **Rejected — Store Production Evidence Only on Local VPS Filesystem:**
   - *Reasoning:* Ties immutable legal evidence to a single VPS disk (high data-loss risk without complex rsync/replication), lacks native pre-signed expiring URLs, and blocks clean multi-origin browser sandboxing for untrusted PDF delivery (`SECURITY_ARCHITECTURE.md` T3).
3. **Rejected — Commit Now to a Specific Cloud Provider (e.g., AWS S3 vs. Cloudflare R2 vs. Backblaze B2 vs. MinIO):**
   - *Reasoning:* Premature during Phase 06 architecture; pricing, egress fees (critical for `$300–$500` ceiling, where zero-egress options like R2/B2 matter), and India/US latency should be decided in a dedicated provisioning ADR immediately before staging setup.
4. **Selected — Abstract Behind a Narrow S3-Compatible `ObjectStore` Interface (Provider Deferred):**
   - *Reasoning:* Allows immediate local development using a filesystem-backed stub (no Docker required per `ADR-003`) while guaranteeing seamless drop-in compatibility with any S3-compatible provider (Cloudflare R2, Backblaze B2, AWS S3, DigitalOcean Spaces, Scaleway, or self-hosted MinIO) via standard AWS SigV4 / S3 API endpoints.

## Selected

1. Define a minimal `ObjectStore` interface in `backend-go` (`PutObject`, `GetObject`, `PresignGetURL(key, ttl)`, `DeleteObject`) and matching S3 client config in `processor-rust` (`GetObject`).
2. Configure entirely via environment variables (`S3_ENDPOINT`, `S3_REGION`, `S3_BUCKET`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_FORCE_PATH_STYLE`).
3. **Local Sandbox (`APP_ENV=local`):** Supports a zero-Docker local filesystem adapter under `./data/objects/` (with HMAC-signed local download endpoint on Go API matching pre-signed URL semantics) so local dev never needs cloud credentials.
4. **Private Bucket Only:** All evidence buckets are private (`Block Public Access = ON`). Readers receive strictly short-lived (`<= 300s`) pre-signed GET URLs minted by Go only for `PUBLISHED`, non-retracted items.
5. **Cloud Provider Selection:** Explicitly deferred to a future Infrastructure/Provisioning ADR prior to staging launch.

## Rationale

The S3 API (`PutObject`, `GetObject`, SigV4 pre-signed URLs) is the universal lingua franca of cloud object storage. Designing to this interface keeps Phase 06 and Prompt 8–10 implementation 100% cloud-agnostic while protecting the `$300–$500` budget from surprise egress costs when choosing a vendor later.

## Trade-offs

- Local filesystem stub must emulate signed expiring URLs so E2E tests exercise the exact same URL-expiration behavior as real S3 pre-signed URLs.
- Subtle S3-compatible provider quirks (e.g., path-style vs. virtual-host addressing, custom `Content-Disposition` response header support) must be smoke-tested once the staging provider is chosen.

## Risks

- Choosing a high-egress provider later could exceed the `$300–$500` budget if a case goes viral with 20 MB PDFs.
- **Mitigation:** Deferring selection preserves full optionality for zero/low-egress S3-compatible storage (e.g., Cloudflare R2 or Backblaze B2) plus PWA offline caching (`REQ-006`) which slashes repeat downloads.

## Consequences

- `evidence_items.object_key` stores only provider-agnostic relative keys (`cases/{case_id}/evidence/{evidence_id}/{sha256}.{ext}`), never full vendor URLs.
- Zero cloud vendor account or billing card required to complete Phase 06 Architecture or Prompt 8–10 local development and CI.

## Reversal Strategy

Switching between any two S3-compatible providers later requires only copying bucket objects (`rclone` / `aws s3 sync`) and updating `S3_ENDPOINT` / `S3_BUCKET` / credentials in runtime env vars—zero code or database changes.
