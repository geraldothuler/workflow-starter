// guardrail.go — drift containment, not evolution blocker.
//
// Three structural checks that protect platform premises:
//
//   chain-diagram-sync  README Mermaid diverges from use-cases/*/definition.yml
//   zero-llm-ops        LLM import detected in pkg/ops/ (zero-LLM probe contract)
//   usecase-definition  use-cases/ directory missing definition.yml
//   context-json-drift  YAML heuristic value diverges from context.json canonical entry
//
// Intentional evolution passes through with an explicit override comment:
//   // wtb-noguard: <check> — <justificativa>
package doccheck

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/docs"
	"github.com/Cobliteam/workflow-toolkit/pkg/memory"
)

var hrefRe = regexp.MustCompile(`href="([^"]+)"`)

// GuardrailResult is the outcome of a single structural check.
type GuardrailResult struct {
	Check  string // check identifier (e.g. "chain-diagram-sync")
	Passed bool
	Detail string // instructive multi-line message when failed
	Fix    string // suggested command to resolve drift
}

// String returns a one-line summary for passed checks and the full detail for failures.
func (r GuardrailResult) String() string {
	if r.Passed {
		return fmt.Sprintf("  ✓ %s", r.Check)
	}
	return r.Detail
}

// RunAll executes all checks and returns results in a stable order.
func RunAll(repoRoot string) []GuardrailResult {
	return []GuardrailResult{
		CheckChainDiagramSync(repoRoot),
		CheckLLMInOpsPackage(repoRoot),
		CheckUseCasesWithoutDefinition(repoRoot),
		CheckAnonymizationInDocs(repoRoot),
		CheckDocsHtmlStandard(repoRoot),
		CheckDocsHtmlLinks(repoRoot),
		CheckMemoryIndexBloat(),
		CheckMemoryContentLeak(),
		CheckMemoryIndexOrphan(),
		CheckContextJsonDrift(repoRoot),
	}
}

// CheckChainDiagramSync verifies that the Mermaid block in README.md matches
// the output of docs.GenerateMermaid() from use-cases/*/definition.yml.
//
// Drift: someone added/changed a use-case without regenerating the diagram.
// Fix:   wtb docs chain --repo <root>
//
// Override (intentional simplified diagram):
//
//	<!-- wtb-noguard: chain-diagram-sync — <justificativa> -->
func CheckChainDiagramSync(repoRoot string) GuardrailResult {
	const check = "chain-diagram-sync"
	const overrideMarker = "wtb-noguard: chain-diagram-sync"

	defs, err := docs.LoadUseCases(repoRoot)
	if err != nil || len(defs) == 0 {
		return GuardrailResult{Check: check, Passed: true} // nothing to compare
	}

	generated := strings.TrimSpace(docs.GenerateMermaid(defs))

	readmePath := filepath.Join(repoRoot, "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		return GuardrailResult{Check: check, Passed: true} // no README to check
	}

	readmeContent := string(data)

	// Intentional evolution: override marker in README suppresses this check.
	if strings.Contains(readmeContent, overrideMarker) {
		return GuardrailResult{Check: check, Passed: true}
	}

	existing := extractMermaidBlock(readmeContent)
	if existing == "" {
		fix := fmt.Sprintf("wtb docs chain --repo %s", repoRoot)
		return GuardrailResult{
			Check:  check,
			Passed: false,
			Detail: guardrailMessage(check,
				"README.md não contém bloco Mermaid de chain.",
				"O diagrama é gerado de use-cases/*/definition.yml e nunca mantido à mão.\n"+
					"Sem ele, a cadeia de workflows não é visível no entry point do repo.",
				"wtb docs chain --repo "+repoRoot,
				"<!-- wtb-noguard: chain-diagram-sync — <justificativa> -->"),
			Fix: fix,
		}
	}

	if existing == generated {
		return GuardrailResult{Check: check, Passed: true}
	}

	fix := fmt.Sprintf("wtb docs chain --repo %s", repoRoot)
	return GuardrailResult{
		Check:  check,
		Passed: false,
		Detail: guardrailMessage(check,
			"Diagrama Mermaid no README diverge dos use-cases atuais.",
			"Um use-case foi adicionado, removido ou alterado sem regenerar.\n"+
				"O diagrama é a fonte de verdade visual da cadeia — drift aqui\n"+
				"significa que README descreve uma plataforma que não existe mais.",
			"wtb docs chain --repo "+repoRoot,
			"<!-- wtb-noguard: chain-diagram-sync — <justificativa> -->"),
		Fix: fix,
	}
}

