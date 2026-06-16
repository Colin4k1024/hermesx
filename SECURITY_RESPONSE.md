# Security Response

This document makes the public security response process explicit for enterprise evaluators and contributors.

## Supported Versions

| Version line | Security support |
|--------------|------------------|
| Latest released minor line | Security fixes accepted and released when feasible |
| Current development branch | Best-effort pre-release review |
| Older minor lines | Not guaranteed unless a maintainer announces an exception |

The current released/development baseline is listed in [README.md](README.md) and [docs/CHANGELOG.en.md](docs/CHANGELOG.en.md).

## Triage Targets

| Severity | Examples | Initial response target | Fix or mitigation target |
|----------|----------|-------------------------|--------------------------|
| Critical | Cross-tenant data exposure, auth bypass, arbitrary code execution in default deployment | 2 business days | 7 business days where feasible |
| High | Privilege escalation, secret leakage, sandbox or egress bypass with practical impact | 3 business days | 14 business days where feasible |
| Medium | Defense-in-depth weakness, denial of service, security-relevant misconfiguration | 5 business days | Next planned patch or minor release |
| Low | Documentation ambiguity, hardening recommendation, low-impact information exposure | 10 business days | Best-effort |

Targets are not contractual SLAs, but they are the maintainer process goal for public open-source handling.

## Disclosure Process

1. Report privately through GitHub Private Vulnerability Reporting or Security Advisories.
2. Maintainers acknowledge and assign a severity.
3. Maintainers coordinate a fix, mitigation, or documented non-issue decision.
4. Public disclosure waits until a fix or mitigation path is available unless active exploitation requires earlier notice.

## Release Artifacts

Security-relevant releases should include:

- Changelog entry with affected versions.
- Regression tests or validation evidence where feasible.
- SBOM artifacts from the supply-chain workflow.
- Release provenance attestation for binary artifacts.
