---
name: add-alert-event
description: >
  Wizard Socrático para adicionar um novo tipo de evento de alerta na Nova Experiência
  (squad GF). Cobre todos os repos afetados — trigger-action-api, blueprint-api,
  trigger-engine, analytics-report-api, herbie-dashboard — com decisões 1-a-1, tradeoffs
  pontuados e sugestão justificada. Invocar antes de implementar qualquer novo evento (GF-XX).
argument-hint: "<nome do evento> [--implement] [--report]"
user-invocable: true
---

# add-alert-event — Novo Evento de Alerta (Nova Experiência)

Wizard Socrático para adicionar um novo evento à **Nova Experiência de Alertas de Política de Frotas**.

- Sem `--implement`: modo wizard — faz as perguntas, documenta decisões, produz plano.
- Com `--implement`: após decisões confirmadas, executa o plano completo e mantém tracking file.
- Com `--report`: lê o tracking file do evento e gera relatório de timeline + lições + melhorias.

**Após cada decisão confirmada**, exibir o tracker de progresso:

```
## Progresso
✅ D1 — Atributo(s): <escolha>
✅ D2 — source: <escolha>
⬜ D3 — UX multiselect
⬜ D4 — emitter_type
⬜ D5 — can_request_video
⬜ D6 — requiresDevice
⬜ D7 — columns_report
⬜ D8 — meta.eventType
```

Não avançar para a próxima decisão sem confirmação explícita do usuário.

---

## Fase 0 — Reconnaissance (sempre automática, sem perguntar)

Execute em paralelo antes de qualquer pergunta:

```bash
# 1. Última migration existente (número + nome)
ls ~/Cobliteam/trigger-action-api/trigger-action-domain/src/main/resources/db/migration/ \
  | sort | tail -5

# 2. Eventos já cadastrados (referência de padrão)
wtb db query fusca \
  "SELECT name, name_identifier, event_emitter_type, can_request_video,
          conditions->>'conditions' as conds_preview
   FROM events ORDER BY created_at"

# 3. Atributos válidos no trigger-engine
cat ~/Cobliteam/trigger-action-api/trigger-action-domain/src/main/resources/schema/common/allTriggerEngineAttributes.json

# 4. AlertType enum + definitions existentes no blueprint
grep -n "AlertType\|VEHICLE_MOVEMENT\|FATIGUE\|DANGEROUS\|DISTRACTED\|getDefinition" \
  ~/Cobliteam/blueprint/blueprint-api/src/main/kotlin/co/cobli/blueprint/application/usecases/CreateAlertViewUseCase.kt \
  | head -50

# 5. I18n messages existentes (referência de padrão de chaves)
grep -n "create\.alert\.event\.\|create\.alert\.condition\." \
  ~/Cobliteam/blueprint/blueprint-api/src/main/resources/internationalization/messages.properties \
  | head -40

# 6. Estado atual de meta.eventType no TriggerEvalProcessor
grep -n "eventType\|causedBy\|riskEventType" \
  ~/Cobliteam/trigger-action-api/trigger-engine/src/main/kotlin/co/cobli/trigger/engine/processor/TriggerEvalProcessor.kt \
  | head -20
```

Registre mentalmente:
- Número da próxima migration (última + 1)
- Atributos disponíveis no trigger-engine
- Padrão de chaves I18n existentes (para manter consistência)
- AlertType enum values já definidos
- **D8 status:** se grep do item 6 retornar `?: riskEventType?.name` → D8 **fechado** — `meta.eventType` já populado para ambos os campos. Pular decisão D8 e documentar como resolvido no tracker.

---

## Fase 1 — Decisões Socráticas (1 pergunta por vez)

Para cada decisão:
1. Apresente o **contexto** com o que o codebase mostra (2–3 linhas)
2. Apresente as **opções** com score (0–10 por: simplicidade, consistência com padrão, manutenibilidade)
3. Dê sua **sugestão** com justificativa
4. Aguarde confirmação antes de avançar
5. Exiba tracker atualizado após confirmação

---

### D1 — Atributo(s) do trigger-engine + valores

**Contexto arquitetural:**
O trigger-engine processa eventos como `UnifiedThingType` — estrutura que unifica dois
tópicos Kafka em um único objeto, com campos **mutuamente exclusivos**:
- `riskEventType: EventType?` → populado por eventos de `risk-event-v2` (marcado `@field:OneShot`)
- `causedBy: VideoEventPB.eventType?` → populado por eventos de `filtered-video-event` (marcado `@field:OneShot`)

`@field:OneShot` garante que esses campos são **limpos** antes de carry-forward entre janelas de 30s
e antes de persistência em state (MergeDataProcessor.clearOneShots()). Cada evento é avaliado
com valores frescos — sem contaminação de eventos anteriores.

Os `triggerEngineAttributes` no conditions JSONB são mapeamentos para esses campos:
- `RISK_EVENT_TYPE` → avalia `unifiedThing.riskEventType`
- `CAUSED_BY` → avalia `unifiedThing.causedBy` (`toFieldName("CAUSED_BY")` = `"causedBy"`)

Quando um evento de risco chega: `riskEventType` está preenchido, `causedBy` é null.
Quando um evento de vídeo chega: `causedBy` está preenchido, `riskEventType` é null.

Uma condição que avalia um campo null retorna `false` — o OR expression avalia a próxima normalmente.

**Opções de modelagem:**

| Opção | Condições JSONB | Quando usar | Simplici. | Consist. | Total |
|-------|----------------|-------------|-----------|----------|-------|
| A — 1 condição RISK_EVENT_TYPE | 1 | Todos os valores existem em risk-event-v2 | 10 | 9 | **19** |
| B — 1 condição VIDEO_CAUSED_BY | 1 | Todos os valores são exclusivos de filtered-video-event | 10 | 9 | **19** |
| C — 2 condições com OR | 2 (RISK + VIDEO) | Valores distribuídos entre os dois campos | 6 | 8 | **14** |

> **Sugestão:** A ou B quando todos os valores estão no mesmo campo. C quando os valores
> se distribuem pelos dois campos (ex: TAILGATING só em causedBy, HARSHACCELERATION só em riskEventType).

**CRÍTICO — Verificar quais valores existem em cada campo:**
```bash
# O que existe em risk-event-v2 (riskEventType)?
wtb db query snowflake \
  "SELECT event_type, COUNT(*) FROM DATA_PLATFORM.SILVER.RISK_EVENTS
   WHERE event_type IN ('TAILGATING','DISTRACTEDDRIVING','SMOKING','PHONEUSAGE',
                        'HARSHACCELERATION','HARSHCORNERING','HARSHBREAKING')
   AND created_at > DATEADD(day,-30,CURRENT_TIMESTAMP)
   GROUP BY 1 ORDER BY 2 DESC"
```

**⚠️ Gotcha confirmado:** TAILGATING = 0 ocorrências em RISK_EVENTS (30 dias Snowflake, 2026-04-07).
Pipeline descarta TAILGATING — só existe em `causedBy` (CAUSED_BY).

**⚠️ Normalização riskeventsv2 — não confundir proto names com pipeline values:**
O proto `video/events.proto` define `HARSHACCELERATION`, `HARSHCORNERING`, `HARSHBREAKING` como `causedBy` brutos.
Porém o pipeline riskeventsv2 **normaliza** esses eventos de câmera para o schema de sensor:
- `HARSHACCELERATION` → emitido como `FAST_ACCELERATION_35/45/55` em RISK_EVENTS
- `HARSHCORNERING` → emitido como `SPEEDY_TURN_LEFT/RIGHT` em RISK_EVENTS
- `HARSHBREAKING` → emitido como `HARD_BREAK_35/45/55` em RISK_EVENTS
- `TAILGATING` → **exceção**: filtrado fora do riskeventsv2, chega apenas como `causedBy`

Consequência: adicionar HARSH* ao `possibleValues` de `CAUSED_BY` criaria condições mortas — nunca disparariam em prod.
Regra: **sempre validar via Snowflake** (`DATA_PLATFORM.SILVER.RISK_EVENTS`, 30 dias) antes de adicionar valores a `possibleValues`.

**Pergunta D1:** Quais valores de evento este alerta precisa cobrir?
Para cada um, em qual campo do UnifiedThingType ele aparece?

---

### D2 — source: USER vs INTERNAL por condição

**Contexto:** Cada condição JSONB tem campo `source`:

**`source=USER`** → blueprint renderiza multiselect:
1. `possibleValues` no DB lista as opções disponíveis
2. blueprint chama trigger-action-api, recebe `possibleValues`
3. `enrichWithApiConditions()` injeta `multiSelectOptions: [{value, label}]`
4. herbie-dashboard renderiza `InlineSentenceField` com widget `MULTISELECT`
5. Label de cada opção vem de I18n: `create.alert.condition.<paramKey>.option.<VALUE>`

**`source=INTERNAL`** → blueprint usa `conditionsDescription` (texto fixo, sem widget).

**⚠️ Bug crítico com source=USER condição opcional:** Se o usuário não seleciona nada,
`buildConditionMap()` retorna `null` → trigger inteiro quebra. Toda condição `source=USER`
deve ter `conditionRequired=true` ou garantia de seleção mínima.

**Padrão Label-Group Routing — para eventos com valores em campos diferentes do UnifiedThingType:**

Duas condições no DB (ambas `source=USER`), um único ConditionSpec no blueprint com
`triggerEngineAttributes: List<String>`. O blueprint faz o merge automático de `possibleValues`
e o roteamento via `valueToConditionId`.

```
DB:
  C1: RISK_EVENT_TYPE, source=USER, possibleValues=[FAST_ACCELERATION_35, ..., HARD_BREAK_55]
  C2: CAUSED_BY,  source=USER, possibleValues=[TAILGATING]
  Expression: C1 OR C2

blueprint ConditionSpec (único):
  triggerEngineAttributes = listOf("RISK_EVENT_TYPE", "CAUSED_BY")
  paramKey = "dangerous_driving_events"
  widget = MULTISELECT

Fluxo blueprint:
  resolveConditionMappings()  → filtra condições cujo triggerEngineAttribute está na lista
                                → constrói valueToConditionId: {value → conditionId}
  enrichWithApiConditions()   → mescla possibleValues de ambas as condições em multiSelectOptions flat
  reverseConditionParams()    → edit mode: reconstitui paramKey a partir de {conditionId: [vals]}

Invariante: cada value deve estar no possibleValues de no máximo uma condição por evento.
Violação = IllegalStateException no startup do blueprint — nunca silencioso.
```

**⚠️ `optionGroups` é SOMENTE roteamento interno — nunca UI:**

`ConditionSpec.optionGroups` (se presente) serve exclusivamente para construir `valueToConditionId`:
qual value vai para qual `conditionId` no DB. Ele **nunca** é serializado para o herbie-dashboard.

`enrichWithApiConditions()` sempre emite `multiSelectOptions` no formato **flat**:
```
{value: String, label: String}  ← CORRETO (herbie-dashboard espera este formato)
{value: List<String>, label: String}  ← ERRADO (grouped — herbie-dashboard não suporta)
```

Regra: mesmo com `triggerEngineAttributes` (2+ condições), `multiSelectOptions` é sempre flat — uma
entrada por valor individual, com label I18n individual. `reverseConditionParams()` usa
`valueToConditionId` para reconstituir `{conditionId: [vals]}` no modo edit, sem grouped UI.

**⚠️ Atributo correto confirmado:** `CAUSED_BY` é o nome válido no schema (`allTriggerEngineAttributes.json`) e mapeia para `causedBy` via `toFieldName()` no trigger-engine. `VIDEO_CAUSED_BY` **não existe** — causaria condição sempre false.

**Opções:**

| Opção | source | Widget UI | Flexib. | Simplici. | Segurança | Total |
|-------|--------|-----------|---------|-----------|-----------|-------|
| A — USER único campo | USER | Multiselect flat | Alta | 9 | 9 | **27** |
| B — INTERNAL para tudo | INTERNAL | Sem widget | Nenhuma | 10 | 10 | **28** |
| C — Label-Group Routing (2x USER + blueprint triggerEngineAttributes) | USER+USER | 1 multiselect unificado | Alta | 8 | 10 | **28** |

> **Sugestão:** A quando todos os valores estão em um único campo. C quando os valores
> se distribuem por campos diferentes. B quando o produto não quer configuração pelo usuário.

**Pergunta D2:** O usuário vai selecionar quais tipos de evento? Se sim, todos os valores
cobertos existem em um único campo do UnifiedThingType?

---

### D3 — UX: Flat multiselect vs Grouped multiselect (apenas se source=USER + ENUM)

**Contexto:** O blueprint atualmente suporta `MULTISELECT` flat apenas.
Grouped multiselect (categorias visuais) não existe — exigiria PR em blueprint + herbie-dashboard.

**Opções:**

| Opção | UX | Esforço blueprint-api | Esforço herbie-dashboard | Total |
|-------|----|-----------------------|--------------------------|-------|
| A — Flat multiselect (sem grupos) | Lista plana | 0 (já existe) | 0 | **Zero** |
| B — Grouped multiselect (novo) | Agrupado por categoria | Médio | Médio | **Alto** |
| C — Múltiplos multiselects (1 por grupo) | Campos separados | Baixo | 0 | **Baixo** |

> **Sugestão:** A para V1 — flat multiselect com labels descritivos é suficiente e zero esforço.

**Pergunta D3:** Uma lista plana é suficiente ou o produto especificou agrupamento por categoria?

---

### D4 — event_emitter_type: SPOT vs START_END

**Contexto:**
- `SPOT`: evento instantâneo — 1 notificação por ocorrência.
- `START_END`: evento com duração — notifica na abertura e no fechamento.

> **Sugestão:** SPOT para 99% dos eventos de telemetria de risco. START_END apenas se
> o produto especificou "alerta quando X acontecer por mais de N minutos".

**Pergunta D4:** Este evento representa uma ocorrência instantânea ou tem duração?

---

### D5 — can_request_video

**Diretiva (não perguntar):** `can_request_video = false` é **obrigatório para todos os eventos** até decisão explícita do time GF Core.
Definição do time GF Core (2026-04-20): nenhum evento deve habilitar solicitação de vídeo no momento atual.
Registrar D5 = `false` automaticamente sem perguntar ao usuário.

---

### D6 — requiresDevice / "Exclusivo Cobli Cam" (apenas se cam-only)

**Contexto:** Tag "Exclusivo Cobli Cam" no formulário. Metadata do blueprint-api (sem coluna no banco).

Verificar se já existe em `AlertTypeDefinition`:
```bash
grep -n "requiresDevice\|DeviceRequirement" \
  ~/Cobliteam/blueprint/blueprint-api/src/main/kotlin/co/cobli/blueprint/domain/model/AlertTypeDefinition.kt
```

> **Sugestão:** Implementar como campo opcional em AlertTypeDefinition. Não adicionar coluna no banco.

**Pergunta D6:** Este evento deve ser restrito a frotas com Cobli Cam?

---

### D7 — columns_report (colunas do Export Excel)

**Contexto:** `columns_report` JSONB define colunas do Export Excel via `DynamicTriggerEventsExcelSerializer`.

**Schema correto** (confirmado em `analytics-report-shared/.../dynamic/ColumnDefinition.kt`):
```json
{
  "version": "v1.0",
  "columns": [
    {"valuePath": "meta.address",   "label": "Endereço",        "columnType": "TEXT",     "order": 0},
    {"valuePath": "meta.videoUrl",  "label": "Vídeo",           "columnType": "URL",      "order": 1},
    {"valuePath": "meta.eventType", "label": "Tipo de evento",  "columnType": "TEXT",     "order": 2}
  ]
}
```

**⚠️ Campos que NÃO existem em TriggerEventMeta** — nunca incluir:
- `meta.location` — não existe
- `meta.speed` — não existe

**Campos disponíveis em `resolveValue()`** (DynamicTriggerEventsExcelSerializer):
- `meta.<campo>` → parsed do JSON da coluna `meta` em `trigger_events`
- `triggerEvent.meta.<campo>` → alias de `meta.*`
- `trigger.<campo>` → campos do objeto `Trigger` (ex: `trigger.name`)

Tipos suportados: `TEXT`, `NUMBER`, `DECIMAL`, `DATETIME`, `URL`.

