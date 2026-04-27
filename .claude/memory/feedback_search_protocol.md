---
name: feedback_search_protocol
description: Protocolo de busca em múltiplas camadas — fallback obrigatório quando wtb doc search falha
type: feedback
---

`wtb doc search` cobre apenas docs.db (FTS5). Artefatos em savepoints/*.md, artifacts/ e clipboard não são indexados. Após 2 tentativas sem resultado, escalar imediatamente para fallback.

**Por que:** Busca por rascunho de anúncio de webhook (31/03/2026) levou 10+ rounds. O artefato estava em savepoints/*.md — nunca seria encontrado via wtb doc search.

**Protocolo em 3 camadas:**

**Camada 1 — docs.db (padrão):**
```bash
wtb doc search "<termo>"
wtb doc search "<sinônimo>"          # sempre tentar pelo menos 2 termos
```

**Camada 2 — git + savepoints/*.md (após 2 falhas):**
```bash
git -C ~/workflow log --oneline --since="<data>" | grep -i "<termo>"
grep -ri "<termo>" ~/workflow/savepoints/
grep -ri "<termo>" ~/workflow/artifacts/
```

**Camada 3 — busca ampla (último recurso):**
```bash
grep -ri "<termo>" ~/workflow/.claude/memory/
find ~/workflow/artifacts -name "*.md" | xargs grep -li "<termo>"
```

**Regra de ativação:** se Camada 1 falhar em 2 tentativas com termos diferentes → ir direto para Camada 2. Não tentar mais variações no wtb.

**Termos de busca:** preferir substantivos do conteúdo ("anúncio", "câmera", "draft") em vez de metadados ("relançamento", "rascunho") — o conteúdo salvo usa linguagem técnica, não descritiva.
