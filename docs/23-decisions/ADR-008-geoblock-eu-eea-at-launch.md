# ADR-008: Geoblock EU/EEA Traffic on Reader Surface at Launch (Known Limitation)

- **Status:** Accepted
- **Date:** 2026-11-27
- **Deciders:** Product Owner, Architect (MODE A)
- **Refs:** `docs/22-progress/REQUIREMENTS_STATUS.md` (Q3 — RESOLVED), `docs/22-progress/PO_SCOPING_DECISIONS.md` (#1, #4), `docs/01-product/PRODUCT_REQUIREMENTS.md` (`REQ-028`), `docs/02-requirements/EVIDENCE_INTEGRITY_SPEC.md` (§6)

## Context

PO Scoping Decision #1 established that EU-origin cases will not be published at launch unless GDPR compliance is explicitly verified (`REQ-028`). However, under EU GDPR Article 3 (territorial scope), serving web content, IP-logged HTTP requests, or future user-facing features to **EU/EEA-resident readers** can trigger GDPR applicability—particularly for a true-crime site dealing with criminal-offense records (GDPR Art. 10) and right-to-erasure requests (GDPR Art. 17). The `$1,000–$1,500` launch legal budget does not fund EU GDPR counsel.

## Problem

Blocking only EU-origin *cases* leaves an open compliance gap if EU/EEA residents access the open web PWA (`mobile-web`) or submit correction forms containing personal data, exposing a solo founder to unvetted GDPR compliance and EU Representative (Art. 27) obligations before GDPR counsel is funded.

## Alternatives

1. **Rejected — Leave the reader PWA globally open to EU/EEA traffic and rely solely on anonymous browsing:**
   - *Reasoning:* IP addresses, correction-form submissions (`REQ-030`), and criminal-record content served to EU data subjects still carry non-trivial GDPR territorial and Art. 10/17 ambiguity without EU counsel review.
2. **Rejected — Fund EU GDPR counsel prior to v1.0 launch:**
   - *Reasoning:* Exceeds the hard `$2,000` six-month ceiling (`$1,000–$1,500` legal allocation).
3. **Selected — Enforce IP-level geoblocking for EU/EEA countries on the reader surface at launch, returning a clear "Not available in your region" page, documented as a known limitation until GDPR counsel is funded:**
   - *Reasoning:* Demonstrates clear territorial non-targeting of the EU/EEA market at zero additional API cost (via edge country header or self-hosted GeoIP database) while keeping launch scope cleanly bounded to US/India/non-EEA markets.

## Selected

1. **IP-level geoblocking of all 30 EU/EEA member states** (27 EU member states + Iceland, Liechtenstein, Norway; UK evaluated separately with counsel or included conservatively if advised) MUST be enforced at the edge / reverse-proxy / Go middleware layer on the public reader surface (`mobile-web` and public `/api/v1/*` endpoints) at MVP and v1.0 launch.
2. **Blocked visitors receive a clear, neutral static page** ("This service is not currently available in your region") with HTTP `451 Unavailable For Legal Reasons` (or `403 Forbidden` where edge tooling dictates), without loading case content, evidence assets, or tracking scripts.
3. **No EU-origin cases** are published at launch (continuing enforcement of `REQ-028`: `eu_origin = true` blocked unless `gdpr_verified = true`).
4. **Known Limitation & Lift Condition:** This geoblock is explicitly recorded as a temporary launch limitation. It may be lifted **only after dedicated EU GDPR legal counsel is funded** and signs off on GDPR compliance (including Art. 6, Art. 10 criminal-offense data, Art. 17 erasure, and Art. 27 representation).

## Rationale

Combining (a) zero EU-origin cases (`REQ-028`), (b) zero consumer user accounts at launch (`ADR-007`), and (c) IP-level EU/EEA geoblocking provides a clear, demonstrable good-faith posture that the platform is not targeting EU/EEA residents during its solo-founder MVP/v1.0 stage, while preserving `$0` paid-API budget compliance (using Cloudflare free-tier `CF-IPCountry` header or a local MaxMind GeoLite2 / DB-IP country database).

## Trade-offs

- Legitimate readers located in the EU/EEA (or traveling there) cannot read cases at launch.
- Adds a small edge/middleware configuration and GeoIP lookup responsibility to the Phase 06 architecture.

## Risks

- **VPN / Proxy Bypass:** IP-level geoblocking can be bypassed by users running VPNs; this is an industry-standard limitation and accepted as reasonable territorial gating when paired with non-EU targeting and zero EU-origin cases.
- **Stale GeoIP Data:** Self-hosted IP-to-country tables drift over time if not periodically refreshed.
- **Mitigation:** Prefer edge-provided country headers (e.g., Cloudflare free tier) with a scheduled monthly GeoIP database update fallback if self-hosted.
- **Admin Access:** Founder (`ADMIN`) operates from India; ensure geoblock applies strictly to the public reader surface (or allows explicit `ADMIN` access if ever traveling) so the founder is never locked out of `admin-web` or the emergency kill-switch (`REQ-031`).

## Consequences

- Phase 06 (Architecture) MUST specify the exact geoblocking layer (edge/reverse-proxy + Go API enforcement), the EU/EEA ISO-3166-1 alpha-2 country-code allow/deny list, the static "Not available in your region" response, and automated tests verifying blocked vs. permitted country headers.
- `mobile-web` PWA service worker must never pre-cache or bypass the geoblock response for EU/EEA IPs.
- Public Terms / Legal Notice must clearly state territorial availability (not offered in the EU/EEA at launch).

## Reversal Strategy

Once GDPR legal counsel is funded and verifies compliance (and any required privacy/erasure/Art. 10 controls are implemented), remove the EU/EEA country codes from the edge/middleware geoblock configuration, update the Terms of Service, and mark this ADR Superseded.