**Tradução de `meta.eventType`:** valores são traduzidos por `translateEventType()` no serializer.
Ao adicionar novo evento, **sempre** adicionar o case em `translateEventType()`:
```kotlin
// analytics-report-consumer/.../DynamicTriggerEventsExcelSerializer.kt
"FAST_ACCELERATION_35", "FAST_ACCELERATION_45", "FAST_ACCELERATION_55" -> "Aceleração brusca"
"SPEEDY_TURN_LEFT", "SPEEDY_TURN_RIGHT" -> "Curva perigosa"
"HARD_BREAK_35", "HARD_BREAK_45", "HARD_BREAK_55" -> "Frenagem brusca"
"TAILGATING" -> "Distância insegura"
// Valores sem case retornam o próprio valor bruto — sem crash
```

**`meta.eventType`:** verificar Fase 0 item 6 antes de decidir. Se `?: riskEventType?.name` já existe no TriggerEvalProcessor → campo populado para ambos os campos, pode incluir no columns_report sem pendência. Só é gap se o fix ainda não estiver na branch.

**Pergunta D7:** Quais colunas além das padrão (data/hora, veículo, motorista, endereço, localização)
devem aparecer no Export Excel? "Tipo de evento" e "Link para vídeo" são necessários?

---

### D8 — meta.eventType: verificar estado atual primeiro (Fase 0 item 6)

**⚠️ Antes de qualquer decisão:** executar o grep da Fase 0 item 6. O resultado determina o ponto de partida.

**Estado com fix aplicado (Fase 0 retorna `?: riskEventType?.name`):**

```kotlin
eventType = unifiedData.data?.causedBy?.name ?: unifiedData.data?.riskEventType?.name,
```

| Evento fonte | meta.eventType |
|-------------|----------------|
| `filtered-video-event` (causedBy) | **Populado** — ex: "TAILGATING" |
| `risk-event-v2` (riskEventType) | **Populado** — ex: "FAST_ACCELERATION_35" |

→ D8 fechado. Nenhuma ação necessária. Pular esta decisão; documentar como resolvido no tracker.
→ Testes de integração devem usar `assertEquals("<VALOR>", results.single().event.meta.eventType)` — nunca `assertNull`.

---

**Estado sem fix (Fase 0 retorna apenas `causedBy?.name`):**

```kotlin
eventType = unifiedData.data?.causedBy?.name,  // riskEventType = null
```

| Evento fonte | meta.eventType |
|-------------|----------------|
| `filtered-video-event` (causedBy) | **Populado** |
| `risk-event-v2` (riskEventType) | **null** |

Fix: adicionar `?: unifiedData.data?.riskEventType?.name`. `TriggerEvent` é output do Flink (não estado) — zero risco Kryo.

**Opções (apenas se fix não está na branch):**

| Opção | Impl. | Risco | Valor UX | Total |
|-------|-------|-------|----------|-------|
| A — Corrigir agora | 1 linha | Zero | Alto | **30** |
| B — Adiar para V2 | Zero | Zero | Nenhum agora | **20** |

> **Sugestão:** A — 1 linha, zero risco.

**Pergunta D8:** "Tipo de evento" é requisito V1? Se sim, corrigir agora (se ainda não está na branch).

---

## Fase 2 — Validação de consistência (automática antes do plano)

```bash
# 1. Atributos existem no schema?
cat ~/Cobliteam/trigger-action-api/trigger-action-domain/src/main/resources/schema/common/allTriggerEngineAttributes.json \
  | python3 -c "import json,sys; d=json.load(sys.stdin); [print('OK:', a) if a in d else print('ERRO:', a, 'não existe') for a in ['RISK_EVENT_TYPE','CAUSED_BY']]"

# 2. Migration number está livre?
NEXT=$(ls ~/Cobliteam/trigger-action-api/trigger-action-domain/src/main/resources/db/migration/ \
  | sort | tail -1 | grep -oP '^V\d+' | python3 -c "import sys; v=sys.stdin.read().strip(); print(f'V{int(v[1:])+1:03d}')")
echo "Próxima migration: $NEXT"
ls ~/Cobliteam/trigger-action-api/trigger-action-domain/src/main/resources/db/migration/ \
  | grep "^$NEXT" && echo "CONFLITO" || echo "OK — $NEXT livre"

# 3. eventNameIdentifier existe no DB?
wtb db query fusca "SELECT name_identifier FROM events WHERE name_identifier = '<NAME_IDENTIFIER>'"

# 4. AlertType enum não conflita?
grep "<NOME_EVENTO>" \
  ~/Cobliteam/blueprint/blueprint-api/src/main/kotlin/co/cobli/blueprint/domain/model/AlertType.kt

# 5. Campos do columns_report existem no schema ColumnDefinition? (obrigatório antes de escrever migration)
cat ~/Cobliteam/analytics-report-api/analytics-report-shared/src/main/kotlin/co/cobli/analytics/report/api/models/trigger/events/dynamic/ColumnDefinition.kt
# Verificar: order: Int (obrigatório, sem default), columnType enum values
cat ~/Cobliteam/analytics-report-api/analytics-report-shared/src/main/kotlin/co/cobli/analytics/report/api/models/trigger/events/dynamic/ColumnsReport.kt
# Verificar: version: String (obrigatório)

# 6. Cada valuePath existe como campo real? (cross-check com TriggerEventMeta + resolveValue())
grep -n "meta\.\|triggerEvent\.\|trigger\." \
  ~/Cobliteam/analytics-report-api/analytics-report-consumer/src/main/kotlin/co/cobli/analytics/report/api/serializer/trigger/events/DynamicTriggerEventsExcelSerializer.kt \
  | grep "valuePath\|startsWith\|removePrefix" | head -20
# meta.address ✓ / meta.eventType ✓ / meta.videoUrl ✓
# meta.speed ✗ (não existe) / meta.location ✗ (não existe)
```

**Checklist de coerência obrigatório para columns_report antes de commitar migration:**

| Campo | Verificação | Consequência de erro |
|-------|-------------|---------------------|
| `"version": "v1.0"` | Obrigatório no JSON | `JsonMappingException` ao desserializar |
| `"order": N` em cada coluna | `ColumnDefinition.order: Int` sem default | `JsonMappingException` ao desserializar |
| `valuePath` existe em `resolveValue()` | Ler `DynamicTriggerEventsExcelSerializer.kt` | Coluna sempre vazia no Excel |
| `columnType` é valor válido do enum | Ler `ColumnType.kt` | `JsonMappingException` ao desserializar |
| `translateEventType()` tem case para cada value | Ler serializer | Valor bruto (ex: "TAILGATING") no Excel |

---

## Fase 3 — Plano de implementação

Após todas as decisões confirmadas, gerar plano no formato abaixo:

```markdown
# Plano — <NOME_EVENTO>

## Tracker de decisões

| # | Decisão | Escolha | Justificativa |
|---|---------|---------|---------------|
| D1 | Atributo(s) | RISK_EVENT_TYPE / VIDEO_CAUSED_BY / dual | ... |
| D2 | source | USER / INTERNAL | ... |
| D3 | UX multiselect | Flat / Grouped | ... |
| D4 | emitter_type | SPOT / START_END | ... |
| D5 | can_request_video | true / false | ... |
| D6 | requiresDevice | COBLI_CAM / null | ... |
| D7 | columns_report | lista de colunas | ... |
| D8 | meta.eventType | corrigir agora / adiar | ... |
```

---

## PR 1 — trigger-action-api (migration SQL)

**Regra obrigatória — 1 PR por repo:** antes de criar qualquer PR, verificar se já existe um aberto:
```bash
gh pr list --repo Cobliteam/trigger-action-api --state open --search "<event-slug>"
gh pr list --repo Cobliteam/blueprint --state open --search "<event-slug>"
gh pr list --repo Cobliteam/analytics-report-api --state open --search "<event-slug>"
gh pr list --repo Cobliteam/herbie-dashboard --state open --search "<event-slug>"
```
Se encontrar PR aberto → empurrar as novas mudanças no branch existente, **nunca abrir segundo PR**.

**Path correto:** `trigger-action-domain/src/main/resources/db/migration/`
Branch: `feat/gf-XX-add-<event-slug>`

