# workflow-starter

Metodologia e ferramentas para engenharia aumentada com IA — template para devs que usam Claude Code.

Contém o `wtb` CLI (Go), skills para Claude Code, e uma base de conhecimento pré-carregada com repos indexados e templates de artefatos.

---

## O que é

Este repo é um ponto de partida para montar um ambiente de trabalho estruturado com Claude Code. A ideia central: Claude não trabalha "solto" — ele opera dentro de contratos, regras de cadeia e uma base de conhecimento persistente que cresce com o uso.

O `CLAUDE.md` na raiz é o entry point. Toda sessão com Claude Code começa lendo esse arquivo.

---

## O que está incluído

| Item | O que é |
|------|---------|
| `CLAUDE.md` | Contratos, princípios e chain rules — lido pelo Claude em toda sessão |
| `_core/metacognition.md` | 8 primitivas cognitivas do modelo de pensamento |
| `cmd/wtb/` + `pkg/` | Código-fonte do CLI `wtb` em Go |
| `docs.db` | SQLite com templates de artefatos (discovery, savepoint, postmortem, etc.) e memória operacional |
| `repos.duckdb` | DuckDB com repos indexados (handlers, modelos, eventos, APIs, configuração) |
| `backlog.db` | SQLite vazio — scaffold para tarefas |
| `.claude/skills/` | Skills para Claude Code (pdf, playwright, grill-me, daily, tdd, ...) |
| `.claude/memory/` | Arquivos de feedback e metodologia — carregados sob demanda |
| `docs/guides/` | Guias de uso: getting-started, cycle-close, ops-response, yaml-driven-design, memory-system |
| `docs/workflow/platform/` | Referência técnica da plataforma |
| `scripts/` | `db-backup.sh`, `memory-observer.sh`, `export-to-starter.sh` |

---

## Quick start

### 1. Pré-requisitos

- Go 1.22+
- SQLite3 (`brew install sqlite3`)
- Claude Code CLI

### 2. Clonar e compilar o `wtb`

```bash
git clone git@github.com:geraldothuler/workflow-starter ~/workflow
cd ~/workflow
go build -o ~/bin/wtb ./cmd/wtb
```

Adicione `~/bin` ao `$PATH` se necessário.

### 3. Configurar o repo root

```bash
# Em ~/.zshrc ou ~/.bashrc:
export WTB_REPO_ROOT="$HOME/workflow"
```

### 4. Verificar

```bash
wtb status
wtb repo status          # mostra os 50 repos e staleness do índice
wtb doc list --type template   # lista os 11 templates disponíveis
```

### 5. Iniciar Claude Code neste diretório

```bash
cd ~/workflow
claude
```

O Claude lerá o `CLAUDE.md` automaticamente e terá acesso a todos os comandos `wtb`.

---

## Principais comandos

### Artefatos (`wtb doc`)

```bash
wtb doc add --type discovery --title "Investigação X" --date 2026-04-26
# → abre com template de discovery pré-preenchido (timeline + lições aprendidas)

wtb doc list --type savepoint --since 2026-04-01
wtb doc search "kafka checkpoint"
wtb doc get <id>
wtb doc template get discovery      # ver template
wtb doc template set discovery --file meu-template.md  # customizar
```

Tipos disponíveis: `discovery`, `savepoint`, `postmortem`, `incident`, `review`, `1on1`, `poc`, `runbook`, `reference`, `draft`, `config`

### Repoindex (`wtb repo`)

```bash
wtb repo status                  # saúde do índice — repos ok vs stale
wtb repo status --stale 14       # threshold customizado (dias)
wtb repo show fusca              # snapshot completo de um repo
wtb repo show fusca --table      # formato legível
wtb repo query "SELECT name, trigger_type FROM handlers WHERE repo_id=(...)" --table
wtb repo topology                # grafo Kafka: producer→topic→consumer
wtb repo impact fusca iris       # o que muda se esses repos forem unificados

# Adicionar um repo novo ao índice:
wtb repo index meu-repo --path ~/projects/meu-repo
```

### Memória (`wtb memory`)

```bash
wtb memory set checkpoint_interval_ms 5000 --type config --topic kafka --desc "Safe default"
wtb memory get heuristics        # carrega arquivo de tópico
wtb memory list --topic kafka
wtb memory list --stale 60       # entradas não verificadas em 60+ dias
```

