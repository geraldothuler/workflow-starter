# Skill: /daily — Daily Report

Gera o report diário do período desde a última daily, coleta fontes, produz PDF com seção executiva destacada e abre automaticamente.

---

## Quando usar

Invocar no horário configurado para sua daily (ex: seg/qua/sex às 11h) ou quando o usuário pedir `/daily`.

Configure o schedule: `wtb memory set daily_schedule "seg,qua,sex 11:00" --type config --topic daily`

---

## 1. Calcular o período

Período: desde 30min após a daily anterior até a hora da daily atual.

Configure o horário: `wtb memory get daily_schedule`

Data de corte para `--since` do git: data de início.

---

## 2. Coletar as fontes

### Fonte 1 — git log (repos do projeto)

Descobrir repos automaticamente:

```bash
# Via repoindex
wtb repo list
# ou via MCP: repo_list tool

# Para cada repo com path local configurado:
git -C <path-do-repo> log --since="<data-inicio>" --until="<data-fim>" \
  --oneline --all 2>/dev/null
```

Configure paths dos repos: `wtb memory list --topic repos`

### Fonte 2 — wtb doc (discoveries, savepoints, incidents)

```bash
wtb doc list --since <data-inicio> --type discovery
wtb doc list --since <data-inicio> --type savepoint
wtb doc list --since <data-inicio> --type incident
```

Para cada artefato relevante: `wtb doc get <id>` para expandir o conteúdo.

### Fonte 3 — Google Calendar (opcional)

Se MCP Google Calendar disponível:

```
gcal_list_events:
  timeMin: <data-inicio>T<hora-corte>:00<timezone-offset>
  timeMax: <data-fim>T<hora-daily>:00<timezone-offset>
  timeZone: <sua-timezone>   # ex: America/Sao_Paulo
  calendarId: primary
```

Configure timezone: `wtb memory set daily_timezone "America/Sao_Paulo" --type config --topic daily`

### Fonte 4 — Slack (opcional)

Se MCP Slack disponível, configure os canais via memória:

```bash
wtb memory set daily_slack_channels "#engineering,#incidents" \
  --type config --topic daily \
  --desc "Canais Slack para coletar contexto da daily"
```

Consultas padrão:
```
slack_search_public_and_private: query="in:#<canal-principal> after:<data-inicio>"
slack_search_public_and_private: query="from:<seu-usuario> after:<data-inicio>"
slack_search_public_and_private: query="in:#<canal-incidentes> after:<data-inicio>"
```

Para threads de incidente: `slack_read_thread` para identificar root cause.

---

## 3. Coletar Próximos Passos

### Agenda futura (Google Calendar)

```
gcal_list_events:
  timeMin: <data-fim>T<hora-daily>:00<timezone-offset>
  timeMax: <data-fim + 5 dias>T18:00:00<timezone-offset>
  timeZone: <sua-timezone>
```

### Backlog in-progress e blocked

```bash
wtb backlog list --status in-progress
wtb backlog list --status blocked
```

---

## 4. Classificar os itens em categorias

| Categoria | Critério |
|:----------|:---------|
| **Planejado** | Trabalho do backlog/board: features, tech debt, melhorias |
| **Bugs** | Bugs identificados e corrigidos |
| **Operacional** | Trabalho não planejado: incidents, atendimentos, investigations |

Dentro de cada categoria: ordenar **cronologicamente**.

---

## 5. Estrutura do report

### Resumo Executivo (Stakeholder)

Três sub-grupos em linguagem executiva — sem jargão técnico:

1. **Entregas Planejadas** — bullets do backlog concluídos no período
2. **Correções de Bugs** — em linguagem de impacto de negócio
3. **Trabalho Operacional** — incidents contidos, investigations, pendências

Cada bullet referencia a seção correspondente com `{\footnotesize \textit{-- \hyperref[sec-XN]{§XN}}}`.

### Seção Planejado (nova página)

