//go:build browser

package render

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg" // register JPEG decoder for image.Decode
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// HELPERS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// startBrowserTestServer cria httptest server com handlers do Lens
func startBrowserTestServer(data *LensData) *httptest.Server {
	s := NewServer(0, data)
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.serveIndex)
	mux.HandleFunc("/api/backlog", s.serveBacklog)
	mux.HandleFunc("/api/meta", s.serveMeta)
	mux.HandleFunc("/static/", s.serveStatic)
	return httptest.NewServer(mux)
}

// newBrowserContext cria context chromedp headless (ou visivel com BROWSER_INSPECT=1)
func newBrowserContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	inspect := os.Getenv("BROWSER_INSPECT") == "1"

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", !inspect),
		chromedp.Flag("disable-gpu", !inspect),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)

	if inspect {
		opts = append(opts, chromedp.WindowSize(1400, 900))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)

	ctx, cancel := chromedp.NewContext(allocCtx)

	timeout := 30 * time.Second
	if inspect {
		timeout = 10 * time.Minute
	}
	ctx, timeoutCancel := context.WithTimeout(ctx, timeout)

	cleanup := func() {
		if inspect {
			t.Log("BROWSER_INSPECT=1 — browser aberto. Pressione Ctrl+C para encerrar.")
			<-ctx.Done()
		}
		timeoutCancel()
		cancel()
		allocCancel()
	}

	return ctx, cleanup
}

// collectConsoleErrors injeta script que captura console.error
// e retorna os erros coletados
func collectConsoleErrors(ctx context.Context, url string) ([]string, error) {
	var errors []string

	// Injetar interceptor de console.error ANTES de carregar a pagina
	err := chromedp.Run(ctx,
		// Navegar e esperar
		chromedp.Navigate(url),

		// Injetar coleta de erros
		chromedp.Evaluate(`
			window.__consoleErrors = [];
			const origError = console.error;
			console.error = function() {
				window.__consoleErrors.push(Array.from(arguments).join(' '));
				origError.apply(console, arguments);
			};
			const origWarn = console.warn;
			console.warn = function() {
				const msg = Array.from(arguments).join(' ');
				if (msg.includes('Error') || msg.includes('error')) {
					window.__consoleErrors.push('WARN: ' + msg);
				}
				origWarn.apply(console, arguments);
			};
			window.onerror = function(msg, src, line, col, error) {
				window.__consoleErrors.push('ONERROR: ' + msg + ' at ' + src + ':' + line);
			};
			window.addEventListener('unhandledrejection', function(e) {
				window.__consoleErrors.push('UNHANDLED: ' + e.reason);
			});
		`, nil),

		// Esperar a pagina carregar e JS executar
		chromedp.Sleep(3*time.Second),

		// Coletar erros
		chromedp.Evaluate(`window.__consoleErrors || []`, &errors),
	)

	return errors, err
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CONSOLE ERROR DETECTION
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestBrowser_ServerMode_NoJSErrors(t *testing.T) {
	ts := startBrowserTestServer(testLensData())
	defer ts.Close()

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	errors, err := collectConsoleErrors(ctx, ts.URL)
	if err != nil {
		t.Fatalf("chromedp failed: %v", err)
	}

	if len(errors) > 0 {
		for i, e := range errors {
			t.Errorf("JS error [%d]: %s", i, e)
		}
		t.Fatalf("expected 0 JS errors in server mode, got %d", len(errors))
	}

	t.Log("Server mode: 0 JS console errors")
}

func TestBrowser_StaticMode_NoJSErrors(t *testing.T) {
	data := testLensData()
	exporter := NewStaticExporter(data)

	dir := t.TempDir()
	if err := exporter.Export(dir); err != nil {
		t.Fatalf("StaticExport failed: %v", err)
	}

	indexPath := filepath.Join(dir, "index.html")
	fileURL := "file://" + indexPath

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	errors, err := collectConsoleErrors(ctx, fileURL)
	if err != nil {
		t.Fatalf("chromedp failed: %v", err)
	}

	// Filtrar erros de CORS/file:// que sao esperados
	var realErrors []string
	for _, e := range errors {
		// file:// pode gerar warnings sobre fonts, ignorar
		if strings.Contains(e, "fonts.googleapis.com") {
			continue
		}
		// file:// pode gerar warnings sobre fetch, ignorar
		if strings.Contains(e, "Failed to fetch") || strings.Contains(e, "NetworkError") {
			continue
		}
		realErrors = append(realErrors, e)
	}

	if len(realErrors) > 0 {
		for i, e := range realErrors {
			t.Errorf("JS error [%d]: %s", i, e)
		}
		t.Fatalf("expected 0 JS errors in static mode, got %d", len(realErrors))
	}

	t.Log("Static mode: 0 JS console errors (excluding expected file:// warnings)")
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// DOM RENDERING
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestBrowser_DOM_EpicGridRendered(t *testing.T) {
	ts := startBrowserTestServer(testLensData())
	defer ts.Close()

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	var epicCardCount int
	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`document.querySelectorAll('.epic-card').length`, &epicCardCount),
	)
	if err != nil {
		t.Fatalf("chromedp failed: %v", err)
	}

	if epicCardCount < 2 {
		t.Errorf("expected at least 2 epic cards rendered, got %d", epicCardCount)
	}

	t.Logf("Epic cards rendered: %d", epicCardCount)
}

