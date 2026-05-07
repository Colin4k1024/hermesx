# Enterprise SaaS Demo

End-to-end demonstration of HermesX as an Enterprise Agent Runtime.

## Scenario: Internal Customer Support Agent SaaS

A company (ACME Corp) deploys Hermes as their internal agent platform, with multiple departments as tenants.

## Prerequisites

```bash
# Start infrastructure
docker compose -f docker-compose.dev.yml up -d

# Or use production stack
docker compose -f docker-compose.prod.yml up -d
```

## Run the Demo

```bash
# Execute all 11 steps
./demo.sh

# Or run individual steps
./demo.sh step1   # Create tenant
./demo.sh step7   # Write memory
```

## What It Demonstrates

| Step | Capability | Enterprise Value |
|------|-----------|-----------------|
| 1 | Tenant creation | Multi-tenancy isolation |
| 2 | API Key management | Credential lifecycle |
| 3 | User provisioning | Identity management |
| 4 | Session creation | Conversation tracking |
| 5 | Chat completion | Agent execution |
| 6 | Tool execution | Auditable tool calls |
| 7 | Memory persistence | Cross-session knowledge |
| 8 | Usage query | Cost attribution |
| 9 | Audit log review | Compliance visibility |
| 10 | GDPR export | Data portability |
| 11 | Tenant cleanup | Right to erasure |