Uma subseção por item, rotulada `§P1, §P2, ...` com heading ID `{#sec-p1}`.

### Seção Bugs (nova página)

Uma subseção por bug, rotulada `§B1, §B2, ...` com heading ID `{#sec-b1}`.
Conteúdo: sintoma, causa raiz, fix, confirmação.

### Seção Operacional (nova página)

Uma subseção por item, rotulada `§O1, §O2, ...` com heading ID `{#sec-o1}`.

### Próximos Passos

- **Compromissos de Agenda** — reuniões relevantes do período seguinte
- **Pendências Técnicas** — itens com ação pendente identificados no período

---

## 6. Gerar PDF

**NUNCA usar pandoc direto** — sempre invocar `/pdf` para garantir revisão linguística.

**Header-includes obrigatório:**
```yaml
header-includes:
  - \usepackage{tcolorbox}
  - \tcbuselibrary{breakable}
  - \usepackage{xcolor}
  - \definecolor{execbg}{RGB}{235,243,255}
  - \definecolor{execborder}{RGB}{41,98,181}
```

**tcolorbox do Resumo Executivo:**
```latex
\begin{tcolorbox}[title={\large\textbf{Resumo Executivo}}, colback=execbg, colframe=execborder, fonttitle=\bfseries\large, breakable, left=8pt, right=8pt, top=6pt, bottom=6pt]

{\small\textbf{\textcolor{execborder}{Entregas Planejadas}}}
\begin{itemize}
  \item \textbf{Tema} descrição executiva. {\footnotesize \textit{-- \hyperref[sec-p1]{§P1}}}
\end{itemize}

\vspace{4pt}
{\small\textbf{\textcolor{execborder}{Correções de Bugs}}}
\begin{itemize}
  \item \textbf{Tema} impacto de negócio resolvido. {\footnotesize \textit{-- \hyperref[sec-b1]{§B1}}}
\end{itemize}

\vspace{4pt}
{\small\textbf{\textcolor{execborder}{Trabalho Operacional}}}
\begin{itemize}
  \item \textbf{Tema} situação e status. {\footnotesize \textit{-- \hyperref[sec-o1]{§O1}}}
\end{itemize}

\vspace{4pt}
{\small\textbf{\textcolor{execborder}{Próximos Passos}}}
\begin{itemize}
  \item \textbf{Hoje/amanhã:} compromissos imediatos relevantes.
  \item \textbf{Urgente:} pendências com prazo explícito.
\end{itemize}

\end{tcolorbox}
```

**Separadores de página obrigatórios:**
- `\newpage` antes de cada seção principal

**Salvar em:** `/tmp/daily-report-YYYY-MM-DD.pdf`

```bash
open /tmp/daily-report-YYYY-MM-DD.pdf
```

---

## 7. Regras de qualidade

- Verificar visualmente TODAS as páginas (pdftoppm + Read) antes de abrir
- Checklist linguístico: acentuação, concordância
- Tabelas: separadores proporcionais — nenhum overflow
- Stakeholder: sem jargão técnico (sem nomes de classe, endpoint, variável)
- Seções técnicas: todos os detalhes relevantes (PRs, root cause, métricas)
- Incidente: root cause validado pelo thread — nunca assumir
- Se uma categoria estiver vazia no período: omitir a seção

---

## Configuração rápida

```bash
# Schedule
wtb memory set daily_schedule "seg,qua,sex 11:00" --type config --topic daily

# Timezone
wtb memory set daily_timezone "America/Sao_Paulo" --type config --topic daily

# Canais Slack (adaptar)
wtb memory set daily_slack_channels "#engineering,#incidents" --type config --topic daily

# Usuário Slack
wtb memory set daily_slack_user "seu.usuario" --type config --topic daily
```

---

## Dependências

- `tcolorbox` via `tlmgr --usermode install tcolorbox pgf environ trimspaces pdfcol`
- `pandoc` + `xelatex`
- `pdftoppm` para verificação visual
- MCPs opcionais: Google Calendar, Slack
