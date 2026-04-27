#!/usr/bin/env bash
# memory-observer.sh — coleta artefatos de sessão para análise de memória
#
# Uso:
#   bash ~/workflow/scripts/memory-observer.sh [--repo <path>] [--hours N]
#
# Output: contexto estruturado para Claude propor comandos wtb memory set
# Chamado pelo Session Exit Rule (passo 1) antes de editar topic files manualmente.
#
# NÃO salva nada automaticamente — Claude propõe, usuário confirma.

set -euo pipefail

WORKFLOW_PATH="${WORKFLOW_PATH:-$HOME/workflow}"
REPO_PATH=""
HOURS=8

while [[ $# -gt 0 ]]; do
    case $1 in
        --repo)  REPO_PATH="$2"; shift 2 ;;
        --hours) HOURS="$2";    shift 2 ;;
        -h|--help)
            echo "Uso: memory-observer.sh [--repo <path>] [--hours N]"
            echo "  --repo   caminho do repositório ativo (ex: ~/fusca)"
            echo "  --hours  janela de tempo em horas (default: 8)"
            exit 0
            ;;
        *) echo "Flag desconhecida: $1"; exit 1 ;;
    esac
done

# ── header ────────────────────────────────────────────────────────────────────

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║  Memory Observer — artefatos de sessão                       ║"
echo "║  Propósito: Claude analisa e propõe wtb memory set commands   ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# ── artefatos ─────────────────────────────────────────────────────────────────

echo "## SESSION.YML (contexto operacional ativo)"
echo '```'
cat "$WORKFLOW_PATH/session.yml" 2>/dev/null | head -40 || echo "(sem session.yml)"
echo '```'
echo ""

echo "## GIT LOG — últimas ${HOURS}h"
echo '```'
if [[ -n "$REPO_PATH" && -d "$REPO_PATH" ]]; then
    git -C "$REPO_PATH" log \
        --oneline \
        --since="${HOURS} hours ago" \
        --format="%h %s (%cr)" \
        2>/dev/null || echo "(sem commits no período)"
    echo ""
    echo "--- diff stat ---"
    git -C "$REPO_PATH" diff \
        --stat \
        "HEAD@{${HOURS} hours ago}..HEAD" \
        2>/dev/null | tail -20 || true
else
    echo "(--repo não fornecido ou inválido — pular git log)"
fi
echo '```'
echo ""

echo "## BACKLOG — tarefas concluídas/iniciadas hoje"
echo '```'
wtb backlog list --status done    2>/dev/null | tail -15 || true
echo "---"
wtb backlog list --status in-progress 2>/dev/null | tail -10 || true
echo '```'
echo ""

echo "## MEMORY ATUAL — não repetir keys existentes"
echo '```'
wtb memory list 2>/dev/null || echo "(sem entries)"
echo '```'
echo ""

# ── instrução para Claude ─────────────────────────────────────────────────────

cat << 'INSTRUCAO'
---
## INSTRUÇÃO PARA CLAUDE — analisar e propor saves

Analise os artefatos acima e proponha **até 5** comandos `wtb memory set`
para fatos operacionais não-óbvios aprendidos nesta sessão.

### Critérios de INCLUSÃO (vale salvar)
- Threshold calibrado empiricamente (ex: limite safe descoberto em prod)
- ID de conexão, endpoint ou config nova usada em prod
- Decisão de processo não documentada (ex: "usar user iris para DELETE ScyllaDB")
- Bug ou padrão de falha encontrado e confirmado
- Regra de processo nova ou corrigida que não está em CLAUDE.md

### Critérios de EXCLUSÃO (não salvar)
- Já existe key similar em memory list acima → propor update se o valor mudou
- Está no código-fonte (derivável de git log/grep)
- É temporário desta sessão (ex: branch name, PR number)
- Está em CLAUDE.md como chain rule estável

### Regra de verificação — obrigatória antes de propor
Backlog descriptions capturam intenção/contexto do PR, não necessariamente a config final
que foi ao ar. Antes de propor save baseado em backlog:
1. Verificar no arquivo de config real (helm values, YAML, código) se o valor está lá
2. Checar se existe arquivo de override de prod (ex: prod.yaml) que contradiz o default
Se não verificar e a config disser outra coisa → não propor (falso positivo é pior que silêncio)

### Tipos válidos
- `threshold` — limite numérico calibrado (OOM risk, HPA min/max, etc.)
- `config`    — ID de conexão, endpoint, parâmetro de infra
- `fact`      — fato operacional confirmado, decisão de design
- `rule`      — regra de processo nova (vai para MEMORY.md §Regras)

### Formato de saída obrigatório (uma por proposta)

---
**Fato:** <descrição do fato em 1 linha>
**Motivo:** <por que não é óbvio — o que seria perdido sem este save>
**Comando:** `wtb memory set <key> "<valor>" --type <tipo> --topic <topic> --desc "<desc curta>"`
---

Se não houver fatos novos que atendam os critérios, responder:
> Nenhum fato novo identificado nesta sessão que não esteja já em memory ou CLAUDE.md.
INSTRUCAO
