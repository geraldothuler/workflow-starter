# CLAUDE.md — Workflow Platform Orchestrator

**Proposito:** Contratos, princípios e chain rules da plataforma. Entry point via `wtb` CLI.
**Versao:** 4.0 | **Ultima atualizacao:** 2026-03-15

**Referências rápidas:**
- Referência técnica: `wtb doc get platform-reference-workflow-toolkit`
- Estado atual e pendências: `wtb backlog list`
- Metacognição: `_core/metacognition.md`

---

## 1. CORE THINKING MODEL

8 primitivas composíveis — detalhes em `_core/metacognition.md`:

| # | Primitiva | Camada | Uso principal |
|---|-----------|--------|---------------|
| 1 | Socratic Probe | Decisao | Input resolution: "Por que este input é necessário?" |
| 2 | Options Scoring | Decisao | ROI scoring em reviews, plan template selection |
| 3 | Environment Probe | Evidencia | 14 probes ops: kubectl, psql, gh, Kafka, etc |
| 4 | Heuristic-First | Evidencia | 42 regras deterministicas, zero LLM |
| 5 | Compliance Scoring | Formalizacao | LGPD consent, PII detection, security gates |
| 6 | Configuration-as-Code | Formalizacao | 17 YAML configs embedded, definition.yml por use-case |
| 7 | Progressive Enforcement | Enforcement | Human checkpoints, auth-blocking chain |
| 8 | ADR Capture | Registro | Savepoints, postmortems, chain de artefatos |

---

## 2. PRINCÍPIOS CENTRAIS

### YAML-Driven Design — Princípio Mais Importante

Qualquer lógica que pode variar por contexto, usuário ou projeto **DEVE viver em YAML**, não hardcoded em Go.

| Criterio | Go | YAML + plan template |
|----------|----|---------------------|
| Heurística complexa, estado ou desempenho | sim | nao |
| Composição de shell commands | nao | sim |
| Poll loop ou branching condicional simples | nao | sim |
| Parsing de formatos binários ou JSON complexo | sim | nao |

**YAML-first gate — obrigatório antes de criar qualquer nova função Go:**

1. Pode ser um shell command composto (curl, kubectl, jq, grep)? → **YAML**
2. O output precisa ser `OpsResult` estruturado (MCP, scoring, CLI integrado)? → Go
3. Envolve parsing JSON aninhado, estado entre chamadas ou lógica condicional complexa? → Go
4. Se respondeu "sim" apenas ao 1: YAML. Se respondeu "sim" ao 2 ou 3: Go.

**No-hardcode rule — obrigatório em todo plan template YAML:**

- Vars de ambiente (profile AWS, contexto kubectl, namespace, URLs) → sempre `""` — nunca hardcodar
- Resolução em runtime: `session.yml` → env var → prompt (Dependency Chain Rule)
- Exceção permitida: defaults genéricos sem vínculo de ambiente (ex: `page_size: "5"`, `risk: low`)

### Backward Compatibility Contract

| Contrato | Regra |
|----------|-------|
| Funções Go exportadas | Nunca alterar assinatura — adicionar nova, deprecar antiga |
| Structs | Apenas adicionar campos (`omitempty`) — nunca remover |
| YAML configs | Campos novos sempre opcionais com default no code |
| CLI flags/subcomandos | Apenas adicionar — aliases para renomeações |

---

## 3. WORKFLOW CATALOG

**Organizacionais** (artefatos documentais, cadeia humana):

| Tipo | Trigger | Artefato | Handoff |
|------|---------|----------|---------|
| **incident** | Anomalia em producao | savepoint-YYYY-MM-DD.md | → ops-response, postmortem |
| **postmortem** | Incidente encerrado | NNN-context-YYYY-MM-DD.md | → review |
| **review** | Postmortem concluido | NNN-context-YYYY-MM-DD.md | → 1on1 |
| **1on1** | Revisao de colaboracao | NNN-YYYY-MM-DD.md | → premises.md |
| **discovery** | Pergunta tecnica em aberto | NNN-topic-YYYY-MM-DD.md | → qualquer / terminal |
| **poc** | Hipotese definida, validacao pendente | NNN-context-YYYY-MM-DD.md | → review (✓), postmortem (✗), discovery (∂) |

**Técnicos** (pipelines com engines Go):

