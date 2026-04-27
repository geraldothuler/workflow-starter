package patterns

/*
Sistema de Patterns em Camadas para Economia de Tokens

PROBLEMA: Enviar 23KB de patterns 97 vezes = 2.2MB desperdiçado
SOLUÇÃO: 3 camadas de condensação baseadas no contexto

CAMADA 1 - CORE (~300 bytes)
  - Apenas anti-patterns críticos
  - Usado em: Critérios de aceite, validações

CAMADA 2 - ESSENTIALS (~2KB)
  - Anti-patterns + resumo de cada pattern
  - Usado em: Épicos, Histórias

CAMADA 3 - FULL (~23KB)
  - Conteúdo completo (lido de arquivos embarcados)
  - Usado em: Extração inicial, Deep Dives específicos
*/

import (
	"embed"
	"os"
	"path/filepath"
	"sync"
)

//go:embed data/*.md
var patternFiles embed.FS

// Layer representa camada de patterns
type Layer int

const (
	Core       Layer = iota // ~300 bytes - Anti-patterns apenas
	Essentials              // ~2KB - Anti-patterns + resumos
	Full                    // ~23KB - Conteúdo completo
)

// PatternLayer extrai patterns em diferentes níveis
type PatternLayer struct {
	customPatterns map[string]string
	mu             sync.RWMutex
}

// loadEmbedded carrega um arquivo embarcado, retornando string vazia se falhar
func loadEmbedded(filename string) string {
	data, err := patternFiles.ReadFile("data/" + filename)
	if err != nil {
		return ""
	}
	return string(data)
}

// GetGoldenPathsCore retorna apenas decisões críticas
func (pl *PatternLayer) GetGoldenPathsCore() string {
	if content := loadEmbedded("golden-paths-core.md"); content != "" {
		return content
	}
	return fallbackGoldenPathsCore
}

// GetTeamPatternsCore retorna apenas anti-patterns
func (pl *PatternLayer) GetTeamPatternsCore() string {
	if content := loadEmbedded("team-patterns-core.md"); content != "" {
		return content
	}
	return fallbackTeamPatternsCore
}

// GetGoldenPathsEssentials retorna resumo executivo
func (pl *PatternLayer) GetGoldenPathsEssentials() string {
	if content := loadEmbedded("golden-paths-essentials.md"); content != "" {
		return content
	}
	return fallbackGoldenPathsEssentials
}

// GetTeamPatternsEssentials retorna resumo do time
func (pl *PatternLayer) GetTeamPatternsEssentials() string {
	if content := loadEmbedded("team-patterns-essentials.md"); content != "" {
		return content
	}
	return fallbackTeamPatternsEssentials
}

// GetGoldenPathsFull retorna documentação completa de golden paths
func (pl *PatternLayer) GetGoldenPathsFull() string {
	return loadEmbedded("golden-paths-full.md")
}

// GetTeamPatternsFull retorna documentação completa de team patterns
func (pl *PatternLayer) GetTeamPatternsFull() string {
	return loadEmbedded("team-patterns-full.md")
}

// GetCombined combina GP + TP em uma camada
func (pl *PatternLayer) GetCombined(layer Layer) string {
	var result string

	switch layer {
	case Core:
		result = pl.GetGoldenPathsCore() + "\n" + pl.GetTeamPatternsCore()
	case Essentials:
		result = pl.GetGoldenPathsEssentials() + "\n" + pl.GetTeamPatternsEssentials()
	case Full:
		result = pl.GetGoldenPathsFull() + "\n" + pl.GetTeamPatternsFull()
	}

	// Append custom patterns if any
	pl.mu.RLock()
	if pl.customPatterns != nil {
		for _, content := range pl.customPatterns {
			result += "\n" + content
		}
	}
	pl.mu.RUnlock()

	return result
}

// LoadCustomPatterns carrega patterns customizados de um diretório
func (pl *PatternLayer) LoadCustomPatterns(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	pl.mu.Lock()
	defer pl.mu.Unlock()

	if pl.customPatterns == nil {
		pl.customPatterns = make(map[string]string)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".md" && ext != ".txt" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		pl.customPatterns[entry.Name()] = string(data)
	}
	return nil
}

// GetRecommendedLayer retorna camada recomendada por fase
func (pl *PatternLayer) GetRecommendedLayer(phase string) Layer {
	switch phase {
	case "extraction":
		return Essentials
	case "epics":
		return Essentials
	case "stories":
		return Essentials
	case "criteria":
		return Core
	case "deep-dive-tech":
		return Full
	case "deep-dive-story":
		return Essentials
	default:
		return Core
	}
}

// GetEstimatedTokens estima tokens por camada
func (pl *PatternLayer) GetEstimatedTokens(layer Layer) int {
	switch layer {
	case Core:
		return 100
	case Essentials:
		return 700
	case Full:
		return 7000
	}
	return 0
}

// FormatWithHeader adiciona header explicativo
func (pl *PatternLayer) FormatWithHeader(patterns string, layer Layer) string {
	var header string

	switch layer {
	case Core:
		header = `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
⚠️  PADRÕES OBRIGATÓRIOS DA EMPRESA
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

IMPORTANTE: Decisões abaixo são MANDATÓRIAS. Violar = código rejeitado.
`
	case Essentials:
		header = `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🎯 GOLDEN PATHS + 👥 TEAM PATTERNS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Padrões validados e decisões arquiteturais do time.
SEMPRE seguir estes padrões ao gerar código/épicos/histórias.
`
	case Full:
		header = `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📚 DOCUMENTAÇÃO COMPLETA DE PATTERNS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
`
	}

	return header + patterns
}

// --- Fallback hardcoded content (used if embedded files fail) ---

var fallbackGoldenPathsCore = `
🎯 GOLDEN PATHS - DECISÕES CRÍTICAS:

Observabilidade:
  ✅ DataDog (obrigatório)
  ❌ Prometheus, Grafana, ELK, Jaeger

Streaming:
  ✅ Kafka + Flink
  ❌ Kafka Streams, Spark Streaming

Dados:
  ✅ ScyllaDB (write-heavy), PostgreSQL (analytics)
  ❌ MongoDB, DynamoDB, Cassandra
`

var fallbackTeamPatternsCore = `
👥 TEAM PATTERNS - ANTI-PATTERNS:

Linguagem:
  ✅ Kotlin (obrigatório)
  ❌ Java (muito verboso)

Build & Quality:
  ✅ Gradle + Ktlint
  ❌ Maven

CI/CD:
  ✅ CircleCI
  ❌ GitHub Actions, Jenkins

Schema:
  ✅ Git submodule
  ❌ Avro Schema Registry

Infra:
  ✅ mTLS direto
  ❌ Service Mesh (Istio, Linkerd)
`

var fallbackGoldenPathsEssentials = `
🎯 GOLDEN PATHS (Resumo):

GP-001: Event Sourcing com Kafka
GP-002: Stream Processing com Flink
GP-003: Camada de Dados Híbrida
GP-004: Cache com Redis
GP-005: Notificações Assíncronas
GP-006: Observabilidade ÚNICA com DataDog
GP-007: Deploy Blue-Green
GP-008: Segurança mTLS + JWT
`

var fallbackTeamPatternsEssentials = `
👥 TEAM PATTERNS (Resumo):

TP-001: Backend em Kotlin + Spring Boot
TP-002: Data Processing
TP-003: Databases
TP-004: Build & Quality
TP-005: CI/CD
TP-006: Schema Management
TP-007: Observabilidade
TP-008: Anti-Patterns Infra
`
