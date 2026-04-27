> 📍 [README](../../README.md) > Guides > Cycle Close

# Cycle Close — Session Exit

Procedimento para encerrar um ciclo de trabalho de forma estruturada.

## Comando principal

```bash
wtb cycle-check --save --repo <path>
```

## Sinais avaliados

| Sinal | Peso | Critério |
|-------|------|---------|
| `git_changes` | 2 | arquivos modificados desde HEAD |
| `tests_pass` | 3 | `go test ./...` passa |
| `build_pass` | 3 | binário compila |
| `time_elapsed` | 1 | ≥30min desde último savepoint |
| `packages_touched` | 1 | pacotes Go distintos modificados |

**Threshold:** 6/10 para savepoint automático.

## Ordem de execução (Session Exit Rule)

```mermaid
flowchart TD
    A[/pod-cleanup] --> B[memory-observer.sh]
    B --> C[editar topic files]
    C --> D[db-backup.sh]
    D --> E{DBs saudáveis?}
    E -->|sim| F[wtb cycle-check --save]
    E -->|não| G[investigar corrupção]
    F --> H[commit: só savepoint .md]
```

## Regra de commit

```bash
git add savepoints/YYYY-MM/savepoint-*.md
git commit -m "docs(savepoint): <contexto> YYYY-MM-DD"
```

Nunca bundlar outros arquivos modificados com o commit do savepoint.
