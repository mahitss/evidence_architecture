# ADR-004: Enforce Go `1.27.4` Tarball Precedence Over EOL Debian Go `1.19.8`

- **Status:** Accepted
- **Date:** 2026-11-26
- **Deciders:** Product Owner, Architect (MODE A)
- **Refs:** `.tool-versions`, `docs/24-setup/KNOWN_ENV_LIMITATIONS.md`, `Makefile`

## Context

During the Prompt 2 capability probe, Debian 12's `apt-get install -y golang-go` installed Go `1.19.8` at `/usr/bin/go` (end-of-life, lacking modern standard library packages, loop-variable semantics, and toolchain directives). Subsequently, official Go `1.27.4` (verified against `https://go.dev/VERSION?m=text` and binary SHA-256) was installed from tarball at `/usr/local/go/bin/go`. However, the container's default `PATH` (`/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin`) does not include `/usr/local/go/bin`, so an unconfigured shell resolves bare `go` to `/usr/bin/go` (`1.19.8`).

## Problem

If any build script, developer shell, or CI step invokes `go` without `/usr/local/go/bin` ahead of `/usr/bin`, it silently picks up EOL Go `1.19.8`, leading to cryptic compile failures, missing security patches, or incompatible module resolution.

## Alternatives

1. **Rejected — Purge `golang-go` via `apt-get remove`:**
   - *Reasoning:* `dpkg` on this host is currently in a dirty state (`postgresql-common` and `sysstat` unconfigured due to `/var/log` overlay restrictions). Running `apt` package removals risks cascading `dpkg` failures or unintended side effects and violates our policy of avoiding `apt` mutations without explicit approval.
2. **Rejected — Replace `/usr/bin/go` with a manual symlink to `/usr/local/go/bin/go`:**
   - *Reasoning:* Mutating `dpkg`-owned paths under `/usr/bin` creates silent package-manager drift, could be overwritten by any future package trigger, and hides misconfigured `PATH` environments instead of surfacing them explicitly.
3. **Selected — Enforce `PATH` precedence (`/usr/local/go/bin` before `/usr/bin`) in `Makefile`, shell setup docs, and CI verification:**
   - *Reasoning:* Non-destructive, transparent, works across every environment, and provides an automated fail-fast gate (`make env-check`).

## Selected

Keep official Go `1.27.4` in `/usr/local/go/bin/go`, prepend `/usr/local/go/bin` (and `$HOME/.cargo/bin`) in the root `Makefile`, document `export PATH="/usr/local/go/bin:$HOME/.cargo/bin:$PATH"` in `docs/24-setup/LOCAL_DEV.md`, enforce `command -v go == /usr/local/go/bin/go` in `make env-check`, and require a dedicated CI step verifying `PATH` ordering and pinned Go version before building `backend-go/`.

## Rationale

Explicit `PATH` ordering combined with a hard assertion in `make env-check` (and CI) guarantees that builds either use verified Go `1.27.4` or fail immediately with an actionable diagnostic message, without touching the fragile `dpkg` state.

## Trade-offs

- Interactive subshells outside `make` still resolve `/usr/bin/go` (`1.19.8`) unless the developer exports `PATH` in their shell profile.
- Slightly more boilerplate in shell onboarding and CI workflow definitions.

## Risks

- A developer running `go build` directly in a fresh shell without sourcing `PATH` will hit Go `1.19.8` errors.
- Future upgrades to Go in `/usr/local/go` must also update `.tool-versions` and CI version checks.

## Consequences

- All `make` targets automatically resolve `/usr/local/go/bin/go` (`1.27.4`).
- CI workflows (to be created in `docs/14-devops` / `.github/workflows`) MUST include an explicit step verifying `which go`, `go version`, and `PATH` ordering before compiling or testing Go code.

## Reversal Strategy

If the host container is rebuilt without Debian's `golang-go` package (or once `dpkg` is clean and `golang-go` is formally approved for removal), remove the shadowing warning from `KNOWN_ENV_LIMITATIONS.md` while keeping the `go version` check in `make env-check` and CI.
