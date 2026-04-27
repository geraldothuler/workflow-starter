---
name: kafka-topics
description: >
  Criar, descrever e listar tópicos Kafka no ambiente Cobli (prod e dev).
  Usa docker run --network host com imagem confluentinc/cp-kafka:7.6.3.
  Requer VPN ativa para acessar os brokers.
  Ativar quando o usuário pedir "cria tópico", "descreve tópico", "lista tópicos kafka",
  "criar kafka topic", "DLT topic", ou qualquer operação de administração de tópico Kafka.
user-invocable: true
---

# Kafka Topics — Administração

**Imagem:** `confluentinc/cp-kafka:7.6.3`
**Requisito:** VPN Cobli ativa + Docker rodando localmente
**Referência:** https://www.notion.so/cobli/Creating-kafka-topics-7ae4b9e76c8c43da847376d29f2cd305

---

## Bootstrap servers

| Ambiente | Brokers |
|----------|---------|
| **prod** | `kafka-msk-1.prod.aws.cobli.co:9092,kafka-msk-2.prod.aws.cobli.co:9092,kafka-msk-3.prod.aws.cobli.co:9092` |
| **dev** | verificar com `wtb memory get kafka` ou equipe de infra |

---

## Configurações padrão por ambiente

| Ambiente | `--replication-factor` | `--partitions` |
|----------|----------------------|----------------|
| prod | 3 | 10 (aumentar conforme necessidade) |
| dev | 1 | 3 |

---

## Fluxo 1 — Criar tópico

```bash
TOPIC="nome-do-topico"
BOOTSTRAP="kafka-msk-1.prod.aws.cobli.co:9092"
RF=3        # prod=3, dev=1
PARTITIONS=10  # prod=10, dev=3

docker run --rm --network host confluentinc/cp-kafka:7.6.3 \
  kafka-topics --create \
  --topic "$TOPIC" \
  --bootstrap-server "$BOOTSTRAP" \
  --replication-factor "$RF" \
  --partitions "$PARTITIONS"
```

**Após criar:** sempre confirmar com o Fluxo 2 (describe).

---

## Fluxo 2 — Descrever tópico (confirmar criação / ver partições e ISR)

```bash
TOPIC="nome-do-topico"
BOOTSTRAP="kafka-msk-1.prod.aws.cobli.co:9092"

docker run --rm --network host confluentinc/cp-kafka:7.6.3 \
  kafka-topics --describe \
  --topic "$TOPIC" \
  --bootstrap-server "$BOOTSTRAP"
```

Checar: `PartitionCount`, `ReplicationFactor` e que `Isr` == `Replicas` em todas as partições (ISR completo = saudável).

---

## Fluxo 3 — Listar tópicos

```bash
BOOTSTRAP="kafka-msk-1.prod.aws.cobli.co:9092"

docker run --rm --network host confluentinc/cp-kafka:7.6.3 \
  kafka-topics --list \
  --bootstrap-server "$BOOTSTRAP"
```

---

## Padrão de nomes DLT

Spring Kafka `DeadLetterPublishingRecoverer` publica por padrão em `<topic-original>.DLT`.
Ao criar um consumer com error handler + DLT, criar o tópico DLT junto:

```
status-event       → status-event.DLT
device-driver-identification → device-driver-identification.DLT
```

**Atenção:** tópicos com `.` (ponto) geram warning de colisão com `_` (underscore) no Kafka. É esperado — ignorar o warning se o nome for intencional.

---

## Histórico de tópicos criados

| Tópico | Partições | RF | Data | Contexto |
|--------|-----------|-----|------|---------|
| `status-event.DLT` | 10 | 3 | 2026-03-26 | DLT do consumer fusca-identification-token — permissão negada na tabela `identification_token_not_found_log` causava loop infinito |
