# Use Case: Code Review — Análise e Postagem de PR

**Tipo:** technical | **Engine:** llm + shell (gh CLI)

---

## Quando usar

- PR aberto aguardando review com rigor
- Mudanças de configuração (logback, k8s values, Flink config, env vars)
- Breaking changes potenciais — comportamento silenciosamente diferente
- Qualquer PR que vai direto para produção após merge

## Quando NÃO usar

- PR trivial (typo, bump de versão) — revise diretamente
- Objetivo é só confirmar CI verde → use `ci-watch`
- PR já mergeado → use `postmortem` se o problema causou incidente

---

## Contrato do processo

> **Nunca postar comentário, commit ou merge sem pedido expresso.**
>
> Apresentar rascunho → iterar → aguardar "aprovado" → postar.
> Parar e perguntar em ambiguidades antes de avançar.
> Aprovação de postagem não implica aprovação de merge.
>
> **Não reclamar de PR body vazia.** O time não adota descrição obrigatória no PR.
> Se contexto for necessário, buscar no Jira ou perguntar diretamente.
>
> **Para 🔴 candidatos com estado externo** (facets Datadog, consumers downstream, contratos de API):
> verificar se o risco existe na realidade antes de bloquear — ou perguntar ao operador.
> Um 🔴 teórico que o autor já tratou é pior do que um 🟡 com pergunta de validação.
>
> **CI pendente = aguardar.** Se o PR tem CI failing ou pending, não postar review nem comentário.
> Para CI failing por deprecação: handoff para `ci-fix` antes de continuar o review.
> Só retomar o review após `conclusion = success` confirmado.

---

## 1. Buscar contexto do PR {#fetch}

```bash
# Diff completo — ler o arquivo INTEIRO de cada arquivo modificado
gh pr diff {PR} --repo {owner/repo}

# Metadados
gh pr view {PR} --repo {owner/repo} --json title,body,files,headRefOid

# Contexto Jira (se disponível) — usar Atlassian MCP: getJiraIssue(issueIdOrKey="SS-XXXX")

# Reviews automáticos existentes (CodeRabbit, SonarQube, etc.)
# Ler como input com ceticismo — ver seção 2.5
gh api repos/{owner/repo}/pulls/{PR}/reviews \
  --jq '.[] | {user: .user.login, state, body: .body[:200]}'

# Comentários inline já postados
gh api repos/{owner/repo}/pulls/{PR}/comments \
  --jq '.[] | {user: .user.login, path, line, body: .body[:200]}'
```

---

## 1.5. Contexto ortogonal — estado real em produção {#ortho}

Para PRs de configuração (logback, k8s, Flink, DB migration), coletar evidências do
ambiente **antes** de classificar riscos. Um 🔴 com evidência de produção é mais forte
(e mais honesto) do que um 🔴 teórico.

| Tipo de PR | O que coletar | Como |
|-----------|--------------|------|
| Logging / logback | Estrutura atual dos logs — facets ativos, dashboards | Datadog Log Explorer, `wtb ops logs-analyze` |
| DB migration | Tamanho da tabela, locks, índices | `wtb ops db-health` |
| k8s values | Estado dos pods, HPA, recursos | `wtb ops k8s-status` |
| Flink / Kafka config | Lag, backlog, epoch errors | `wtb ops kafka-status` |
| API contract | Consumers downstream, versão em uso | `gh api` + docs |

**Quando não é possível acessar produção:** descrever o risco como hipótese com
pergunta de validação, não como 🔴 blocante. Preferir:
> "Há facets/monitores no Datadog para os campos MDC do Severino?"

em vez de:
> "🔴 Mudança de estrutura — consumers vão quebrar."

---

## 2. Lentes de análise {#analyze}

Para cada arquivo do diff, passar pelas lentes:

| Lente | Pergunta |
|-------|----------|
| **Correção** | O código faz o que o PR declara? Há casos não cobertos? |
| **Completude** | Testes faltando? Edge cases ignorados? |
| **Consistência** | Diverge de padrões do projeto? Nomenclatura inconsistente? |
| **Risco operacional** | Mudança silenciosa de comportamento? Breaking change downstream? |
| **Clareza** | Legível por quem não escreveu? Condições confusas? |

**Checklist de risco operacional** (avaliar sempre):
- [ ] Variáveis de env adicionadas — todas referenciadas no código?
- [ ] Appenders/filtros de log — aplicados em todos os paths de produção?
- [ ] Estrutura de output JSON — consumidores downstream precisam migrar?
- [ ] Condições de roteamento — consistentes entre si?

**Padrões estabelecidos no projeto** (não flaggar como issue):

| Padrão | Contexto | Razão |
|--------|---------|-------|
| PR body vazia | Qualquer PR | Time não adota descrição obrigatória |
| Campos numéricos como `string` no DTO de request de importação | `ValidateImport*Dto`, `ImportRow*Dto` | Planilhas chegam como string — serviço valida e converte; `string` no input, `float/int` no output é design intencional |

**Checklist anti-falso-positivo** (antes de flaggar elemento ausente):
- [ ] Leu o diff **inteiro** do arquivo afetado — não apenas as primeiras ocorrências?
- [ ] Verificou todas as seções onde o elemento poderia aparecer (ex: appender JSON *e* console)?
- [ ] Para claims "X não existe no código", fez busca no diff completo antes de flaggar?
- [ ] Para mudanças de estrutura de output: perguntou se é intencional antes de declarar breaking change?

---

## 2.5. Ferramentas automáticas: CodeRabbit, SonarQube e similares {#auto-tools}

Ferramentas de review automático rodam análise estática **sem contexto de domínio**,
arquitetura ou decisões históricas do time. Usar como input, nunca como verdade.

**São confiáveis para:**
- Padrões sintáticos, formatação, cobertura quantitativa
- Vulnerabilidades conhecidas (OWASP top 10, CVEs catalogadas)
- Code smells bem-definidos (duplicação, complexidade ciclomática)

**São não-confiáveis para:**
- Semântica de negócio: "esse campo deveria ser obrigatório?"
- Contexto organizacional: "esse padrão é aceitável neste time?"
- Trade-offs arquiteturais: "refatorar agora vs crescer o debt"
- Mudanças intencionais que parecem erros: dead code que é fallback deliberado,
  configuração que parece redundante mas segue padrão de org

**Protocolo:**
1. Ler os findings da ferramenta **antes** de analisar o diff
2. Para cada finding que parece relevante: validar contra o diff e contexto do PR manualmente
3. Nunca copiar/amplificar um finding sem verificação independente
4. Se um item parece óbvio demais para o autor ter ignorado → provavelmente há contexto
   que a ferramenta não tem. Perguntar antes de flaggar.

> **Regra:** o erro de amplificar um falso positivo de ferramenta automática
> é mais custoso do que ignorar um finding não verificado.

---

## 3. Formato de comentário {#format}

Cada item segue esta estrutura:

```
**{🔴|🟡|🟢} {título curto}**

Observação: o que foi identificado e por que importa.

[Diagrama Mermaid se o fluxo ou estrutura facilita a explicação]

| Opção | Vantagem | Desvantagem | Score |
|-------|----------|-------------|-------|
| ...   | ...      | ...         | ⭐⭐⭐  |

Recomendação: ação concreta ou pergunta de validação.

```suggestion
<código proposto>
```
```

### Taxonomia de severidade

| Badge | Label | Critério | Impacto no review |
|-------|-------|----------|-------------------|
| 🔴 | Change request | Bug, risco operacional, verificação obrigatória antes do merge | → REQUEST_CHANGES |
| 🟡 | Sugestão | Melhoria não bloqueante: legibilidade, consistência, overhead evitável | neutro |
| 🟢 | Excelente | Boa prática reconhecida — validar explicitamente, sem exagero | neutro |

