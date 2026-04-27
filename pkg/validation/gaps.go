// pkg/validation/gaps.go
package validation

import (
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// GapType represents the type of gap found
type GapType string

const (
	GapPatternNotAddressed GapType = "pattern_not_addressed"
	GapNFRMissing          GapType = "nfr_missing"
	GapStackAmbiguous      GapType = "stack_ambiguous"
	GapUserContextMissing  GapType = "user_context_missing"
	GapDependenciesMissing GapType = "dependencies_missing"
	GapEventsMissing       GapType = "events_missing"
	GapScaleMissing        GapType = "scale_missing"
	GapVolumetryVague      GapType = "volumetry_vague"
	GapNFRNoNumbers        GapType = "nfr_no_numbers"
	GapEdgeCasesMissing    GapType = "edge_cases_missing"
	GapSecurityMissing     GapType = "security_missing"
	GapObservabilityMissing GapType = "observability_missing"
	GapDeployMissing       GapType = "deploy_missing"
)

// Gap represents a detected gap in the narrative
type Gap struct {
	Type        GapType
	Title       string
	Context     string
	Question    string
	Examples    []string
	CobliExample string
	Severity    string // "high", "medium", "low"
}

// GapDetector detects gaps in project narrative
type GapDetector struct {
	goldenPath *types.GoldenPath
	narrative  string
}

// NewGapDetector creates a new gap detector
func NewGapDetector(goldenPath *types.GoldenPath, narrative string) *GapDetector {
	return &GapDetector{
		goldenPath: goldenPath,
		narrative:  strings.ToLower(narrative),
	}
}

// DetectAll detects all gaps in the narrative
func (gd *GapDetector) DetectAll() []Gap {
	var gaps []Gap
	
	// Check for Event Sourcing if it's in golden path
	if gd.hasPattern("event sourcing") && !gd.mentionsEvents() {
		gaps = append(gaps, Gap{
			Type:     GapEventsMissing,
			Title:    "Event Sourcing não mencionado",
			Context:  "Golden path define Event Sourcing como obrigatório",
			Question: "Quais eventos de domínio você precisa rastrear?",
			Examples: []string{
				"VehiclePositionUpdated",
				"RouteOptimized",
				"FuelConsumed",
				"DeliveryCompleted",
			},
			CobliExample: `Domínio: Telemetria
VehiclePositionUpdated
  • vehicle_id, lat, lng, timestamp
  • Volume: 500K eventos/segundo
  • Rastreabilidade completa`,
			Severity: "high",
		})
	}
	
	// Check for performance requirements if scale is mentioned
	if gd.mentionsScale() && !gd.mentionsPerformance() {
		gaps = append(gaps, Gap{
			Type:     GapNFRMissing,
			Title:    "Requisitos de Performance não definidos",
			Context:  "Você mencionou volume/escala mas não definiu performance",
			Question: "Quais são os requisitos de performance?",
			Examples: []string{
				"Latência < 200ms (p95)",
				"Throughput: 10K req/s",
				"Disponibilidade: 99.9%",
			},
			CobliExample: `Referência Cobli (500K devices):
  • API REST: < 200ms p95
  • Streaming: < 50ms p95
  • Dashboard: < 3s load
  • Disponibilidade: 99.9%`,
			Severity: "high",
		})
	}
	
	// Check for stack specification
	if !gd.mentionsStack() {
		gaps = append(gaps, Gap{
			Type:     GapStackAmbiguous,
			Title:    "Stack técnico não especificado",
			Context:  "Golden path permite múltiplas stacks",
			Question: "Qual stack será usada neste projeto?",
			Examples: []string{
				"Backend: Go + Gin (padrão Cobli)",
				"Frontend: React + TypeScript",
				"Mobile: React Native",
			},
			CobliExample: `Stack Backend Cobli:
  Go + Gin + Kafka + ScyllaDB
  
  Por quê?
  • Performance: 500K eventos/s
  • Concorrência: goroutines nativas
  • Deploy: binário único`,
			Severity: "medium",
		})
	}
	
	// Check for user context
	if !gd.mentionsUsers() {
		gaps = append(gaps, Gap{
			Type:     GapUserContextMissing,
			Title:    "Contexto de usuários faltando",
			Context:  "Não está claro quem usará o sistema",
			Question: "Quem são os usuários principais?",
			Examples: []string{
				"Gestores de frota (decisores)",
				"Motoristas (usuários finais)",
				"Analistas (power users)",
			},
			CobliExample: `Usuários Cobli:
  • Gestores: dashboard, analytics, decisões
  • Motoristas: app mobile, execução
  • Analistas: BI, relatórios, insights`,
			Severity: "medium",
		})
	}
	
	// Check for scale/volume definition
	if !gd.mentionsVolume() {
		gaps = append(gaps, Gap{
			Type:     GapScaleMissing,
			Title:    "Volume/escala não quantificado",
			Context:  "Não há números sobre volume esperado",
			Question: "Qual o volume esperado?",
			Examples: []string{
				"100K usuários ativos/dia",
				"1M transações/dia",
				"10TB dados armazenados",
			},
			CobliExample: `Escala Cobli:
  • Dispositivos: 500K ativos
  • Eventos: 10M/dia processados
  • Usuários: 100K simultâneos
  • Crescimento: 3x em 2 anos`,
			Severity: "high",
		})
	}

	// Check for security/auth mentions
	if !gd.mentionsSecurity() {
		gaps = append(gaps, Gap{
			Type:     GapSecurityMissing,
			Title:    "Segurança/autenticação não mencionada",
			Context:  "Não há menção a autenticação, autorização ou segurança",
			Question: "Como será feita a autenticação e autorização?",
			Examples: []string{
				"OAuth2 + JWT para APIs",
				"RBAC para controle de acesso",
				"mTLS entre serviços",
			},
			Severity: "medium",
		})
	}

	// Check for observability (monitoring/logging/tracing)
	if !gd.mentionsObservability() {
		gaps = append(gaps, Gap{
			Type:     GapObservabilityMissing,
			Title:    "Observabilidade não definida",
			Context:  "Não há menção a monitoramento, logging ou tracing",
			Question: "Como será feita a observabilidade do sistema?",
			Examples: []string{
				"Prometheus + Grafana para métricas",
				"ELK/Loki para logs centralizados",
				"Jaeger/OpenTelemetry para tracing distribuído",
			},
			Severity: "low",
		})
	}

	// Check for deploy/CI/CD mentions
	if !gd.mentionsDeploy() {
		gaps = append(gaps, Gap{
			Type:     GapDeployMissing,
			Title:    "Deploy/CI/CD não definido",
			Context:  "Não há menção a estratégia de deploy ou pipeline CI/CD",
			Question: "Como será o deploy e pipeline de CI/CD?",
			Examples: []string{
				"GitHub Actions + ArgoCD",
				"Kubernetes com Helm charts",
				"Blue-green deployment",
			},
			Severity: "low",
		})
	}

	return gaps
}

// Helper methods to detect mentions in narrative

func (gd *GapDetector) hasPattern(pattern string) bool {
	// Check if golden path has this pattern
	// Simplified - in real implementation, check actual golden path structure
	return true
}

func (gd *GapDetector) mentionsEvents() bool {
	keywords := []string{"evento", "event", "sourcing", "stream"}
	return gd.containsAny(keywords)
}

func (gd *GapDetector) mentionsPerformance() bool {
	keywords := []string{"performance", "latência", "latency", "throughput", "ms", "segundos"}
	return gd.containsAny(keywords)
}

func (gd *GapDetector) mentionsScale() bool {
	keywords := []string{"usuários", "users", "devices", "dispositivos", "milhões", "million", "k "}
	return gd.containsAny(keywords)
}

func (gd *GapDetector) mentionsStack() bool {
	keywords := []string{"go", "python", "node", "react", "angular", "vue", "stack", "framework"}
	return gd.containsAny(keywords)
}

func (gd *GapDetector) mentionsUsers() bool {
	keywords := []string{"usuário", "user", "cliente", "customer", "gestor", "manager", "motorista", "driver"}
	return gd.containsAny(keywords)
}

func (gd *GapDetector) mentionsVolume() bool {
	keywords := []string{"mil", "thousand", "milhão", "million", "bilhão", "billion"}
	return gd.containsAny(keywords)
}

func (gd *GapDetector) mentionsSecurity() bool {
	keywords := []string{"auth", "autenticação", "autorização", "security", "segurança", "jwt", "oauth", "rbac", "ssl", "tls", "encryption", "criptografia"}
	return gd.containsAny(keywords)
}

func (gd *GapDetector) mentionsObservability() bool {
	keywords := []string{"monitoring", "monitoramento", "logging", "tracing", "observability", "observabilidade", "metrics", "métricas", "prometheus", "grafana", "datadog", "newrelic"}
	return gd.containsAny(keywords)
}

func (gd *GapDetector) mentionsDeploy() bool {
	keywords := []string{"deploy", "ci/cd", "pipeline", "kubernetes", "k8s", "docker", "container", "helm", "argocd", "github actions", "circleci", "jenkins"}
	return gd.containsAny(keywords)
}

func (gd *GapDetector) containsAny(keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(gd.narrative, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// QualityScore calculates a quality score (0-100) for the narrative
func (gd *GapDetector) QualityScore() int {
	gaps := gd.DetectAll()

	// Start with 100
	score := 100

	// Deduct points for each gap
	for _, gap := range gaps {
		switch gap.Severity {
		case "high":
			score -= 20
		case "medium":
			score -= 10
		case "low":
			score -= 5
		}
	}

	if score < 0 {
		score = 0
	}

	return score
}

// --- Input-based gap detection (consolidated from pkg/gaps) ---

// InputDetector detects gaps in structured project input
type InputDetector struct {
	goldenPath *types.GoldenPath
}

// NewInputDetector creates a new input-based gap detector
func NewInputDetector(gp *types.GoldenPath) *InputDetector {
	return &InputDetector{goldenPath: gp}
}

// DetectAll detects all gaps in the project input
func (d *InputDetector) DetectAll(input *types.ProjectInput) []Gap {
	var gaps []Gap

	// Gap: volumetria vaga
	if strings.Contains(strings.ToLower(input.Volumetry), "muito") ||
		strings.Contains(strings.ToLower(input.Volumetry), "muitos") {
		gaps = append(gaps, Gap{
			Type:     GapVolumetryVague,
			Title:    "Volumetria não especificada",
			Context:  "Termos vagos como 'muito' ou 'muitos' detectados",
			Question: "Quantos eventos/usuários/requisições especificamente?",
			Examples: []string{"500k eventos/s", "10k usuários simultâneos", "1M registros/dia"},
			Severity: "high",
		})
	}

	// Gap: RNFs faltando números
	if input.NFRs != "" && !containsNumbers(input.NFRs) {
		gaps = append(gaps, Gap{
			Type:     GapNFRNoNumbers,
			Title:    "RNFs sem métricas mensuráveis",
			Context:  "RNFs devem ter números concretos",
			Question: "Qual a latência P99? Throughput? Uptime?",
			Examples: []string{"Latência P99 <30s", "Throughput >500k/s", "Uptime 99.9%"},
			Severity: "high",
		})
	}

	// Gap: edge cases ausentes
	if input.EdgeCases == "" {
		gaps = append(gaps, Gap{
			Type:     GapEdgeCasesMissing,
			Title:    "Edge cases não documentados",
			Context:  "Cenários de falha não cobertos",
			Question: "O que acontece se API externa cair? Se disco lotar?",
			Examples: []string{"Timeout >30s → circuit breaker", "Disco >90% → alarme"},
			Severity: "medium",
		})
	}

	return gaps
}

// CalculateQualityScore calculates quality score for project input
func (d *InputDetector) CalculateQualityScore(input *types.ProjectInput) int {
	score := 100
	gaps := d.DetectAll(input)

	for _, gap := range gaps {
		switch gap.Severity {
		case "high":
			score -= 20
		case "medium":
			score -= 10
		case "low":
			score -= 5
		}
	}

	if score < 0 {
		score = 0
	}

	return score
}

func containsNumbers(text string) bool {
	for _, char := range text {
		if char >= '0' && char <= '9' {
			return true
		}
	}
	return false
}
