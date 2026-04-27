---
name: workflow-conventions
description: Operational conventions for the workflow platform. Apply automatically when working in ~/workflow — artifact placement, archiving rules, skill promotion gate, and memory file routing.
user-invocable: false
---

# Workflow Platform — Operational Conventions

Complement to CLAUDE.md. These are granular rules that guide day-to-day decisions without needing to re-read contracts.

## Artifact placement (quick ref)

Everything goes in `~/workflow/` (the repo). The only exception is credential **values**, which live in macOS Keychain.

| Artifact | Destination |
|----------|-------------|
| Savepoints (técnico + rico) | `docs.db` via `wtb cycle-check --save` + `wtb doc add --type savepoint` — **nunca .md** |
| Discoveries, 1on1, postmortem, review, runbook, incident | `docs.db` via `wtb doc add --type <tipo>` |
| Drafts — comunicados, anúncios, emails, posts Slack | `docs.db` via `wtb doc add --type draft` |
| Backlog e tarefas | `backlog.db` via `wtb backlog add` |
| Session state | `session.yml` — gitignored, env-specific |
| DB credentials cache | `~/.workflow/db-creds.yml` — gitignored, nunca no repo |
| Monitor scripts | `monitors/` |
| Git hooks | `hooks/` |
| Utility scripts, notebooks | `scripts/`, `notebooks/` |
| Operational patterns/runbooks | `patterns/` |
| Go code, plan templates, use-case definitions | `pkg/`, `templates/`, `use-cases/` |
| Named DB queries | `db-queries/<repo>.yml` |

**Credential resolution chain:** `session.yml` → env var → secret store (`secret-get.sh` — macOS Keychain / GNOME Keyring / pass) → prompt.
Registered Keychain services: `workflow-dd-api-key`, `workflow-dd-app-key`, `workflow-slack-token`.

## Archiving rules

Archive a discovery when its status becomes `concluded`, `defer`, or `published`:
1. `git mv docs/workflow/discovery/<file> docs/workflow/discovery/archive/YYYY-MM/`
2. Update `discovery/INDEX.md` — remove from active table, add line in archive section

Archive savepoints: new savepoints go to `savepoints/YYYY-MM/`. At month turn, folder is already correct — no migration needed.

## MCPs — usar antes de subprocess/CLI

| MCP | Quando usar | Ferramentas chave |
|-----|------------|-------------------|
| **GitHub** | PR review, CI checks, issues, deployments | `list_check_runs`, `get_pull_request`, `list_workflow_runs` |
| **Atlassian** | Jira: buscar, criar, transicionar; Confluence | `getJiraIssue`, `searchJiraIssuesUsingJql`, `createJiraIssue` |
| **Notion** | Docs Notion | `notion-fetch`, `notion-search`, `notion-create-pages` |
| **Figma** | Designs, refinamentos | `get_design_context`, `get_screenshot` |

**Regra:** MCP sempre antes de `gh`/curl/subprocess — output JSON estruturado.

## Memory topic files — load on demand

Do not preload all memory files. Load the relevant topic file when the task requires it.

| Topic | File | Load when |
|-------|------|-----------|
| Airbyte, Datadog, CDC | `memory/airbyte-ops.md` | incident, sync health, ops |
| Kotlin/Gradle, Spring Boot | `memory/kotlin-gradle.md` | any Kotlin PR |
| k8s prod access (psql, scylla) | `memory/k8s-prod-access.md` | prod investigation |
| Investigation methodology | `memory/investigation-methodology.md` | data analysis |
| Webhook architecture | `memory/webhook-ss2269.md` | webhook work |

## Skill promotion gate

A local skill (`.claude/skills/<name>/SKILL.md`) is promoted to the company marketplace via `/promote-skill <name>`.

**Gate:** the skill must have `marketplace-ready: true` in its YAML frontmatter. `/promote-skill` refuses if this field is absent or false.

Add it when the skill has been validated in real sessions and contains no workflow-specific dependencies (no `wtb` CLI, no hardcoded paths, no credentials).

## docs/ — Arquivos com dependência em Go tests

**Antes de mover/deletar qualquer arquivo em `docs/`, verificar se Go tests referenciam o path:**

```bash
grep -r "docs/STATUS\|docs/guides\|docs/research\|docs/architecture\|docs/compliance" pkg/
```

Estruturas que NÃO podem ser deletadas sem atualizar os testes correspondentes:

| Path | Testado em |
|------|-----------|
| `docs/STATUS.yml` | `pkg/doccheck/doccheck_test.go` — valida paths, stats, skills count |
| `docs/guides/` | `pkg/doccheck/doccheck_test.go` — espera ≥5 arquivos, breadcrumb, cross-refs |

Ao mover arquivos referenciados em `STATUS.yml`: atualizar os paths no arquivo antes de commitar.

## GitHub Pages — HTML Docs

HTML docs live under `docs/` organized by semantic subfolder. **Never add a new HTML file to `docs/` root.**

| Subfolder | Content |
|-----------|---------|
| `docs/platform/` | Architecture, guardrails, analysis, reports |
| `docs/postmortem/` | Postmortem HTML docs |
| `docs/incident/` | Incident dashboards |
| `docs/discovery/` | Discovery walkthroughs, cost analysis |

**Adding a new HTML doc:**
1. Drop the `.html` file in the appropriate subfolder
2. Add one entry to that subfolder's `index.json`
3. Never touch `docs/index.html` — it loads cards dynamically via `fetch(subfolder/index.json)`

**`index.json` entry schema:**
```json
{
  "title": "...",
  "href": "filename.html",
  "category": "case-study|discovery|analysis|platform|report|architecture",
  "badge": { "label": "...", "color": "blue|green|orange|purple|yellow|cyan" },
  "desc": "...",
  "tags": ["tag1", "tag2"],
  "order": 1
}
```

To add a new subfolder: create the folder + `index.json`, then add the path to the `INDEXES` array in `docs/index.html`.

## Fusca → Snowflake Silver (current state)

Two Airbyte connections, both 1h schedule, both active:
- Keychain `workflow-airbyte-conn-fusca-cdc` — Fusca [cdc], avg 8.6 min
- Keychain `workflow-airbyte-conn-fusca-actlog` — Fusca [activity_log], avg 25.9 min ⚠ monitor: >50 min = overlap risk