```sql
-- Idempotente: re-executar sem efeito se o evento já existir
INSERT INTO events (id, name, name_identifier, group_name, enabled,
                    can_request_video, event_emitter_type, conditions, columns_report)
SELECT
  gen_random_uuid(),
  '<Nome Legível>',
  '<NAME_IDENTIFIER>',        -- UPPERCASE, ex: 'DANGEROUS_DRIVING'
  'RISCO',
  true,
  false,                      -- can_request_video = false OBRIGATÓRIO (diretiva Reyes 2026-04-20)
  'SPOT',
  '{
    "version": "v1.0",
    "conditions": [
      {
        "id": "<uuid-fixo>",  -- gerar via gen_random_uuid() mas fixar no arquivo (requer idempotência)
        "conditionType": "ENUM",
        "triggerEngineAttribute": "<ATTR>",
        "operator": "IN",
        "source": "USER",
        "possibleValues": ["<VAL1>", "<VAL2>"],
        "conditionRequired": false,
        "order": 0
      }
    ],
    "expression": [
      {"type": "CONDITION", "conditionId": "<mesmo-uuid-fixo>"}
    ]
  }',
  '{
    "version": "v1.0",
    "columns": [
      {"valuePath": "meta.address",   "label": "Endereço",       "columnType": "TEXT",     "order": 0},
      {"valuePath": "meta.eventType", "label": "Tipo de evento", "columnType": "TEXT",     "order": 1},
      {"valuePath": "meta.videoUrl",  "label": "Vídeo",          "columnType": "URL",      "order": 2}
    ]
  }'
WHERE NOT EXISTS (SELECT 1 FROM events WHERE name_identifier = '<NAME_IDENTIFIER>');
```

Para **Label-Group Routing** (valores distribuídos entre RISK_EVENT_TYPE e VIDEO_CAUSED_BY):
```sql
-- 2 condições USER com OR. blueprint usa triggerEngineAttributes: List<String> no ConditionSpec.
-- INVARIANTE: nenhum value pode aparecer em possibleValues de duas condições do mesmo evento.
'{
  "version": "v1.0",
  "conditions": [
    {
      "id": "<uuid-c1>",
      "conditionType": "ENUM",
      "triggerEngineAttribute": "RISK_EVENT_TYPE",
      "operator": "IN",
      "source": "USER",
      "possibleValues": [
        "FAST_ACCELERATION_35","FAST_ACCELERATION_45","FAST_ACCELERATION_55",
        "SPEEDY_TURN_LEFT","SPEEDY_TURN_RIGHT",
        "HARD_BREAK_35","HARD_BREAK_45","HARD_BREAK_55"
      ],
      "conditionRequired": false,
      "order": 0
    },
    {
      "id": "<uuid-c2>",
      "conditionType": "ENUM",
      "triggerEngineAttribute": "CAUSED_BY",
      "operator": "IN",
      "source": "USER",
      "possibleValues": ["TAILGATING"],
      "conditionRequired": false,
      "order": 1
    }
  ],
  "expression": [
    {"type": "CONDITION", "conditionId": "<uuid-c1>"},
    {"type": "OPERATOR",  "operator": "OR"},
    {"type": "CONDITION", "conditionId": "<uuid-c2>"}
  ]
}'
WHERE NOT EXISTS (SELECT 1 FROM events WHERE name_identifier = '<NAME_IDENTIFIER>');
```

**Teste pós-migration:**
```bash
wtb db query fusca "SELECT name, name_identifier, conditions FROM events WHERE name_identifier = '<NAME_IDENTIFIER>'"
```

### JSON fixture obrigatório para JsonSchemaValidationTest

**Por que:** `JsonSchemaValidationTest` percorre todos os arquivos em
`trigger-action-domain/src/main/resources/events/` e valida cada um contra o schema.
Sem o arquivo, o teste não falha (apenas ignora), mas o padrão do projeto é ter um JSON
por evento para documentar e validar as condições.

**Criar junto com a migration** em:
`trigger-action-domain/src/main/resources/events/conditions/v1.0_<event-slug>.json`

O JSON deve espelhar exatamente as condições da migration (mesmos UUIDs):

```json
{
  "version": "v1.0",
  "conditions": [
    {
      "id": "<uuid-fixo-da-migration>",
      "conditionType": "ENUM",
      "triggerEngineAttribute": "<ATTR>",
      "operator": "IN",
      "source": "USER",
      "possibleValues": ["<VAL1>", "<VAL2>"],
      "conditionRequired": false,
      "order": 0
    }
  ],
  "expression": [
    {"type": "CONDITION", "conditionId": "<uuid-fixo-da-migration>"}
  ]
}
```

**Para Label-Group Routing** (2 condições + OR):
```json
{
  "version": "v1.0",
  "conditions": [
    {"id": "<uuid-c1>", "conditionType": "ENUM", "triggerEngineAttribute": "RISK_EVENT_TYPE",
     "operator": "IN", "source": "USER", "possibleValues": ["..."], "conditionRequired": false, "order": 0},
    {"id": "<uuid-c2>", "conditionType": "ENUM", "triggerEngineAttribute": "CAUSED_BY",
     "operator": "IN", "source": "USER", "possibleValues": ["TAILGATING"], "conditionRequired": false, "order": 1}
  ],
  "expression": [
    {"type": "CONDITION", "conditionId": "<uuid-c1>"},
    {"type": "OPERATOR", "operator": "OR"},
    {"type": "CONDITION", "conditionId": "<uuid-c2>"}
  ]
}
```

**Validar após criar:**
```bash
./gradlew :trigger-action-domain:test --tests "co.cobli.trigger.action.domain.schema.JsonSchemaValidationTest" --rerun-tasks --daemon -q
```

---

## PR 2 — blueprint-api (AlertType + AlertTypeDefinition + I18n)

Branch: `feat/gf-XX-blueprint-<event-slug>`

### 2a. AlertType enum (se não existir)
Arquivo: `blueprint-api/src/main/kotlin/co/cobli/blueprint/domain/model/AlertType.kt`

### 2b. AlertTypeDefinition em CreateAlertViewUseCase.getDefinition()

**Padrão single-field (source=USER, 1 condição):**
```kotlin
AlertType.<NOVO_EVENTO> -> AlertTypeDefinition(
    icon = "<icon-name>",
    defaultTitle = "<Título do Alerta>",
    defaultDescription = "<Descrição para o usuário>",
    eventNameIdentifier = "<NAME_IDENTIFIER>",   // UPPERCASE — igual ao name_identifier no DB
    mandatoryConditions = listOf(
        ConditionRow(listOf(
            ConditionSpec(
                id = "field_<param_key>",
                paramKey = "<param_key>",
                sentenceTextKey = "create.alert.condition.<param_key>.text",
                defaultSentenceText = "Alertar quando o tipo de evento for",
                schemaType = ConditionSchemaType.ARRAY,
                widget = ConditionWidget.MULTISELECT,
                conditionType = "ENUM",
                widgetOptions = mapOf("placeholder" to "Selecionar tipos de evento"),
            ),
        )),
    ),
    targetEntities = listOf(
        TargetEntitySpec(key = "vehicles", entity = EntityType.VEHICLE, icon = "truck"),
        TargetEntitySpec(key = "groups",   entity = EntityType.VEHICLE_GROUP, icon = "layers"),
    ),
)
```

**Padrão Label-Group Routing (2 condições USER, 1 ConditionSpec, triggerEngineAttributes):**
```kotlin
AlertType.<NOVO_EVENTO> -> AlertTypeDefinition(
    icon = "<icon-name>",
    defaultTitle = "<Título do Alerta>",
    defaultDescription = "<Descrição para o usuário>",
    eventNameIdentifier = "<NAME_IDENTIFIER>",   // UPPERCASE — igual ao name_identifier no DB
    mandatoryConditions = listOf(
        ConditionRow(listOf(
            ConditionSpec(
                id = "field_<param_key>",
                paramKey = "<param_key>",
                sentenceTextKey = "create.alert.condition.<param_key>.text",
                defaultSentenceText = "Alertar quando o tipo de evento for",
                schemaType = ConditionSchemaType.ARRAY,
                widget = ConditionWidget.MULTISELECT,
                // triggerEngineAttributes (lista) em vez de conditionType (string):
                // resolveConditionMappings() filtra condições do DB por triggerEngineAttribute ∈ lista
                // enrichWithApiConditions() mescla possibleValues de todas em um flat multiSelectOptions
                // valueToConditionId faz o roteamento value → conditionId correto
                triggerEngineAttributes = listOf("RISK_EVENT_TYPE", "CAUSED_BY"),
                widgetOptions = mapOf("placeholder" to "Selecionar eventos de risco"),
            ),
        )),
    ),
    targetEntities = listOf(
        TargetEntitySpec(key = "vehicles", entity = EntityType.VEHICLE, icon = "truck"),
        TargetEntitySpec(key = "groups",   entity = EntityType.VEHICLE_GROUP, icon = "layers"),
    ),
)
// Nota: conditionType NÃO deve ser especificado junto com triggerEngineAttributes — são mutuamente exclusivos.
```

