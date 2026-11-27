# Phase 03 Requirements Status & Readiness Audit

- **Document Path:** `docs/22-progress/REQUIREMENTS_STATUS.md`
- **Phase:** Phase 03 — Requirements (Charter PART 05 & PART 12 Definition of Ready)
- **Date:** 2026-11-26
- **PO Scoping Decisions Confirmation:** **CONFIRMED** — All six Product Owner answers from Phase A are recorded **verbatim** in `docs/22-progress/PO_SCOPING_DECISIONS.md` and traced across all Phase 03 deliverables.

---

## 1. Deliverables Inventory & Completeness

| Phase | Deliverable Path | Status | Verification Summary |
|---|---|---|---|
| A | `docs/22-progress/PO_SCOPING_DECISIONS.md` | Complete | All 6 PO scoping answers recorded verbatim + 4 surfaced tensions logged. |
| B | `docs/01-product/PRODUCT_REQUIREMENTS.md` | Complete | All 8 PRD sections, 4 concrete personas, 32 numbered `REQ-001`–`REQ-032`, 9 explicit non-goals, MVP + v1.0 metrics, PO + ADR constraints. |
| C | `docs/02-requirements/EVIDENCE_INTEGRITY_SPEC.md` | Complete | 4 Verification Tiers, 3 License Statuses, full 4×3 (12-cell) Display Rules Matrix, Genuine Gaps format, conflicting-sources rules, 6 Prohibited Content categories with legal citations. |
| D | `docs/02-requirements/FEATURE_CATALOG.md` | Complete | 32/32 features mapped 1:1 with categories, priorities, dependencies, stories, testable acceptance criteria, and test types. |
| E | `docs/02-requirements/PERMISSION_MATRIX.md` | Complete | 4 roles × 13 actions matrix (`ALLOWED` / `DENIED` / `ALLOWED_WITH_APPROVAL`) + mandatory server-side enforcement policy. |
| F | `docs/02-requirements/USER_STORIES.md` | Complete | 25 user stories (6–7 per persona across 4 personas) covering 32/32 `REQ-xxx` IDs. |
| G | `docs/22-progress/REQUIREMENTS_STATUS.md` | Complete | This status & legal-risk readiness report. |

---

## 2. Feature Count & Acceptance Criteria Audit

| Priority Tier | Feature Count | Share |
|---|---:|---:|
| **MVP** | **17** | 53.1% |
| **Launch (v1.0)** | **10** | 31.3% |
| **Post-launch (v1.1+)** | **5** | 15.6% |
| **Total Features** | **32** | **100.0%** |

- **Features with NO clear, testable acceptance criteria:** **`0`** *(32 of 32 requirements audited against Charter PART 12 Definition of Ready; every requirement specifies concrete HTTP status codes, schema/UI assertions, latency/timing bounds, or matrix coverage).*

---

## 3. Features with Legal-Risk Flags

Every feature carrying meaningful exposure under **Defamation**, **Copyright**, or **Privacy (India DPDP Act 2023 / EU GDPR)** is flagged below for Phase 06 (Architecture) controls and pre-launch legal counsel review:

| Feature ID | Priority | Legal Risk Flag(s) | Specific Legal Exposure | Required Architectural / Editorial Mitigation |
|---|---|---|---|---|
| `REQ-002` | MVP | **Defamation** | Official Briefing summarizing criminal allegations without clarifying unverified claims or non-conviction status can lose US Fair Report Privilege and trigger libel claims (US state law / India BNS § 356). | Restrict Official Briefing strictly to Tier 1 primary records; enforce mandatory non-empty Genuine Gaps section (`REQ-027`). |
| `REQ-003` | MVP | **Copyright**, **Privacy** | Hosting raw PDF/image exhibits may accidentally expose copyrighted third-party attachments inside PACER dockets (17 U.S.C. § 504) or unredacted victim/witness PII. | Mandatory SHA-256 provenance, license classification (`REQ-024`), and pre-publish PII/prohibited-content gate (`REQ-028`). |
| `REQ-004` | MVP | **Defamation**, **Copyright** | Mislabeling secondary journalism (Tier 3) or self-published copy (Tier 4) as primary fact, or omitting `LICENSED` attribution / `RECREATED` watermarks, creates misrepresentation and derivative-work liability. | Server-side 4×3 Display Rules Matrix enforcement rejecting `BLOCKED` pairs (`HTTP 422`) and missing `license_record_id` / `recreation_source_ids`. |
| `REQ-005` | Launch | **Defamation** | Averaging or selectively omitting competing accounts of a disputed allegation undermines fair and accurate reporting defenses. | Side-by-side attributed rendering with explicit dates, tiers, and controlling judicial disposition callout. |
| `REQ-006` | Launch | **Defamation**, **Copyright**, **Privacy** | Offline PWA cached case packs could retain a retracted/redacted document on a user's device after an emergency takedown (`REQ-031`). | Architecture (Phase 06) MUST include cache-version / manifest invalidation so returning online clients immediately purge revoked/redacted case packs. |
| `REQ-010` | Launch | **Defamation** | Timeline dates presented as settled fact when sources conflict can misrepresent sequence of alleged wrongdoing. | Mandatory `Disputed Sources` indicator linked directly to side-by-side comparison (`REQ-005`). |
| `REQ-013` | Post-launch | **Defamation**, **Privacy** | Cross-case entity profiles linking living individuals across multiple criminal dossiers risk defamation-by-implication and profiling concerns. | Display mandatory legal-status qualifiers (e.g., *"witness"*, *"alleged, not convicted"*), exclude redacted items, and review with counsel before v1.1 enablement. |
| `REQ-015` | Launch | **Privacy (DPDP / GDPR)** | Collecting email/password/session data from Indian users requires explicit, informed, revocable consent under India DPDP Act 2023 (and GDPR if EU users register). | Mandatory `dpdp_consent = true`, versioned `consent_records` table, Argon2id hashing, minimal PII collection. |
| `REQ-017` | Launch | **Privacy (DPDP / GDPR)** | Failing to provide timely data-access exports breaches DPDP Section 11 (Right to access information about personal data). | Self-service authenticated `GET /api/v1/me/export` JSON archive scoped strictly to session user. |
| `REQ-018` | Launch | **Privacy (DPDP / GDPR)** | Retaining personal data after consent withdrawal violates DPDP Section 6/12; conversely, deleting audit records destroys legal-defense evidence. | Immediate session revocation + PII scrub (`<= 72h`) while preserving irreversibly anonymized `audit_logs` hashes. |
| `REQ-019` | Post-launch | **Defamation**, **Privacy** | User-posted true-crime theories accusing unconvicted living individuals carry severe defamation and India IT Rules 2021 intermediary liability. | Deferred to Post-launch; requires `>= 1` published `evidence_id`, `policy_ack`, and counsel review of safe-harbor terms before enabling. |
| `REQ-020` | Post-launch | **Defamation**, **Privacy** | Delayed removal of reported defamatory/doxxing UGC jeopardizes intermediary safe harbor under US CDA § 230 / India IT Act § 79. | Automated immediate quarantine on `>= 1` `defamation`/`doxxing`/`living_person_accusation` report. |
| `REQ-021` | Post-launch | **Defamation**, **Privacy** | Inconsistent or undocumented moderation decisions weaken legal defense in notice-and-takedown disputes. | `ADMIN`-only moderation queue with mandatory reason codes and immutable `moderation_audit` trail. |
| `REQ-022` | Post-launch | **Defamation**, **Copyright** | External links in UGC can drive traffic to infringing or defamatory off-platform material. | Server-side rate limiting (`<= 3/24h`) and rejection of raw external URLs (`HTTP 422`). |
| `REQ-024` | MVP | **Copyright**, **Privacy** | Ingesting unverified files without license tracking or text-extraction screening risks publishing copyrighted works or embedded PII. | Mandatory `license_status`, `license_record_id` / `recreation_source_ids`, SHA-256 hashing, and `prohibited_content_cleared` flag. |
| `REQ-026` | MVP | **Defamation** *(spoliation)* | Destroying evidence records during an active legal dispute creates spoliation of evidence risk. | Block hard deletion on `published` cases (`HTTP 409`), restrict deletion to `ADMIN`, and retain immutable `audit_logs` with `sha256_hash`. |
| `REQ-027` | MVP | **Defamation**, **Copyright**, **Privacy** | Publishing Case 1 without legal counsel sign-off violates the PO's hard legal precondition and exposes the founder directly. | Server-enforced state machine requiring `legal_signoff_id`, Genuine Gaps, zero blocked items, and `ADMIN` role. |
| `REQ-028` | MVP | **Privacy (GDPR / DPDP)**, **Defamation** | Publishing EU-origin criminal-offense data without GDPR Art. 10 lawful basis or publishing prohibited gore/victim PII creates acute statutory liability. | Automated server-side gate blocking `eu_origin = true` without `gdpr_verified = true` and blocking uncleared prohibited-content flags (`HTTP 422`). |
| `REQ-029` | Launch | **Defamation**, **Copyright** | Inability to prove exact wording and citations live on a historical date weakens defamation/Fair Report defense. | Immutable `case_snapshots` table (`UPDATE`/`DELETE` blocked) storing full revision payload + SHA-256 manifest. |
| `REQ-030` | MVP | **Defamation**, **Copyright**, **Privacy (DPDP)** | Lack of an accessible grievance/takedown channel violates India DPDP Grievance Redressal requirements and US DMCA / defamation mitigation best practices. | Public `ANONYMOUS`-accessible submission endpoint returning unique `ticket_id`, rate-limited (`<= 5/hr/IP`). |
| `REQ-031` | MVP | **Defamation**, **Copyright**, **Privacy** | Slow response to valid takedown notices compounds damages and legal fees. | `ADMIN` emergency unpublish/redaction kill-switch clearing public API, search index, and caches in `<= 60 seconds`. |
| `REQ-032` | Launch | **Defamation** | Silent post-publication edits appear deceptive; transparent prompt corrections mitigate defamation damages under retraction statutes. | Public, timestamped `#corrections-log` per published case. |