func TestBrowser_DOM_KPICardsRendered(t *testing.T) {
	ts := startBrowserTestServer(testLensData())
	defer ts.Close()

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	var kpiCount int
	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`document.querySelectorAll('.kpi-card').length`, &kpiCount),
	)
	if err != nil {
		t.Fatalf("chromedp failed: %v", err)
	}

	if kpiCount < 4 {
		t.Errorf("expected at least 4 KPI cards, got %d", kpiCount)
	}

	t.Logf("KPI cards rendered: %d", kpiCount)
}

func TestBrowser_DOM_HeroTitlePopulated(t *testing.T) {
	ts := startBrowserTestServer(testLensData())
	defer ts.Close()

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	var heroTitle string
	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`document.getElementById('heroTitle').textContent`, &heroTitle),
	)
	if err != nil {
		t.Fatalf("chromedp failed: %v", err)
	}

	// Apos carregamento, titulo NAO deve ser "Carregando..."
	if heroTitle == "Carregando..." || heroTitle == "" {
		t.Errorf("heroTitle not populated: %q (expected project title)", heroTitle)
	}

	t.Logf("Hero title: %q", heroTitle)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CSS VALIDATION
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestBrowser_CSS_VariablesApplied(t *testing.T) {
	ts := startBrowserTestServer(testLensData())
	defer ts.Close()

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	var accentValue string
	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`getComputedStyle(document.documentElement).getPropertyValue('--accent').trim()`, &accentValue),
	)
	if err != nil {
		t.Fatalf("chromedp failed: %v", err)
	}

	if accentValue == "" {
		t.Error("CSS variable --accent not applied (empty)")
	}

	t.Logf("CSS --accent: %q", accentValue)
}

func TestBrowser_CSS_EpicCardVisible(t *testing.T) {
	ts := startBrowserTestServer(testLensData())
	defer ts.Close()

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	var display string
	var visibility string
	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`
			(() => {
				const card = document.querySelector('.epic-card');
				if (!card) return 'NO_CARD_FOUND';
				return getComputedStyle(card).display;
			})()
		`, &display),
		chromedp.Evaluate(`
			(() => {
				const card = document.querySelector('.epic-card');
				if (!card) return 'NO_CARD_FOUND';
				return getComputedStyle(card).visibility;
			})()
		`, &visibility),
	)
	if err != nil {
		t.Fatalf("chromedp failed: %v", err)
	}

	if display == "none" || display == "NO_CARD_FOUND" {
		t.Errorf("epic card display: %q (expected visible)", display)
	}
	if visibility == "hidden" {
		t.Errorf("epic card visibility: %q (expected visible)", visibility)
	}

	t.Logf("Epic card: display=%q, visibility=%q", display, visibility)
}

