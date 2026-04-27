---
name: PDF skill — invocar Skill tool, nunca pandoc direto
description: Ao gerar qualquer PDF (repo-guide, onboarding, doc), SEMPRE invocar Skill /pdf. Nunca chamar pandoc diretamente.
type: feedback
---

Invocar SEMPRE a Skill `/pdf` ao gerar PDFs de guias técnicos. Nunca chamar `pandoc` diretamente no Bash.

**Why:** Chamar pandoc direto pula a revisão linguística pt-BR obrigatória (checklist BLOQUEANTE no pdf/SKILL.md). Resultado documentado: todos os acentos ausentes nos guias gerados — "Visao Geral", "servico", "operacoes", etc. O fato de a regra estar escrita no SKILL.md não garante execução; precisa ser um habit check antes de qualquer pandoc call.

**How to apply:** Antes de qualquer geração de PDF — verificar: "Estou prestes a chamar pandoc no Bash?" → parar → usar `Skill tool` com `skill: "pdf"` e `args: "arquivo: ... — resource-path: ... — output: ..."` em vez disso. Isso garante: revisão linguística, flags corretos, loop visual (pdftoppm + checklist), correção de overflow antes de entregar. Única exceção aceita: regeneração após correção já dentro do fluxo do Skill /pdf.
