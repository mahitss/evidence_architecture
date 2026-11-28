# ADR-017: OCR Execution Strategy for `processor-rust`

- **Status:** Accepted
- **Date:** 2026-11-27
- **Deciders:** Engineering / Architecture (Phase 09 — Prompt 8d)
- **Related Docs:** `docs/05-architecture/LOW_LEVEL_DESIGN.md`, `docs/05-architecture/SEQUENCE_DIAGRAMS.md`, `docs/24-setup/LOCAL_DEV.md`

---

## Context

The Rust document processor (`processor-rust/`) consumes `evidence.process` jobs from Redis Streams (`ADR-011`) and extracts searchable UTF-8 text from uploaded evidence files (`application/pdf`, `text/plain`, `image/png`, `image/jpeg`).

Evidence PDFs fall into two categories:
1. **Digital-text PDFs** (court orders, DOJ/SEC/IRS press releases, electronic docket filings) whose text layer can be extracted directly in pure Rust (`lopdf`, used in place of `libpdfium` where native PDFium shared libraries are absent).
2. **Scanned / image-only PDFs and standalone images** (historical public-record scans, physical exhibits) where native PDF text extraction yields fewer than `20` characters per page (`MIN_CHARS_PER_PAGE = 20`) and Optical Character Recognition (OCR) is required.

We need an OCR integration strategy for MVP that is:
- Safe against malformed/adversarial inputs (a crash or hang on one document must not take down the worker process),
- Free of C/C++ FFI build-time coupling in Rust builds (`cargo build` / `cargo test` must succeed even on hosts without `libtesseract-dev` headers),
- Capable of graceful degradation when OCR binaries are absent from the local environment, and
- Compatible with local-first development without external API dependencies or per-page cloud costs.

---

## Decision

Invoke the **`tesseract` CLI as an OS subprocess** via `tokio::process::Command` at MVP:

1. **Startup probe:** On startup (and lazily via `OnceLock`), `processor-rust` executes `tesseract --version`.
   - If `tesseract` is present on `PATH`, OCR is enabled and `/health` reports `"tesseract":"available"`.
   - If `tesseract` is missing, `processor-rust` logs a structured warning (`"tesseract not found on PATH; OCR disabled (digital PDFs will still process)"`), sets `ocr_disabled = true`, reports `"tesseract":"missing"` on `/health`, and continues processing digital-text PDFs and plain-text files normally.
   - If a scanned PDF or image requires OCR while `ocr_disabled = true`, the job fails permanently with error code `OCR_UNAVAILABLE`, marks `evidence_items.processing_status = 'FAILED'`, records an `audit_log` entry (`evidence.processing_failed`), and routes the message to `evidence.process.dead`.
2. **Safe subprocess invocation (no shell interpolation):**
   - Arguments are passed directly as an `argv` array (`tesseract <image_path> stdout -l eng --psm 6`), never via `sh -c`.
   - For scanned PDFs, pages are rasterized into a per-job isolated `tempfile::TempDir` using `pdftoppm` (with `gs` / Ghostscript fallback), then OCR'd page-by-page.
   - Each subprocess invocation enforces a strict **60-second per-page timeout** (`kill_on_drop(true)`), terminating runaway processes cleanly.
   - OCR output is bounded by a **10 MB max extracted text cap** to prevent memory exhaustion on pathological documents.

---

## Alternatives Considered and Rejected

### 1. `tesseract-rs` (Rust C++ FFI bindings to `libtesseract`)
- **Why rejected:** Requires `libtesseract-dev`, `libleptonica-dev`, `pkg-config`, and `clang`/`libclang` headers on every developer workstation and CI build container just to compile `processor-rust`. Additionally, linking C++ OCR libraries directly into the Rust address space means any memory bug or abort inside Leptonica/Tesseract on a malformed scan terminates the entire async worker process (affecting concurrent jobs) and makes hard per-page timeouts difficult to enforce safely without leaking native memory.

### 2. `leptess` (Rust wrapper over Leptonica + Tesseract C API)
- **Why rejected:** Carries the same native C/C++ build-time toolchain coupling and process-isolation drawbacks as `tesseract-rs`. `leptess` also tracks upstream Tesseract releases slowly and cannot be compiled cleanly on minimal containers without system dev packages, breaking the zero-apt build workflow for local digital-PDF development.

### 3. Cloud OCR APIs (Google Cloud Vision, AWS Textract, Azure AI Document Intelligence)
- **Why rejected:** Introduces external network calls, API credential management, variable per-page billing, and third-party data transfer of unredacted `DRAFT` investigative exhibits prior to legal review. At MVP scale (low-volume admin-curated case uploads), local subprocess OCR provides predictable zero marginal cost, full data residency, and offline local-dev parity.

---

## Consequences

### Positive
- **Zero native build dependencies:** `cargo clippy`, `cargo test`, and `cargo build --release` succeed on any host with the Rust toolchain alone.
- **Process & memory isolation:** A crash, OOM, or hang inside `tesseract` is isolated to a single child process killed after 60 seconds; the Tokio worker pool stays healthy and routes the offending job to retry/DLQ.
- **Clean upgrade path:** Because OCR is encapsulated behind `src/ocr.rs`, switching to a dedicated OCR sidecar or managed service in v1.1+ requires changing only one module.

### Negative / Trade-offs
- **Subprocess spawn overhead:** Spawning a `tesseract` process per scanned page adds ~20–50ms of OS overhead compared to in-process FFI. This is negligible relative to OCR CPU time (0.5–3s/page) and MVP ingestion volume.
- **Runtime package dependency for OCR hosts:** Production worker images and CI runners executing OCR tests must install `tesseract-ocr` and `poppler-utils` (or `ghostscript`).

---

## Verification Status

- **Code path & graceful degradation:** **VERIFIED** via unit tests (`src/ocr.rs`, `src/extract.rs`, `src/processor.rs`, `src/consumer.rs`) and real PostgreSQL/Redis integration tests (`tests/integration.rs`).
- **Live `tesseract` OCR execution on `tests/fixtures/scanned.pdf`:** **`UNVERIFIED-IN-SANDBOX`** — `tesseract` is not installed in the local sandbox environment and `apt-get` is prohibited during Prompt 8d. **MUST VERIFY ON FIRST CI RUN** after installing `tesseract-ocr` and `poppler-utils` (see `docs/14-devops/CI_CD_DESIGN.md`).
