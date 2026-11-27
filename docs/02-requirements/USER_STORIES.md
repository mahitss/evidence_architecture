# User Stories (`USER_STORIES.md`)

- **Document Path:** `docs/02-requirements/USER_STORIES.md`
- **Phase:** Phase 03 — Requirements (Charter PART 05)
- **Personas Source:** `docs/01-product/PRODUCT_REQUIREMENTS.md` Section 4
- **Total User Stories:** **25** (6–7 per persona across all 4 personas, providing 100% coverage of `REQ-001` – `REQ-032`)

---

## Persona 1: Priya Nair — Evidence-First Financial Crime Reader
*29, Bengaluru, India — AML / Financial Compliance Analyst*

- **`US-PRIYA-01`** `[MVP]` — *Refs: `REQ-001`, `REQ-007`, `REQ-014`*  
  As **Priya Nair**, I want to install the mobile PWA and filter the public catalog to `financial_crime` cases without creating an account, so that I can browse white-collar enforcement dossiers cleanly on my phone during my commute.

- **`US-PRIYA-02`** `[MVP]` — *Refs: `REQ-002`, `REQ-003`, `REQ-004`*  
  As **Priya Nair**, I want to open an Official Briefing and tap directly into primary SEC complaints or DOJ indictments showing their Tier 1 badge, docket citation, and SHA-256 hash, so that I can verify factual claims against the original legal filing instead of trusting summaries.

- **`US-PRIYA-03`** `[MVP]` — *Refs: `REQ-008`, `REQ-011`*  
  As **Priya Nair**, I want to follow a chronological case timeline linked to evidence items and run full-text searches inside extracted PDF filings, so that I can trace specific shell companies and wire transfers across dozens of pages in seconds.

- **`US-PRIYA-04`** `[Launch]` — *Refs: `REQ-009`, `REQ-012`*  
  As **Priya Nair**, I want to filter timelines by Verification Tier and event type (`transaction`, `filing`) and apply faceted search filters, so that I can isolate legally established financial milestones from secondary commentary.

- **`US-PRIYA-05`** `[Launch]` — *Refs: `REQ-015`*  
  As **Priya Nair**, I want to create an account with clear India DPDP Act 2023 consent disclosure and secure session authentication, so that I can save my preferences while knowing exactly what personal data is collected.

- **`US-PRIYA-06`** `[Post-launch]` — *Refs: `REQ-013`*  
  As **Priya Nair**, I want to open an Entity Cross-Reference page for an intermediary bank or shell entity across published cases, so that I can see every verified filing and timeline event where that entity appears.

---

## Persona 2: Marcus Vance — Historical Public-Record Researcher
*44, Austin, TX, USA — High School History Teacher & Amateur Legal Historian*

- **`US-MARCUS-01`** `[MVP]` — *Refs: `REQ-001`, `REQ-014`*  
  As **Marcus Vance**, I want to filter the published catalog to `historical_public_record` cases anonymously, so that I can explore fully public historical court cases immediately without hitting a login wall.

- **`US-MARCUS-02`** `[MVP]` — *Refs: `REQ-002`, `REQ-004`*  
  As **Marcus Vance**, I want to read a mandatory Genuine Gaps panel and clear Verification Tier / License Status warnings on every case, so that I immediately know which historical records were lost or remain unverified and whether an exhibit is an original scan or a recreation.

- **`US-MARCUS-03`** `[Launch]` — *Refs: `REQ-005`, `REQ-010`*  
  As **Marcus Vance**, I want disputed historical dates and contradictory witness or newspaper accounts to be flagged on the timeline and displayed side-by-side with explicit source attribution, so that I can weigh conflicting primary and secondary records myself.

- **`US-MARCUS-04`** `[Launch]` — *Refs: `REQ-006`, `REQ-007`*  
  As **Marcus Vance**, I want to download an entire published case pack (briefing, timeline, gaps, and primary documents) to my mobile PWA for offline reading, so that I can study historical dockets on flights and weekends without internet access.

- **`US-MARCUS-05`** `[MVP]` — *Refs: `REQ-003`, `REQ-008`, `REQ-011`*  
  As **Marcus Vance**, I want to search across transcribed historical court transcripts and jump from timeline entries straight to the archived exhibit viewer, so that I can prepare accurate primary-source lessons for my history students.

- **`US-MARCUS-06`** `[Post-launch]` — *Refs: `REQ-019`, `REQ-022`*  
  As **Marcus Vance**, I want to post an evidence-cited analytical theory linking specific published archival documents on a historical case, so that I can contribute grounded historical hypotheses to the reader community.

---

## Persona 3: Rohan Kulkarni — Solo Founder, Admin & Lead Case Researcher
*31, Pune, India — Solo Founder (Product Owner)*

- **`US-ROHAN-01`** `[MVP]` — *Refs: `REQ-016`, `REQ-023`*  
  As **Rohan Kulkarni**, I want to create and edit draft cases with Genuine Gaps inside a strictly role-protected Admin Panel isolated from public endpoints, so that unfinished investigations are never leaked before legal review.

- **`US-ROHAN-02`** `[MVP]` — *Refs: `REQ-004`, `REQ-024`*  
  As **Rohan Kulkarni**, I want uploaded FOIA and PACER PDFs to be automatically SHA-256 hashed and text-extracted by the Rust processor while enforcing mandatory Verification Tier, License Status, and `license_record_id` / `recreation_source_ids` validation, so that I cannot accidentally ingest unattributed or blocked material.

