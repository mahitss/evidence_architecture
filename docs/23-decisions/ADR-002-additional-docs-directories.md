# ADR-002: Retain Additional Documentation Directories (`24-setup`, `25-toolchain`)

- **Status:** Accepted
- **Date:** 2026-11-26
- **Deciders:** Product Owner, Architect (MODE A)
- **Refs:** Charter PART 06, ADR-001

## Context

Charter PART 06 defines 24 documentation directories (`00-overview` through `23-decisions`) covering product, architecture, engineering domains, operations, governance, progress, and ADRs. However, our sandboxed gVisor host has material local-development constraints (`KNOWN_ENV_LIMITATIONS.md`, `LOCAL_DEV.md`) and pinned multi-language toolchains (`.tool-versions` covering Go `1.27.4`, Rust `1.100.0`, Node `22.23.5`, PostgreSQL `15.20`, Redis `7.0.15`) that require dedicated, discoverable documentation homes.

## Problem

Charter PART 06 does not explicitly allocate a directory for developer workstation/sandbox onboarding guides or toolchain pinning policies. Burying local setup instructions inside `13-infrastructure` or `20-operations` obscures them from first-time contributors and blurs production operations with local sandbox mechanics.

## Alternatives

1. **Rejected — Fold setup and toolchain docs into `docs/14-devops/` and `docs/20-operations/`:**
   - *Reasoning:* `14-devops` governs CI/CD pipelines and release automation, while `20-operations` governs production runbooks and incident response. Mixing local gVisor sandbox quirks (`--auth=trust`, `/var/log` overlay workarounds) with production operational docs creates safety risk and reduces onboarding discoverability.
2. **Rejected — Place `LOCAL_DEV.md` and `KNOWN_ENV_LIMITATIONS.md` directly in the repository root:**
   - *Reasoning:* Clutters the monorepo root with environment-specific markdown files, breaks the convention that all long-form documentation lives under `docs/`, and leaves no structured folder for future toolchain policies.
3. **Selected — Retain `docs/24-setup/` and `docs/25-toolchain/` as a formally documented deviation from Charter PART 06:**
   - *Reasoning:* Preserves the exact 00–23 charter numbering without renumbering any canonical directory, keeps root clean, and provides explicit, high-visibility homes for local developer setup and toolchain governance.

## Selected

Keep `docs/24-setup/` (holding `LOCAL_DEV.md`, `KNOWN_ENV_LIMITATIONS.md`, `README.md`, `.gitkeep`) and `docs/25-toolchain/` (holding toolchain standards, compiler/linter policies, `README.md`, `.gitkeep`) alongside the 24 Charter PART 06 directories, formally recorded as an approved deviation.

## Rationale

Appending `24-setup` and `25-toolchain` after `23-decisions` is strictly additive: zero Charter PART 06 paths are altered, developers immediately find setup docs via `README.md` links, and future governance reviews can verify the exception directly against this ADR.

## Trade-offs

- `docs/` contains 26 directories rather than strictly 24.
- Any strict directory-count validator based on Charter PART 06 must allowlist `24-setup` and `25-toolchain`.

## Risks

- Unchecked proliferation of ad-hoc `26-*`, `27-*` folders if future exceptions are added without an ADR.
- Minor conceptual overlap between `docs/14-devops/` and `docs/25-toolchain/` regarding CI toolchain versions.

## Consequences

- Local onboarding docs live in `docs/24-setup/` and toolchain governance docs live in `docs/25-toolchain/`.
- Any further deviation from the 24+2 `docs/` layout requires a new ADR approved by the Product Owner.

## Reversal Strategy

If a future charter revision adds canonical homes for local setup and toolchain docs (or the Product Owner chooses strict 24-folder compliance), move `docs/24-setup/*` into `docs/14-devops/local-setup/` and `docs/25-toolchain/*` into `docs/14-devops/toolchain/` using `git mv`, update links in root `README.md`, and mark this ADR Superseded.
