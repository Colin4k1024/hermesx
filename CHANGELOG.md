# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- ExecutionReceipt for immutable tool audit trail
- OpenAPI specification covering all endpoints
- Go and TypeScript SDKs
- Governance client interface for L2 integration
- Oris metric event emission for self-evolution loop
- Grafana dashboard with 5 monitoring panels
- Backup automation scripts for Redis and MinIO
- K8s Job-based sandbox for enterprise deployment
- Notion platform adapter
- Rate limiter integration tests
- govulncheck baseline suppression file

### Changed
- Updated Go SDK to fix compilation errors
- Enhanced security with scope-based access control

### Fixed
- Go SDK duplicate type declarations
- CI integration test configuration

## [2.4.0-dev] - 2026-06-25

### Added
- Multi-tenant isolation with PG RLS
- 50+ tools with 15 platform adapters
- LLM resilience with 3-layer circuit breaker
- OTel tracing and Prometheus metrics
- Safety pipeline and secret leak scanner
- Workflow engine with Eino Agent Runtime
- GDPR compliance endpoints
- Usage metering and alerting

## [2.3.0] - 2026-05-15

### Added
- Initial SaaS mode support
- API Key authentication
- Rate limiting with Redis
- MinIO object storage integration

## [2.2.0] - 2026-04-01

### Added
- Multi-platform gateway (Telegram, Discord, Slack, etc.)
- Tool execution framework
- Session management

## [2.1.0] - 2026-03-01

### Added
- Core agent implementation
- LLM provider abstraction
- Basic tool support

## [2.0.0] - 2026-02-01

### Added
- Initial release
- Go-based agent runtime
- CLI interface

[Unreleased]: https://github.com/Colin4k1024/hermesx/compare/v2.4.0-dev...HEAD
[2.4.0-dev]: https://github.com/Colin4k1024/hermesx/compare/v2.3.0...v2.4.0-dev
[2.3.0]: https://github.com/Colin4k1024/hermesx/compare/v2.2.0...v2.3.0
[2.2.0]: https://github.com/Colin4k1024/hermesx/compare/v2.1.0...v2.2.0
[2.1.0]: https://github.com/Colin4k1024/hermesx/compare/v2.0.0...v2.1.0
[2.0.0]: https://github.com/Colin4k1024/hermesx/releases/tag/v2.0.0
