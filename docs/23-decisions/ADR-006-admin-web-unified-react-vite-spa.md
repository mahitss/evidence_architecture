# ADR-006: Unify `admin-web` on React + Vite + Tailwind SPA (No Next.js)

- **Status:** Accepted
- **Date:** 2026-11-27
- **Deciders:** Product Owner, Architect (MODE A)
- **Refs:** `docs/22-progress/REQUIREMENTS_STATUS.md` (Q1 — RESOLVED), `docs/22-progress/PO_SCOPING_DECISIONS.md` (#3, #4, #5), `docs/01-product/PRODUCT_REQUIREMENTS.md` (`REQ-007`, `REQ-023`–`REQ-028`)

## Context

In Phase 03 scoping (`PO_SCOPING_DECISIONS.md` #5), the Product Owner initially envisioned two separate web clients:
- `mobile-web`: an installable, offline-capable mobile-first Progressive Web App (PWA) built with **React + Vite + Tailwind**.
- `admin-web`: a separate internal case-authoring panel using **Next.js**.

At the same time, the project operates under a strict solo-founder team model (1 founder + 1 AI implementation agent, zero ops staff) and a `$300–$500` six-month infrastructure ceiling covering VPS, domain, GitHub, and object storage.

## Problem

Maintaining Next.js solely for an internal, single-admin authoring dashboard introduces unnecessary architectural and operational overhead:
- Two distinct frontend frameworks, routers, build pipelines, and mental models for one solo founder.
- A persistent Node.js SSR runtime on the VPS solely to serve authenticated admin screens that require neither SEO nor server-side rendering.
- Duplicated TypeScript API client types, form validation, and Evidence Integrity badge/matrix rendering components between `mobile-web` and `admin-web`.

## Alternatives

1. **Rejected — Keep Next.js for `admin-web` and React + Vite + Tailwind for `mobile-web`:**
   - *Reasoning:* Adds SSR runtime memory/ops overhead, App Router vs. client SPA divergence, and extra dependency maintenance with zero user-facing benefit for an authenticated internal admin tool.
2. **Rejected — Merge `admin-web` directly into the `mobile-web` bundle:**
   - *Reasoning:* Bloats the public reader PWA bundle with admin-only authoring/upload code and increases the risk of accidental route or UI exposure. Separate deployable apps preserve clean network/origin isolation (e.g., separate subdomain or reverse-proxy access controls).
3. **Selected — Unify both `mobile-web` and `admin-web` as separate React + Vite + Tailwind SPAs:**
   - *Reasoning:* One framework, one build toolchain, zero Node SSR runtime in production (both compile to static assets served directly by the reverse proxy / Go backend), while preserving clean deployment isolation between the public reader PWA and the internal admin panel.

## Selected

1. **`admin-web` MUST be built as a React + Vite + Tailwind SPA**, matching the exact frontend stack of `mobile-web`.
2. **Next.js is explicitly NOT used** in MVP or v1.0.
3. `admin-web` remains a **separate application directory and build artifact** from `mobile-web`, communicating exclusively with the Go backend (`backend-go`) over authenticated `/api/v1/admin/*` endpoints.
4. Shared TypeScript contracts (API DTOs, Verification Tier / License Status display matrix logic) may be shared cleanly without framework lock-in.

## Rationale

An internal case-authoring panel used by 1–2 trusted roles (`ADMIN`, `RESEARCHER`) gains nothing from SSR, static site generation, or edge middleware. Unifying on React + Vite + Tailwind eliminates an entire class of operational complexity, keeps VPS memory footprint minimal within the `$300–$500` budget, and aligns directly with the solo-founder engineering model.

## Trade-offs

- Admin pages render client-side after loading the JS bundle (negligible impact for a single authenticated author on broadband/desktop).
- Any future need for server-rendered admin reports or BFF patterns must live in the Go backend (`backend-go`) rather than Next.js Server Actions / Route Handlers.

## Risks

- Client-side SPAs must never be trusted for access control; an attacker inspecting the static `admin-web` bundle can see admin route paths and UI logic.
- **Mitigation:** Mandatory server-side RBAC enforcement in `backend-go` (`REQ-016`, `PERMISSION_MATRIX.md`), strict CORS/cookie policies, and optional reverse-proxy IP allowlisting or separate admin subdomain.

## Consequences

- `PO_SCOPING_DECISIONS.md` Answer #5 is superseded specifically with respect to the Next.js admin framework choice.
- Phase 06 (Architecture) will specify static asset delivery for both `mobile-web` (PWA with service worker + manifest) and `admin-web` (authenticated authoring SPA) without provisioning a Node.js SSR server in production.
- `admin-web/README.md` pending stack question is closed in favor of React + Vite + Tailwind.

## Reversal Strategy

If a future team requires SSR or complex server-driven workflows for an external researcher portal, `admin-web` can be replaced independently with Next.js or another framework without touching `backend-go`, `processor-rust`, or `mobile-web` because all business logic and auth live behind `/api/v1/admin/*`.