// CheckLLMInOpsPackage verifies that no file in pkg/ops/ imports pkg/llm.
//
// The zero-LLM probe contract means all ops probes use deterministic heuristics:
// millisecond latency, $0 cost, auditable output.
//
// Drift:     LLM added to a probe for convenience, without architectural decision.
// Evolution: intentional — add override and document the reasoning (discovery/ADR).
// Override:  // wtb-noguard: zero-llm — <justificativa>
func CheckLLMInOpsPackage(repoRoot string) GuardrailResult {
	const check = "zero-llm-ops"

	opsDir := filepath.Join(repoRoot, "pkg", "ops")
	matches, err := filepath.Glob(filepath.Join(opsDir, "*.go"))
	if err != nil || len(matches) == 0 {
		return GuardrailResult{Check: check, Passed: true}
	}

	const llmImport = `"github.com/Cobliteam/workflow-toolkit/pkg/llm"`
	const overrideMarker = "wtb-noguard: zero-llm"

	var violations []string
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		if strings.Contains(content, overrideMarker) {
			continue // intentional — override present
		}
		if strings.Contains(content, llmImport) {
			rel, _ := filepath.Rel(repoRoot, path)
			violations = append(violations, rel)
		}
	}

	if len(violations) == 0 {
		return GuardrailResult{Check: check, Passed: true}
	}

	fileList := "  " + strings.Join(violations, "\n  ")
	return GuardrailResult{
		Check:  check,
		Passed: false,
		Detail: guardrailMessage(check,
			"Import de pkg/llm detectado em pkg/ops/:\n"+fileList,
			"pkg/ops/ é zona zero-LLM: decisões por regras determinísticas,\n"+
				"latência de milissegundos, custo $0, resultado auditável.\n"+
				"É o que diferencia os probes de uma chamada genérica de IA.",
			"Mova a lógica para config/*.yml (heurística YAML-driven)\n"+
				"ou orquestre com LLM fora do probe, via pkg/runner.",
			"// wtb-noguard: zero-llm — <justificativa da decisão>"),
	}
}

// CheckUseCasesWithoutDefinition verifies that every directory under use-cases/
// contains a definition.yml file.
//
// Drift:     new use-case directory created (scaffold, copy) without the YAML contract.
// Evolution: a new use-case intentionally in progress — add definition.yml minimal.
func CheckUseCasesWithoutDefinition(repoRoot string) GuardrailResult {
	const check = "usecase-definition"

	dirs, err := filepath.Glob(filepath.Join(repoRoot, "use-cases", "*"))
	if err != nil {
		return GuardrailResult{Check: check, Passed: true}
	}

	var missing []string
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "definition.yml")); os.IsNotExist(err) {
			rel, _ := filepath.Rel(repoRoot, dir)
			missing = append(missing, rel)
		}
	}

	if len(missing) == 0 {
		return GuardrailResult{Check: check, Passed: true}
	}

	dirList := "  " + strings.Join(missing, "\n  ")
	return GuardrailResult{
		Check:  check,
		Passed: false,
		Detail: guardrailMessage(check,
			"Diretório(s) em use-cases/ sem definition.yml:\n"+dirList,
			"Todo use-case precisa de um definition.yml — é o contrato que\n"+
				"define inputs, steps, engine e chain. Sem ele, o runner não\n"+
				"executa e o diagrama de chain fica incompleto.",
			"Crie use-cases/<tipo>/definition.yml seguindo o modelo em\n"+
				"use-cases/incident/definition.yml.",
			""),
	}
}

