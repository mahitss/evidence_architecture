import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { VitePWA } from 'vite-plugin-pwa';

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['icon.svg', 'apple-touch-icon.png', 'offline.html'],
      manifest: {
        id: '/',
        name: 'Case Files',
        short_name: 'CaseFiles',
        description: 'Documented public-record case files. Every claim is sourced. Every source is tiered.',
        theme_color: '#8b6f47',
        background_color: '#0f1115',
        display: 'standalone',
        orientation: 'any',
        scope: '/',
        start_url: '/',
        icons: [
          {
            src: '/pwa-192x192.png',
            sizes: '192x192',
            type: 'image/png',
          },
          {
            src: '/pwa-512x512.png',
            sizes: '512x512',
            type: 'image/png',
          },
          {
            src: '/pwa-maskable-512x512.png',
            sizes: '512x512',
            type: 'image/png',
            purpose: 'maskable',
          },
          {
            src: '/icon.svg',
            sizes: 'any',
            type: 'image/svg+xml',
            purpose: 'any',
          },
        ],
      },
      workbox: {
        globPatterns: ['**/*.{js,css,html,ico,png,svg,woff2}'],
        // Keep the dynamic PDF.js worker chunk out of precache so initial SW install stays lightweight
        globIgnores: ['**/pdf.worker*.mjs', '**/pdf.worker*.js'],
        navigateFallback: '/index.html',
        navigateFallbackDenylist: [/^\/api\//],
        runtimeCaching: [
          {
            // App fonts / static assets — CacheFirst
            urlPattern: /\.(?:woff2?|ttf|otf|png|svg|ico)$/i,
            handler: 'CacheFirst',
            options: {
              cacheName: 'casefiles-static-assets',
              expiration: {
                maxEntries: 60,
                maxAgeSeconds: 60 * 60 * 24 * 30, // 30 days
              },
            },
          },
          {
            // Published cases & evidence metadata — StaleWhileRevalidate (1h TTL)
            urlPattern: ({ url, request }) =>
              request.method === 'GET' &&
              (/^\/api\/v1\/cases(?:\/.*)?$/.test(url.pathname) ||
                /^\/api\/v1\/evidence\/[^/]+$/.test(url.pathname)),
            handler: 'StaleWhileRevalidate',
            options: {
              cacheName: 'casefiles-api-records',
              expiration: {
                maxEntries: 120,
                maxAgeSeconds: 60 * 60, // 1 hour
              },
              cacheableResponse: {
                statuses: [200],
              },
            },
          },
          {
            // Full-text search — NetworkFirst with short fallback timeout
            urlPattern: ({ url, request }) =>
              request.method === 'GET' && /^\/api\/v1\/search$/.test(url.pathname),
            handler: 'NetworkFirst',
            options: {
              cacheName: 'casefiles-api-search',
              networkTimeoutSeconds: 4,
              expiration: {
                maxEntries: 40,
                maxAgeSeconds: 60 * 15, // 15 minutes
              },
              cacheableResponse: {
                statuses: [200],
              },
            },
          },
        ],
      },
      devOptions: {
        enabled: false,
      },
    }),
  ],
  build: {
    target: 'es2022',
    sourcemap: false,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('pdfjs-dist')) {
            return 'pdfjs';
          }
          if (id.includes('react-router') || id.includes('@tanstack/react-query')) {
            return 'vendor-app';
          }
          if (id.includes('node_modules/react/') || id.includes('node_modules/react-dom/')) {
            return 'vendor-react';
          }
          return undefined;
        },
      },
    },
  },
  server: {
    host: '127.0.0.1',
    port: 5173,
  },
  preview: {
    host: '127.0.0.1',
    port: 4173,
  },
});
