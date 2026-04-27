---
name: rds-pg-eval
description: >
  Avaliação completa de performance de banco PostgreSQL no RDS: queries lentas,
  índices (uso, bloat, oportunidade), autovacuum, locks e Performance Insights via AWS CLI + Datadog.
  Ativar quando houver alerta de DB Load, latência alta, timeout de query ou suspeita de bloat/missing index.
user-invocable: true
---

# rds-pg-eval — Avaliação de Performance RDS/PostgreSQL

## Escopo

Avalia o banco PostgreSQL do atlas-api em prod (e adaptável a outros serviços).
Não executa nenhuma alteração — apenas diagnóstico e recomendações.

---

## 1. Pod de acesso psql

Criar pod efêmero para consultas (usar quando não há pod ativo):

```bash
# Criar pod com PGPASSWORD já injetado (reutilizar se atlas-ops/atlas-vacuum existir)
kubectl get pods -n planning --context cobli-prod | grep -E "atlas-ops|atlas-vacuum"

# Se não existir — criar:
kubectl run atlas-ops --context cobli-prod -n planning \
  --image=postgres:15 --restart=Never \
  --env="PGPASSWORD=$(kubectl exec -n planning <app-pod> --context cobli-prod \
    -- env | grep PGPASSWORD | cut -d= -f2)" \
  --command -- sleep 3600

# Alternativa: pegar PGPASSWORD do pod de app em execução
PGPASS=$(kubectl exec -n planning --context cobli-prod \
  $(kubectl get pod -n planning --context cobli-prod -l app=atlas-api-app -o jsonpath='{.items[0].metadata.name}') \
  -- env | grep PGPASSWORD | cut -d= -f2-)
```

Conexão padrão:
```
host=atlas-api-db.cobli.co  dbname=atlasApi  user=atlas-api
```

Alias para usar nos comandos abaixo:
```bash
PSQL='kubectl exec -n planning atlas-ops --context cobli-prod -- psql "host=atlas-api-db.cobli.co dbname=atlasApi user=atlas-api"'
```

---

## 2. Queries lentas agora

```sql
-- Queries ativas > 5s (ordem por duração)
SELECT pid,
       round(extract(epoch FROM now() - query_start)::numeric, 1) AS duration_s,
       state, wait_event_type, wait_event,
       left(query, 150) AS query
FROM pg_stat_activity
WHERE state != 'idle'
  AND pid != pg_backend_pid()
  AND query_start < now() - interval '5s'
ORDER BY duration_s DESC;
```

**Diagnóstico de wait events:**

| wait_event_type | wait_event | Significado |
|----------------|------------|-------------|
| IO | DataFileRead | Lendo páginas do heap/índice do disco — seq scan ou buffer miss |
| IO | DataFileWrite | Escritas — autovacuum, WAL, UPDATE massivo |
| Lock | relation / tuple | Lock de tabela/linha — contention |
| LWLock | buffer_content | Pressão no shared_buffers |
| IPC | BgWorkerShutdown | Autovacuum sinalizando |

---

## 3. Plano de execução de queries lentas

```sql
-- Substituir pelos parâmetros reais da query ativa
EXPLAIN (ANALYZE false, FORMAT TEXT)
<query_copiada_do_pg_stat_activity>;
```

**Red flags no EXPLAIN:**

| Nó | Problema | Fix |
|----|----------|-----|
| `Seq Scan on <tabela grande>` | Sem índice ou planner preferiu seq scan | Índice composto ou parcial |
| `BitmapAnd` de dois índices | Índice composto resolve com single scan | `CREATE INDEX (col1, col2)` |
| `Sort` após `Bitmap Heap Scan` | Índice não cobre ORDER BY | Adicionar sort col ao índice |
| `Recheck Cond: ... Filter: col=X` | col não está no índice — heap fetch desnecessário | Adicionar col ao índice |
| `rows=8489 width=538` alto width | Busca colunas TOAST grandes (JSONB, text) | Avaliar lazy load na app |

---

## 4. Índices — uso e bloat