**Padrão source=INTERNAL (conditionsDescription):**
```kotlin
AlertType.<NOVO_EVENTO> -> AlertTypeDefinition(
    icon = "<icon-name>",
    defaultTitle = "<Título do Alerta>",
    defaultDescription = "<Descrição para o usuário>",
    eventNameIdentifier = "<NAME_IDENTIFIER>",
    conditionsDescription = "<Descrição fixa do que é monitorado>",
    mandatoryConditions = emptyList(),
    optionalConditions = emptyList(),
    targetEntities = listOf(...),
)
```

### 2c. I18n — messages.properties (e messages_en_US.properties)
Arquivo: `blueprint-api/src/main/resources/internationalization/messages.properties`

```properties
# Evento
create.alert.event.<eventSlug>.title=<Título Legível>
create.alert.event.<eventSlug>.description=<Descrição do alerta>

# Condição (apenas para source=USER)
create.alert.condition.<paramKey>.text=Alertar quando o tipo de evento for
create.alert.condition.<paramKey>.placeholder=Selecionar tipos de evento

# Opções do ENUM (1 linha por value de possibleValues)
create.alert.condition.<paramKey>.option.<VAL1>=<Label Legível 1>
create.alert.condition.<paramKey>.option.<VAL2>=<Label Legível 2>
```

**Exemplos de labels validados:**
```properties
create.alert.condition.dangerous_driving_events.option.FAST_ACCELERATION_35=Aceleração brusca (35 km/h)
create.alert.condition.dangerous_driving_events.option.FAST_ACCELERATION_45=Aceleração brusca (45 km/h)
create.alert.condition.dangerous_driving_events.option.FAST_ACCELERATION_55=Aceleração brusca (55 km/h)
create.alert.condition.dangerous_driving_events.option.SPEEDY_TURN_LEFT=Curva perigosa (esquerda)
create.alert.condition.dangerous_driving_events.option.SPEEDY_TURN_RIGHT=Curva perigosa (direita)
create.alert.condition.dangerous_driving_events.option.HARD_BREAK_35=Frenagem brusca (35 km/h)
create.alert.condition.dangerous_driving_events.option.HARD_BREAK_45=Frenagem brusca (45 km/h)
create.alert.condition.dangerous_driving_events.option.HARD_BREAK_55=Frenagem brusca (55 km/h)
create.alert.condition.dangerous_driving_events.option.TAILGATING=Distância insegura
```

**Dependência:** PR 1 mergeado antes de validar blueprint em dev.

---

## PR 3 — analytics-report-api

**Infraestrutura já existente no master** (confirmar antes de implementar qualquer coisa):
```bash
# Confirmar arquivos existentes — não reimplementar
ls ~/Cobliteam/analytics-report-api/analytics-report-consumer/src/main/kotlin/co/cobli/analytics/report/consumer/trigger/
# Deve ter: DynamicTriggerEventsExcelSerializer.kt, DynamicTriggerEventsGeneratorStrategy.kt

ls ~/Cobliteam/analytics-report-api/analytics-report-shared/src/main/kotlin/co/cobli/analytics/report/api/dynamic/
# Deve ter: ColumnsReport.kt, ColumnDefinition.kt, DynamicTriggerEvent.kt
```

**Arquitetura da trilha dinâmica** (nova experiência):
```
Kafka request (eventId != null)
  → DynamicTriggerEventsGeneratorStrategy
    → findEventById(eventId) → EventDefinition com ColumnsReport
    → getDynamicTriggerEvents(reportRequest) usando trigger_events_dynamic.sql
      (SQL separado do legado — sem JOINs em conditions/rules)
    → DynamicTriggerEventsExcelSerializer.serialize(event, columnsReport)
      → resolveValue(event, valuePath) → valor raw
      → translateEventType(rawValue) → label legível
```

**Único PR necessário para novo evento:**
1. `translateEventType()` — adicionar cases para os novos valores ENUM
2. `trigger_events.sql` — corrigir de INNER JOIN para LEFT JOIN (ver abaixo)

**Nenhum código novo** se `DynamicTriggerEventsExcelSerializer` já estiver conectado ao `DynamicTriggerEventsGeneratorStrategy` — validar antes.

### Fix obrigatório: trigger_events.sql — INNER JOIN gap

**Bug:** triggers da nova experiência têm `event_id` mas NÃO têm `condition_id`. A query legada faz INNER JOIN em `conditions c on c.category = te.meta->>'conditionCategory'` → `conditionCategory=null` → zero rows → eventos **invisíveis**.

**Fix:**
```sql
-- ANTES (query legada — INNER JOIN quebra nova experiência):
JOIN conditions c on c.category = te.meta->>'conditionCategory'
JOIN rules r on r.id = c.rule_id

-- DEPOIS (LEFT JOIN + join correto por id):
LEFT JOIN conditions c on c.id = t.condition_id
LEFT JOIN rules r on r.id = c.rule_id
LEFT JOIN events e on e.id = t.event_id   -- NOVO — para columns_report

-- SELECT: adicionar
e.columns_report::text as columns_report,

-- WHERE rule_id: adicionar OR para nova experiência
AND (cast(:rule_id as uuid) IS NULL
     OR c.rule_id = CAST(:rule_id AS uuid)
     OR t.event_id = CAST(:rule_id AS uuid))  -- novo
```

**Verificar se DynamicTriggerEventsExcelSerializer já está conectado:**
```bash
grep -n "eventId\|DynamicTrigger\|TODO" \
  ~/Cobliteam/analytics-report-api/analytics-report-consumer/src/main/kotlin/co/cobli/analytics/report/consumer/trigger/TriggerEventsGeneratorStrategy.kt \
  | head -20
```

- Se TODO ainda existe: conectar `DynamicTriggerEventsExcelSerializer` quando `eventId != null`
- Se já conectado: nenhum código adicional — `columns_report` da migration + `translateEventType()` são suficientes

---

## PR 4 — trigger-engine (meta.eventType para riskEventType) ← APENAS SE D8=agora

Branch: `feat/gf-XX-trigger-engine-meta-event-type`
Arquivo: `TriggerEvalProcessor.kt` linha ~171

```kotlin
// Antes:
eventType = unifiedData.data?.causedBy?.name,

// Depois (captura ambos os campos):
eventType = unifiedData.data?.causedBy?.name ?: unifiedData.data?.riskEventType?.name,
```

**Zero risco Kryo** — `TriggerEvent` é output do Flink, não estado serializado.

---

## PR 5 — herbie-dashboard (validação + requiresDevice se D6=COBLI_CAM)

**Sem D6:** Nenhum código a escrever. Validar manualmente:
1. Formulário de criação mostra o novo evento na lista
2. Multiselect exibe os tipos com labels corretos
3. Tabela de acionamentos renderiza colunas do columns_report
4. Export Excel contém as colunas definidas

**Com D6 (requiresDevice=COBLI_CAM):**
```bash
grep -rn "requiresDevice\|COBLI_CAM\|cobliCam" \
  ~/Cobliteam/herbie-dashboard/apps/dashboard/src/ | head -20
```

---

## Gate obrigatório antes de cada commit — test + intTest

**Regra universal:** nunca commitar sem testes passando. Se o escopo não tiver testes cobrindo a mudança, criar antes de commitar.

### trigger-action-api (Kotlin/Gradle)
```bash
# Testes unitários do módulo afetado
./gradlew :trigger-action-domain:test --daemon -q     # migration: sem testes unitários diretos
./gradlew :trigger-engine:test --daemon -q             # se TriggerEvalProcessor foi alterado
./gradlew :trigger-job:test --daemon -q                # se trigger-job foi alterado

# Testes de integração (requer docker-compose up -d postgres)
./gradlew :trigger-action-api:intTest --daemon -q      # valida migration aplicada + event no DB
```

**Se não existir teste para o escopo:**
- Migration nova → criar `EventsMigrationIntTest` que valida `SELECT name_identifier FROM events WHERE name_identifier = 'X'`
- TriggerEvalProcessor alterado → criar/atualizar `TriggerEvalProcessorTest` para o novo caso
- Novo campo em domínio → criar unit test no módulo correspondente

