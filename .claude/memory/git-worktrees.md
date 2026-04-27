---
name: Git worktrees — padrão de uso
description: Quando e como usar git worktrees para trabalho paralelo em branches, especialmente em repos com build pesado
type: reference
originSessionId: 85bd1d39-67c3-4ded-99a5-5f18d62fc621
---
## Quando usar

Repos com build pesado (webhook-builder, fusca) onde há necessidade concorrente real:
- Iterando com CodeRabbit num PR enquanto precisa investigar prod no `main`
- Aplicando hotfix urgente sem descartar work-in-progress

**Não vale para:** herbie-dashboard, janus, ai (builds leves — `git stash` resolve).

## Comandos

```bash
# Criar worktree para nova branch a partir do origin
git worktree add ../webhook-builder-hotfix origin/main -b hotfix/SS-XXXX

# Criar worktree para branch existente
git worktree add ../webhook-builder-feat feat/SS-XXXX

# Listar worktrees ativos
git worktree list

# Remover ao finalizar (branch deve estar sem alterações uncommitted)
git worktree remove ../webhook-builder-hotfix
```

## Tradeoffs

| Aspecto | Detalhe |
|---------|---------|
| Builds independentes | Cada worktree compila separado — sem recompilar ao trocar branch |
| Disco | Cópia do working tree (não do .git) — ~tamanho do repo por worktree |
| Submódulos | Precisam ser inicializados em cada worktree (`git submodule update --init`) |
| Um branch = um worktree | Não é possível ter o mesmo branch em dois worktrees simultaneamente |

## Submódulos (webhook-builder tem schema-registry)

```bash
cd ../webhook-builder-hotfix
git submodule update --init --recursive
```

## Agent tool

O parâmetro `isolation: "worktree"` do Agent tool usa worktrees internamente — já é o padrão para agentes autônomos que precisam de branches isolados.

## Worktrees ativos — todos os repos (padrão desde 2026-04-11)

Convenção: dir principal = master/main; feature branches = worktree sufixado com nome curto do branch.
**Exceção**: herbie-dashboard — dir principal está no feature branch (untracked files bloquearam switch); worktree `-master` tem o master.

| Repo (dir principal = master) | Worktree(s) de feature |
|-------------------------------|------------------------|
| `~/trigger-action-api` | `-gf163` (feat/GF-163-fix), `-gf13` (GF-13), `-gf17` (GF-17) |
| `~/iris` | `-SS-2269-get-event-types` |
| `~/fusca` | `-identification-token-hpa-maxreplicas-10` |
| `~/icarus` | `-icarus-min-replicas` |
| `~/severino` | `-user-service-spec-flaky-test` |
| `~/jaiminho-sms` | `-fusca-get-device-by-id` |
| `~/herbie-api` | `-vehicles-normalize-stage3` |
| `~/herbie-dashboard` (feature) | `-master` (master) |
| `~/janus` | `-gf-13-gf-17-camera-media-trigger-null-fix` |
| `~/ai` | `-batch-create-rfid` |

**Regra geral:** sempre trabalhar no worktree do branch correto — nunca editar no diretório errado.
Ao criar novo branch: `git -C <repo-principal> worktree add ~/<repo>-<nome> <branch>`.

## CLI: `wt` (~/workflow/bin/wt, já no PATH)

```bash
wt ls                              # lista todos os worktrees de todos os repos
wt add <repo> <branch>             # cria ~/<repo>-<short>
wt rm  <repo> <short> [--branch]   # remove worktree (--branch apaga branch local)
wt clean                           # remove worktrees com remote branch deletado (interativo)
wt go  <repo> [<short>]            # cd no worktree certo (shell function no ~/.zshrc)
```

`wt path` (interno) faz fuzzy match: `gf163`, `GF-163`, `gf-13` resolvem o dir correto.
Adicionar novo worktree ao finalizar branch do PR: `wt rm <repo> <short> --branch`.