```sql
-- Todos os índices por tamanho com contagem de scans
SELECT i.relname AS "table",
       i.indexrelname AS "index",
       i.idx_scan,
       i.idx_tup_read,
       pg_size_pretty(pg_relation_size(i.indexrelid)) AS size,
       t.n_dead_tup,
       round(100.0 * t.n_dead_tup / nullif(t.n_live_tup + t.n_dead_tup, 0), 1) AS dead_pct
FROM pg_stat_user_indexes i
JOIN pg_stat_user_tables t ON t.relname = i.relname
ORDER BY pg_relation_size(i.indexrelid) DESC;
```

**Critérios de avaliação:**

| Situação | Ação |
|----------|------|
| `idx_scan = 0` e size > 50 MB | Drop candidate — confirmar via Datadog APM 7 dias |
| `idx_scan < 100` e size > 200 MB | Investigar via APM antes de dropar |
| `dead_pct > 10%` | Autovacuum atrasado — verificar `last_autovacuum` e `reloptions` |
| BitmapAnd de dois índices no EXPLAIN | Candidato a índice composto |

**Verificar validade de índices:**
```sql
SELECT ix.indexrelid::regclass AS index_name,
       ix.indisvalid, ix.indisready,
       pg_size_pretty(pg_relation_size(ix.indexrelid)) AS size
FROM pg_index ix
JOIN pg_class c ON c.oid = ix.indexrelid
WHERE c.relname = '<nome_do_indice>';
```

---

## 5. Autovacuum — status e calibração

```sql
-- Status de vacuum e dead tuples por tabela
SELECT relname,
       n_live_tup, n_dead_tup,
       round(100.0 * n_dead_tup / nullif(n_live_tup + n_dead_tup, 0), 1) AS dead_pct,
       last_vacuum, last_autovacuum, last_analyze, last_autoanalyze,
       vacuum_count, autovacuum_count,
       unnest(reloptions) AS reloption
FROM pg_stat_user_tables
LEFT JOIN pg_class ON relname = pg_class.relname
WHERE pg_class.relkind = 'r'
ORDER BY n_dead_tup DESC;
```

**Threshold de disparo autovacuum:**
```
threshold = max(autovacuum_vacuum_threshold, n_live_tup × autovacuum_vacuum_scale_factor)
default: scale_factor = 0.2  →  27M rows × 0.2 = 5.4M dead antes de disparar
tuned:   scale_factor = 0.05 →  27M rows × 0.05 = 1.35M dead (4× mais frequente, menos IO por run)
```

**Aplicar calibração (sem lock, idempotente):**
```sql
ALTER TABLE <tabela> SET (
    autovacuum_vacuum_scale_factor  = 0.05,
    autovacuum_vacuum_cost_delay    = 2,
    autovacuum_analyze_scale_factor = 0.05
);
-- Verificar:
SELECT unnest(reloptions) FROM pg_class WHERE relname = '<tabela>' AND relkind = 'r';
```

---

## 6. Progresso de operações em curso

```sql
-- CREATE INDEX CONCURRENTLY em progresso
SELECT relid::regclass AS table,
       phase, blocks_done, blocks_total,
       round(100.0*blocks_done/nullif(blocks_total,0),1) AS pct,
       tuples_done, tuples_total
FROM pg_stat_progress_create_index;

-- VACUUM em progresso
SELECT relid::regclass AS table,
       phase, heap_blks_scanned, heap_blks_total,
       round(100.0*heap_blks_scanned/nullif(heap_blks_total,0),1) AS pct,
       dead_tuple_count
FROM pg_stat_progress_vacuum;
```

**Fases do CREATE INDEX CONCURRENTLY:**
1. `initializing` → `waiting for writers before build` → `building index` → `waiting for readers before cleanup` → `index validation: scanning table` → concluído (0 rows)

---

## 7. Locks — contention ativa

```sql
-- Locks bloqueantes com query de quem bloqueia e quem espera
SELECT blocked.pid AS waiting_pid,
       round(extract(epoch FROM now() - blocked.query_start)) AS waiting_s,
       blocking.pid AS blocking_pid,
       left(blocked.query, 100) AS waiting_query,
       left(blocking.query, 100) AS blocking_query
FROM pg_stat_activity blocked
JOIN pg_stat_activity blocking
  ON blocking.pid = ANY(pg_blocking_pids(blocked.pid))
WHERE cardinality(pg_blocking_pids(blocked.pid)) > 0;
```