### blueprint-api (Kotlin/Gradle)
```bash
./gradlew :blueprint-api:test --daemon -q
```

**Se não existir teste para o escopo:**
- `AlertTypeDefinition` novo campo → criar unit test em `AlertTypeDefinitionTest`
- `CreateAlertViewUseCase` novo comportamento → criar/atualizar `CreateAlertViewUseCaseTest` cobrindo o novo fluxo (ex: `whenOptionGroupsThenMultiSelectOptionsAreGrouped`)
- `reverseConditionParams` com optionGroups → criar teste de round-trip: salva grupos → reconstitui grupos

### analytics-report-api / trigger-engine
```bash
./gradlew test --daemon -q   # no módulo raiz ou no módulo afetado
```

**Só commitar após:** todos os testes passando localmente.

---

## Checklist final

### Gate obrigatório antes de commitar cada PR

**Antes de `git commit` em qualquer PR deste escopo:**

```bash
# No repo do PR sendo commitado:
/cobli-review   # CodeRabbit + padrões Cobli + checklist Kotlin
```

Não commitar se `/cobli-review` reportar findings bloqueantes de:
- Null safety (NPE, `!!` não justificado)
- Padrões Cobli Kotlin (ktlint, naming, imports)
- Lógica incorreta apontada pelo review

Findings cosméticos (comentários, formatting já coberto por ktlintFormat) podem ser ignorados se ktlintFormat já rodou.

---

### Checklist técnico por PR

**PR 1 — trigger-action-api migration:**
- [ ] Migration usa `INSERT INTO ... SELECT ... WHERE NOT EXISTS` — nunca `VALUES (...)` sem idempotência
- [ ] UUIDs das condições são fixos no arquivo (não `gen_random_uuid()` inline) — necessário para idempotência e para o JSON fixture
- [ ] JSON fixture criado em `events/conditions/v1.0_<slug>.json` com mesmos UUIDs da migration
- [ ] `JsonSchemaValidationTest` rodado com `--rerun-tasks` após criar o fixture
- [ ] `columns_report` tem `"version": "v1.0"` no nível raiz
- [ ] Cada coluna tem `"order": N` (obrigatório, sem default em `ColumnDefinition`)
- [ ] Cada `valuePath` foi verificado em `resolveValue()` do serializer — nunca inferir
- [ ] `columnType` é um dos valores válidos do enum `ColumnType` (TEXT, NUMBER, DECIMAL, DATETIME, URL, BOOLEAN)
- [ ] Cada value em `possibleValues` tem tradução em `translateEventType()` (se coluna `meta.eventType` existe)
- [ ] Atributo(s) do trigger-engine verificados em `allTriggerEngineAttributes.json` — nunca assumir nome
- [ ] Testes de integração com OR expression: `assertEquals("<VALOR>", meta.eventType)` para cenários positivos — nunca `assertNull` (D8 pode já estar resolvido; verificar Fase 0 item 6 antes de assumir null)
- [ ] `name_identifier` em UPPERCASE
- [ ] `can_request_video = false` (obrigatório — não alterar sem decisão do time GF Core)

**PR 2 — blueprint-api:**
- [ ] I18n: todas as chaves `create.alert.condition.<paramKey>.option.<VALUE>` para cada value em `possibleValues`
- [ ] Testes em `CreateAlertViewUseCaseTest` cobrindo o novo evento (condição, multiselect, edit mode)
- [ ] `triggerEngineAttributes` vs `conditionType` — nunca os dois juntos

**PR 3 — analytics-report-api:**
- [ ] `translateEventType()` atualizado para os novos values
- [ ] `TriggerEventsExcelSerializer` mantém null-safe para `condition?.name` etc.

### Validação end-to-end
- [ ] Testes unitários passando (`./gradlew test`)
- [ ] Testes de integração passando (`./gradlew intTest` com docker-compose up)
- [ ] Migration aplicada em dev e evento aparece na lista
- [ ] blueprint-api retorna o evento no Event Selector
- [ ] Formulário de criação renderiza multiselect com labels corretos
- [ ] Alerta criado dispara corretamente para um evento de teste (frota Polentas)
- [ ] Tabela de acionamentos mostra o alerta
- [ ] Export Excel gera colunas especificadas no columns_report com labels traduzidos
- [ ] Jira tickets → Done após PR mergeado com CI verde

---

## Padrões consolidados (não perguntar — aplicar como default)

| Padrão | Regra | Origem |
|--------|-------|--------|
| Atributo `CAUSED_BY` | `CAUSED_BY` é o nome correto no schema — `toFieldName()` converte para `causedBy` (campo UnifiedThingType). `VIDEO_CAUSED_BY` não existe e causa condição sempre false | allTriggerEngineAttributes.json + EventExpressionFactory.toFieldName() confirmados 2026-04-20 |
| TAILGATING nunca em risk-event-v2 | Snowflake: 0 ocorrências em 30 dias; pipeline descarta | Investigação GF-13 |
| Migration path | `trigger-action-domain/src/main/resources/db/migration/` | Não `trigger-action-infrastructure` |
| Migration number | Verificar última no branch atual — não assumir por número de task | Última: V075 (2026-04-20) |
| eventNameIdentifier | Sempre UPPERCASE no DB e no blueprint | Bug corrigido: `dangerous_driving` → `DANGEROUS_DRIVING` |
| Label-Group Routing | 1 ConditionSpec com `triggerEngineAttributes: List<String>` — não 2 specs separados | Blueprint implementado 2026-04-20 |
| conditionType vs triggerEngineAttributes | Mutuamente exclusivos — não usar os dois juntos | AlertTypeDefinition.kt |
| meta.eventType (riskEventType) | **Verificar Fase 0 item 6 antes de assumir.** Se `?: riskEventType?.name` já existe → populado (não null). Fix pode já estar na codebase. `assertNull` em testes é errado nesse caso — usar `assertEquals("<VALOR>", ...)` | TriggerEvalProcessor.kt:171 — confirmado com fix em 2026-04-20 (GF-13) |
| meta.eventType (causedBy) | Populado — `causedBy?.name` (atributo DB: `CAUSED_BY`) | TriggerEvalProcessor.kt:171 |
| requiresDevice | Metadata blueprint-api, sem coluna no banco | Decisão GF-18 |
| 3-layer dedup | Ativo via originalEventId + unique index + REQUIRES_NEW | PR #366 GF-163 |
| @OneShot riskEventType + causedBy | Limpos antes de carry-forward — sem contaminação entre janelas | PR #367 |
| **Gate pré-commit** | SEMPRE: `./gradlew test` antes de commitar. Se escopo não tem teste → criar antes de commitar. intTest com docker-compose para mudanças que tocam DB ou integração. | Padrão universal 2026-04-20 |
| **Migration idempotente** | Sempre `INSERT INTO ... SELECT ... WHERE NOT EXISTS (SELECT 1 FROM events WHERE name_identifier = '...')` — nunca `VALUES (...)` direto. IDs de condição devem ser fixos (não `gen_random_uuid()` inline). | GF-13 rework 2026-04-20 |
| **JSON fixture obrigatório** | Criar `events/conditions/v1.0_<slug>.json` junto com cada migration — mesmos UUIDs. Rodar `JsonSchemaValidationTest --rerun-tasks` para validar. | Diretiva Reyes + JsonSchemaValidationTest 2026-04-20 |
| **can_request_video = false obrigatório** | Todos os eventos novos devem ter `can_request_video = false` até decisão explícita do time GF Core. Não perguntar em D5. | Definição time GF Core 2026-04-20 |
| **DynamicSerializer já existe** | `DynamicTriggerEventsExcelSerializer`, `DynamicTriggerEventsGeneratorStrategy`, `ColumnsReport`/`ColumnDefinition` existem em master — verificar antes de criar modelos novos. Reimplementar cria duplicatas | analytics-report-api investigado 2026-04-20 |
| **ColumnsReport schema** | `version: String` + `columns: List<ColumnDefinition>`. Cada column: `valuePath, label, columnType, measurementUnit?, valueFormat?, order: Int`. Sem `valueTranslations` — traduções ficam hardcoded em `translateEventType()` | analytics-report-shared/.../dynamic/ |
| **trigger_events.sql INNER JOIN gap** | Query legada faz INNER JOIN em `conditions c on c.category = te.meta->>'conditionCategory'` — nova experiência tem `conditionCategory=null` → zero rows. Fix: LEFT JOIN + `on c.id = t.condition_id` | analytics-report-api investigado 2026-04-20 |
| **event_by_id.sql já existe** | `analytics-report-shared/.../queries/event_by_id.sql` já consulta `events` por `id`. Não criar `event_columns_report.sql` redundante | analytics-report-api |
| **`trigger_events_dynamic.sql`** | SQL separado para trilha nova experiência. `DynamicTriggerEventsGeneratorStrategy` usa `getDynamicTriggerEvents()` com este SQL — não usa `trigger_events.sql` | analytics-report-api |

