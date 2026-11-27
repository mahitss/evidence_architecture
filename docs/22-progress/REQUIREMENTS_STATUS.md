# Phase 03 Requirements Status & Readiness Audit

- **Document Path:** `docs/22-progress/REQUIREMENTS_STATUS.md`
- **Phase:** Phase 03 — Requirements (Charter PART 05 & PART 12 Definition of Ready)
- **Last Updated:** 2026-11-27 *(All 5 PO open questions resolved via `ADR-006`–`ADR-009` and `EVIDENCE_INTEGRITY_SPEC.md §2`)*
- **PO Scoping Decisions Confirmation:** **CONFIRMED** — All six Product Owner answers from Phase A are recorded **verbatim** in `docs/22-progress/PO_SCOPING_DECISIONS.md` and traced across all Phase 03 deliverables and ADRs `006`–`009`.

---

## 1. Deliverables Inventory & Completeness

| Phase | Deliverable Path | Status | Verification Summary |
|---|---|---|---|
| A | `docs/22-progress/PO_SCOPING_DECISIONS.md` | Complete | All 6 PO scoping answers recorded verbatim + 4 surfaced tensions logged. |
| B | `docs/01-product/PRODUCT_REQUIREMENTS.md` | Complete | All 8 PRD sections, 4 concrete personas, 32 numbered `REQ-001`–`REQ-032`, 9 explicit non-goals, MVP + v1.0 metrics, PO + ADR constraints. |
| C | `docs/02-requirements/EVIDENCE_INTEGRITY_SPEC.md` | Complete *(Updated 2026-11-27)* | 4 Verification Tiers, 3 License Statuses, hard third-party PACER exhibit rule + DB `CHECK` constraint (Q5), full 4×3 (12-cell) Display Rules Matrix, Genuine Gaps format, conflicting-sources rules, 6 Prohibited Content categories with legal citations. |
| D | `docs/02-requirements/FEATURE_CATALOG.md` | Complete | 32/32 features mapped 1:1 with categories, priorities, dependencies, stories, testable acceptance criteria, and test types. *(See ADR-007 override for `REQ-015`/`017`/`018` in §2 below.)* |
| E | `docs/02-requirements/PERMISSION_MATRIX.md` | Complete | 4 roles × 13 actions matrix (`ALLOWED` / `DENIED` / `ALLOWED_WITH_APPROVAL`) + mandatory server-side enforcement policy. |
| F | `docs/02-requirements/USER_STORIES.md` | Complete | 25 user stories (6–7 per persona across 4 personas) covering 32/32 `REQ-xxx` IDs. |
| G | `docs/22-progress/REQUIREMENTS_STATUS.md` | Complete *(Updated 2026-11-27)* | This status, legal-risk readiness report, and PO decision resolution record. |
| ADRs | `docs/23-decisions/ADR-006`–`ADR-009` | Complete *(Added 2026-11-27)* | `ADR-006` (Unified React+Vite+Tailwind `admin-web`), `ADR-007` (Defer reader accounts to v1.1), `ADR-008` (Geoblock EU/EEA at launch), `ADR-009` (Defer Community Theories to v1.1, schema-ready). |

---

## 2. Feature Count & Acceptance Criteria Audit

| Priority Tier | Original P6 Baseline | Effective Count (Post `ADR-007`) | Share | Notes |
|---|---:|---:|---:|---|
| **MVP** | 17 | **17** | 53.1% | Unchanged (`REQ-001`–`004`, `007`, `008`, `011`, `014`, `016`, `023`–`028`, `030`, `031`). Internal `ADMIN`/`RESEARCHER` auth retained. |
| **Launch (v1.0)** | 10 | **7** | 21.9% | Anonymous reading only (`REQ-005`, `006`, `009`, `010`, `012`, `029`, `032`). |
| **Post-launch (v1.1+)** | 5 | **8** | 25.0% | `REQ-013`, `REQ-019`–`022` (`ADR-009`) **plus `REQ-015`, `REQ-017`, `REQ-018` moved from Launch to v1.1 per `ADR-007`** (gated on funded India DPDP counsel). |
| **Total Features** | **32** | **32** | **100.0%** | All 32 requirements accounted for. |

