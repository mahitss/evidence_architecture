# Evidence Integrity Specification (Core Legal & Epistemic Contract)

- **Document Path:** `docs/02-requirements/EVIDENCE_INTEGRITY_SPEC.md`
- **Phase:** Phase 03 — Requirements (Charter PART 05)
- **Governs:** `REQ-002`, `REQ-003`, `REQ-004`, `REQ-005`, `REQ-008`, `REQ-010`, `REQ-024`, `REQ-027`, `REQ-028`, `REQ-030`, `REQ-031`, `REQ-032`
- **Legal Gate:** Template disclaimers, correction workflows, and Case 1 must receive written sign-off from qualified media/First Amendment legal counsel before publication (`docs/22-progress/PO_SCOPING_DECISIONS.md` #4).

---

## 1. Verification Tiers

Every evidence item, factual assertion, and timeline event stored in the system MUST be assigned exactly one Verification Tier (`1`–`4`). Higher-numbered tiers may NEVER silently overwrite or masquerade as lower-numbered primary records.

### Tier 1 — Primary Legal Record
- **(a) What qualifies:** Official government or judicial records with direct legal provenance—court dockets, indictments, criminal complaints, plea agreements, trial/sentencing transcripts, judicial opinions, appellate rulings, SEC/DOJ/ED/FinCEN enforcement orders and official agency press releases, and verified FOIA productions.
- **(b) Conflict resolution:** When two Tier 1 records conflict (e.g., an initial indictment allegation vs. a later superseding indictment, acquittal, or amended judgment), the **most recent final judicial disposition governs legal status**, while both records are preserved and attributed chronologically (never silently deleted).
- **(c) UI display:** Displays solid **Tier 1 — Primary Legal Record** badge with docket/agency number, issuing authority, filing/publication date, and source link/SHA-256 hash. (Subject to License Status rules in Section 3.)
- **(d) Eligible for "Official Briefing" tab:** **YES** (subject to Section 3 Display Rules Matrix — only `PUBLIC_DOMAIN` or `LICENSED` items appear without warnings; `RECREATED` items carry an explicit recreation disclosure).

### Tier 2 — Direct Subject Statement
- **(a) What qualifies:** Verbatim, on-the-record statements where a subject, witness, prosecutor, defense attorney, or official speaks in their own words—long-form recorded interviews, sworn congressional/committee testimony transcripts (outside formal court filings), published memoirs/autobiographies, or direct written public statements/letters.
- **(b) Conflict resolution:** Never overrides a Tier 1 judicial finding. Where a Tier 2 statement contradicts a Tier 1 record (e.g., a defendant's interview denying facts established in a guilty plea or verdict), both are shown side-by-side (Section 5) with clear attribution ("In a 2022 interview, X stated...; however, the 2021 plea agreement states...").
- **(c) UI display:** Displays **Tier 2 — Direct Subject Statement** badge plus an amber context banner: *"First-person statement — reflects the speaker's own account, not an independent court finding."*
- **(d) Eligible for "Official Briefing" tab:** **NO** as standalone factual basis. May appear ONLY in the **Evidence / Context** or **Conflicting Accounts** panels (or explicitly quoted as a secondary attributed perspective, never stated as established fact in Official Briefing).

### Tier 3 — Journalistic Paraphrase
- **(a) What qualifies:** Reporting, investigative articles, or summaries from established, reputable news organizations with editorial fact-checking standards (e.g., Reuters, AP, WSJ, NYT, ProPublica, Financial Times) that paraphrase events or cite unnamed/secondary sources without reproducing the underlying primary filing.
- **(b) Conflict resolution:** Subordinated to Tier 1 and Tier 2. If two Tier 3 outlets disagree, attribute both explicitly by outlet and date; if a Tier 3 report conflicts with a Tier 1 filing, flag the discrepancy and cite the Tier 1 filing as controlling.
- **(c) UI display:** Displays **Tier 3 — Journalistic Report (Secondary)** badge plus a mandatory amber warning banner: *"Secondary reporting — paraphrased by journalists; primary legal record not independently reproduced here."*
- **(d) Eligible for "Official Briefing" tab:** **NO**. Strictly excluded from the Official Briefing tab; permitted only in supplementary background/context panels with mandatory warning banners and proper licensing/fair-use citation links (full article text is NEVER scraped or mirrored without a license).

### Tier 4 — Marketing / Bio / Self-Description Copy
- **(a) What qualifies:** Promotional websites, corporate press releases, LinkedIn/social bios, speaker profiles, book blurbs, or self-published PR statements produced by a subject or their organization.
- **(b) Conflict resolution:** Treated purely as evidence of *what a party publicly claimed about themselves at a point in time*—never as proof of underlying fact. Contradictions with Tier 1 records (e.g., a fraudulent fund's promotional claim vs. SEC complaint) must be explicitly labeled *"Self-described claim (contradicted by Tier 1 filing)."*
- **(c) UI display:** Displays **Tier 4 — Self-Description / Unverified Claim** badge plus a prominent high-visibility warning banner: *"Self-published promotional or biographical claim — not independently verified."*
- **(d) Eligible for "Official Briefing" tab:** **NO**. Blocked from Official Briefing; allowed only as explicitly labeled contextual exhibits (e.g., demonstrating alleged misrepresentations in a financial-crime case when paired with a Tier 1 enforcement record).

---

## 2. License Statuses

Every evidence asset (document, photo, audio/text excerpt, diagram) MUST carry exactly one of three License Status values:

### `PUBLIC_DOMAIN`
- **What qualifies:** Works of the U.S. federal government (17 U.S.C. § 105 — federal court opinions, DOJ/SEC/FBI press releases, federal indictments/dockets authored by federal officers), works whose copyright term has verifiably expired (historical public-record cases), or works explicitly dedicated to the public domain (e.g., CC0).
- **Hard Rule — Third-Party PACER / Docket Exhibits (PO Decision Q5):** Third-party exhibits attached to a federal PACER or court docket (such as private photographs, corporate slide decks, bank emails, or third-party reports filed by litigants) are **NEVER** auto-tagged or permitted as `PUBLIC_DOMAIN` solely because they appear in a court filing. Every third-party docket exhibit MUST be handled strictly through one of three permitted paths:
  1. **Cited via a Tier 1 court opinion / judicial order** (quoting or referencing the court's own Tier 1 description without republishing the underlying copyrighted exhibit file),
  2. **`LICENSED`** (with explicit written rights-holder permission archived via a mandatory `license_record_id`), or
  3. **`RECREATED`** (our own original factual diagram or structured paraphrase created strictly from Tier 1/Tier 2 text, linked via `recreation_source_ids` and watermarked).
- **Database Enforcement Contract (Phase 06 Schema Requirement):** The `evidence_items` table MUST include a `source_type` column (including `'third_party_exhibit'`) and enforce a database-level `CHECK` constraint that raises a constraint violation whenever `evidence_items.source_type = 'third_party_exhibit' AND license_status = 'PUBLIC_DOMAIN'`:
  ```sql
  CONSTRAINT chk_no_public_domain_third_party_exhibit
    CHECK (NOT (source_type = 'third_party_exhibit' AND license_status = 'PUBLIC_DOMAIN'))
  ```
- **Required provenance evidence:** `source_type`, `source_url` (or FOIA/PACER docket reference), `issuing_authority` (or expiration year justification for historical works), and `license_basis_note` stored on the evidence record.
- **UI display:** Subtle neutral label: `Public Domain (Primary Gov/Court Record)` or `Public Domain (Expired Copyright)` with clickable provenance citation.

### `LICENSED`
- **What qualifies:** Copyrighted works (e.g., photographs, news excerpts, archival scans, state court transcripts where copyright is claimed, or licensed media) for which the publisher holds an explicit written license, Creative Commons license (with attribution compliance), or formal permission from the rights holder.
- **Required provenance evidence:** Mandatory `license_record_id` referencing an internal license ledger record containing rights holder name, license type/scope (e.g., `CC-BY-4.0`, `Direct Written Grant`), territory/term, required credit line, and archived proof of permission. **Server rejects saving `LICENSED` items without `license_record_id`.**
- **UI display:** Displays `Licensed` tag with the exact required rights-holder attribution / credit line directly beneath the asset.

### `RECREATED`
- **What qualifies:** Original diagrams, financial flowcharts, redacted/re-typeset document transcriptions, architectural floorplans, or visual reconstructions created in-house strictly from official Tier 1/Tier 2 descriptions when the original visual is unavailable, copyrighted, or visually restricted.
- **Required provenance evidence:** Mandatory `recreation_source_ids` (linking to the underlying Tier 1/Tier 2 citation(s) describing the reconstructed content) and `recreation_author_note` stating what was reconstructed and what details were omitted.
- **UI display:** Mandatory persistent visual watermark/frame label on the asset AND prominent disclosure banner: **`RECREATED VISUAL — Created from official descriptions (see cited source); not an original photograph or filing scan.`**

---

## 3. Display Rules Matrix (`Verification Tier × License Status`)

All 12 combinations (`4 × 3`) are enforced **server-side** (`REQ-004`, `REQ-027`) and mirrored in the PWA UI. Only **Tier 1 × `PUBLIC_DOMAIN`** and **Tier 1 × `LICENSED`** may appear without warning banners.

| Verification Tier | `PUBLIC_DOMAIN` | `LICENSED` | `RECREATED` |
|---|---|---|---|
| **Tier 1**<br>*(Primary Legal Record)* | **ALLOWED — CLEAN (No Warning).**<br>Full **Tier 1** badge + `Public Domain` provenance tag. Eligible for **Official Briefing**, Timeline, and Evidence Viewer. | **ALLOWED — CLEAN (No Warning).**<br>Full **Tier 1** badge + `Licensed` attribution/credit line (`license_record_id` required). Eligible for **Official Briefing**, Timeline, and Evidence Viewer. | **ALLOWED WITH WARNING.**<br>Full **Tier 1** badge + mandatory **`RECREATED`** visual frame/watermark + banner *"Recreated from Tier 1 record — not an original scan."* Allowed in Evidence Viewer/Timeline; Official Briefing requires inline `[Recreated]` tag. |
| **Tier 2**<br>*(Direct Subject Statement)* | **ALLOWED WITH WARNING.**<br>**Tier 2** badge + `Public Domain` tag + amber warning banner (*"First-person statement — not an independent court finding"*). **Excluded from Official Briefing.** | **ALLOWED WITH WARNING.**<br>**Tier 2** badge + `Licensed` credit line (`license_record_id` required) + amber warning banner (*"First-person statement — not an independent court finding"*). **Excluded from Official Briefing.** | **ALLOWED WITH DOUBLE WARNING.**<br>**Tier 2** badge + **`RECREATED`** watermark + double amber banner (*"First-person statement"* + *"Recreated visual, not original"*). **Excluded from Official Briefing.** |
| **Tier 3**<br>*(Journalistic Paraphrase)* | **ALLOWED WITH WARNING (Link/Citation Only; Historical PD Only).**<br>**Tier 3** badge + `Public Domain` tag (e.g., pre-1930 newspaper) + amber warning (*"Secondary journalistic report — primary record not reproduced"*). **Excluded from Official Briefing.** | **ALLOWED WITH WARNING (Licensed Excerpt/Link Only).**<br>**Tier 3** badge + `Licensed` credit (`license_record_id` required) + amber warning (*"Secondary journalistic report"*). **Excluded from Official Briefing.** | **BLOCKED FROM PUBLICATION.**<br>API returns HTTP `422` (`EVIDENCE_POLICY_BLOCKED`). Recreating visuals solely from secondary journalistic paraphrases risks compounding inaccuracies and copyright derivative-work claims. |
| **Tier 4**<br>*(Marketing / Bio / Self-Description)* | **ALLOWED WITH HIGH WARNING (Context Exhibit Only).**<br>**Tier 4** badge + `Public Domain` tag + high-visibility warning (*"Self-published claim — unverified"*). Requires paired Tier 1 reference. **Excluded from Official Briefing.** | **ALLOWED WITH HIGH WARNING (Context Exhibit Only).**<br>**Tier 4** badge + `Licensed` credit (`license_record_id` required) + high-visibility warning (*"Self-published claim — unverified"*). **Excluded from Official Briefing.** | **BLOCKED FROM PUBLICATION.**<br>API returns HTTP `422` (`EVIDENCE_POLICY_BLOCKED`). Recreating assets from unverified marketing/bio copy is prohibited. |

*Additional Hard Blocks (independent of matrix cell):*
- Any item with `source_type = 'third_party_exhibit'` and `license_status = 'PUBLIC_DOMAIN'` -> **BLOCKED AT API (`HTTP 422`) AND POSTGRESQL DB (`CHECK` constraint violation)**.
- Any item with `license_status = LICENSED` and empty `license_record_id` -> **BLOCKED (`HTTP 422`)**.
- Any item with `license_status = RECREATED` and empty `recreation_source_ids` -> **BLOCKED (`HTTP 422`)**.
- Any item lacking `source_citation` or `sha256_hash` -> **BLOCKED (`HTTP 422`)**.

---

## 4. "Genuine Gaps" Disclosure (Mandatory on Every Case)

Intellectual honesty and defamation defense require explicitly showing what is *unknown* or *unverified*. **No case may transition to `published` (`REQ-027`) without at least one Genuine Gaps disclosure entry.**

### Required Format
Displayed in a dedicated, uncollapsed `#genuine-gaps` section on every Case Dossier Overview (`REQ-002`) as a structured bulleted list. Every item MUST contain three fields:
1. **Claim / Question:** The specific factual question, allegation, or timeline gap relevant to the case.
2. **Why It Could Not Be Verified:** Concrete reason (e.g., *"Exhibit B remains sealed by federal court order"*, *"FOIA request #XXXX pending agency processing"*, *"1934 county clerk ledger destroyed in fire"*, *"No primary docket record found; only secondary press reports exist"*).
3. **Highest Available Evidence Tier:** Explicit label (`None`, `Tier 2`, `Tier 3`, or `Tier 4`) with optional citation link.

### Example Entry
- **Claim:** Exact date wire transfer #4 cleared the offshore correspondent bank.  
  **Why unverified:** Bank routing attachment (Exhibit 14-C) is sealed under protective order in *U.S. v. Example* (S.D.N.Y. Docket 21-cr-0000).  
  **Available evidence:** `Tier 3` (2022 Reuters trial summary cites "late March 2019"; not corroborated by unsealed Tier 1 filings).

---

## 5. Handling Conflicting Sources

When two or more credible sources disagree on a date, dollar amount, sequence of events, or attribution:

1. **Never average, interpolate, or silently pick a winner.** If Source A states `$12.4M` and Source B states `$13.1M`, we NEVER display `$12.75M` or silently drop Source B.
2. **Attribute every version explicitly:** Store each competing claim as a structured `conflicting_source` entry with `source_name`, `verification_tier`, `source_date`, `excerpt_or_summary`, and `evidence_id` / `citation_url`.
3. **Side-by-Side UI Presentation (`REQ-005`, `REQ-010`):**
   - On desktop/tablet (`>= 768px`): Render a two-column (or multi-column) comparison card titled **"Conflicting Sources — Attributed Accounts"** with each source's Tier badge, date, and verbatim/attributed figure side-by-side.
   - On mobile (`< 768px`, e.g. `390px`): Render stacked paired cards inside a unified bordered "Conflicting Sources" container with an explicit `Account A vs. Account B` header so neither source appears subordinate.
   - If a Tier 1 final judicial ruling supersedes an earlier allegation (e.g., original indictment vs. final sentencing order), display a neutral status callout: *"Controlling Legal Status: [Final Order, Date]"* while keeping the earlier filing visible for historical completeness.

---

## 6. Prohibited Content (Zero-Tolerance List)

Publishing any of the following is strictly prohibited across all cases, evidence items, timelines, and future community theories. Violations block publication (`REQ-028`) and trigger immediate emergency redaction (`REQ-031`).

| Prohibited Category | Rule | Specific Legal Risks Cited |
|---|---|---|
| **1. Raw Autopsy Images & Medical Examiner Photos** | Never ingest, store, display, or recreate graphic post-mortem photographs, autopsy diagrams showing body trauma, or forensic wound photos. | **Invasion of privacy / surviving-family rights** (e.g., *National Archives & Records Admin. v. Favish*, 541 U.S. 157 (2004) recognizing family privacy in death-scene photos), **intentional infliction of emotional distress (IIED)**, and platform/host Acceptable Use Policy termination. |
| **2. Living Victims' Full Names & Identifying PII Without Consent** | Never publish full names, home addresses, phone numbers, financial account numbers, or identifying photographs of living victims, minors, or uncharged third-party witnesses without documented explicit consent (redact PII even if unredacted in a raw filing). | **India DPDP Act 2023** (unlawful processing of personal data without consent/legitimate use, statutory penalties up to ₹250 crore), **US publication of private facts / victim-privacy statutes**, and **doxxing / harassment liability**. |
| **3. Naming Accused-Not-Convicted Persons as Guilty** | Never state or imply that any living individual who was investigated, indicted, acquitted, had charges dismissed, or settled *without admitting liability* (common in SEC civil settlements) is guilty of a crime. Always use precise legal attribution (*"alleged in DOJ indictment"*, *"charged with"*, *"settled without admitting or denying SEC allegations"*, *"acquitted on all counts"*). | **Defamation / Libel per se** (US state defamation law; loss of Fair Report Privilege if reporting is inaccurate or unfair), **India criminal/civil defamation** (BNS § 356 / tort law), and severe damages exposure exceeding the founder's `$2,000` budget. |
| **4. Crime-Scene Gore & Graphic Violence** | Never publish graphic crime-scene photographs, blood/bodily injury imagery, or gratuitous violent media—even if obtained via FOIA or public archives. | **Surviving-family privacy claims (*Favish*)**, **harmful-content intermediary liability**, **payment/app-store/hosting bans** (blocking future monetization and PWA/Tauri distribution), and ethical non-exploitation mandate. |
| **5. Unlicensed Third-Party Copyrighted Works & Paywalled Articles** | Never mirror full news articles, copyrighted agency photos, or private third-party exhibits attached to PACER dockets without a verified `LICENSED` record (`license_record_id`). | **US Copyright Act (17 U.S.C. § 504)** statutory damages (`$750–$30,000` per work; up to `$150,000` willful), **DMCA takedown** loss of safe harbor, and **Indian Copyright Act 1957** infringement liability. |
| **6. Unverified EU-Origin Personal Data (Launch Block)** | Do not publish EU-origin cases involving living EU data subjects at launch unless full GDPR lawful-basis verification (`gdpr_verified = true`) is documented by counsel. | **EU GDPR (Articles 6, 10, 17, 83)** — strict restrictions on processing criminal-offense data (Art. 10) and right-to-erasure enforcement. |