**Cancelar query bloqueante (somente se autorizado):**
```sql
SELECT pg_cancel_backend(<pid>);   -- SIGINT — cancela a query, mantém conexão
SELECT pg_terminate_backend(<pid>); -- SIGTERM — mata a conexão
```

---

## 8. AWS Performance Insights

**Pré-requisito:** credenciais AWS configuradas (`aws sts get-caller-identity` deve retornar sem erro).

**Descobrir resource ID:**
```bash
aws rds describe-db-instances \
  --region us-east-1 \
  --query 'DBInstances[?contains(DBInstanceIdentifier, `atlas`)].{id:DBInstanceIdentifier,rid:DbiResourceId,pi:PerformanceInsightsEnabled}' \
  --output table
```

**Top SQL por DB Load (janela de investigação):**
```bash
aws pi describe-dimension-keys \
  --service-type RDS \
  --identifier "db-VUEPUBAEYZCCQBGXBJCDPIZFOU" \
  --start-time "2026-04-13T07:00:00Z" \
  --end-time "2026-04-13T12:00:00Z" \
  --period-in-seconds 3600 \
  --metric "db.load.avg" \
  --group-by '{"Group": "db.sql", "Limit": 10}' \
  --region us-east-1 \
  --output json | python3 -c "
import json,sys
d = json.load(sys.stdin)
print(f'Window: {d[\"AlignedStartTime\"]} → {d[\"AlignedEndTime\"]}')
for k in d['Keys']:
    sql = k['Dimensions'].get('db.sql.statement', 'N/A')[:120]
    print(f'  load={k[\"Total\"]:.3f}  {sql}')
"
```

**Top Wait Events:**
```bash
aws pi describe-dimension-keys \
  --service-type RDS \
  --identifier "db-VUEPUBAEYZCCQBGXBJCDPIZFOU" \
  --start-time "<ISO8601>" \
  --end-time "<ISO8601>" \
  --period-in-seconds 3600 \
  --metric "db.load.avg" \
  --group-by '{"Group": "db.wait_event_type", "Limit": 10}' \
  --region us-east-1 \
  --output json | python3 -c "
import json,sys
d = json.load(sys.stdin)
for k in d['Keys']:
    ev = k['Dimensions'].get('db.wait_event_type.name', str(k['Dimensions']))
    print(f'  {k[\"Total\"]:.3f}  {ev}')
"
```

**Referência atlas-api-db-prod:**
- DBInstanceIdentifier: `atlas-api-db-prod`
- DbiResourceId: `db-VUEPUBAEYZCCQBGXBJCDPIZFOU`
- Instance: `db.t4g.xlarge` (4 vCPU, 16 GB RAM)
- Performance Insights: habilitado, retenção 7 dias

---

## 9. Datadog — DB Load + IO histórico

Usar MCP Datadog (`mcp__datadog__get_datadog_metric`):

```
Queries:
  avg:aws.rds.dbload{aws_rds_instance:arn:aws:rds:us-east-1:911383825788:db:atlas-api-db-prod}
  avg:aws.rds.read_iops{dbinstanceidentifier:atlas-api-db-prod}
  avg:aws.rds.write_iops{dbinstanceidentifier:atlas-api-db-prod}
  avg:aws.rds.disk_queue_depth{dbinstanceidentifier:atlas-api-db-prod}

Monitor de sobrecarga: ID 162708902 (threshold: db.load > 20)
```

**Baseline histórico atlas-api-db (Apr 2026):**

| Métrica | Normal (business hours) | Alerta |
|---------|------------------------|--------|
| DB Load (sessions) | 1.5–3.5 avg | > 8 avg / > 20 pico |
| Read IOPS | 500–2.000 | > 3.000 sustained |
| Write IOPS | 80–400 | > 800 sustained |
| Disk Queue Depth | 0.3–2.5 | > 10 |

---

