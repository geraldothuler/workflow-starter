package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	
	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// Input representa dados de entrada
type Input struct {
	BacklogPath    string
	DeepDivesPath  string
	TasksPath      string
}


// AutoDetectInput detecta inputs automaticamente
func AutoDetectInput(basePath ...string) (Input, error) {
	base := "."
	if len(basePath) > 0 {
		base = basePath[0]
	}
	return Input{
		BacklogPath:   filepath.Join(base, "backlog.json"),
		DeepDivesPath: filepath.Join(base, "deep-dives.json"),
		TasksPath:     filepath.Join(base, "tasks.json"),
	}, nil
}

// LoadOrBuild carrega backlog e converte para LensData
func LoadOrBuild(input *Input, rebuild ...bool) (*LensData, error) {
	// 1. Carregar backlog.json
	backlogData, err := os.ReadFile(input.BacklogPath)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler backlog: %w", err)
	}

	// 2. Tentar carregar como LensData (novo formato - com effort/milestones)
	var lensData LensData
	if err := json.Unmarshal(backlogData, &lensData); err == nil {
		// JSON já está no formato LensData completo
		return &lensData, nil
	}
	
	// 3. Fallback: carregar como types.Backlog (formato antigo)
	var backlog types.Backlog
	if err := json.Unmarshal(backlogData, &backlog); err != nil {
		return nil, fmt.Errorf("erro ao parsear backlog: %w", err)
	}

	// 4. Carregar deep dives se existir
	var deepDives []types.DeepDive
	if input.DeepDivesPath != "" {
		ddData, err := os.ReadFile(input.DeepDivesPath)
		if err == nil {
			json.Unmarshal(ddData, &deepDives)
		}
	}

	// 5. Converter formato antigo para novo
	convertedData := ConvertToLensData(&backlog, deepDives)

	return convertedData, nil
}

// Server servidor do Lens
type Server struct {
	port int
	data *LensData
}

// NewServer cria novo servidor
func NewServer(port int, data *LensData) *Server {
	return &Server{
		port: port,
		data: data,
	}
}


