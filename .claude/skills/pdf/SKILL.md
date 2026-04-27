---
name: pdf
description: >
  Gera PDF a partir de um artefato existente (report de terminal, HTML, Markdown ou texto).
  Engine preferida: LaTeX (xelatex via BasicTeX) para máxima qualidade tipográfica.
  Fallback: typst (sem LaTeX). Output salvo em /tmp/ por padrão.
  Ativar quando o usuário pedir "gera PDF", "exporta PDF", "pdf disso".
user-invocable: true
---

# PDF — Geração de documentos

## Stack

- **pandoc 3.9** — conversão Markdown → PDF
- **xelatex** (BasicTeX) — engine preferida, qualidade tipográfica máxima
- **typst 0.14.2** — fallback quando LaTeX não disponível
- **Detecção automática de engine:**

```bash
ls /Library/TeX/texbin/xelatex &>/dev/null && ENGINE="/Library/TeX/texbin/xelatex" || ENGINE="typst"
```

> **ATENÇÃO:** `which xelatex` pode retornar vazio mesmo com BasicTeX instalado.
> Path real: `/Library/TeX/texbin/xelatex`. Sempre usar path completo no `--pdf-engine`.

## Instalação do LaTeX (uma vez)

```bash
brew install --cask basictex
sudo tlmgr update --self
sudo tlmgr install collection-fontsrecommended
```

---

## Procedimento

### 1. Identificar a fonte

O usuário pode apontar para:
- Texto apresentado no terminal (report, resumo, análise)
- Arquivo Markdown existente
- Arquivo HTML existente
- Conteúdo inline descrito no prompt

### 2. Preparar o Markdown intermediário

Sempre escrever Markdown limpo antes de chamar pandoc.
Regras de formatação:

### 2b. Revisão linguística — checklist BLOQUEANTE para pt-BR

**Executar antes de chamar pandoc.** Ler o Markdown de cima a baixo e verificar cada item.
Se qualquer item falhar → corrigir no arquivo antes de prosseguir.

**Acentuação (erros mais comuns):**
- [ ] Palavras com `~`: `informação`, `operação`, `configuração`, `função`, `gestão`, `ação`, `integração`, `conexão`, `versão`
- [ ] Palavras com acento agudo: `técnico`, `específico`, `público`, `único`, `índice`, `código`, `módulo`, `número`, `série`, `análise`, `métricas`, `tópico`, `básico`
- [ ] Palavras com acento circunflexo: `você`, `também`, `através`, `após`, `então`, `têm`, `são`, `estão`
- [ ] Palavras com cedilha: `serviço`, `configuração`, `ação`, `execução`, `exceção`, `posição`, `exibição`
- [ ] xelatex com UTF-8 suporta todos os acentos — nunca remover para "evitar erro LaTeX"

**Ortografia e gramática:**
- [ ] Sem palavras truncadas (ex: `configura` em vez de `configuração`, `tecnico` em vez de `técnico`)
- [ ] Concordância: substantivo + adjetivo no mesmo gênero/número
- [ ] Pontuação: vírgula antes de `mas`, `porém`; dois-pontos antes de lista

**Preservar sem alteração:**
- Termos técnicos ingleses: `BLoC`, `GraphQL`, `JWT`, `Protobuf`, `Kafka`, `Spring Boot`
- Nomes de variáveis/classes/métodos: `driverCode`, `SessionRepository`, `reverse_geocode()`
- IDs de artefatos e paths de sistema: `gf-core-mapa-t-cnico-...` permanece como está (não acentuar slugs)

> **Regra prática:** qualquer palavra portuguesa com mais de 4 letras que não tem acento é suspeita.
> Percorrer o texto procurando por elas antes de gerar.

- Título principal: `# Título`
- Subtítulos de seção: `## Seção`
- Subtítulos internos: `### Subseção`
- Tabelas: GFM (pipes alinhados) — ver seção "Regras para tabelas"
- Código/métricas inline: backtick simples (atenção ao overflow — ver abaixo)
- Listas de status: `- **Label:** valor`
- Separadores: linha em branco (não `---` dentro do conteúdo)

