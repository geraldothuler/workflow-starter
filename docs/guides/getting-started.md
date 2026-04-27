> 📍 [README](../../README.md) > Guides > Getting Started

# Getting Started — workflow-toolkit

## Instalação

```bash
cd ~/workflow && go build ./cmd/wtb/... -o ~/bin/wtb
```

## Primeiro uso

```bash
wtb status           # estado atual
wtb cycle-check      # avalia ciclo
wtb backlog list     # tarefas ativas
wtb doc list         # artefatos
```

## Fluxo básico

```mermaid
flowchart LR
    A[session.yml] --> B[wtb status]
    B --> C{ciclo estável?}
    C -->|sim| D[wtb cycle-check --save]
    C -->|não| E[continuar iterando]
```

## Próximos passos

- [Ops Response](ops-response.md)
- [Cycle Close](cycle-close.md)