// CheckDocsHtmlStandard verifies that every HTML file in docs/ (except index.html):
//  1. Has a breadcrumb link back to index.html: href="index.html"
//  2. Has a corresponding card in docs/index.html: href="<filename>"
//
// Drift:     new document added without breadcrumb or index card.
// Evolution: docs/index.html intentionally excludes a private/draft doc — add override.
// Override:  <!-- wtb-noguard: docs-html-standard — <justificativa> --> in the HTML file.
func CheckDocsHtmlStandard(repoRoot string) GuardrailResult {
	const check = "docs-html-standard"

	docsDir := filepath.Join(repoRoot, "docs")
	htmlFiles, _ := filepath.Glob(filepath.Join(docsDir, "*.html"))
	if len(htmlFiles) == 0 {
		return GuardrailResult{Check: check, Passed: true}
	}

	indexPath := filepath.Join(docsDir, "index.html")
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		return GuardrailResult{Check: check, Passed: true}
	}
	indexContent := string(indexData)

	var missingBreadcrumb, missingFromIndex []string
	for _, path := range htmlFiles {
		name := filepath.Base(path)
		if name == "index.html" || strings.HasSuffix(name, ".bak") {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		if strings.Contains(content, "wtb-noguard: docs-html-standard") {
			continue
		}
		if !strings.Contains(content, `href="index.html"`) {
			rel, _ := filepath.Rel(repoRoot, path)
			missingBreadcrumb = append(missingBreadcrumb, rel)
		}
		if !strings.Contains(indexContent, `href="`+name+`"`) {
			rel, _ := filepath.Rel(repoRoot, path)
			missingFromIndex = append(missingFromIndex, rel)
		}
	}

	if len(missingBreadcrumb) == 0 && len(missingFromIndex) == 0 {
		return GuardrailResult{Check: check, Passed: true}
	}

	var parts []string
	if len(missingBreadcrumb) > 0 {
		parts = append(parts, "Sem breadcrumb (href=\"index.html\" no footer):\n  "+strings.Join(missingBreadcrumb, "\n  "))
	}
	if len(missingFromIndex) > 0 {
		parts = append(parts, "Não listado em docs/index.html:\n  "+strings.Join(missingFromIndex, "\n  "))
	}

	return GuardrailResult{
		Check:  check,
		Passed: false,
		Detail: guardrailMessage(check,
			strings.Join(parts, "\n\n"),
			"Todo HTML em docs/ precisa:\n"+
				"  1. Footer com: <a href=\"index.html\">← Todos os documentos</a>\n"+
				"  2. Card correspondente em docs/index.html\n"+
				"Sem isso o documento fica inacessível pela navegação do GitHub Pages.",
			"Adicione o breadcrumb no footer e o card em docs/index.html.",
			"<!-- wtb-noguard: docs-html-standard — <justificativa> -->"),
	}
}

