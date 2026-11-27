# ADR-012: Use Self-Hosted Meilisearch for Search (Not Elasticsearch or Postgres FTS)

- **Status:** Accepted
- **Date:** 2026-11-27
- **Deciders:** Product Owner, Architect (MODE A)
- **Refs:** `docs/05-architecture/HIGH_LEVEL_DESIGN.md`, `docs/09-api/openapi.yaml` (`/api/v1/search`), `docs/10-security/SECURITY_ARCHITECTURE.md` (T5), `docs/01-product/PRODUCT_REQUIREMENTS.md` (`REQ-011`, `REQ-012`)

## Context

Readers need fast (`<= 500ms` p95), intuitive full-text search across published case titles, Official Briefings, extracted PDF court filings, and OCR'd historical documents, with clean faceted filtering (`category`, `verification_tier`, `license_status`) and highlighted snippets (`REQ-011`, `REQ-012`). Phase 03 initially sketched PostgreSQL `tsvector` FTS; during Phase 06 architecture, the PO selected **Meilisearch** to deliver typo-tolerant search over noisy OCR text without Elasticsearch's operational footprint.

## Problem

Historical court scans and multi-page FOIA PDFs processed via OCR frequently contain minor spelling artifacts (e.g., `O`/`0`, `rn`/`m`) and readers frequently misspell complex defendant or shell-company names. Meanwhile, running a JVM search cluster violates our solo-founder and `$300–$500` VPS RAM constraints.

## Alternatives

1. **Rejected — Elasticsearch / OpenSearch:**
   - *Reasoning:* Requires a heavy JVM heap (`>= 2–4 GB` RAM alone), complex shard/index-template tuning, cluster-health babysitting, and steep operational overhead completely unsuitable for a single small VPS operated by a solo founder.
2. **Rejected — PostgreSQL Native Full-Text Search (`tsvector` / `pg_trgm`) Alone:**
   - *Reasoning:* Poor out-of-the-box typo tolerance on noisy OCR'd proper nouns and shell-company names, expensive `ts_headline` CPU overhead on multi-megabyte legal PDFs competing directly with primary transactional queries, and cumbersome facet-count + typo-ranking ergonomics compared to a dedicated search engine.
3. **Rejected — Algolia / Hosted Search SaaS:**
   - *Reasoning:* Violates the PO's `$0` paid third-party API constraint (`PO_SCOPING_DECISIONS.md` #4) and places OCR'd legal document text on a third-party SaaS whose record-size limits (`10 KB` per record on free tier) break on multi-page court filings.
4. **Selected — Self-Hosted Meilisearch (Single Rust Binary):**
   - *Reasoning:* Single lightweight Rust binary (`< 256 MB` RAM for 10–50 cases), built-in typo tolerance ideal for OCR text and entity names, sub-50ms ranked snippet highlighting, native facet filtering, zero per-query SaaS fees, and clean Docker service container support in CI.

## Selected

1. Deploy self-hosted **Meilisearch** bound strictly to loopback/private interface (`127.0.0.1:7700`) protected by internal API keys (never exposed directly to browsers).
2. **PostgreSQL remains the sole durable source of truth:** `evidence_items.extracted_text` is persisted in PostgreSQL first, and Meilisearch holds **only `PUBLISHED`, non-retracted** documents—making the search index 100% deterministically rebuildable via `cmd/reindex` at any time.
3. **Strict Sanitization (T5):** `backend-go` passes `q` strictly as Meilisearch's structured query string and constructs filter arrays solely from allowlisted enum values, unconditionally injecting `case_status = "PUBLISHED"`.
4. **Failure Mode:** If Meilisearch is unreachable, `GET /api/v1/search` fails cleanly within `2s` (`HTTP 503 SEARCH_UNAVAILABLE`); direct case browsing, timelines, and evidence viewing via PostgreSQL remain unaffected, while `cases.search_sync_pending = TRUE` queues automatic retry for pending publishes/retractions.

## Rationale

Meilisearch dramatically improves investigative discovery over OCR'd court filings (where typo tolerance matters most) while preserving solo-founder simplicity (single binary, near-zero tuning, `$0` license cost) and keeping PostgreSQL isolated from heavy text-ranking CPU spikes.

## Trade-offs

- Introduces one additional local/VPS daemon (`meilisearch`) and an eventual-consistency boundary between PostgreSQL and Meilisearch (mitigated by synchronous index calls on publish/retract + `search_sync_pending` 30-second reconciliation loop + 5-second retraction retry).
- Supersedes the PostgreSQL `tsvector` implementation detail mentioned in `PRODUCT_REQUIREMENTS.md` (`REQ-011`) while fully satisfying and exceeding `REQ-011`/`REQ-012` latency, snippet, and facet acceptance criteria.

## Risks

- **Stale Retracted Document in Search (`REQ-031`):** If Meilisearch is momentarily down during a kill-switch retraction, a hit could linger briefly in search results.
- **Mitigation:** Retraction immediately commits `status = 'RETRACTED'` in Postgres (so clicking any result immediately returns `404`), retries Meilisearch document deletion every 5 seconds until confirmed, and Go cross-filters/validates live status when `search_sync_pending` flags exist.

## Consequences

- `backend-go/internal/store` implements a narrow `SearchIndex` interface backed by Meilisearch HTTP API.
- Local sandbox (`scripts/bootstrap-env.sh` / `Makefile` in Prompt 8) and GitHub Actions CI will include the Meilisearch binary/container.

## Reversal Strategy

Because `evidence_items.extracted_text` is stored durably in PostgreSQL and accessed behind a Go `SearchIndex` interface, reverting to PostgreSQL `tsvector` or switching to Typesense/OpenSearch requires implementing one Go adapter and running a reindex with zero schema loss.
