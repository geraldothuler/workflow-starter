---
name: cobli-review
description: >
  Code review completo seguindo os padrões Cobli: aplica best-practices universais,
  sub-skills por contexto (Terraform, Helm, logging) e CodeRabbit automático.
  USAR SEMPRE para qualquer pedido de code review, PR review, análise de qualidade ou
  revisão de código em repos Cobli — inclusive quando acionado por pr-ready, ci-fix ou
  qualquer fluxo autônomo. Substitui coderabbit:code-review diretamente.
argument-hint: "[--deep] [--no-coderabbit] [-t all|committed|uncommitted] [--base branch]"
user-invocable: true
---

# cobli-review — Code Review Cobli

Wrapper da skill `code-review@cobliteam-tools`, que fica invisível na lista por conflito de
nome com o plugin oficial `coderabbit`. Esta skill expõe o mesmo comportamento com nome
único.

## Quando usar

- `/cobli-review` — review completo do diff atual
- `/cobli-review --no-coderabbit` — só best-practices + sub-skills, sem CodeRabbit
- `/cobli-review -t committed` — só mudanças commitadas
- `/cobli-review --base main` — diff contra branch específica

## O que faz (em ordem)

### 1. Best-practices universais (sempre)

Aplica `best-practices.md` do plugin `code-review@cobliteam-tools` — cobre segurança,
custo, linguagem e estilo de review.

### 2. Sub-skills por contexto

| Arquivos alterados | Sub-skill | Tema |
|:-------------------|:----------|:-----|
| `*.tf`, `*.tfvars` | terraform | Versão de providers, tagging de recursos AWS |
| `values*.yaml`, `Chart.yaml`, `templates/**`, `deploy/*.yaml` | helm-charts | Secrets via AWS SSM |
| `*.go`, `*.kt`, `*.scala`, `*.js`, `*.ts` (qualquer fonte) | logging | Níveis, libs, formato, PII, metadata |

### 3. CodeRabbit (padrão — pular com `--no-coderabbit`)

Roda o CodeRabbit e aplica o ciclo autônomo de correção:

1. Implementar feature (se solicitado)
2. Rodar CodeRabbit review
3. Montar lista de findings
4. Corrigir críticos e warnings sistematicamente
5. Re-rodar review para verificar fixes
6. Repetir até limpo ou só issues de nível info

## Fase 4 — Deep Review com agentes paralelos (apenas com `--deep`)

Quando `--deep` for passado, após o CodeRabbit dispare **3 agents em paralelo** via Agent tool.
Cada agente recebe: o diff completo + arquivos relevantes lidos do repo + seu prompt focado.
Consolide os findings dos 3 em uma seção `## Deep Review` no final do relatório.

### Agent 1 — context-hunter

```
prompt: "Você é um revisor especializado em consistência de código. Analise o diff abaixo
e responda: (1) Algum padrão estabelecido no restante do repo foi quebrado? (2) Algum
contrato existente (API, evento Kafka, schema) foi alterado sem versionamento? (3) Existe
acoplamento novo inesperado com outros módulos? Seja específico — cite arquivo:linha.
Diff: <diff>"
```

### Agent 2 — security-auditor

```
prompt: "Você é um auditor de segurança. Analise o diff abaixo e identifique: (1) Edge
cases não tratados que podem causar exceção em prod; (2) Possíveis race conditions ou
problemas de concorrência; (3) Exposição de PII ou dados sensíveis em logs/respostas;
(4) Falhas de auth/authz ou bypass de permissão. Cite arquivo:linha para cada finding.
Diff: <diff>"
```

### Agent 3 — socratic-critic

```
prompt: "Você é um engenheiro sênior pessimista e experiente. Analise o diff abaixo e
responda: (1) O que pode falhar silenciosamente em prod sob carga alta? (2) Se precisar
fazer rollback desta mudança, há algum side effect irreversível? (3) Existe alguma
suposição implícita no código que pode não ser verdadeira em todos os ambientes?
(4) O que o autor provavelmente não testou? Seja direto e específico.
Diff: <diff>"
```

### Output esperado

```markdown
## Deep Review

### Context Hunter
- [finding 1]
- [finding 2]

### Security Auditor
- [finding 1]

### Socratic Critic
- [finding 1]
- [finding 2]
```

Se nenhum dos 3 agentes encontrar issues relevantes, omitir a seção ou registrar "Nenhum finding crítico identificado."

## Referência

Plugin original: `~/.claude/plugins/cache/cobliteam-tools/code-review/1.1.0/skills/code-review/SKILL.md`
