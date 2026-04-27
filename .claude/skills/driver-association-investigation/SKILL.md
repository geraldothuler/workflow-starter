---
name: driver-association-investigation
description: >
  Investigação de associações incorretas de motorista em frotas Cobli.
  Identifica a fonte real da associação (face_id, iButton, painel, app, API),
  detecta padrões de falso positivo no reconhecimento facial e gera análise
  para o CS/time de câmera. Ativar quando: cliente reporta associação incorreta
  de motorista, suspeita de bug em reconhecimento facial, ou pedido de auditoria
  de associações.
user-invocable: true
---

# Skill: driver-association-investigation

---

## Contexto

**Fonte de verdade:** Fusca PostgreSQL (`device_driver_association_input_log`)

**Sources possíveis:**
| source | Significado |
|--------|-------------|
| `face_id` | Câmera de reconhecimento facial — automático, sem ação de usuário |
| `sherlock_driver_ibutton` | iButton físico inserido no leitor do veículo |
| `dashboard` | Associação manual via painel web |
| `app` | Associação manual via app mobile |

**Types:**
- `IDENTIFY` — motorista associado ao dispositivo
- `UNIDENTIFY` — associação encerrada

---

## Procedimento

### 1. Buscar o motorista na frota

```sql
SELECT id, name, cpf, driver_code
FROM driver
WHERE fleet_id = '<fleet_id>'
  AND name ILIKE '%<nome>%'
  AND deleted_at IS NULL;
```

### 2. Buscar device_id do veículo

```sql
SELECT v.license_plate, v.device_id, d.imei
FROM vehicle v
JOIN device d ON d.id = v.device_id
WHERE v.fleet_id = '<fleet_id>'
  AND v.license_plate ILIKE '<placa>';
```

### 3. Input log do veículo no período

```sql
SELECT
  l.driver_id,
  d.name AS driver_name,
  l.source,
  l.type,
  l.association_date AT TIME ZONE 'America/Sao_Paulo' AS association_br,
  l.created_at       AT TIME ZONE 'America/Sao_Paulo' AS created_br
FROM device_driver_association_input_log l
JOIN driver d ON d.id = l.driver_id
WHERE l.device_id = '<device_id>'
  AND l.association_date >= '<start>'
  AND l.association_date <= '<end>'
ORDER BY l.association_date;
```

### 4. Input log do motorista suspeito (todos os veículos)

```sql
SELECT
  l.device_id,
  l.source,
  l.type,
  l.association_date AT TIME ZONE 'America/Sao_Paulo' AS association_br
FROM device_driver_association_input_log l
WHERE l.driver_id = '<driver_id>'
  AND l.association_date >= '<start>'
ORDER BY l.association_date DESC
LIMIT 50;
```

### Padrão de acesso ao Fusca

```bash
PGPASS=$(kubectl get secret fusca-api-secrets -n organization --context cobli-prod \
  -o jsonpath='{.data.spring-datasource-password}' | base64 -d)

kubectl run psql-drv-inv --image=postgres:15 -n organization \
  --context cobli-prod-devices --restart=Never --rm -i -- \
  bash -c "PGPASSWORD='${PGPASS}' psql 'postgresql://fusca-api@fusca-db.cobli.co:5432/fusca' -t -A -F'|' -c \"<query>\""
```

**Atenção:** `--rm -i` sempre no foreground — nunca em background (trava aguardando stdin).

---

## Diagnóstico por source

### face_id — padrão de falso positivo

**Sinal crítico:** UNIDENTIFY de motorista A e IDENTIFY de motorista B com diferença ≤ 1 segundo no mesmo device.

```
XX:XX:XX.999 — Motorista A  UNIDENTIFY  (face_id)
XX:XX:XX.000 — Motorista B  IDENTIFY    (face_id)   ← mesmo instante
```

**Interpretação:** fisicamente impossível ser troca real de motorista. O sistema de reconhecimento facial está oscillando entre dois candidatos para o **mesmo rosto** — embeddings faciais similares ou qualidade ruim da imagem.

**Ação:** acionar time de câmera para:
1. Verificar similaridade dos embeddings entre os dois motoristas
2. Recalibrar/re-treinar cadastros faciais na frota
3. Confirmar com cliente qual motorista estava realmente no veículo

### sherlock_driver_ibutton

Associação por iButton físico — o motorista inseriu o token no leitor do veículo. Não é ação manual.

**Sinal de problema:** IDENTIFY em múltiplos veículos simultaneamente → iButton cross-fleet (token registrado em mais de uma frota). Ver `driver_identification_token` para verificar.

### dashboard / app

Ação manual por usuário. O log **não registra qual usuário** fez a ação — apenas o source. Para identificar o usuário, verificar `activity_log` no Fusca ou logs de auditoria se disponíveis.

---

## Tabelas relevantes

| Tabela | Uso |
|--------|-----|
| `device_driver_association_input_log` | Fonte primária — todos os eventos de associação com source |
| `vehicle_driver_association_history` | Histórico consolidado por veículo (sem source) |
| `driver_identification_token` | Tokens iButton do motorista |
| `driver` | Cadastro do motorista (fleet_id, name, cpf, driver_code) |
| `vehicle` | Veículo (license_plate, device_id, fleet_id) |
| `device` | Dispositivo (imei, cobli_id) |

**Observações:**
- `vehicle_driver_association_history` não tem coluna `source` — usar `device_driver_association_input_log` para investigação
- `device` não tem `vehicle_id` — join pelo `vehicle.device_id`
- `driver` tem `fleet_id` direto (não tabela de join `fleet_driver`)

---

## Lições aprendidas (BUG-2124 — Imetame, 2026-03-12)

- **`face_id` não é ação de usuário**: cliente pode não saber que tem câmera ativa ou que o motorista tem rosto cadastrado
- **Diferença ≤ 1s entre UNIDENTIFY/IDENTIFY = mesmo frame**: sinal definitivo de falso positivo do reconhecimento facial, não troca física
- **O relatório do cliente exibe "painel ou app"** mas `source=face_id` no banco — a UI do relatório não distingue a origem corretamente; sempre verificar o banco
- **Padrão de alternância**: A→B→A→B no mesmo veículo em curtos intervalos = modelo oscilando entre dois candidatos com score similar
- **`vehicle_driver_association_history` pode estar vazia** mesmo com eventos no `input_log` — sempre consultar `input_log`
- **Colunas que não existem**: `fleet_driver` (tabela), `device.vehicle_id`, `driver.external_id`, `vehicle.current_device_id` — não usar

---

## Output esperado

```
[VEÍCULO: ODS2H64 | device: 862798052485766]
Período: 2026-02-01 a 2026-02-10

Eventos do motorista João Batista Euzebio:
  - 100% via face_id
  - 0 eventos via dashboard/app

Padrão identificado: FALSO POSITIVO face_id
  - 12 ocorrências de UNIDENTIFY(Wanderson) + IDENTIFY(João) com ≤1s de diferença
  - Provável confusão de embedding entre Wanderson e João

Recomendação: acionar time de câmera para recalibração dos embeddings.
```
