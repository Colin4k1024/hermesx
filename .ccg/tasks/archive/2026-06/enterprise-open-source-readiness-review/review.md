# Enterprise Open-Source Readiness Review

## Executive Assessment

HermesX is not a toy project: it already has a meaningful enterprise foundation across multi-tenancy, RBAC/API keys/OIDC wiring, audit logs, execution receipts, safety/egress controls, workflows, observability assets, deployment assets, and CI gates. The local baseline is green:

- `go test ./...` passed.
- `scripts/check_tenant_sql.sh` passed.
- `scripts/check_tenant_sql_mysql.sh` passed.
- `npm --prefix webui run typecheck` passed.

No Critical issue was confirmed in this review. The remaining enterprise gap is mostly about hardening evidence, production deployment posture, supply-chain attestations, governance maturity, and operational proof.

External dual-model review was attempted twice per CCG rules. Both attempts timed out or were cancelled because the model wrapper suffered stream disconnects/session timeout before producing usable findings. This report is therefore based on local repository inspection and the verification commands above.

## Findings

### High: Enterprise sign-off still depends on live OIDC and real DR evidence

Evidence:
- `docs/ENTERPRISE_READINESS.en.md` marks OIDC live IdP evidence, K8s Job sandbox validation, egress smoke, observability staging import, and DR evidence as follow-ups.
- `docs/artifacts/2026-06-09-enterprise-release-evidence/README.md` says live Keycloak/Auth0 runs were not executed and DR/PITR checks did not pass in the local environment.

Impact:
- Enterprise security teams will not treat in-process OIDC unit tests and local render checks as enough for production identity and recovery claims.
- DR claims need measured RTO/RPO from a prepared environment, not just scripts and runbooks.

Recommended next steps:
- Add a repeatable staging evidence suite for Keycloak and Auth0, including `/v1/me`, RBAC allow/deny, expired/wrong-audience/missing-tenant failures, API-key fallback, and JWKS rotation.
- Run PostgreSQL PITR plus Redis and MinIO restore drills in a prepared environment; publish redacted evidence with RTO/RPO.
- Promote these to release gates before calling `v2.4.0` enterprise-ready.

### High: Production Helm/Compose posture is usable but not enterprise hardened

Evidence:
- `deploy/helm/hermesx/values.yaml` defaults to `image.tag: latest` and stores database/API/LLM/MinIO secrets as plain env values.
- `deploy/helm/hermesx/templates/deployment.yaml` injects those values directly instead of `secretKeyRef` or external secret references.
- The Helm chart has container `securityContext`, HPA, PDB, and probes, but no NetworkPolicy, ServiceAccount/RBAC split, ExternalSecret support, or read-only root filesystem.
- `docker-compose.prod.yml` exposes PostgreSQL, Redis, MinIO, OTel, and Jaeger ports on host interfaces and defaults `SAAS_ALLOWED_ORIGINS` to `*` when unset.

Impact:
- Platform teams can test it, but a production cluster review will flag secret handling, mutable image references, broad network exposure, and missing namespace-level controls.

Recommended next steps:
- Replace secret env defaults with Secret/ExternalSecret patterns and fail install if required secrets are placeholders.
- Pin images by immutable tag/digest and publish upgrade/rollback guidance.
- Add optional NetworkPolicy, pod security settings, read-only root filesystem support, serviceAccount/RBAC configuration, and ingress/TLS examples.
- Split compose files clearly into `dev`, `staging`, and `prod-secure` profiles; avoid wildcard CORS fallback in production examples.

### High: Supply-chain story lacks enterprise artifacts

Evidence:
- `.github/workflows/security.yml` runs govulncheck, gosec, and Trivy SARIF upload.
- Release workflow builds binaries and checksums, but no SBOM, provenance, signing, SLSA attestation, dependency update automation, or license inventory was found.
- A prior PRD claims CodeQL coverage, but current `security.yml` does not run CodeQL.

Impact:
- Enterprise open-source intake often requires SBOM, signed artifacts/images, provenance, vulnerability policy, dependency update cadence, and third-party license review.

Recommended next steps:
- Generate CycloneDX or SPDX SBOMs for Go, WebUI, and container images.
- Sign binaries and images with cosign/sigstore; attach GitHub artifact attestations or SLSA provenance.
- Add CodeQL or clarify that Trivy/gosec/govulncheck are the supported SAST set.
- Add Dependabot/Renovate with digest update support and a third-party license/NOTICE inventory.

### High: Runtime governance needs persistent operator evidence for denied egress

Evidence:
- `internal/egress/admin_handler.go` exposes `/admin/v1/egress/blocked-log`, but the implementation is explicitly a placeholder that returns active rules, not persisted blocked decisions.

Impact:
- Egress deny-all and SSRF controls are strong primitives, but enterprise incident response needs an auditable history of denied attempts, source tenant/user/tool, reason, host/path, and trace correlation.