### Critérios de score (⭐ = baixo, ⭐⭐⭐ = alto)

- **Impacto operacional**: afeta produção, Datadog, consumers downstream?
- **Impacto de legibilidade**: confunde a próxima pessoa que ler o arquivo?
- **Reversibilidade**: pode ser corrigido pós-merge sem risco?
- **Esforço de correção**: 1 linha vs refactor

### `suggestion` blocks

Use `suggestion` quando:
- A mudança proposta é clara e cabe em 1–N linhas contíguas
- As linhas estão no diff (added/changed, `side: RIGHT`)
- O aplicador pode clicar "Apply suggestion" sem precisar entender o contexto completo

Não use `suggestion` quando:
- A mudança envolve múltiplos arquivos
- É uma pergunta de validação, não uma proposta de código
- A linha não está no diff (context lines não aceitam suggestions via API)

---

## 4. Tom padrão {#tone}

> Consultivo, amigável, evolutivo — sem arrogância.

- Observação antes de prescrição
- Tradeoffs antes de recomendação
- Elogiar o que está bem feito — explicitamente, sem exagero
- Perguntas abertas para itens de validação ("confirmar se há facets...")
- Evitar "deveria", "está errado" — preferir "proposta:", "vale verificar:"

---

## 5. Postar o review {#posting}

Após aprovação explícita do operador:

```bash
# Montar payload JSON (Python para evitar escaping de backticks)
python3 - <<'PYEOF'
import json

review = {
    "commit_id": "{HEAD_SHA}",
    "body": "{resumo geral}",
    "event": "REQUEST_CHANGES",  # ou COMMENT, APPROVE
    "comments": [
        {
            "path": "{arquivo}",
            "line": {linha_no_novo_arquivo},
            "side": "RIGHT",
            "body": "{corpo do comentário com ```suggestion...```}"
        }
    ]
}

with open('/tmp/review.json', 'w', encoding='utf-8') as f:
    json.dump(review, f, ensure_ascii=False)
PYEOF

# Postar
gh api repos/{owner/repo}/pulls/{pr}/reviews \
  --method POST \
  --input /tmp/review.json \
  --jq '{id: .id, state: .state, submitted_at: .submitted_at}'
```

**Notas:**
- `line` = número da linha no arquivo da branch (não posição no diff)
- Linhas não modificadas no diff retornam erro 422 — usar só linhas changed/added
- `side: RIGHT` para linhas da nova versão do arquivo
- Para multi-linha: adicionar `start_line` (início do bloco a substituir)

---

## 5.5. Gate de merge {#merge-gate}

Antes de mergear, verificar o estado do PR:

```bash
gh pr view {PR} --repo {owner/repo} \
  --json state,mergeable,mergeStateStatus,reviewDecision,author \
  --jq '{state,mergeable,mergeStateStatus,reviewDecision,author: .author.login}'
```

### Interpretação do `mergeStateStatus`

| Status | Significado | Ação |
|--------|-------------|------|
| `CLEAN` | Pronto para merge | Mergear após confirmação do operador |
| `BLOCKED` + `reviewDecision: REVIEW_REQUIRED` | Falta aprovação | Ver abaixo — **perguntar** |
| `BLOCKED` + outros motivos | Branch protection, checks | Investigar antes de agir |
| `UNSTABLE` | CI com warning | Verificar se é bloqueante |
| `BEHIND` | Branch desatualizada | `gh pr update-branch` ou rebase |

### Regra de aprovação

Se `reviewDecision: REVIEW_REQUIRED`:

1. **Verificar se o operador é o autor do PR:**
   ```bash
   gh pr view {PR} --repo {owner/repo} --json author --jq '.author.login'
   ```
