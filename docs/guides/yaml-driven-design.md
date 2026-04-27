> 📍 [README](../../README.md) > Guides > YAML-Driven Design

# YAML-Driven Design

Princípio central da plataforma: lógica variável por contexto vive em YAML, não em Go.

## Gate de decisão

```
Pode ser um shell command composto? → YAML
Precisa de OpsResult estruturado?  → Go
Envolve parsing JSON complexo?     → Go
```

## Estrutura de um use-case

```
use-cases/<tipo>/
├── definition.yml   ← entradas, steps, engine
└── guide.md
```

## Exemplo: definition.yml

```yaml
id: ops-response
type: pipeline
inputs:
  - name: symptom
    required: true
    resolve: [session, ask]
steps:
  - name: probe_environment
    engine: pkg/ops
```

## No-hardcode rule

Vars de ambiente em `definition.yml` → sempre `""`.
Resolução em runtime: `session.yml` → env var → prompt.
