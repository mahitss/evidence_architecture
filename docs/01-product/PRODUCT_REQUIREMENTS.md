# Product Requirements Document (PRD)

- **Product (Working Title):** True Crime Investigation App
- **Document Path:** `docs/01-product/PRODUCT_REQUIREMENTS.md`
- **Phase:** Phase 03 — Requirements (Charter PART 05)
- **Inputs:** `docs/22-progress/PO_SCOPING_DECISIONS.md`, `docs/02-requirements/EVIDENCE_INTEGRITY_SPEC.md`, `docs/23-decisions/ADR-001`–`ADR-005`
- **Status:** Draft for PO Review (Definition of Ready compliant)

---

## 1. Executive Summary

The True Crime Investigation App is a mobile-first web application that lets readers investigate real criminal cases directly through legally sourced primary records—court dockets, indictments, regulatory orders, agency releases, and verified timelines—rather than dramatized storytelling. At launch, the product ships 10 deeply researched, public-record cases (6 financial crime / money laundering cases and 4 historical public-record cases) built around a strict four-tier evidence verification standard, transparent license tracking, and mandatory disclosure of what could not be verified. Operated by a solo founder with a `$2,000` six-month budget, the MVP and v1.0 launch are free to use, delivered as an installable offline-capable Progressive Web App (PWA) with a separate admin case-authoring panel, and governed by a mandatory pre-publication legal review gate.

---

## 2. Vision Statement

Give curious readers direct, legally defensible access to primary-source case files, verified timelines, and transparent evidentiary boundaries for real financial and historical criminal cases.

---

## 3. Product Differentiation

Most true-crime apps and media platforms optimize for narrative entertainment rather than evidentiary rigor. This product differentiates specifically on **depth of primary evidence** and **legal defensibility**:

| Competitor / Category | Their Primary Model | Our Concrete Differentiation |
|---|---|---|
| **Bloody Likely** (true-crime discussion / interactive sleuthing) | Entertainment-oriented case summaries and open social speculation with minimal primary docket linkage. | Every fact in our Official Briefing traces to a **Tier 1 primary legal record** (court docket, indictment, SEC/DOJ filing) with SHA-256 file integrity hashing, explicit Verification Tier badges, and a mandatory **Genuine Gaps** panel stating what could *not* be proven. |
| **Casefile** (narrative podcast) | Linear audio storytelling synthesized from secondary books, press coverage, and transcripts; listeners cannot inspect source documents or verify claims themselves. | Interactive, searchable **primary-source dossier** where users read the underlying filings, jump from any timeline event straight to the exact source document, and compare **conflicting sources side-by-side** without editorial averaging. |
| **Law&Crime+** (live trial video & breaking crime news) | High-volume breaking courtroom video and live coverage of ongoing trials, carrying heightened pre-conviction defamation risk and shallow document indexing. | Strictly curated, **non-breaking** financial crime and historical public-record cases governed by a codified `EVIDENCE_INTEGRITY_SPEC.md` (4×3 Verification Tier × License Status display matrix), mandatory pre-publish legal sign-off (`legal_signoff_id`), prohibited-content blocks, and a public correction/takedown log. |

---

## 4. Target Users (Personas)

### Persona 1: Priya Nair — Evidence-First Financial Crime Reader
- **Age / Location / Role:** 29, Bengaluru, India — AML / Financial Compliance Analyst
- **Motivations:** Follows white-collar fraud, money laundering, and regulatory enforcement cases during her commute; frustrated by superficial YouTube summaries that omit actual SEC complaints and indictments.
- **Jobs-to-Be-Done:** Inspect primary financial-crime filings on her phone, trace shell-company timelines linked to exact docket exhibits, and search across documents to see how schemes unraveled.
- **Mapped Stories:** `US-PRIYA-01` through `US-PRIYA-06` (`docs/02-requirements/USER_STORIES.md`).