2. **Se for o autor:** não pode aprovar o próprio PR — informar e aguardar outro reviewer
3. **Se não for o autor:** **perguntar** antes de aprovar — não assumir

   > "O PR está bloqueado por falta de aprovação. Você não é o autor — quer aprovar?"

   A resposta depende do contexto: revisou o conteúdo? Tem contexto suficiente?
   Não é automático — é uma decisão do operador.

**Regra geral:** autorização de merge não está implícita em nenhuma outra ação anterior.
Cada passo (postar review, aprovar, mergear) requer confirmação explícita do operador.

---

## 6. Caso de referência: SS-2204 — PR #380 Cobliteam/severino {#reference}

**Contexto:** Reescrita de `logback.xml` — `LogstashEncoder` → `LoggingEventCompositeJsonEncoder` + configuração de logging JSON para Datadog.

**Itens identificados:**

| # | Item | Arquivo | Linha | Severidade |
|---|------|---------|-------|-----------|
| 1 | `ROOT_LOG_LEVEL: INFO` redundante com default | values.yaml | — | 🟢 Excelente |
| 2 | Condição `DD_LOGS_INJECTION` vs `DD_SERVICE` inconsistente | logback.xml | 24 | 🟡 Sugestão |
| 3 | Filtro `InstanceAlreadyExistsException` ausente no appender JSON | logback.xml | 8 | 🔴 Change request |
| 4 | `scanPeriod="30 seconds"` desnecessário em container | logback.xml | 1 | 🟡 Sugestão |
| 5 | MDC keys movidas de top-level para `custom.*` | logback.xml | 45 | 🔴 Verificar |

**Decisões tomadas neste review:**
- Item 1 (🟢): pulado na postagem — não agrega informação nova no PR
- Review type: REQUEST_CHANGES (itens 3 e 5 exigem ação antes do merge)
- `suggestion` blocks em linhas 1, 8, 24 (todas no diff da branch)
- Item 5: comentário de validação sem suggestion — é gate de confirmação, não mudança de código

**Post-mortem (2026-02-25):**

| Item | Diagnóstico | Causa | Ação corretiva |
|------|-------------|-------|----------------|
| 3 🔴 | **Falso positivo** — filtro estava no PR (linha 65, appender JSON) | Diff lido parcialmente: análise parou no appender `console`, não verificou o bloco `JSON` | REQUEST_CHANGES dismissed |
| 5 🔴 | Contexto ausente — `custom.*` é intencional, já em uso em produção | Não coletamos estado real do ambiente (Datadog) antes de flaggar | APPROVED após confirmação do autor |
| 2 🟡 | Contexto parcial — `scanPeriod` segue padrão do `k8s-clusters` org | Padrão organizacional não visível só pelo PR | Válido levantar, contexto adicional útil |

**Lições:**
- Para "elemento ausente": ler o diff inteiro do arquivo antes de flaggar (checklist anti-falso-positivo)
- Para "breaking change de estrutura": verificar estado real em produção ou perguntar antes de bloquear
- Ferramenta de review: não havia CodeRabbit/Sonar neste PR — mas o mesmo princípio aplica

**Padrão identificado:** configuração de infra (logback, k8s values) tem alta probabilidade de breaking change silencioso — lente de "risco operacional" sempre alta prioridade. **Mas:** risco operacional precisa de evidência, não apenas de plausibilidade teórica.

---

## 7. Handoffs possíveis

```
code-review
  └─► merge           ← após itens endereçados pelo autor
  └─► postmortem      ← se padrão sistêmico vale documentar (ex: filtros de log sempre incompletos)
  └─► discovery       ← se análise revela tema maior que vale explorar separadamente
```

---

## Artefatos gerados

- **GitHub PR Review** — comentários inline + verdict (principal)
- `~/.workflow/code-review/NNN-<context>-YYYY-MM-DD.md` — preservar apenas reviews complexos ou com padrões sistêmicos que valem referência futura
