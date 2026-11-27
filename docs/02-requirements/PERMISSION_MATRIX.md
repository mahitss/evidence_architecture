# Role & Permission Matrix (RBAC Specification)

- **Document Path:** `docs/02-requirements/PERMISSION_MATRIX.md`
- **Phase:** Phase 03 — Requirements (Charter PART 05)
- **Governs:** `REQ-014`, `REQ-015`, `REQ-016`, `REQ-017`, `REQ-018`, `REQ-019`, `REQ-020`, `REQ-021`, `REQ-023`, `REQ-025`, `REQ-026`, `REQ-027`, `REQ-030`, `REQ-031`

---

## 1. Role Definitions

| Role | Description | Launch Availability |
|---|---|---|
| **`ADMIN`** | Solo founder / system owner. Full administrative control over cases, evidence deletion (subject to audit & published-case locks), legal-gate publication, emergency kill-switch, and moderation. | MVP |
| **`RESEARCHER`** | Trusted case author/researcher invited by `ADMIN` (future contractor or founder's non-privileged authoring role). Can create/edit draft cases, ingest evidence, build timelines, and submit drafts to `in_legal_review`, but **cannot** publish live, delete evidence, or moderate users. | MVP |
| **`COMMUNITY_MEMBER`** | Authenticated reader who has accepted India DPDP Act 2023 consent (`REQ-015`). Can read published content, export/delete their own personal data, and (Post-launch) post/report evidence-cited community theories. | Launch (v1.0) |
| **`ANONYMOUS`** | Unauthenticated public visitor. Can browse published cases, view timelines and evidence, search published content, and submit public correction/takedown/DPDP grievance requests without creating an account. | MVP |

---

## 2. Permission Matrix (4 Roles × 13 Actions)

**Legend:**
- **`ALLOWED`** — Action is permitted directly upon passing standard input validation and resource scope checks.
- **`DENIED`** — Action is strictly prohibited. Server MUST return HTTP `401 Unauthorized` (for `ANONYMOUS`) or HTTP `403 Forbidden` (for authenticated roles) with zero state mutation.
- **`ALLOWED_WITH_APPROVAL`** — Action is permitted **only** when mandatory server-verified approval/precondition gates are satisfied (e.g., legal counsel sign-off ID, published-case unpublish precondition, or lockout safeguard). Missing approval returns HTTP `422 Unprocessable Entity` or HTTP `409 Conflict`.

| # | Action | `ADMIN` | `RESEARCHER` | `COMMUNITY_MEMBER` | `ANONYMOUS` | Governing Reqs & Scope Notes |
|---:|---|---|---|---|---|---|
| 1 | **view case** | `ALLOWED` *(all statuses: draft, review, approved, published, unpublished)* | `ALLOWED` *(published + assigned/draft cases)* | `ALLOWED` *(published cases only)* | `ALLOWED` *(published cases only)* | `REQ-001`, `REQ-002`, `REQ-014`, `REQ-023` — Non-published cases return `404` on public endpoints. |
| 2 | **view evidence** | `ALLOWED` *(all statuses including draft/redacted)* | `ALLOWED` *(published + draft case evidence)* | `ALLOWED` *(published, non-redacted evidence only)* | `ALLOWED` *(published, non-redacted evidence only)* | `REQ-003`, `REQ-004`, `REQ-014`, `REQ-031` — Redacted items return tombstone; `BLOCKED` matrix items never served publicly. |
| 3 | **view community theories** *(Post-launch)* | `ALLOWED` *(all statuses including quarantined/removed)* | `ALLOWED` *(visible theories only)* | `ALLOWED` *(visible theories only)* | `ALLOWED` *(visible theories only)* | `REQ-019`, `REQ-020`, `REQ-021` — Quarantined/removed theories hidden from all non-`ADMIN` roles. |
| 4 | **create case** | `ALLOWED` | `ALLOWED` *(created in `draft` status only)* | `DENIED` | `DENIED` | `REQ-023` — All new cases initialize in `draft` status isolated from public routes. |
| 5 | **edit case** | `ALLOWED` *(draft/review/approved; published edits require re-publish snapshot)* | `ALLOWED` *(draft cases only; locked once `in_legal_review` or `published`)* | `DENIED` | `DENIED` | `REQ-023`, `REQ-025`, `REQ-029` — Prevents silent unreviewed edits to live cases. |
| 6 | **publish case** | `ALLOWED_WITH_APPROVAL` *(requires valid `legal_signoff_id`, `>=1` Genuine Gap, `0` blocked items, prohibited-content & EU/GDPR gates)* | `DENIED` *(may only submit `draft` -> `in_legal_review`)* | `DENIED` | `DENIED` | `REQ-027`, `REQ-028`, `REQ-029` — Hard legal counsel sign-off gate enforced server-side. |
| 7 | **delete evidence** | `ALLOWED_WITH_APPROVAL` *(draft cases only with mandatory reason + audit log; published evidence must be unpublished/redacted via `REQ-031` first)* | `DENIED` | `DENIED` | `DENIED` | `REQ-026`, `REQ-031` — Preserves legal chain-of-custody; hard-deleting published evidence returns HTTP `409`. |
| 8 | **post theory** *(Post-launch)* | `ALLOWED` | `ALLOWED` | `ALLOWED` *(requires `>=1` published `evidence_id`, `policy_ack=true`, rate limit `<=3/24h`)* | `DENIED` | `REQ-019`, `REQ-022` — Guarded post-launch feature; external raw URLs rejected (`422`). |
| 9 | **report theory** *(Post-launch)* | `ALLOWED` | `ALLOWED` | `ALLOWED` | `DENIED` | `REQ-020` — `>=1` defamation/doxxing/living-person report triggers immediate auto-quarantine. |
| 10 | **moderate theory** *(Post-launch)* | `ALLOWED` *(approve, remove, suspend_user with mandatory reason + audit log)* | `DENIED` | `DENIED` | `DENIED` | `REQ-021` — Solely `ADMIN` to guarantee consistent legal risk adjudication. |
| 11 | **submit correction request** | `ALLOWED` | `ALLOWED` | `ALLOWED` | `ALLOWED` | `REQ-030` — Open to all roles (including `ANONYMOUS`) for factual corrections, defamation/copyright takedowns, and DPDP grievances; IP rate-limited (`<=5/hr`). |
| 12 | **export data** | `ALLOWED` *(own account data only)* | `ALLOWED` *(own account data only)* | `ALLOWED` *(own account data only)* | `DENIED` | `REQ-017` — DPDP Act 2023 self-service JSON export; user identity derived strictly from authenticated session. |
| 13 | **delete account** | `ALLOWED_WITH_APPROVAL` *(own account only; blocked with HTTP `409` if last remaining `ADMIN` to prevent lockout)* | `ALLOWED` *(own account only; password re-auth required)* | `ALLOWED` *(own account only; password re-auth required)* | `DENIED` | `REQ-018` — DPDP Act 2023 erasure + consent withdrawal; immediately revokes sessions and retains anonymized audit hashes only. |

---

## 3. Mandatory Server-Side Enforcement Policy (`REQ-016`)

Client-side hiding of buttons, routes, or navigation tabs in the PWA (`mobile-web`) or Admin Panel (`admin-web`) is strictly a UX convenience and **is NEVER treated as an authorization control**. All permission checks MUST be enforced server-side in the Go API (`backend-go`):

1. **Deny-by-Default Route Middleware:**
   - Every non-public endpoint under `/api/v1/admin/*`, `/api/v1/me/*`, and authenticated write routes MUST pass through session-validation middleware followed by explicit role-allowlist middleware (`RequireRole(roles...)`).
   - Missing or invalid session cookies MUST return HTTP `401 Unauthorized` before handler execution.
   - Valid sessions lacking the required role MUST return HTTP `403 Forbidden` before any database read/write on protected resources.

2. **Resource-Level & State-Machine Authorization (Preventing IDOR & Workflow Bypass):**
   - Self-service endpoints (`GET /api/v1/me/export`, `DELETE /api/v1/me`) MUST resolve the target `user_id` exclusively from the verified server-side session token—never from client-supplied query params or JSON body fields.
   - Public read endpoints (`GET /api/v1/cases/:slug`, `/evidence/:id`, `/search`) MUST unconditionally filter `WHERE status = 'published'` and `is_redacted = false` inside SQL queries so draft or unpublished records can never leak even if a client guesses a UUID or slug.
   - `ALLOWED_WITH_APPROVAL` transitions (`publish case`, `delete evidence`, `delete account` for `ADMIN`) MUST execute inside a PostgreSQL transaction validating every precondition (`legal_signoff_id`, Genuine Gaps count `>= 1`, `eu_origin`/`gdpr_verified`, `prohibited_content_cleared`, and non-published lock) atomically before committing.

3. **Immutable Audit Logging on Privileged Actions:**
   - Every `ADMIN` or `RESEARCHER` mutation affecting publication status, evidence deletion, emergency unpublish/redaction (`REQ-031`), or theory moderation (`REQ-021`) MUST write an append-only row to `audit_logs` within the same database transaction (`actor_id`, `role`, `action`, `target_id`, `reason_code`, `utc_timestamp`).

4. **Mandatory Negative RBAC Verification (CI Gate):**
   - Per `REQ-016`, the Go integration test suite MUST include an automated matrix test covering **every `DENIED` cell** in Section 2, asserting exact HTTP `401` (`ANONYMOUS`) or `403` (authenticated unauthorized roles) and verifying zero database state changes.