// Start inicia servidor
func (s *Server) Start() error {
	http.HandleFunc("/", s.serveIndex)
	http.HandleFunc("/api/backlog", s.serveBacklog)
	http.HandleFunc("/api/meta", s.serveMeta)
	http.HandleFunc("/static/", s.serveStatic)
	
	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("\n🌐 Lens Server rodando em: http://localhost%s\n", addr)
	fmt.Println("📊 Endpoints:")
	fmt.Println("  /            - Interface visual")
	fmt.Println("  /api/backlog - JSON completo")
	fmt.Println("  /api/meta    - Metadados")
	fmt.Println("\n⏹️  Pressione Ctrl+C para parar")
	
	return http.ListenAndServe(addr, nil)
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	// Preparar dados para o template
	var buf bytes.Buffer
	
	// Executar template
	if err := GetIndexTemplate().Execute(&buf, s.data); err != nil {
		http.Error(w, "Erro ao renderizar template", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(buf.Bytes())
}

func (s *Server) serveBacklog(w http.ResponseWriter, r *http.Request) {
	// Servir LensData completo (inclui effort, milestones, documents, etc)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.data)
}

// replaceFirst substitui apenas a primeira ocorrência de old por new
func replaceFirst(s, old, new string) string {
	i := strings.Index(s, old)
	if i == -1 {
		return s
	}
	return s[:i] + new + s[i+len(old):]
}

func (s *Server) serveMeta(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.data.Meta)
}

// Exporter exporta lens para arquivos estáticos
type Exporter struct {
	data *LensData
}

// NewExporter cria novo exporter
func NewExporter(data *LensData) *Exporter {
	return &Exporter{data: data}
}

// Export exporta para diretório
func (e *Exporter) Export(dir string) error {
	// Criar diretório
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	
	// Escrever index.html usando template
	indexPath := filepath.Join(dir, "index.html")
	var buf bytes.Buffer
	if err := GetIndexTemplate().Execute(&buf, e.data); err != nil {
		return fmt.Errorf("erro ao renderizar template: %w", err)
	}
	if err := os.WriteFile(indexPath, buf.Bytes(), 0644); err != nil {
		return err
	}
	
	// Copiar arquivos estáticos (CSS/JS)
	staticDir := filepath.Join(dir, "static")
	if err := os.MkdirAll(staticDir, 0755); err != nil {
		return err
	}
	
	// Copiar styles.css
	stylesData, _ := fs.ReadFile(GetStaticFS(), "styles.css")
	os.WriteFile(filepath.Join(staticDir, "styles.css"), stylesData, 0644)
	
	// Copiar scripts.js
	scriptsData, _ := fs.ReadFile(GetStaticFS(), "scripts.js")
	os.WriteFile(filepath.Join(staticDir, "scripts.js"), scriptsData, 0644)
	
	// Escrever backlog.json
	backlogPath := filepath.Join(dir, "backlog.json")
	jsData := buildExportJSON(e.data)

	backlogJSON, err := json.MarshalIndent(jsData, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(backlogPath, backlogJSON, 0644); err != nil {
		return err
	}

	return nil
}

// buildExportJSON constrói JSON completo para export (usado por Exporter e StaticExporter)
func buildExportJSON(data *LensData) map[string]interface{} {
	jsData := map[string]interface{}{
		"meta": map[string]interface{}{
			"title":       data.Meta.Title,
			"subtitle":    data.Meta.Subtitle,
			"lang":        data.Meta.Lang,
			"kpis":        data.Meta.KPIs,
			"totalEpics":  data.Meta.TotalEpics,
			"totalStories": data.Meta.TotalStories,
		},
		"epics":      make(map[string]interface{}),
		"deep_dives": make(map[string]interface{}),
	}

	// Converter épicos
	epicsMap := jsData["epics"].(map[string]interface{})
	for epicId, epic := range data.Epics {
		stories := []map[string]interface{}{}
		for _, story := range epic.Stories {
			stories = append(stories, map[string]interface{}{
				"code":                story.Code,
				"title":               story.Title,
				"what":                story.What,
				"why":                 story.Why,
				"acceptance_criteria": story.AcceptanceCriteria,
				"effort":              story.Effort,
			})
		}

		epicsMap[epicId] = map[string]interface{}{
			"id":          epic.ID,
			"title":       epic.Title,
			"description": epic.Description,
			"summary":     epic.Summary,
			"priority":    epic.Priority,
			"stories":     stories,
		}
	}

	// Converter deep dives (com classification/scope/story_id)
	deepDivesMap := jsData["deep_dives"].(map[string]interface{})
	for key, dd := range data.DeepDives {
		ddEntry := map[string]interface{}{
			"term":          dd.Term,
			"what_is":       dd.WhatIs,
			"why_here":      dd.WhyHere,
			"configuration": dd.Configuration,
			"patterns":      dd.Patterns,
			"decisions":     dd.Decisions,
		}
		if dd.Classification != "" {
			ddEntry["classification"] = dd.Classification
		}
		if dd.Scope != "" {
			ddEntry["scope"] = dd.Scope
		}
		if dd.StoryID != "" {
			ddEntry["story_id"] = dd.StoryID
		}
		deepDivesMap[key] = ddEntry
	}

	// Effort
	if data.Effort.TotalSPs > 0 {
		jsData["effort"] = data.Effort
	}

	// Milestones
	if len(data.Milestones) > 0 {
		jsData["milestones"] = data.Milestones
	}

	// Metrics
	if data.Metrics != nil {
		jsData["metrics"] = data.Metrics
	}

	return jsData
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// STATIC EXPORTER - GitHub Pages / Offline
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// StaticExporter exporta Lens como arquivo único self-contained
type StaticExporter struct {
	data *LensData
}

// NewStaticExporter cria novo static exporter
func NewStaticExporter(data *LensData) *StaticExporter {
	return &StaticExporter{data: data}
}

// Export exporta para arquivo HTML único com dados inline
func (se *StaticExporter) Export(dir string, title ...string) error {
	// Criar diretório
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório: %w", err)
	}
	
	// Gerar HTML self-contained
	html, err := se.generateStaticHTML(title...)
	if err != nil {
		return fmt.Errorf("erro ao gerar HTML: %w", err)
	}
	
	// Salvar index.html
	indexPath := filepath.Join(dir, "index.html")
	if err := os.WriteFile(indexPath, []byte(html), 0644); err != nil {
		return fmt.Errorf("erro ao salvar index.html: %w", err)
	}
	
	// Gerar README.md
	readme := se.generateREADME(title...)
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte(readme), 0644); err != nil {
		return fmt.Errorf("erro ao salvar README.md: %w", err)
	}
	
	return nil
}

// generateStaticHTML gera HTML completo com dados inline
func (se *StaticExporter) generateStaticHTML(title ...string) (string, error) {
	projectTitle := se.data.Meta.Title
	if len(title) > 0 && title[0] != "" {
		projectTitle = title[0]
	}
	
	// Serializar dados como JSON (usa função compartilhada)
	jsData := buildExportJSON(se.data)

	// Marshal para JSON
	jsonBytes, err := json.Marshal(jsData)
	if err != nil {
		return "", fmt.Errorf("erro ao serializar dados: %w", err)
	}
	
	// Renderizar template base
	var buf bytes.Buffer
	if err := GetIndexTemplate().Execute(&buf, se.data); err != nil {
		return "", fmt.Errorf("erro ao renderizar template: %w", err)
	}
	html := buf.String()
	
	// Modificar título
	html = replaceFirst(html, "<title>Workflow Lens</title>", 
		fmt.Sprintf("<title>%s - Workflow Lens</title>", projectTitle))
	
	// Adicionar dados inline antes de </body>
	dataScript := fmt.Sprintf(`
    <!-- ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ -->
    <!-- STATIC EXPORT DATA (Workflow v3.0) -->
    <!-- ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ -->
    <script>
        // Dados do backlog embedded (modo estático)
        window.BACKLOG_DATA = %s;
        
        // Flag de modo estático
        window.STATIC_MODE = true;
        
        // Info de geração
        console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━');
        console.log('📦 Workflow Lens v3.0 - Static Export');
        console.log('   Self-contained HTML (offline-capable)');
        console.log('   Data size: ' + (JSON.stringify(window.BACKLOG_DATA).length / 1024).toFixed(1) + ' KB');
        console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━');
    </script>
    
</body>`, string(jsonBytes))
	
	html = replaceFirst(html, "</body>", dataScript)
	
	return html, nil
}


// generateREADME gera README.md com instruções
func (se *StaticExporter) generateREADME(title ...string) string {
	projectTitle := se.data.Meta.Title
	if len(title) > 0 && title[0] != "" {
		projectTitle = title[0]
	}
	
	return fmt.Sprintf(`# 📊 %s - Backlog Visualization

Visualização estática gerada por **[Workflow v3.0](https://github.com/Cobliteam/workflow-toolkit)** - AI-Powered Backlog Generator

---

## 🌐 Ver Online

**GitHub Pages:** https://[user].github.io/[repo]

_(Substitua [user] e [repo] pelos seus valores)_

---

## 🚀 Como Usar

### Opção 1: Abrir Localmente (Offline) ✨

**Mais simples - funciona sem internet:**

` + "```bash" + `
# Duplo-clique no arquivo
open index.html

# Ou abra no seu browser favorito
firefox index.html
chrome index.html
edge index.html
` + "```" + `

✅ **Funciona 100%% offline** (` + "`file://`" + `)

---

### Opção 2: GitHub Pages (Recomendado) 🚀

**Hospedagem grátis e pública:**

**Passo 1:** Configure o GitHub Pages

1. Vá em: **Settings** → **Pages**
2. Source: **Deploy from a branch**
3. Branch: **main** → Escolha a pasta:
   - **` + "`/ (root)`" + `** - Se ` + "`index.html`" + ` está na raiz
   - **` + "`/docs`" + `** - Se está em ` + "`docs/`" + `
   - **` + "`/dist`" + `** - Se está em ` + "`dist/`" + `
4. Clique em **Save**

**Passo 2:** Aguarde ~1 minuto

GitHub vai buildar e publicar automaticamente.

**Passo 3:** Acesse

URL será: ` + "`https://[user].github.io/[repo]`" + `

💡 GitHub mostra o link exato depois de configurar!

---

### Opção 3: Python Server (Desenvolvimento) 🐍

**Para testar localmente com servidor:**

` + "```bash" + `
# Na pasta que contém index.html
python -m http.server 8000

# Ou com Python 2
python -m SimpleHTTPServer 8000

# Abra no browser:
# http://localhost:8000
` + "```" + `

---

## 📁 Arquivos

| Arquivo | Tamanho Aprox. | Descrição |
|---------|----------------|-----------|
| **index.html** | ~100-300KB | Visualização completa **(self-contained)** |
| **README.md** | ~2KB | Este arquivo |

**Características:**
- ✅ **Self-contained**: Todo HTML + CSS + JavaScript + Dados em um arquivo
- ✅ **Zero dependências**: Não precisa npm, node, nada!
- ✅ **Portável**: Copie e funciona anywhere

---

## ✨ Features

- ✅ **Funciona offline** - Abre direto em ` + "`file://`" + `
- ✅ **Zero dependências** - Vanilla JS, sem build tools
- ✅ **GitHub Pages ready** - Hospedagem grátis e ilimitada
- ✅ **Compartilhável** - Email, USB, Dropbox, qualquer lugar
- ✅ **Versionável** - Git history = evolução do seu backlog
- ✅ **Responsivo** - Funciona em mobile, tablet, desktop
- ✅ **Rápido** - Carrega em <100ms (tudo local)

---

## 🔄 Atualizar

Para atualizar após mudanças no backlog:

` + "```bash" + `
# 1. Re-gerar backlog (se necessário)
wtb run backlog project-definition.md

# 2. Re-exportar Lens
wtb lens export --output ./dist

# 3. Commit
git add dist/
git commit -m "docs: atualizar backlog visualization"
git push

# 4. GitHub Pages atualiza automaticamente em ~1 minuto ✨
` + "```" + `

---

## 💡 Compartilhar

### 📧 Por Email
Anexe ` + "`index.html`" + ` - destinatário abre direto no browser.

### 🔗 Por Link
- **GitHub Pages**: ` + "`https://[user].github.io/[repo]`" + `
- **Dropbox**: Upload + compartilhe link público
- **Google Drive**: Upload + "Anyone with link can view"

### 💾 Por USB
Copie ` + "`index.html`" + ` para USB - funciona em qualquer computador.

### 💬 Por Slack/Teams
Faça upload do ` + "`index.html`" + ` - colegas baixam e abrem.

---

## 🔍 Troubleshooting

### Não carrega os dados?

**Verifique o console do browser** (F12):
- ✅ Deve mostrar: ` + "`📦 Workflow Lens - Static Mode`" + `
- ❌ Se mostrar erro, arquivo pode estar corrompido

**Soluções:**
1. Re-exporte: ` + "`wtb lens export --output ./dist`" + `
2. Limpe cache do browser (Ctrl+Shift+R)
3. Abra em outro browser

### GitHub Pages não funciona?

**Checklist:**
- [ ] Arquivo está em ` + "`/`" + `, ` + "`/docs`" + ` ou ` + "`/dist`" + ` conforme configurado
- [ ] Settings → Pages está configurado
- [ ] Aguardou 1-2 minutos após push
- [ ] URL está correto: ` + "`https://[user].github.io/[repo]`" + `

### Arquivo muito grande?

Se ` + "`index.html`" + ` > 1MB:
- GitHub Pages serve até ~1MB sem problemas
- Gzip reduz para ~30%% do tamanho original
- Considere reduzir número de histórias

---

## 🤝 Contribuindo

Encontrou um bug? Tem sugestão?

1. Abra issue no repositório do Workflow
2. Ou envie PR com melhorias

---

## 📝 Gerado por

**Workflow v3.0 (Compliance Edition)**  
AI-Powered Backlog Generator with LGPD/GDPR Compliance

🌐 **Repositório**: https://github.com/Cobliteam/workflow-toolkit  
📚 **Documentação**: README.md no repositório  
⚖️ **Compliance**: LEGAL.md, SECURITY.md, COMPLIANCE.md

---

## 📄 Licença

Este backlog visualization foi gerado por Workflow (MIT License).  
O conteúdo do backlog pertence ao projeto **%s**.

---

**Última geração:** ` + "`" + `wtb lens export` + "`" + ` executado em [timestamp]
`, projectTitle, projectTitle)
}

// serveStatic serve arquivos CSS/JS
func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	// Remover /static/ do path
	path := strings.TrimPrefix(r.URL.Path, "/static/")
	
	// Ler arquivo do embed FS
	data, err := fs.ReadFile(GetStaticFS(), path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	
	// Definir Content-Type
	if strings.HasSuffix(path, ".css") {
		w.Header().Set("Content-Type", "text/css")
	} else if strings.HasSuffix(path, ".js") {
		w.Header().Set("Content-Type", "application/javascript")
	}
	
	w.Write(data)
}