### Persona 2: Marcus Vance — Historical Public-Record Researcher
- **Age / Location / Role:** 44, Austin, TX, USA — High School History Teacher & Amateur Legal Historian
- **Motivations:** Studies historical criminal cases with fully public court records; cares deeply about distinguishing sworn testimony from later newspaper retellings and wants to read case packs offline on flights or weekends.
- **Jobs-to-Be-Done:** Filter historical cases, examine conflicting historical accounts side-by-side with source attribution, read explicit "Genuine Gaps" notes, and cache complete case packs offline on his mobile PWA.
- **Mapped Stories:** `US-MARCUS-01` through `US-MARCUS-06`.

### Persona 3: Rohan Kulkarni — Solo Founder, Admin & Lead Case Researcher
- **Age / Location / Role:** 31, Pune, India — Solo Founder (Product Owner)
- **Motivations:** Research, assemble, legally verify, and publish 10 high-integrity launch cases (scaling to 50) without a human engineering or moderation staff, while keeping legal liability and hosting costs inside a `$2,000` 6-month ceiling.
- **Jobs-to-Be-Done:** Upload primary-source PDFs with automatic SHA-256 hashing and text extraction, tag Verification Tier and License Status, author timelines and Genuine Gaps, pass the mandatory legal sign-off gate before publishing, and trigger an instant redaction/unpublish kill-switch if a valid takedown arrives.
- **Mapped Stories:** `US-ROHAN-01` through `US-ROHAN-07`.

### Persona 4: David Morales — Investigative Journalist & Fact-Checker
- **Age / Location / Role:** 41, Chicago, IL, USA — Freelance Investigative Journalist & Media Fact-Checker
- **Motivations:** Evaluates primary-source accuracy, checks whether a publication fairly attributes disputed claims, and occasionally submits factual corrections or right-of-reply notices on behalf of sources.
- **Jobs-to-Be-Done:** Audit source citations and license provenance on any claim, inspect the public Corrections & Updates log, submit a tracked correction/takedown request without needing an account, and verify DPDP data export/deletion rights when logged in.
- **Mapped Stories:** `US-DAVID-01` through `US-DAVID-06`.

---

## 5. Feature Inventory (`REQ-001` – `REQ-032`)

Every requirement below has a single testable acceptance criterion and maps 1:1 to `docs/02-requirements/FEATURE_CATALOG.md`. Priorities: **MVP** (minimum shippable dossier & legal gate for Case 1), **Launch** (v1.0 target with 10 cases), **Post-launch** (v1.1+ after validation).

### (a) Evidence Browsing & Viewing
- **`REQ-001` (MVP):** Display a catalog of published cases filterable by launch category (`financial_crime`, `historical_public_record`), showing case title, category badge, jurisdiction tag, evidence count, and last-updated date.
  - *Acceptance Criterion:* Calling `GET /api/v1/cases?category=financial_crime` returns HTTP `200` containing only `published` cases with `category = financial_crime` and excludes all `draft`, `in_legal_review`, and `unpublished` cases.
- **`REQ-002` (MVP):** Display a Case Dossier Overview containing the Official Briefing, case metadata, and a mandatory "Genuine Gaps" disclosure section listing every unverified claim, why it could not be verified, and what tier evidence exists.
  - *Acceptance Criterion:* Rendering a published case page displays the Official Briefing and a non-empty `#genuine-gaps` list matching 100% of stored `case_gaps` records; attempting to publish a case with `0` gap records fails with HTTP `422`.
- **`REQ-003` (MVP):** Provide an Evidence Item Viewer supporting PDF documents, images, and structured text transcripts with zoom/page navigation, primary source citation (docket/agency reference and URL), and SHA-256 integrity hash display.
  - *Acceptance Criterion:* Opening any published evidence item renders its content viewer (PDF pages or image/text) along with exact `source_citation`, `retrieved_at`, and 64-character `sha256_hash` matching the database record.
- **`REQ-004` (MVP):** Render Verification Tier (`Tier 1`–`Tier 4`) badges, License Status (`PUBLIC_DOMAIN`, `LICENSED`, `RECREATED`) labels, and required warning banners strictly per the 4×3 Display Rules Matrix in `EVIDENCE_INTEGRITY_SPEC.md`.
  - *Acceptance Criterion:* Automated unit and UI tests across all 12 `(verification_tier × license_status)` combinations verify that only `(Tier 1 × PUBLIC_DOMAIN)` and `(Tier 1 × LICENSED)` render without warning banners, and every `BLOCKED` combination is rejected by the API with HTTP `422` and never rendered publicly.