**Não usar:**
- Emojis (renderizam em branco no xelatex)
- HTML inline
- Links longos inline (usar footnotes se necessário)
- Unicode: `↔ ├ └ ─` → substituir por `<--> +-- +-- -` (incompatíveis com lmmono10)

### 3. Configuração pandoc — LaTeX (preferida)

```bash
pandoc input.md \
  --pdf-engine=/Library/TeX/texbin/xelatex \
  -V geometry:margin=2.5cm \
  -V fontsize=11pt \
  -V linestretch=1.4 \
  -V colorlinks=true \
  -V linkcolor=NavyBlue \
  -o output.pdf
```

**Para relatório executivo (mais compacto):**
```bash
pandoc input.md \
  --pdf-engine=/Library/TeX/texbin/xelatex \
  -V geometry:margin=2cm \
  -V fontsize=10pt \
  -V linestretch=1.3 \
  -V colorlinks=true \
  -o output.pdf
```

### 4. Configuração pandoc — Typst (fallback)

```bash
pandoc input.md \
  --pdf-engine=typst \
  -V fontsize=11pt \
  -o output.pdf
```

### 5. Metadados no Markdown

Sempre incluir bloco YAML no topo.
Para documentos com imagens (figuras matplotlib, etc.), adicionar `header-includes`:

```markdown
---
title: "Título do documento"
subtitle: "Subtítulo opcional"
date: "YYYY-MM-DD"
author: "Para: Nome"
lang: pt-BR
header-includes:
  - \usepackage{float}
  - \floatplacement{figure}{H}
---
```

> **NÃO usar** `-H <(echo ...)` no pandoc — falha com `ioError`. Sempre colocar em `header-includes` no YAML.

### 6. Gerar PDF

```bash
pandoc input.md --pdf-engine=/Library/TeX/texbin/xelatex [flags] -o output.pdf
```

Salvar em `/tmp/<nome-descritivo>-YYYY-MM-DD.pdf` ou no destino solicitado.

### 7. Loop de verificação visual (obrigatório)

**Nunca entregar o PDF sem verificar visualmente.** O loop garante que tabelas, diagramas e paginação estejam corretos antes de apresentar ao usuário.

#### Renderizar páginas

```bash
pdftoppm -r 120 -png output.pdf /tmp/chk
# Gera: /tmp/chk-01.png, /tmp/chk-02.png, ...
```

#### Páginas a inspecionar obrigatoriamente

| Página | O que verificar |
|:-------|:----------------|
| p.1–3 | Capa, sumário, início do conteúdo |
| Todas com tabelas largas | Overflow de colunas, conteúdo fora da borda |
| Todas com diagramas | Setas, sobreposição, corte de imagem |
| Última seção | Heading órfão, rodapé isolado |

Para documentos longos (> 10 páginas), inspecionar também as páginas no início de cada seção principal.

#### Checklist de aprovação

- [ ] Nenhuma célula de tabela com texto ultrapassando a borda da coluna
- [ ] Nenhum heading (`##`, `###`) isolado no fim da página sem conteúdo na mesma página
- [ ] Nenhuma imagem cortada ou fora da área visível
- [ ] Nenhuma sobreposição entre elementos
- [ ] Diagramas com setas conectando bordas de boxes (não passando por dentro)

#### Se reprovar algum critério

1. Identificar a linha do Markdown responsável
2. Aplicar correção (ver seção "Troubleshooting")
3. Regenerar PDF (passo 6)
4. Re-inspecionar as páginas afetadas
5. Repetir até aprovação em todos os critérios

---

## Regras para tabelas — evitar overflow

### Separadores proporcionais ao conteúdo (regra mais importante)

O pandoc controla a largura das colunas pela **proporção de dashes** nos separadores.
Separadores curtos com conteúdo longo → overflow garantido.

**Como dimensionar:** medir o conteúdo mais longo de cada coluna e usar dashes proporcionais.

```markdown
| Coluna A (máx 20 chars) | Coluna B (máx 45 chars) | Coluna C (máx 35 chars) |
|:------------------------|:----------------------------------------------|:----------------------------------|
```

Usar `:---` (left-align) sempre que a coluna tiver conteúdo variável.