// CheckDocsHtmlLinks verifies that internal links in docs/*.html are not broken
// and that links to .md files use absolute GitHub URLs (not relative paths).
//
// Broken link:   href="missing.html" where docs/missing.html does not exist.
// Relative MD:   href="README.md" or href="../docs/guide.md" — GitHub Pages does not
//                serve .md files; must use https://github.com/<repo>/blob/main/path.
//
// Scope: only href="..." with relative targets (http/https/# are skipped).
func CheckDocsHtmlLinks(repoRoot string) GuardrailResult {
	const check = "docs-html-links"

	docsDir := filepath.Join(repoRoot, "docs")
	htmlFiles, _ := filepath.Glob(filepath.Join(docsDir, "*.html"))
	if len(htmlFiles) == 0 {
		return GuardrailResult{Check: check, Passed: true}
	}

	var broken, relativeMD []string

	for _, path := range htmlFiles {
		if strings.HasSuffix(path, ".bak") {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		rel, _ := filepath.Rel(repoRoot, path)

		for _, m := range hrefRe.FindAllStringSubmatch(content, -1) {
			href := m[1]
			// Skip absolute URLs and anchors
			if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") ||
				strings.HasPrefix(href, "#") || strings.HasPrefix(href, "mailto:") ||
				strings.HasPrefix(href, "javascript:") {
				continue
			}
			// Relative .md link → must be absolute GitHub URL
			if strings.Contains(href, ".md") {
				relativeMD = append(relativeMD, fmt.Sprintf("%s → %s", rel, href))
				continue
			}
			// Relative .html link → check existence
			if strings.HasSuffix(href, ".html") {
				target := filepath.Join(docsDir, href)
				if _, err := os.Stat(target); os.IsNotExist(err) {
					broken = append(broken, fmt.Sprintf("%s → %s", rel, href))
				}
			}
		}
	}

	if len(broken) == 0 && len(relativeMD) == 0 {
		return GuardrailResult{Check: check, Passed: true}
	}

	var parts []string
	if len(broken) > 0 {
		parts = append(parts, "Links internos quebrados:\n  "+strings.Join(broken, "\n  "))
	}
	if len(relativeMD) > 0 {
		parts = append(parts, "Links .md relativos (GitHub Pages não serve .md):\n  "+strings.Join(relativeMD, "\n  "))
	}

	return GuardrailResult{
		Check:  check,
		Passed: false,
		Detail: guardrailMessage(check,
			strings.Join(parts, "\n\n"),
			"Links internos quebrados tornam o documento inacessível.\n"+
				"Links .md relativos não funcionam no GitHub Pages — use URL absoluta:\n"+
				"  https://github.com/Cobliteam/workflow-toolkit/blob/main/<path>",
			"Corrija os hrefs: verifique existência dos .html e substitua\n"+
				".md relativos por URLs absolutas do GitHub.",
			""),
	}
}

// CheckMemoryIndexBloat verifies that MEMORY.md (the LLM's always-loaded index) stays within
// the enforced line limit. MEMORY.md is a pure index — all content belongs in topic files.
//
// Drift: heuristics or verbose blocks leak into MEMORY.md over time → context window wasted
//        on content that is only relevant to a subset of tasks.
// Fix:   move verbose content to the appropriate memory/topic-file.md
// Limit: 160 lines (generous to accommodate the Keychain + topic table index structure).
//
// The check resolves MEMORY.md via $HOME/.claude/projects/*workflow*/memory/MEMORY.md.
// Skip gracefully if the file cannot be located (CI or different environment).
func CheckMemoryIndexBloat() GuardrailResult {
	const check = "memory-index-bloat"
	const maxLines = 160

	path := findMemoryFile()
	if path == "" {
		return GuardrailResult{Check: check, Passed: true} // not found → skip
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return GuardrailResult{Check: check, Passed: true}
	}

	count := strings.Count(string(data), "\n")
	if count <= maxLines {
		return GuardrailResult{Check: check, Passed: true}
	}

	return GuardrailResult{
		Check:  check,
		Passed: false,
		Detail: guardrailMessage(check,
			fmt.Sprintf("MEMORY.md tem %d linhas (limite: %d).", count, maxLines),
			"MEMORY.md é carregado em toda sessão. Conteúdo detalhado aqui desperdiça\n"+
				"contexto em tarefas que não precisam desse conhecimento.",
			"Mova heurísticas detalhadas, snippets de código e blocos longos\n"+
				"para o topic file correspondente em memory/*.md.\n"+
				"MEMORY.md deve conter apenas: tabela Keychain, índice de topic files,\n"+
				"tabela de repos, e regras em uma linha.",
			""),
	}
}

// CheckMemoryContentLeak verifies that MEMORY.md contains no fenced code blocks.
// Code blocks (```) are a signal that implementation detail leaked into the index file.
//
// Drift: someone adds a curl snippet, bash command, or config block directly to MEMORY.md
//        instead of the relevant topic file.
// Fix:   move the code block to memory/topic-file.md and replace with a one-liner + ref.
//
// Override: add <!-- wtb-noguard: memory-content-leak — <justificativa> --> to MEMORY.md.
func CheckMemoryContentLeak() GuardrailResult {
	const check = "memory-content-leak"

	path := findMemoryFile()
	if path == "" {
		return GuardrailResult{Check: check, Passed: true}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return GuardrailResult{Check: check, Passed: true}
	}

	content := string(data)

	// Allow override
	if strings.Contains(content, "wtb-noguard: memory-content-leak") {
		return GuardrailResult{Check: check, Passed: true}
	}

	// Count fenced code blocks (``` occurrences in pairs)
	fenceCount := strings.Count(content, "```")
	if fenceCount == 0 {
		return GuardrailResult{Check: check, Passed: true}
	}

	return GuardrailResult{
		Check:  check,
		Passed: false,
		Detail: guardrailMessage(check,
			fmt.Sprintf("MEMORY.md contém %d marcador(es) de bloco de código (```).", fenceCount),
			"Blocos de código em MEMORY.md indicam que conteúdo de implementação\n"+
				"vazou para o arquivo de índice. MEMORY.md deve ser prosa, tabelas\n"+
				"e one-liners — código pertence nos topic files.",
			"Mova o bloco para o topic file correspondente (memory/*.md)\n"+
				"e substitua por uma referência: \"Ver memory/topic-file.md\"",
			"<!-- wtb-noguard: memory-content-leak — <justificativa> -->"),
	}
}

// CheckContextJsonDrift verifies that values annotated with `# context.json: <key>`
// in heuristic YAML files match the corresponding entries in context.json.
//
// Drift: someone updated context.json via `wtb memory set` but the YAML literal
//        was not updated (or vice versa) — heuristic runs with stale threshold.
//
// Pattern matched (inline on a value line):
//
//	value: <X>  # context.json: <key>
//
// Header comments like `# context.json: key1, key2` are intentionally skipped —
// they document the relationship without asserting a specific value.
func CheckContextJsonDrift(repoRoot string) GuardrailResult {
	const check = "context-json-drift"

	store, err := memory.LoadStore(repoRoot)
	if err != nil || len(store.FilterByTopic("")) == 0 {
		return GuardrailResult{Check: check, Passed: true}
	}

	heuristicsDir := filepath.Join(repoRoot, "pkg", "ops", "config", "heuristics")
	files, err := filepath.Glob(filepath.Join(heuristicsDir, "*.yml"))
	if err != nil || len(files) == 0 {
		return GuardrailResult{Check: check, Passed: true}
	}

	// Matches lines like:  "        value: 0.30  # context.json: webhook_delivery_ratio_warn"
	// Captures: [1] yaml value, [2] context.json key
	re := regexp.MustCompile(`^\s+value:\s+(\S+)\s+#\s+context\.json:\s+(\w+)`)

	type driftEntry struct {
		file   string
		line   int
		key    string
		inYAML string
		inJSON string
	}
	var drifts []driftEntry

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(repoRoot, path)
		for i, line := range strings.Split(string(data), "\n") {
			m := re.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			yamlVal, key := m[1], m[2]
			entry, ok := store.Get(key)
			if !ok {
				drifts = append(drifts, driftEntry{
					file: rel, line: i + 1, key: key,
					inYAML: yamlVal, inJSON: "(ausente em context.json)",
				})
				continue
			}
			if !contextValuesEqual(yamlVal, entry.Value) {
				drifts = append(drifts, driftEntry{
					file: rel, line: i + 1, key: key,
					inYAML: yamlVal, inJSON: entry.Value,
				})
			}
		}
	}

	if len(drifts) == 0 {
		return GuardrailResult{Check: check, Passed: true}
	}

	var lines []string
	for _, d := range drifts {
		lines = append(lines, fmt.Sprintf("  %s:%d  key=%s  yaml=%s  context.json=%s",
			d.file, d.line, d.key, d.inYAML, d.inJSON))
	}

	return GuardrailResult{
		Check:  check,
		Passed: false,
		Detail: guardrailMessage(check,
			"Valores divergentes entre heurísticas YAML e context.json:\n"+strings.Join(lines, "\n"),
			"O valor canônico vive em context.json. A anotação `# context.json: <key>`\n"+
				"documenta qual chave deve ser consultada antes de agir.\n"+
				"Drift significa que a heurística roda com um threshold desatualizado.",
			"Sincronize: wtb memory set <key> <valor> --type <type> --topic <topic> --desc \"<desc>\"",
			""),
	}
}

