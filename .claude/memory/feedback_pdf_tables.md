---
name: PDF — Casos edge e detalhes de DER matplotlib
description: Complemento à skill pdf — casos edge de tabelas e template detalhado de DER com matplotlib para docs de onboarding
type: feedback
---

## Regras base de tabelas e loop visual

As regras fundamentais (separadores proporcionais, backtick overflow, prefixo de rotas, unicode, loop pdftoppm) estão na **skill `pdf`** — fonte canônica.

Este arquivo cobre apenas casos edge e o template DER.

---

## Casos edge de tabelas

### Backtick em coluna de consumers Kafka

Nomes de consumer classes (`TriggerEngineEventsConsumer`, `DeviceHealthStatusEventsConsumer`) são 28–35 chars e não quebram linha. Usar separador largo `:-----------------------------------` E manter backtick somente se < 28 chars.

### Tabela de controllers com rotas longas

Padrão confirmado (analytics-report-api, 2026-03-31):
1. Nota antes da tabela: `> Prefixo base: \`/analytics/v1/\``
2. Coluna "Sufixo da rota" sem backtick
3. Coluna "Controller" sem backtick (classe é óbvia pelo cabeçalho)
4. Separador col 1: `:----------------------------------------` (40 dashes, cobre `DriverVehicleAssociationController` = 35 chars + margem)

### Seção com 2–3 linhas isolada no fim de página

Converter `## Heading` + conteúdo curto em `**Heading** — conteúdo inline`. Evita que LaTeX force nova página antes do heading (comportamento padrão para `\subsection`).

---

## Template DER matplotlib (onboarding-doc)

### Constantes de layout

```python
HDR_H  = 0.42   # altura do header da entidade
ROW_H  = 0.30   # altura por campo
GAP    = 0.25   # espaço vertical entre entidades na mesma coluna
W_ENT  = 3.8    # largura padrão de entidade

def entity_height(n_fields):
    return HDR_H + n_fields * ROW_H
```

### Grid de colunas (3 colunas típico)

```python
COL1_X, COL2_X, COL3_X = 0.3, 4.7, 9.5
MARGIN_TOP = 0.3

# Calcular posições bottom-up por coluna
def layout_column(entities, col_x):
    y = MARGIN_TOP
    positions = {}
    for e in entities:
        positions[e['name']] = (col_x, y)
        y += entity_height(len(e['fields'])) + GAP
    return positions
```

### `draw_entity` com edge midpoints (obrigatório)

```python
from matplotlib.patches import FancyBboxPatch

def draw_entity(ax, x, y, title, fields, w=W_ENT, fc='#EBF5FB', ec='#2980B9', fontsize=7.5):
    h = entity_height(len(fields))
    patch = FancyBboxPatch((x, y), w, h,
                           boxstyle='round,pad=0.05',
                           facecolor=fc, edgecolor=ec, linewidth=1.2)
    ax.add_patch(patch)
    # Header
    ax.text(x + w/2, y + h - HDR_H/2, title,
            ha='center', va='center', fontsize=fontsize,
            fontweight='bold', color='white',
            bbox=dict(facecolor=ec, edgecolor='none',
                      boxstyle='round,pad=0.05', alpha=0.9))
    # Campos
    for i, field in enumerate(fields):
        fy = y + h - HDR_H - (i + 0.5) * ROW_H
        ax.text(x + 0.12, fy, field, ha='left', va='center',
                fontsize=fontsize - 0.5, color='#2C3E50')
    # Retornar edge midpoints — obrigatório para setas
    return {
        'top':    (x + w/2, y + h),
        'bottom': (x + w/2, y),
        'left':   (x,       y + h/2),
        'right':  (x + w,   y + h/2),
    }
```

### Função de seta (edge-to-edge)

```python
def arrow(ax, p1, p2, label='', color='#7F8C8D', lw=1.3, rad=0.0):
    ax.annotate('', xy=p2, xytext=p1,
                arrowprops=dict(arrowstyle='->', color=color,
                                lw=lw, connectionstyle=f'arc3,rad={rad}'))
    if label:
        mx, my = (p1[0] + p2[0]) / 2, (p1[1] + p2[1]) / 2
        ax.text(mx, my + 0.06, label, ha='center', va='bottom',
                fontsize=6.5, color=color)
```

**Why:** Aprendido na geração do mapa GF Core (2026-03-31) após múltiplas iterações de pdftoppm para corrigir setas que passavam por dentro dos boxes de entidade.