- **`REQ-005` (Launch):** Render conflicting source accounts for any disputed fact side-by-side with explicit per-source attribution, verification tier, and publication/filing date, without averaging or merging claims.
  - *Acceptance Criterion:* When a fact or timeline event has `>= 2` linked `conflicting_sources` records, the UI renders each account in a distinct attributed column/card showing `source_name`, `verification_tier`, and `source_date`, with no synthesized single value replacing them.
- **`REQ-006` (Launch):** Allow users to download a published case dossier (briefing, timeline, gaps, and evidence assets up to a 50 MB per-case cap) into browser Cache Storage / IndexedDB for offline reading on the mobile PWA.
  - *Acceptance Criterion:* After clicking "Save Offline" and disconnecting network connectivity in Playwright/DevTools offline mode, reloading the saved case URL renders the Official Briefing, timeline, Genuine Gaps, and cached evidence items with zero fatal errors.
- **`REQ-007` (MVP):** Deliver the reader client as an installable mobile-first PWA (React + Vite + Tailwind) with valid Web App Manifest, registered service worker, and responsive layout across `360px`–`1440px` viewports.
  - *Acceptance Criterion:* The production build serves `/manifest.webmanifest` (valid icons, `display: standalone`, `start_url`), registers a service worker returning HTTP `200`, and produces `0px` horizontal overflow (`scrollWidth <= clientWidth`) at `390×844` viewport.

### (b) Case Timelines
- **`REQ-008` (MVP):** Render a chronological Case Timeline displaying each event's date (exact date or bounded date range), summary, Verification Tier badge, and direct clickable links to supporting evidence items.
  - *Acceptance Criterion:* `GET /api/v1/cases/:id/timeline` returns events sorted ascending by `(event_start_date, sort_order)`, and clicking any linked evidence chip navigates directly to that evidence item in `REQ-003`.
- **`REQ-009` (Launch):** Allow filtering timeline events by Verification Tier (`Tier 1`–`Tier 4`), tagged entity, and event type (`filing`, `transaction`, `arrest`, `hearing`, `verdict`, `release`).
  - *Acceptance Criterion:* Applying filter query params (`?tier=1&event_type=filing`) returns only events matching all selected criteria and updates the visible event count to match the filtered array length.
- **`REQ-010` (Launch):** Flag timeline events that have conflicting source dates or descriptions with a "Disputed Sources" indicator that opens the side-by-side comparison (`REQ-005`).
  - *Acceptance Criterion:* Any timeline event with `has_conflict = true` renders a visible `Disputed Sources` button; clicking it opens the attributed side-by-side comparison panel showing all conflicting entries for that event.

### (c) Search
- **`REQ-011` (MVP):** Provide full-text search across published case titles, summaries, timeline events, and Rust-extracted evidence document text using PostgreSQL `tsvector` / GIN indexes, returning ranked matches with highlighted snippets.
  - *Acceptance Criterion:* Calling `GET /api/v1/search?q=<term>` on a seeded case containing `<term>` in an uploaded PDF's extracted text returns HTTP `200` within `500ms` (p95 on local PostgreSQL with 10 cases) with at least one result referencing that `evidence_id` and a non-empty highlighted snippet; draft/unpublished cases are excluded.
- **`REQ-012` (Launch):** Provide faceted search filtering by case ID, category, evidence type, Verification Tier, and License Status with deterministic pagination (`limit`, `offset`, `total_count`).
  - *Acceptance Criterion:* Calling `GET /api/v1/search?q=<term>&category=financial_crime&tier=1&limit=10&offset=0` returns at most 10 results all matching the specified facets and an accurate `total_count` equal to the unpaginated SQL match count.
