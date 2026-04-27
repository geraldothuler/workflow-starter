# workflow-starter

Metodologia e ferramentas para engenharia aumentada com IA — template para devs que usam Claude Code.

Contém o `wtb` CLI (Go), skills para Claude Code, e uma base de conhecimento pré-carregada com 50 repos indexados e 11 templates de artefatos.

---

## O que é

Este repo é um ponto de partida para montar um ambiente de trabalho estruturado com Claude Code. A ideia central: Claude não trabalha "solto" — ele opera dentro de contratos, regras de cadeia e uma base de conhecimento persistente que cresce com o uso.

O `CLAUDE.md` na raiz é o entry point. Toda sessão com Claude Code começa lendo esse arquivo.

---

## O que está incluído

| Item | O que é |
|------|---------|
| `CLAUDE.md` | Contratos, princípios e chain rules — lido pelo Claude em toda sessão |
| `cmd/wtb/` + `pkg/` | Código-fonte do CLI `wtb` em Go |
| `docs.db` | SQLite com 11 templates de artefatos (discovery, savepoint, postmortem, etc.) |
| `repos.duckdb` | DuckDB com 50 repos indexados (handlers, modelos, eventos, APIs, configuração) |
| `backlog.db` | SQLite vazio — scaffold para tarefas |
| `.claude/skills/` | 21 skills para Claude Code (pdf, playwright, grill-me, daily, tdd, ...) |
| `.claude/memory/` | Arquivos de feedback e metodologia — carregados sob demanda |
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

## Estrutura de arquivos

```
workflow/
├── CLAUDE.md               # entry point — contratos e chain rules
├── cmd/wtb/                # CLI source
├── pkg/                    # pacotes Go
├── docs.db                 # artefatos (discovery, savepoint, templates...)
├── repos.duckdb            # repoindex (handlers, modelos, eventos, APIs)
├── backlog.db              # tarefas
├── .claude/
│   ├── skills/             # skills Claude Code
│   └── memory/             # feedback e metodologia
├── docs/workflow/platform/ # referência técnica da plataforma
└── scripts/                # utilitários de manutenção
```

---

## Licença

MIT
