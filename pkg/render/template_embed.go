package render

import (
	"embed"
	"html/template"
	"io/fs"
)

//go:embed templates/*
var templatesFS embed.FS

// Templates HTML compilados
var (
	indexTemplate *template.Template
	staticFS      fs.FS
)

func init() {
	// Carregar template HTML
	var err error
	indexTemplate, err = template.ParseFS(templatesFS, "templates/index.html")
	if err != nil {
		panic("Erro ao carregar template: " + err.Error())
	}

	// Preparar FS para arquivos estáticos (CSS/JS)
	staticFS, err = fs.Sub(templatesFS, "templates")
	if err != nil {
		panic("Erro ao criar sub-filesystem: " + err.Error())
	}
}

// GetIndexTemplate retorna o template HTML principal
func GetIndexTemplate() *template.Template {
	return indexTemplate
}

// GetStaticFS retorna o filesystem com CSS/JS
func GetStaticFS() fs.FS {
	return staticFS
}
