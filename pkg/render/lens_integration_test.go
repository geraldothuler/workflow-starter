//go:build integration

package render

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// HELPER: Cria server httptest isolado (sem poluir DefaultServeMux)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func newTestServer(data *LensData) *httptest.Server {
	s := NewServer(0, data)
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.serveIndex)
	mux.HandleFunc("/api/backlog", s.serveBacklog)
	mux.HandleFunc("/api/meta", s.serveMeta)
	mux.HandleFunc("/static/", s.serveStatic)
	return httptest.NewServer(mux)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// HTTP HANDLER TESTS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestIntegration_ServeIndex_ReturnsHTML(t *testing.T) {
	ts := newTestServer(testLensData())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer resp.Body.Close()

	// Status 200
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Content-Type
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected Content-Type text/html, got %q", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	// Estrutura HTML basica
	checks := []struct {
		needle string
		desc   string
	}{
		{"<html", "abertura html"},
		{"</html>", "fechamento html"},
		{"<head>", "head tag"},
		{"</body>", "fechamento body"},
		{`id="epicGrid"`, "div epicGrid"},
		{`id="deepDivePanel"`, "deep dive panel"},
		{`id="viewEpicDetail"`, "epic detail view"},
		{`id="planning-dashboard"`, "planning dashboard"},
		{`id="milestones-roadmap"`, "milestones roadmap"},
		{`id="kpiGrid"`, "KPI grid"},
		{`/static/scripts.js`, "referencia ao scripts.js"},
		{`/static/styles.css`, "referencia ao styles.css"},
	}

	for _, c := range checks {
		if !strings.Contains(html, c.needle) {
			t.Errorf("HTML missing %s (expected %q)", c.desc, c.needle)
		}
	}

	// Tamanho minimo (template renderizado deve ter ao menos 1KB)
	if len(body) < 1024 {
		t.Errorf("HTML too small: %d bytes (expected > 1KB)", len(body))
	}
}

func TestIntegration_ServeBacklog_ReturnsJSON(t *testing.T) {
	fixture := testLensData()
	ts := newTestServer(fixture)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/backlog")
	if err != nil {
		t.Fatalf("GET /api/backlog failed: %v", err)
	}
	defer resp.Body.Close()

	// Status 200
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Content-Type
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	// JSON valido e parseavel para LensData
	body, _ := io.ReadAll(resp.Body)
	var data LensData
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("failed to unmarshal LensData: %v", err)
	}

	// Dados presentes
	if data.Meta.Title != "Workflow SaaS Platform" {
		t.Errorf("expected title 'Workflow SaaS Platform', got %q", data.Meta.Title)
	}
	if len(data.Epics) != 5 {
		t.Errorf("expected 5 epics, got %d", len(data.Epics))
	}
	if len(data.DeepDives) != 5 {
		t.Errorf("expected 5 deep dives, got %d", len(data.DeepDives))
	}
	if data.Effort.TotalSPs != 34 {
		t.Errorf("expected 34 total SPs, got %d", data.Effort.TotalSPs)
	}
}

func TestIntegration_ServeMeta_ReturnsJSON(t *testing.T) {
	ts := newTestServer(testLensData())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/meta")
	if err != nil {
		t.Fatalf("GET /api/meta failed: %v", err)
	}
	defer resp.Body.Close()

	// Status 200
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Content-Type
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	// JSON parseavel com campo title
	body, _ := io.ReadAll(resp.Body)
	var meta MetaData
	if err := json.Unmarshal(body, &meta); err != nil {
		t.Fatalf("failed to unmarshal MetaData: %v", err)
	}
	if meta.Title != "Workflow SaaS Platform" {
		t.Errorf("expected title 'Workflow SaaS Platform', got %q", meta.Title)
	}
	if len(meta.KPIs) != 4 {
		t.Errorf("expected 4 KPIs, got %d", len(meta.KPIs))
	}
}

