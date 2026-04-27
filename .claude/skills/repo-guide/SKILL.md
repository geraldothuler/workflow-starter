---
name: repo-guide
description: >
  Gera guia técnico completo de um repositório: DFD, DER, fluxo por operação,
  upstream/downstream, particularidades. Output: PDF em ~/workflow/artifacts/<repo>/.
  Usar quando o usuário pedir "guia do repo X", "me explica o X", "doc técnica do X".
user-invocable: true
---

# repo-guide — Guia Técnico de Repositório

## Quando usar

- `/repo-guide <nome-do-repo>` — gera guia completo
- Pedidos como "me explica o bolovo", "cria doc técnica do herbie-toy", "guia do iris"

**Argumento:** nome do repo conforme cadastrado no repoindex (`wtb repo list`).

---

## Protocolo anti-alucinação — OBRIGATÓRIO

> Toda afirmação no guia deve ter evidência direta no código ou nos arquivos do repo.
> Se não for verificável, marcar com `[investigar]` e mover para "Pontos a verificar".
> **Nunca inferir** com base em: stack típica, padrão comum do time, "faz sentido que use".

### Padrões de alucinação confirmados (não repetir)

| Padrão | Alucinação típica | Como verificar |
|--------|-------------------|----------------|
| Cliente HTTP | "usa Feign" porque outros Kotlin usam Feign | Grep `@FeignClient`, `RestTemplate`, `WebClient` no código |
| Reverse geocode | "usa Nominatim" porque é o padrão OSS | Ler a query/função SQL ou o service impl que faz o geocode |
| Upstream callers | listar serviços que "logicamente chamariam" | Grep no código dos suspeitos pelo nome/host deste repo; nunca assumir |
| Serialização Kafka | "usa Protobuf" porque o resto do time usa | Ver `@KafkaListener` value deserializer ou `build.sbt` schema-registry |
| Storage | "usa Redis" porque a stack tem Redis | Ler `application.conf`/`application.yml` — só listar o que estiver configurado |
| Batch vs pontual | "chama em lote" porque seria mais eficiente | Ler o service/repository impl — pontual vs batch é detalhe crítico |

### Regra de verificação por seção

**Downstream (o que este repo chama):**
- Para cada entrada em `external_apis` do repoindex: ler a classe que faz a chamada
- Verificar: método HTTP, endpoint real, se é lote ou pontual, cliente HTTP usado
- Dúvida em qualquer campo → `[investigar: ler <Classe>.<método>]`

**Upstream (quem chama este repo):**
- **Proibido inferir.** Evidência aceita: (a) grep do nome/host deste repo em repos vizinhos, (b) `wtb doc search`, (c) declaração explícita em README/Helm ingress
- Se não encontrou evidência: "Callers não confirmados — investigar via grep nos repos suspeitos"
- Helm `ingress` / `routePrefix` confirma exposição pública — não confirma quem chama

**Tecnologia de integração:**
- HTTP client: grep obrigatório antes de escrever (`@FeignClient` / `RestTemplate` / `WebClient` / `OkHttp`)
- Kafka serde: ler o consumer config ou `@KafkaListener` annotation antes de escrever "Protobuf"
- Cache: só listar se tiver `@Cacheable`, Redis config, ou Hive box declarado no código

### Marcador de incerteza

Quando um campo não pode ser verificado antes de escrever o guia, usar:

```
[investigar: <o que ler e onde>]
```

Exemplo: `[investigar: ler GeocodingRequesterImpl — confirmar se é Feign ou RestTemplate]`

Concentrar todos os `[investigar]` numa seção final do guia:

```markdown
## Pontos a verificar

- [ ] <claim> — verificar lendo <arquivo>:<linha aproximada>
```

---

## Processo

### 0. Verificar repoindex

```bash
wtb repo show <repo>   # JSON completo: handlers, models, external_apis, config_vars
wtb repo list          # confirmar nome exato se necessário
```

Se o repo não estiver indexado: indexar primeiro com `wtb repo index <repo> --verbose`.

### 1. Coletar dados estruturados

Do `wtb repo show <repo>` extrair:

| Campo | Uso no guia |
|-------|------------|
| `repo.lang` / `repo.framework` | Seção stack |
| `repo.owner` | Seção visão geral |
| `handlers` | Seção fluxo por operação + DFD |
| `models` | Seção DER |
| `external_apis` | Seção downstream + DFD |
| `config_vars` | Seção particularidades |
| `deployment_units` | Seção deploy (repos k8s) |
| `chart_snapshots` | Recursos k8s (CPU, mem, replicas) |