func TestBrowser_LayoutContainerBounds(t *testing.T) {
	ts := startBrowserTestServer(testLensData())
	defer ts.Close()

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	var resultsJSON string
	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`
			(() => {
				const maxW = 960;
				const checks = [
					{sel: '.hero-content', name: 'Hero Content'},
					{sel: '#kpiGrid', name: 'KPI Grid'},
					{sel: '.main', name: 'Main Content'},
					{sel: '#deepDivesSection', name: 'Deep Dives'},
					{sel: '#metricsSection', name: 'Metrics'},
					{sel: '.planning-section .section-header', name: 'Planning Header'},
					{sel: '.milestones-section .section-header', name: 'Milestones Header'}
				];
				var failures = [];
				var log = [];
				for (var i = 0; i < checks.length; i++) {
					var c = checks[i];
					var el = document.querySelector(c.sel);
					if (!el) { log.push(c.name + ': not found'); continue; }
					var w = el.getBoundingClientRect().width;
					log.push(c.name + ': ' + Math.round(w) + 'px');
					if (w > maxW + 1) {
						failures.push(c.name + ' (' + c.sel + '): ' + Math.round(w) + 'px exceeds 960px');
					}
				}
				return JSON.stringify({failures: failures, log: log});
			})()
		`, &resultsJSON),
	)
	if err != nil {
		t.Fatalf("chromedp failed: %v", err)
	}

	var result struct {
		Failures []string `json:"failures"`
		Log      []string `json:"log"`
	}
	if err := json.Unmarshal([]byte(resultsJSON), &result); err != nil {
		t.Fatalf("parse results: %v", err)
	}

	for _, l := range result.Log {
		t.Logf("  %s", l)
	}

	if len(result.Failures) > 0 {
		for _, f := range result.Failures {
			t.Errorf("❌ Layout overflow: %s", f)
		}
		t.Fatalf("expected all sections to fit within 960px container, got %d overflow(s)", len(result.Failures))
	}

	t.Log("✅ All sections within 960px container bounds")
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// INTERACOES
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestBrowser_Click_EpicDetail(t *testing.T) {
	ts := startBrowserTestServer(testLensData())
	defer ts.Close()

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	var detailHTML string
	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Sleep(3*time.Second),

		// Clicar no primeiro epic card
		chromedp.Evaluate(`
			(() => {
				const card = document.querySelector('.epic-card');
				if (card) card.click();
				return card ? 'clicked' : 'no_card';
			})()
		`, nil),

		chromedp.Sleep(1*time.Second),

		// Verificar que detail view tem conteudo
		chromedp.Evaluate(`document.getElementById('viewEpicDetail').innerHTML`, &detailHTML),
	)
	if err != nil {
		t.Fatalf("chromedp failed: %v", err)
	}

	if len(detailHTML) == 0 {
		t.Error("viewEpicDetail empty after clicking epic card")
	}

	t.Logf("Epic detail HTML length: %d chars", len(detailHTML))
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// DEEP DIVES ACCESSIBILITY (v3.3)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestBrowser_DeepDives_SectionVisible(t *testing.T) {
	ts := startBrowserTestServer(testLensData())
	defer ts.Close()

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	var sectionDisplay string
	var cardCount int
	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`
			(() => {
				const section = document.getElementById('deepDivesSection');
				if (!section) return 'NOT_FOUND';
				return getComputedStyle(section).display;
			})()
		`, &sectionDisplay),
		chromedp.Evaluate(`document.querySelectorAll('.dd-overview-card').length`, &cardCount),
	)
	if err != nil {
		t.Fatalf("chromedp failed: %v", err)
	}

	if sectionDisplay == "none" || sectionDisplay == "NOT_FOUND" {
		t.Errorf("Deep Dives section not visible: display=%q", sectionDisplay)
	}

	if cardCount < 1 {
		t.Errorf("expected at least 1 deep dive overview card, got %d", cardCount)
	}

	t.Logf("Deep Dives section: display=%q, cards=%d", sectionDisplay, cardCount)
}

func TestBrowser_DeepDives_StoryIndicator(t *testing.T) {
	ts := startBrowserTestServer(testLensData())
	defer ts.Close()

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	var indicatorCount int
	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Sleep(3*time.Second),

		// Click first epic to show story list
		chromedp.Evaluate(`
			(() => {
				const card = document.querySelector('.epic-card');
				if (card) card.click();
				return 'ok';
			})()
		`, nil),

		chromedp.Sleep(1*time.Second),

		// Count story deep-dive indicators
		chromedp.Evaluate(`document.querySelectorAll('.story-dd-indicator').length`, &indicatorCount),
	)
	if err != nil {
		t.Fatalf("chromedp failed: %v", err)
	}

	// At least one story should have deep dive indicator (E2.1 has JWT)
	t.Logf("Story deep-dive indicators found: %d", indicatorCount)
}

func TestBrowser_DeepDives_TagRowVisible(t *testing.T) {
	ts := startBrowserTestServer(testLensData())
	defer ts.Close()

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	var tagCount int
	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Sleep(3*time.Second),

		// Click last epic (E2 has JWT deep dive)
		chromedp.Evaluate(`
			(() => {
				const cards = document.querySelectorAll('.epic-card');
				if (cards.length > 1) cards[cards.length - 1].click();
				else if (cards.length > 0) cards[0].click();
				return 'ok';
			})()
		`, nil),

		chromedp.Sleep(1*time.Second),

		// Click story to expand it
		chromedp.Evaluate(`
			(() => {
				const story = document.querySelector('.story-head');
				if (story) story.click();
				return 'ok';
			})()
		`, nil),

		chromedp.Sleep(500*time.Millisecond),

		// Count tag elements with deep dive link
		chromedp.Evaluate(`document.querySelectorAll('.tag.has-dive').length`, &tagCount),
	)
	if err != nil {
		t.Fatalf("chromedp failed: %v", err)
	}

	t.Logf("Deep dive tags in expanded story: %d", tagCount)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// STATIC EXPORT BROWSER
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestBrowser_StaticExport_RendersOffline(t *testing.T) {
	data := testLensData()
	exporter := NewStaticExporter(data)

	dir := t.TempDir()
	if err := exporter.Export(dir); err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	indexPath := filepath.Join(dir, "index.html")
	fileURL := "file://" + indexPath

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	var epicCardCount int
	var backlogDataExists bool
	err := chromedp.Run(ctx,
		chromedp.Navigate(fileURL),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`document.querySelectorAll('.epic-card').length`, &epicCardCount),
		chromedp.Evaluate(`typeof window.BACKLOG_DATA !== 'undefined' && window.BACKLOG_DATA !== null`, &backlogDataExists),
	)
	if err != nil {
		t.Fatalf("chromedp failed: %v", err)
	}

	if !backlogDataExists {
		t.Error("window.BACKLOG_DATA not found in static export")
	}

	// Em static mode, epics devem ser renderizados a partir do BACKLOG_DATA inline
	t.Logf("Static export: %d epic cards rendered, BACKLOG_DATA exists: %v", epicCardCount, backlogDataExists)
}

