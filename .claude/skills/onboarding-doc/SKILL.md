---
name: onboarding-doc
description: >
  Gera documento PDF de onboarding técnico completo para um squad, a partir de análise
  direta dos repositórios. Produz mapa de repos, DER, fluxos, API REST, Kafka e métricas
  em um único PDF navegável.
user-invocable: true
---

# onboarding-doc — Mapa Técnico de Squad

## Quando usar

- `/onboarding-doc <squad>` — gera mapa técnico completo de um squad
- Solicitação de "mapa técnico", "doc de onboarding", "visão geral de repos"

**Argumento:** nome do squad ou lista de repos explícita.

---

## Processo

### 1. Levantamento de repos

Para cada repo do squad, coletar:

| Campo | Onde buscar |
|:------|:------------|
| Stack (linguagem, framework, versão) | `build.gradle.kts`, `package.json`, `pom.xml`, `requirements.txt` |
| Storage | `application.conf`, `helm/`, migrations |
| Função (1–2 linhas) | README, `application.conf`, comentários de entry point |
| Sub-time | README, CODEOWNERS, contexto do squad |

### 2. Por repo — coletar detalhes

| O que | Onde buscar |
|:------|:------------|
| Schema do banco (DER) | Migrations Flyway (`V*.sql`), entidades JPA (`@Entity`), modelos Slick/Quill/Exposed |
| API REST | Controllers (`@RestController`, `Router`, `routes.conf`, `@app.route`) |
| Kafka consumers | `@KafkaListener`, `ConsumerFactory`, `KafkaConsumer` |
| Kafka producers | `KafkaTemplate`, `ProducerFactory`, `KafkaProducer` |
| Métricas | `statsd`, `StatsD`, `kamon`, `@Timed`, `datadog-lambda` |
| Dependências up/downstream | `OpenFeign`, `WebClient`, `RestTemplate`, imports externos |
| Flink: pipelines | `KeyedProcessFunction`, `ProcessFunction`, fontes e sinks de tópicos |

### 3. Gerar diagramas PNG (matplotlib)

Setup do venv:

```bash
python3 -m venv /tmp/diag-venv
/tmp/diag-venv/bin/pip install matplotlib -q
/tmp/diag-venv/bin/python /tmp/gen_diagrams.py
```

#### Tipos de diagrama

| Diagrama | Quando incluir |
|:---------|:---------------|
| Arch overview | sempre — todos os repos do squad em um único diagrama |
| DER por repo | quando há schema não-trivial (> 3 tabelas ou relações importantes) |
| Flow por domínio | quando há pipeline Flink, ETL ou fluxo multisserviço |
| Auth flow | quando há gateway/proxy de autenticação |

#### Template obrigatório para DER

Setas sempre de borda a borda — nunca com coordenadas hardcoded dentro dos boxes.

```python
HDR_H, ROW_H, GAP = 0.42, 0.30, 0.25

def entity_height(n_fields):
    return HDR_H + n_fields * ROW_H

def draw_entity(ax, x, y, title, fields, w=3.8, fc='#EBF5FB', ec='#2980B9'):
    h = entity_height(len(fields))
    # FancyBboxPatch + texto de campos
    # Retornar midpoints das bordas — obrigatório
    return {
        'top':    (x + w/2, y + h),
        'bottom': (x + w/2, y),
        'left':   (x,       y + h/2),
        'right':  (x + w,   y + h/2),
    }

# Layout: calcular posições bottom-up por coluna
y = 0.3
for entity in col1:
    pts[entity['name']] = draw_entity(ax, COL1_X, y, ...)
    y += entity_height(len(entity['fields'])) + GAP

# Setas: sempre de edge midpoint para edge midpoint
ax.annotate('', xy=pts['B']['left'], xytext=pts['A']['right'],
            arrowprops=dict(arrowstyle='->', color='#555'))
```

### 4. Montar Markdown

Arquivo: `/tmp/<squad>-technical-map-YYYY-MM-DD.md`

#### Estrutura obrigatória

**Regra de ordenação (validada no GF Core onboarding):**
- Squads descritos inline no Contexto — não como seções separadas antes da tabela de repos
- Tabela de repos com coluna Squad — todos os repos juntos em uma única tabela
- Detalhes por repo agrupados depois da tabela — nunca intercalados
- Pontos de atenção e sugestões sempre ao fim, antes das referências

