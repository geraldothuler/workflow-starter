# Skill: /alarm-digest — Digest de alarmes GF Core

Analisa os canais `#alarms-gf-core` (P1) e `#notifications-gf-core` (P2/P3), classifica monitores por criticidade e salva um digest no `docs.db`. Retoma automaticamente do ponto onde parou na última execução.

**Hierarquia de canais:**
- **P1 — #alarms-gf-core** (`C0ARTSY4WGJ`): requerem ação imediata. 94 monitors.
- **P2/P3 — #notifications-gf-core** (`C0ASN5KTLE4`): sinais de degradação, não urgentes. 35 monitors.

---

## Quando usar

- `/alarm-digest` — processa ambos os canais desde o `last_run_ts` até agora
- `/alarm-digest --hours 48` — forçar janela manual (ignora `last_run_ts`)
- `/alarm-digest --triage` — análise completa: inclui NO DATA + recomendações de calibração

---

## 1. Determinar a janela de tempo

```bash
wtb memory list --topic gf-core 2>/dev/null | grep alarm-digest-last-run
```

- Se existir → `window_start = last_run_ts`
- Se não existir → `window_start = "2026-04-09T07:31:57-03:00"` (criação dos canais)
- Se `--hours N` passado → `window_start = now - N hours` (ignora state)

`window_end = now` (timestamp exato antes de qualquer query)

---

## 2. Buscar monitores que dispararam na janela

### Credenciais

```bash
DD_API_KEY=$(bash ~/workflow/scripts/secret-get.sh workflow-dd-api-key)
DD_APP_KEY=$(bash ~/workflow/scripts/secret-get.sh workflow-dd-app-key)
```

### Fetch — dois canais em paralelo

```bash
# P1
curl -s "https://api.datadoghq.com/api/v1/monitor/search?query=notification%3Aslack-alarms-gf-core&per_page=100&page=0" \
  -H "DD-API-KEY: $DD_API_KEY" -H "DD-APPLICATION-KEY: $DD_APP_KEY"

# P2/P3
curl -s "https://api.datadoghq.com/api/v1/monitor/search?query=notification%3Aslack-notifications-gf-core&per_page=100&page=0" \
  -H "DD-API-KEY: $DD_API_KEY" -H "DD-APPLICATION-KEY: $DD_APP_KEY"
```

**Filtro de exibição:**
- `overall_state == 'Alert'` → **sempre exibir** (ATIVO persistente, independente de last_triggered_ts)
- `overall_state == 'No Data'` → exibir apenas em `--triage`
- `overall_state == 'OK'` AND `last_triggered_ts >= window_start_unix` → RECUPERADO (disparou na janela)

Converter `window_start` para Unix timestamp:
```bash
python3 -c "from datetime import datetime; print(int(datetime.fromisoformat('WINDOW_START').timestamp()))"
```

---

## 3. Classificar os monitores

Mesma lógica para ambos os canais — o canal de origem define o label de prioridade:

| Classificação | Critério | P1 significado | P2/P3 significado |
|:-------------|:---------|:--------------|:-----------------|
| 🔴 **ATIVO** | `overall_state == 'Alert'` | Incidente em andamento | Degradação ativa |
| 🔁 **FLAPPING** | RECUPERADO + ≥ 3 transições na janela | Threshold mal calibrado | Sinal recorrente |
| 🟡 **RECUPERADO** | `overall_state == 'OK'` AND `last_triggered_ts` na janela | Disparou e resolveu | Spike pontual |
| ⚫ **NO DATA** | `overall_state == 'No Data'` | Monitor morto | Monitor morto |
| ✅ **SILENCIOSO** | `last_triggered_ts` fora da janela | **Não mencionar** | **Não mencionar** |

### §3.1 Detectar FLAPPING

⚠️ `tags=monitor_id:X` retorna apenas audits. Usar `service tag + sources=alert`:

```bash
curl -s "https://api.datadoghq.com/api/v1/events?start={window_start_unix}&end={window_end_unix}&sources=alert&tags=service:{SERVICE}&per_page=50" \
  -H "DD-API-KEY: $DD_API_KEY" -H "DD-APPLICATION-KEY: $DD_APP_KEY"
```

Contar `alert_type` em `['error','warning']`. Se count ≥ 3 → FLAPPING.

**Fallback para monitors sem service tag clara** (kafka lag, cassandra-sync, etc.): RECUPERADO se `last_triggered_ts` na janela e `overall_state == 'OK'`.

---

## 4. Montar o digest