| Tipo | Trigger | Engine principal | Handoff |
|------|---------|-----------------|---------|
| **backlog** | Narrativa/transcricao disponivel | pkg/backlog + extractor + techref | → review |
| **investigation** | Causa raiz nao clara | pkg/playbook + infracontext | → postmortem |
| **ops-response** | On-call escalado / degradacao | pkg/ops (14 probes) + runner | → investigation |
| **ci-watch** | CI failure em PR | shell + ci-fix-ktlint plan | → merge |
| **code-review** | PR aberto aguardando review | llm + gh CLI | → merge |

Definições completas: `use-cases/<tipo>/definition.yml`

---

## 4. GOVERNANÇA

**O que entra no CLAUDE.md:** contratos, princípios e chain rules estáveis.
**O que NÃO entra:** pending tasks, histórico de implementação, estado de sessão.

| Escopo | Acesso |
|--------|--------|
| Contratos e regras | `CLAUDE.md` (este arquivo) |
| Referência técnica | `wtb doc get platform-reference-workflow-toolkit` |
| Estado e pendências | `wtb backlog list` |
| Discoveries, savepoints, runbooks | `wtb doc list/search/get` |

### Regra de artefatos — tudo no repo

Todo artefato vai para `~/workflow/` (este repo). A única exceção são **valores de credenciais**, que ficam no macOS Keychain — nunca em arquivos commitados.

| Artefato | Destino |
|----------|---------|
| Backlog e tarefas | `backlog.db` → `wtb backlog` |
| Discoveries, savepoints, runbooks, 1on1, postmortem, incident, review | `docs.db` → `wtb doc` |
| Session state | `session.yml` (gitignored) |
| Calibrações e premissas | `wtb doc get premissas-da-colabora-o-geraldo-claude-code` |

**Credenciais → secret store (macOS Keychain / GNOME Keyring / pass / env):**
```bash
# Ler (qualquer OS):   bash ~/workflow/scripts/secret-get.sh <service>
# Gravar (qualquer OS): bash ~/workflow/scripts/secret-set.sh <service> <valor>
# macOS legado direto: security find-generic-password -s "<service>" -a "$USER" -w
# Env var fallback:    WORKFLOW_SECRET_<SERVICE_UPPER>=<valor>
```
Serviços registrados: `workflow-dd-api-key`, `workflow-dd-app-key`, `workflow-slack-token`.

---

## 5. CHAIN RULES

**Cadeia operacional:**
```
incident → ops-response → investigation → postmortem → review → 1on1 → premises.md
backlog → review
```
Handoffs não são obrigatórios — incidente simples não precisa de investigation formal.
Artefatos: `docs.db` → `wtb doc add --type <tipo>`

### Code Review Rule
Todo code review em repos Cobli — seja iniciado pelo usuário, por `pr-ready`, por `ci-fix` agent ou qualquer outro fluxo — **DEVE usar `/cobli-review`**, nunca `coderabbit:code-review` diretamente.
- `/cobli-review` → review padrão (best-practices + sub-skills + CodeRabbit)
- `/cobli-review --deep` → PRs de feature grande ou mudança de contrato (adiciona 3 agentes paralelos)
- `/cobli-review --no-coderabbit` → quando CodeRabbit já foi rodado no ciclo

### Grill-Me Rule
Antes de iniciar qualquer plano de implementação não trivial: **invocar `/grill-me`** para validar decisões de design antes de escrever código. Não esperar o usuário pedir explicitamente.

### Savepoint Rule
Ao concluir ciclo estável: `wtb cycle-check --save --repo <path>`. Marcar tarefas concluídas: `wtb backlog done <id>`.

### Schema-Registry Rule

| Situação | Ação obrigatória |
|----------|-----------------|
| Mudança de proto necessária | PR no `schema-registry` origin — nunca editar submodulo diretamente |
| Submodulo com mudanças locais | PROIBIDO — reverter imediatamente |
| Adição de campo | Sempre `optional` — nunca quebrar contrato |
| Remoção ou renomeação | Proibido sem versionamento explícito e aprovação |

### Ops-Diagnostic Rule
Antes de declarar qualquer serviço saudável, verificar **obrigatoriamente** os 6 checks abaixo — nunca apenas pods Running:

