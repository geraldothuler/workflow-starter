> 📍 [README](../../../README.md) > Platform Reference

# Platform Reference — workflow-toolkit

**Runtime:** Go 1.24 | **CLI entry point:** `cmd/wtb/` | **LLM providers:** Claude, ChatGPT, Gemini, Ollama, Azure

---

## CLI Subcommands (`cmd/wtb/`)

| File | Command | Description |
|------|---------|-------------|
| `cycle.go` | `wtb cycle-check` | Cycle-end detection and savepoint generation |
| `ops.go` | `wtb ops` | Ops Toolbox — 14 infrastructure probes |
| `doc_cmd.go` | `wtb doc` | Workflow artefact store (discovery, savepoint, runbook, 1on1) |
| `backlog_tasks.go` | `wtb backlog` | Operational task backlog (SQLite) |
| `memory.go` | `wtb memory` | Memory management (get, where, set, list, validate) |
| `monitor.go` | `wtb monitor` | Monitor management (Datadog) |
| `guardrail.go` | `wtb guardrail` | Drift containment and pre-commit hooks |
| `store.go` | `wtb store` | Ops result log |
| `testenv.go` | `wtb testenv` | Integration test environment orchestration |
| `docs.go` | `wtb docs` | Docs chain generation |
| `scaffold.go` → `new` | `wtb new` | Workflow scaffolding |
| `serve_cmd.go` | `wtb serve` | wtb daemon (HTTP over Unix socket — eliminates DuckDB lock contention) |
| `main.go` → `chain` | `wtb chain next` | Chain traversal for documentary use-cases (incident, postmortem, discovery) |
| `webhook_cmd.go` | `wtb webhook status/setup` | Webhook readiness check and guided setup (ngrok, Keychain, GitHub, Datadog) |

---

## Package Inventory (`pkg/`)

```
pkg/
├── audit/             — Audit trail and event log
├── chain/             — Chain traversal (recursive use-case chaining via chain.to)
├── auth/              — Authentication and credential resolution
├── backlog/           — Backlog management (SQLite, tagging, search)
├── compliance/        — LGPD consent, PII detection, security gates
├── context/           — Context loader (CLAUDE.md, skills, patterns)
├── credentials/       — Credential provider (Keychain, env, session)
├── critical_path/     — Critical path analysis
├── cycles/            — Cycle-end detection, savepoint rendering
├── dbops/             — Database query runner (PostgreSQL, ScyllaDB, Snowflake)
├── doccheck/          — Documentation drift prevention (CI guardrails)
├── docs/              — Docs utilities
├── docstore/          — Workflow artefact store (SQLite, full-text search)
├── export/            — Export pipeline
├── exporter/          — Exporter implementations
├── extractor/         — Backlog and transcript extractor
├── feasibility/       — Feasibility scoring
├── features/          — FEATURES.yml registry loader
├── i18n/              — Internationalization utilities
├── infracontext/      — Infrastructure context (k8s, Kafka, DB probes)
├── journey/           — User journey tracking
├── llm/               — LLM client (Claude, ChatGPT, Gemini, Ollama, Azure)
├── logging/           — Structured logging
├── mcp/               — MCP server (25 tools)
├── memory/            — Memory management (context.json, topic files)
├── monitor/           — Monitor management (Datadog threshold calibration)
├── ops/               — Ops Toolbox (14 deterministic probes)
├── parser/            — Parser utilities (YAML, Markdown, proto)
├── patterns/          — Architecture patterns registry
├── patterns_catalog/  — Pattern catalog and discovery
├── playbook/          — Playbook runner (investigation, ops-response)
├── privacy/           — Privacy utilities (anonymization, LGPD)
├── render/            — Backlog and artefact rendering (HTML, Markdown)
├── repoindex/         — Repository indexing and code topology analysis
├── runner/            — Pipeline runner (use-case orchestration)
├── scaffold/          — Workflow scaffolding (new incident, discovery, etc.)
├── security/          — Security scanning and compliance
├── server/            — MCP server transport
├── sources/           — Data source plugins
├── spec/              — Specification utilities
├── store/             — Ops result log store
├── sync/              — Sync utilities
├── taskstore/         — Task store (SQLite)
├── webhook/           — Webhook ingestion (GitHub HMAC, Datadog token, event routing, headless dispatch)
├── wtbserver/         — wtb daemon server, job registry, and HTTP client (Unix socket)
├── techref/           — Technical reference loader
├── testenv/           — Integration test environment orchestration
├── transport/         — HTTP/gRPC transport utilities
├── types/             — Shared type definitions
├── ui/                — UI utilities (terminal rendering)
├── validation/        — Input validation
└── validation/examples/ — Validation usage examples
```

---

## LLM Providers (`pkg/llm/`)

| Provider | Constant | Notes |
|----------|----------|-------|
| Claude | `ProviderClaude` | Default — Anthropic API |
| ChatGPT | `ProviderChatGPT` | OpenAI API |
| Gemini | `ProviderGemini` | Google AI API |
| Ollama | `ProviderOllama` | Local inference |
| Azure | `ProviderAzure` | Azure OpenAI |