// contextValuesEqual returns true if a and b represent the same value.
// Compares as float64 when both are parseable, falls back to string equality.
func contextValuesEqual(a, b string) bool {
	fa, errA := strconv.ParseFloat(strings.TrimSpace(a), 64)
	fb, errB := strconv.ParseFloat(strings.TrimSpace(b), 64)
	if errA == nil && errB == nil {
		return fa == fb
	}
	return strings.TrimSpace(a) == strings.TrimSpace(b)
}

// CheckMemoryIndexOrphan verifies that every topic file referenced in MEMORY.md
// as `memory/*.md` actually exists on disk alongside MEMORY.md.
//
// Drift: entry added to MEMORY.md pointing to a topic file that was never created,
//        renamed, or was deleted — the pointer becomes a dead reference.
//
// Fix: create the missing topic file or remove the stale pointer from MEMORY.md.
func CheckMemoryIndexOrphan() GuardrailResult {
	const check = "memory-index-orphan"

	memPath := findMemoryFile()
	if memPath == "" {
		return GuardrailResult{Check: check, Passed: true}
	}

	data, err := os.ReadFile(memPath)
	if err != nil {
		return GuardrailResult{Check: check, Passed: true}
	}

	memDir := filepath.Dir(memPath)

	// Match backtick-quoted memory/*.md references: `memory/some-file.md`
	re := regexp.MustCompile("`memory/([^`]+\\.md)`")
	seen := map[string]bool{}
	var missing []string

	for _, m := range re.FindAllStringSubmatch(string(data), -1) {
		filename := m[1]
		if seen[filename] {
			continue
		}
		seen[filename] = true
		target := filepath.Join(memDir, filename)
		if _, err := os.Stat(target); os.IsNotExist(err) {
			missing = append(missing, "memory/"+filename)
		}
	}

	if len(missing) == 0 {
		return GuardrailResult{Check: check, Passed: true}
	}

	fileList := "  " + strings.Join(missing, "\n  ")
	return GuardrailResult{
		Check:  check,
		Passed: false,
		Detail: guardrailMessage(check,
			"Ponteiros em MEMORY.md para topic files inexistentes:\n"+fileList,
			"MEMORY.md é um índice — cada `memory/*.md` referenciado deve existir em disco.\n"+
				"Ponteiro morto significa que o contexto não pode ser carregado quando necessário.",
			"Crie o topic file ausente ou remova o ponteiro de MEMORY.md.",
			""),
	}
}

