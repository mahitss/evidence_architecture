# True Crime Investigation App *(Working Title)*

## Vision

The True Crime Investigation App is an evidence-first investigative workspace designed to ingest, structure, cross-reference, and analyze complex case files—connecting timelines, entities, primary-source documents, and spatial leads through a Go API backend, a Rust document-processing pipeline, and modern web/mobile interfaces.

## How to Start

1. Verify your local toolchain and `PATH` precedence:
   ```bash
   make env-check
   ```
2. Start local PostgreSQL and Redis backing services (direct processes, no Docker required):
   ```bash
   make services-up
   make services-status
   ```
3. Follow the complete setup and troubleshooting guide in [`docs/24-setup/LOCAL_DEV.md`](docs/24-setup/LOCAL_DEV.md).

## Project Reports & Environment Documentation

- [Phase 01 — Reconnaissance Report (`docs/22-progress/RECON_REPORT.md`)](docs/22-progress/RECON_REPORT.md)
- [Prompt 2 — Environment Capability Probe (`docs/22-progress/ENV_PROBE_REPORT.md`)](docs/22-progress/ENV_PROBE_REPORT.md)
- [Known Environment Limitations (`docs/24-setup/KNOWN_ENV_LIMITATIONS.md`)](docs/24-setup/KNOWN_ENV_LIMITATIONS.md)
- [Local Development Setup Guide (`docs/24-setup/LOCAL_DEV.md`)](docs/24-setup/LOCAL_DEV.md)
- [Pinned Toolchain Versions (`.tool-versions`)](.tool-versions)
