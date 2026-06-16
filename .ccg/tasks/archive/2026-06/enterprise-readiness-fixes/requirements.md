# Requirements

User requested starting fixes for the enterprise open-source readiness gaps identified in the previous review.

Scope for this first implementation batch:
- Improve supply-chain CI artifacts and dependency hygiene.
- Harden Helm and production Compose defaults where safe without requiring a live cluster.
- Add persistent egress blocked-event storage/API wiring if it fits existing store patterns.
- Improve governance and documentation consistency.
- Remove tracked local AI-agent config files from the repository.

Out of scope for this batch:
- Producing live Keycloak/Auth0 evidence.
- Producing real DR/PITR RTO/RPO evidence.
- Claiming `v2.4.0` stable enterprise sign-off.
- Large redesign of auth, tenancy, or billing.

Verification expectations:
- `go test ./...`
- tenant SQL static checks
- relevant Helm/Compose render checks
- WebUI typecheck if frontend-adjacent docs or config are touched
