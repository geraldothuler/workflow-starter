# Use Case: CI Fix — Diagnóstico e Correção de Falha de CI

**Tipo:** technical | **Engine:** heuristic + shell (gh CLI, docker CLI)

---

## Contrato do processo

> **NUNCA commitar um fix sem completar a auditoria completa.**
>
> Um fix parcial que falha custa mais (commits extras, CI loops) do que
> pesquisar direito na primeira vez.
>
> **NUNCA postar comentário, review ou merge com CI pending ou failing.**
> O sentinel não dorme.
>
> **Verificar hipóteses localmente antes de commitar.**
> `docker manifest inspect`, `java -version`, `apt-cache show` — custo zero,
> evita loop de tentativas.

---

## 1. Ler o log antes de qualquer hipótese {#read-log}

```bash
# GitHub Actions
gh run view <run_id> --log-failed --repo {owner/repo}

# CircleCI — se não tiver acesso à API, pedir ao operador o trecho com o erro
# O erro real está nas últimas linhas do step que falhou

# Verificar qual job falhou especificamente
gh api repos/{owner/repo}/commits/{sha}/statuses \
  --jq '.[] | {context, state, description}' | sort | uniq
```

**Regra:** sem o log, o diagnóstico é cego. Cada tentativa sem log é um commit perdido.

---

## 2. Classificar a falha {#failure-patterns}

### Padrões conhecidos

| Padrão no log | Tipo | Root cause | Fix |
|---------------|------|-----------|-----|
| `image not found` / `manifest unknown` | Docker base image | Imagem removida do registry | Atualizar FROM |
| `ubuntu-2204:YYYY.MM.N deprecated` | Machine image | CircleCI depreca versões ~12 meses | `ubuntu-2204:current` |
| `SecurityManager is deprecated` | JVM incompatibility | sbt/runtime chama `setSecurityManager` em Java 17+ | Instalar Java ≤ 11 |
| `bad constant pool index` | JVM incompatibility | Compilador Scala interno (2.12.x) incompatível com Java 17+ | Instalar Java ≤ 11 |
| `E: Package 'X' has no installation candidate` | apt missing | `apt-get install` sem `apt-get update` | Adicionar `apt-get update -qq` antes |
| `HTTP 404` em apt | apt stale cache | Versão cacheada foi superseded, repo atualizou | Adicionar `apt-get update -qq` antes |
| `orb not found` / `orb version deprecated` | CircleCI orb | Orb version removida do registry | Atualizar para versão atual |
| `aws: command not found` | AWS CLI | aws-cli orb desatualizado | Atualizar `circleci/aws-cli` orb |
| `Error: cannot find module` | Node deps | Cache de node_modules desatualizado | `npm ci` ou invalidar cache |

### Stack legado — mapa de compatibilidade conhecido

| Stack | Versão | Java compatível | Nota |
|-------|--------|----------------|------|
| sbt | 1.3.x | ≤ 11 | Scala 2.12.10 interno quebra com Java 17+ (`bad constant pool index`) |
| sbt | 1.5.x+ | ≤ 17 | SecurityManager com flag `-Djava.security.manager=allow` |
| sbt | 1.9.x+ | ≤ 21 | LTS atual para projetos novos |
| Scala | 2.11.x | ≤ 17 | Scala 2.11 EOL; funciona com Java 17 mas sbt 1.3 não |
| Scala | 2.12.x (≥ 2.12.15) | ≤ 21 | Suporte Java 17 adicionado em 2.12.15 |
| Play | 2.6.x | ≤ 11 | Framework EOL |
| Gradle | < 7.3 | ≤ 16 | Java 17 requer Gradle 7.3+ |

**Regra para stack antigo:** quando sbt < 1.5 ou Scala < 2.12.15, instalar Java 11 explicitamente.
`JAVA_TOOL_OPTIONS=-Djava.security.manager=allow` **não resolve** — suprime o SecurityManager
mas não a incompatibilidade de bytecode do Scala 2.12.10 interno.

---

## 3. Auditoria completa antes do primeiro commit {#audit-checklist}

