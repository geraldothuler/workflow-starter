package techref

import (
	"strings"
)

// ContextInfo contém informações sobre o contexto ao redor de uma palavra
type ContextInfo struct {
	HasVerbBefore     bool
	HasVerbAfter      bool
	HasTechBefore     bool
	HasTechAfter      bool
	IsStartOfSentence bool
	WordBefore        string
	WordAfter         string
}

// analyzeContext analisa o contexto ao redor de uma palavra (backward compat)
func analyzeContext(text string, word string, position int) ContextInfo {
	return analyzeContextWithRegistry(DefaultRegistry(), text, word, position)
}

// analyzeContextWithRegistry analisa contexto usando registry configurável
func analyzeContextWithRegistry(reg *TechRegistry, text string, word string, position int) ContextInfo {
	if reg == nil {
		reg = DefaultRegistry()
	}

	info := ContextInfo{}

	// Separar em palavras
	words := strings.Fields(text)

	// Encontrar índice da palavra
	wordIndex := -1
	for i, w := range words {
		if strings.Contains(w, word) {
			wordIndex = i
			break
		}
	}

	if wordIndex == -1 {
		return info
	}

	// Verificar se é início de sentença
	info.IsStartOfSentence = wordIndex == 0

	// Pegar palavra anterior
	if wordIndex > 0 {
		info.WordBefore = cleanWord(words[wordIndex-1])
		info.HasVerbBefore = reg.IsVerb(info.WordBefore)
		info.HasTechBefore = reg.IsKnownTech(info.WordBefore)
	}

	// Pegar palavra posterior
	if wordIndex < len(words)-1 {
		info.WordAfter = cleanWord(words[wordIndex+1])
		info.HasVerbAfter = reg.IsVerb(info.WordAfter)
		info.HasTechAfter = reg.IsKnownTech(info.WordAfter)
	}

	return info
}

// isValidIsolated decide se palavra isolada é válida baseado em contexto
func isValidIsolated(word string, context ContextInfo) bool {
	return isValidIsolatedWithRegistry(DefaultRegistry(), word, context)
}

// isValidIsolatedWithRegistry decide se palavra é válida usando registry
func isValidIsolatedWithRegistry(reg *TechRegistry, word string, context ContextInfo) bool {
	if reg == nil {
		reg = DefaultRegistry()
	}

	// Regra 1: Se tem verbo antes, provavelmente não é tech
	if context.HasVerbBefore {
		if !context.IsStartOfSentence {
			return false
		}
	}

	// Regra 2: Se tem verbo depois, provavelmente não é tech
	if context.HasVerbAfter {
		return false
	}

	// Regra 3: Se está entre techs conhecidas, pode ser tech também
	if context.HasTechBefore && context.HasTechAfter {
		return true
	}

	// Regra 4: Se a palavra em si é conhecida, aceitar
	if reg.IsKnownTech(word) {
		return true
	}

	// Regra 5: Sem evidência positiva → rejeitar (conservativo)
	return false
}

// cleanWord remove pontuação de palavra
func cleanWord(word string) string {
	word = strings.Trim(word, ".,;:!?()[]{}\"'")
	return word
}

// isVerb verifica se palavra é verbo (backward compat)
func isVerb(word string) bool {
	return DefaultRegistry().IsVerb(word)
}

// isKnownTech verifica se é tecnologia conhecida (backward compat)
func isKnownTech(word string) bool {
	return DefaultRegistry().IsKnownTech(word)
}
