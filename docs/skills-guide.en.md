# Skills Guide

> Hermes Agent Skills system: format, sources, installation, multi-tenant isolation, and built-in skills.

## Overview

Skills are pluggable capability modules for the Hermes Agent, defined via `SKILL.md` files that contain instructions and behaviors, granting the Agent specialized domain expertise.

## SKILL.md Format

Each Skill is defined by a `SKILL.md` file containing YAML frontmatter and a Markdown body:

```markdown
---
name: "skill-name"
description: "One-line description"
version: "1.0.0"
author: "author"
tags: ["tag1", "tag2"]
---

# Skill Title

Write detailed instructions, behavior rules, and examples for the Skill here.
The Agent reads this content as part of its system prompt to guide its behavior.
```

### Field Reference

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Unique Skill identifier used for installation and reference |
| `description` | Yes | Short description for search and display |
| `version` | No | Semantic version number |
| `author` | No | Author information |
| `tags` | No | Tag array for categorization and search |

### Body Content

The Markdown body is the instruction content the Agent actually reads. It should include:

- Role definition and behavior guidelines
- Available tools and operation descriptions
- Input/output format conventions
- Example conversations or interaction flows
- Limitations and notes

## Skill Sources

### 1. Local Skills

Stored in the user's home directory:

```
~/.hermes/skills/{skill-name}/SKILL.md
```

CLI mode loads local Skills directly.

### 2. Built-in Skills

The repository's `skills/` directory contains 81 pre-configured Skills in 26 categories:

| Category | Description | Examples |
|----------|-------------|---------|
| `software-development` | Software development | Code review, refactoring, debugging |
| `research` | Research and analysis | Literature review, data analysis |
| `creative` | Creative writing | Storytelling, copywriting |
| `data-science` | Data science | Data cleaning, visualization |
| `devops` | DevOps | CI/CD, containerization |
| `gaming` | Gaming | Game design, NPC dialogue |
| `github` | GitHub operations | PR review, Issue management |
| `productivity` | Productivity tools | Task management, note-taking |
| `red-teaming` | Security testing | Penetration testing, vulnerability analysis |
| `smart-home` | Smart home | Home Assistant control |
| `domain` | Domain-specific | Custom business skills |
| ... | ... | ... |

See the repository's `skills/` directory for the complete list.

### 3. MinIO Tenant Skills (SaaS Mode)

In SaaS multi-tenant mode, each tenant has independent Skills stored in MinIO/S3 object storage:

```
MinIO Bucket: hermes-skills
├── {tenant-id-1}/
│   ├── .manifest.json          # Skill manifest (hash, source, modification status)
│   ├── _soul/SOUL.md           # Tenant personality file
│   ├── skill-a/SKILL.md
│   └── skill-b/SKILL.md
├── {tenant-id-2}/
│   ├── .manifest.json
│   ├── _soul/SOUL.md
│   ├── skill-c/SKILL.md
│   └── skill-d/SKILL.md
```

**Isolation guarantee**:
- Each tenant can only access their own Skills
- Skill paths are isolated with `{tenant-id}/` as prefix
- Different tenants can have Skills with the same name but different content

### Auto-Provisioning

**Triggered automatically when creating a tenant**: When a new tenant is created via `POST /v1/tenants`, the system asynchronously executes:

1. **Skill sync**: Copies all 81 built-in skills from the `skills/` directory to the tenant's MinIO prefix
2. **Soul creation**: Generates a default `SOUL.md` personality file at `{tenant-id}/_soul/SOUL.md`
3. **Manifest write**: Creates `.manifest.json` recording each skill's SHA-256 hash and source

**Full sync on service startup**: When `hermes saas-api` starts, it iterates through all tenants for incremental sync:
- Newly added built-in skills are automatically installed
- Updated built-in skills are overwritten (unless the user has modified them)
- User-modified skills (`user_modified: true`) are not overwritten

#### Skill Manifest (.manifest.json)

```json
{
  "version": 1,
  "skills": {
    "code-review": {
      "hash": "a1b2c3d4...",
      "source": "builtin",
      "installed_at": "2026-04-29T12:00:00Z",
      "user_modified": false
    },
    "my-custom-skill": {
      "hash": "",
      "source": "user",
      "installed_at": "2026-04-29T13:00:00Z",
      "user_modified": true
    }
  },
  "synced_at": "2026-04-29T12:00:00Z"
}
```

#### Configuring MinIO

```bash
export MINIO_ENDPOINT="localhost:9000"
export MINIO_ACCESS_KEY="hermes"
export MINIO_SECRET_KEY="hermespass"
export MINIO_BUCKET="hermes-skills"
export MINIO_USE_SSL="false"
```

### Skills Management API

SaaS mode provides a RESTful API to manage tenant skills without directly operating MinIO:

#### List Skills

```bash
curl http://localhost:8080/v1/skills \
  -H "Authorization: Bearer hk_your_api_key"
```

#### Upload/Update a Skill

```bash
curl -X PUT http://localhost:8080/v1/skills/my-custom-skill \
  -H "Authorization: Bearer hk_your_api_key" \
  -d '---
name: "my-custom-skill"
description: "Custom business skill"
version: "1.0.0"
---

# My Custom Skill
...'
```

Uploaded skills are automatically marked as `user_modified` and will not be overwritten by subsequent built-in skill syncs.