> Auditar TODOS os recursos de uma vez. Não corrigir só o que causou o erro visível.

### CircleCI config.yml — checklist

- [ ] **Machine image**: `ubuntu-XXXX:YYYY.MM.N` → verificar se ainda existe
  ```bash
  # CircleCI depreca versões ~12 meses após lançamento
  # Versões de jan/2024 deprecadas ~jan/2025
  ```
- [ ] **Orbs**: verificar versão de cada orb (`aws-ecr`, `aws-cli`, `helm`, `aws-eks`, `github-cli`)
  ```bash
  # Orbs velhos: circleci/aws-ecr@6.x (atual: 9.x), circleci/aws-cli@2.x (atual: 4.x)
  ```
- [ ] **Docker executor images** (`cimg/openjdk:X`, `cimg/python:X`): verificar se tag existe
- [ ] **JVM default da machine image**: qual Java vem no executor? Compatível com o runtime?

### Dockerfile — checklist

- [ ] **FROM**: a tag base existe no registry?
  ```bash
  docker manifest inspect <image>:<tag>
  ```
- [ ] **Imagens depreciadas**: `openjdk:*` → migrar para `eclipse-temurin:*`
  ```
  openjdk:8-jre-alpine3.9    → eclipse-temurin:8-jre-alpine
  openjdk:11-jre             → eclipse-temurin:11-jre
  openjdk:17-jre             → eclipse-temurin:17-jre
  ```
- [ ] **Runtime de APK/APT**: o `apk add`/`apt-get install` ainda funciona? Precisa de `update` antes?

---

## 4. Verificar hipóteses localmente {#verify-commands}

```bash
# Verificar se imagem Docker existe
docker manifest inspect <image>:<tag>
# "no such manifest" → não existe → trocar

# Verificar Java disponível na máquina
java -version

# Verificar versão de pacote no apt (antes de tentar instalar)
apt-cache show openjdk-11-jdk 2>/dev/null | grep Version | head -3
# Se vazio: o pacote pode precisar de apt-get update primeiro

# Verificar se orb existe no CircleCI registry
curl -s "https://circleci.com/api/v2/orb/circleci/aws-ecr" | python3 -c "
import sys,json; d=json.load(sys.stdin); print(d.get('latest_version',''))
"

# Cross-check em repos similares do org
find ~/Cobliteam -name "config.yml" -path "*/.circleci/*" 2>/dev/null \
  | xargs grep -l "ubuntu-2204\|aws_ecr\|openjdk" 2>/dev/null \
  | head -10
# → verificar qual versão repos que estão funcionando usam
```

---

## 5. Aguardar CI verde {#poll-ci}

```bash
SHA=$(git rev-parse HEAD)
REPO="owner/repo"

echo "Aguardando CI do commit $SHA..."
for i in $(seq 1 80); do
  CONCLUSION=$(gh api "repos/$REPO/commits/$SHA/check-runs" \
    --jq '.check_runs[] | select(.name=="test_build_publish_workflow") | .conclusion' 2>/dev/null)
  STATUS=$(gh api "repos/$REPO/commits/$SHA/check-runs" \
    --jq '.check_runs[] | select(.name=="test_build_publish_workflow") | .status' 2>/dev/null)
  echo "[$(date +%H:%M:%S)] status=$STATUS conclusion=$CONCLUSION"
  if [ "$CONCLUSION" = "success" ] || [ "$CONCLUSION" = "failure" ] || [ "$CONCLUSION" = "cancelled" ]; then
    echo "DONE: $CONCLUSION"
    break
  fi
  sleep 30
done
```

**Regra:** só prosseguir (comentário, review, merge) quando `conclusion = success`.

---

## 6. Regras de commit para CI fix {#commit-rules}

- **Um commit** consolidado com todos os fixes da auditoria — não uma sequência
- **Mensagem** explica o root cause, não o sintoma:
  - ✅ `ci: instalar Java 11 — sbt 1.3.2 + Scala 2.12.10 interno incompatível com Java 17`
  - ❌ `ci: fix build`
