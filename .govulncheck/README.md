# Govulncheck Baseline Suppression

This directory contains the baseline suppression file for `govulncheck`, Go's official vulnerability checker.

## Purpose

The baseline suppression file (`.govulncheck/baseline.json`) is used to suppress known false positives or acceptable vulnerabilities that have been reviewed and accepted by the team.

## Current Status

As of the last update, there are **no suppressed vulnerabilities**. All Go dependencies are clean.

## How to Add a Suppression

If `govulncheck` reports a vulnerability that you want to suppress:

1. **Review the vulnerability** - Ensure it's a false positive or acceptable risk
2. **Document the decision** - Add an entry to `baseline.json` with:
   - Vulnerability ID (e.g., `GO-2024-1234`)
   - Reason for suppression
   - Date of review
   - Reviewer name
3. **Update the CI workflow** - The workflow will automatically check for the baseline file

## Baseline File Format

```json
{
  "version": "1.0.0",
  "description": "Govulncheck baseline suppression file for HermesX",
  "last_updated": "YYYY-MM-DD",
  "suppressed_vulnerabilities": [
    {
      "id": "GO-2024-1234",
      "package": "golang.org/x/example",
      "reason": "False positive - not exploitable in our usage",
      "reviewed_by": "developer@example.com",
      "reviewed_date": "2024-01-15"
    }
  ]
}
```

## Running Govulncheck Locally

```bash
# Install govulncheck
go install golang.org/x/vuln/cmd/govulncheck@latest

# Run vulnerability check
govulncheck ./...

# Run with baseline (if implemented)
govulncheck -baseline=.govulncheck/baseline.json ./...
```

## References

- [govulncheck documentation](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck)
- [Go vulnerability database](https://vuln.go.dev/)
- [Security workflow](../.github/workflows/security.yml)
