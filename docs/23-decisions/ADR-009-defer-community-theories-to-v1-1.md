# ADR-009: Defer Community Theories to v1.1 (Schema-Ready, No Launch UI/Queue)

- **Status:** Accepted
- **Date:** 2026-11-27
- **Deciders:** Product Owner, Architect (MODE A)
- **Refs:** `docs/22-progress/REQUIREMENTS_STATUS.md` (Q4 — RESOLVED), `docs/22-progress/PO_SCOPING_DECISIONS.md` (#3, #4), `docs/01-product/PRODUCT_REQUIREMENTS.md` (`REQ-019`–`REQ-022`), `ADR-007`

## Context

Phase 03 requirements define **Community Theories** (`REQ-019` evidence-cited posting, `REQ-020` reporting + 1-report instant auto-quarantine for defamation/doxxing/living-person accusations, `REQ-021` `ADMIN` asynchronous moderation queue, and `REQ-022` rate limits + raw external URL blocking). However, PO Scoping Decision #3 specifies a solo founder with **zero continuous human moderation staff**, and `ADR-007` defers public `COMMUNITY_MEMBER` accounts to v1.1.

## Problem

Shipping user-generated content (UGC) in a true-crime application at launch introduces acute defamation, doxxing, and intermediary-liability risk (US CDA § 230 nuances, India IT Rules 2021) without dedicated moderation staffing or legal review of a moderation policy. Conversely, cutting Community Theories permanently from the architecture could force disruptive database migrations when the feature is added post-traction.

## Alternatives

1. **Rejected — Build and ship Community Theories in v1.0 with auto-quarantine:**
   - *Reasoning:* Conflicts with `ADR-007` (no reader accounts in v1.0), diverts solo-founder engineering time from shipping the 10 high-integrity launch cases, and lacks legal counsel review of the UGC moderation policy.
2. **Rejected — Cut Community Theories permanently from roadmap and schema design:**
   - *Reasoning:* Eliminates a valuable v1.1 engagement differentiator (structured, evidence-cited analytical discussions) and risks awkward retroactive schema changes later.
3. **Selected — Defer Community Theories (`REQ-019`–`REQ-022`) to v1.1, design the data model to support them cleanly now, and do NOT build the UI tab, public submission APIs, or moderation queue at launch:**
   - *Reasoning:* Zero launch legal/moderation exposure and zero wasted v1.0 UI/API build time, while keeping the PostgreSQL data model cleanly prepared for v1.1 once moderation-policy legal review is funded.

## Selected

1. **Community Theories (`REQ-019`–`REQ-022`) remain in the product roadmap targeting v1.1** (explicitly deferred, NOT cut).
2. **Schema-Ready Design (Phase 06):** Phase 06 (Architecture) MUST design the relational data model to support Community Theories cleanly using separate, normalized tables (e.g., `community_theories`, `theory_evidence_citations`, `theory_reports`, and `moderation_audit`) with explicit moderation status enums (`visible`, `quarantined`, `removed`) and mandatory `reason_code` fields—decoupled from core `cases` and `evidence_items` tables.
3. **Zero Launch Build Surface:** MVP and v1.0 MUST **NOT** build or expose:
   - The Community Theories tab or submission UI in `mobile-web`.
   - Public theory posting/reporting HTTP endpoints (`/api/v1/cases/:id/theories`, report endpoints).
   - The moderation queue UI or moderation endpoints in `admin-web`.
4. **Hard Prerequisite for v1.1:** Before Community Theories are built or enabled in v1.1:
   - Consumer accounts (`ADR-007`) must be live with DPDP review complete, **AND**
   - **Legal review of the UGC moderation policy** (defamation, living-person accusation rules, notice-and-takedown workflow, and intermediary safe-harbor terms) must be explicitly funded and signed off by counsel.

## Rationale

Designing clean, isolated tables on paper during Phase 06 costs near zero and guarantees foreign-key integrity with `cases`, `evidence_items`, and future `users`, while refusing to ship unreviewed UGC UI/endpoints protects the solo founder's `$2,000` budget and legal defensibility during the critical first 10 cases.

## Trade-offs

- v1.0 is a pure single-publisher investigative reading experience with no reader-to-reader interaction (beyond the public Correction & Takedown form `REQ-030`).
- Schema specs drafted in Phase 06 may require minor adjustments once v1.1 moderation legal counsel reviews specific notice-and-takedown metadata fields.

## Risks

- Unused database tables created prematurely in production could drift from evolving v1.1 legal requirements.
- **Mitigation:** Design the tables in the Phase 06 Data Model specification, and gate actual migration execution (or keep the v1.1 migration script cleanly separated from v1.0 core migrations) until v1.1 implementation begins.

## Consequences

- `mobile-web` v1.0 navigation excludes the Community Theories tab.
- `admin-web` v1.0 navigation excludes the Moderation Queue screen.
- Phase 06 Data Model documentation includes the full normalized schema design for `community_theories`, `theory_reports`, and `moderation_audit` clearly tagged `v1.1 — Do Not Expose at Launch`.
- `PERMISSION_MATRIX.md` post-launch actions (#3 `view community theories`, #8 `post theory`, #9 `report theory`, #10 `moderate theory`) remain dormant until v1.1.

## Reversal Strategy

To enable Community Theories in v1.1, complete the funded legal review of the moderation policy (and `ADR-007` DPDP counsel review), apply the v1.1 schema migration, implement `REQ-019`–`REQ-022` with full RBAC + auto-quarantine tests, and mark this ADR Completed. To permanently remove the feature instead, drop the v1.1 schema design appendix and mark this ADR Superseded.
