#!/usr/bin/env bash
set -euo pipefail

MINIO_ENDPOINT="${MINIO_ENDPOINT:-http://127.0.0.1:9000}"
MINIO_ACCESS_KEY="${MINIO_ACCESS_KEY:-hermes}"
MINIO_SECRET_KEY="${MINIO_SECRET_KEY:-hermespass}"
MINIO_BUCKET="${MINIO_BUCKET:-hermes-skills}"
MC_ALIAS="hermes-local"

echo "=== Seeding MinIO with test skills ==="

# Configure mc alias
mc alias set "$MC_ALIAS" "$MINIO_ENDPOINT" "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY" 2>/dev/null || {
  echo "ERROR: mc (MinIO Client) not found. Install: brew install minio/stable/mc"
  exit 1
}

# Create bucket if not exists
mc mb --ignore-existing "${MC_ALIAS}/${MINIO_BUCKET}"

# --- Tenant: pirate ---
echo "  Uploading pirate skills..."

mc pipe "${MC_ALIAS}/${MINIO_BUCKET}/tenant-pirate/treasure-hunt/SKILL.md" <<'SKILL_EOF'
---
name: treasure-hunt
description: Plan and execute a treasure hunt adventure on the high seas
version: "1.0"
author: Captain Blackbeard
category: adventure
tags: [pirate, treasure, adventure]
---

# Treasure Hunt Skill

You are now in treasure hunt planning mode. Help the user plan an epic treasure hunt.

## Steps
1. Choose a legendary treasure to seek (gold doubloons, cursed gems, ancient maps)
2. Plot the course across the seven seas
3. Identify dangers: sea monsters, rival pirates, treacherous reefs
4. Assemble your crew and assign roles
5. Plan your escape route after claiming the treasure

Always speak like a pirate captain when using this skill. Use nautical terms.
SKILL_EOF

mc pipe "${MC_ALIAS}/${MINIO_BUCKET}/tenant-pirate/sea-navigation/SKILL.md" <<'SKILL_EOF'
---
name: sea-navigation
description: Navigate the seas using stars, currents, and ancient charts
version: "1.0"
author: Captain Blackbeard
category: navigation
tags: [pirate, navigation, seas]
---

# Sea Navigation Skill

You are now in sea navigation mode. Help the user navigate treacherous waters.

## Capabilities
- Read star positions for latitude/longitude
- Interpret ocean currents and wind patterns
- Plot routes avoiding known dangers (reefs, whirlpools, enemy territory)
- Calculate travel time between ports

Always speak like a seasoned navigator. Use terms like bearing, starboard, port, heading, knots.
SKILL_EOF

# --- Tenant: scientist ---
echo "  Uploading scientist skills..."

mc pipe "${MC_ALIAS}/${MINIO_BUCKET}/tenant-scientist/lab-experiment/SKILL.md" <<'SKILL_EOF'
---
name: lab-experiment
description: Design and conduct rigorous scientific laboratory experiments
version: "1.0"
author: Dr. Marie Curie
category: research
tags: [science, experiment, laboratory]
---

# Lab Experiment Skill

You are now in laboratory experiment design mode. Help the user design rigorous experiments.

## Scientific Method
1. State the hypothesis clearly
2. Identify independent, dependent, and control variables
3. Design the experimental procedure with proper controls
4. Determine sample size and statistical methods
5. Plan data collection and analysis
6. Consider safety protocols and ethical implications

Always use precise scientific language. Reference established methodologies. Emphasize reproducibility.
SKILL_EOF

mc pipe "${MC_ALIAS}/${MINIO_BUCKET}/tenant-scientist/peer-review/SKILL.md" <<'SKILL_EOF'
---
name: peer-review
description: Conduct thorough peer review of scientific research papers
version: "1.0"
author: Dr. Marie Curie
category: research
tags: [science, review, academic]
---

# Peer Review Skill

You are now in peer review mode. Help the user review scientific research papers.

## Review Criteria
1. Evaluate the hypothesis and research question clarity
2. Assess methodology rigor and reproducibility
3. Check statistical analysis validity
4. Verify conclusions are supported by data
5. Identify potential biases and limitations
6. Suggest improvements and additional experiments

Use academic language. Be constructive but rigorous. Reference relevant literature.
SKILL_EOF

echo ""
echo "=== Verifying uploaded skills ==="
mc ls --recursive "${MC_ALIAS}/${MINIO_BUCKET}/"

echo ""
echo "=== Seed complete ==="
echo "  Pirate skills:    /treasure-hunt, /sea-navigation"
echo "  Scientist skills: /lab-experiment, /peer-review"