### 2. Ler arquivos-chave do repo

Ler conforme stack — sempre verificar se o arquivo existe antes:

**Flutter/Dart:**
```
pubspec.yaml            → versão, dependências críticas (graphql_flutter, flutter_bloc, firebase)
catalog-info.yaml       → owner, lifecycle, links Firebase/CircleCI
Makefile                → comandos release-tags, deploy, test
README.md               → contexto, setup, links lojas
lib/main.dart           → providers registrados, módulos inicializados
lib/modules.dart        → service locator, dependências injetadas
```

**Kotlin/Spring Boot:**
```
src/main/resources/application.conf ou application.yml
deploy/helm/values.yaml + values-prod.yaml
src/main/kotlin/**/Application.kt
Makefile ou build.gradle.kts (versão)
```

**Scala/Flink ou Play:**
```
build.sbt               → versão, dependências
src/main/resources/application.conf
conf/routes             → endpoints (Play)
```

**Node/TypeScript:**
```
package.json            → versão, scripts, dependências
src/mastra/index.ts ou src/server.ts
```

### 3. Gerar diagramas matplotlib

Setup do venv (sempre):
```bash
python3 -m venv /tmp/rg-venv
/tmp/rg-venv/bin/pip install matplotlib -q
```

Output: `/tmp/rg-<repo>/` — criar o diretório antes.

#### 3a. DFD Nível 1 — sempre gerar

Mostra o app/serviço no centro, com setas para cada sistema externo (APIs, BDs, queues).
Agrupar external_apis por domínio (mesmo host ou mesma tecnologia).

Template base:
```python
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches
from matplotlib.patches import FancyBboxPatch, FancyArrowPatch

fig, ax = plt.subplots(figsize=(14, 9))
ax.set_xlim(0, 14); ax.set_ylim(0, 9); ax.axis('off')

# Cores por tipo
COLORS = {
    'app':     '#2980B9',   # azul — o repo em si
    'rest':    '#27AE60',   # verde — REST APIs
    'graphql': '#8E44AD',   # roxo — GraphQL
    'storage': '#E67E22',   # laranja — storages (Hive, SharedPrefs, Firebase)
    'infra':   '#7F8C8D',   # cinza — Firebase, LaunchDarkly, OneSignal
}

def box(ax, x, y, w, h, label, color, fontsize=10):
    patch = FancyBboxPatch((x, y), w, h,
        boxstyle="round,pad=0.1", fc=color, ec='white', lw=1.5, alpha=0.9, zorder=2)
    ax.add_patch(patch)
    ax.text(x + w/2, y + h/2, label, ha='center', va='center',
            fontsize=fontsize, color='white', fontweight='bold', zorder=3, wrap=True)
    return {'left': (x, y+h/2), 'right': (x+w, y+h/2),
            'top': (x+w/2, y+h), 'bottom': (x+w/2, y)}

def arrow(ax, src, dst, label='', color='#555'):
    ax.annotate('', xy=dst, xytext=src,
        arrowprops=dict(arrowstyle='->', color=color, lw=1.5))
    mx, my = (src[0]+dst[0])/2, (src[1]+dst[1])/2
    if label:
        ax.text(mx, my+0.12, label, ha='center', fontsize=8, color=color)
```

Regras de layout DFD:
- App/serviço central: `x=5.5, y=3.8, w=3, h=1.4`
- APIs à direita: coluna `x=10`, distribuídas verticalmente
- Storages locais à esquerda: coluna `x=0.5`
- Infra (Firebase, LD) abaixo: linha `y=0.5`
- Setas da borda do box central para a borda do destino — nunca de centro a centro com coordenadas hardcoded

#### 3b. DER — gerar somente se há models com campos

Flutter: incluir apenas models com `dialect=domain` e campos definidos (ignorar `bloc-state`).
Kotlin/Scala: usar models com `dialect=postgres` ou `dialect=cassandra`.