### Formato de saída (chat)

```
## Alarm Digest — GF Core
**Período:** {window_start_br} → {window_end_br}

### 🔴 P1 — Ação imediata (#alarms-gf-core)
Disparados: N  |  Ativos: N  |  Flapping: N  |  Recuperados: N

#### Ativos
| Monitor | ID | Desde | Ciclos | Link |
|---------|-----|-------|--------|------|

#### Flapping
| Monitor | ID | Ciclos | Link |
|---------|-----|--------|------|

#### Recuperados
| Monitor | ID | Disparou | Link |
|---------|-----|---------|------|

---

### 🟠 P2/P3 — Sinais de melhoria (#notifications-gf-core)
Disparados: N  |  Ativos: N  |  Flapping: N  |  Recuperados: N

#### Ativos
| Monitor | ID | Desde | Link |
|---------|-----|-------|------|

#### Flapping
| Monitor | ID | Ciclos | Link |
|---------|-----|--------|------|

#### Recuperados
| Monitor | ID | Disparou | Link |
|---------|-----|---------|------|

---

### ⚫ No Data — monitors possivelmente mortos  [apenas --triage]
| Canal | Monitor | ID | Último dado | Recomendação |
|-------|---------|-----|------------|-------------|

---

### Recomendações
- **P1 🔴 [nome]**: ação necessária — descrição
- **P1 🔁 [nome]**: calibrar threshold — sugestão
- **P2 🔁 [nome]**: sinal recorrente — avaliar melhoria
- **⚫ [nome]**: desativar ou corrigir métrica
```

Horários sempre em BRT (America/Sao_Paulo, -03:00).
Links DD: `https://app.datadoghq.com/monitors/{ID}`

Se zero monitores dispararam em ambos os canais: `Nenhum alerta no período {window_start_br} → {window_end_br}. ✅`

---

## 5. Salvar no docs.db

```bash
wtb doc add \
  --type discovery \
  --title "Alarm Digest GF Core — {YYYY-MM-DD}" \
  --date {YYYY-MM-DD} \
  --content "..."
```

Conteúdo: mesma estrutura do chat, em markdown.
Após salvar: `Salvo: wtb doc get <id-gerado>`

---

## 6. Atualizar o state

```bash
wtb memory set alarm-digest-last-run "{window_end_iso}" \
  --type timestamp \
  --topic gf-core \
  --desc "Último alarm digest executado"
```

**Só atualizar após o digest ser salvo com sucesso.**

---

## 7. Contexto fixo

| Campo | Valor |
|:------|:------|
| P1 canal | `#alarms-gf-core` (`C0ARTSY4WGJ`) — ação imediata, ~94 monitors |
| P2/P3 canal | `#notifications-gf-core` (`C0ASN5KTLE4`) — sinais, ~35 monitors |
| Ambos criados | 2026-04-09 07:31–07:32 BRT |
| DD API key | Keychain `workflow-dd-api-key` |
| DD App key | Keychain `workflow-dd-app-key` |
| Monitor search endpoint | `GET /api/v1/monitor/search?query=notification:slack-{CANAL}&per_page=100` |
| Flapping endpoint | `GET /api/v1/events?start=X&end=Y&sources=alert&tags=service:{SVC}&per_page=50` |

---

## 8. Monitores de referência (contexto histórico)

**P1 — monitors que já flapparam:**
- cerberus-api latency (105695424) — 5× em 3 dias, threshold p50 > 500ms apertado
- blueprint-api error rate (254781549) — 8× em 3 dias, atualmente em ALERT

**P2/P3 — monitors mais ativos recentemente:**
- atlas-api p90 latency (38686883) — dispara esporadicamente
- icarus-v1 latency × 2 (108010132, 105003187) — latência de endpoints de eventos
- herbie-api-dashboard latency (93165620) — picos de latência no dashboard
- webhook-sender fleet slot timeout (266094623) — semáforo saturado

**Monitors No Data (candidatos a limpeza):**
- P1: Fusca RDS CPU Credit Balance (142000588) — sem dados desde fev/2025
- P1: [fusca] DLT consumer replay_error (263404710) — nunca teve dados
- P2: [fusca] DLT consumer replay ativo (263404711) — nunca teve dados
- P2: Resource post /vehicle-groups error rate (174354342) — nunca teve dados

---

## Dependências

- `wtb memory set/list` (state persistence)
- `wtb doc add` (salvar digest)
- `DD_API_KEY` + `DD_APP_KEY` via Keychain
- Python 3 (parse de timestamps)
