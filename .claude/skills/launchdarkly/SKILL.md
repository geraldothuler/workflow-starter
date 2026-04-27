---
name: launchdarkly
description: >
  Consultar e gerenciar feature flags no LaunchDarkly da Cobli.
  Suporta: listar estado de uma flag por ambiente, ver targets/rules,
  adicionar ou remover emails/fleet_ids de uma flag, e ligar/desligar flag.
  Ativar quando o usuário pedir "feature flag", "launch darkly", "adiciona na flag",
  "quem tem acesso à flag", "liga/desliga flag".
user-invocable: true
---

# LaunchDarkly — Consulta e Gerenciamento de Feature Flags

**API:** `https://app.launchdarkly.com/api/v2/`
**Token:** SSM `/cobli/launchdarkly/api-token` (region `us-east-1`)
**Projeto:** `default`
**Ambientes principais:** `production`, `test`, `dev`

---

## Setup

```bash
API_TOKEN=$(aws ssm get-parameter --name /cobli/launchdarkly/api-token \
  --with-decryption --query Parameter.Value --output text --region us-east-1)
```

---

## Fluxo 1 — Consultar flag

```bash
FLAG="nome-da-flag"  # ex: import-identifier-ai, safety-ranking

curl -sf "https://app.launchdarkly.com/api/v2/flags/default/$FLAG" \
  -H "Authorization: $API_TOKEN" | python3 -c "
import sys, json
d = json.load(sys.stdin)
print('name:', d.get('name'))
print('key:', d.get('key'))
for env_name, env_data in d.get('environments', {}).items():
    on = env_data.get('on')
    rules = env_data.get('rules', [])
    targets = env_data.get('targets', [])
    print(f'  [{env_name}] on={on} rules={len(rules)} targets={len(targets)}')
    for r in rules:
        for c in r.get('clauses', []):
            vals = c.get('values', [])
            print(f'    clause attr={c.get(\"attribute\")} op={c.get(\"op\")} count={len(vals)} sample={vals[:5]}')
    for t in targets:
        print(f'    target variation={t.get(\"variation\")} values={t.get(\"values\", [])}')
"
```

**Interpretação de variation:**
- `variation=0` = True (habilitado)
- `variation=1` = False (desabilitado / controle)

---

## Fluxo 2 — Adicionar emails a um target

Flags baseadas em **usuário** (email) usam `targets` com `variation=0` (True).

```bash
FLAG="nome-da-flag"
ENV="production"   # production | test | dev
EMAILS='["novo@cobli.co"]'   # array JSON

# 1. Buscar targets atuais
CURRENT=$(curl -sf "https://app.launchdarkly.com/api/v2/flags/default/$FLAG" \
  -H "Authorization: $API_TOKEN" | python3 -c "
import sys, json
d = json.load(sys.stdin)
targets = d['environments']['$ENV'].get('targets', [])
true_target = next((t for t in targets if t['variation'] == 0), None)
print(json.dumps(true_target.get('values', []) if true_target else []))
")

# 2. Merge e patch
python3 - << PYEOF
import json, subprocess, urllib.request

api_token = subprocess.check_output(
    ['aws','ssm','get-parameter','--name','/cobli/launchdarkly/api-token',
     '--with-decryption','--query','Parameter.Value','--output','text',
     '--region','us-east-1'], text=True).strip()

current = json.loads('$CURRENT')
new_emails = $EMAILS
merged = sorted(set(current + new_emails))

body = json.dumps({
    'comment': 'Add users via workflow-toolbox',
    'environmentKey': '$ENV',
    'instructions': [{
        'kind': 'replaceUserTargets',
        'values': merged,
        'variationId': None,  # será preenchido pelo passo abaixo
    }]
})

# Abordagem mais simples: usar updateTargets
body = json.dumps({
    'comment': 'Add users via workflow-toolbox',
    'environmentKey': '$ENV',
    'instructions': [{
        'kind': 'addUserTargets',
        'values': new_emails,
        'variationId': '0',  # index da variation True
    }]
})

req = urllib.request.Request(
    f'https://app.launchdarkly.com/api/v2/flags/default/$FLAG',
    data=body.encode(),
    headers={
        'Authorization': api_token,
        'Content-Type': 'application/json; domain-model=launchdarkly.semanticpatch',
    },
    method='PATCH'
)
with urllib.request.urlopen(req, timeout=15) as resp:
    result = json.loads(resp.read())
    targets = result['environments']['$ENV'].get('targets', [])
    true_target = next((t for t in targets if t['variation'] == 0), {})
    print(f'OK — {len(true_target.get(\"values\",[]))} usuários com acesso')
PYEOF
```

