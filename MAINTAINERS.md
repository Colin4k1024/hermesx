# Maintainers

HermesX currently uses a maintainer-led governance model.

| Area | Primary maintainer |
|------|--------------------|
| Project direction and releases | [@Colin4k1024](https://github.com/Colin4k1024) |
| Security-sensitive changes | [@Colin4k1024](https://github.com/Colin4k1024) |
| Deployment and CI/CD | [@Colin4k1024](https://github.com/Colin4k1024) |

Review routing is enforced through [.github/CODEOWNERS](.github/CODEOWNERS).

## Maintainer Responsibilities

- Review public API, security, deployment, and governance changes before merge.
- Keep release notes, supported versions, and enterprise readiness evidence current.
- Triage security reports according to [SECURITY_RESPONSE.md](SECURITY_RESPONSE.md).
- Avoid merging changes that weaken tenant isolation, auditability, or production-safe defaults without explicit documentation.

## Contributor IP Policy

HermesX does not currently require a CLA. Contributions are accepted under the inbound-equals-outbound model: by submitting a contribution, you agree it is licensed under the project MIT license.
