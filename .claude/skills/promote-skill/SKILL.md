---
name: promote-skill
description: Promote a local skill to the Cobli agents-marketplace. Transforms .claude/skills/<name>/ into the marketplace plugin format and opens a PR.
argument-hint: <skill-name>
allowed-tools: Bash, Read, Write
disable-model-invocation: true
user-invocable: true
---

# Promote Skill to Marketplace

Promotes a local skill from `.claude/skills/<name>/` to `Cobliteam/agents-marketplace`.

## Instructions

Run the promote script in plan mode first:

```bash
bash ~/workflow/.claude/skills/promote-skill/scripts/promote.sh "$ARGUMENTS" --plan
```

Present the output to the user and wait for explicit confirmation before proceeding.

After confirmation:

```bash
bash ~/workflow/.claude/skills/promote-skill/scripts/promote.sh "$ARGUMENTS" --execute
```

If the script exits with a non-zero code, read the error message and explain it to the user without retrying automatically.
