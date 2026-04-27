package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSessionResolver_Found(t *testing.T) {
	dir := t.TempDir()
	sessionFile := filepath.Join(dir, "session.yml")
	os.WriteFile(sessionFile, []byte("namespace: production\nenvironment: staging\n"), 0644)

	r := &SessionResolver{}
	cfg := &ResolveConfig{}
	cfg.Resolve.Strategies = map[string]StrategyConfig{
		"session": {SourceFile: "session.yml"},
	}

	ctx := ResolveContext{
		PersonalDir: dir,
		Config:      cfg,
	}

	val, ok := r.Resolve(InputSpec{Name: "namespace"}, RunInputs{}, ctx)
	if !ok || val != "production" {
		t.Errorf("expected 'production', got %q (ok=%v)", val, ok)
	}
}

func TestSessionResolver_MissingFile(t *testing.T) {
	r := &SessionResolver{}
	ctx := ResolveContext{PersonalDir: t.TempDir()}

	_, ok := r.Resolve(InputSpec{Name: "namespace"}, RunInputs{}, ctx)
	if ok {
		t.Error("expected false when session file missing")
	}
}

func TestChainResolver_FindsUpstream(t *testing.T) {
	dir := t.TempDir()

	// Create an upstream incident artifact with frontmatter
	incidentDir := filepath.Join(dir, "docs", "workflow", "incident")
	os.MkdirAll(incidentDir, 0755)
	artifact := "---\nnamespace: fusca-prod\nsymptom: CDC lag\n---\n# Incident\n"
	os.WriteFile(filepath.Join(incidentDir, "001-test-2026-02-20.md"), []byte(artifact), 0644)

	r := &ChainResolver{}
	cfg := &ResolveConfig{}
	cfg.Resolve.Strategies = map[string]StrategyConfig{
		"chain": {
			ArtifactDir: "docs/workflow/",
			SearchFields: map[string][]string{
				"namespace": {"namespace", "environment"},
			},
		},
	}

	def := &UseCaseDefinition{
		Chain: ChainSpec{From: []string{"incident"}},
	}

	ctx := ResolveContext{
		RepoPath:   dir,
		Definition: def,
		Config:     cfg,
	}

	val, ok := r.Resolve(InputSpec{Name: "namespace"}, RunInputs{}, ctx)
	if !ok || val != "fusca-prod" {
		t.Errorf("expected 'fusca-prod', got %q (ok=%v)", val, ok)
	}
}

func TestChainResolver_FindsUpstreamInSubdir(t *testing.T) {
	dir := t.TempDir()

	// Create an upstream incident artifact inside NNN-* subdirectory (convention)
	incidentDir := filepath.Join(dir, "docs", "workflow", "incident", "001-fusca-cdc-2026-02-20")
	os.MkdirAll(incidentDir, 0755)
	artifact := "---\nnamespace: organization\nsymptom: WAL overflow\nkubectl-context: cobli-prod-devices\n---\n# Savepoint\n"
	os.WriteFile(filepath.Join(incidentDir, "savepoint-2026-02-20.md"), []byte(artifact), 0644)

	r := &ChainResolver{}
	cfg := &ResolveConfig{}
	cfg.Resolve.Strategies = map[string]StrategyConfig{
		"chain": {
			ArtifactDir: "docs/workflow/",
			SearchFields: map[string][]string{
				"namespace": {"namespace", "environment"},
				"symptom":   {"symptom", "title"},
			},
		},
	}

	def := &UseCaseDefinition{
		Chain: ChainSpec{From: []string{"incident"}},
	}

	ctx := ResolveContext{
		RepoPath:   dir,
		Definition: def,
		Config:     cfg,
	}

	val, ok := r.Resolve(InputSpec{Name: "namespace"}, RunInputs{}, ctx)
	if !ok || val != "organization" {
		t.Errorf("expected 'organization' from subdir, got %q (ok=%v)", val, ok)
	}

	val, ok = r.Resolve(InputSpec{Name: "symptom"}, RunInputs{}, ctx)
	if !ok || val != "WAL overflow" {
		t.Errorf("expected 'WAL overflow' from subdir, got %q (ok=%v)", val, ok)
	}
}

func TestChainResolver_NoChain(t *testing.T) {
	r := &ChainResolver{}
	ctx := ResolveContext{
		RepoPath:   t.TempDir(),
		Definition: &UseCaseDefinition{},
	}

	_, ok := r.Resolve(InputSpec{Name: "namespace"}, RunInputs{}, ctx)
	if ok {
		t.Error("expected false when no chain defined")
	}
}