#### Delete a Skill

```bash
curl -X DELETE http://localhost:8080/v1/skills/my-custom-skill \
  -H "Authorization: Bearer hk_your_api_key"
```

For detailed API reference see the [API Reference documentation](api-reference.md#skills-management-v1skills).

#### Direct Upload via mc CLI (Advanced)

```bash
mc alias set hermes http://localhost:9000 hermes hermespass
mc cp ./my-skill/SKILL.md hermes/hermes-skills/{tenant-id}/my-skill/SKILL.md
```

Or batch upload using the seed script:

```bash
./scripts/seed_minio_skills.sh
```

### 4. Skills Hub

Hermes supports discovering and installing Skills from an online Hub.

#### Default Hub Sources

| Source | Type | Trust Level | Description |
|--------|------|-------------|-------------|
| agentskills.io | URL | community | Community Skill marketplace |
| hermes-official | GitHub | trusted | Official optional Skills |

#### Search Skills

```bash
hermes skill search "code review"
```

The search engine queries all configured Hub sources and returns a list of matching Skills.

#### Install a Skill

```bash
hermes skill install <skill-name> --source <source-url>
```

Installation process:
1. Download `SKILL.md` from Hub
2. Run security scan (based on trust level)
3. Write to `~/.hermes/skills/{name}/SKILL.md`
4. Update lock file `~/.hermes/skills/.hub/lock.json`

#### Uninstall a Skill

```bash
hermes skill uninstall <skill-name>
```

## Security Scanning

Skills installed from the Hub undergo security scanning:

| Trust Level | Scan Intensity | Failure Handling |
|-------------|---------------|------------------|
| `builtin` | No scan | Install directly |
| `trusted` | Standard scan | Warn but allow installation |
| `community` | Strict scan | Block suspicious Skills |

Scan checks include:
- Dangerous instructions (e.g., attempting to execute system commands)
- Sensitive data access patterns
- Injection attack patterns

When the security scan decision is `InstallBlock`, the Skill file is automatically deleted.

## Lock File

Installed Hub Skills are recorded in the lock file:

```
~/.hermes/skills/.hub/lock.json
```

```json
[
  {
    "name": "code-review",
    "source": "https://agentskills.io/api/skills/code-review",
    "installed": "2026-04-29T12:00:00Z"
  }
]
```

The lock file is used to:
- Track installed Skills and their sources
- Support batch updates and version management
- Audit Skill installation history

## Skill Isolation in SaaS Mode

In the SaaS multi-tenant environment, Skills implement complete tenant isolation:

### Load Priority

```
1. Tenant-specific Skills (MinIO: {tenant-id}/skill-name/)
2. Global shared Skills (local skills/ directory, copied to MinIO via auto-sync)
```

### Skill Injection in Chat

When a user sends a chat request, the system automatically loads all installed skills for the tenant from MinIO and injects the skill list into the system prompt:

```
## Available Skills
- code-review: Code review assistant
- debugging: Debugging expert
- my-custom-skill: Custom business skill
```

### Isolation Testing

The project includes a complete Skill isolation test script:

```bash
# Run Skill isolation tests
./scripts/test_real_skill_isolation.sh
```

Tests verify:
- After assigning different Skills to different tenants, each tenant's Agent behaves as expected
- Cross-tenant access to other tenants' Skills does not occur
- Skill content changes do not affect other tenants

### Example: Assigning Different Personalities to Tenants

```bash
# Tenant A: pirate style
mc cp pirate-skill/SKILL.md hermes/hermes-skills/tenant-${TENANT_A}/pirate/SKILL.md

# Tenant B: scientist style
mc cp scientist-skill/SKILL.md hermes/hermes-skills/tenant-${TENANT_B}/scientist/SKILL.md
```

Both tenants call the same Chat API, but the Agent exhibits different personalities and expertise.

## Creating Custom Skills

### 1. Create Directory and File

```bash
mkdir -p ~/.hermes/skills/my-custom-skill
cat > ~/.hermes/skills/my-custom-skill/SKILL.md << 'EOF'
---
name: "my-custom-skill"
description: "Custom business Skill"
version: "1.0.0"
author: "your-name"
tags: ["custom", "business"]
---

# My Custom Skill

## Role
You are a professional XX assistant.

## Behavior Guidelines
- Always use professional terminology
- Keep answers concise and accurate

## Output Format
Output results in Markdown format.
EOF
```

### 2. Verify Loading

```bash
hermes skill list
```

### 3. Deploy to SaaS (Recommended: Use API)

```bash
curl -X PUT http://localhost:8080/v1/skills/my-custom-skill \
  -H "Authorization: Bearer hk_your_api_key" \
  --data-binary @~/.hermes/skills/my-custom-skill/SKILL.md
```

Or directly via MinIO:

```bash
mc cp ~/.hermes/skills/my-custom-skill/SKILL.md \
  hermes/hermes-skills/${TENANT_ID}/my-custom-skill/SKILL.md
```

## Related Documentation

- [Getting Started](saas-quickstart.md) — Basic environment setup
- [Configuration Guide](configuration.md) — MinIO and Skills configuration
- [Architecture Overview](architecture.md) — Skills system in the architecture
- [Authentication](authentication.md) — Tenant and API Key management