### Backtick inline não quebra linha no LaTeX

Conteúdo em backtick (`` `código` ``) é renderizado como `\texttt{}` e **não quebra linha** em células de tabela — mesmo com separadores largos.

**Remover backtick quando:**
- Nome de classe/método > 25 chars em tabela (ex: `DriverVehicleAssociationController` → sem backtick)
- Lista de queries/mutations em célula de tabela
- Rotas de API com prefixo repetitivo (ver abaixo)

**Manter backtick quando:**
- Nomes de tabela de banco curtos (`user_events`, `triggers`)
- Nomes de tópicos Kafka (geralmente < 30 chars)
- Valores de configuração únicos e curtos

### Fatorar prefixo comum em rotas

Quando muitas linhas compartilham o mesmo prefixo de rota, extrair antes da tabela:

```markdown
> Prefixo base de todas as rotas: `/analytics/v1/`

| Controller | Sufixo da rota | Descrição |
|:----------------------------------------|:----------------------|:--------------------------------------|
| SafetyRankingController | safety-ranking | Ranking de segurança |
| DriverVehicleAssociationController | driver-vehicle-association | Associações motorista-veículo |
```

Em vez de repetir `/analytics/v1/driver-vehicle-association` (43 chars com backtick) em cada linha.

### Limite de colunas por conteúdo

| Colunas | Conteúdo total estimado | Viável? |
|:--------|:------------------------|:--------|
| 2 | qualquer | sempre |
| 3 | até ~110 chars somados | sim |
| 4 | até ~90 chars somados | com cuidado |
| 4+ com conteúdo longo | — | dividir em 2 tabelas |

### Quando a coluna ainda transborda após redimensionar — quebrar a tabela

Se após ampliar os separadores o conteúdo de uma célula ainda não couber (célula com > 40 chars sem backtick, ou qualquer backtick > 25 chars):

**Opção 1 — Dividir em 2 tabelas complementares** (preferida quando há coluna "Detalhe" longa):
```markdown
<!-- tabela principal: identificação -->
| Sistema | Protocolo | Rota principal |
|:--------|:----------|:---------------|
| herbie-geocoder | REST | POST /geolocation |

<!-- tabela de detalhes separada -->
| Sistema | Comportamento |
|:--------|:--------------|
| herbie-geocoder | Chamada pontual lat/lon → address string; usa RestTemplate, sem cache |
```

**Opção 2 — Converter coluna longa em lista após a tabela:**
```markdown
| Campo | Tipo |
|:------|:-----|
| reverse_geocode | função SQL |

**reverse_geocode:** combina planet_osm_line (OSM) com QUALOCEP_ENDERECO/BAIRRO/CIDADE
em 4 passos: EPSG:4326→3857, busca via 15m, bounding box CEPs 500m, JOIN QUALOCEP.
```

**Opção 3 — Linha de continuação na própria célula** (pandoc suporta com pipe table multiline):
Não disponível em GFM — usar opção 1 ou 2.

**Nunca:** deixar overflow visível no PDF aprovado. Toda tabela deve passar no check visual.

### Outras regras

| Regra | Detalhe |
|:------|:--------|
| Máximo 6–7 colunas | Mais que isso → dividir ou remover colunas menos relevantes |
| Cabeçalhos curtos | `% 12V` em vez de `% regime 12V` |
| Sem bold `**` em células | Ocupa espaço extra; usar nota de rodapé se necessário |
| Coluna "Observação" | Sempre a última e mais curta; texto ≤ 5 palavras |

---

## Regras para figuras matplotlib embutidas

| Regra | Detalhe |
|:------|:--------|
| Imagem sempre `{ width=100% }` | Evita "Float too large for page" |
| Figura alta (3+ subplots) | Usar `{ width=84% }` — reduz altura e garante título + figura na mesma página |
| Footer da figura: `y=-0.01` | `fig.text(0.5, -0.01, ...)` — evita sobreposição com último subplot |
| Não duplicar conteúdo | Se há tabela matplotlib na figura, não repetir como tabela Markdown |
| venv para matplotlib | `python3 -m venv /tmp/venv && /tmp/venv/bin/pip install matplotlib` |