- **`REQ-013` (Post-launch):** Provide an Entity Cross-Reference view listing every verified evidence item and timeline event across published cases mentioning a selected person, organization, or docket entity.
  - *Acceptance Criterion:* Calling `GET /api/v1/entities/:id/appearances` returns all linked published evidence items and timeline events for that entity with case IDs and verification tiers, excluding draft/redacted items.

### (d) User Accounts & Auth
- **`REQ-014` (MVP):** Permit `ANONYMOUS` visitors to browse published cases, view timelines, inspect evidence, search public content, and submit correction requests without logging in.
  - *Acceptance Criterion:* Unauthenticated requests (`no Authorization header / cookie`) to public read endpoints (`GET /api/v1/cases`, `/cases/:id`, `/timeline`, `/evidence/:id`, `/search`) and `POST /api/v1/corrections` return HTTP `200`/`201`, while all write/admin endpoints return HTTP `401`.
- **`REQ-015` (Launch):** Provide email/password registration, login, and logout with Argon2id password hashing, HTTP-only Secure `SameSite=Lax` session cookies, and explicit India DPDP Act 2023 consent checkbox + timestamped consent record.
  - *Acceptance Criterion:* Registration without `dpdp_consent = true` fails with HTTP `422`; valid registration stores an Argon2id hash (never plaintext), persists a `consent_records` row with `policy_version` and UTC timestamp, and sets an `HttpOnly; SameSite=Lax` session cookie.
- **`REQ-016` (MVP):** Enforce server-side Role-Based Access Control (RBAC) across `ADMIN`, `RESEARCHER`, `COMMUNITY_MEMBER`, and `ANONYMOUS` on every API endpoint per `docs/02-requirements/PERMISSION_MATRIX.md`.
  - *Acceptance Criterion:* Automated integration tests verifying every `DENIED` cell in `PERMISSION_MATRIX.md` confirm HTTP `401` (for `ANONYMOUS`) or HTTP `403` (for authenticated unauthorized roles) with zero state mutation.
- **`REQ-017` (Launch):** Provide self-service DPDP personal data export (`GET /api/v1/me/export`) generating a JSON archive of all personal data associated with the authenticated user.
  - *Acceptance Criterion:* An authenticated user calling `GET /api/v1/me/export` receives HTTP `200` (`application/json`) containing their profile email, `consent_records`, bookmarks, submitted theories, and correction requests, and receives HTTP `403` if attempting to request another user's ID.
- **`REQ-018` (Launch):** Provide self-service DPDP account deletion and consent withdrawal (`DELETE /api/v1/me`), purging PII within 72 hours (immediate logical erasure + background/synchronous PII scrub) while retaining anonymized legal-defense audit logs.
  - *Acceptance Criterion:* Calling `DELETE /api/v1/me` with password confirmation returns HTTP `200`, immediately invalidates active sessions (subsequent `/me` returns `401`), removes email/display-name PII from `users`, and preserves anonymized `audit_logs` rows referencing only an opaque hash.

### (e) Community Theories *(Post-launch — reflects solo-founder no-continuous-moderation constraint)*
- **`REQ-019` (Post-launch):** Allow authenticated `COMMUNITY_MEMBER` users to post structured case theories that cite at least one published `evidence_id` and require explicit acceptance of the living-person/defamation rules.
  - *Acceptance Criterion:* `POST /api/v1/cases/:id/theories` without at least one valid published `evidence_id` or without `policy_ack = true` returns HTTP `422`; valid submissions persist and render with clickable evidence citations.
- **`REQ-020` (Post-launch):** Allow authenticated users to report a community theory with a reason code (`defamation`, `doxxing`, `living_person_accusation`, `harassment`, `spam`), automatically quarantining (hiding from public view) any theory receiving `>= 1` `defamation`/`doxxing`/`living_person_accusation` report or `>= 3` total reports.
  - *Acceptance Criterion:* Submitting a `defamation` report against a visible theory immediately transitions its status to `quarantined`, removes it from `GET /api/v1/cases/:id/theories` public responses, and enqueues an item in the admin moderation queue.