---

## 4. Open Questions for the Product Owner (Resolve Before Phase 06 Architecture)

The following **5 open questions** surface real tensions between the PO's scoping answers, the `$2,000` budget, the solo-founder constraint, and cross-jurisdiction compliance. Architecture (`Prompt 7`) should not assume answers silently:

1. **Admin Panel Framework (`admin-web`): Next.js vs. Unified React + Vite + Tailwind?**
   - *Tension:* PO Answer #5 specified React + Vite + Tailwind for `mobile-web` (PWA) and **Next.js** for the separate `admin-web` authoring panel (`admin-web/README.md` currently notes a pending decision against the existing Vite starter). For a single-user internal admin tool on a `$300–$500` 6-month VPS budget maintained by 1 founder + 1 AI agent, running Next.js adds a second frontend framework, separate build config, and an extra Node SSR runtime when a static React + Vite + Tailwind SPA served behind Nginx/Go achieves the same authoring workflow at near-zero RAM/ops cost.
   - *Decision Needed:* Keep **Next.js** for `admin-web` as requested, or simplify `admin-web` to a separate **React + Vite + Tailwind** SPA sharing components/types with `mobile-web`?

2. **Dual-Jurisdiction Legal Counsel Budget (`$1,000–$1,500`) — US Media Law vs. India DPDP Compliance:**
   - *Tension:* PO Answer #4 allocates `$1,000–$1,500` for legal review by a media/First Amendment lawyer in the target content market (US) covering template disclaimers, correction process, and Case 1. However, PO Answer #1 sets **India** as the primary operating jurisdiction for user data under the **India DPDP Act 2023** (and IT Rules 2021). A US First Amendment attorney generally cannot advise on Indian DPDP / BNS § 356 compliance, and `$1,000–$1,500` rarely covers two separate jurisdictions' legal engagements.
   - *Decision Needed:* Will you (a) defer user accounts (`REQ-015`, `REQ-017`, `REQ-018`) until after MVP so MVP collects zero user-account PII (limiting MVP personal data strictly to correction-form emails in `REQ-030`), or (b) allocate part of the legal budget / separate Indian counsel for DPDP privacy notice & grievance officer compliance?

3. **EU-Resident Users Signing Up on Open Web PWA vs. Blocking EU-Origin Cases:**
   - *Tension:* Blocking EU-origin *cases* (`REQ-028`) prevents EU criminal-record processing (GDPR Art. 10) for case subjects, but GDPR still applies to personal data of **EU-resident readers** if they create accounts (`REQ-015`) on a globally accessible web PWA.
   - *Decision Needed:* For Launch (`v1.0`), should we (a) block EU/EEA account registration at signup (allowing anonymous reading only), or (b) design our DPDP consent/export/deletion flows (`REQ-015`, `REQ-017`, `REQ-018`) to satisfy baseline GDPR Art. 6/15/17 requirements simultaneously (no tracking cookies, explicit opt-in, self-service JSON export + 72h deletion)?

4. **Community Theories (`REQ-019`–`REQ-022`) — Confirm Post-Launch Deferral or Cut Entirely?**
   - *Tension:* PO Answer #3 explicitly rules out a continuous human moderation queue. We scoped Community Theories to **Post-launch (v1.1+)** with 1-report instant auto-quarantine for defamation/doxxing/living-person accusations. Even with auto-quarantine, UGC increases India IT Rules / defamation surface area for a solo founder.
   - *Decision Needed:* Confirm whether `REQ-019`–`REQ-022` stay deferred to **Post-launch (v1.1+)** (zero engineering spend in MVP/v1.0, DB schema hooks optional) or should be removed from the v1 roadmap altogether.

5. **Third-Party Exhibits Attached to US Federal PACER Dockets (Copyright Policy):**
   - *Tension:* Under 17 U.S.C. § 105, works authored by US federal officers (indictments, judicial opinions, SEC complaints) are `PUBLIC_DOMAIN`, **but** third-party exhibits filed by litigants inside a PACER docket (e.g., private photos, corporate slide decks, bank emails, news clippings) generally retain their underlying third-party copyright.
   - *Decision Needed:* Confirm as an editorial/architecture rule for `Case 1`–`Case 10` that third-party docket exhibits are **never** marked `PUBLIC_DOMAIN` automatically—they must either be summarized/cited via Tier 1 court-opinion quotes, obtained under `LICENSED` permission, or `RECREATED` strictly as factual data charts/diagrams from Tier 1 text (subject to US First Amendment counsel review).