func TestBrowser_StaticExport_StaticModeFlag(t *testing.T) {
	data := testLensData()
	exporter := NewStaticExporter(data)

	dir := t.TempDir()
	if err := exporter.Export(dir); err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	indexPath := filepath.Join(dir, "index.html")

	// Verificar que o flag existe no HTML sem abrir browser
	content, _ := os.ReadFile(indexPath)
	if !strings.Contains(string(content), "STATIC_MODE = true") {
		t.Error("static export missing STATIC_MODE flag")
	}

	// Verificar via browser que o flag é acessível
	ctx, cancel := newBrowserContext(t)
	defer cancel()

	var staticMode bool
	err := chromedp.Run(ctx,
		chromedp.Navigate("file://"+indexPath),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`window.STATIC_MODE === true`, &staticMode),
	)
	if err != nil {
		t.Fatalf("chromedp failed: %v", err)
	}

	if !staticMode {
		t.Error("window.STATIC_MODE not true in browser context")
	}

	t.Log("Static mode flag verified in browser")
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// PAGE SIZE / PERFORMANCE
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestBrowser_PageLoad_Performance(t *testing.T) {
	ts := startBrowserTestServer(testLensData())
	defer ts.Close()

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	start := time.Now()

	var domReady bool
	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`document.readyState === 'complete'`, &domReady),
	)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("chromedp failed: %v", err)
	}

	if !domReady {
		t.Error("page not fully loaded after 3s")
	}

	// Pagina deve carregar em menos de 10s
	if elapsed > 10*time.Second {
		t.Errorf("page load too slow: %v (expected < 10s)", elapsed)
	}

	t.Logf("Page load time: %v (DOM ready: %v)", elapsed, domReady)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// DIAGNOSTICS (helper para debug — nao e teste, nao falha)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestBrowser_Diagnostics(t *testing.T) {
	ts := startBrowserTestServer(testLensData())
	defer ts.Close()

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	var diagnostics string
	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`
			(() => {
				const d = {};
				d.epicCards = document.querySelectorAll('.epic-card').length;
				d.kpiCards = document.querySelectorAll('.kpi-card').length;
				d.heroTitle = document.getElementById('heroTitle').textContent;
				d.heroSubtitle = document.getElementById('heroSubtitle').textContent;
				d.cssVarAccent = getComputedStyle(document.documentElement).getPropertyValue('--accent').trim();
				d.cssVarBg = getComputedStyle(document.documentElement).getPropertyValue('--bg').trim();
				d.bodyFontFamily = getComputedStyle(document.body).fontFamily;
				d.deepDivePanelDisplay = getComputedStyle(document.getElementById('deepDivePanel')).transform;
				d.planningDashboardHTML = document.getElementById('planning-dashboard').innerHTML.length;
				d.milestonesHTML = document.getElementById('milestones-roadmap').innerHTML.length;
				d.deepDivesSectionDisplay = document.getElementById('deepDivesSection') ? getComputedStyle(document.getElementById('deepDivesSection')).display : 'NOT_FOUND';
				d.deepDiveCards = document.querySelectorAll('.dd-overview-card').length;
				d.metricsSectionDisplay = document.getElementById('metricsSection') ? getComputedStyle(document.getElementById('metricsSection')).display : 'NOT_FOUND';
				d.metricsCards = document.querySelectorAll('.metrics-card').length;
				d.totalElements = document.querySelectorAll('*').length;
				return JSON.stringify(d, null, 2);
			})()
		`, &diagnostics),
	)
	if err != nil {
		t.Logf("Diagnostics failed (non-fatal): %v", err)
		return
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🔍 Browser Diagnostics")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println(diagnostics)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// METRICS DASHBOARD (v3.3)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestBrowser_Metrics_SectionVisible(t *testing.T) {
	ts := startBrowserTestServer(testLensData())
	defer ts.Close()

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	var sectionDisplay string
	var cardCount int
	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`
			(() => {
				const section = document.getElementById('metricsSection');
				if (!section) return 'NOT_FOUND';
				return getComputedStyle(section).display;
			})()
		`, &sectionDisplay),
		chromedp.Evaluate(`document.querySelectorAll('.metrics-card').length`, &cardCount),
	)
	if err != nil {
		t.Fatalf("chromedp failed: %v", err)
	}

	// metricsSection should be visible (display != "none") when metrics data exists
	if sectionDisplay == "none" || sectionDisplay == "NOT_FOUND" {
		t.Errorf("Metrics section not visible: display=%q (expected block or similar)", sectionDisplay)
	}

	// Should have at least 4 metrics cards (Techs Extracted, Trivial Filtered, LLM Calls Saved, Total Cost)
	if cardCount < 4 {
		t.Errorf("expected at least 4 metrics cards, got %d", cardCount)
	}

	t.Logf("Metrics section: display=%q, cards=%d", sectionDisplay, cardCount)
}

