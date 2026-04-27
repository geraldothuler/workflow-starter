# Skill: /daily — Daily Report GF Core

Gera o report diário do período desde a última daily, coleta 4 fontes, produz PDF com seção executiva destacada e abre automaticamente.

---

## Quando usar

Invocar toda segunda, quarta e sexta às 11h BRT (ou quando o usuário pedir `/daily`).

---

## 1. Calcular o período

- Schedule: **seg / qua / sex às 11h BRT**
- Período: desde as **11h30 da daily anterior** até as **11h da daily atual**

Exemplos:
- Sex 10/04 11h → período: Qua 08/04 11h30 até Sex 10/04 11h
- Qua 08/04 11h → período: Seg 06/04 11h30 até Qua 08/04 11h
- Seg 06/04 11h → período: Sex 04/04 11h30 até Seg 06/04 11h

Data de corte para `--since` do git: data de início (ex: `--since="2026-04-08 11:30"`).

---

## 2. Coletar as 4 fontes

### Fonte 1 — git log (todos os repos ativos)

```bash
git -C ~/Cobliteam/<repo> log --since="<data-inicio> 11:30" --until="<data-fim> 11:00" \
  --oneline --all 2>/dev/null
```

Repos a varrer: herbie-api, fusca, herbie-dashboard, webhook-builder, janus, iris, georeg-api, geofence-api, trigger-action-api, osm-enhanced-api.

### Fonte 2 — wtb doc (discoveries, savepoints, incidents)

```bash
wtb doc list --since <data-inicio> --type discovery
wtb doc list --since <data-inicio> --type savepoint
wtb doc list --since <data-inicio> --type incident
```

Para cada artefato relevante: `wtb doc get <id>` para expandir o conteúdo.

### Fonte 3 — Google Calendar

Usar `gcal_list_events` com:
- `timeMin`: `<data-inicio>T11:30:00-03:00`
- `timeMax`: `<data-fim>T11:00:00-03:00`
- `timeZone`: `America/Sao_Paulo`
- `calendarId`: `primary`

Incluir: reuniões com contexto relevante (1:1s, kick-offs, reviews, alinhamentos técnicos).

### Fonte 4 — Slack

```
# Atendimentos
slack_search_public_and_private: query="in:#help-tech-account-mgm after:<data-inicio>"
slack_search_public_and_private: query="in:#help-tech after:<data-inicio>"

# Contribuições do Geraldo
slack_search_public_and_private: query="from:$WORKFLOW_SLACK_USER after:<data-inicio>"

# Incidentes
slack_search_public_and_private: query="in:#incidentes after:<data-inicio>"
```

Para threads de incidente: `slack_read_thread` para identificar causa raiz real, fix e timeline. **Nunca assumir root cause sem confirmar no thread do canal de incidente.**

---

## 3. Coletar Próximos Passos

### Agenda futura (Google Calendar)

Buscar eventos do período seguinte: do fim da daily atual até o fim da próxima semana útil.

```
gcal_list_events:
  timeMin: <data-fim>T11:00:00-03:00
  timeMax: <data-fim + 5 dias>T18:00:00-03:00
  timeZone: America/Sao_Paulo
```

Incluir reuniões relevantes (1:1s, dailies, retrospectivas, reviews, syncs). Omitir: eventos de trabalho remoto/Home, PDI, eventos all-day sem conteúdo técnico.

### Backlog in-progress e blocked

```bash
wtb backlog list --status in-progress
wtb backlog list --status blocked
```

### Pendências técnicas do período

Identificar itens com "pendente", "aguardando", "~X dias" nas seções Fora do Board — esses viram pendências técnicas nos Próximos Passos.

---

## 4. Classificar os itens em 3 categorias

Antes de escrever o report, classificar cada item coletado:

| Categoria | Critério |
|:----------|:---------|
| **Jira** | Trabalho planejado no board (SS ticket): features, tech debt, HPA, migrations |
| **Bugs** | Bugs identificados e corrigidos — independente de ter ticket ou não |
| **Fora do board** | Trabalho não planejado: incidents, atendimentos, investigations, postmortems, guardrails reativos |

Dentro de cada categoria: ordenar **cronologicamente** (qua → qui → sex, manhã → tarde → noite).

---

## 4. Estrutura do report

### Resumo Executivo (Stakeholder)

Três sub-grupos dentro do box azul, em linguagem executiva — sem jargão técnico:

1. **Entregas Planejadas** — bullets de Jira concluídos no período
2. **Correções de Bugs** — bugs resolvidos, em linguagem de impacto de negócio
3. **Trabalho Operacional** — incidents contidos, investigations, postmortems, pendências

Cada bullet referencia a seção correspondente com `{\footnotesize \textit{-- \hyperref[sec-XN]{§XN}}}`.

### Atendimentos #help-tech

Tabela: `Quando | Solicitante | Tema | Status`

### Contexto Organizacional

Lista de reuniões relevantes com contexto.

### Próximos Passos (após Contexto Organizacional, antes do \newpage do Jira)

Duas subseções:

**Compromissos de Agenda** — lista ordenada cronologicamente de reuniões relevantes do período seguinte (até a próxima daily + dias subsequentes). Formato: `- **Dia DD/MM HHh** -- Título (participantes-chave)`

