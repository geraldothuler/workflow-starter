package render

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// FIXTURES COMPARTILHADOS: usados por integration e browser tests
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// testLensData cria LensData representativo para testes de integracao e screenshots.
// Contem: 5 epicos, 8 stories, 5 deep dives, KPIs, effort, milestones, metrics.
// Dados suficientes para preencher todas as secoes do Lens com conteudo visual rico.
func testLensData() *LensData {
	return &LensData{
		Meta: MetaData{
			Title:        "Workflow SaaS Platform",
			Subtitle:     "Backend + Auth + Observability + Deploy",
			Lang:         "pt-BR",
			TotalEpics:   5,
			TotalStories: 8,
			KPIs: []KPI{
				{Label: "Épicos", Value: "5"},
				{Label: "Histórias", Value: "8"},
				{Label: "Story Points", Value: "34"},
				{Label: "Deep Dives", Value: "5"},
			},
		},
		Epics: map[string]EpicLens{
			"E1": {
				ID: "E1", Code: "E1", Title: "Backend API", Priority: "high",
				Summary:     "API REST completa com autenticação e CRUD de recursos principais",
				Description: "Desenvolvimento da API REST com Go, incluindo endpoints de usuarios e produtos",
				Stories: []StoryLens{
					{
						ID: "E1.1", Code: "E1.1", Title: "Setup projeto Go",
						What: "Criar estrutura base do projeto", Why: "Fundacao para todo o backend",
						Effort: 3, AcceptanceCriteria: []string{"Go module criado", "CI configurado", "Linting habilitado"},
						Tags: []interface{}{"Go", "setup"},
					},
					{
						ID: "E1.2", Code: "E1.2", Title: "CRUD usuarios",
						What: "Implementar CRUD completo", Why: "Funcionalidade core do sistema",
						Effort: 5, AcceptanceCriteria: []string{"GET /users", "POST /users", "PUT /users/:id", "DELETE /users/:id"},
						Tags: []interface{}{"Go", "PostgreSQL"},
					},
				},
			},
			"E2": {
				ID: "E2", Code: "E2", Title: "Autenticacao e Seguranca", Priority: "high",
				Summary:     "Sistema de autenticacao JWT com refresh tokens e controle de acesso",
				Description: "Autenticacao stateless com JWT tokens e middleware de autorizacao",
				Stories: []StoryLens{
					{
						ID: "E2.1", Code: "E2.1", Title: "Login JWT",
						What: "Implementar login com JWT", Why: "Seguranca do sistema",
						Effort: 5, AcceptanceCriteria: []string{"POST /login retorna JWT", "Token valido por 15min", "Refresh token rotation"},
						Tags: []interface{}{"JWT", "security"},
					},
					{
						ID: "E2.2", Code: "E2.2", Title: "Middleware de autorizacao",
						What: "Middleware para proteger rotas", Why: "Controle de acesso granular",
						Effort: 3, AcceptanceCriteria: []string{"Rotas protegidas por role", "401 para tokens invalidos"},
						Tags: []interface{}{"JWT", "Go"},
					},
				},
			},
			"E3": {
				ID: "E3", Code: "E3", Title: "Infraestrutura e Deploy", Priority: "medium",
				Summary:     "Pipeline CI/CD automatizado com Docker e deploy em producao",
				Description: "Containerizacao com Docker e pipeline automatizado de deploy",
				Stories: []StoryLens{
					{
						ID: "E3.1", Code: "E3.1", Title: "Pipeline CI/CD",
						What: "Configurar pipeline automatizado", Why: "Deploy confiavel e rapido",
						Effort: 3, AcceptanceCriteria: []string{"Build automatico no push", "Testes rodam no CI", "Deploy automatico em staging"},
						Tags: []interface{}{"Docker", "CI/CD"},
					},
					{
						ID: "E3.2", Code: "E3.2", Title: "Containerizacao Docker",
						What: "Criar Dockerfiles otimizados", Why: "Ambiente consistente em todos os stages",
						Effort: 5, AcceptanceCriteria: []string{"Multi-stage build", "Imagem < 50MB", "Docker Compose para dev"},
						Tags: []interface{}{"Docker", "Go"},
					},
				},
			},
			"E4": {
				ID: "E4", Code: "E4", Title: "Observabilidade", Priority: "medium",
				Summary:     "Monitoramento completo com metricas, logs estruturados e alertas",
				Description: "Stack de observabilidade com Prometheus, Grafana e logging estruturado",
				Stories: []StoryLens{
					{
						ID: "E4.1", Code: "E4.1", Title: "Metricas Prometheus",
						What: "Expor metricas da aplicacao", Why: "Visibilidade da saude do sistema",
						Effort: 3, AcceptanceCriteria: []string{"Endpoint /metrics", "Latencia por endpoint", "Contadores de erro"},
						Tags: []interface{}{"Prometheus", "Go"},
					},
				},
			},
			"E5": {
				ID: "E5", Code: "E5", Title: "Documentacao API", Priority: "low",
				Summary:     "Documentacao interativa da API com OpenAPI e exemplos de uso",
				Description: "Swagger UI gerado automaticamente a partir de anotacoes no codigo",
				Stories: []StoryLens{
					{
						ID: "E5.1", Code: "E5.1", Title: "OpenAPI Swagger",
						What: "Gerar docs da API automaticamente", Why: "Facilitar integracao de clientes",
						Effort: 2, AcceptanceCriteria: []string{"Swagger UI acessivel em /docs", "Todos endpoints documentados"},
						Tags: []interface{}{"OpenAPI", "Go"},
					},
				},
			},
		},
		Stories: map[string]StoryLens{
			"E1.1": {ID: "E1.1", Code: "E1.1", Title: "Setup projeto Go", Effort: 3},
			"E1.2": {ID: "E1.2", Code: "E1.2", Title: "CRUD usuarios", Effort: 5},
			"E2.1": {ID: "E2.1", Code: "E2.1", Title: "Login JWT", Effort: 5},
			"E2.2": {ID: "E2.2", Code: "E2.2", Title: "Middleware de autorizacao", Effort: 3},
			"E3.1": {ID: "E3.1", Code: "E3.1", Title: "Pipeline CI/CD", Effort: 3},
			"E3.2": {ID: "E3.2", Code: "E3.2", Title: "Containerizacao Docker", Effort: 5},
			"E4.1": {ID: "E4.1", Code: "E4.1", Title: "Metricas Prometheus", Effort: 3},
			"E5.1": {ID: "E5.1", Code: "E5.1", Title: "OpenAPI Swagger", Effort: 2},
		},
		DeepDives: map[string]DeepDiveLens{
			"Go": {
				ID: "dd-go", Term: "Go",
				WhatIs:         "Linguagem compilada com concorrência nativa e garbage collector eficiente",
				WhyHere:        "Escolhida para o backend por performance e simplicidade no deploy",
				Configuration:  "Go 1.24+, módulos habilitados, linting com golangci-lint",
				Patterns:       []string{"Clean Architecture", "Repository Pattern", "Dependency Injection via interfaces"},
				AntiPatterns:   []string{"God functions", "Shared mutable state sem mutex"},
				Decisions:      []string{"Go vs Node.js (performance)", "Stdlib HTTP vs framework (chi, gin)"},
				RelatedTerms:   []string{"goroutine", "channel", "interface"},
				Classification: "critical",
				Scope:          "epic",
			},
			"PostgreSQL": {
				ID: "dd-pg", Term: "PostgreSQL",
				WhatIs:         "Banco relacional open-source com suporte a JSON, full-text search e extensões",
				WhyHere:        "Armazena dados de usuários, sessões e logs de auditoria",
				Configuration:  "PostgreSQL 16, connection pooling via pgbouncer, migrations com golang-migrate",
				Patterns:       []string{"Prepared statements", "Connection pooling", "Database migrations"},
				AntiPatterns:   []string{"N+1 queries", "Missing indexes em colunas filtradas"},
				Decisions:      []string{"PostgreSQL vs MySQL (JSON support)", "ORM vs SQL puro (sqlx)"},
				RelatedTerms:   []string{"SQL", "pgx", "sqlx"},
				Classification: "standard",
				Scope:          "epic",
			},
			"JWT": {
				ID: "dd-jwt", Term: "JWT",
				WhatIs:         "JSON Web Token para autenticação stateless entre cliente e servidor",
				WhyHere:        "Necessário para login seguro sem sessão server-side",
				Configuration:  "RS256 com refresh tokens, expiry 15min access / 7d refresh",
				Patterns:       []string{"Bearer token no header Authorization", "Refresh token rotation"},
				AntiPatterns:   []string{"Armazenar JWT em localStorage", "Tokens sem expiração"},
				Decisions:      []string{"RS256 vs HS256 (segurança)", "Token expiry: 15min vs 1h"},
				RelatedTerms:   []string{"OAuth2", "Bearer", "refresh token"},
				Classification: "specific",
				Scope:          "story",
			},
			"Docker": {
				ID: "dd-docker", Term: "Docker",
				WhatIs:         "Plataforma de containerização que empacota aplicações com suas dependências",
				WhyHere:        "Garante ambiente consistente do dev ao production, simplifica deploy",
				Configuration:  "Multi-stage builds, Alpine base, Docker Compose para ambiente local",
				Patterns:       []string{"Multi-stage builds", "Layer caching", ".dockerignore otimizado"},
				AntiPatterns:   []string{"Rodar como root", "Imagens baseadas em Ubuntu sem necessidade"},
				Decisions:      []string{"Alpine vs Distroless (tamanho vs debug)", "Compose vs K8s (escala)"},
				RelatedTerms:   []string{"container", "Dockerfile", "registry"},
				Classification: "standard",
				Scope:          "epic",
			},
			"Prometheus": {
				ID: "dd-prom", Term: "Prometheus",
				WhatIs:         "Sistema de monitoramento e alertas com modelo de dados dimensional",
				WhyHere:        "Coleta métricas de performance e saúde da API para dashboards e alertas",
				Configuration:  "Prometheus 2.x, scrape interval 15s, retention 30d, Grafana para visualização",
				Patterns:       []string{"RED metrics (Rate, Errors, Duration)", "Labels para dimensões", "Histogramas para latência"},
				AntiPatterns:   []string{"High cardinality labels", "Métricas sem significado de negócio"},
				Decisions:      []string{"Prometheus vs Datadog (custo)", "Pull vs Push model (simplicidade)"},
				RelatedTerms:   []string{"Grafana", "PromQL", "alertmanager"},
				Classification: "specific",
				Scope:          "story",
			},
		},
		Effort: EffortSummary{
			TotalStories: 8,
			TotalSPs:     34,
			TotalDays:    17,
			ByEpic: map[string]EpicEffort{
				"E1": {EpicID: "E1", Stories: 2, SPs: 8, Days: 4, Percentage: 23.5},
				"E2": {EpicID: "E2", Stories: 2, SPs: 8, Days: 4, Percentage: 23.5},
				"E3": {EpicID: "E3", Stories: 2, SPs: 8, Days: 4, Percentage: 23.5},
				"E4": {EpicID: "E4", Stories: 1, SPs: 3, Days: 2, Percentage: 8.8},
				"E5": {EpicID: "E5", Stories: 1, SPs: 2, Days: 1, Percentage: 5.9},
			},
		},
		Milestones: []Milestone{
			{
				ID: "M1", Title: "MVP Backend",
				Description: "API funcional com CRUD e autenticacao", EpicIDs: []string{"E1", "E2"},
				TotalSPs: 16, DaysEstimate: 8,
			},
			{
				ID: "M2", Title: "Infra e Deploy",
				Description: "Pipeline CI/CD e containerizacao completa", EpicIDs: []string{"E3"},
				TotalSPs: 8, DaysEstimate: 4,
			},
			{
				ID: "M3", Title: "Observabilidade",
				Description: "Monitoramento com metricas e alertas em producao", EpicIDs: []string{"E4"},
				TotalSPs: 3, DaysEstimate: 2,
			},
			{
				ID: "M4", Title: "Release v1.0",
				Description: "Documentacao completa e primeira versao publica", EpicIDs: []string{"E5"},
				TotalSPs: 2, DaysEstimate: 1,
			},
		},
		Metrics: &GenerationMetricsLens{
			TotalTechsExtracted: 14,
			TrivialFiltered:     5,
			ClassificationStats: map[string]int{"trivial": 5, "standard": 4, "specific": 3, "critical": 2},
			LLMCallsMade:        8,
			LLMCallsSaved:       12,
			ReductionPercent:     60.0,
			TotalInputTokens:    4500,
			TotalOutputTokens:   1800,
			TotalCost:           0.05,
		},
	}
}