func TestBrowser_Metrics_ReductionDisplayed(t *testing.T) {
	ts := startBrowserTestServer(testLensData())
	defer ts.Close()

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	var metricsHTML string
	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`
			(() => {
				const container = document.getElementById('metrics-dashboard');
				if (!container) return '';
				return container.innerHTML;
			})()
		`, &metricsHTML),
	)
	if err != nil {
		t.Fatalf("chromedp failed: %v", err)
	}

	// The fixture has ReductionPercent: 60.0, LLMCallsSaved: 12
	// The JS renders: value = metrics.llm_calls_saved and label includes "60%"
	if !strings.Contains(metricsHTML, "60%") {
		t.Errorf("expected ReductionPercent '60%%' in metrics dashboard, HTML length=%d", len(metricsHTML))
	}

	if !strings.Contains(metricsHTML, "12") {
		t.Errorf("expected LLMCallsSaved '12' in metrics dashboard")
	}

	t.Logf("Metrics dashboard HTML length: %d chars, contains reduction: %v",
		len(metricsHTML), strings.Contains(metricsHTML, "60%"))
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// i18n VALIDATION (v3.3)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func TestBrowser_i18n_HTMLLangAttribute(t *testing.T) {
	ts := startBrowserTestServer(testLensData())
	defer ts.Close()

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	var lang string
	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`document.documentElement.lang`, &lang),
	)
	if err != nil {
		t.Fatalf("chromedp failed: %v", err)
	}

	// testLensData() sets Meta.Lang = "pt-BR"
	if lang != "pt-BR" {
		t.Errorf("expected html lang='pt-BR', got %q", lang)
	}

	t.Logf("HTML lang attribute: %q", lang)
}

func TestBrowser_i18n_SectionTitlesInPtBR(t *testing.T) {
	ts := startBrowserTestServer(testLensData())
	defer ts.Close()

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	var resultsJSON string
	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`
			(() => {
				var checks = {};
				// Check known section title elements for PT-BR content
				var titleIds = [
					'epicsSectionTitle',
					'timelineSectionTitle',
					'deepDivesSectionTitle',
					'planningSectionTitle',
					'metricsSectionTitle'
				];
				for (var i = 0; i < titleIds.length; i++) {
					var el = document.getElementById(titleIds[i]);
					checks[titleIds[i]] = el ? el.textContent.trim() : 'NOT_FOUND';
				}
				return JSON.stringify(checks);
			})()
		`, &resultsJSON),
	)
	if err != nil {
		t.Fatalf("chromedp failed: %v", err)
	}

	var titles map[string]string
	if err := json.Unmarshal([]byte(resultsJSON), &titles); err != nil {
		t.Fatalf("parse titles: %v", err)
	}

	// PT-BR section titles expected (from i18n system in scripts.js)
	ptBRIndicators := []string{"Épicos", "Planejamento", "Linha do Tempo", "Deep Dives", "Insights"}
	foundPtBR := false
	for id, title := range titles {
		t.Logf("  %s: %q", id, title)
		for _, indicator := range ptBRIndicators {
			if strings.Contains(title, indicator) {
				foundPtBR = true
			}
		}
	}

	if !foundPtBR {
		t.Error("no PT-BR section titles found — i18n may not be applied")
	}

	t.Logf("i18n PT-BR titles verified: %v", foundPtBR)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// SCREENSHOT CAPTURE (make screenshots)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// screenshotDir retorna o caminho para docs/images/ relativo ao root do projeto
func screenshotDir() string {
	// Subir de pkg/render/ para root do projeto
	dir, err := filepath.Abs(filepath.Join("..", "..", "docs", "images"))
	if err != nil {
		return "docs/images"
	}
	return dir
}

// elementBounds represents the absolute position and size of a DOM element
type elementBounds struct {
	X      int
	Y      int
	Width  int
	Height int
}