- **`US-ROHAN-03`** `[MVP]` — *Refs: `REQ-025`, `REQ-026`*  
  As **Rohan Kulkarni**, I want to build chronological timelines with attached evidence and conflicting sources while restricting permanent evidence deletion to `ADMIN` with mandatory audit logs and published-case locks, so that I maintain an intact legal chain-of-custody even if I later collaborate with a researcher.

- **`US-ROHAN-04`** `[MVP]` — *Refs: `REQ-027`, `REQ-028`*  
  As **Rohan Kulkarni**, I want a hard server-side legal publication state machine that blocks publishing unless counsel sign-off (`legal_signoff_id`), Genuine Gaps, prohibited-content clearance, and EU/GDPR checks are satisfied, so that my `$2,000`-budget solo operation never ships an unvetted defamation or copyright liability.

- **`US-ROHAN-05`** `[MVP]` — *Refs: `REQ-031`*  
  As **Rohan Kulkarni**, I want a one-action emergency unpublish and evidence redaction kill-switch that clears public API responses and search results within 60 seconds, so that I can respond immediately to any credible legal takedown or privacy notice.

- **`US-ROHAN-06`** `[Launch]` — *Refs: `REQ-029`, `REQ-032`*  
  As **Rohan Kulkarni**, I want every published revision archived as an immutable snapshot and every post-publish fix recorded in a public Corrections & Updates log, so that I can prove exactly what was live on any date and demonstrate editorial good faith.

- **`US-ROHAN-07`** `[Post-launch]` — *Refs: `REQ-020`, `REQ-021`, `REQ-022`*  
  As **Rohan Kulkarni**, I want reported defamation/doxxing community theories to auto-quarantine immediately and land in an asynchronous moderation queue with audit logs and automated rate limits, so that I can safely support community theories later without hiring 24/7 moderators.

---

## Persona 4: David Morales — Investigative Journalist & Fact-Checker
*41, Chicago, IL, USA — Freelance Investigative Journalist & Media Fact-Checker*

- **`US-DAVID-01`** `[MVP]` — *Refs: `REQ-003`, `REQ-004`*  
  As **David Morales**, I want to inspect the exact docket citation, license provenance (`PUBLIC_DOMAIN` / `LICENSED` / `RECREATED`), SHA-256 hash, and warning banners on any exhibit, so that I can independently audit whether the app's claims match the underlying public record.

- **`US-DAVID-02`** `[Launch]` — *Refs: `REQ-005`, `REQ-010`, `REQ-012`*  
  As **David Morales**, I want to run faceted searches filtered to Tier 1 primary filings and examine side-by-side conflicting source panels on disputed events, so that I can verify whether the publication fairly attributes competing accounts without bias.

- **`US-DAVID-03`** `[MVP]` — *Refs: `REQ-014`, `REQ-030`*  
  As **David Morales**, I want to submit a factual correction, right-of-reply notice, or takedown request with supporting citations without creating an account and receive a unique tracking ID, so that errors or legal concerns are formally logged and traceable.

- **`US-DAVID-04`** `[Launch]` — *Refs: `REQ-032`*  
  As **David Morales**, I want to review a public, timestamped Corrections & Updates log on every case page, so that I can verify that reported inaccuracies or redactions are transparently documented rather than silently scrubbed.

- **`US-DAVID-05`** `[Launch]` — *Refs: `REQ-015`, `REQ-017`, `REQ-018`*  
  As **David Morales**, I want to export all personal data linked to my account in JSON and permanently delete my account on demand, so that I can test and exercise self-service DPDP privacy rights without emailing support.

- **`US-DAVID-06`** `[Post-launch]` — *Refs: `REQ-019`, `REQ-020`*  
  As **David Morales**, I want to flag any community theory that makes an unverified accusation against a living person and see it immediately quarantined from public view, so that reader comments never compromise the platform's legal defensibility.

---

## Requirement Coverage Cross-Check (`32 / 32` Covered)

| Requirement Range | Covering User Stories |
|---|---|
| `REQ-001` – `REQ-007` | `US-PRIYA-01`, `US-PRIYA-02`, `US-MARCUS-01`, `US-MARCUS-02`, `US-MARCUS-03`, `US-MARCUS-04`, `US-MARCUS-05`, `US-ROHAN-02`, `US-DAVID-01`, `US-DAVID-02` |
| `REQ-008` – `REQ-010` | `US-PRIYA-03`, `US-PRIYA-04`, `US-MARCUS-03`, `US-MARCUS-05`, `US-DAVID-02` |
| `REQ-011` – `REQ-013` | `US-PRIYA-03`, `US-PRIYA-04`, `US-PRIYA-06`, `US-MARCUS-05`, `US-DAVID-02` |
| `REQ-014` – `REQ-018` | `US-PRIYA-01`, `US-PRIYA-05`, `US-MARCUS-01`, `US-ROHAN-01`, `US-DAVID-03`, `US-DAVID-05` |
| `REQ-019` – `REQ-022` | `US-MARCUS-06`, `US-ROHAN-07`, `US-DAVID-06` |
| `REQ-023` – `REQ-026` | `US-ROHAN-01`, `US-ROHAN-02`, `US-ROHAN-03` |
| `REQ-027` – `REQ-029` | `US-ROHAN-04`, `US-ROHAN-06` |
| `REQ-030` – `REQ-032` | `US-ROHAN-05`, `US-ROHAN-06`, `US-DAVID-03`, `US-DAVID-04` |