// findMemoryFile resolves the path to MEMORY.md in the Claude Code project cache.
// Returns empty string if not found (e.g. CI environment, different user).
func findMemoryFile() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	pattern := filepath.Join(homeDir, ".claude", "projects", "*workflow*", "memory", "MEMORY.md")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}
	return matches[0]
}

// extractMermaidBlock returns the trimmed content between the first ```mermaid and ``` fences.
func extractMermaidBlock(content string) string {
	var inBlock bool
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if !inBlock && line == "```mermaid" {
			inBlock = true
			continue
		}
		if inBlock && line == "```" {
			break
		}
		if inBlock {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// guardrailMessage formats an instructive guardrail failure message.
// situation: what was detected
// why:       the premise being protected
// action:    what to do to resolve
// override:  override comment syntax (empty = no override available)
func guardrailMessage(check, situation, why, action, override string) string {
	var sb strings.Builder
	line := strings.Repeat("━", 52)
	fmt.Fprintf(&sb, "\n⚠  wtb guardrail — %s\n%s\n", check, line)
	fmt.Fprintf(&sb, "%s\n\n", situation)
	fmt.Fprintf(&sb, "%s\n\n", why)
	fmt.Fprintf(&sb, "→ %s\n", action)
	if override != "" {
		fmt.Fprintf(&sb, "\nEvolução intencional? Adicione no arquivo:\n  %s\n", override)
	}
	return sb.String()
}