// getElementBounds returns the union bounding box of one or more DOM elements.
// Coordinates are absolute (relative to page top, not viewport).
func getElementBounds(ctx context.Context, selectors ...string) (*elementBounds, error) {
	quotedSels := make([]string, len(selectors))
	for i, s := range selectors {
		quotedSels[i] = "'" + s + "'"
	}

	var result map[string]interface{}
	err := chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				var sels = [%s];
				var minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
				for (var i = 0; i < sels.length; i++) {
					var el = document.querySelector(sels[i]);
					if (!el) continue;
					var r = el.getBoundingClientRect();
					minX = Math.min(minX, r.left + window.scrollX);
					minY = Math.min(minY, r.top + window.scrollY);
					maxX = Math.max(maxX, r.left + window.scrollX + r.width);
					maxY = Math.max(maxY, r.top + window.scrollY + r.height);
				}
				if (minX === Infinity) return null;
				return {x: Math.round(minX), y: Math.round(minY),
				        width: Math.round(maxX - minX), height: Math.round(maxY - minY)};
			})()
		`, strings.Join(quotedSels, ",")), &result),
	)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("elements not found: %v", selectors)
	}

	b := &elementBounds{
		X:      int(result["x"].(float64)),
		Y:      int(result["y"].(float64)),
		Width:  int(result["width"].(float64)),
		Height: int(result["height"].(float64)),
	}
	if b.Width <= 0 || b.Height <= 0 {
		return nil, fmt.Errorf("element %v has zero dimensions (hidden?)", selectors)
	}
	return b, nil
}

// cropImage crops a region from a full-page screenshot and returns PNG bytes.
// Input is JPEG (from chromedp.FullScreenshot), output is actual PNG.
func cropImage(fullPage []byte, bounds *elementBounds, padding int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(fullPage))
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	imgB := img.Bounds()
	cropRect := image.Rect(
		max(bounds.X-padding, imgB.Min.X),
		max(bounds.Y-padding, imgB.Min.Y),
		min(bounds.X+bounds.Width+padding, imgB.Max.X),
		min(bounds.Y+bounds.Height+padding, imgB.Max.Y),
	)

	if cropRect.Dx() <= 0 || cropRect.Dy() <= 0 {
		return nil, fmt.Errorf("crop has no area (%dx%d) — element may be off-screen",
			cropRect.Dx(), cropRect.Dy())
	}

	// Create normalized image with 0,0 origin
	dst := image.NewRGBA(image.Rect(0, 0, cropRect.Dx(), cropRect.Dy()))
	draw.Draw(dst, dst.Bounds(), img, cropRect.Min, draw.Src)

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	return buf.Bytes(), nil
}

// cropAndSave gets element bounds, crops from a pre-captured screenshot, and saves as PNG.
func cropAndSave(ctx context.Context, t *testing.T, capture []byte, filename string, padding int, selectors ...string) {
	t.Helper()

	bounds, err := getElementBounds(ctx, selectors...)
	if err != nil {
		t.Errorf("bounds for %s: %v", filename, err)
		return
	}

	cropped, err := cropImage(capture, bounds, padding)
	if err != nil {
		t.Errorf("crop %s: %v", filename, err)
		return
	}

	outPath := filepath.Join(screenshotDir(), filename)
	if err := os.WriteFile(outPath, cropped, 0644); err != nil {
		t.Errorf("write %s: %v", outPath, err)
		return
	}

	t.Logf("📸 %s (%d KB, %dx%d)", filename, len(cropped)/1024,
		bounds.Width+2*padding, bounds.Height+2*padding)
}

// getViewportBounds returns viewport-relative element bounds (no scrollY offset).
// Use with viewport screenshots (CaptureScreenshot) where coordinates are relative to the viewport.
func getViewportBounds(ctx context.Context, selectors ...string) (*elementBounds, error) {
	quotedSels := make([]string, len(selectors))
	for i, s := range selectors {
		quotedSels[i] = fmt.Sprintf("'%s'", s)
	}

	var result map[string]interface{}
	err := chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				var sels = [%s];
				var minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
				for (var i = 0; i < sels.length; i++) {
					var el = document.querySelector(sels[i]);
					if (!el) continue;
					var r = el.getBoundingClientRect();
					minX = Math.min(minX, r.left);
					minY = Math.min(minY, r.top);
					maxX = Math.max(maxX, r.left + r.width);
					maxY = Math.max(maxY, r.top + r.height);
				}
				if (minX === Infinity) return null;
				return {x: Math.round(Math.max(0, minX)), y: Math.round(Math.max(0, minY)),
				        width: Math.round(maxX - Math.max(0, minX)),
				        height: Math.round(maxY - Math.max(0, minY))};
			})()
		`, strings.Join(quotedSels, ",")), &result),
	)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("elements not found: %v", selectors)
	}

	b := &elementBounds{
		X:      int(result["x"].(float64)),
		Y:      int(result["y"].(float64)),
		Width:  int(result["width"].(float64)),
		Height: int(result["height"].(float64)),
	}
	if b.Width <= 0 || b.Height <= 0 {
		return nil, fmt.Errorf("element %v has zero dimensions (hidden?)", selectors)
	}
	return b, nil
}

