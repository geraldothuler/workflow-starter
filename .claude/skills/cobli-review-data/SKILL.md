# Skill: /cobli-review-data

**Propósito:** Gerar relatórios de validação (A) e normalização (B) de dados comparando vehicles.db.json da branch main com a branch da PR.

**Escopo:** Herbie API — validação estrutural de crawler Molicar (trimestral).

**Invocação:** Comentário em PR do herbie-api:
```
/cobli-review-data A       # Validação apenas
/cobli-review-data B       # Normalização apenas
/cobli-review-data A+B     # Ambas (padrão)
```

---

## Input

- **PR comment context:** Deve estar comentando em um PR aberto do herbie-api (obtém PR#, branch, owner, repo automaticamente via GitHub API)
- **Scope:** String `A`, `B`, ou `A+B` (extraído do comentário)

---

## Execution

1. **Carregar dados:**
   - `vehicles.db.json` da branch `main`
   - `vehicles.db.json` da branch atual (PR)

2. **Análise de Validação (Scope A):**
   - **Estrutura hierárquica:** category → maker → year → model_year → model → version
   - **Row count delta:** Δ% entre versões (PASS: ±10%, WARN: ±10-20%, FAIL: >20%)
   - **Schema validation:** Campos obrigatórios (model, version, maker)
   - **Makers:** Novos adicionados, removidos
   - **Sampling:** Verificar 10 registros aleatórios

3. **Análise de Normalização (Scope B):**
   - **Duplicates:** Registros idênticos
   - **Casing:** Inconsistências maiúsculas/minúsculas
   - **Internal codes:** Valores numéricos internos que devem ser removidos
   - **Tier names:** Inconsistências em nomes de trim (GLS, GLB, LS, LIMIT, PLUS)
   - **Near-duplicates:** Levenshtein distance ≤ 2 em version strings

4. **Post result:** Comentário GitHub com relatório markdown + status ✅/⚠️

---

## Output

**GitHub PR comment** com:
- Status geral (PASS/WARN/FAIL)
- Métricas de validação (row count, delta%, schema)
- Métricas de normalização (duplicates, casing, codes, tiers, near-dupes)
- Recomendação: ✅ pronto para merge | ⚠️ revisar issues específicos

---

## Environment

| Var | Origem | Uso |
|-----|--------|-----|
| `GITHUB_TOKEN` | GitHub user/workflow token | API autenticado |
| `PR_NUMBER` | GitHub context (extraído do URL ou env) | ID do PR |
| `PR_BRANCH` | GitHub context (branch name) | Comparação vs main |
| `GH_OWNER` | Padrão: `CobliteamTeam` | Repo owner |
| `GH_REPO` | Padrão: `herbie-api` | Repo name |
| `SCOPE` | Extraído do comentário (A/B/A+B) | Seleção de análise |

---

## Implementation

**Módulo:** `herbie-dashboard-api/src/vehicles-info-db/cobli_review_data_skill.py`

**Classes:**
- `VehiclesDataAnalyzer`: Carrega JSON, extrai records, análise deltas
- `analyze_validation()`: Row count Δ, schema, makers
- `analyze_normalization()`: Duplicates, casing, codes, tiers, near-dupes
- `load_vehicles_json(branch)`: Git show + json.loads
- `post_github_comment(owner, repo, pr_number, body, token)`: API v3

**Erro handling:**
- Branch não encontrada → fallback para HEAD
- vehicles.db.json não existe → erro explícito (setup incompleto)
- GitHub API erro → retry 1x, log, return False

---

## Workflow Integration

**Trigger:** PR comment em herbie-api contendo `/cobli-review-data`

**Fluxo:**
1. GitHub webhook (se configurado) → disparar CI job
2. Ou: Claude Code lê comment via GitHub MCP → invoca skill
3. Skill carrega, analisa, posta comentário
4. Reviewer aprova/pede changes

**Quando usar:**
- ✅ Após webhook automático criar PR (botão "review data")
- ✅ Review manual de PR de atualização de dados
- ✅ Troubleshooting: dados não atualizando corretamente

---

## Examples

### Exemplo 1: Validação apenas
```
/cobli-review-data A
```
Output:
```
## 📋 Validation Report

**Status:** `PASS`
**Records (main):** 15,237
**Records (PR):** 15,305
**Delta:** +0.4%

**✅ Schema:** Valid
```

### Exemplo 2: Tudo (padrão)
```
/cobli-review-data A+B
```
Output:
```
## 📋 Validation Report
...
## 🔧 Normalization Report
- Duplicates removed: 3
- Casing issues: 2
- Internal codes removed: 1
- Tier name issues: 0
- Near-duplicates detected: 0
**✅ Data is clean and normalized.**
```

### Exemplo 3: Normalização apenas
```
/cobli-review-data B
```
Output:
```
## 🔧 Normalization Report
**Total Issues:** 6
...
```

---

## Troubleshooting

| Problema | Causa | Solução |
|----------|-------|---------|
| "Could not load from PR branch" | Branch não tem vehicles.db.json | Verificar se crawl_vehicle_data.py foi executado |
| "GITHUB_TOKEN not set" | CI/action não passou token | Verificar secrets configuradas no workflow |
| Comment não postado | PR não existe ou token sem acesso | Verificar PR# e repo |
| Delta > 20% (FAIL) | Estrutura mudou ou dados inválidos | Review schema em crawl_vehicle_data.py |

---

## Next Phase Integration

- **Phase 5 complete:** Skill `/cobli-review-data` criado
- **Phase 6:** Integração com `crawl_vehicle_data.py` + `github_webhook.py`
  - webhook chama `handle_webhook_payload()` (cria PR)
  - PR comment com `/cobli-review-data A+B` invoca skill
  - Skill compara versões e posta análise
- **Phase 7:** Documentação SKILL.md + operacional runbook
- **Phase 8:** Deploy em prod + testes e2e

---

**Versão:** 1.0 | **Última atualização:** 2026-04-13
