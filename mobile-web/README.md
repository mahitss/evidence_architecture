# Case Files — Mobile-Web PWA Reader (`mobile-web/`)

Anonymous, offline-capable Progressive Web App (PWA) for reading published public-record case dossiers and inspecting primary-source exhibits.

- **Stack:** React 19, Vite 6, TypeScript (strict, zero `any`), Tailwind CSS v4, React Router v6, TanStack Query v5, Zod, `pdfjs-dist` (lazy-loaded), `vite-plugin-pwa`, `uuid`.
- **Architecture:** `ADR-006` (React + Vite + Tailwind SPA), `ADR-007` (anonymous reader — no auth, no accounts, no tracking), `ADR-008` (HTTP 451 EU/EEA/UK geoblock), `ADR-014` (PWA-first mobile-web).
- **UI Primitives:** Hand-rolled WCAG AA accessible primitives in `src/components/ui/` (no external UI component libraries).

## Quick Start

```bash
cd mobile-web
cp .env.example .env.local
npm install
npm run dev
```

The dev server starts on `http://127.0.0.1:5173` and connects to the Go Reader API at `VITE_API_URL` (default `http://127.0.0.1:8080`).

## Environment Variables

See `.env.example`:

| Variable | Required | Default | Description |
|---|---|---|---|
| `VITE_API_URL` | No | `http://127.0.0.1:8080` | Base URL for `backend-go` Reader API (no trailing slash) |

## Commands

```bash
npm run dev        # Start local Vite dev server (127.0.0.1:5173)
npm run typecheck  # Strict TypeScript check (tsc --noEmit)
npm run test       # Run Vitest unit, MSW route, and axe-core accessibility tests
npm run build      # Typecheck + production bundle + PWA Service Worker (dist/)
npm run preview    # Preview production build locally (127.0.0.1:4173)
```

## Routes

- `/` — Published case directory with search bar, category filters, and cursor pagination.
- `/case/:slug` — Case dossier with Genuine Gaps callout, evidence list (tier & license badges), and chronological timeline.
- `/evidence/:id` — Exhibit viewer preceded by the **mandatory non-collapsible Source & Licensing Panel**, `RECREATED` notice when applicable, PDF/image/download viewer, and SHA-256 integrity checksum.
- `/search` — Debounced full-text search with safe `<mark>` highlight rendering and category filtering.
- `/correction` — Anonymous factual correction / takedown form with `Idempotency-Key`, visually hidden bot honeypot field, and 429 retry countdown.
- `/geoblocked` — Static HTTP 451 availability notice (no API requests, no bypass).
- `/about` — Archive methodology, legal disclaimer (`docs/19-compliance/COMPLIANCE.md`), and correction link.
- `*` — Archival 404 page.

## PWA & Offline Behavior

- **Manifest:** `Case Files` (`CaseFiles`), `display: standalone`, theme `#8b6f47`, background `#0f1115`, SVG + PNG (192, 512, maskable 512, Apple 180) icons.
- **Caching (Workbox):**
  - App shell & static assets: `CacheFirst` (precached excluding `pdf.worker.min.mjs`).
  - `GET /api/v1/cases*` and `/api/v1/evidence/*`: `StaleWhileRevalidate` (1-hour TTL).
  - `GET /api/v1/search`: `NetworkFirst` (4-second timeout).
  - Offline fallback document: `public/offline.html`.
- **Install Prompt:** Deferred until a reader has viewed at least 3 distinct cases (`localStorage`), with native Chromium prompt or iOS Safari instructions modal.