### Anti-sobreposição em diagramas matplotlib (DFD, DER, fluxo)

**Regra geral:** calcular o espaço ocupado pelo texto antes de posicionar o box. Nunca posicionar dois boxes sem verificar que não se sobrepõem.

**Separação mínima entre boxes:**
- Horizontal: `GAP_H = 0.4` entre bordas de boxes adjacentes
- Vertical: `GAP_V = 0.35` entre bordas de boxes na mesma coluna
- Labels de seta: `my + 0.18` — nunca `my + 0.05` (fica dentro do box de destino)

**Texto dentro de box — regras de wrapping:**
```python
# Calcular altura necessária dado o número de linhas
def box_height(n_lines, fontsize=9):
    return max(0.7, n_lines * fontsize * 0.022 + 0.25)

# Nunca colocar mais de N chars por linha num box de largura w
# Regra prática: w=3.0 → max 28 chars/linha; w=4.0 → max 38 chars/linha
# Se o texto for mais longo, quebrar em múltiplas linhas antes de passar para box()
```

**Verificação de sobreposição antes de salvar:**
```python
# Após definir todos os boxes, checar pares adjacentes:
# Para cada par (b1, b2): assert b1['right'][0] + GAP_H <= b2['left'][0]
# Se falhar → aumentar figsize ou reduzir xlim do ax
```

**Checklist visual do diagrama (antes de incluir no guide.md):**
- [ ] Nenhum label de seta dentro de um box (sobreposição texto/caixa)
- [ ] Nenhum box parcialmente fora dos limites do ax (`xlim`, `ylim`)
- [ ] Nenhum texto de box ultrapassando a borda da caixa
- [ ] Setas apontando para bordas dos boxes, não para o centro
- [ ] Nenhuma seta cruzando por dentro de outro box

**Quando o DFD não couber na figura sem sobreposição:**
1. Aumentar `figsize` primeiro (ex: `(15, 9)` → `(18, 10)`)
2. Se ainda não couber: dividir em 2 diagramas (ex: DFD-upstream e DFD-downstream)
3. Nunca comprimir boxes nem reduzir `fontsize` abaixo de 7.5

---

## Regras para controle de paginação

| Regra | Detalhe |
|:------|:--------|
| Heading órfão | `\newpage` antes de `## Seção` que ficaria isolada no fim da página |
| Seção com 2–3 linhas isolada | Converter `## Heading` para `**bold inline**` — evita quebra de página automática antes do heading |
| `\newpage` em Markdown | Colocar em linha isolada — pandoc trata como raw LaTeX automaticamente |
| Seção longa | Se tem > 5 linhas de conteúdo, verificar se cabe na mesma página |

---

## Exemplos de uso

- `/pdf` — gera PDF do último artefato produzido na sessão
- `/pdf fusca report` — gera PDF do report do Fusca apresentado no terminal
- `/pdf <caminho/arquivo.md>` — converte arquivo existente

---

## Troubleshooting

| Problema | Solução |
|:---------|:--------|
| `xelatex not found` (mesmo com BasicTeX) | Usar path completo: `--pdf-engine=/Library/TeX/texbin/xelatex` |
| `xelatex not installed` | `brew install --cask basictex` + `sudo tlmgr install collection-fontsrecommended` |
| `Could not find typst` | `brew install typst` |
| `Could not find pandoc` | `brew install pandoc` |
| Tabela overflow (texto ultrapassa coluna) | Ampliar separadores `:---` + remover backticks de nomes longos |
| Heading órfão (título só na página) | `\newpage` antes do heading |
| Figura não exibida (typst) | typst não embute imagens externas — usar xelatex obrigatoriamente |
| Float too large | `header-includes: [\usepackage{float}, \floatplacement{figure}{H}]` + `{ width=100% }` |
| `-H <(echo ...)` falha com ioError | Usar `header-includes` no YAML frontmatter |
| Emojis em branco | Remover emojis do Markdown |
| Unicode em branco (↔ ├ └ ─) | Substituir por `<--> +-- +-- -` |
| Fonte LaTeX faltando | `sudo tlmgr install <pacote>` |