- **Features with NO clear, testable acceptance criteria:** **`0`** *(32 of 32 requirements audited against Charter PART 12 Definition of Ready; every requirement specifies concrete HTTP status codes, schema/UI assertions, latency/timing bounds, or matrix coverage).*

---

## 3. Features with Legal-Risk Flags

Every feature carrying meaningful exposure under **Defamation**, **Copyright**, or **Privacy (India DPDP Act 2023 / EU GDPR)** is flagged below for Phase 06 (Architecture) controls and pre-launch legal counsel review:

| Feature ID | Effective Priority | Legal Risk Flag(s) | Specific Legal Exposure | Required Architectural / Editorial Mitigation |
|---|---|---|---|---|
| `REQ-002` | MVP | **Defamation** | Official Briefing summarizing criminal allegations without clarifying unverified claims or non-conviction status can lose US Fair Report Privilege and trigger libel claims (US state law / India BNS § 356). | Restrict Official Briefing strictly to Tier 1 primary records; enforce mandatory non-empty Genuine Gaps section (`REQ-027`). |
| `REQ-003` | MVP | **Copyright**, **Privacy** | Hosting raw PDF/image exhibits may accidentally expose copyrighted third-party attachments inside PACER dockets (17 U.S.C. § 504) or unredacted victim/witness PII. | Mandatory SHA-256 provenance, license classification (`REQ-024`), DB `CHECK` constraint blocking `third_party_exhibit` + `PUBLIC_DOMAIN` (`EVIDENCE_INTEGRITY_SPEC.md §2`), and pre-publish PII/prohibited-content gate (`REQ-028`). |
| `REQ-004` | MVP | **Defamation**, **Copyright** | Mislabeling secondary journalism (Tier 3) or self-published copy (Tier 4) as primary fact, or omitting `LICENSED` attribution / `RECREATED` watermarks, creates misrepresentation and derivative-work liability. | Server-side 4×3 Display Rules Matrix enforcement rejecting `BLOCKED` pairs (`HTTP 422`) and missing `license_record_id` / `recreation_source_ids`. |
| `REQ-005` | Launch | **Defamation** | Averaging or selectively omitting competing accounts of a disputed allegation undermines fair and accurate reporting defenses. | Side-by-side attributed rendering with explicit dates, tiers, and controlling judicial disposition callout. |
| `REQ-006` | Launch | **Defamation**, **Copyright**, **Privacy** | Offline PWA cached case packs could retain a retracted/redacted document on a user's device after an emergency takedown (`REQ-031`). | Architecture (Phase 06) MUST include cache-version / manifest invalidation so returning online clients immediately purge revoked/redacted case packs. |
| `REQ-010` | Launch | **Defamation** | Timeline dates presented as settled fact when sources conflict can misrepresent sequence of alleged wrongdoing. | Mandatory `Disputed Sources` indicator linked directly to side-by-side comparison (`REQ-005`). |
| `REQ-013` | Post-launch (v1.1+) | **Defamation**, **Privacy** | Cross-case entity profiles linking living individuals across multiple criminal dossiers risk defamation-by-implication and profiling concerns. | Display mandatory legal-status qualifiers (e.g., *"witness"*, *"alleged, not convicted"*), exclude redacted items, and review with counsel before v1.1 enablement. |
| `REQ-015` | Post-launch (v1.1 — `ADR-007`) | **Privacy (DPDP / GDPR)** | Collecting email/password/session data from Indian users requires explicit, informed, revocable consent under India DPDP Act 2023 (and GDPR if EU users register). | Deferred to v1.1 (`ADR-007`); gated on funded India DPDP counsel review prior to release. EU/EEA reader surface geoblocked at launch (`ADR-008`). |
| `REQ-017` | Post-launch (v1.1 — `ADR-007`) | **Privacy (DPDP / GDPR)** | Failing to provide timely data-access exports breaches DPDP Section 11 (Right to access information about personal data). | Deferred to v1.1 (`ADR-007`); self-service authenticated `GET /api/v1/me/export` JSON archive scoped strictly to session user once accounts launch. |
| `REQ-018` | Post-launch (v1.1 — `ADR-007`) | **Privacy (DPDP / GDPR)** | Retaining personal data after consent withdrawal violates DPDP Section 6/12; conversely, deleting audit records destroys legal-defense evidence. | Deferred to v1.1 (`ADR-007`); immediate session revocation + PII scrub (`<= 72h`) preserving irreversibly anonymized `audit_logs` hashes once accounts launch. |
| `REQ-019` | Post-launch (v1.1 — `ADR-009`) | **Defamation**, **Privacy** | User-posted true-crime theories accusing unconvicted living individuals carry severe defamation and India IT Rules 2021 intermediary liability. | Deferred to v1.1 (`ADR-009`); schema-ready only, no UI/queue at launch; requires funded legal review of moderation policy before enabling. |
| `REQ-020` | Post-launch (v1.1 — `ADR-009`) | **Defamation**, **Privacy** | Delayed removal of reported defamatory/doxxing UGC jeopardizes intermediary safe harbor under US CDA § 230 / India IT Act § 79. | Deferred to v1.1 (`ADR-009`); automated immediate quarantine on `>= 1` `defamation`/`doxxing`/`living_person_accusation` report when built. |
| `REQ-021` | Post-launch (v1.1 — `ADR-009`) | **Defamation**, **Privacy** | Inconsistent or undocumented moderation decisions weaken legal defense in notice-and-takedown disputes. | Deferred to v1.1 (`ADR-009`); `ADMIN`-only moderation queue with mandatory reason codes and immutable `moderation_audit` trail when built. |
| `REQ-022` | Post-launch (v1.1 — `ADR-009`) | **Defamation**, **Copyright** | External links in UGC can drive traffic to infringing or defamatory off-platform material. | Deferred to v1.1 (`ADR-009`); server-side rate limiting (`<= 3/24h`) and rejection of raw external URLs (`HTTP 422`) when built. |
| `REQ-024` | MVP | **Copyright**, **Privacy** | Ingesting unverified files without license tracking or text-extraction screening risks publishing copyrighted works or embedded PII. | Mandatory `source_type`, `license_status`, DB `CHECK` constraint blocking `third_party_exhibit` + `PUBLIC_DOMAIN` (`EVIDENCE_INTEGRITY_SPEC.md §2`), `license_record_id` / `recreation_source_ids`, SHA-256 hashing, and `prohibited_content_cleared` flag. |
| `REQ-026` | MVP | **Defamation** *(spoliation)* | Destroying evidence records during an active legal dispute creates spoliation of evidence risk. | Block hard deletion on `published` cases (`HTTP 409`), restrict deletion to `ADMIN`, and retain immutable `audit_logs` with `sha256_hash`. |
| `REQ-027` | MVP | **Defamation**, **Copyright**, **Privacy** | Publishing Case 1 without legal counsel sign-off violates the PO's hard legal precondition and exposes the founder directly. | Server-enforced state machine requiring `legal_signoff_id`, Genuine Gaps, zero blocked items, and `ADMIN` role. |
| `REQ-028` | MVP | **Privacy (GDPR / DPDP)**, **Defamation** | Publishing EU-origin criminal-offense data without GDPR Art. 10 lawful basis or publishing prohibited gore/victim PII creates acute statutory liability. | Automated server-side gate blocking `eu_origin = true` without `gdpr_verified = true` and uncleared prohibited-content flags (`HTTP 422`), reinforced by EU/EEA reader geoblocking (`ADR-008`). |
| `REQ-029` | Launch | **Defamation**, **Copyright** | Inability to prove exact wording and citations live on a historical date weakens defamation/Fair Report defense. | Immutable `case_snapshots` table (`UPDATE`/`DELETE` blocked) storing full revision payload + SHA-256 manifest. |
| `REQ-030` | MVP | **Defamation**, **Copyright**, **Privacy (DPDP)** | Lack of an accessible grievance/takedown channel violates legal redress best practices and US DMCA / defamation mitigation standards. | Public `ANONYMOUS`-accessible submission endpoint returning unique `ticket_id`, rate-limited (`<= 5/hr/IP`), minimal purpose-limited contact handling. |
| `REQ-031` | MVP | **Defamation**, **Copyright**, **Privacy** | Slow response to valid takedown notices compounds damages and legal fees. | `ADMIN` emergency unpublish/redaction kill-switch clearing public API, search index, and caches in `<= 60 seconds`. |
| `REQ-032` | Launch | **Defamation** | Silent post-publication edits appear deceptive; transparent prompt corrections mitigate defamation damages under retraction statutes. | Public, timestamped `#corrections-log` per published case. |

