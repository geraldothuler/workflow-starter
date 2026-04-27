> 📍 [README](../../README.md) > Guides > wtb como MCP Global

# wtb como MCP Global

O `wtb serve` expõe 40 tools MCP via HTTP em `localhost:7654/mcp`. Configurado como daemon launchd + entrada global no `~/.claude/settings.json`, o `wtb` fica disponível em **qualquer sessão Claude Code**, de qualquer diretório.

## Setup (uma vez)

```bash
# Compilar wtb
go build -o ~/bin/wtb ./cmd/wtb

# Registrar daemon launchd + MCP no settings.json
./scripts/wtb-daemon-setup.sh
```

O script:
1. Cria `~/Library/LaunchAgents/com.workflow.wtb-daemon.plist` — inicia `wtb serve` no login e mantém vivo
2. Adiciona `"workflow": { "url": "http://localhost:7654/mcp" }` em `~/.claude/settings.json`
3. Carrega o daemon imediatamente
4. Testa se o endpoint responde

Após rodar, reinicie o Claude Code para carregar o MCP na sessão.

## Verificação

```bash
# Daemon rodando?
launchctl list | grep wtb

# MCP respondendo?
curl -s http://localhost:7654/mcp

# Logs do daemon
tail -f ~/.wtb/daemon.log
```

## Tools disponíveis

### Documentos (`doc_*`)

| Tool | O que faz |
|------|-----------|
| `doc_get` | Retorna conteúdo completo de um documento por ID |
| `doc_list` | Lista documentos com filtros (type, repo, since, tag) |
| `doc_search` | Busca full-text em títulos e conteúdo |
| `doc_add` | Cria novo documento (aplica template automaticamente) |

### Memória (`memory_*`)

| Tool | O que faz |
|------|-----------|
| `memory_get` | Retorna entrada por chave ou lista por tópico |
| `memory_list` | Lista todas as entradas, filtra por tópico ou staleness |
| `memory_set` | Armazena ou atualiza entrada no docs.db |

### Repoindex (`repo_*`)

| Tool | O que faz |
|------|-----------|
| `repo_show` | Snapshot completo de um repo (handlers, models, eventos, APIs) |
| `repo_list` | Lista todos os repos indexados |
| `repo_status` | Classifica repos OK/STALE, sugere re-indexação |
| `repo_query` | SQL SELECT direto no repos.duckdb |
| `repo_topology` | Grafo Kafka: quem produz → tópico → quem consome |
| `repo_impact` | Tópicos Kafka compartilhados entre repos |
| `repo_grep` | Busca keyword em handlers, models, eventos, APIs, config vars |
| `repo_similar` | Repos que compartilham tópicos Kafka com um dado repo |

### Workflows e Ops

| Tool | O que faz |
|------|-----------|
| `workflow_run` | Executa pipeline de use-case (ops-response, investigation, backlog) |
| `workflow_list_use_cases` | Lista use-cases disponíveis |
| `workflow_new` | Cria artefato de workflow (incident, postmortem, review, 1on1) |
| `workflow_status` | Status da plataforma |
| `ops_probe` | Roda probe ops individual (kubectl, psql, Kafka) |
| `ops_db_health` | Health check de banco de dados |
| `ops_k8s_status` | Status de deployments Kubernetes |
| `ops_kafka_status` | Status de tópicos Kafka |
| `ops_logs_analyze` | Análise de logs |
| `playbook_run` | Executa playbook ops |
| `playbook_list` | Lista playbooks disponíveis |

## Opções avançadas

```bash
# Porta customizada
./scripts/wtb-daemon-setup.sh --port 7655

# Binário em path não-padrão
./scripts/wtb-daemon-setup.sh --wtb-bin /usr/local/bin/wtb

# Repo root diferente
./scripts/wtb-daemon-setup.sh --repo-root /caminho/para/workflow

# Desinstalar daemon
./scripts/wtb-daemon-setup.sh --uninstall
```

## Quando o daemon cai

O launchd reinicia automaticamente. Se o problema persistir:

```bash
tail -f ~/.wtb/daemon.log       # ver erro
launchctl list | grep wtb       # verificar status (exit code na segunda coluna)
launchctl unload ~/Library/LaunchAgents/com.workflow.wtb-daemon.plist
launchctl load  ~/Library/LaunchAgents/com.workflow.wtb-daemon.plist
```

Se o binário precisar ser recompilado após mudança de código:
```bash
go build -o ~/bin/wtb ./cmd/wtb
launchctl kickstart -k gui/$(id -u)/com.workflow.wtb-daemon
```