---

## Anti-padrões (nunca repetir)

| Anti-padrão | Consequência | Regra correta |
|-------------|-------------|---------------|
| Usar `VIDEO_CAUSED_BY` | Condição sempre false — alerta nunca dispara | Usar `CAUSED_BY` |
| Criar `EventColumnsReport`/`EventColumnSpec` novos | Duplicata de `ColumnsReport`/`ColumnDefinition` em `dynamic` package | Verificar master antes de criar modelos |
| Criar `event_columns_report.sql` | Duplicata de `event_by_id.sql` já existente | Verificar queries existentes |
| Assumir campo `valueTranslations` no `ColumnsReport` | Compilation error — campo não existe | Usar `translateEventType()` no serializer |
| Usar `meta.location` ou `meta.speed` no `columns_report` | Campos não existem em `TriggerEventMeta` → coluna sempre vazia | Verificar `TriggerEventMeta` antes de mapear |
| Não corrigir INNER JOIN no `trigger_events.sql` | Todos os eventos da nova experiência invisíveis no painel legado | LEFT JOIN obrigatório ao adicionar qualquer novo evento |
| Reescrever serializer quando infraestrutura já existe | Regressão em eventos existentes (FATIGUE, etc.) | Estender `translateEventType()` apenas |
| Emitir `multiSelectOptions` com `value: List<String>` (grouped) | herbie-dashboard não suporta — crash silencioso no render | Sempre flat: `{value: String, label: String}` por valor individual |
| Usar `optionGroups` para estruturar a UI | `optionGroups` é roteamento interno (`valueToConditionId`) — não chega ao front | Routing: `optionGroups`; UI: flat `multiSelectOptions` |
| Adicionar `HARSHACCELERATION`/`HARSHCORNERING`/`HARSHBREAKING` ao `CAUSED_BY` | Double-fire: câmera produz FAST_ACCELERATION_*/SPEEDY_TURN_*/HARD_BREAK_* em RISK_EVENTS E HARSH* em VIDEO_EVENTS. Trigger dispararia duas vezes para o mesmo evento físico. Evidência: RISK_EVENTS 30d = 0 ocorrências para HARSH*; VIDEO_EVENTS = 190K+76K+22K ocorrências | Validar via `DATA_PLATFORM.SILVER.RISK_EVENTS` + `VIDEO_EVENTS` (30 dias) antes de incluir em `possibleValues` |

---

---

## Fase 4 — Tracking (automático durante `--implement`)

### Arquivo de tracking

Ao iniciar `--implement`, criar (ou atualizar se já existir):

**Path:** `~/workflow/.claude/memory/add-alert-event-<slug>.md`
(ex: `add-alert-event-dangerous-driving.md`)

**Estrutura obrigatória:**

```markdown
---
name: add-alert-event tracking — <NOME_EVENTO>
description: Timeline e rastreio de PRs do evento <NOME_EVENTO>
type: project
---

## Evento
- **Nome:** <Nome Legível>
- **Identificador:** <NAME_IDENTIFIER>
- **Iniciado em:** <YYYY-MM-DD HH:MM>
- **Concluído em:** — (atualizar ao merge do último PR)

## Decisões

| # | Decisão | Escolha | Confirmado em |
|---|---------|---------|---------------|
| D1 | Atributo(s) | ... | YYYY-MM-DD HH:MM |
| D2 | source | ... | |
| D3 | UX multiselect | ... | |
| D4 | emitter_type | ... | |
| D5 | can_request_video | ... | |
| D6 | requiresDevice | ... | |
| D7 | columns_report | ... | |
| D8 | meta.eventType | ... | |

## PRs

| # | Repo | Branch | PR URL | Status CI | CodeRabbit | Mergeado em |
|---|------|--------|--------|-----------|------------|-------------|
| 1 | trigger-action-api | feat/... | https://... | ⬜ | ⬜ | — |
| 2 | blueprint-api | feat/... | https://... | ⬜ | ⬜ | — |
| 3 | analytics-report-api | feat/... | https://... | ⬜ | ⬜ | — |
| 4 | herbie-dashboard | feat/... | https://... | ⬜ | ⬜ | — |
| 5 | trigger-engine | feat/... | — | — | — | — |

Legenda: ⬜ Pendente | 🔄 Rodando | ✅ Verde | ❌ Falhou

## Iterações por PR (rework)

| PR | Commits extras | Motivo | Tempo perdido |
|----|---------------|--------|---------------|

## Achados de CI/CodeRabbit

| PR | Finding | Causa raiz | Prevenível? |
|----|---------|------------|-------------|
```

### Quando atualizar

| Evento | Ação |
|--------|------|
| Decisão D1–D8 confirmada | Preencher linha na tabela Decisões |
| `gh pr create` executado | Adicionar linha na tabela PRs com URL |
| CI verde / falhou | Atualizar coluna Status CI |
| CodeRabbit finding resolvido | Registrar em Achados; marcar CodeRabbit ✅ |
| PR mergeado | Preencher "Mergeado em" |
| Retrabalho (novo commit por finding) | Adicionar linha em Iterações |

Salvar via `Write` tool diretamente no arquivo de tracking.
Adicionar ao `MEMORY.md` se ainda não existir:
```
- [Tracking <NOME>](add-alert-event-<slug>.md) — PRs e timeline do evento <NOME_EVENTO>
```

---

## Fase 5 — Relatório (`--report`)

Ao invocar `/add-alert-event <evento> --report`:

1. Ler o arquivo `~/workflow/.claude/memory/add-alert-event-<slug>.md`
2. Preencher cada seção com os dados reais do evento — sem placeholders
3. Gerar o relatório abaixo no chat

**Regra de 1 PR por repo:** cada repo deve ter exatamente 1 PR aberto por escopo de evento.
Antes de gerar o relatório, verificar:
```bash
gh pr list --repo Cobliteam/<repo> --state open --search "<event-slug>"
```
Se houver mais de 1 PR aberto no mesmo repo para o mesmo evento → sinalizar como risco antes de apresentar o relatório.

---

### Exemplo preenchido — DANGEROUS_DRIVING