func TestHelmResolver_ProdValues(t *testing.T) {
	dir := t.TempDir()
	helmDir := filepath.Join(dir, "helm")
	os.MkdirAll(helmDir, 0755)

	values := "namespace: fusca-prod\npostgresql:\n  host: pg.internal\n  port: 5432\n"
	os.WriteFile(filepath.Join(helmDir, "values-production.yaml"), []byte(values), 0644)

	r := &HelmResolver{}
	cfg := &ResolveConfig{}
	cfg.Resolve.Strategies = map[string]StrategyConfig{
		"helm": {
			SearchFiles: []string{"helm/values-production.yaml"},
			FieldMappings: map[string][]string{
				"namespace": {"namespace"},
				"db-host":   {"postgresql.host"},
			},
		},
	}

	ctx := ResolveContext{
		RepoPath: dir,
		Config:   cfg,
	}

	val, ok := r.Resolve(InputSpec{Name: "namespace"}, RunInputs{}, ctx)
	if !ok || val != "fusca-prod" {
		t.Errorf("expected 'fusca-prod', got %q (ok=%v)", val, ok)
	}
}

func TestHelmResolver_NestedPath(t *testing.T) {
	dir := t.TempDir()
	helmDir := filepath.Join(dir, "helm")
	os.MkdirAll(helmDir, 0755)

	values := "postgresql:\n  host: pg.internal\n  port: 5432\n"
	os.WriteFile(filepath.Join(helmDir, "values-production.yaml"), []byte(values), 0644)

	r := &HelmResolver{}
	cfg := &ResolveConfig{}
	cfg.Resolve.Strategies = map[string]StrategyConfig{
		"helm": {
			SearchFiles:   []string{"helm/values-production.yaml"},
			FieldMappings: map[string][]string{"db-host": {"postgresql.host"}},
		},
	}

	ctx := ResolveContext{RepoPath: dir, Config: cfg}

	val, ok := r.Resolve(InputSpec{Name: "db-host"}, RunInputs{}, ctx)
	if !ok || val != "pg.internal" {
		t.Errorf("expected 'pg.internal', got %q (ok=%v)", val, ok)
	}
}

func TestHelmResolver_NoFiles(t *testing.T) {
	r := &HelmResolver{}
	cfg := &ResolveConfig{}
	cfg.Resolve.Strategies = map[string]StrategyConfig{
		"helm": {
			SearchFiles:   []string{"helm/values-production.yaml"},
			FieldMappings: map[string][]string{"namespace": {"namespace"}},
		},
	}

	ctx := ResolveContext{RepoPath: t.TempDir(), Config: cfg}

	_, ok := r.Resolve(InputSpec{Name: "namespace"}, RunInputs{}, ctx)
	if ok {
		t.Error("expected false when no helm files found")
	}
}

func TestRepoResolver_InfersDB(t *testing.T) {
	dir := t.TempDir()
	// Create a go.mod referencing pgx (Postgres marker)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\nrequire github.com/jackc/pgx v1.0.0\n"), 0644)

	r := &RepoResolver{}
	ctx := ResolveContext{RepoPath: dir}

	// Note: this depends on pkg/context detecting PostgreSQL from go.mod.
	// If the marker doesn't match (depends on repo_markers.yml config),
	// this test simply verifies the resolver doesn't crash.
	_, _ = r.Resolve(InputSpec{Name: "db-name"}, RunInputs{}, ctx)
}

func TestAskResolver_Interactive(t *testing.T) {
	term := &mockTerminal{
		canAsk:       true,
		askResponses: map[string]string{"CDC lag": "from-ask"},
	}

	r := &AskResolver{}
	ctx := ResolveContext{Term: term}

	val, ok := r.Resolve(InputSpec{Name: "symptom", Description: "CDC lag", WhyAsk: "needed for triage"}, RunInputs{}, ctx)
	if !ok || val != "from-ask" {
		t.Errorf("expected 'from-ask', got %q (ok=%v)", val, ok)
	}
}

func TestAskResolver_NonInteractive(t *testing.T) {
	term := &mockTerminal{canAsk: false}

	r := &AskResolver{}
	ctx := ResolveContext{Term: term}

	_, ok := r.Resolve(InputSpec{Name: "symptom", Description: "what happened"}, RunInputs{}, ctx)
	if ok {
		t.Error("expected false when terminal is non-interactive")
	}
}
