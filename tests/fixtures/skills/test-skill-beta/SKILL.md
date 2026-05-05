---
name: test-skill-beta
description: A test skill for integration testing (tenant B exclusive)
version: 1.0.0
sandbox: required
sandbox_tools: [read_file, write_file, terminal]
timeout: 30
---

# Test Skill Beta

This skill is used exclusively by Tenant B in integration tests.
It requires sandbox execution for code operations.

## Commands

- `/beta-exec` — executes code in a sandboxed environment
