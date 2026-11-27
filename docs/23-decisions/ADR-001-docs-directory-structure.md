# ADR-001: Conform `docs/` Directory Structure to Charter PART 06

- **Status:** Accepted
- **Date:** 2026-11-26
- **Deciders:** Product Owner, Architect (MODE A)
- **Refs:** Charter PART 06, Audit Flag 1, ADR-002

## Context

Charter PART 06 defines a canonical 24-folder documentation taxonomy under `docs/` (`00-overview` through `23-decisions`). During Prompt 3 (commit `5a1229a`), because the full charter PART 06 folder list was not yet in the repository, 26 directories were initialized using provisional slugs (`01-vision`, `02-research`, `03-personas`, `04-requirements`, `05-domain-model`, `06-architecture`, `07-adr`, `08-data-model`, `09-api-contracts`, `10-ui-ux`, `11-security`, `12-privacy-compliance`, `13-ingestion-pipeline`, `14-search-indexing`, `18-deployment`, `19-operations`, `20-roadmap`, `21-backlog`, `23-decisions-risks`, plus `24-setup` and `25-toolchain`).

## Problem

Leaving the provisional Prompt 3 directory layout creates immediate drift against the Universal Enterprise Product Engineering System v1.0 charter, breaking cross-project navigation, governance audits, and future automation that expects charter PART 06 paths.

## Alternatives

1. **Rejected — Keep the Prompt 3 structure and amend the charter:**
   - *Reasoning:* Mutating an enterprise-wide standard to accommodate a one-time provisional scaffold undermines consistency across repositories, forces manual exceptions in every audit checklist, and provides no structural advantage over PART 06.
2. **Rejected — Keep both directory structures side-by-side:**
   - *Reasoning:* Maintaining parallel trees duplicates documentation ownership, creates broken/ambiguous canonical paths (e.g., `05-architecture` vs `06-architecture`), and causes stale content drift as contributors edit different folders.
3. **Selected — Migrate to exact Charter PART 06 taxonomy with two explicitly documented exceptions (`24-setup`, `25-toolchain`):**
   - *Reasoning:* Restores 100% alignment with PART 06 while preserving git history via `git mv` and isolating the two environment-specific additions under ADR-002.

## Selected

Adopt the exact 24 Charter PART 06 directories (`00-overview` through `23-decisions`) plus the two ADR-002 exceptions (`24-setup`, `25-toolchain`), using the following `git mv` mapping:

| Prior Directory (Prompt 3) | Target Path (Charter PART 06) |
|---|---|
| `docs/01-vision` | `docs/01-product/VISION.md` |
| `docs/02-research` | `docs/03-research/` |
| `docs/03-personas` | `docs/01-product/PERSONAS.md` |
| `docs/04-requirements` | `docs/02-requirements/` |
| `docs/05-domain-model` | `docs/05-architecture/DOMAIN_MODEL.md` |
| `docs/06-architecture` | `docs/05-architecture/` |
| `docs/07-adr` | `docs/23-decisions/` |
| `docs/08-data-model` | `docs/08-database/DATA_MODEL.md` |
| `docs/09-api-contracts` | `docs/09-api/API_CONTRACTS.md` |
| `docs/10-ui-ux` | `docs/04-design/` |
| `docs/11-security` | `docs/10-security/` |
| `docs/12-privacy-compliance` | `docs/11-privacy/` (compliance copy in `docs/19-compliance/COMPLIANCE.md`) |
| `docs/13-ingestion-pipeline` | `docs/13-infrastructure/INGESTION.md` |
| `docs/14-search-indexing` | `docs/09-api/SEARCH.md` |
| `docs/15-testing` | `docs/15-testing/` (unchanged) |
| `docs/16-performance` | `docs/16-performance/` (unchanged) |
| `docs/17-observability` | `docs/17-observability/` (unchanged) |
| `docs/18-deployment` | `docs/13-infrastructure/DEPLOYMENT.md` |
| `docs/19-operations` | `docs/20-operations/` |
| `docs/20-roadmap` | `docs/01-product/ROADMAP.md` |
| `docs/21-backlog` | `docs/01-product/BACKLOG.md` |
| `docs/22-progress` | `docs/22-progress/` (unchanged) |
| `docs/23-decisions-risks` | `docs/21-governance/RISKS.md` (ADRs in `docs/23-decisions/`) |
| `docs/24-setup` | `docs/24-setup/` (preserved exception per ADR-002) |
| `docs/25-toolchain` | `docs/25-toolchain/` (preserved exception per ADR-002) |

New Charter PART 06 directories initialized: `06-frontend`, `07-backend`, `12-ai-ml`, `14-devops`, `18-reliability`, `19-compliance`, `21-governance`.

## Rationale

Performing the rename while only 1 foundation commit exists and zero application code has landed keeps migration cost minimal, preserves rename history via `git mv`, and prevents charter drift before architecture and implementation artifacts accumulate.

## Trade-offs

- Slightly more top-level folders (26 vs 24) due to retaining `24-setup` and `25-toolchain`.
- Early placeholder references (such as `backend-go/README.md` mentioning `docs/06-architecture/`) will need a minor link update when `backend-go/` is scaffolded.

## Risks

- Contributors referencing Prompt 3 paths from memory could link to removed folders.
- Automated charter linters checking for *strictly* 24 folders could flag `24-setup` and `25-toolchain` unless configured to recognize ADR-002.

## Consequences

- All future product, architecture, security, compliance, and operational documentation lands in standardized Charter PART 06 locations.
- Every folder under `docs/` has a `.gitkeep` and a one-line `README.md` clarifying ownership.

## Reversal Strategy

Because all file transitions were performed with `git mv` in a single atomic commit, reverting to an alternative taxonomy requires only `git revert <commit>` or scripted `git mv` back to the prior mapping table above.