Template DER (mesmo padrão do onboarding-doc):
```python
HDR_H, ROW_H, GAP = 0.42, 0.30, 0.25

def entity_height(n_fields):
    return HDR_H + n_fields * ROW_H

def draw_entity(ax, x, y, title, fields, w=3.8, fc='#EBF5FB', ec='#2980B9'):
    h = entity_height(len(fields))
    patch = FancyBboxPatch((x, y), w, h,
        boxstyle="round,pad=0.05", fc=fc, ec=ec, lw=1.5, zorder=2)
    ax.add_patch(patch)
    ax.text(x + w/2, y + h - HDR_H/2, title, ha='center', va='center',
            fontsize=9, fontweight='bold', color='#1A252F', zorder=3)
    ax.axhline(y + h - HDR_H, xmin=(x)/ax.get_xlim()[1],
               xmax=(x+w)/ax.get_xlim()[1], color=ec, lw=1, zorder=3)
    for i, field in enumerate(fields):
        fy = y + h - HDR_H - (i + 0.65) * ROW_H
        ax.text(x + 0.15, fy, field, fontsize=7.5, va='center', color='#2C3E50', zorder=3)
    # retornar midpoints de borda
    return {
        'right':  (x + w, y + h/2),
        'left':   (x,     y + h/2),
        'top':    (x + w/2, y + h),
        'bottom': (x + w/2, y),
    }
```

#### 3c. Fluxo por BLoC/Área — gerar para os 3-5 fluxos mais importantes

Diagrama de sequência simplificado (colunas = atores, linhas = chamadas).
Selecionar fluxos de maior relevância operacional:
- Flutter: AssociationBloc, fluxo de login, fluxo principal do app
- Kotlin: endpoints críticos (POST/PUT de negócio, consumers Kafka)
- Priorizar fluxos mencionados em backlog/BUG aberto

### 4. Compor o markdown

Arquivo: `/tmp/rg-<repo>/guide.md`

```markdown
# <NomeRepo> — Guia Técnico
**Data:** YYYY-MM-DD | **Versão indexada:** <versão do pubspec/package.json/build.sbt>
**Stack:** <lang>/<framework> | **Owner:** <owner> | **Lifecycle:** production/beta

---

## 1. Visão Geral

<2-3 parágrafos: o que o repo faz, para quem, contexto no ecossistema>

### Stack
| Componente | Versão/Detalhe |
|-----------|----------------|
| Linguagem | |
| Framework | |
| State management | (Flutter: BLoC X.Y) |
| Comunicação | REST + GraphQL / Kafka / etc |
| Storage local | (Flutter: Hive + SharedPreferences) |
| Observabilidade | Datadog / Firebase Crashlytics / etc |

### Deploy
<como o artefato é publicado: lojas, k8s, lambda, etc>
<comandos relevantes do Makefile>

---

## 2. DFD — Fluxo de Dados

![DFD](dfd.png)

<legenda: explicar cada sistema externo, o que é trocado>

---

## 3. DER — Modelos de Dados

![DER](der.png)  ← omitir seção se não há models com campos

<ou para Flutter: descrever Hive boxes e SharedPreferences relevantes>

---

## 4. Fluxo por Operação

Para cada BLoC/área (Flutter) ou conjunto de endpoints (Kotlin/Scala):

### 4.X <NomeBLoC / Área Funcional>

![Fluxo](flow-<nome>.png)

| Evento/Endpoint | Chamadas downstream | Estado resultante |
|----------------|---------------------|-------------------|
| EventName | api.method() | StateClass |

---

## 5. Upstream

Quem aciona este repo:
- **Flutter app:** usuário via UI → eventos de tela → BLoC
- **Kotlin REST:** outros serviços, herbie-dashboard via janus, cron jobs
- **Flink:** tópicos Kafka consumidos

---

## 6. Downstream

Sistemas que este repo chama:

| Sistema | Protocolo | Endpoints/Tópicos principais |
|---------|-----------|------------------------------|
| herbie-api | REST | POST /dash/driver/assign, ... |
| janus | GraphQL | GetDriverData, UpdateDriver, ... |
| Firebase | SDK | Crashlytics, Analytics |

---

## 7. Particularidades

<pontos que não são óbvios na leitura do código>

Exemplos a cobrir:
- Deploy: ciclo de release (lojas ~1-7d, k8s rolling, lambda zip)
- Feature flags: LaunchDarkly keys usados
- Modo offline: quais operações funcionam sem rede
- Cache local: Hive boxes / SharedPrefs / cached_query_flutter
- Autenticação: fluxo de token, refresh, MFA
- Limitações conhecidas / bugs ativos do backlog

---

## Referências
- Discovery relacionado: <wtb doc search resultado>
- BUG/ticket aberto: <backlog entries>
- Repo path: <path local>
```

### 4b. Checklist de conteúdo — antes de gerar o PDF

Revisar o guide.md respondendo cada item. Se qualquer resposta for "não sei" ou "não li o código" → corrigir antes de prosseguir.

