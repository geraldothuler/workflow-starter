package extractor

import (
	"fmt"
	"strings"
)

// PromptBuilder constrói prompts otimizados para extração
type PromptBuilder struct {
	transcript   string
	contextInfo  string
	useExamples  bool
	useChainOfThought bool
}

// NewPromptBuilder cria novo builder
func NewPromptBuilder(transcript string, contextInfo string) *PromptBuilder {
	return &PromptBuilder{
		transcript:        transcript,
		contextInfo:       contextInfo,
		useExamples:       true,
		useChainOfThought: true,
	}
}

// Build constrói o prompt completo
func (pb *PromptBuilder) Build() string {
	var prompt strings.Builder

	// System role
	prompt.WriteString(pb.systemRole())
	prompt.WriteString("\n\n")

	// Transcript
	prompt.WriteString("TRANSCRIÇÃO DA REUNIÃO:\n")
	prompt.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	prompt.WriteString(pb.transcript)
	prompt.WriteString("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Context info
	if pb.contextInfo != "" {
		prompt.WriteString("CONTEXTO ADICIONAL (Golden Paths & Team Patterns):\n")
		prompt.WriteString(pb.contextInfo)
		prompt.WriteString("\n\n")
	}

	// Task description
	prompt.WriteString(pb.taskDescription())
	prompt.WriteString("\n\n")

	// Extraction rules
	prompt.WriteString(pb.extractionRules())
	prompt.WriteString("\n\n")

	// Few-shot examples
	if pb.useExamples {
		prompt.WriteString(pb.fewShotExamples())
		prompt.WriteString("\n\n")
	}

	// Chain of thought instructions
	if pb.useChainOfThought {
		prompt.WriteString(pb.chainOfThoughtInstructions())
		prompt.WriteString("\n\n")
	}

	// Output format
	prompt.WriteString(pb.outputFormat())

	return prompt.String()
}

// systemRole define o papel do LLM
func (pb *PromptBuilder) systemRole() string {
	return `Você é um Product Manager experiente com 10+ anos de experiência analisando requisitos de projetos de software.

Suas especialidades:
- Extrair requisitos de conversas informais
- Identificar tecnologias mencionadas ou implícitas
- Inferir volumetria e NFRs de descrições vagas
- Normalizar terminologia técnica
- Detectar inconsistências e gaps

Você está analisando uma transcrição de reunião de kickoff de projeto.`
}

// taskDescription descreve a tarefa
func (pb *PromptBuilder) taskDescription() string {
	return `TAREFA:
Extraia e estruture as seguintes informações da reunião:

1. **CONTEXTO** (2-3 parágrafos)
   - O que é o projeto
   - Qual problema resolve
   - Por que é necessário

2. **PROBLEMA** (1-2 parágrafos)
   - Situação atual problemática
   - Dor específica dos usuários/clientes
   - Impacto no negócio

3. **OBJETIVOS** (lista)
   - Metas mensuráveis
   - KPIs esperados
   - Resultados de negócio

4. **VOLUMETRIA**
   - Usuários, dispositivos, transações/segundo
   - Volumes atuais e projetados
   - Picos esperados

5. **STACK TÉCNICO**
   - Tecnologias mencionadas explicitamente
   - Tecnologias sugeridas/inferidas (com justificativa)
   - Separar: confirmadas vs sugeridas

6. **REQUISITOS NÃO-FUNCIONAIS (NFRs)**
   - Latência, throughput
   - Uptime, disponibilidade
   - Segurança, compliance
   - Escalabilidade`
}

// extractionRules define regras de extração
func (pb *PromptBuilder) extractionRules() string {
	return `REGRAS DE EXTRAÇÃO CRÍTICAS:

🎯 **REGRA MÁXIMA PRIORIDADE - GOLDEN PATHS**
- Golden Paths são padrões OBRIGATÓRIOS e VALIDADOS pela empresa
- SEMPRE prefira tecnologias dos Golden Paths sobre outras
- Se Golden Path específico existe, siga-o RIGOROSAMENTE
- Exemplo: Se GP diz "DataDog para observabilidade", NÃO sugira Prometheus/Grafana
- Golden Paths têm autoridade MÁXIMA sobre inferências

📋 **Processamento de Linguagem Natural**
- Ignore vícios: "né", "tipo", "sei lá", "uhm", "ah", "então"
- Ignore hesitações: "...como é o nome?", "qual era mesmo?"
- Normalize gírias: "pra caramba" = "muito", "de boa" = "fácil"

🔢 **Volumetria e Números**
- "meio milhão" → 500.000
- "uns 100 mil" → 100.000 (aproximado)
- "muitos" → inferir baseado em contexto (ex: B2B = 1K-10K, B2C = 100K+)
- Se não mencionado, INFIRA ordem de grandeza e marque como inferido

💻 **Tecnologias**
- Normalize nomes: "aquele banco NoSQL" → identificar qual (ScyllaDB, MongoDB, etc.)
- PRIORIDADE 1: Use tecnologias dos Golden Paths
- PRIORIDADE 2: Use tecnologias mencionadas explicitamente  
- PRIORIDADE 3: Infira baseado em contexto
- Se mencionam apenas categoria, sugira baseado em contexto
- Exemplo: "precisa de cache" + "baixa latência" → Redis
- Use Golden Paths para sugestões quando relevante

⚡ **NFRs de Frases Vagas**
- "tem que ser rápido" → Latência P95 < 500ms (inferir valor razoável)
- "não pode cair" → Uptime 99.9%
- "escala bem" → Escalabilidade horizontal
- "seguro" → Autenticação + autorização + criptografia

🎯 **Confiança (Confidence Score)**
- 1.0 = Mencionado explicitamente com número
- 0.9 = Mencionado explicitamente sem número exato
- 0.7-0.8 = Inferido com contexto forte
- 0.5-0.6 = Inferido com contexto fraco
- < 0.5 = Especulação (não incluir)

🔗 **Golden Paths**
- Se tecnologia mencionada aparece em GP, referenciar o pattern
- Exemplo: "Kafka" mencionado + GP-001 existe → citar GP-001

⚠️  **Avisos (Warnings)**
- Sempre adicione warning para itens inferidos
- Marque contradições se detectar
- Sinalize gaps críticos (ex: sem NFRs mencionados)`
}

// fewShotExamples fornece exemplos
func (pb *PromptBuilder) fewShotExamples() string {
	return `EXEMPLOS DE EXTRAÇÃO:

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
EXEMPLO 1: Boa Extração

INPUT:
"A gente tem uns 500 dispositivos. Precisa processar rápido, tipo menos de 1 segundo."

OUTPUT:
{
  "volumetry": {
    "devices": "500",
    "devices_confidence": 0.9
  },
  "nfrs": ["Latência P95 < 1s"],
  "explicit_mentions": [
    {"type": "volumetry", "value": "500 dispositivos", "confidence": 0.9}
  ]
}

REASONING: "uns 500" = ~500 (alta confiança), "rápido" + "menos de 1 segundo" = requisito de latência específico

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
EXEMPLO 2: Inferência Correta

INPUT:
"É um app B2C né. Vai ter bastante gente usando."

OUTPUT:
{
  "volumetry": {
    "users": "100000 (inferido)",
    "users_confidence": 0.6
  },
  "warnings": ["Volume de usuários inferido como ~100K baseado em contexto B2C - validar com time"],
  "inferred_items": [
    {"type": "volumetry", "value": "100K usuários", "confidence": 0.6, "rationale": "B2C típico = 100K+ usuários"}
  ]
}

REASONING: B2C implica volume alto, "bastante gente" confirma, inferência razoável é 100K+ com confiança média

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
EXEMPLO 3: Normalização de Tecnologia

INPUT:
"Precisa daquele banco NoSQL que a gente usa, sabe? O rápido."

Com contexto: [Team Patterns menciona ScyllaDB]

OUTPUT:
{
  "stack": [
    {
      "name": "ScyllaDB",
      "confidence": 0.8,
      "source": "inferred",
      "rationale": "Menção a 'banco NoSQL rápido que a gente usa' + Team Patterns indica ScyllaDB em uso"
    }
  ]
}

REASONING: "banco NoSQL" + "a gente usa" + "rápido" + contexto de Team Patterns → ScyllaDB com boa confiança`
}

// chainOfThoughtInstructions instrui chain-of-thought
func (pb *PromptBuilder) chainOfThoughtInstructions() string {
	return `RACIOCÍNIO (Chain-of-Thought):

Antes de gerar o JSON final, PENSE:

1. **O que foi dito EXPLICITAMENTE?**
   Liste todos os fatos mencionados diretamente

2. **O que pode ser INFERIDO com ALTA confiança?**
   - Quais pistas existem?
   - O contexto (GP/TP) ajuda?
   - A inferência é razoável?

3. **O que NÃO deve ser inferido?**
   - Informações sem base suficiente
   - Especulações sem contexto

4. **Quais WARNINGS são necessários?**
   - Itens inferidos precisam validação?
   - Há contradições?
   - Falta informação crítica?

5. **VALIDAÇÃO final:**
   - Os números fazem sentido?
   - As tecnologias são compatíveis?
   - Os NFRs são realistas?

Não inclua este raciocínio no JSON final, mas use-o para gerar output de alta qualidade.`
}

// outputFormat especifica formato de saída
func (pb *PromptBuilder) outputFormat() string {
	return `FORMATO DE SAÍDA:

Retorne UM ÚNICO JSON válido (sem markdown, sem explicações):

{
  "context": "string (2-3 parágrafos)",
  "problem": "string (1-2 parágrafos)",
  "objectives": ["string", ...],
  "volumetry": {
    "metric_name": "value",
    "metric_name_confidence": 0.0-1.0,
    ...
  },
  "stack": [
    {
      "name": "TechName",
      "confidence": 0.0-1.0,
      "source": "explicit|inferred",
      "rationale": "string (opcional, para inferidos)"
    }
  ],
  "nfrs": ["string", ...],
  "overall_confidence": 0.0-1.0,
  "section_confidence": {
    "context": 0.0-1.0,
    "problem": 0.0-1.0,
    "volumetry": 0.0-1.0,
    "stack": 0.0-1.0,
    "nfrs": 0.0-1.0
  },
  "speakers": ["Nome", ...],
  "warnings": ["string", ...],
  "explicit_mentions": [
    {"type": "tech|nfr|volumetry", "value": "string", "speaker": "Nome", "timestamp": "HH:MM"}
  ],
  "inferred_items": [
    {"type": "tech|nfr|volumetry", "value": "string", "confidence": 0.0-1.0, "rationale": "string"}
  ]
}

CRÍTICO:
- Retorne APENAS o JSON
- Sem markdown code fences
- Sem texto antes ou depois
- JSON válido e parseable`
}

// BuildSimplified constrói prompt simplificado (fallback)
func (pb *PromptBuilder) BuildSimplified() string {
	return fmt.Sprintf(`Analise esta transcrição e extraia:
- Contexto do projeto
- Problema sendo resolvido
- Tecnologias mencionadas
- Requisitos (volumetria, NFRs)

TRANSCRIÇÃO:
%s

Retorne JSON estruturado.`, pb.transcript)
}
