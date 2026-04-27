---
name: add-public-doc
description: >
  Documenta um endpoint interno no repositório público api-docs (OpenAPI → ReadMe SaaS).
  Lê o handler via repoindex + grep no código, gera snippet YAML OpenAPI completo
  (path, method, operationId, summary, security, parameters, responses básicas) e abre
  PR no api-docs. Também detecta gaps (handlers sem cobertura pública) para o repo informado.
argument-hint: "<repo-name> <handler-path-or-operationId> [--gap-only]"
user-invocable: true
---

# add-public-doc — Documentar Endpoint Público no api-docs

Gera snippet OpenAPI para um handler interno e abre PR no `api-docs`.

- Sem `--gap-only`: modo geração — cria snippet + PR para o endpoint especificado.
- Com `--gap-only`: lista handlers sem cobertura pública (gap detection) e encerra.

---

## Fase 0 — Reconnaissance (sempre automática)

Execute em paralelo antes de qualquer pergunta:

```bash
# 1. Handler no repoindex
wtb repo show <repo-name> --section handlers --table 2>/dev/null | head -30

# 2. Endpoints já documentados para este serviço
wtb repo canvas <repo-name> 2>/dev/null | grep -A 30 "PUBLIC API SURFACE"

# 3. Estrutura atual do spec.yaml (seções por comentário)
gh api "repos/Cobliteam/api-docs/contents/spec/spec.yaml" \
  | python3 -c "import json,sys,base64; d=json.load(sys.stdin); print(base64.b64decode(d['content']).decode())" \
  | grep "^  #\|^  /"

# 4. Exemplo de path YAML do mesmo recurso (para inferir padrão)
# (buscar na seção correspondente do spec.yaml)
```

Se `--gap-only`: após Fase 0, listar gaps do Canvas e encerrar sem gerar snippet.

---

## Fase 1 — Identificar o handler

1. Buscar o handler no resultado do `wtb repo show`:
   - Verificar `trigger_detail` (path HTTP), `trigger_type`, `name` (nome da função/controller)
2. Se handler não encontrado no repoindex: grep direto no repo
   ```bash
   grep -rn "path.*<handler-path>\|mapping.*<handler-path>" ~/Cobliteam/<repo-name>/src/ \
     --include="*.kt" --include="*.java" | head -20
   ```
3. Confirmar com o usuário: "Encontrei o handler `<nome>` em `<arquivo>:<linha>`. Prosseguir?"

---

## Fase 2 — Extrair detalhes do handler

Com o arquivo e linha do handler, ler o código para extrair:

```bash
# Ler arquivo do handler (±30 linhas ao redor)
# Buscar: método HTTP, path completo, parâmetros (query/path/body), tipo de retorno
```

Inferir:
- **method**: anotação `@GetMapping`, `@PostMapping`, etc.
- **path completo**: `routePrefix` do Helm + path da anotação
- **query params**: `@RequestParam` com nome e tipo
- **path params**: `@PathVariable`
- **body**: `@RequestBody` → nome do DTO
- **response**: tipo de retorno → campos principais
- **auth**: verifica se rota está em `authRegexWhitelist` do Helm (sem auth) ou precisa de API key

---

## Fase 3 — Gerar snippet YAML OpenAPI

Montar o snippet seguindo o padrão do api-docs:

```yaml
# Arquivo: spec/paths/v1/<recurso>/<operacao>.yaml
get:  # ou post/put/patch/delete
  operationId: <camelCase, ex: listVehicles>
  summary: <Uma linha descritiva em português>
  description: >-
    <Descrição mais longa opcional>
  tags:
    - <Categoria, ex: Veículos>
  security:
    - APIKey: []   # omitir se rota pública sem auth
  parameters:
    - name: <param>
      in: query  # ou path
      required: false
      schema:
        type: string
      description: <descrição>
  responses:
    "200":
      description: <descrição do sucesso>
      content:
        application/json:
          schema:
            type: object
            properties:
              data:
                type: array
                items:
                  $ref: "../../../components/schemas/<Schema>.yaml"
    "401":
      description: Chave API inválida ou ausente
      content:
        application/json:
          schema:
            $ref: "../../../components/schemas/Error.yaml"
    "500":
      description: Erro interno do servidor
```

**Entrada no spec.yaml** (adicionar na seção correta por comentário `# <recurso> #`):
```yaml
  /public/v1/<recurso>:
    $ref: "paths/v1/<recurso>/<operacao>.yaml"
```

Mostrar ambos os snippets ao usuário para revisão antes de criar os arquivos.

---

## Fase 4 — Criar PR no api-docs

Após confirmação do usuário:

```bash
# Clonar api-docs se não existir localmente
[ ! -d ~/Cobliteam/api-docs ] && git clone git@github.com:Cobliteam/api-docs.git ~/Cobliteam/api-docs

cd ~/Cobliteam/api-docs
git fetch origin
git checkout -b feat/add-<operationId>-endpoint origin/master

# Criar o arquivo de path
mkdir -p spec/paths/v1/<recurso>/
cat > spec/paths/v1/<recurso>/<operacao>.yaml << 'EOF'
<snippet gerado na Fase 3>
EOF

# Adicionar entrada no spec.yaml na seção correta
# (usar Edit para inserir na posição certa, após o comentário da seção)

# Validar spec (se redocly CLI disponível)
npx @redocly/cli lint spec/spec.yaml 2>/dev/null || echo "redocly não disponível — validação manual necessária"

git add spec/paths/v1/<recurso>/<operacao>.yaml spec/spec.yaml
git commit -m "feat: add public endpoint <method> /public/v1/<recurso>"
git push -u origin feat/add-<operationId>-endpoint

gh pr create \
  --title "feat: document <operationId> endpoint" \
  --body "$(cat <<'PREOF'
## O que muda
- Adiciona endpoint \`<METHOD> /public/v1/<recurso>\` à spec OpenAPI pública

## Por que
Documentação pública do endpoint \`<handler-name>\` em \`<repo-name>\`.

## Como testar
1. Abrir https://docs.cobli.co/reference/ após merge
2. Verificar que o endpoint aparece na seção \`<categoria>\`

🤖 Generated with [Claude Code](https://claude.com/claude-code)
PREOF
)"
```

Retornar URL do PR ao usuário.

---

## Notas

- **Após o PR ser mergeado**: rodar `wtb repo import-openapi ~/Cobliteam/api-docs` para atualizar o repoindex
- **Gap detection rápida**: `wtb repo canvas <repo-name>` mostra seção `PUBLIC API SURFACE` com gaps
- **Validação da spec**: redocly CLI (`npx @redocly/cli lint`) verifica `$ref` quebrados e campos obrigatórios
- **Schemas de componentes**: se o endpoint expõe um novo tipo de resposta, criar `spec/components/schemas/<Nome>.yaml` antes do path file