**Pendências Técnicas** — itens com ação pendente identificados no período:
- Pendências das seções Fora do Board (ex: "auditar TTL", "alinhar com tech lead")
- Backlog in-progress e blocked relevantes
- Urgências com prazo explícito aparecem primeiro

### Seção Jira (nova página)

Uma subseção por item, rotulada `§J1, §J2, ...`, com heading ID `{#sec-j1}` etc.
Ordenada cronologicamente. Conteúdo: PR, merge date, resultado mensurável.

### Seção Bugs (nova página)

Uma subseção por bug, rotulada `§B1, §B2, ...`, com heading ID `{#sec-b1}` etc.
Ordenada cronologicamente. Conteúdo: sintoma, causa raiz, fix, confirmação.

### Seção Fora do Board (nova página)

Uma subseção por item, rotulada `§F1, §F2, ...`, com heading ID `{#sec-f1}` etc.
Ordenada cronologicamente. Conteúdo: timeline, ação, resultado ou pendência.

---

## 5. Gerar PDF

**NUNCA usar pandoc direto** — sempre invocar `/pdf` para garantir revisão linguística pt-BR.

**Header-includes obrigatório:**
```yaml
header-includes:
  - \usepackage{tcolorbox}
  - \tcbuselibrary{breakable}
  - \usepackage{xcolor}
  - \definecolor{execbg}{RGB}{235,243,255}
  - \definecolor{execborder}{RGB}{41,98,181}
```

**tcolorbox do Resumo Executivo (4 grupos obrigatórios):**
```latex
\begin{tcolorbox}[title={\large\textbf{Resumo Executivo}}, colback=execbg, colframe=execborder, fonttitle=\bfseries\large, breakable, left=8pt, right=8pt, top=6pt, bottom=6pt]

{\small\textbf{\textcolor{execborder}{Entregas Planejadas}}}
\begin{itemize}
  \item \textbf{Tema} descrição executiva. {\footnotesize \textit{-- \hyperref[sec-j1]{§J1}}}
\end{itemize}

\vspace{4pt}
{\small\textbf{\textcolor{execborder}{Correções de Bugs}}}
\begin{itemize}
  \item \textbf{Tema} impacto de negócio resolvido. {\footnotesize \textit{-- \hyperref[sec-b1]{§B1}}}
\end{itemize}

\vspace{4pt}
{\small\textbf{\textcolor{execborder}{Trabalho Operacional}}}
\begin{itemize}
  \item \textbf{Tema} situação e status. {\footnotesize \textit{-- \hyperref[sec-f1]{§F1}}}
\end{itemize}

\vspace{4pt}
{\small\textbf{\textcolor{execborder}{Próximos Passos}}}
\begin{itemize}
  \item \textbf{Hoje/amanhã:} compromissos imediatos relevantes (meetings, decisões pendentes).
  \item \textbf{Urgente:} pendências com prazo explícito.
  \item \textbf{Bloqueado (XX-NNN):} descrição resumida se houver item bloqueado no backlog.
\end{itemize}

\end{tcolorbox}
```

**Regras do grupo Próximos Passos no tcolorbox:**
- Máximo 3-4 bullets — apenas o mais relevante para stakeholders
- Urgências com prazo explícito (ex: "~3 dias") sempre presentes
- Itens bloqueados no backlog com ticket visível
- Não repetir agenda completa — a lista detalhada está na seção Próximos Passos

**Separadores de página obrigatórios:**
- `\newpage` antes de `## Jira`
- `\newpage` antes de `## Bugs`
- `\newpage` antes de `## Fora do Board`

**Heading IDs obrigatórios** (para os hyperlinks funcionarem):
```markdown
### §J1 -- Qua DD/MM -- Título {#sec-j1}
### §B1 -- Qui DD/MM -- Título {#sec-b1}
### §F1 -- Qui DD/MM HHh -- Título {#sec-f1}
```

**Salvar em:** `/tmp/daily-report-YYYY-MM-DD.pdf`

**Abrir automaticamente após verificação visual:**
```bash
open /tmp/daily-report-YYYY-MM-DD.pdf
```

---

## 6. Regras de qualidade

- Verificar visualmente TODAS as páginas (pdftoppm + Read) antes de abrir
- Checklist linguístico pt-BR: acentuação, cedilha, concordância
- Tabelas: separadores proporcionais ao conteúdo — nenhum overflow
- Stakeholder: sem jargão técnico (sem nomes de classe, endpoint, variável)
- Seções técnicas: todos os detalhes relevantes (PRs, root cause, métricas)
- Incidente: root cause validado pelo thread Slack — nunca assumir
- Duplicatas: consolidar em uma seção quando o mesmo tema aparece em múltiplas fontes
- Se uma categoria estiver vazia no período: omitir a seção, não criar seção vazia

---

## Dependências

- `tcolorbox` instalado no TEXMFHOME via `tlmgr --usermode --usertree ~/Library/texmf install tcolorbox pgf environ trimspaces pdfcol`
- `pandoc` + `xelatex` (`/Library/TeX/texbin/xelatex`)
- `pdftoppm` para verificação visual
- MCPs: Google Calendar (`gcal_list_events`), Slack (`slack_search_public_and_private`, `slack_read_thread`)
