---
name: Nunca usar "pré-existente" como desculpa para falha de teste
description: Proibido descartar falhas de CI/teste local como "pré-existentes" — toda falha deve ser resolvida ou escalada
type: feedback
originSessionId: 85bd1d39-67c3-4ded-99a5-5f18d62fc621
---
Nunca usar "pré-existente" para justificar falhas de teste ou CI que aparecem durante a validação local.

**Why:** Isso mascara problemas reais e passa a impressão de que o ambiente está degradado quando pode ser um problema nosso. Geraldo fica cego para regressões verdadeiras.

**How to apply:**
- Se um teste falha durante `./gradlew test`, investigar a causa — independentemente de quando o teste foi escrito
- Se a falha for de infra (Cassandra, Scylla, Kafka não rodando), dizer explicitamente: "teste X falha porque ScyllaDB não está no ar — rodar `docker-compose up cassandra` antes" e propor a solução
- Se a falha for desconhecida, reportar com stack trace e aguardar instrução
- NUNCA escrever frases como "são falhas pré-existentes", "já falhava antes", "não é relacionado ao nosso PR"
- Se o ambiente estiver incompleto, descrever o que está faltando e perguntar como prosseguir