**Nota:** `addUserTargets` requer o `variationId` como UUID, não index.
Usar `replaceTargets` com a lista completa é mais seguro:

```python
# Buscar variationId da variation=0 (True)
FLAG_DATA = requests.get(f'{BASE_URL}flags/default/{FLAG}', headers=headers).json()
variation_id = FLAG_DATA['variations'][0]['_id']  # index 0 = True

body = {
    'comment': 'Add users via workflow-toolbox',
    'environmentKey': ENV,
    'instructions': [{
        'kind': 'addUserTargets',
        'values': new_emails,        # lista de emails a adicionar
        'variationId': variation_id, # UUID da variation True
    }]
}
semantic_headers = {
    'Authorization': api_token,
    'Content-Type': 'application/json; domain-model=launchdarkly.semanticpatch',
}
r = requests.patch(f'{BASE_URL}flags/default/{FLAG}', json=body, headers=semantic_headers)
```

---

## Fluxo 3 — Adicionar fleet_ids a uma rule

Para flags com rule baseada em `fleet_id` (ex: `safety-ranking`):

```python
import requests

BASE_URL = 'https://app.launchdarkly.com/api/v2/'

def get_rule_data(flag, env='production'):
    r = requests.get(f'{BASE_URL}flags/default/{flag}', headers=headers)
    env_data = r.json()['environments'][env]
    rule = env_data['rules'][0]
    clause = rule['clauses'][0]
    return rule['_id'], clause['_id'], clause['values']

def add_fleets_to_flag(flag, fleet_ids_to_add, env='production'):
    rule_id, clause_id, current_values = get_rule_data(flag, env)
    merged = sorted(set(current_values + fleet_ids_to_add))

    body = {
        'comment': 'Add fleets via workflow-toolbox',
        'environmentKey': env,
        'instructions': [{
            'kind': 'updateClause',
            'ruleId': rule_id,
            'clauseId': clause_id,
            'clause': {
                'attribute': 'fleet_id',
                'op': 'in',
                'values': merged,
            }
        }]
    }
    semantic_headers = {
        'Authorization': api_token,
        'Content-Type': 'application/json; domain-model=launchdarkly.semanticpatch',
    }
    r = requests.patch(f'{BASE_URL}flags/default/{flag}', json=body, headers=semantic_headers)
    print(f'Status: {r.status_code}')

    # Validação
    _, _, new_values = get_rule_data(flag, env)
    missing = [f for f in fleet_ids_to_add if f not in new_values]
    if missing:
        print(f'AVISO: não encontrados após patch: {missing}')
    else:
        print(f'OK — {len(new_values)} fleet_ids na flag')
```

---

## Fluxo 4 — Listar todas as flags

```bash
curl -sf "https://app.launchdarkly.com/api/v2/flags/default?limit=50" \
  -H "Authorization: $API_TOKEN" | python3 -c "
import sys, json
d = json.load(sys.stdin)
flags = d.get('items', [])
print(f'{len(flags)} flags:')
for f in sorted(flags, key=lambda x: x['key']):
    print(f'  {f[\"key\"]:50s} {f[\"name\"]}')
"
```

---

## Flags conhecidas

| Key | Nome | Tipo de segmentação | Obs |
|-----|------|-------------------|-----|
| `allow-list`          | Allow List       | fleet_id (rule clause) | 42 frotas em prod; 2ª rule = segmentMatch `test-segment` |
| `vehicle-immobilizer` | Vehicle Immobilizer | fleet_id (rule clause) | 2 rules: 3 frotas (rule 1) + 40 frotas (rule 2) |
| `import-identifier-ai` | Import RFID AI | email (user targets) | — |
| `safety-ranking` | Safety Ranking | fleet_id (rule clause) | — |
| `crm-feature-flag` | CRM | — | — |

---

## Gotchas

- **Semantic patch** obrigatório: `Content-Type: application/json; domain-model=launchdarkly.semanticpatch`
- **`variation=0` = True** para flags booleanas criadas com default off
- **`addUserTargets`** requer UUID da variation, não índice — buscar `FLAG_DATA['variations'][0]['_id']`
- **`updateClause`** é para regras por atributo (fleet_id, etc.) — `addUserTargets` é para targets diretos
- Flag `on=False` no ambiente = flag desligada globalmente, independente de targets/rules
- Testar sempre em `test` antes de `production`
