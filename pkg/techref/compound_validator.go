package techref

import (
	"strings"
)

// isValidCompound valida se duas palavras formam tecnologia válida (backward compat)
func isValidCompound(word1, word2 string) bool {
	return isValidCompoundWithRegistry(DefaultRegistry(), word1, word2)
}

// isValidCompoundWithRegistry valida usando registry configurável
func isValidCompoundWithRegistry(reg *TechRegistry, word1, word2 string) bool {
	// Padrão 1: Tech + Tech (ex: "Spring Boot", "React Native")
	if isTechTechWithRegistry(reg, word1, word2) {
		return true
	}

	// Padrão 2: Tech + Versão (ex: "Java 11", "Python 3")
	if isTechVersionWithRegistry(reg, word1, word2) {
		return true
	}

	// Padrão 3: Tech + Modifier (ex: "React Native", "Spring Cloud")
	if isTechModifierWithRegistry(reg, word1, word2) {
		return true
	}

	// Padrão inválido: contém verbo
	if hasVerbWithRegistry(reg, word1, word2) {
		return false
	}

	// Padrão inválido: termo de negócio
	if isBusinessTermWithRegistry(reg, word1, word2) {
		return false
	}

	// Se passou por tudo, rejeitar (conservador)
	return false
}

// isTechTechWithRegistry verifica se ambas são tech words conhecidas
func isTechTechWithRegistry(reg *TechRegistry, w1, w2 string) bool {
	knownWords := reg.CompoundKnownWords()
	return knownWords[w1] && knownWords[w2]
}

// isTechVersionWithRegistry verifica se é tech + número
func isTechVersionWithRegistry(reg *TechRegistry, w1, w2 string) bool {
	if len(w2) > 0 && w2[0] >= '0' && w2[0] <= '9' {
		techs := reg.CompoundKnownTechs()
		return techs[w1]
	}
	return false
}

// isTechModifierWithRegistry verifica se é tech + modificador válido
func isTechModifierWithRegistry(reg *TechRegistry, w1, w2 string) bool {
	modifiers := reg.CompoundTechModifiers()
	return modifiers[w2]
}

// hasVerbWithRegistry verifica se contém verbo
func hasVerbWithRegistry(reg *TechRegistry, w1, w2 string) bool {
	verbs := reg.CompoundVerbs()
	return verbs[w1] || verbs[w2]
}

// isBusinessTermWithRegistry verifica se é termo de negócio
func isBusinessTermWithRegistry(reg *TechRegistry, w1, w2 string) bool {
	terms := reg.CompoundBusinessTerms()
	return terms[w1] || terms[w2]
}

// validateCompoundStrict valida composto com regras estritas (backward compat)
func validateCompoundStrict(compound string) bool {
	return validateCompoundStrictWithRegistry(DefaultRegistry(), compound)
}

// validateCompoundStrictWithRegistry valida composto usando registry
func validateCompoundStrictWithRegistry(reg *TechRegistry, compound string) bool {
	parts := strings.Split(compound, " ")
	if len(parts) != 2 {
		return false
	}
	return isValidCompoundWithRegistry(reg, parts[0], parts[1])
}

// Backward compatibility wrappers (unexported, used internally)
func isTechTech(w1, w2 string) bool {
	return isTechTechWithRegistry(DefaultRegistry(), w1, w2)
}

func isTechVersion(w1, w2 string) bool {
	return isTechVersionWithRegistry(DefaultRegistry(), w1, w2)
}

func isTechModifier(w1, w2 string) bool {
	return isTechModifierWithRegistry(DefaultRegistry(), w1, w2)
}

func hasVerb(w1, w2 string) bool {
	return hasVerbWithRegistry(DefaultRegistry(), w1, w2)
}

func isBusinessTerm(w1, w2 string) bool {
	return isBusinessTermWithRegistry(DefaultRegistry(), w1, w2)
}