### Backlog (`wtb backlog`)

```bash
wtb backlog add --title "Investigar latência p99 em X" --repo fusca
wtb backlog list
wtb backlog done <id>
```

### Ciclo de sessão (`wtb cycle-check`)

```bash
wtb cycle-check --repo ~/workflow          # verifica estado da sessão
wtb cycle-check --save --repo ~/workflow   # grava savepoint técnico no docs.db
```

---

## Adicionando seus próprios repos ao índice

O `repos.duckdb` vem com 50 repos pré-indexados. Para adicionar os seus:

```bash
# Requer ANTHROPIC_API_KEY ou Claude Code CLI autenticado
wtb repo index meu-repo --path ~/projects/meu-repo

# Verificar resultado:
wtb repo show meu-repo --table
wtb repo status  # aparecerá na lista
```

O indexador extrai automaticamente handlers, modelos, eventos, APIs externas e variáveis de configuração usando LLM.

---

## Customizando

### Tipos de documento

Crie `doc-types.yml` na raiz do repo para adicionar ou substituir os tipos padrão:

```yaml
types:
  - discovery
  - savepoint
  - postmortem
  - rfc        # tipo customizado
  - adr        # architecture decision record
```

### Templates

```bash
wtb doc template set rfc --file templates/rfc.md
```

A partir daí, `wtb doc add --type rfc --title "..."` aplica o template automaticamente.

### Threshold de staleness do índice

```bash
wtb memory set repoindex_stale_days 14 --type config --topic repoindex --desc "Re-indexar a cada 2 semanas"
```

---

## MCPs — multiplique o potencial do Claude

MCPs (Model Context Protocol servers) são o maior diferencial deste stack. Sem eles, o Claude usa subprocess e parse de texto. Com eles, opera diretamente sobre dados estruturados — PR, CI, Jira, logs, Slack — sem intermediários.

**Recomendação forte:** configure os MCPs relevantes antes de começar a usar no dia a dia.

```bash
/mcp-setup    # wizard interativo: detecta o que está faltando e guia a configuração
```

| MCP | Para que serve |
|-----|---------------|
| **GitHub** | PR review, CI checks, issues, deployments |
| **Atlassian** | Jira, Confluence — criar, buscar, transicionar |
| **Datadog** | Logs, métricas APM, traces, RCA |
| **Slack** | Ler canais, buscar contexto, rascunhar mensagens |
| **Notion** | Criar e atualizar documentação |
| **Figma** | Leitura de designs, geração de código |
| **Browser** | Automação, scraping, testes E2E |

Referência completa: [`docs/guides/mcps.md`](docs/guides/mcps.md)

---

## Guias de uso

Os guias em `docs/guides/` cobrem os fluxos principais:

| Guia | O que explica |
|------|--------------|
| `getting-started.md` | Primeira sessão, ativação, configuração inicial |
| `mcps.md` | MCPs recomendados, por que usar, como configurar cada um |
| `cycle-close.md` | Como encerrar uma sessão: CI, savepoint, memória, backup |
| `ops-response.md` | Diagnóstico de incidente com 6 checks obrigatórios |
| `yaml-driven-design.md` | Princípio YAML-first: quando Go, quando YAML |
| `memory-system.md` | Sistema de memória: `wtb memory`, `docs.db`, topic files |

---

## Estrutura de arquivos

```
workflow/
├── CLAUDE.md               # entry point — contratos e chain rules
├── _core/
│   └── metacognition.md    # 8 primitivas cognitivas
├── cmd/wtb/                # CLI source
├── pkg/                    # pacotes Go
├── docs.db                 # artefatos + memória operacional
├── repos.duckdb            # repoindex (handlers, modelos, eventos, APIs)
├── backlog.db              # tarefas
├── .claude/
│   ├── skills/             # skills Claude Code
│   └── memory/             # feedback e metodologia
├── docs/
│   ├── guides/             # getting-started, cycle-close, ops-response, yaml-driven-design, memory-system
│   └── workflow/platform/  # referência técnica da plataforma
└── scripts/                # utilitários de manutenção
```

---

## Licença

MIT