- **`REQ-021` (Post-launch):** Provide an `ADMIN` asynchronous moderation queue to review quarantined/reported theories and execute `approve`, `remove`, or `suspend_user` actions with mandatory reason codes and audit logging.
  - *Acceptance Criterion:* An `ADMIN` calling `POST /api/v1/admin/theories/:id/moderate` with `action=remove` and `reason_code` updates the theory status to `removed`, appends an immutable `moderation_audit` record, and returns HTTP `403` for `RESEARCHER` and `COMMUNITY_MEMBER` roles.
- **`REQ-022` (Post-launch):** Enforce automated server-side rate limits (`<= 3` theories per 24 hours per user) and external-link / unconvicted-living-person accusation guardrails on theory submission.
  - *Acceptance Criterion:* Submitting a 4th theory within 24 hours from the same account returns HTTP `429` (`Retry-After` header set), and submitting text containing raw external URLs outside the app's evidence citations returns HTTP `422`.

### (f) Admin Case Builder
- **`REQ-023` (MVP):** Provide authenticated `ADMIN` and `RESEARCHER` users in the Admin Panel CRUD workflows for draft cases (title, slug, category, jurisdiction, `eu_origin` flag, summary, Genuine Gaps items) isolated from public endpoints.
  - *Acceptance Criterion:* Creating a draft case via `POST /api/v1/admin/cases` returns HTTP `201` with `status = draft`, is visible on `GET /api/v1/admin/cases/:id`, and returns HTTP `404` on public `GET /api/v1/cases/:slug`.
- **`REQ-024` (MVP):** Provide evidence file ingestion in the Admin Panel that sends uploaded PDFs/images/text to the Rust document processor (`processor-rust`) for SHA-256 hashing, metadata extraction, and text indexing, requiring `verification_tier`, `license_status`, `source_citation`, and `license_record_id` (mandatory when `license_status = LICENSED`).
  - *Acceptance Criterion:* Uploading a PDF triggers SHA-256 computation and text extraction stored in PostgreSQL; omitting `source_citation` or omitting `license_record_id` when `license_status = LICENSED` returns HTTP `422` and rejects persistence.
- **`REQ-025` (MVP):** Allow `ADMIN` and `RESEARCHER` users to create, edit, reorder, attach evidence IDs to, and record conflicting sources on draft case timeline events.
  - *Acceptance Criterion:* Saving a timeline event with linked `evidence_ids` and optional `conflicting_sources` persists in PostgreSQL and is reflected in exact chronological order on preview `GET /api/v1/admin/cases/:id/timeline`.
- **`REQ-026` (MVP):** Restrict permanent deletion of evidence items strictly to `ADMIN` (`RESEARCHER` denied), requiring a deletion reason, recording an append-only audit log entry (with SHA-256 hash), and blocking hard deletion of evidence attached to a `published` case unless first unpublished or redacted via `REQ-031`.
  - *Acceptance Criterion:* `DELETE /api/v1/admin/evidence/:id` returns HTTP `403` for `RESEARCHER`, returns HTTP `409` for `ADMIN` if attached to a `published` case, and for a `draft` case deletes the record while inserting a non-deletable row in `audit_logs`.

### (g) Legal Review Workflow
- **`REQ-027` (MVP):** Enforce a server-side case publication state machine (`draft` -> `in_legal_review` -> `approved` -> `published`) where transitioning to `published` is strictly restricted to `ADMIN` and requires: (1) zero `BLOCKED` `(tier × license)` evidence items, (2) `>= 1` Genuine Gaps entry, (3) completed Prohibited Content checklist, and (4) a non-empty `legal_signoff_id` referencing external legal counsel sign-off.
  - *Acceptance Criterion:* Attempting `POST /api/v1/admin/cases/:id/publish` when any of the 4 preconditions is missing (or when called by `RESEARCHER`) fails with HTTP `422` (missing gate) or HTTP `403` (role denied) and leaves `status` unchanged; satisfying all 4 transitions `status` to `published` and records `published_at` and `legal_signoff_id`.
- **`REQ-028` (MVP):** Enforce automated pre-review jurisdiction and prohibited-content gates blocking `in_legal_review` / `published` transitions if `eu_origin = true` without `gdpr_verified = true`, or if any evidence item has `prohibited_content_cleared != true`.
  - *Acceptance Criterion:* Attempting to submit or publish a case with `eu_origin = true, gdpr_verified = false` or with any evidence item having `prohibited_content_cleared = false` returns HTTP `422` with a structured list of blocking violations.
