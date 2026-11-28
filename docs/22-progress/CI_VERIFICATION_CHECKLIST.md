# CI Verification Checklist (`UNVERIFIED-IN-SANDBOX` Consolidated Tracker)

- **Document Path:** `docs/22-progress/CI_VERIFICATION_CHECKLIST.md`
- **Purpose:** Single authoritative checklist consolidating every `UNVERIFIED-IN-SANDBOX` item that cannot be fully executed inside the local gVisor sandbox and **MUST** be verified on the first CI run before production launch.
- **Closes:** Follow-up `FU9` (Prompt 8d audit)
- **Cross-Reference:** `docs/22-progress/STATUS_LEDGER.md`

---

## Mandatory First-CI-Run Verification Matrix

| # | Item | Source prompt | What to verify | How to verify | Owner |
|---|---|---|---|---|---|
| 1 | Non-root Makefile `services-up`/`services-down` | P5 | `make services-up` and `make services-down` work cleanly as a non-root user (`run_as()` Branch 2, no `su`/`chown`) | CI runner executes as non-root by default; run `make env-check && make services-up && make services-status && make services-down` on first CI run | CI |
| 2 | Live `tesseract` OCR | P8d, ADR-017 | `tesseract` subprocess OCR extracts accurate UTF-8 text from scanned image-only PDFs within 60s/page timeout | Install `tesseract-ocr` and `poppler-utils` via `apt-get` in CI, run `processor-rust/tests/fixtures/scanned.pdf` through `processor-rust`, assert `ocr_used = true` and non-empty extracted text | CI |
| 3 | `lopdf` vs. `pdfium` quality comparison | P8d, P8e, ADR-018 | `lopdf` extracts `>= 90%` of expected text compared to `pdfium` on representative corpus (government press releases, PACER court dockets, historical public records) | Extract text from corpus PDFs with both `lopdf` and `pdfium`, diff normalized character counts per category, fail CI job (and trigger `pdfium` migration per ADR-018) if any category falls below `90%` | CI |
| 4 | `staticcheck` pinned version | P8c, P8d, `CI_CD_DESIGN.md` | `honnef.co/go/tools/cmd/staticcheck@v0.8.2` (`2026.2.2`) compiles cleanly on Go `1.27.4` and passes `staticcheck ./...` with zero findings | Run `go install honnef.co/go/tools/cmd/staticcheck@v0.8.2` and `cd backend-go && staticcheck ./...` in CI; fail pipeline on install or lint error | CI |
| 5 | Full pipeline: admin upload -> Rust process -> Meilisearch index -> reader search finds it | P8e | End-to-end ingest, async extraction, legal review, publish indexing, and anonymous reader search (`GET /api/v1/search`) plus retract de-indexing | Start Postgres 15, Redis 7, Meilisearch, `processor-rust`, and `backend-go` in CI; execute full integration test uploading PDF -> `COMPLETED` -> legal review -> `POST /publish` -> `GET /search?q=<pdf term>` returns case -> `POST /retract` removes it | CI |

---

## Tracking Policy

- Every new `UNVERIFIED-IN-SANDBOX` item introduced in future prompts **MUST** be appended to the table above and linked from `docs/22-progress/STATUS_LEDGER.md`.
- An item may only be marked **CLOSED / VERIFIED** in this file after a green CI run provides captured command output proving the verification criteria passed.