// captureSection scrolls to an element, takes a viewport screenshot, crops, and saves as PNG.
// Solves the FullScreenshot painting issue where off-screen elements aren't rendered by Chrome.
func captureSection(ctx context.Context, t *testing.T, filename string, padding int, selectors ...string) {
	t.Helper()

	// Scroll the element into view
	if err := scrollToElement(ctx, selectors[0]); err != nil {
		t.Errorf("scroll for %s: %v", filename, err)
		return
	}

	// Wait for repaint
	chromedp.Run(ctx, chromedp.Sleep(300*time.Millisecond))

	// Take viewport screenshot (PNG, elements in view are guaranteed painted)
	var viewportPNG []byte
	if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&viewportPNG)); err != nil {
		t.Errorf("viewport capture for %s: %v", filename, err)
		return
	}

	// Get viewport-relative bounds (no scrollY offset)
	bounds, err := getViewportBounds(ctx, selectors...)
	if err != nil {
		t.Errorf("viewport bounds for %s: %v", filename, err)
		return
	}

	// Crop and save
	cropped, err := cropImage(viewportPNG, bounds, padding)
	if err != nil {
		t.Errorf("crop %s: %v", filename, err)
		return
	}

	outPath := filepath.Join(screenshotDir(), filename)
	if err := os.WriteFile(outPath, cropped, 0644); err != nil {
		t.Errorf("write %s: %v", outPath, err)
		return
	}

	t.Logf("📸 %s (%d KB, %dx%d)", filename, len(cropped)/1024,
		bounds.Width+2*padding, bounds.Height+2*padding)
}

// injectHighlight adds a dashed accent border around a DOM element.
// Used to annotate interactive elements in screenshots. Browser-rendered for perfect quality.
func injectHighlight(ctx context.Context, selector string) error {
	js := `
		(() => {
			var prev = document.getElementById('screenshot-annotation');
			if (prev) prev.remove();
			var el = document.querySelector('` + selector + `');
			if (!el) return 'not_found';
			var rect = el.getBoundingClientRect();
			var sy = window.scrollY, sx = window.scrollX;
			var c = document.createElement('div');
			c.id = 'screenshot-annotation';
			c.style.cssText = 'position:absolute;top:0;left:0;width:100%;height:100%;pointer-events:none;z-index:9999';
			var h = document.createElement('div');
			h.style.cssText = 'position:absolute;top:'+(rect.top+sy-4)+'px;left:'+(rect.left+sx-4)+'px;width:'+(rect.width+8)+'px;height:'+(rect.height+8)+'px;border:2.5px dashed #1a6b4e;border-radius:8px;box-shadow:0 0 0 4px rgba(26,107,78,0.15)';
			c.appendChild(h);
			document.body.appendChild(c);
			return 'ok';
		})()`

	return chromedp.Run(ctx, chromedp.Evaluate(js, nil))
}

// removeHighlight removes the annotation overlay injected by injectHighlight.
func removeHighlight(ctx context.Context) error {
	return chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				var el = document.getElementById('screenshot-annotation');
				if (el) el.remove();
				return 'ok';
			})()
		`, nil),
	)
}

// scrollToElement scrolls an element into view
func scrollToElement(ctx context.Context, selector string) error {
	return chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const el = document.querySelector('%s');
				if (el) el.scrollIntoView({behavior: 'instant', block: 'start'});
				return el ? 'ok' : 'not_found';
			})()
		`, selector), nil),
		chromedp.Sleep(500*time.Millisecond),
	)
}