1. Pods: status, age, restart count
2. HPA: `kubectl get hpa -n <ns> --context <ctx>` — replicas atuais vs min/max, métricas de scaling
3. CPU/memória: `kubectl top pods` vs limites do deployment
4. Latência p99 e error rate: Datadog APM ou `wtb ops db-health` — nunca assumir saudável sem verificar
5. Deploy recente: `kubectl rollout status` + eventos do namespace
6. Logs de erro crítico: OOM, CrashLoop, exceções nos últimos 15min

**Proibido:** reportar "parece saudável" ou "looks healthy" sem completar todos os 6 checks. Análise rasa causa incidentes não detectados (ex: cerberus CPU throttling declarado saudável sem verificar latência).

### Ops-Probes Rule
exit 3 de probe script → carregar skill `ops-probes` antes de investigar.

### Push Rule
Após `git push`: `gh run list --limit 1` → `gh run watch <id>`. Só encerrar após CI verde.

### CI-Gate Rule
Após todo `gh pr create` ou `git push` em PR aberto: **disparar Agent em background** — aguarda CI verde, roda CodeRabbit e corrige findings autonomamente.

```
Agent(
  description: "CI fix + CodeRabbit PR #N",
  prompt: "PR #<N> <branch> — verifica CI via gh pr checks, roda CodeRabbit review, corrige findings críticos, faz push. Repete até CI verde e sem findings bloqueantes.",
  run_in_background: true
)
```

**Status one-shot** (sem loop): `gh pr checks <PR#>` — nunca usar sleep loops.

**Default após disparar o Agent: prosseguir imediatamente.** Só bloquear se o próximo passo explicitamente requer CI verde daquele PR.

| Situação | Ação obrigatória |
|----------|-----------------|
| Após `gh pr create` / `git push` em PR aberto | Disparar Agent background → **continuar sem aguardar** |
| Próximo passo **depende** do CI deste PR | Declarar a dependência; aguardar Agent antes desse passo específico |
| Próximos passos são **independentes** | Prosseguir em paralelo — declarar explicitamente ao usuário |
| CI falhou (Agent reporta) | Diagnosticar causa raiz, corrigir, novo commit — nunca `--no-verify` |
| PR mergeado mas deploy pendente | Confirmar com usuário se próximo passo requer o deploy ou só o código |

### PR-per-Commit Rule
Cada commit com testes passando e funcionalidade verificada → **PR imediato** — não acumular commits locais.
- Criar branch antes do commit: `feat/<context>` ou `feat/SS-XXXX-<description>`
- Push + `gh pr create` logo após o commit
- Seguir Push Rule após o merge (CI verde obrigatório)
- Commits de savepoint e docs podem agrupar em um único PR se forem da mesma sessão

### Pre-PR Branch Rule
Antes de criar qualquer branch em repo externo: `git fetch origin` → `git checkout -b <branch> origin/<main-branch>`.
Confirmar: `git log origin/master..HEAD` sem commits extras antes do `gh pr create`.
Card Jira → **Done apenas quando PR mergeado na branch principal com CI verde**.

### Dependency Chain Rule
Precedência: `definition.yml` > `session.yml` > Push Rule > Socratic ask.
- Missão crítica (prod, incidente ativo) → **perguntar explicitamente** antes de merge
- Dev regular → risco aceitável → prosseguir (Push Rule aplica)

### PR Traceability Rule
Branch com ticket ID (`feat/ss-2273-*`) → Jira GitHub App linka automaticamente.
Template: seções `## O que muda`, `## Por que`, `## Rastreabilidade` (com link Jira `[SS-XXXX](https://cobliteam.atlassian.net/browse/SS-XXXX)`), `## Como testar`.
Workflow docs: adicionar `jira_tickets: [SS-XXXX]` no frontmatter quando o doc fecha um ticket.

### Session Exit Rule
Ao encerrar: CI verde? → `wtb cycle-check --repo <path>` → se `git_changes > 0`: propor savepoint + atualizar `backlog.md` → aguardar confirmação do usuário.

