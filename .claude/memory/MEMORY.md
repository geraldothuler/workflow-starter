# Memory — workflow platform

## Credenciais

```bash
bash ~/workflow/scripts/secret-get.sh <service>   # ler (macOS/Linux/pass/env)
bash ~/workflow/scripts/secret-set.sh <service> <valor>  # gravar
```

## Como retomar sessão

1. `cd ~/workflow && wtb backlog list` — tarefas ativas (pending/in-progress/blocked)
2. `wtb doc search "<contexto>"` → `wtb doc list --type discovery` — artefatos anteriores

---

## Como usar o CLI de memória

Todo conhecimento operacional vive nos **topic files** — carregar sob demanda via `wtb memory get <topic>`.

- Fato novo → `wtb memory where "<descrição>"` → seguir recomendação
- Threshold/config → `wtb memory set <key> <val> --type <tipo> --topic <t>`
- Regra narrativa → `wtb memory append --topic <topic> "<conteúdo>"`
- MEMORY.md bloat → mover para topic file, nunca condensar inline

## Topic files — carregar sob demanda

| Tópico | Arquivo | Quando |
|--------|---------|--------|
| Autonomia: parar só em gates externos (PR, push, deploy, Slack) | `memory/feedback_autonomy_gates.md` | build/edit/query são autônomos |
| Consultar MEMORY.md + wtb doc search ANTES de "não sei" | `memory/feedback_check_memory_before_asking.md` | comando desconhecido |
| **PADRÃO UNIVERSAL:** 1-a-1 socraticamente (1 pergunta/vez → tradeoffs + score → recomendação) | `memory/feedback_socratic_standard.md` + `memory/socratic_example.md` | SEMPRE em múltiplas opções/decisões |
| grill-me obrigatório antes de todo plano não trivial | `memory/feedback_grill_me.md` | qualquer plano |
| Git worktrees — quando usar, tradeoffs, submódulos | `memory/git-worktrees.md` | trabalho paralelo em branches |
| Savepoint: sempre CLI (wtb doc add), nunca .md | `memory/feedback_savepoint_cli.md` | criar savepoint |
| Busca em 3 camadas: wtb doc search (2x) → git log → grep artifacts/ | `memory/feedback_search_protocol.md` | buscar artefato de sessão anterior |
| Rascunhos (Slack, email, Jira) → `wtb doc add --type draft` imediatamente | `memory/feedback_draft_persistence.md` | qualquer texto para envio externo |
| Slack: apresentar no chat — nunca enviar diretamente | `memory/feedback_slack_draft_clipboard.md` | qualquer mensagem Slack |
| **PROIBIDO** "pré-existente" — toda falha deve ser resolvida | `memory/feedback_no_preexisting_excuse.md` | qualquer falha de teste |
| Comentários de código sempre em inglês | `memory/feedback_code_comments_english.md` | qualquer edição de código |
| Comunicação externa (PR, Slack, Jira) sempre em pt-BR | `memory/feedback_communication_language.md` | qualquer texto para sistema externo |
| PDF — tabelas sem overflow: separadores proporcionais, backtick não quebra linha | `memory/feedback_pdf_tables.md` | gerar PDF com tabelas |
| PDF — SEMPRE invocar Skill /pdf, nunca pandoc direto | `memory/feedback_pdf_skill_mandatory.md` | qualquer geração de PDF |

## Skills disponíveis

| Skill | Quando invocar |
|-------|---------------|
| skill `grill-me` | Antes de qualquer plano não trivial — validar decisões de design |
| skill `savepoint` | Encerrar ciclo de trabalho, fim de sessão |
| skill `pr-ready` | Ciclo PR autônomo: rebase → CodeRabbit → CI |
| skill `tdd` | Desenvolvimento orientado a testes |
| skill `pdf` | Gerar PDF, repo-guide, onboarding |
| skill `video-frames` | Investigação visual de bugs via frames de vídeo |
| skill `agent-browser` | Scraping e automação web autônoma |
| skill `playwright` | Testes E2E e automação de browser |
| skill `chrome-devtools` | Debug de aplicações web (network, console, performance) |
| skill `ai-radar` | Monitorar canais de AI/ML no Slack |
| skill `estimate` | Estimativa calibrada de feature/epic |
| skill `repo-guide` | Gerar guia técnico completo de repositório |
| skill `repo-doc` | Gerar PDF de onboarding técnico para um squad |
| skill `onboarding-doc` | Documento de onboarding para novo desenvolvedor |
| skill `daily` | Gerar daily report (Dev + Stakeholder) |
| skill `postmortem` | Gerar postmortem de incidente |
| skill `rds-pg-eval` | Avaliação de performance PostgreSQL/RDS |
| skill `health-check` | Health check completo de serviços k8s |
| skill `ops-probes` | Probes operacionais de diagnóstico (k8s, Flink, Kafka) |
| skill `pod-cleanup` | Limpeza de pods efêmeros k8s antes de savepoint |
| skill `kafka-topics` | Criar, descrever e listar tópicos Kafka |
| skill `alarm-digest` | Digest diário de alarmes — classifica ATIVO/FLAPPING/RECUPERADO |
| skill `flink-deploy-recovery` | Recovery de job Flink com CrashLoop ou state corruption |
| skill `webhook-lag-resend` | Diagnóstico de lag e resend de webhook |
| skill `webhook-local-validate` | Ciclo completo de validação local de jobs Flink webhook |
| skill `launchdarkly` | Consultar e gerenciar feature flags (LaunchDarkly) |
| skill `kotlin-conventions` | Convenções Kotlin/Gradle para repos da empresa |
| skill `cobli-review` | Code review completo (CodeRabbit + boas práticas) |
| skill `cobli-review-data` | Code review para PRs de dados/analytics |
| skill `add-alert-event` | Adicionar novo tipo de evento de alerta |
| skill `add-public-doc` | Documentar endpoint no repositório público de APIs |
| skill `driver-association-investigation` | Investigar associações incorretas de motorista/dispositivo |
| skill `pr-comment` | Responder findings de PR com contexto e argumentação |
| skill `promote-skill` | Promover skill local para global |
| skill `workflow-conventions` | Convenções operacionais da plataforma workflow |
