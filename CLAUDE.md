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
| Calibrações e premissas | `wtb doc search premissas` → `wtb doc get <id>` |

**Credenciais → Keychain:**
```bash
# Ler: security find-generic-password -s "<service>" -a $(whoami) -w
# Gravar: security add-generic-password -s "<service>" -a $(whoami) -w "<valor>"
```
Registrar serviços conforme MCPs utilizados (ex: `workflow-dd-api-key`, `workflow-slack-token`, `workflow-github-token`).

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
Todo code review — seja iniciado pelo usuário, por `pr-ready`, por `ci-fix` agent ou qualquer outro fluxo — deve usar o skill de review configurado para o projeto. Nunca chamar `coderabbit:code-review` diretamente sem antes verificar se há um skill de review customizado em `.claude/skills/`.

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
- Criar branch antes do commit: `feat/<context>` ou `feat/TICKET-XXXX-<description>`
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
Branch com ticket ID (`feat/TICKET-XXXX-*`) → Jira/Linear GitHub App linka automaticamente.
Template: seções `## O que muda`, `## Por que`, `## Rastreabilidade` (com link para o ticket), `## Como testar`.
Workflow docs: adicionar `tickets: [TICKET-XXXX]` no frontmatter quando o doc fecha um ticket.

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
2. Carregar premissas: `wtb doc search premissas` → `wtb doc get <id>`
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
wtb status                               # status
wtb run ops-response                     # workflow ops completo
wtb ops db-health --namespace org        # probe individual
wtb mcp-serve                            # MCP server para Claude Code
wtb new incident <context> --repo <path> # scaffolding
wtb cycle-check --save --repo <path>     # savepoint
bash ~/workflow/scripts/memory-observer.sh --repo <path> [--hours N]  # Session Exit: propõe wtb memory set
bash ~/workflow/scripts/db-backup.sh                                  # Session Exit: health check SQLite + backup .db
wtb memory get <topic>                   # carrega arquivo de tópico
wtb memory where "<descrição>"           # roteia onde armazenar um fato
wtb memory set <key> <val> --type --topic --desc  # armazena fato estruturado
wtb memory list [--topic <t>] [--stale N]         # lista entradas de memória no docs.db
wtb memory validate                      # guardrails bloat + content-leak
wtb backlog list [--status S] [--repo R] [--tag T] [--jira J]  # tarefas ativas
wtb backlog search <keyword>             # busca full-text no DB
wtb backlog add --title "..." [flags]    # nova tarefa
wtb backlog done/block/start <id>        # transição de status
wtb doc list [--type T] [--since D] [--repo R] [--tag T]  # artefatos (discovery, savepoint, runbook, 1on1)
wtb doc search <keyword>                 # busca full-text
wtb doc get <id>                         # conteúdo completo
wtb doc import <dir> --type T [-r]       # importar diretório
wtb doc delete <id>                      # soft-delete
```

**Regra de uso do `wtb memory`:**
- Ao aprender novo fato operacional: `wtb memory where "<descrição>"` → seguir recomendação
- Para carregar contexto de tópico: `wtb memory get <topic>` (alias do topic-map.yml)
- Credenciais e IDs → sempre Keychain, nunca em arquivo commitado
- **Antes de qualquer ação com threshold/limite:** `wtb memory list --topic <topic>` — nunca usar valor literal de skill ou heurística sem verificar docs.db primeiro
- Skills e heurísticas referenciam chaves por nome — o valor canônico está no docs.db

Referência técnica: `wtb doc get platform-reference-workflow-toolkit`
Pendências e estado: `wtb backlog list`

---

**Versao:** 4.0 | **Ultima atualizacao:** 2026-03-15