**No último savepoint do dia:** revisar a sessão e atualizar memory files com lições aprendidas — antes de criar o savepoint, não depois. Ordem:
0. `/pod-cleanup` — deletar pods efêmeros Completed (cqlsh-*, *-probe, kubectl-run-*, evicted), sinalizar suspeitos de outros times
1. `bash ~/workflow/scripts/memory-observer.sh --repo <path>` → Claude analisa e propõe `wtb memory set` commands → executar os aprovados
2. Novas heurísticas / padrões confirmados → topic files (`memory/*.md`)
3. Novos IDs de conexão, endpoints, credenciais → topic file relevante
4. Regras de processo novas ou corrigidas → `MEMORY.md`
5. Skills / workflow-conventions com novo comportamento validado → `SKILL.md` correspondente
6. Rodar `monitors/cost-report.sh` — registrar custo do dia no savepoint
7. `bash ~/workflow/scripts/db-backup.sh` — health check SQLite + backup de `backlog.db` e `docs.db` → se falhar, **não prosseguir**
8. `wtb cycle-check --save` — grava savepoint técnico (sinais, score) no `docs.db`
9. `wtb doc add --type savepoint --title "Savepoint YYYY-MM-DD — <contexto>" --date YYYY-MM-DD` — adiciona savepoint rico (o que foi feito, pendências, custo) no `docs.db` → consultável via `wtb doc get <id>`
   Não há arquivo `.md` a commitar — savepoints vivem exclusivamente no `docs.db`.

---

## 6. ARTEFATOS

Artefatos em `backlog.db` (tarefas) e `docs.db` (discovery, savepoint, runbook, 1on1, postmortem, incident, review, reference).
Convenção de IDs: `NNN-<context>-YYYY-MM-DD` (zero-padded). Regras de placement: `workflow-conventions/SKILL.md`.

**Ao iniciar qualquer workflow:**
0. `cd ~/workflow` — garantir CWD correto (ou confirmar `WTB_REPO_ROOT` carregado no shell)
1. Carregar `session.yml` como contexto ativo
2. Carregar premissas: `wtb doc get premissas-da-colabora-o-geraldo-claude-code`
3. Para 1on1: registrar nova sessão com `wtb doc add --type 1on1 --title "..." --date YYYY-MM-DD --file <path>`
4. **AI Radar:** verificar se já rodou hoje:
   ```bash
   cat ~/workflow/.ai-radar-last-run 2>/dev/null
   ```
   Se output != data de hoje (ou arquivo ausente) → executar skill `ai-radar`.
   Silencioso se nada relevante; apresentar apenas se houver achados acionáveis.
   Após rodar (com ou sem achados): `echo $(date +%Y-%m-%d) > ~/workflow/.ai-radar-last-run`
5. **Carregar valores calibrados:** `wtb memory list` — internalizar antes de qualquer ação com threshold ou limite. Nunca usar valor literal de skill/heurística sem este passo.
6. **Carregar backlog:** `wtb backlog list` — tarefas ativas no DB (pending/in-progress/blocked).
7. **Repoindex refresh:** atualizar métricas e monitores DD se stale (silencioso se já frescos):
   ```bash
   wtb repo dd-metrics --all --stale 20 2>/dev/null
   wtb repo dd-enrich  --all --stale 20 2>/dev/null
   ```
   Execução silenciosa — apresentar output só se houver repos novos ou erro.

---

## 7. ACTIVATION

> **CWD:** todos os comandos `wtb` requerem `WTB_REPO_ROOT` ou execução a partir de `~/workflow`.
> `WTB_REPO_ROOT` está em `~/.zshrc` — qualquer shell com o env carregado funciona de qualquer diretório.
> Fallback manual: `cd ~/workflow && wtb <cmd>`.