- **Body do commit** lista cada item corrigido e por quê

---

## 7. Caso de referência: severino SS-2204 / PR #380 {#reference}

**Contexto:** PR de feature com CI failing em `publish_docker_image`. `test` passou. Correção levou 5 commits.

**Linha do tempo de erros:**

| # | Commit | Hipótese | Resultado | Causa do erro |
|---|--------|---------|-----------|---------------|
| 1 | machine image → `current` | `ubuntu-2204:2024.01.1` depreciado | `publish_docker_image` ainda falhou | Auditoria incompleta: não verificou base image do Dockerfile |
| 2 | Dockerfile `openjdk:8-jre-alpine3.9` → `eclipse-temurin:8-jre-alpine` | Base image removida do Docker Hub | `publish_docker_image` ainda falhou | Auditoria incompleta: não verificou JVM do executor |
| 3 | `find Java 11 via filesystem` | Java 11 pré-instalado na imagem | Mesma falha | `find` retornou vazio silenciosamente; sbt rodou com Java 17 do SO |
| 4 | `JAVA_TOOL_OPTIONS=-Djava.security.manager=allow` | Java 17 + SecurityManager flag | Mesma falha | Pesquisa insuficiente: flag suprime SecurityManager mas não incompatibilidade de bytecode do Scala 2.12.10 interno |
| 5 | `apt-get install openjdk-11-jdk` (sem update) | Java 11 disponível no ubuntu-22.04 | 404 em apt | Cache do apt desatualizado — versão cacheada foi superseded |
| 6 | `apt-get update && apt-get install openjdk-11-jdk` | — | aguardando | — |

**Root cause completo (multi-camada):**

```
ubuntu-2204:2024.01.1 depreciado
  → ubuntu-2204:current como fix
  → ubuntu-2204:current = Java 17 default
  → sbt 1.3.2 usa Scala 2.12.10 interno
  → Scala 2.12.10 incompatível com Java 17 (bad constant pool index)
  → fix: instalar Java 11 via apt (com apt-get update)
```

**Lição estrutural:** stack antigo (sbt 1.3.x + Scala 2.11.x + Play 2.6.x) tem
incompatibilidades em cascata com infraestrutura moderna. Auditar toda a cadeia
JVM → sbt → Scala antes de qualquer fix de CI neste repositório.

**O que o fix de um commit teria sido:**

```yaml
# .circleci/config.yml — um único commit com as 3 mudanças

# 1. machine image
image: ubuntu-2204:current        # era: ubuntu-2204:2024.01.1

# 2. install Java 11 (nova etapa)
- run:
    name: Instalar Java 11 (sbt 1.3.2 + Scala 2.12.10 interno incompatível com Java 17)
    command: |
      sudo apt-get update -qq
      sudo apt-get install -y openjdk-11-jdk
      echo "export JAVA_HOME=/usr/lib/jvm/java-11-openjdk-amd64" >> $BASH_ENV
      echo 'export PATH=$JAVA_HOME/bin:$PATH' >> $BASH_ENV
      source $BASH_ENV
      java -version

# docker/Dockerfile — mesma mudança
FROM eclipse-temurin:8-jre-alpine  # era: openjdk:8-jre-alpine3.9
```

---

## 8. Handoffs possíveis

```
ci-fix
  └─► code-review    ← retoma o review após CI verde
  └─► ci-watch       ← confirma CI antes de liberar
  └─► postmortem     ← se o loop foi custoso (≥ 3 commits de tentativa)
```

---

## 9. Captura de lição nova {#capture-lesson}

Quando uma falha não está coberta na tabela de padrões (seção 2):

1. Identificar o padrão no log (string detectável)
2. Identificar o root cause
3. Documentar o fix verificado
4. Adicionar linha na tabela em `#failure-patterns`
5. Commitar em `use-cases/ci-fix/guide.md`

Isso é o embrião da **Fase 2**: quando o detector automático (`pkg/cycles`) tiver
sinais suficientes, a captura passa de manual para proposta automática.
