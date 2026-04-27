package repoindex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FlutterDartParser handles Flutter/Dart mobile apps.
// Supports two common layouts:
//
//   - flat:   lib/<feature>/bloc/        (bolovo-double-fried style)
//   - nested: lib/app/modules/<feature>/ (herbie-toy style)
type FlutterDartParser struct{}

func (p *FlutterDartParser) Lang() string      { return "dart" }
func (p *FlutterDartParser) Framework() string { return "flutter" }

// Layers returns file groups for LLM indexing.
//
//   - infra:          pubspec.yaml, catalog-info.yaml, Makefile, firebase.json
//   - api:            HTTP clients + repositories (flat and nested layouts)
//   - feat-<module>:  one layer per feature module — bloc + event + state files only.
//                     Keeps each LLM call small (~2-5 files) to avoid timeouts.
func (p *FlutterDartParser) Layers(repoPath string) ([]Layer, error) {
	var layers []Layer

	// Layer 1: infra
	infra := Layer{Name: "infra"}
	for _, rel := range []string{
		"pubspec.yaml",
		"catalog-info.yaml",
		"Makefile",
		"firebase.json",
	} {
		if fileExists(repoPath, rel) {
			infra.Files = append(infra.Files, filepath.Join(repoPath, rel))
		}
	}
	if len(infra.Files) > 0 {
		layers = append(layers, infra)
	}

	// Layer 2: api — network + repository directories (flat and nested layouts)
	api := Layer{Name: "api"}
	for _, dir := range []string{"lib/network", "lib/repository", "lib/app/repositories"} {
		dirPath := filepath.Join(repoPath, filepath.FromSlash(dir))
		if fi, err := os.Stat(dirPath); err == nil && fi.IsDir() {
			filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				if strings.HasSuffix(path, ".dart") || strings.HasSuffix(path, ".graphql") {
					api.Files = append(api.Files, path)
				}
				return nil
			})
		}
	}
	if len(api.Files) > 0 {
		layers = append(layers, api)
	}

	// Layers 3+: one layer per feature module.
	// Each layer contains the bloc, event, and state files for that module only (~2-5 files).
	// This avoids timeout from bundling all features into a single large LLM call.
	for _, layer := range p.featureLayers(repoPath) {
		layers = append(layers, layer)
	}

	return layers, nil
}

// featureLayers discovers feature modules and returns one Layer per module.
// A module qualifies when it contains at least one *_bloc.dart file.
// Each layer includes: *_bloc.dart, *_event.dart, *_state.dart for that module.
func (p *FlutterDartParser) featureLayers(repoPath string) []Layer {
	skipDirs := map[string]bool{
		"__generated": true, "base_components": true, "utils": true,
		"extension": true, "theme": true, "common_types": true,
		"code_gen": true, "network": true, "repository": true,
		"repositories": true, "formatter": true, "models": true,
		"enums": true, "helpers": true, "exceptions": true, "services": true,
	}

	// featureBases covers both flat (bolovo) and nested (herbie-toy) layouts.
	featureBases := []string{"lib", "lib/app", "lib/app/modules", "lib/app/core"}

	seenModules := map[string]bool{} // prevent duplicates across bases
	var result []Layer

	for _, base := range featureBases {
		basePath := filepath.Join(repoPath, filepath.FromSlash(base))
		entries, err := os.ReadDir(basePath)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || skipDirs[e.Name()] {
				continue
			}
			modName := e.Name()
			modDir := filepath.Join(basePath, modName)

			// Collect bloc/event/state files recursively within this module dir.
			var files []string
			filepath.Walk(modDir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				if strings.HasSuffix(path, "_bloc.dart") ||
					strings.HasSuffix(path, "_event.dart") ||
					strings.HasSuffix(path, "_state.dart") ||
					strings.HasSuffix(path, "_cubit.dart") {
					files = append(files, path)
				}
				return nil
			})

			// Only emit a layer if this module has at least one BLoC/Cubit.
			hasBloc := false
			for _, f := range files {
				if strings.HasSuffix(f, "_bloc.dart") || strings.HasSuffix(f, "_cubit.dart") {
					hasBloc = true
					break
				}
			}
			if !hasBloc {
				continue
			}

			// Deduplicate across featureBases using the file set as key.
			key := modName
			if seenModules[key] {
				continue
			}
			seenModules[key] = true

			result = append(result, Layer{
				Name:  "feat-" + modName,
				Files: files,
			})
		}
	}
	return result
}