```bash
# ── Status e ciclo ──────────────────────────────────────────────────────────
wtb status                               # resumo de ~/.workflow/
wtb cycle-check --save --repo <path>     # savepoint de fim de ciclo
wtb guardrail                            # detecta drift de YAML/framework
bash ~/workflow/scripts/memory-observer.sh --repo <path> [--hours N]
bash ~/workflow/scripts/db-backup.sh

# ── Workflows e agentes ─────────────────────────────────────────────────────
wtb run <use-case> [--input k=v ...]     # executa pipeline use-case
wtb run ops-response --input symptom="CDC lag"
wtb agent list                           # lista use-cases type:agent
wtb agent spec <id> [--input k=v ...]   # exibe bloco spawn_agent renderizado
wtb chain next <use-case-id>             # segue chain.to de use-case documental
wtb new incident <context> --repo <path> # scaffolding de artefato
wtb docs generate <use-case-id>          # gera documentação de use-case

# ── Memória ─────────────────────────────────────────────────────────────────
wtb memory get <topic>                   # carrega arquivo de tópico
wtb memory where "<descrição>"           # roteia onde armazenar um fato
wtb memory set <key> <val> --type --topic --desc
wtb memory list [--topic <t>] [--stale N]
wtb memory validate                      # guardrails bloat + content-leak
wtb memory append --topic <t> "<texto>"  # narrativa em topic file

# ── Backlog ─────────────────────────────────────────────────────────────────
wtb backlog list [--status S] [--repo R] [--tag T] [--jira J]
wtb backlog search <keyword>
wtb backlog add --title "..." [flags]
wtb backlog done/block/start <id>

# ── Docs (discovery, savepoint, runbook, 1on1…) ─────────────────────────────
wtb doc list [--type T] [--since D] [--repo R] [--tag T]
wtb doc search <keyword>
wtb doc get <id>
wtb doc add --type <T> --title "..." --date YYYY-MM-DD [--file <path>]
wtb doc import <dir> --type T [-r]
wtb doc delete <id>
wtb doc template get <type>              # template para type (stored em docs.db)
wtb doc template set <type> --file <path>

# ── Repoindex ───────────────────────────────────────────────────────────────
wtb repo show <name>                     # snapshot completo (handlers, models, APIs, events)
wtb repo list                            # repos indexados
wtb repo status [--stale N]             # saúde dos índices — sinaliza defasados
wtb repo grep "<pattern>" [--repo <r>]  # busca regexp em código fonte
wtb repo impact <repo1> [repo2 ...]     # análise de impacto de merge
wtb repo topology                        # mapa producer→topic→consumer
wtb repo canvas [<name>]                 # visão de migração (5 eixos)
wtb repo query "<sql>"                   # SQL direto em repos.duckdb
wtb repo similar <name>                  # repos semanticamente similares
wtb repo index <name> --path <path>      # (re)indexa um repo
wtb repo dd-metrics --all --stale 20     # atualiza métricas DD
wtb repo dd-enrich  --all --stale 20     # atualiza monitors DD

# ── Ops Toolbox (zero-LLM) ─────────────────────────────────────────────────
wtb ops probe --namespace <ns>           # todos os probes: auth+DB+k8s+Kafka
wtb ops db-health --namespace <ns>       # health de banco (PostgreSQL/Scylla)
wtb ops k8s-status --namespace <ns> --deployment <d>
wtb ops kafka-status --namespace <ns>
wtb ops logs-analyze --file <path> --patterns kafka,oom
wtb ops github --repo owner/repo --scope pr|ci|issues|releases
wtb ops jira --url <url> --email <e> --token <t> --project <p>
wtb ops slack --token <tok> --channel <ch>
wtb ops snowflake --account <a> --user <u> --query "<sql>"
wtb ops airbyte --url <url> --workspace-id <ws>
wtb ops plan new --template <tpl> --scenario "<desc>"
wtb ops plan show / execute
wtb store trend <probe> [--last N]       # histórico de probes (ops-log.db)

# ── Infra do daemon e MCP ───────────────────────────────────────────────────
wtb serve [--mcp-port 7654] [--daemon]   # daemon: Unix socket + MCP HTTP
wtb mcp-serve                            # MCP stdio (para Claude Code settings.json)
wtb webhook status / setup               # gestão de webhook ingestion

# ── Monitoramento ───────────────────────────────────────────────────────────
wtb monitor slack                        # poll Slack para sinais P0/P1 (zero-LLM)
```

**Regra de uso do `wtb memory`:**
- Ao aprender novo fato operacional: `wtb memory where "<descrição>"` → seguir recomendação
- Para carregar contexto de tópico: `wtb memory get <topic>` (alias do topic-map.yml)
- Credenciais e IDs → sempre Keychain, nunca context.json
- **Antes de qualquer ação com threshold/limite:** `wtb memory list --topic <topic>` — nunca usar valor literal de skill ou heurística sem verificar context.json primeiro
- Skills e heurísticas referenciam chaves (`context.json: <key>`), não valores — o valor canônico está no context.json

Referência técnica: `wtb doc get platform-reference-workflow-toolkit`
Pendências e estado: `wtb backlog list`

---

**Versao:** 4.0 | **Ultima atualizacao:** 2026-03-15
