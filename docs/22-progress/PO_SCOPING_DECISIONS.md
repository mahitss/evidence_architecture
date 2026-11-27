# Product Owner Scoping Decisions (Phase 03 Input)

- **Captured Date:** 2026-11-26
- **Phase:** Phase 03 — Product Requirements (Prompt 6, Phase A)
- **Status:** Auditable baseline input for all Phase 03 requirements and subsequent architecture phases.
- **Note:** All six answers below are recorded verbatim from the Product Owner's response.

---

## Verbatim PO Responses

> **1. JURISDICTION:** India as the primary operating jurisdiction for user data (DPDP compliance). Primary content jurisdiction: United States — FOIA-eligible government works, DOJ/ED/SEC press releases, PACER court records. EU-origin cases not shown at launch unless GDPR compliance is explicitly verified.
>
> **2. MONETIZATION:** Free for MVP. Subscription ($4.99-$9.99/month) evaluated after 1,000 MAU and at least 3 case drops. Billing (Domain 21) is explicitly OUT of scope for MVP and v1.0.
>
> **3. TEAM:** Solo founder (Product Owner) plus one AI implementation agent. No human engineering team at launch. No 24/7 on-call, no continuous human moderation queue, no operational staff. Scope must reflect this.
>
> **4. BUDGET:** $2,000 ceiling for the first 6 months. Allocation: $1,000-1,500 for legal review of template disclaimers, correction process, and one case file (media/First Amendment lawyer in target market); $300-500 for infrastructure (VPS, GitHub, domain, object storage); $200 for PACER/FOIA costs; $0 for paid APIs (use free tiers: CourtListener, FBI Wanted, MuckRock). Legal counsel is REQUIRED before the first case publishes.
>
> **5. PLATFORM PRIORITY:** Mobile-first PWA as the only shipped client for MVP and v1.0 (React + Vite + Tailwind, installable, offline-capable). Native Tauri iOS/Android builds deferred until a real dev environment with mobile toolchains is available (see ADR-003). Web admin panel (Next.js) is separate for case authoring.
>
> **6. LAUNCH CASE CATEGORIES:** (b) Financial crimes / money laundering AND (c) Historical cases with fully public court records. Explicitly DEFER (a) cold-case DNA and (d) document-heavy to v1.1. First 10 cases: 6 financial crimes + 4 historical public-record cases.

---

## Surfaced Tensions & Audit Notes (Non-Modifying)

Per charter governance, the following cross-document tensions surfaced from these answers are logged for PO resolution in `docs/22-progress/REQUIREMENTS_STATUS.md` prior to Phase 06 (Architecture):

1. **Dual-framework frontend footprint vs. solo-founder / `$300–$500` infra budget:** Answer #5 names React + Vite + Tailwind for the Mobile PWA and Next.js for the separate Web Admin panel, while `admin-web/README.md` currently notes a pending decision regarding the existing Vite+React starter. Maintaining two separate frontend frameworks and a Node SSR server for a single-admin internal authoring tool increases build/hosting complexity.
2. **Solo-founder no-moderation constraint vs. Community Theories:** Answer #3 prohibits a continuous human moderation queue; therefore Community Theories (`REQ-019`–`REQ-022`) are scoped `Post-launch` with strict auto-quarantine guardrails rather than MVP/v1.0.
3. **Dual-jurisdiction legal budget (`$1,000–$1,500`) vs. India DPDP + US First Amendment scope:** Counsel budget likely covers one US media/First Amendment review (template disclaimers, correction flow, and 1 case file); separate India DPDP Act 2023 / IT Rules legal review is not explicitly funded in the `$1,000–$1,500` envelope.
4. **EU-origin case exclusion vs. EU user access under GDPR:** Blocking EU-origin *cases* does not automatically exempt the app from GDPR if EU-resident *users* register accounts on the open web PWA.