## 10. Protocolo de diagnóstico — sequência recomendada

```
1. Datadog DB Load (now-7d) → baseline vs hoje
2. pg_stat_activity → queries ativas > 5s, wait events
3. EXPLAIN das queries lentas → Seq Scan? BitmapAnd? Sort desnecessário?
4. pg_stat_user_indexes → idx_scan baixo? tamanho alto? dead_pct?
5. pg_stat_user_tables → autovacuum atrasado? dead tuples acumulando?
6. Performance Insights → top SQL + wait events na janela do incidente
7. Locks → quem está bloqueando quem?
```

---

## 11. Criação segura de índices em prod

**Regra:** sempre `CONCURRENTLY` em tabelas com > 500k rows. Nunca em Flyway migration diretamente.

```bash
# 1. Rodar via pod em background
kubectl exec -n planning atlas-ops --context cobli-prod -- bash -c "
nohup psql 'host=atlas-api-db.cobli.co dbname=atlasApi user=atlas-api' -c \"
CREATE INDEX CONCURRENTLY idx_<nome>
ON <tabela> (<colunas>)
[WHERE <predicado_parcial>];
\" > /tmp/idx_<nome>.log 2>&1 &
echo \"PID: \$!\"
"

# 2. Monitorar progresso
# (usar query da seção 6)

# 3. Verificar validade ao concluir
# (usar query da seção 4 — indisvalid=t)

# 4. Adicionar migration IF NOT EXISTS (no-op em prod)
# atlas-shared/src/main/resources/db/migration/VXX.0__Create_index_<nome>.sql
```

**Índices parciais:** preferir quando a query tem predicado fixo (`WHERE completion_status IS NULL`). Menor tamanho, writes mais baratos.

---

## 12. Referências atlas-api

| Tabela | Rows | Índices críticos | Risco |
|--------|------|-----------------|-------|
| activity | 27.4M | `route_id_idx`, `activity_fleet_id_type_completion_status_start_time_idx`, `idx_activity_fleet_vehicle_open` | TOAST bloat se autovacuum atrasado |
| destination | 68.6M | `destination_pkey` | Maior tabela — sem índice adicional além de PK |
| route | 2.5M | `route_fleet_id_idx`, `status_idx`, `start_time_idx`, `idx_route_fleet_status_start` | TOAST 29 GB (path JSONB ~10 KB/row) |

**Autovacuum calibrado (aplicado 2026-04-14):**
- `route`: scale_factor 0.05, cost_delay 2
- `activity`: scale_factor 0.05, cost_delay 2

---

## 13. Referências trigger-action-db

| Item | Valor |
|------|-------|
| DBInstanceIdentifier | `trigger-action-one` |
| DbiResourceId | `db-PMOQXZAH6LCBDL6IPZ66UVGN2I` |
| Engine | Aurora PostgreSQL 15.12 |
| Instance class | db.r7g.xlarge (4 vCPU, 32 GB RAM) |
| Storage | Aurora (sem IOPS fixo — billing por IO request) |
| MultiAZ | Não |
| App pods | namespace `monitoring`, context `cobli-prod` |
| Flink pods | namespace `monitoring`, context `cobli-prod-devices` |
| DB endpoint | `trigger-action-db-prod.cobli.co:5432/triggerAction` |
| DB user | `trigger-action-api` |
| Password env var | `SPRING_DATASOURCE_PASSWORD` (no pod `trigger-action-api-app-deployment-*`) |

**Como criar pod psql:**
```bash
PGPASS=$(kubectl exec -n monitoring --context cobli-prod \
  $(kubectl get pod -n monitoring --context cobli-prod -l app=trigger-action-api-app -o jsonpath='{.items[0].metadata.name}') \
  -- env | grep SPRING_DATASOURCE_PASSWORD | cut -d= -f2-)

kubectl run trigger-ops --context cobli-prod-devices -n monitoring \
  --image=postgres:15 --restart=Never \
  --env="PGPASSWORD=$PGPASS" \
  --command -- sleep 3600
```