// TestBrowser_CaptureScreenshots captures 10 section-level PNG crops for docs/images/.
// Strategy: scrolls to each section, takes a viewport screenshot (CaptureScreenshot),
// then crops using viewport-relative getBoundingClientRect() bounds.
// This avoids the FullScreenshot painting issue where off-screen elements are blank.
// Gated by WTB_CAPTURE_SCREENSHOTS=1 (use: make screenshots).
func TestBrowser_CaptureScreenshots(t *testing.T) {
	if os.Getenv("WTB_CAPTURE_SCREENSHOTS") != "1" {
		t.Skip("Set WTB_CAPTURE_SCREENSHOTS=1 to capture (or use: make screenshots)")
	}

	outDir := screenshotDir()
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("cannot create output dir %s: %v", outDir, err)
	}

	ts := startBrowserTestServer(testLensData())
	defer ts.Close()

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	// Setup: viewport 1440x900, navegar e esperar carregamento completo
	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1440, 900),
		chromedp.Navigate(ts.URL),
		chromedp.Sleep(3*time.Second),
	)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// ── Phase 1: Overview sections (scroll + viewport capture for each) ──

	// 1. Hero + KPIs — capture from top of page (already in view)
	captureSection(ctx, t, "lens-01-hero-kpis.png", 0, ".hero", ".kpi-strip")

	// 2. Epic List
	captureSection(ctx, t, "lens-02-epic-list.png", 20, "#viewEpicList")

	// 3. Deep Dives Overview
	captureSection(ctx, t, "lens-03-deep-dives.png", 20, "#deepDivesSection")

	// 4. Timeline (was blank with FullScreenshot — now scroll + viewport capture)
	captureSection(ctx, t, "lens-04-timeline.png", 20, "#timelineSection")

	// 5. Planning Dashboard
	captureSection(ctx, t, "lens-05-planning.png", 20, ".planning-section")

	// 6. Metrics Dashboard
	captureSection(ctx, t, "lens-06-metrics.png", 20, "#metricsSection")

	// 7. Roadmap / Milestones
	captureSection(ctx, t, "lens-07-roadmap.png", 20, ".milestones-section")

	// ── Phase 2: Epic Detail + Story Detail ───────────────────────────

	// Navigate back to top and click first epic card
	chromedp.Run(ctx,
		chromedp.Evaluate(`window.scrollTo(0,0)`, nil),
		chromedp.Sleep(300*time.Millisecond),
	)
	chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				var card = document.querySelector('.epic-card');
				if (card) card.click();
				return 'ok';
			})()
		`, nil),
		chromedp.Sleep(1*time.Second),
	)
	chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				var story = document.querySelector('.story-head');
				if (story) story.click();
				return 'ok';
			})()
		`, nil),
		chromedp.Sleep(500*time.Millisecond),
	)

	// 8. Epic Detail — annotate story-dd-indicator, then viewport capture
	injectHighlight(ctx, ".story-dd-indicator")
	scrollToElement(ctx, "#viewEpicDetail")
	chromedp.Run(ctx, chromedp.Sleep(300*time.Millisecond))
	var capture8 []byte
	chromedp.Run(ctx, chromedp.CaptureScreenshot(&capture8))
	bounds8, _ := getViewportBounds(ctx, "#viewEpicDetail")
	if bounds8 != nil {
		cropped8, err := cropImage(capture8, bounds8, 20)
		if err == nil {
			os.WriteFile(filepath.Join(outDir, "lens-08-epic-detail.png"), cropped8, 0644)
			t.Logf("📸 lens-08-epic-detail.png (%d KB, %dx%d)",
				len(cropped8)/1024, bounds8.Width+40, bounds8.Height+40)
		}
	}
	removeHighlight(ctx)

	// 9. Story Detail — annotate deep dive tags
	injectHighlight(ctx, ".tag.has-dive")
	chromedp.Run(ctx, chromedp.Sleep(200*time.Millisecond))
	var capture9 []byte
	chromedp.Run(ctx, chromedp.CaptureScreenshot(&capture9))
	bounds9, _ := getViewportBounds(ctx, ".story-card.open")
	if bounds9 != nil {
		cropped9, err := cropImage(capture9, bounds9, 20)
		if err == nil {
			os.WriteFile(filepath.Join(outDir, "lens-09-story-detail.png"), cropped9, 0644)
			t.Logf("📸 lens-09-story-detail.png (%d KB, %dx%d)",
				len(cropped9)/1024, bounds9.Width+40, bounds9.Height+40)
		}
	}
	removeHighlight(ctx)

	// ── Phase 3: Deep Dive Panel ──────────────────────────────────────

	// Click deep dive tag to open panel, with verification
	var clickResult string
	chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				var tag = document.querySelector('.tag.has-dive');
				if (tag) { tag.click(); return 'clicked: ' + tag.textContent; }
				var techTag = document.querySelector('.tech-tag');
				if (techTag) { techTag.click(); return 'clicked-tech: ' + techTag.textContent; }
				return 'no_tag';
			})()
		`, &clickResult),
		chromedp.Sleep(1*time.Second),
	)
	t.Logf("Deep dive tag click: %s", clickResult)

	// Verify panel opened; if not, try direct JS call as fallback
	var panelOpen bool
	chromedp.Run(ctx,
		chromedp.Evaluate(`document.getElementById('deepDivePanel').classList.contains('open')`, &panelOpen),
	)
	if !panelOpen {
		t.Log("Panel not opened via tag click, trying direct openDeepDive() call")
		chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					var keys = Object.keys(window.BACKLOG_DATA && window.BACKLOG_DATA.deep_dives || {});
					if (keys.length > 0) { openDeepDive(keys[0]); return 'opened: ' + keys[0]; }
					return 'no_deep_dives';
				})()
			`, &clickResult),
			chromedp.Sleep(1*time.Second),
		)
		t.Logf("Direct openDeepDive: %s", clickResult)
	}

	// 10. Deep Dive Panel — viewport screenshot (panel is fixed-position overlay)
	chromedp.Run(ctx, chromedp.Evaluate(`window.scrollTo(0,0)`, nil), chromedp.Sleep(300*time.Millisecond))
	var capture10 []byte
	if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&capture10)); err != nil {
		t.Errorf("viewport capture for deep dive panel: %v", err)
	} else {
		outPath := filepath.Join(outDir, "lens-10-deep-dive.png")
		if err := os.WriteFile(outPath, capture10, 0644); err != nil {
			t.Errorf("write panel PNG: %v", err)
		} else {
			t.Logf("📸 lens-10-deep-dive.png (%d KB, 1440x900 viewport)", len(capture10)/1024)
		}
	}

	// ── Summary ───────────────────────────────────────────────────────

	files, _ := filepath.Glob(filepath.Join(outDir, "lens-*.png"))
	t.Logf("\n📸 %d screenshots capturados em %s", len(files), outDir)
	var totalSize int64
	for _, f := range files {
		info, _ := os.Stat(f)
		if info != nil {
			t.Logf("   %s (%d KB)", filepath.Base(f), info.Size()/1024)
			totalSize += info.Size()
		}
	}
	t.Logf("   Total: %d KB", totalSize/1024)
}