```markdown
---
title: "Squad <X> — Mapa Técnico"
subtitle: "Onboarding técnico completo"
date: "YYYY-MM-DD"
lang: pt-BR
header-includes:
  - \usepackage{float}
  - \floatplacement{figure}{H}
---

# Contexto
(propósito do squad, 3–5 linhas; sub-times descritos inline como bullet list — não como seções)

## Recursos Externos para Aprendizado

Roadmap.sh oferece guias estruturados por função e tecnologia. Use conforme o perfil do squad:

| Perfil | Roadmaps Recomendados | Link |
|--------|----------------------|------|
| Backend (Go, Kafka, databases) | Backend Developer, Go, Docker, Kubernetes | https://roadmap.sh/backend |
| DevOps (K8s, Terraform, monitoring) | DevOps Engineer, Docker, Kubernetes | https://roadmap.sh/devops |
| Frontend (TypeScript, React, NX) | Frontend Developer, TypeScript, React | https://roadmap.sh/frontend |
| Full Stack | Full Stack Developer | https://roadmap.sh/full-stack |

**Nota:** roadmap.sh oferece padrões genéricos. Valide sempre com `wtb memory get <topic>` para padrões Cobli específicos (conventions, precommit hooks, migrations, etc).

\newpage

# Repositórios

(tabela 5 colunas: Repo | Squad | Stack | Função | Storage — todos os repos em uma tabela única)

![Visão Geral](/tmp/diag-arch-overview.png){ width=100% }

## Fluxo Geral — <Domínio>
(um diagrama por domínio principal)

\newpage

# Detalhes por Repositório

## <repo-1>
(Stack, Função, DER, API REST, Kafka, Métricas, Dependências)

## <repo-2>
...

\newpage

# Pontos de Atenção e Sugestões
(Riscos Ativos, Dívida Técnica, Descobertas Positivas, Recomendações)

\newpage

# Referências
```

### 5. Gerar PDF com verificação visual

Seguir a **skill `pdf`** integralmente — inclui:
- Configuração pandoc + xelatex
- Regras de tabelas (separadores proporcionais, backtick overflow, prefixo de rotas)
- Loop obrigatório: `pdftoppm` → checklist de aprovação → corrigir → regenerar

```bash
pandoc /tmp/<squad>-technical-map-YYYY-MM-DD.md \
  --pdf-engine=/Library/TeX/texbin/xelatex \
  -V geometry:margin=2.5cm \
  -V fontsize=10pt \
  -V linestretch=1.3 \
  -V colorlinks=true \
  -V linkcolor=NavyBlue \
  -o ~/workflow/docs/<squad>-technical-map-YYYY-MM-DD.pdf
```

**Output:** `open ~/workflow/docs/<squad>-technical-map-YYYY-MM-DD.pdf`

---

### 6. Registrar discovery no docs.db

Após gerar o PDF, criar entrada no docs.db com a mesma estrutura do PDF (sem formatação LaTeX):

```bash
wtb doc add \
  --type discovery \
  --title "<Squad> — Mapa Técnico Completo de Repositórios" \
  --date YYYY-MM-DD \
  --tag "onboarding,<squad>,<tags-de-domínio>" \
  --content "# <Squad> — Mapa Técnico Completo de Repositórios

## Contexto
(propósito, squads inline como bullet list)

## Repositórios
(tabela com colunas: Repo | Squad | Stack | Função | Storage)

## Detalhes por Repositório
### <repo-1>
...
### <repo-2>
...

## Pontos de Atenção e Sugestões
(numerados, com contexto)

## Referências
(IDs de outros docs, paths de artefatos)"
```

**Estrutura obrigatória para o conteúdo da discovery (validada GF Core 2026-04-01):**
- Contexto: squads descritos inline como bullet list — nunca como seções `###` antes da tabela
- Tabela única com coluna Squad — todos os repos juntos, nunca divididos por sub-time
- Detalhes por repo: seções `###` agrupadas após a tabela — nunca intercaladas com a tabela
- Pontos de Atenção e Sugestões: sempre ao fim, antes das referências

---

## Referência de qualidade

Primeiro mapa gerado: `gf-core-technical-map-2026-03-31.pdf`
- 19 repos, 57 páginas, 8 diagramas PNG
- Disponível em `~/workflow/docs/` como referência visual