**PSQL (usar direto, sem alias em variável shell):**
```bash
kubectl exec -n monitoring trigger-ops --context cobli-prod-devices -- \
  psql "host=trigger-action-db-prod.cobli.co dbname=triggerAction user=trigger-action-api" \
  -c "SELECT ..."
```

**Estado do banco (Apr 2026):**

| Tabela | Partições | Total | Rows/dia | Partição mais antiga |
|--------|-----------|-------|----------|---------------------|
| trigger_events | 736 (daily) | 255 GB | ~1M | 2024-04-29 |
| notifications | ~736 (daily) | ~250 GB | ~1M | ~2024-04-29 |

**Baseline trigger-action-db (Apr 2026):**

| Métrica | Normal | Pico observado (15-Apr) |
|---------|--------|------------------------|
| Read IOPS avg | 200–500 | 4.237 avg / 26.899 max |
| Write IOPS | 150–430 | 535 (estável) |
| DB Load (sessions) | 0.19–0.35 | 1.16 |
| Disk Queue Depth | 0.03–0.10 | 0.42 |

**Causa raiz dos picos de IOPS (diagnóstico 2026-04-19):**

`SELECT DISTINCT trigger_events JOIN triggers` sem filtro `event_time` → zero partition pruning → 468 `Bitmap Index Scan` por query. Com `FLINK_ASYNC_MAX_PARALLEL_REQUESTS=50`: 50 × 468 = 23.400 index lookups simultâneos → 26k IOPS.

**Anomalia:** `trigger_events_default` tem 1.4 GB de índices (783 MB + 616 MB) para 1 row — index bloat de quando a partição acumulava dados antes das partições diárias existirem.

---

## 14. Aurora PostgreSQL — particularidades

- `StorageType: aurora` + `Iops: None` = Aurora distributed storage; sem limite fixo de IOPS; billing por IO request
- `aws rds describe-db-instances` retorna `StorageType: aurora` mesmo para Aurora PostgreSQL — NÃO confundir com falta de provisioning
- IOPS muito altos (>10k) são possíveis e não indicam saturação se `disk_queue_depth < 1`
- Disk queue depth é o indicador correto de saturação em Aurora — se baixo, o banco está servindo o IO
- Performance Insights funciona igual ao RDS padrão (mesmo DbiResourceId, mesmo `aws pi` CLI)

---

## 15. Tabelas particionadas — diagnóstico específico

**Query para contar partições e tamanho total:**
```sql
SELECT count(*) AS partition_count,
       pg_size_pretty(sum(pg_total_relation_size(c.oid))) AS total_size
FROM pg_class c
JOIN pg_inherits i ON i.inhrelid = c.oid
JOIN pg_class p ON p.oid = i.inhparent
WHERE p.relname = 'trigger_events';
```

**Distribuição por mês:**
```sql
SELECT 
    substring(c.relname FROM 'trigger_events_p(\d{4}_\d{2})') AS year_month,
    count(*) AS partitions,
    pg_size_pretty(sum(pg_total_relation_size(c.oid))) AS size
FROM pg_class c
JOIN pg_inherits i ON i.inhrelid = c.oid
JOIN pg_class p ON p.oid = i.inhparent
WHERE p.relname = 'trigger_events'
  AND c.relname != 'trigger_events_default'
GROUP BY 1
ORDER BY 1 DESC;
```

**Contar Bitmap Index Scans no EXPLAIN (proxy para partições varridas):**
```bash
# Número de partições que a query vai varrer:
kubectl exec ... -- psql "..." -c "EXPLAIN ... ;" | grep -c "Bitmap Index Scan"
```

**Red flags em tabelas particionadas:**

| Situação | Diagnóstico | Fix |
|----------|-------------|-----|
| `N Bitmap Index Scans` no EXPLAIN sendo N ≈ total de partições | Query sem filtro na coluna de partição (ex: `event_time`) | Adicionar `WHERE col_partição >= NOW() - INTERVAL 'X days'` na query |
| Partição default com índices >>MB mas poucas rows | Index bloat pós-limpeza de dados históricos | `REINDEX INDEX CONCURRENTLY <idx>` na default — **sequencial, nunca dois em paralelo na mesma partição (deadlock garantido)** |
| N > 500 partições | Ausência de retenção/detach | `ALTER TABLE t DETACH PARTITION p_antiga` (sem lock) |
| Query JPA sem date range em tabela particionada | SELECT DISTINCT cross-partition explode IOPS | Forçar filtro na camada de repositório |

