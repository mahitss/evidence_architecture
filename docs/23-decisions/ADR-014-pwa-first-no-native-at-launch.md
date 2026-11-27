# ADR-014: Ship Mobile-First PWA Only at Launch (No Native iOS/Android Builds)

- **Status:** Accepted
- **Date:** 2026-11-27
- **Deciders:** Product Owner, Architect (MODE A)
- **Refs:** `docs/22-progress/PO_SCOPING_DECISIONS.md` (#3, #4, #5), `ADR-003`, `ADR-006`, `ADR-008`, `docs/01-product/PRODUCT_REQUIREMENTS.md` (`REQ-006`, `REQ-007`)

## Context

The product targets mobile-first true-crime readers who inspect primary legal documents on phones and want offline case reading during commutes or travel. Earlier planning considered native mobile wrappers (Tauri iOS/Android), while `ADR-003` confirmed that the current sandbox lacks mobile SDKs/macOS Xcode toolchains and `ADR-008` requires rapid legal/territorial control (EU/EEA geoblocking) and instant kill-switch retraction (`REQ-031`).

## Problem

Shipping native iOS and Android binaries at launch introduces Apple Developer (`$99/yr`) and Google Play fees, opaque App Store true-crime content-moderation rejections, slow store review queues (blocking emergency legal updates), macOS build hardware requirements, and dual-platform maintenance for a solo founder.

## Alternatives

1. **Rejected — Tauri Mobile (iOS + Android) at MVP / v1.0:**
   - *Reasoning:* Blocked in the current gVisor sandbox (`ADR-003` — no Android SDK/NDK, no Xcode/macOS), slows shipping the 10 launch cases, and risks Apple App Store rejection for sensitive criminal-case subject matter.
2. **Rejected — React Native / Flutter Native Mobile App:**
   - *Reasoning:* Introduces a second frontend stack diverging from `admin-web` (`ADR-006`), adds App Store review lag that conflicts with fast legal/correction responsiveness, and doubles QA overhead for a solo founder.
3. **Selected — Installable, Offline-Capable Mobile-First PWA (`mobile-web`, React + Vite + Tailwind) as Sole Shipped Client for MVP & v1.0:**
   - *Reasoning:* Shares the exact React + Vite + Tailwind stack with `admin-web` (`ADR-006`), installs cleanly to iOS/Android home screens via Web App Manifest + Service Worker (`REQ-007`), supports offline case packs via Cache Storage / IndexedDB (`REQ-006`), enforces instant server-side EU/EEA geoblocking (`ADR-008`) and kill-switch retractions (`REQ-031`) without app-store gatekeeping, and costs `$0`.

## Selected

1. `mobile-web` (React + Vite + Tailwind PWA) is the **only shipped reader client** for MVP and v1.0.
2. Full mobile ergonomics (`360px`–`1440px` responsive layout, `0px` horizontal overflow at `390×844`, Web App Manifest, registered Service Worker, offline case-pack caching up to 50 MB/case with versioned cache invalidation for retractions) are mandatory launch criteria (`REQ-006`, `REQ-007`).
3. Reinforces `ADR-008`: Because all readers load the web origin directly, edge/middleware IP geoblocking applies immediately without relying on app-store regional storefront workarounds.
4. Native mobile packaging (Tauri Mobile or Capacitor) is explicitly deferred until post-v1.0 product-market fit (`>= 1,000` MAU) and availability of native build hardware/CI runners.

## Rationale

A well-engineered PWA delivers 95% of native reading utility (home-screen icon, standalone viewport, offline document packs) at 10% of the solo-founder engineering and legal-operations cost, while preserving instant server-controlled legal takedowns (`REQ-031`).

## Trade-offs

- No Apple App Store / Google Play organic store search discovery at launch.
- iOS Safari imposes stricter background Service Worker and Cache Storage eviction limits than native apps (mitigated by keeping per-case offline packs `<= 50 MB` and surfacing clear "Saved / Re-download" status in the PWA UI).
- No native push notifications on older iOS versions without home-screen installation (not needed at MVP since there are no user accounts or breaking news alerts).

## Risks

- Cached offline case packs on a reader's phone could hold a document after an emergency retraction (`REQ-031`).
- **Mitigation:** On every online visit, the PWA Service Worker checks a lightweight `/api/v1/cases` version/retraction manifest and immediately purges any locally cached case pack whose status is no longer `PUBLISHED`.

## Consequences

- `mobile-web` and `admin-web` share identical React + Vite + Tailwind tooling (`ADR-006`).
- Zero mobile SDKs or Apple/Google store accounts required for MVP or v1.0.

## Reversal Strategy

Because `mobile-web` is a clean React + Vite SPA talking strictly to `/api/v1/*` JSON endpoints, wrapping it later with Tauri Mobile or Capacitor in a macOS/Android CI environment requires zero backend changes.
