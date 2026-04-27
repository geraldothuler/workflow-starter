---
name: grill-me — obrigatório antes de qualquer plano
description: O skill grill-me deve ser usado antes de qualquer plano de implementação não trivial. Geraldo confirmou que é fantástico e quer adotar para todo plano.
type: feedback
---

Antes de iniciar qualquer plano de implementação não trivial, usar o skill `grill-me` para validar decisões de design.

**Why:** Geraldo avaliou o skill como "fantástico" e quer adotar para todo plano. Resolve ambiguidades antes de escrever código, evita retrabalho por decisões de design não validadas.

**How to apply:** Quando o usuário apresentar um plano ou pedir para implementar algo com múltiplas decisões de design em aberto — invocar `/grill-me` antes de escrever qualquer código. Não precisa esperar o usuário pedir explicitamente.

**Formato obrigatório das perguntas:** Uma pergunta por vez (1-a-1), socraticamente. Cada pergunta deve incluir:
1. A sugestão recomendada com justificativa
2. As alternativas com prós/contras

Não enviar múltiplas perguntas na mesma mensagem. Aguardar resposta antes da próxima.
