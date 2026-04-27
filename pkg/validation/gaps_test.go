package validation

import (
	"testing"

	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

func TestNewGapDetector(t *testing.T) {
	gd := NewGapDetector(&types.GoldenPath{}, "some narrative")
	if gd == nil {
		t.Fatal("expected non-nil gap detector")
	}
}

func TestDetectAll_CompleteNarrative(t *testing.T) {
	narrative := `Sistema de telemetria IoT com event sourcing e streaming.
	Performance: latência < 200ms p95, throughput 500k eventos/segundo.
	Stack: Go + Kafka + ScyllaDB + React + TypeScript.
	Usuários: gestores de frota e motoristas.
	Volume: 10 milhões de eventos por dia, 500 mil dispositivos.
	Security: autenticação via OAuth2 + JWT, RBAC para controle de acesso.
	Observability: monitoring com Prometheus e Grafana, logging centralizado.
	Deploy: CI/CD com GitHub Actions, Kubernetes com Helm charts.`

	gd := NewGapDetector(&types.GoldenPath{}, narrative)
	gaps := gd.DetectAll()

	if len(gaps) != 0 {
		t.Errorf("expected 0 gaps for complete narrative, got %d", len(gaps))
		for _, g := range gaps {
			t.Logf("  gap: %s - %s", g.Type, g.Title)
		}
	}
}

func TestDetectAll_EmptyNarrative(t *testing.T) {
	gd := NewGapDetector(&types.GoldenPath{}, "")
	gaps := gd.DetectAll()

	if len(gaps) == 0 {
		t.Error("expected gaps for empty narrative")
	}
	// Should detect: events missing, stack missing, user missing, volume missing
	if len(gaps) < 3 {
		t.Errorf("expected at least 3 gaps, got %d", len(gaps))
	}
}

func TestDetectAll_MissingEvents(t *testing.T) {
	narrative := "Sistema com Go e React para usuários. Volume: 1 milhão requests."
	gd := NewGapDetector(&types.GoldenPath{}, narrative)
	gaps := gd.DetectAll()

	found := false
	for _, g := range gaps {
		if g.Type == GapEventsMissing {
			found = true
			if g.Severity != "high" {
				t.Errorf("expected high severity, got %s", g.Severity)
			}
		}
	}
	if !found {
		t.Error("expected events missing gap")
	}
}

func TestDetectAll_ScaleWithoutPerformance(t *testing.T) {
	narrative := "Sistema para 500k dispositivos IoT com event sourcing. Stack: Go. Usuários: gestores. Volume: 1 milhão."
	gd := NewGapDetector(&types.GoldenPath{}, narrative)
	gaps := gd.DetectAll()

	found := false
	for _, g := range gaps {
		if g.Type == GapNFRMissing {
			found = true
		}
	}
	if !found {
		t.Error("expected NFR missing gap when scale mentioned without performance")
	}
}

func TestQualityScore_Perfect(t *testing.T) {
	narrative := `Event sourcing streaming system. Performance latência 200ms throughput.
	Stack: Go React framework. Usuário gestor motorista.
	Volume: 1 milhão requests por dia.
	Security: autenticação OAuth2, JWT tokens.
	Monitoring com Prometheus, logging centralizado.
	Deploy via CI/CD pipeline com Kubernetes.`

	gd := NewGapDetector(&types.GoldenPath{}, narrative)
	score := gd.QualityScore()

	if score != 100 {
		t.Errorf("expected score 100 for complete narrative, got %d", score)
	}
}

func TestQualityScore_WithGaps(t *testing.T) {
	gd := NewGapDetector(&types.GoldenPath{}, "")
	score := gd.QualityScore()

	if score >= 100 {
		t.Errorf("expected score < 100 with gaps, got %d", score)
	}
	if score < 0 {
		t.Errorf("score should not be negative, got %d", score)
	}
}

func TestMentionsEvents(t *testing.T) {
	gd := NewGapDetector(nil, "usando event sourcing para auditoria")
	if !gd.mentionsEvents() {
		t.Error("should detect event mention")
	}

	gd2 := NewGapDetector(nil, "sistema simples sem nada")
	if gd2.mentionsEvents() {
		t.Error("should not detect events")
	}
}

func TestMentionsStack(t *testing.T) {
	gd := NewGapDetector(nil, "usando react e node para frontend")
	if !gd.mentionsStack() {
		t.Error("should detect stack mention")
	}
}

func TestMentionsUsers(t *testing.T) {
	gd := NewGapDetector(nil, "os gestores e motoristas usam o sistema")
	if !gd.mentionsUsers() {
		t.Error("should detect user mention")
	}
}

func TestContainsAny(t *testing.T) {
	gd := NewGapDetector(nil, "performance latência throughput")
	if !gd.containsAny([]string{"latência"}) {
		t.Error("should find keyword")
	}
	if gd.containsAny([]string{"nonexistent"}) {
		t.Error("should not find missing keyword")
	}
}

// --- New gap type tests ---

func TestDetectAll_MissingSecurity(t *testing.T) {
	narrative := "Sistema com Go e React. Event sourcing. Usuários: gestores. Volume: 1 milhão. Performance: latência 200ms. Monitoring com Prometheus. Deploy via CI/CD."
	gd := NewGapDetector(&types.GoldenPath{}, narrative)
	gaps := gd.DetectAll()

	found := false
	for _, g := range gaps {
		if g.Type == GapSecurityMissing {
			found = true
			if g.Severity != "medium" {
				t.Errorf("expected medium severity, got %s", g.Severity)
			}
		}
	}
	if !found {
		t.Error("expected security_missing gap")
	}
}

func TestDetectAll_MissingObservability(t *testing.T) {
	narrative := "Sistema com Go e React. Event sourcing. Usuários: gestores. Volume: 1 milhão. Performance: latência 200ms. Autenticação OAuth2. Deploy CI/CD pipeline."
	gd := NewGapDetector(&types.GoldenPath{}, narrative)
	gaps := gd.DetectAll()

	found := false
	for _, g := range gaps {
		if g.Type == GapObservabilityMissing {
			found = true
			if g.Severity != "low" {
				t.Errorf("expected low severity, got %s", g.Severity)
			}
		}
	}
	if !found {
		t.Error("expected observability_missing gap")
	}
}

func TestDetectAll_MissingDeploy(t *testing.T) {
	narrative := "Sistema com Go e React. Event sourcing. Usuários: gestores. Volume: 1 milhão. Performance: latência 200ms. Autenticação OAuth2. Monitoring Prometheus."
	gd := NewGapDetector(&types.GoldenPath{}, narrative)
	gaps := gd.DetectAll()

	found := false
	for _, g := range gaps {
		if g.Type == GapDeployMissing {
			found = true
			if g.Severity != "low" {
				t.Errorf("expected low severity, got %s", g.Severity)
			}
		}
	}
	if !found {
		t.Error("expected deploy_missing gap")
	}
}

func TestMentionsSecurity(t *testing.T) {
	gd := NewGapDetector(nil, "autenticação via OAuth2 com JWT")
	if !gd.mentionsSecurity() {
		t.Error("should detect security mention")
	}

	gd2 := NewGapDetector(nil, "sistema simples sem nada")
	if gd2.mentionsSecurity() {
		t.Error("should not detect security")
	}
}

func TestMentionsObservability(t *testing.T) {
	gd := NewGapDetector(nil, "monitoring com Prometheus e Grafana")
	if !gd.mentionsObservability() {
		t.Error("should detect observability mention")
	}

	gd2 := NewGapDetector(nil, "sistema simples sem nada")
	if gd2.mentionsObservability() {
		t.Error("should not detect observability")
	}
}

func TestMentionsDeploy(t *testing.T) {
	gd := NewGapDetector(nil, "deploy via CI/CD pipeline com Kubernetes")
	if !gd.mentionsDeploy() {
		t.Error("should detect deploy mention")
	}

	gd2 := NewGapDetector(nil, "sistema simples sem nada")
	if gd2.mentionsDeploy() {
		t.Error("should not detect deploy")
	}
}

// --- InputDetector tests (consolidated from pkg/gaps) ---

func TestNewInputDetector(t *testing.T) {
	d := NewInputDetector(&types.GoldenPath{})
	if d == nil {
		t.Fatal("expected non-nil input detector")
	}
}

func TestInputDetectAll_NoGaps(t *testing.T) {
	d := NewInputDetector(&types.GoldenPath{})
	input := &types.ProjectInput{
		Volumetry: "500k eventos por segundo",
		NFRs:      "Latência P99 < 200ms, throughput 10000 req/s",
		EdgeCases: "Timeout, circuit breaker, disco cheio",
	}

	gaps := d.DetectAll(input)
	if len(gaps) != 0 {
		t.Errorf("expected 0 gaps for complete input, got %d", len(gaps))
	}
}

func TestInputDetectAll_VagueVolumetry(t *testing.T) {
	d := NewInputDetector(&types.GoldenPath{})
	input := &types.ProjectInput{
		Volumetry: "muitos usuários acessando",
		NFRs:      "Latência < 100ms",
		EdgeCases: "timeout handling",
	}

	gaps := d.DetectAll(input)
	found := false
	for _, g := range gaps {
		if g.Type == GapVolumetryVague {
			found = true
			if g.Severity != "high" {
				t.Errorf("expected severity high, got %s", g.Severity)
			}
		}
	}
	if !found {
		t.Error("expected volumetry_vague gap")
	}
}

func TestInputDetectAll_NFRNoNumbers(t *testing.T) {
	d := NewInputDetector(&types.GoldenPath{})
	input := &types.ProjectInput{
		Volumetry: "500k events/s",
		NFRs:      "sistema deve ser rápido e seguro",
		EdgeCases: "handled",
	}

	gaps := d.DetectAll(input)
	found := false
	for _, g := range gaps {
		if g.Type == GapNFRNoNumbers {
			found = true
		}
	}
	if !found {
		t.Error("expected nfr_no_numbers gap")
	}
}

func TestInputDetectAll_EdgeCasesMissing(t *testing.T) {
	d := NewInputDetector(&types.GoldenPath{})
	input := &types.ProjectInput{
		Volumetry: "500k events",
		NFRs:      "latência 100ms",
		EdgeCases: "",
	}

	gaps := d.DetectAll(input)
	found := false
	for _, g := range gaps {
		if g.Type == GapEdgeCasesMissing {
			found = true
			if g.Severity != "medium" {
				t.Errorf("expected severity medium, got %s", g.Severity)
			}
		}
	}
	if !found {
		t.Error("expected edge_cases_missing gap")
	}
}

func TestInputCalculateQualityScore_Perfect(t *testing.T) {
	d := NewInputDetector(&types.GoldenPath{})
	input := &types.ProjectInput{
		Volumetry: "500k events/s",
		NFRs:      "P99 < 200ms",
		EdgeCases: "timeout, circuit breaker",
	}

	score := d.CalculateQualityScore(input)
	if score != 100 {
		t.Errorf("expected score 100, got %d", score)
	}
}

func TestInputCalculateQualityScore_WithGaps(t *testing.T) {
	d := NewInputDetector(&types.GoldenPath{})
	input := &types.ProjectInput{
		Volumetry: "muitos acessos",
		NFRs:      "deve ser rápido",
		EdgeCases: "",
	}

	score := d.CalculateQualityScore(input)
	if score >= 100 {
		t.Errorf("expected score < 100 with gaps, got %d", score)
	}
}

func TestContainsNumbers(t *testing.T) {
	if !containsNumbers("latência 100ms") {
		t.Error("should detect numbers")
	}
	if containsNumbers("sem numeros aqui") {
		t.Error("should not detect numbers")
	}
}

// --- Resolver tests (consolidated from pkg/gaps) ---

func TestNewAutoResolver(t *testing.T) {
	r := NewAutoResolver()
	if r == nil {
		t.Fatal("expected non-nil resolver")
	}
	res, err := r.Resolve(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(res) != 0 {
		t.Error("expected empty resolutions")
	}
}

func TestNewInteractiveResolver(t *testing.T) {
	r := NewInteractiveResolver()
	if r == nil {
		t.Fatal("expected non-nil resolver")
	}
	res, err := r.Resolve(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(res) != 0 {
		t.Error("expected empty resolutions")
	}
}