**Detach de partições antigas (sem lock, operação online):**
```sql
ALTER TABLE trigger_events DETACH PARTITION trigger_events_p2024_04_29;
-- A partição vira tabela independente — pode ser arquivada ou dropada depois
```

---

## 16. O que deu errado nesta sessão (não repetir)

1. **PSQL alias com variável shell não funciona no Bash tool:**
   ```bash
   # ❌ NÃO FAZER — erro "command not found"
   PSQL="kubectl exec ..."
   $PSQL -c "SELECT ..."

   # ✅ FAZER — comando direto sem alias em variável
   kubectl exec -n monitoring trigger-ops --context cobli-prod-devices -- \
     psql "host=... dbname=... user=..." -c "SELECT ..."
   ```

2. **`db.wait_event` como group-by no PI falha se PI não coleta esse detalhe:**
   Usar `db.wait_event_type` (funciona sempre) em vez de `db.wait_event` na primeira consulta. Só descer para `db.wait_event` se `db.wait_event_type` mostrar IO alto.

3. **Contexto k8s errado para Flink jobs:**
   App pods do trigger-action-api: `cobli-prod` namespace `monitoring`.
   Flink pods (trigger-engine JM + TM): `cobli-prod-devices` namespace `monitoring`.
   Criar o pod psql no mesmo contexto dos Flink pods.

4. **EXPLAIN com tipo de dado errado:** Ao montar EXPLAIN com UUID, usar cast explícito: `'...'::uuid`. Sem cast, erro `operator does not exist: uuid = integer`.

5. **`relname` ambíguo em JOINs com pg_class + pg_stat_user_tables:**
   Sempre qualificar: `c.relname`, `s.relname` etc.

6. **Dois `REINDEX INDEX CONCURRENTLY` em paralelo na mesma partição → deadlock imediato:**
   Ambos precisam de `ShareUpdateExclusiveLock` na relação — blocam um ao outro. Sempre rodar sequencialmente: disparar o segundo apenas após o primeiro concluir (`REINDEX` no log = sucesso).

---

## 17. Estratégias bem-sucedidas nesta sessão

1. **Descoberta do DbiResourceId:**
   ```bash
   aws rds describe-db-instances \
     --query "DBInstances[?contains(DBInstanceIdentifier,'<serviço>')].{id:DBInstanceIdentifier,ResourceId:DbiResourceId,endpoint:Endpoint.Address,class:DBInstanceClass,status:DBInstanceStatus}" \
     --output table
   ```

2. **PI top SQL com 24h + zoom no pico:**
   - Primeira: `now-24H`, `period 3600` → identifica janela crítica
   - Segunda: janela estreita do pico, `period 1800`, `Limit 15` → queries específicas
   - Terceira: mesma janela com `group-by db.wait_event_type` → confirma IO vs CPU

3. **Dimensionar impacto de particionamento:**
   ```bash
   EXPLAIN ... | grep -c "Bitmap Index Scan"
   # → 468 = 468 partições escaneadas por query
   # × FLINK_ASYNC_MAX_PARALLEL_REQUESTS → estimativa de IOPS máximo
   ```

4. **Senha do pod de app em vez de SSM:**
   `kubectl exec -- env | grep SPRING_DATASOURCE_PASSWORD` — mais rápido para diagnóstico ad-hoc.

5. **Detectar index bloat na partição default:**
   ```sql
   SELECT i.relname, i.indexrelname,
          pg_size_pretty(pg_relation_size(i.indexrelid)) AS size,
          t.n_live_tup
   FROM pg_stat_user_indexes i
   JOIN pg_stat_user_tables t ON t.relname = i.relname
   WHERE i.relname LIKE '%_default%'
   ORDER BY pg_relation_size(i.indexrelid) DESC;
   ```
   Ratio GB/row revela bloat imediatamente.