func TestIntegration_ServeStatic_CSS(t *testing.T) {
	ts := newTestServer(testLensData())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/static/styles.css")
	if err != nil {
		t.Fatalf("GET /static/styles.css failed: %v", err)
	}
	defer resp.Body.Close()

	// Status 200
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Content-Type
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/css") {
		t.Errorf("expected Content-Type text/css, got %q", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	css := string(body)

	// CSS nao vazio
	if len(body) < 500 {
		t.Errorf("CSS too small: %d bytes (expected > 500)", len(body))
	}

	// Conteudo CSS esperado
	cssChecks := []struct {
		needle string
		desc   string
	}{
		{"--accent", "CSS variable --accent"},
		{".epic-card", "CSS class .epic-card"},
		{".kpi-strip", "CSS class .kpi-strip"},
		{".deep-dive-panel", "CSS class .deep-dive-panel"},
	}
	for _, c := range cssChecks {
		if !strings.Contains(css, c.needle) {
			t.Errorf("CSS missing %s (expected %q)", c.desc, c.needle)
		}
	}
}

func TestIntegration_ServeStatic_JS(t *testing.T) {
	ts := newTestServer(testLensData())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/static/scripts.js")
	if err != nil {
		t.Fatalf("GET /static/scripts.js failed: %v", err)
	}
	defer resp.Body.Close()

	// Status 200
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Content-Type
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "javascript") {
		t.Errorf("expected Content-Type javascript, got %q", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	js := string(body)

	// JS nao vazio
	if len(body) < 500 {
		t.Errorf("JS too small: %d bytes (expected > 500)", len(body))
	}

	// Funcoes JS esperadas
	jsChecks := []struct {
		needle string
		desc   string
	}{
		{"loadBacklog", "funcao loadBacklog"},
		{"renderBacklog", "funcao renderBacklog"},
		{"showDeepDive", "funcao showDeepDive"},
		{"showEpicDetail", "funcao showEpicDetail"},
		{"renderTimeline", "funcao renderTimeline"},
		{"renderPlanningDashboard", "funcao renderPlanningDashboard"},
		{"renderMilestonesRoadmap", "funcao renderMilestonesRoadmap"},
	}
	for _, c := range jsChecks {
		if !strings.Contains(js, c.needle) {
			t.Errorf("JS missing %s (expected %q)", c.desc, c.needle)
		}
	}
}

func TestIntegration_ServeStatic_NotFound(t *testing.T) {
	ts := newTestServer(testLensData())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/static/nonexistent.txt")
	if err != nil {
		t.Fatalf("GET /static/nonexistent.txt failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// METRICS & CLASSIFICATION API (v3.3)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestIntegration_ServeBacklog_IncludesMetrics(t *testing.T) {
	fixture := testLensData()
	ts := newTestServer(fixture)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/backlog")
	if err != nil {
		t.Fatalf("GET /api/backlog failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Parse into raw JSON to check metrics field
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	// metrics field must exist and not be null
	metricsRaw, ok := raw["metrics"]
	if !ok {
		t.Fatal("API response missing 'metrics' field")
	}
	if metricsRaw == nil {
		t.Fatal("'metrics' field is null")
	}

	metrics, ok := metricsRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("'metrics' is not an object: %T", metricsRaw)
	}

	// reduction_percent should be > 0
	reduction, ok := metrics["reduction_percent"].(float64)
	if !ok || reduction <= 0 {
		t.Errorf("expected reduction_percent > 0, got %v", metrics["reduction_percent"])
	}

	// classification_stats should have 4 fields
	clsStats, ok := metrics["classification_stats"].(map[string]interface{})
	if !ok {
		t.Fatal("classification_stats missing or not an object")
	}
	expectedKeys := []string{"trivial", "standard", "specific", "critical"}
	for _, key := range expectedKeys {
		if _, exists := clsStats[key]; !exists {
			t.Errorf("classification_stats missing key %q", key)
		}
	}

	t.Logf("Metrics: reduction=%.1f%%, classification_stats keys=%d",
		reduction, len(clsStats))
}

func TestIntegration_ServeBacklog_IncludesClassification(t *testing.T) {
	fixture := testLensData()
	ts := newTestServer(fixture)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/backlog")
	if err != nil {
		t.Fatalf("GET /api/backlog failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var data LensData
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("failed to unmarshal LensData: %v", err)
	}

	// Each deep dive should have classification and scope
	for term, dd := range data.DeepDives {
		if dd.Classification == "" {
			t.Errorf("deep dive %q missing classification", term)
		}
		if dd.Scope == "" {
			t.Errorf("deep dive %q missing scope", term)
		}

		// Validate classification is a valid value
		validCls := map[string]bool{"trivial": true, "standard": true, "specific": true, "critical": true}
		if !validCls[dd.Classification] {
			t.Errorf("deep dive %q has invalid classification %q", term, dd.Classification)
		}

		// Validate scope is a valid value
		validScope := map[string]bool{"epic": true, "story": true, "global": true}
		if !validScope[dd.Scope] {
			t.Errorf("deep dive %q has invalid scope %q", term, dd.Scope)
		}
	}

	t.Logf("Verified classification+scope for %d deep dives", len(data.DeepDives))
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// STATIC EXPORT TESTS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestIntegration_StaticExport_GeneratesHTML(t *testing.T) {
	data := testLensData()
	exporter := NewStaticExporter(data)

	dir := t.TempDir()
	if err := exporter.Export(dir); err != nil {
		t.Fatalf("StaticExporter.Export failed: %v", err)
	}

	// Arquivo index.html existe
	indexPath := filepath.Join(dir, "index.html")
	info, err := os.Stat(indexPath)
	if err != nil {
		t.Fatalf("index.html not found: %v", err)
	}

	// Tamanho > 1KB
	if info.Size() < 1024 {
		t.Errorf("index.html too small: %d bytes (expected > 1KB)", info.Size())
	}

	content, _ := os.ReadFile(indexPath)
	html := string(content)

	// Estrutura HTML
	checks := []struct {
		needle string
		desc   string
	}{
		{"<html", "abertura html"},
		{"</html>", "fechamento html"},
		{"window.BACKLOG_DATA", "dados inline BACKLOG_DATA"},
		{"window.STATIC_MODE = true", "flag static mode"},
	}
	for _, c := range checks {
		if !strings.Contains(html, c.needle) {
			t.Errorf("Static HTML missing %s (expected %q)", c.desc, c.needle)
		}
	}
}

func TestIntegration_StaticExport_DataIntegrity(t *testing.T) {
	data := testLensData()
	exporter := NewStaticExporter(data)

	dir := t.TempDir()
	if err := exporter.Export(dir); err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(dir, "index.html"))
	html := string(content)

	// Extrair JSON do window.BACKLOG_DATA = {...};
	marker := "window.BACKLOG_DATA = "
	start := strings.Index(html, marker)
	if start == -1 {
		t.Fatal("BACKLOG_DATA marker not found in static HTML")
	}
	start += len(marker)

	// Encontrar o fim do JSON (primeiro ; apos o marker)
	jsonStr := html[start:]
	// O JSON termina antes do proximo ";"
	end := strings.Index(jsonStr, ";")
	if end == -1 {
		t.Fatal("could not find end of BACKLOG_DATA JSON")
	}
	jsonStr = jsonStr[:end]

	// Parsear JSON
	var inlineData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &inlineData); err != nil {
		t.Fatalf("failed to parse inline BACKLOG_DATA: %v\nJSON (first 500 chars): %s", err, jsonStr[:min(500, len(jsonStr))])
	}

	// Verificar presenca de dados
	if _, ok := inlineData["meta"]; !ok {
		t.Error("inline data missing 'meta' key")
	}
	if _, ok := inlineData["epics"]; !ok {
		t.Error("inline data missing 'epics' key")
	}
	if _, ok := inlineData["deep_dives"]; !ok {
		t.Error("inline data missing 'deep_dives' key")
	}

	// Verificar epicos
	epics, ok := inlineData["epics"].(map[string]interface{})
	if !ok {
		t.Fatal("epics is not a map")
	}
	if len(epics) != 5 {
		t.Errorf("expected 5 epics in inline data, got %d", len(epics))
	}
}

func TestIntegration_StaticExport_README(t *testing.T) {
	data := testLensData()
	exporter := NewStaticExporter(data)

	dir := t.TempDir()
	if err := exporter.Export(dir); err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// README.md deve existir
	readmePath := filepath.Join(dir, "README.md")
	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("README.md not found: %v", err)
	}
	if len(content) < 100 {
		t.Errorf("README.md too small: %d bytes", len(content))
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// REGULAR EXPORTER TESTS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestIntegration_Exporter_GeneratesFiles(t *testing.T) {
	data := testLensData()
	exporter := NewExporter(data)

	dir := t.TempDir()
	if err := exporter.Export(dir); err != nil {
		t.Fatalf("Exporter.Export failed: %v", err)
	}

	// Verificar que todos os arquivos esperados existem
	expectedFiles := []struct {
		path string
		desc string
	}{
		{"index.html", "pagina principal"},
		{"backlog.json", "dados do backlog"},
		{"static/styles.css", "stylesheet"},
		{"static/scripts.js", "javascript"},
	}

	for _, f := range expectedFiles {
		fullPath := filepath.Join(dir, f.path)
		info, err := os.Stat(fullPath)
		if err != nil {
			t.Errorf("%s (%s) not found: %v", f.path, f.desc, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("%s (%s) is empty", f.path, f.desc)
		}
	}

	// Verificar backlog.json e parseavel
	backlogData, _ := os.ReadFile(filepath.Join(dir, "backlog.json"))
	var backlogJSON map[string]interface{}
	if err := json.Unmarshal(backlogData, &backlogJSON); err != nil {
		t.Fatalf("backlog.json is not valid JSON: %v", err)
	}
	if _, ok := backlogJSON["epics"]; !ok {
		t.Error("backlog.json missing 'epics' key")
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// TEMPLATE RENDERING TESTS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestIntegration_TemplateRender_NoError(t *testing.T) {
	data := testLensData()
	tmpl := GetIndexTemplate()

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("template.Execute failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("template rendered empty output")
	}

	t.Logf("Template rendered: %d bytes", buf.Len())
}

func TestIntegration_TemplateRender_RequiredElements(t *testing.T) {
	data := testLensData()
	tmpl := GetIndexTemplate()

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("template.Execute failed: %v", err)
	}

	html := buf.String()

	// IDs obrigatorios que devem existir no HTML renderizado
	requiredIDs := []string{
		"epicGrid",
		"viewEpicDetail",
		"deepDivePanel",
		"planning-dashboard",
		"milestones-roadmap",
		"kpiGrid",
		"heroTitle",
		"heroSubtitle",
		"timelineSection",
		"summaryCard",
	}

	for _, id := range requiredIDs {
		needle := `id="` + id + `"`
		if !strings.Contains(html, needle) {
			t.Errorf("rendered HTML missing required element %s", needle)
		}
	}
}

func TestIntegration_TemplateRender_EmptyData(t *testing.T) {
	// Template deve renderizar sem erro mesmo com dados vazios
	data := &LensData{
		Meta:      MetaData{Title: "Empty"},
		Epics:     map[string]EpicLens{},
		Stories:   map[string]StoryLens{},
		DeepDives: map[string]DeepDiveLens{},
	}

	tmpl := GetIndexTemplate()
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("template.Execute with empty data failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("template rendered empty output for empty data")
	}
}