- [ ] Cada entrada em `external_apis` tem o cliente HTTP verificado no código?
- [ ] Callers de upstream têm evidência (grep ou doc) — não foram inferidos?
- [ ] Métodos de geocode, cache, serialização foram lidos no código — não assumidos?
- [ ] Nenhuma frase começa com "provavelmente", "tipicamente usa", "dado que a stack"?
- [ ] Seção "Pontos a verificar" lista todos os `[investigar]` que não foi possível confirmar?

Se não passou em todos: adicionar `[investigar]` nos campos não verificados e incluir a seção "Pontos a verificar" no final do guia.

### 5. Gerar PDF — OBRIGATÓRIO via skill `/pdf`

> **PROIBIDO** chamar `pandoc` diretamente neste passo. A skill `/pdf` é o único caminho.
> Ela executa: revisão linguística pt-BR, engine detection, flags corretos, loop de verificação visual (pdftoppm + checklist), correção de overflow e sobreposição antes de entregar.
> Chamar pandoc diretamente pula todas essas etapas — resultado: acentos faltando, tabelas estouradas, diagramas sobrepostos.

**Antes de invocar, garantir YAML front matter no topo do guide.md:**

```markdown
---
title: "<NomeRepo> — Guia Técnico"
date: "YYYY-MM-DD"
lang: pt-BR
header-includes:
  - \usepackage{float}
  - \floatplacement{figure}{H}
---
```

**Invocar a skill passando arquivo e resource-path:**

```
/pdf arquivo: /tmp/rg-<repo>/guide.md — resource-path: /tmp/rg-<repo> — output: /tmp/<repo>-guide-YYYY-MM-DD.pdf
```

**Mover para destino final após aprovação visual da skill:**

```bash
mkdir -p ~/workflow/artifacts/<repo>
mv /tmp/<repo>-guide-YYYY-MM-DD.pdf ~/workflow/artifacts/<repo>/repo-guide-YYYY-MM-DD.pdf
```

### 6. Registrar no docs.db

```bash
wtb doc add \
  --type reference \
  --title "<NomeRepo> — Guia Técnico YYYY-MM-DD" \
  --date YYYY-MM-DD \
  --repo <repo> \
  --tag "repo-guide,<lang>,<owner>" \
  --content "Guia técnico de <repo>. PDF: ~/workflow/artifacts/<repo>/repo-guide-YYYY-MM-DD.pdf. Seções: DFD, DER, fluxo por operação, upstream/downstream, particularidades."
```

---

## Regras por tipo de stack

### Flutter/Dart
- **DER:** omitir ou limitar a models `dialect=domain` com campos — não há schema SQL
- **Downstream:** agrupar external_apis por host (herbie-api REST vs janus GraphQL vs Firebase)
- **Upstream:** usuários de campo via Play Store / App Store; CI via CircleCI + make release-tags
- **Particularidades obrigatórias:** ciclo de release das lojas, Hive boxes, modo offline, LaunchDarkly

### Kotlin/Spring Boot
- **DER:** migrations Flyway (`V*.sql`) + entidades JPA (`@Entity`) — ler direto do código se models insuficientes
- **Fluxo por operação:** focar em endpoints `@RestController` + consumers `@KafkaListener`
- **Upstream:** outros serviços internos via OpenFeign/WebClient, herbie-dashboard via janus

### Scala/Flink
- **DFD:** source topics → pipeline → sink topics + sinks externos
- **DER:** omitir (stateless ou state simples — documentar Flink state class se relevante)
- **Particularidades:** Kryo state (campo novo = uninstall), consumer group, checkpointing

### Node/TypeScript
- **DER:** se usa Prisma/Sequelize/TypeORM, ler schema
- **Fluxo:** resolvers GraphQL (janus) ou routes (Express/Fastify) ou Mastra steps

---

## Output

| Artefato | Destino |
|----------|---------|
| PDF | `~/workflow/artifacts/<repo>/repo-guide-YYYY-MM-DD.pdf` |
| PNGs (diagramas) | `/tmp/rg-<repo>/` (temporários) |
| Registro docs.db | `wtb doc add --type reference` |

---

## Notas

- Sempre consultar `wtb backlog list --repo <repo>` e `wtb doc search <repo>` antes de escrever — bugs e discoveries abertos devem aparecer na seção de particularidades/referências.
- Para repos sem chart_snapshots (ex: apps móveis), omitir seção de recursos k8s.
- O guia é ponto de entrada, não substitui o código — linkar arquivos-chave quando relevante.
