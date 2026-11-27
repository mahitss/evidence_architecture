# ADR-015: Anonymous-Only Reader Surface with Server-Side Geoblock Middleware

- **Status:** Accepted
- **Date:** 2026-11-27
- **Deciders:** Product Owner, Architect (MODE A)
- **Refs:** `ADR-007`, `ADR-008`, `docs/05-architecture/SYSTEM_CONTEXT.md`, `docs/05-architecture/LOW_LEVEL_DESIGN.md`, `docs/09-api/openapi.yaml`, `docs/10-security/SECURITY_ARCHITECTURE.md` (T1, T8)

## Context

Two foundational PO decisions constrain the public reader surface at launch:
1. **`ADR-007`:** MVP and v1.0 ship strictly as **anonymous reading only** (zero consumer accounts, login, personalization, or server-side bookmarks) to eliminate India DPDP consumer-account obligations until DPDP counsel is funded in v1.1.
2. **`ADR-008`:** EU/EEA visitors must be **geoblocked at the IP level** at launch (receiving a static *"Not available in your region"* page) until EU GDPR counsel is funded.

This ADR defines **where and how** the anonymous reader contract and EU/EEA geoblocking are enforced architecturally inside `backend-go` and the reverse proxy.

## Problem

Relying on client-side JavaScript checks to enforce geoblocking or mixing anonymous reader routes with authenticated admin routes in the same middleware chain risks trivial bypass, accidental draft data exposure, or accidental session-cookie issuance to anonymous visitors.

## Alternatives

1. **Rejected — Client-Side JavaScript Geoblocking in `mobile-web`:**
   - *Reasoning:* Trivially bypassed by curling `/api/v1/cases` directly or disabling JS, still serves API responses and case data to EU/EEA IPs, and requires calling a third-party client IP-lookup API (violating `$0` paid APIs and leaking visitor IPs to third parties).
2. **Rejected — Mandatory Login Wall / Waitlist Gate for All Readers:**
   - *Reasoning:* Violates `ADR-007` (forces consumer PII collection under India DPDP before DPDP counsel is funded) and destroys organic discovery of public-record dossiers.
3. **Rejected — Edge/DNS-Only Geoblocking Without Go Middleware Enforcement:**
   - *Reasoning:* Ties compliance enforcement exclusively to a specific CDN/DNS vendor (conflicting with no-CDN-at-MVP and deferred cloud provider selection in `ADR-013`) and cannot be deterministically verified in local sandbox or GitHub Actions integration tests.
4. **Selected — Dedicated Anonymous Public Router Protected by Server-Side Geoblock Middleware in `backend-go` (Backed by Reverse-Proxy Header Sanitization):**
   - *Reasoning:* Enforces geoblocking deterministically on both HTML/PWA delivery (reverse proxy) and every public API endpoint (`/api/v1/*` in Go middleware), never sets session cookies on public routes, works with `$0` cost via trusted proxy header or local GeoIP country DB, and is 100% unit- and integration-testable in CI.

## Selected

1. **Strict Route Separation:** `backend-go` mounts two distinct middleware chains:
   - **Public Reader Router (`/api/v1/cases`, `/api/v1/evidence`, `/api/v1/search`, `/api/v1/corrections`):** Applies `RequestID → TrustedIP → GeoblockEU → RateLimit → Handler`. Sets **zero** cookies, performs **zero** user authentication, and never queries `admin_users`.
   - **Internal Admin Router (`/api/v1/admin/*`):** Applies `RequestID → TrustedIP → AdminRateLimit → AdminJWTAuth (with Redis denylist) → Handler`. Exempt from reader EU geoblock so the solo founder is never locked out of emergency retractions (`REQ-031`) while traveling.
2. **Geoblock Middleware Behavior (`internal/middleware/geoblock.go`):**
   - Resolves country code from proxy-sanitized GeoIP header or local country MMDB (`GEOIP_DB_PATH`).
   - If ISO-3166-1 alpha-2 code is in the 30-country EU/EEA blocklist, aborts immediately with `HTTP 451 Unavailable For Legal Reasons` (`error.code = REGION_UNAVAILABLE` for API calls; static *"Not available in your region"* page at proxy for browser navigations).
   - **Fail-Closed Boot Check:** In `APP_ENV=staging|production`, if `GEOBLOCK_MODE` is misconfigured or `GEOIP_DB_PATH` cannot be loaded, `backend-go` **refuses to start**.
3. **Zero Tracking Cookies / Zero PII Logs:** Public reader responses set no cookies and access logs record only request path, status, latency, and `request_id` (no raw visitor IP persistence in `audit_log`).

## Rationale

Placing geoblocking directly in `backend-go` middleware (paired with reverse-proxy header stripping per `SECURITY_ARCHITECTURE.md` T8) makes territorial compliance portable across any VPS/cloud host, testable in automated Go CI tests with synthetic country fixtures, and completely decoupled from external SaaS vendors.

## Trade-offs

- Requires maintaining a free local IP-to-country database (e.g., DB-IP Lite or MaxMind GeoLite2) refreshed monthly via cron/systemd timer unless placed behind a trusted edge proxy providing a verified country header.
- Commercial VPN users exiting outside the EU/EEA can still bypass IP-level geoblocking (accepted known limitation per `ADR-008`).

## Risks

- **Header Spoofing (T8):** If `backend-go` were ever exposed directly to the internet without the reverse proxy stripping client `X-Forwarded-For` / country headers, callers could spoof a US IP.
- **Mitigation:** `backend-go` binds strictly to `127.0.0.1:8080` (private interface only) and trusts forwarded headers solely from configured `TRUSTED_PROXY_CIDRS` (`127.0.0.1/32`).

## Consequences

- `openapi.yaml` explicitly documents `security: []` and `451` responses on all 5 Reader endpoints.
- CI integration tests will assert: (a) EU country code (`DE`, `FR`, `IE`) → `HTTP 451`, (b) US/IN country code (`US`, `IN`) → `HTTP 200`, (c) zero `Set-Cookie` headers on all public reader responses.

## Reversal Strategy

When GDPR counsel (`ADR-008`) and India DPDP counsel (`ADR-007`) are funded in v1.1, disable the `GeoblockEU` middleware via configuration (`GEOBLOCK_MODE=off`) and mount the v1.1 authenticated reader middleware on user-specific routes (`/api/v1/me/*`, `/api/v1/cases/:id/theories`) while keeping public reading routes intact.
