# ADR-019: Meilisearch Index Schema & Ranking Configuration

- **Status:** Accepted
- **Date:** 2026-11-28
- **Deciders:** Engineering / Architecture (Phase 09 — Prompt 8e)
- **Related Docs:** `docs/23-decisions/ADR-012-meilisearch-not-elasticsearch.md`, `docs/05-architecture/LOW_LEVEL_DESIGN.md`, `docs/10-security/SECURITY_ARCHITECTURE.md` (T5), `backend-go/internal/search/meili.go`

---

## Context

Per `ADR-012`, the platform uses self-hosted **Meilisearch** for full-text search across published investigative dossiers and their extracted evidence text (`GET /api/v1/search`). We need a deterministic index schema, filter contract, and ranking configuration that:
- Allows anonymous readers to search across both launch categories (`financial_crime`, `historical_public_record`) and within a single category filter,
- Guarantees unpublished/draft/retracted dossiers are never returned in search results (defense-in-depth with `T1` and `T5`),
- Prioritizes exact term matches (case numbers, docket IDs, statutory citations, entity names) over fuzzy typo matches, and
- Keeps publish, retract, and re-review de-indexing operations atomic per case.

---

## Decision

Configure a **single Meilisearch index named `cases`** at MVP with primary key `id` (the case UUID string):

### Indexed Document Fields
| Field | Type | Description |
|---|---|---|
| `id` | `string` (UUID) | Primary key (`cases.id`) |
| `case_slug` | `string` | URL slug (`cases.slug`) |
| `case_title` | `string` | Case title (`cases.title`) |
| `case_summary` | `string` | Case summary / official briefing (`cases.official_briefing`) |
| `status` | `string` | Case status (`PUBLISHED` only when indexed) |
| `category` | `string` | Launch category (`financial_crime` \| `historical_public_record`) |
| `published_at` | `string` (RFC3339) | Publication timestamp (`cases.published_at`) |
| `evidence_count` | `integer` | Count of evidence items attached to the case |
| `evidence_titles` | `string[]` | Titles of all non-redacted completed evidence items |
| `evidence_extracted_text` | `string[]` | Extracted PDF/OCR/text bodies from non-redacted completed evidence items |
| `evidence_source_type` | `string[]` | Distinct `source_type` values across the case's evidence items |

### Index Settings
- **Filterable attributes:** `["status", "category"]`
  - Every reader search unconditionally injects `status = "PUBLISHED"` (and optional allowlisted `category = "<value>"`). User search strings (`q`) are passed exclusively as Meilisearch's structured `q` parameter, never interpolated into filter syntax (`SECURITY_ARCHITECTURE.md` T5).
- **Searchable attributes (in priority order):** `["case_title", "case_summary", "evidence_titles", "evidence_extracted_text"]`
- **Ranking rules:** `["exactness", "words", "typo", "proximity", "attribute", "sort"]`
  - **`exactness` is placed first** (ahead of Meilisearch's default `words` / `typo` ordering) because legal/investigative searches frequently target exact docket numbers, statutory sections, entity names, and exact exhibit terms where exact matches must outrank fuzzy typo matches.

### Indexing Lifecycle
- **On `POST /api/v1/admin/cases/{id}/publish`:** Synchronous upsert to `cases` inside the publish workflow; if indexing fails, the publish fails (search availability is part of the publish contract).
- **On `POST /api/v1/admin/cases/{id}/retract`:** Synchronous delete of document `{case_id}` from `cases`.
- **On `PUT /api/v1/admin/evidence/{id}` when parent case was `PUBLISHED`:** Because the store transaction demotes the parent case to `IN_REVIEW` (`T6`), synchronously delete document `{case_id}` from `cases`; republishing re-indexes it.
- **On evidence processing completion (`processor-rust`):** When a `PUBLISHED` case's evidence finishes processing, update the indexed case document in place with the newly extracted text.

---

## Alternatives Considered and Rejected

### 1. (Rejected) Per-category indexes (`cases_financial_crime`, `cases_historical_public_record`)
- **Why rejected:** Readers frequently search across the entire catalog without pre-selecting a category (e.g., searching a jurisdiction, agency, or legal statute). Splitting into per-category indexes forces multi-index fan-out queries, complicates cursor pagination and cross-category ranking, and duplicates index settings management for zero scale benefit at MVP catalog sizes.

### 2. (Rejected) Separate `evidence` index alongside `cases`
- **Why rejected:** Maintaining two separate indexes (`cases` and `evidence`) turns every publish, retraction (`REQ-031` kill-switch), and post-publish evidence edit (`T6`) into a multi-document, multi-index synchronization workflow. Partial failure could leave individual evidence hits searchable after a parent case is retracted or demoted to `IN_REVIEW`, violating `T1` and increasing operational risk. A single case-keyed document makes publish/retract removal a single atomic document upsert or delete.

### 3. (Rejected) PostgreSQL `tsvector` full-text search only (no Meilisearch)
- **Why rejected:** Already evaluated and rejected in `ADR-012` due to weaker typo tolerance, highlighting ergonomics, and lack of dedicated search ranking tuning compared to Meilisearch.

---

## Consequences

- **Atomic visibility transitions:** Publishing, retracting, or demoting a case updates or removes exactly one document (`id = case_id`) in Meilisearch.
- **Document size bound:** For very large dossiers with dozens of lengthy PDFs, `evidence_extracted_text` per case must be monitored (and capped/truncated per exhibit if needed in v1.1+); at MVP case sizes (<20 exhibits/case), single-document indexing fits comfortably within Meilisearch limits.
- **Exact legal matching:** Placing `exactness` first improves precision for exact exhibit phrases, statute citations, and docket identifiers while still retaining typo tolerance for misspellings.
