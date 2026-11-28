# ADR-018: PDF Text Extraction — `lopdf` Fallback at MVP vs. `pdfium`

- **Status:** Accepted
- **Date:** 2026-11-28
- **Deciders:** Engineering / Architecture (Phase 09 — Prompt 8e, closes FU8 from Prompt 8d audit)
- **Related Docs:** `docs/23-decisions/ADR-017-ocr-strategy.md`, `docs/22-progress/CI_VERIFICATION_CHECKLIST.md`, `docs/05-architecture/LOW_LEVEL_DESIGN.md`, `processor-rust/tests/pdf_quality.rs`

---

## Context

The Rust document processor (`processor-rust/`) extracts UTF-8 text and page counts from uploaded PDF exhibits (`application/pdf`) prior to Meilisearch indexing (`ADR-012`, `ADR-019`).

Originally, the low-level design considered `pdfium-render` wrapping Google's native `libpdfium` C++ library for high-fidelity PDF text extraction and rendering. However, `pdfium-render` requires the native `libpdfium.so` shared library to be installed on the host. In the local development sandbox (`Debian 12` under gVisor), `libpdfium` is not installed and running `apt-get` to install native libraries is prohibited (`docs/24-setup/KNOWN_ENV_LIMITATIONS.md`). During Prompt 8d, the processor implemented pure-Rust PDF text extraction using **`lopdf` (`0.34.0`)** as an immediate fallback.

## Problem

`lopdf` operates directly on raw PDF content streams and font dictionaries without a full layout engine. As a result, `lopdf` extracts text less reliably than `pdfium` from complex PDFs, specifically:
- Multi-column legal filings and court orders,
- PDFs with custom/subsetted ToUnicode CMap or embedded Type0/CID fonts,
- Hybrid PDFs combining scanned page images with embedded OCR text layers.

Federal court records from PACER, DOJ/SEC exhibits, and historical public-record archives frequently exhibit these characteristics. Degraded text extraction directly reduces Meilisearch search recall and snippet accuracy.

---

## Alternatives Considered

### 1. (Rejected) Install `libpdfium` in the local sandbox
- **Why rejected:** Blocked by environment constraints (`apt-get` prohibited in local sandbox; gVisor `/var/log` dpkg limitations documented in `KNOWN_ENV_LIMITATIONS.md`). Would make local development non-reproducible across container recycles.

### 2. (Rejected) Link `libpdfium` at build time via system library path
- **Why rejected:** Still requires `libpdfium.so` to exist at runtime for every `cargo test` and local processor execution. Because the sandbox lacks `libpdfium`, unit and integration tests would fail locally.

### 3. (Rejected) Defer PDF text extraction entirely until CI/production
- **Why rejected:** MVP cannot ship or be verified end-to-end without working document ingestion (`upload -> Redis Streams -> Rust extraction -> Postgres -> Meilisearch -> reader search`).

### 4. (Selected) Use `lopdf` at MVP, enforce a quality benchmark gate, and run `lopdf` vs. `pdfium` comparison tests in CI
- **Why selected:** Unblocks zero-native-dependency local development and deterministic pure-Rust builds today, while treating extraction fidelity as an explicit, measurable quality gate rather than an assumption.
- `processor-rust/tests/pdf_quality.rs` benchmarks `lopdf` against three representative launch document fixtures (government press release, court docket page, historical public record) with non-negotiable minimum character and distinct-word thresholds (`> 500` chars, `> 100` distinct words).
- CI runs both `lopdf` and `pdfium` extraction across a representative corpus (DOJ/SEC press releases, PACER court dockets, historical public records) and compares character recall (`docs/22-progress/CI_VERIFICATION_CHECKLIST.md` #3).

---

## Consequences

- **Search recall risk must be measured, not assumed:** MVP text extraction quality from complex real-world PDFs may be lower than `pdfium`, directly affecting Meilisearch full-text recall.
- **Mandatory CI corpus comparison gate:** CI must run side-by-side `lopdf` vs. `pdfium` extraction tests on a representative corpus covering all launch categories (government press releases, PACER court dockets, historical public records).
- **Hard supersession trigger (`< 90%` recall):** If `lopdf` extracts **`< 90%`** of expected text compared to `pdfium` on **any** document category in the CI corpus, this ADR is automatically superseded by a `pdfium` migration plan before production launch.
- **Zero-weakening rule:** Quality thresholds in `processor-rust/tests/pdf_quality.rs` are a quality gate, never a target to lower.

---

## Reversal / Migration Path

Because PDF parsing is isolated behind `processor-rust/src/pdf.rs` (`extract_pdf_pages(&[u8]) -> Result<PdfExtraction, PdfError>`), migrating to `pdfium` requires:
1. Installing `libpdfium` (e.g., pinned prebuilt `pdfium-binaries` release) into the production/CI worker image.
2. Swapping the extractor implementation inside `src/pdf.rs` (or behind a `PdfExtractor` trait) to `pdfium-render` while keeping `src/extract.rs`, `src/processor.rs`, `src/consumer.rs`, and database/Meilisearch contracts unchanged.
3. Re-running `processor-rust/tests/pdf_quality.rs` and the CI corpus comparison to confirm `>= 99%` recall.
