---
name: Consultar memória antes de perguntar ou declarar ausência
description: Sempre verificar MEMORY.md e wtb doc search antes de dizer "não sei" ou "não tenho registro"
type: feedback
---

Antes de responder que não sabe ou não tem registro de algo, consultar obrigatoriamente:
1. `MEMORY.md` → topic files relevantes
2. `wtb doc search "<termo>"` + sinônimos
3. `wtb doc list --type runbook` / `--type discovery` se a busca retornar nada

Só declarar ausência de registro após esgotar essas fontes.

**Why:** O runbook `herbie-dashboard-padrões-e-heurísticas` tinha o procedimento completo para rodar Playwright localmente (Node 24, .env, bun run playwright:test:smoke), mas foi declarado como "não trivialmente acessível" sem consultar o CLI primeiro.

**How to apply:** Qualquer pergunta sobre "como fazer X" em repos conhecidos → buscar antes de responder. Especialmente comandos de execução, setup local, convenções de projeto.
