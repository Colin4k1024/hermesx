# Plan

## Phase 1 - Supply Chain And Repository Hygiene

- Add dependency update automation for Go, WebUI npm, GitHub Actions, and Docker.
- Add SBOM/provenance/signing-oriented release artifacts where GitHub Actions supports it without requiring extra secrets.
- Remove tracked local AI-agent configuration files already covered by `.gitignore`.
- Add a small docs note describing local AI-agent config policy.

## Phase 2 - Deployment Hardening

- Update Helm values/templates to support existing Secret references for sensitive env vars.
- Avoid mutable `latest` defaults for the HermesX image.
- Add configurable pod security context, read-only-root-filesystem support, service account wiring, and optional NetworkPolicy.
- Tighten production Compose defaults where possible and document public-port caveats.

## Phase 3 - Egress Audit Depth

- Add persistent blocked egress event model/store/migrations for PostgreSQL and MySQL if the existing store seams allow a focused change.
- Wire `SecureTransport` denied decisions to an async recorder.
- Change `/admin/v1/egress/blocked-log` from placeholder to backed query.
- Add focused tests for recorder and API behavior.

## Phase 4 - Governance And Documentation Consistency

- Add maintainer/CODEOWNERS/security response/deprecation/support-window materials.
- Fix role-count and CodeQL/security workflow inconsistencies.
- Update Enterprise Readiness with this batch's improved-but-not-complete status.

## Phase 5 - Verification And Review

- Run Go tests, tenant SQL static checks, Helm template, Docker Compose config, WebUI typecheck if relevant.
- Run or attempt CCG dual-model review; record tool limitations if unavailable.
- Archive the CCG task.