- **`REQ-029` (Launch):** Record an immutable JSON snapshot of every published case revision (briefing, timeline, gaps, evidence IDs + SHA-256 hashes, citations, `legal_signoff_id`) upon every publish or update.
  - *Acceptance Criterion:* Publishing revision `v1` and subsequently publishing revision `v2` retains both immutable snapshots in `case_snapshots` (`UPDATE`/`DELETE` blocked at DB/service layer), retrievable by `ADMIN` via `GET /api/v1/admin/cases/:id/snapshots`.

### (h) Correction & Takedown Requests
- **`REQ-030` (MVP):** Provide a public Correction, Takedown & DPDP Grievance submission form (`POST /api/v1/corrections`) accessible to `ANONYMOUS` and authenticated users, capturing case/evidence reference, request type (`factual_correction`, `defamation_takedown`, `copyright_takedown`, `privacy_dpdp_grievance`), requester contact email, and supporting citation, returning a unique tracking ID.
  - *Acceptance Criterion:* Submitting a valid payload to `POST /api/v1/corrections` returns HTTP `201` with a unique `ticket_id` and UTC `created_at`, persists the record in `correction_requests`, and enforces rate limiting (`<= 5` requests/hour per IP, returning `429` above limit).
- **`REQ-031` (MVP):** Provide an `ADMIN` emergency kill-switch (`POST /api/v1/admin/cases/:id/unpublish` and `POST /api/v1/admin/evidence/:id/redact`) that immediately removes a case or redacts an evidence item from all public API responses and invalidates cache headers/ETags with a mandatory reason code.
  - *Acceptance Criterion:* Within `<= 60` seconds of invoking the kill-switch, public `GET` requests for the unpublished case return HTTP `404` (or redacted tombstone for a single evidence item), search results exclude the removed text, and an immutable `audit_logs` entry is recorded.
- **`REQ-032` (Launch):** Display a public, timestamped "Corrections & Updates" log on every published case page showing all approved post-publication corrections, redactions, or newly added primary sources.
  - *Acceptance Criterion:* When an `ADMIN` publishes a correction note tied to a case, `GET /api/v1/cases/:slug/corrections` returns the ordered list of entries (`date`, `summary`, `affected_section`) and the case page renders them under `#corrections-log`.

---

## 6. Explicit Non-Goals

To protect legal defensibility, budget (`$2,000`), and solo-founder operational sustainability, this product explicitly does **NOT** include:

1. **No raw autopsy photos or crime-scene gore** (zero exceptions; permanently prohibited).
2. **No user-generated case submissions** (all cases are researched, verified, and legally reviewed by the founder/authorized researchers).
3. **No real-time / breaking crime reporting or active-trial coverage** (cases are curated financial crimes and historical public-record matters, never breaking news).
4. **No AI-generated case narratives or AI-synthesized factual conclusions** (every factual statement in Official Briefings must be human-authored and attributed to verifiable primary records; AI may not invent case facts).
5. **No billing, subscriptions, or payment processing in MVP or v1.0** (Billing / Domain 21 is explicitly deferred until after `1,000` MAU and `>= 3` case drops).
6. **No native iOS/Android Tauri builds in MVP or v1.0** (mobile-first PWA is the sole shipped reader client).
7. **No EU-origin cases at launch** (blocked unless GDPR compliance is explicitly verified and documented).
8. **No cold-case DNA/genetic genealogy or broad document-heavy art-theft/corruption categories at launch** (deferred to v1.1).
9. **No 24/7 on-call SLA or continuous human moderation queue** (community theories deferred to post-launch with auto-quarantine guardrails).

---

## 7. Success Metrics