func (p *FlutterDartParser) SystemPrompt() string {
	return `You are a senior Flutter/Dart architect.
You extract structured metadata from source files to populate a code intelligence database.
Repos are Flutter mobile apps using BLoC pattern, GraphQL (graphql_flutter), REST HTTP clients, and Firebase.
Always respond with a single valid JSON object matching the ExtractedLayer schema.
Never include markdown fences, explanations, or extra text — only the JSON object.`
}

func (p *FlutterDartParser) LayerPrompt(layerName, content string) string {
	schema := `{
  "handlers": [{"name":"","handler_file":"","trigger_type":"","trigger_detail":"","timeout":0,"max_retry":0,"concurrency":0,"vpc":false,"description":""}],
  "events": [{"name":"","event_type":"","detail_type":"","bus_name":"","description":""}],
  "models": [{"name":"","table_name":"","dialect":"","fields":[{"name":"","type":"","nullable":true,"primary_key":false,"unique":false}],"associations":[{"assoc_type":"","target_model":"","foreign_key":""}]}],
  "external_apis": [{"name":"","url":"","method":"","auth_type":"","description":""}],
  "db_connections": [{"dialect":"","host_var":"","pool_min":0,"pool_max":0,"pool_idle":0}],
  "config_vars": [{"key":"","source":"","description":""}]
}`

	instructions := map[string]string{
		"infra": fmt.Sprintf(`Extract from pubspec.yaml, catalog-info.yaml, Makefile, and firebase.json:
- config_vars: all environment variable names or build config keys referenced (source=env or source=firebase).
- external_apis: any hardcoded base URLs found (name=constant or config key, url=value, method=infer).
- handlers: leave empty (mobile app — no server-side handlers).
- events: leave empty.
- models: leave empty.
- db_connections: leave empty (mobile client — DB is remote).

Output schema:
%s

Files:
`, schema),
		"api": fmt.Sprintf(`Extract from network (HTTP clients) and repository files:
- external_apis: every external HTTP or GraphQL call:
  - REST: name=method name, url=full URL constant or base domain, method=GET/POST/etc, auth_type=bearer if Authorization header is used.
  - GraphQL: name=query/mutation name, url=GraphQL endpoint, method=POST, auth_type=bearer.
  - Omit internal/localhost calls.
- models: data classes returned by repositories (name=class name, dialect=rest or graphql, fields from constructor or fromJson).
- config_vars: any env var or build config key referenced.
- handlers, events, db_connections: leave empty.

Output schema:
%s

Files:
`, schema),
		"features": fmt.Sprintf(`Extract from BLoC and feature repository files:
- handlers: each BLoC event handler (name=event class name, trigger_type=bloc-event, trigger_detail=BLoC class name, description=what it does).
- models: state classes and domain models defined (name=class name, dialect=bloc-state or domain).
- external_apis: any direct API calls not already captured (repositories calling HTTP/GraphQL).
- events, db_connections, config_vars: leave empty unless explicitly present.

Output schema:
%s

Files:
`, schema),
	}

	instruction, ok := instructions[layerName]
	if !ok {
		// feat-<module> layers use the features instruction.
		if strings.HasPrefix(layerName, "feat-") {
			instruction = instructions["features"]
		} else {
			instruction = fmt.Sprintf("Extract all relevant entities.\n\nOutput schema:\n%s\n\nFiles:\n", schema)
		}
	}

	return instruction + "\n" + content
}