Recommended next steps:
- Add a persistent `egress_block_events` table/store/API with retention/export.
- Wire `SecureTransport` denied decisions into the store asynchronously.
- Add dashboard and alert panels for denied spikes and policy misses.

### Medium: Open-source governance is too lightweight for enterprise buyers

Evidence:
- `GOVERNANCE.md` documents a lightweight maintainer-led model.
- `SUPPORT.md` states support is best effort unless separately agreed.
- `SECURITY.md` uses best-effort triage language and has no fixed response timeline.
- No CODEOWNERS, MAINTAINERS, ADOPTERS, DCO/CLA, LTS policy, or support window file was found.

Impact:
- The project can be used, but buyers will need stronger answers for ownership, review authority, vulnerability response, release compatibility, and support expectations.

Recommended next steps:
- Add `CODEOWNERS`, `MAINTAINERS.md`, security severity/response SLA, supported-version window, deprecation policy, and release cadence.
- Decide DCO vs CLA and document contribution IP expectations.
- Add an adopters/case-studies page once there are external users.

### Medium: Documentation and evidence are broad but not fully self-consistent

Evidence:
- `docs/index.en.md` still says "5 roles", while `docs/RBAC_MATRIX.en.md` lists additional roles such as `platform_admin`, `security_admin`, `billing_admin`, `ops_admin`, and `tenant_admin`.
- A historical security remediation PRD says the security workflow includes CodeQL, but current workflow does not.
- Multiple docs distinguish `v2.3.0` stable from `v2.4.0-dev`, but the surface area is large enough that stale claims are easy to miss.

Impact:
- Enterprise evaluators trust evidence consistency. Small mismatches create doubt about which claims are current and supported.

Recommended next steps:
- Add docs consistency checks for role counts, version labels, OpenAPI path counts, and referenced evidence files.
- Make `docs/ENTERPRISE_READINESS.en.md` the source of truth and link all marketing/README claims back to it.
- Separate archived PRDs from current evidence more visibly.

### Medium: Tool/skill ecosystem needs stronger marketplace governance

Evidence:
- Skills guide documents community/trusted/builtin trust levels and security scan behavior.
- Built-in/tenant skills and hub installation exist, but enterprise distribution artifacts need stronger provenance, signatures, trust policy, and review workflow.

Impact:
- Agents consume skills as instructions. For enterprises, skill provenance and update control are part of the execution security boundary.

Recommended next steps:
- Sign official skill bundles and maintain lockfiles with hashes, source, trust level, and review metadata.
- Add admin policy controls for allowed skill sources, update approval, rollback, and tenant-level quarantine.
- Publish a threat model specifically for skill prompt injection and supply-chain poisoning.

### Medium: Observability exists, but SLO/incident operations are not complete

Evidence:
- Prometheus, Grafana, OTel, alerts, runbooks, and stress report artifacts exist.
- Readiness docs still call for dashboard import and alert dry-run in staging.

Impact:
- Enterprises need operational ownership: SLOs, error budgets, alert routing, dashboard screenshots, runbook drills, and capacity envelopes.

Recommended next steps:
- Define default SLOs for API availability, p95/p99 latency, agent/tool execution failure rate, egress denials, scheduler lag, and queue/backlog.
- Add staging alert dry-run evidence and incident runbooks.
- Publish capacity guidance by deployment size and tenant count.

### Low: Repository hygiene should remove tracked local agent files

Evidence:
- `.claude/settings.local.json` and `.deepseek/trusted` are tracked, while `.gitignore` now ignores `.claude/` and `.deepseek/`.

Impact:
- The visible content is not secret in this checkout, but enterprise security review will flag local agent configuration in the repository.

Recommended next steps:
- Remove these files from Git tracking and add a short policy for local AI-agent config.
- Consider a pre-commit or CI check to prevent future agent-local directories from being committed.

### Low: Dockerfiles should avoid hardcoded regional package mirrors

Evidence:
- `Dockerfile` and `Dockerfile.saas` rewrite Debian sources to `mirrors.aliyun.com`.

Impact:
- This may help one region but weakens reproducibility and can be questioned in global enterprise builds.

Recommended next steps:
- Use default Debian sources by default and make regional mirrors opt-in via build ARG.
- Document expected base image digest and rebuild cadence.

## Suggested Priority Order

1. Close live OIDC and DR evidence, then cut a stable `v2.4.0` with evidence links.
2. Harden Helm/Compose production defaults and Kubernetes security posture.
3. Add SBOM, signing, provenance, dependency automation, and license inventory.
4. Persist denied egress events and expose audit/export/dashboard workflows.
5. Upgrade governance/support/security policy from best-effort to enterprise-readable commitments.
6. Add docs consistency checks and clean stale evidence claims.
7. Deepen skill marketplace governance and operational SLO/runbook evidence.