### MVP Success Metrics (First 1–3 Cases Shipped)
1. **Legal & Evidentiary Zero-Defect Gate:** `100%` of published cases pass `REQ-027`/`REQ-028` with `0` unattributed Tier 3/4 claims in Official Briefings, `>= 1` Genuine Gaps disclosure, and documented external legal counsel sign-off (`legal_signoff_id`) prior to Case 1 publication.
2. **Search & Evidence Performance:** Full-text search (`REQ-011`) p95 latency `<= 500ms` and mobile PWA Largest Contentful Paint (LCP) `<= 2.5s` on simulated 4G (`390×844` viewport).
3. **Emergency Takedown Responsiveness:** Kill-switch (`REQ-031`) removes targeted evidence from public API and search index in `<= 60 seconds` in automated end-to-end tests.
4. **Solo-Founder Authoring Efficiency:** End-to-end case ingestion (upload PDFs, Rust extraction, timeline creation, legal review submission, publish) completes in `<= 4 hours` of tooling time per case (excluding external research).

### v1.0 Launch Success Metrics (10 Cases Shipped: 6 Financial Crimes + 4 Historical Public Records)
1. **Launch Catalog Completeness:** `10` legally signed-off cases live (`6` `financial_crime`, `4` `historical_public_record`), each containing `>= 5` Tier 1 primary legal records and a complete Genuine Gaps section.
2. **Reader Engagement & Depth:** `>= 1,000` Monthly Active Users (MAU) within 6 months of v1.0 launch (monetization evaluation threshold), with `>= 40%` of case readers opening `>= 2` primary evidence documents per session.
3. **Offline PWA Adoption:** `>= 15%` of returning mobile users install the PWA or save at least 1 case pack offline (`REQ-006`/`REQ-007`).
4. **Legal & Compliance Hygiene:** `100%` of valid correction/takedown requests (`REQ-030`) acknowledged within `72 hours`, `100%` of DPDP export/erasure requests (`REQ-017`/`REQ-018`) fulfilled within `72 hours`, and `0` unresolved copyright or defamation disputes.
5. **Budget Adherence:** Total 6-month spend stays `<= $2,000` (`$1,000–$1,500` legal counsel, `$300–$500` infra, `<= $200` PACER/FOIA, `$0` paid third-party APIs).

---

## 8. Constraints

### From PO Scoping Decisions (`docs/22-progress/PO_SCOPING_DECISIONS.md`)
- **Jurisdiction:** India primary operating jurisdiction for user data (India DPDP Act 2023 compliance); US primary content jurisdiction (FOIA-eligible federal works, DOJ/SEC/ED press releases, PACER/CourtListener public court records); EU-origin cases blocked at launch unless GDPR compliance is explicitly verified.
- **Monetization:** Free for MVP and v1.0; Billing (Domain 21) out of scope until `>= 1,000` MAU and `>= 3` case drops.
- **Team:** Solo founder + 1 AI implementation agent; zero human ops/moderation staff; architecture must minimize operational toil.
- **Budget:** `$2,000` 6-month hard ceiling (`$1,000–$1,500` legal review required before Case 1 publishes; `$300–$500` infra; `$200` PACER/FOIA; `$0` paid APIs — free tiers only: CourtListener, FBI Wanted, MuckRock).
- **Platform:** Mobile-first PWA (React + Vite + Tailwind) as the only shipped reader client for MVP and v1.0; separate Next.js web admin panel requested for case authoring *(architectural trade-off flagged in `REQUIREMENTS_STATUS.md`)*; native Tauri mobile builds deferred.
- **Launch Categories:** Strictly `(b) financial_crime` (6 cases) and `(c) historical_public_record` (4 cases) for the first 10 cases.

### From Technical Foundations (`docs/23-decisions/ADR-001`–`ADR-005`)
- **Stack:** Go (`1.27.4`) API backend (`backend-go/`), Rust (`1.100.0`) document/PDF processor (`processor-rust/`), PostgreSQL (`15.20`) primary datastore + FTS, Redis (`7.0.15`) cache/rate-limiting, Node (`22.23.5`) frontend tooling.
- **Local Dev & Security:** Direct local processes in sandbox (no Docker per ADR-003); `scram-sha-256` + TLS mandatory in any deployed environment (ADR-005); Go `PATH` precedence enforced (ADR-004).