---

## 4. Open Questions for the Product Owner — **ALL 5 RESOLVED (2026-11-27)**

All five pre-architecture open questions have been answered by the Product Owner and codified into ADRs and specifications. **Open question count: `0`.**

| # | Question | Status | Decision ID / Spec Reference | Recorded Resolution |
|---|---|---|---|---|
| **Q1** | **`admin-web` framework:** Next.js vs. unified React + Vite + Tailwind SPA? | **RESOLVED** | **`ADR-006`** (`docs/23-decisions/ADR-006-admin-web-unified-react-vite-spa.md`) | **UNIFY on React + Vite + Tailwind SPA** (same stack as `mobile-web` PWA; **no Next.js**). Minimal ops surface, single framework mental model, no SSR needed for internal admin. |
| **Q2** | **Dual-jurisdiction legal budget (`$1,000–$1,500`):** US media law vs. India DPDP consumer compliance? | **RESOLVED** | **`ADR-007`** (`docs/23-decisions/ADR-007-defer-user-accounts-to-v1-1.md`) | **DEFER consumer user accounts (`REQ-015`, `REQ-017`, `REQ-018`) to v1.1.** MVP and v1.0 ship as **anonymous reading only**; internal `ADMIN`/`RESEARCHER` auth remains. Removes India DPDP consumer-account obligations at launch; funded India DPDP counsel is a hard prerequisite for v1.1. |
| **Q3** | **EU/EEA readers on open web PWA** vs. blocking EU-origin cases? | **RESOLVED** | **`ADR-008`** (`docs/23-decisions/ADR-008-geoblock-eu-eea-at-launch.md`) | **GEOBLOCK EU/EEA at launch** via IP-level block on the reader surface showing a *"Not available in your region"* page, alongside no EU-origin cases (`REQ-028`). Documented as a known limitation; lift only when GDPR counsel is funded. |
| **Q4** | **Community Theories (`REQ-019`–`REQ-022`):** Defer to v1.1 or cut? | **RESOLVED** | **`ADR-009`** (`docs/23-decisions/ADR-009-defer-community-theories-to-v1-1.md`) | **DEFER to v1.1 (not cut).** Design the Phase 06 data model to support them (separate tables + moderation fields), but do **NOT** build the UI tab, public submission APIs, or moderation queue at launch. Prerequisite for v1.1: funded legal review of moderation policy. |
| **Q5** | **Third-party PACER exhibits:** Auto-`PUBLIC_DOMAIN` prohibition & enforcement? | **RESOLVED** | **`EVIDENCE_INTEGRITY_SPEC.md §2`** (`docs/02-requirements/EVIDENCE_INTEGRITY_SPEC.md`) + Phase 06 DB `CHECK` constraint | **HARD RULE: NEVER auto-tag third-party PACER exhibits as `PUBLIC_DOMAIN`.** Must be cited via Tier 1 court opinion, `LICENSED` (explicit written permission on file), or `RECREATED` (original factual diagram/paraphrase). Enforced at the PostgreSQL DB layer with `CHECK (NOT (source_type = 'third_party_exhibit' AND license_status = 'PUBLIC_DOMAIN'))`. |
