# ADR-007: Defer Reader User Accounts to v1.1 (Anonymous-Only Launch; Internal Admin Auth Retained)

- **Status:** Accepted
- **Date:** 2026-11-27
- **Deciders:** Product Owner, Architect (MODE A)
- **Refs:** `docs/22-progress/REQUIREMENTS_STATUS.md` (Q2 — RESOLVED), `docs/22-progress/PO_SCOPING_DECISIONS.md` (#1, #4), `docs/01-product/PRODUCT_REQUIREMENTS.md` (`REQ-014`, `REQ-015`, `REQ-016`, `REQ-017`, `REQ-018`, `REQ-030`)

## Context

The product operates with India as its primary operating jurisdiction (triggering India DPDP Act 2023 obligations for consumer personal data) and the United States as its primary content jurisdiction (triggering US First Amendment, defamation, Fair Report Privilege, and copyright considerations). The six-month legal budget is capped at `$1,000–$1,500`, earmarked for US media/First Amendment counsel to review template disclaimers, the correction workflow, and Case 1 prior to publication.

In Phase 03 (`PRODUCT_REQUIREMENTS.md`), reader accounts and DPDP self-service workflows (`REQ-015` registration/login/DPDP consent, `REQ-017` personal data export, `REQ-018` account erasure) were initially scheduled for **Launch (v1.0)**.

## Problem

A `$1,000–$1,500` legal review budget cannot realistically fund competent legal counsel in **two** separate legal regimes (US First Amendment/media law AND India DPDP Act 2023 consumer data-fiduciary compliance) prior to Case 1 publication. Launching consumer user accounts without dedicated India DPDP legal review exposes the solo founder to statutory penalties under the DPDP Act 2023 while providing minimal core reader value for an anonymous-readable free MVP/v1.0.

## Alternatives

1. **Rejected — Ship consumer user accounts in v1.0 without dedicated India DPDP legal review:**
   - *Reasoning:* Unacceptable statutory compliance risk; violates the product's core principle of legal defensibility.
2. **Rejected — Split the `$1,000–$1,500` legal budget across both US media counsel and Indian DPDP counsel:**
   - *Reasoning:* Underfunds the critical US First Amendment/defamation review of Case 1 and disclaimers, risking an incomplete review in both jurisdictions.
3. **Selected — Defer consumer user accounts (`REQ-015`, `REQ-017`, `REQ-018`) to v1.1 and ship MVP + v1.0 as anonymous reading only, retaining internal `ADMIN`/`RESEARCHER` auth:**
   - *Reasoning:* Eliminates consumer user-account PII collection at launch, removes India DPDP consumer data-fiduciary obligations for reader accounts during v1.0, focuses 100% of the `$1,000–$1,500` budget on US media/First Amendment counsel, and accelerates v1.0 delivery.

## Selected

1. **MVP and v1.0 launch strictly as anonymous-reading-only** on the public reader PWA (`mobile-web`). No public registration, consumer login, or reader profile storage ships in MVP or v1.0.
2. **Internal authentication stays in MVP/v1.0** strictly for operational authoring roles (`ADMIN` and invited `RESEARCHER` accounts in `admin-web`), which are internal operational credentials rather than public consumer user data.
3. **`REQ-015` (Account Registration/Login/DPDP Consent), `REQ-017` (DPDP Personal Data Export), and `REQ-018` (DPDP Account Erasure) are reclassified from `Launch (v1.0)` to `Post-launch (v1.1)`.**
4. **Hard Prerequisite for v1.1:** Consumer user accounts MUST NOT be enabled in production until dedicated **India DPDP Act 2023 legal counsel** is funded and completes review of the consent notice, data-export payload, erasure pipeline, and grievance redressal process.
5. **Offline Case Packs (`REQ-006`)** remain in v1.0 using client-side browser storage (Cache Storage / IndexedDB) with zero server-side user PII.

## Rationale

Readers do not need accounts to read primary legal filings, filter timelines, search extracted court PDFs, cache case packs offline, or submit correction notices. Deferring consumer accounts removes an entire compliance and security surface (password resets, consumer breach notification, DPDP consent ledger operations) until the product proves reader traction and can fund India DPDP counsel.

## Trade-offs

- Cross-device synced bookmarks or reading progress are unavailable in v1.0 (local browser storage on-device only).
- Community Theories (`REQ-019`–`REQ-022`), which require authenticated `COMMUNITY_MEMBER` accounts, naturally align with the v1.1 timeline (see `ADR-009`).
- Public correction/takedown submissions (`REQ-030`) still accept a requester contact email so the founder can respond to legal/factual notices; this minimal operational contact data must be purpose-limited and covered in the privacy/correction notice reviewed before launch.

## Risks

- If `mobile-web` accidentally integrates third-party analytics, ad pixels, or tracking cookies, it could inadvertently collect personal data despite having no user accounts.
- **Mitigation:** Zero third-party tracking cookies, zero ad SDKs, and privacy-preserving/no-PII server logging in MVP and v1.0.

## Consequences

- Effective v1.0 scope shifts from `17 MVP + 10 Launch` to **`17 MVP + 7 Launch`** (with **`8 Post-launch (v1.1+)`** features: `REQ-013`, `REQ-015`, `REQ-017`, `REQ-018`, `REQ-019`, `REQ-020`, `REQ-021`, `REQ-022`).
- Phase 06 (Architecture) will design internal `ADMIN`/`RESEARCHER` session auth for MVP/v1.0 and reserve schema extensibility for `COMMUNITY_MEMBER` accounts and `consent_records` in v1.1.
- `REQUIREMENTS_STATUS.md` records this reclassification immediately; `PRODUCT_REQUIREMENTS.md` and `FEATURE_CATALOG.md` remain auditable and will reflect the v1.1 target in Phase 06/implementation planning.

## Reversal Strategy

Once India DPDP legal counsel is funded and signs off on consumer data handling, enable the `COMMUNITY_MEMBER` registration, consent, export, and deletion endpoints (`REQ-015`, `REQ-017`, `REQ-018`) in v1.1 and mark this ADR Superseded or Completed.