```markdown
# Relatório — Direção Perigosa (DANGEROUS_DRIVING)

## O que cada PR entrega — visão de produto

Explicação em linguagem natural do resultado visível para o gestor de frota,
sem jargão técnico. Uma seção por PR.

**PR 1 — trigger-action-api**
O sistema passa a reconhecer "<Nome do Evento>" como tipo de alerta. É a fundação:
sem isso nenhuma das outras mudanças funciona. O gestor ainda não vê nada na tela,
mas o motor de regras já sabe avaliar os eventos configurados.

**PR 2 — blueprint-api**
[Descrever o que o gestor vê ao criar um alerta e ao filtrar a tabela de acionamentos.
Exemplo: "ao abrir o formulário de nova política, '<Nome>' aparece como opção.
O gestor seleciona quais subtipos quer monitorar — por exemplo, só 'Frenagem brusca (45 km/h)'
— e salva."]

**PR 3 — analytics-report-api**
[Descrever o que muda no Export Excel. Exemplo: "a coluna 'Tipo de evento' exibe
o nome legível em vez do código interno — '<Label Legível>' em vez de '<ENUM_VALUE>'."]

**PR 4 — herbie-dashboard**
[Descrever o que muda visualmente no painel. Exemplo: "o nome '<Nome>' aparece
corretamente nas listas e cabeçalhos onde hoje apareceria o identificador técnico."]

---

## Decisões tomadas

| # | Decisão | Escolha | Justificativa |
|---|---------|---------|---------------|
| D1 | Atributo(s) | RISK_EVENT_TYPE (8 valores) + CAUSED_BY (TAILGATING) | Snowflake 30d: TAILGATING tem 0 ocorrências em RISK_EVENTS — pipeline o filtra antes. Os 8 valores de aceleração/curva/frenagem chegam normalizados via riskeventsv2. |
| D2 | source | USER — Label-Group Routing | Usuário escolhe subtipos; valores distribuídos em dois campos do UnifiedThingType → 2 condições DB + 1 ConditionSpec blueprint com `triggerEngineAttributes: listOf("RISK_EVENT_TYPE", "CAUSED_BY")` |
| D3 | UX multiselect | Flat (9 opções individuais) | herbie-dashboard suporta apenas `{value: String, label: String}` flat — grouped exigiria PR extra sem valor para V1 |
| D4 | emitter_type | SPOT | Manobra brusca é evento pontual — 1 notificação por ocorrência, sem duração |
| D5 | can_request_video | false | Definição time GF Core: nenhum evento habilita solicitação de vídeo no V1 |
| D6 | requiresDevice | null | Evento cobre sensor e câmera (riskeventsv2 normaliza câmera → RISK_EVENTS) |
| D7 | columns_report | meta.eventType (TEXT, order:0) + meta.videoUrl (URL, order:1) | Tipo de evento identifica a manobra; videoUrl relevante para frotas com câmera |
| D8 | meta.eventType | Adiar para V2 | TAILGATING (causedBy) já popula meta.eventType; riskEventType fica null só para sensor — impacto parcial, sem urgência V1 |

## PRs — 1 por repo, ordem de deploy

| Ordem | PR | Repo | Escopo | Motivo da posição |
|-------|----|------|--------|-------------------|
| 1º | [#374](https://github.com/Cobliteam/trigger-action-api/pull/374) | trigger-action-api | Migration V076: INSERT events + JSON fixture | Cria o `event_id` e `conditions` no DB — sem isso nenhum outro serviço encontra o evento |
| 2º | [#120](https://github.com/Cobliteam/blueprint/pull/120) | blueprint-api | AlertType + AlertTypeDefinition + I18n + enrichWithApiConditions | Lê `name_identifier=DANGEROUS_DRIVING` no DB — precisa da migration aplicada em dev para validar end-to-end |
| 3º | [#698](https://github.com/Cobliteam/analytics-report-api/pull/698) | analytics-report-api | translateEventType() + DynamicSerializer conectado | Lê `columns_report` da migration para serializar o Excel — independente do blueprint, mas requer PR 1 |
| 4º | [#10989](https://github.com/Cobliteam/herbie-dashboard/pull/10989) | herbie-dashboard | Tradução do nome do evento na UI | Sem dependência de DB — deploy após blueprint para evitar evento visível com formulário incompleto |

> **Merge:** PR 1 → validar em dev → PRs 2, 3 e 4 em qualquer ordem.
> **1 PR por repo:** verificar `gh pr list --state open` antes de criar branch — nunca abrir segundo PR enquanto o primeiro está aberto.

## Timeline de criação

| Etapa | Data | Observação |
|-------|------|------------|
| Reconnaissance + Decisões D1–D6 | 2026-04-07 | Pivô de estratégia: HARSH* retirados de CAUSED_BY (double-fire confirmado via Snowflake) |
| PR 1 (#374) + PR 2 (#120) criados | 2026-04-07 | — |
| Decisões D7–D8 + PR 3 (#698) + PR 4 (#10989) | 2026-04-10 | Investigação analytics-report-api revelou DynamicSerializer já existente em master |
| Rework D3 + idempotência + fix `"operator": "OR"` | 2026-04-20 | optionGroups serializado incorretamente; migration sem WHERE NOT EXISTS |
| Testes GF-41/43 + JSON fixture | 2026-04-20 | EventSpotEmitterExpressionTest, TriggerEvalProcessorTest, v1.0_dangerous_driving.json |
| **TOTAL** | **13 dias corridos** | Com pausas entre sessões |

## Rework (iterações evitáveis)

| PR | Commits extras | Causa raiz | Prevenível com |
|----|---------------|------------|----------------|
| #374 | 3 | `columns_report` sem `version`/`order`; `meta.speed` inexistente; `"value": "OR"` inválido | Checklist de coerência do columns_report + verificar resolveValue() antes de mapear |
| #120 | 3 | `multiSelectOptions` serializado como `value: List<String>` — herbie-dashboard rejeita; linting; CodeRabbit | Documentar contrato flat `{value: String}` na skill (feito) |
| #698 | 3 | `EventColumnsReport` duplicado do `dynamic` package; INNER JOIN quebrava nova experiência; Mockito stub faltando | Verificar master antes de criar modelos; documentar anti-padrão INNER JOIN (feitos) |
```

---

### Template vazio para novo evento

```markdown
# Relatório — <NOME_EVENTO>

## O que cada PR entrega — visão de produto

**PR 1 — trigger-action-api**
[O sistema passa a reconhecer "<Nome>" como tipo de alerta. Fundação — motor de regras
já avalia os eventos, mas nada aparece na UI ainda.]

**PR 2 — blueprint-api**
[O gestor vê "<Nome>" no formulário de criação de alerta. Seleciona os subtipos desejados.
Na tabela de acionamentos, o filtro "Tipo de evento" mostra as opções corretas deste alerta.]

**PR 3 — analytics-report-api**
[No Export Excel, a coluna "Tipo de evento" mostra label legível em vez do código interno.]

**PR 4 — herbie-dashboard**
[O nome "<Nome>" aparece corretamente no painel. Se aplicável: condições cam-only
aparecem desabilitadas com tooltip para frotas sem Cobli Cam.]

---

## Decisões tomadas

| # | Decisão | Escolha | Justificativa |
|---|---------|---------|---------------|
| D1 | Atributo(s) | | |
| D2 | source | | |
| D3 | UX multiselect | | |
| D4 | emitter_type | | |
| D5 | can_request_video | false | Definição time GF Core |
| D6 | requiresDevice | | |
| D7 | columns_report | | |
| D8 | meta.eventType | | |

## PRs — 1 por repo, ordem de deploy

| Ordem | PR | Repo | Escopo | Motivo da posição |
|-------|----|------|--------|-------------------|
| 1º | [#NNN](...) | trigger-action-api | Migration VNNN + JSON fixture | Cria o evento no DB — todos os outros dependem desta linha |
| 2º | [#NNN](...) | blueprint-api | AlertType + I18n | Precisa do name_identifier no DB para validar |
| 3º | [#NNN](...) | analytics-report-api | translateEventType() | Precisa do columns_report da migration |
| 4º | [#NNN](...) | herbie-dashboard | UI | Deploy após blueprint |

## Timeline de criação

| Etapa | Data | Observação |
|-------|------|------------|
| Reconnaissance + Decisões D1–D8 | | |
| PR 1 criado → CI verde | | |
| PR 2 criado → CI verde | | |
| PR 3 criado → CI verde | | |
| PR 4 criado → CI verde | | |
| **TOTAL** | | |

## Rework (iterações evitáveis)

| PR | Commits extras | Causa raiz | Prevenível com |
|----|---------------|------------|----------------|

## Lições aprendidas

(O que não estava documentado na skill e causou retrabalho)

## Propostas de melhoria da skill

(Para cada lição: o que adicionar/corrigir para evitar repetir)
```

Após gerar: propor automaticamente edições à skill via `Edit` tool para incorporar as propostas confirmadas pelo usuário.

---

## Referências

| Recurso | Onde |
|---------|------|
| resolveConditionMappings (triggerEngineAttributes) | `CreateAlertViewUseCase.kt:179` |
| enrichWithApiConditions (merge possibleValues) | `CreateAlertViewUseCase.kt:233` |
| reverseConditionParams | `CreateAlertViewUseCase.kt:346` |
| FATIGUE como referência (ENUM multiselect, single conditionType) | `CreateAlertViewUseCase.kt` (AlertType.FATIGUE) |
| meta.eventType gap | `TriggerEvalProcessor.kt:171` |
| @OneShot fields | `UnifiedThingType.kt` — riskEventType, causedBy, videoURL, videoSeverity, videoFiles, notificationType, notificationInfo, inGeofences |
| AlertTypeDefinition data class | `blueprint-api/.../domain/model/AlertTypeDefinition.kt` |
| DynamicTriggerEventsExcelSerializer | `analytics-report-api/` commit b6f0b8f7 |
| Decisão arquitetural GF-13/14 | `wtb doc get gf-13-gf-14-decis-o-de-arquitetura-dangerous-driving-dual-so` |
| Board GF | https://cobliteam.atlassian.net/jira/software/projects/GF/boards/1909 |
