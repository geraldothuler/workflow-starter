package repoindex

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ProtoSchemaParser indexes a schema-registry repo: proto message types as models,
// asyncapi YAML definitions as Kafka topic contracts (events).
type ProtoSchemaParser struct{}

func (p *ProtoSchemaParser) Lang() string      { return "protobuf" }
func (p *ProtoSchemaParser) Framework() string { return "schema-registry" }

// Layers groups proto files by domain directory + asyncapi YAML as a separate layer.
// Google API protos and build/generated files are excluded.
func (p *ProtoSchemaParser) Layers(repoPath string) ([]Layer, error) {
	// Collect all proto files grouped by their parent directory name.
	dirFiles := map[string][]string{}
	protoRoot := filepath.Join(repoPath, "protos")
	if _, err := os.Stat(protoRoot); err != nil {
		// Fallback: search entire repo for .proto files
		protoRoot = repoPath
	}

	filepath.Walk(protoRoot, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".proto" {
			return nil
		}
		// Skip generated, target, google/api standard protos
		rel, _ := filepath.Rel(protoRoot, path)
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) > 0 {
			top := parts[0]
			if top == "google" || top == "target" || top == "build" {
				return nil
			}
			dirFiles[top] = append(dirFiles[top], path)
		}
		return nil
	})

	// Group directories into thematic layers to limit LLM calls.
	groups := thematicGroups(dirFiles)

	var layers []Layer
	for _, g := range groups {
		if len(g.files) > 0 {
			layers = append(layers, Layer{Name: g.name, Files: g.files})
		}
	}

	// AsyncAPI YAML layer — Kafka topic contracts
	asyncapi := Layer{Name: "asyncapi"}
	filepath.Walk(filepath.Join(repoPath, "asyncapi"), func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".yaml" || ext == ".yml" {
			asyncapi.Files = append(asyncapi.Files, path)
		}
		return nil
	})
	if len(asyncapi.Files) > 0 {
		layers = append(layers, asyncapi)
	}

	return layers, nil
}

type protoGroup struct {
	name  string
	files []string
}

// maxFilesPerLayer caps how many proto files go into a single LLM call.
// Directories with more files are split into dir_1, dir_2, … chunks.
const maxFilesPerLayer = 8

// thematicGroups turns the per-directory file map into ordered layers.
// Each proto directory becomes its own layer (no hardcoded domain mapping).
// If a directory has more than maxFilesPerLayer files it is split into
// named chunks (e.g. "camera_1", "camera_2") so the LLM receives bounded context.
// New directories in schema-registry/protos/ are picked up automatically on
// the next `wtb repo index --force` — zero code changes required.
func thematicGroups(dirFiles map[string][]string) []protoGroup {
	dirs := sortedStringKeys(dirFiles)
	var result []protoGroup
	for _, dir := range dirs {
		files := dirFiles[dir]
		if len(files) <= maxFilesPerLayer {
			result = append(result, protoGroup{name: dir, files: files})
			continue
		}
		for i, chunk := range chunkStrings(files, maxFilesPerLayer) {
			result = append(result, protoGroup{
				name:  fmt.Sprintf("%s_%d", dir, i+1),
				files: chunk,
			})
		}
	}
	return result
}

func sortedStringKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func chunkStrings(s []string, size int) [][]string {
	var chunks [][]string
	for len(s) > 0 {
		if size > len(s) {
			size = len(s)
		}
		chunks = append(chunks, s[:size])
		s = s[size:]
	}
	return chunks
}

func (p *ProtoSchemaParser) SystemPrompt() string {
	return `You are a senior platform architect with deep knowledge of Protocol Buffers (proto3) and event-driven architectures.

You extract structured metadata from .proto files and AsyncAPI YAML specs to populate a code intelligence database.
Focus on:
- proto files: message types → models (with fields and scalar types)
- AsyncAPI YAML: Kafka channel definitions → events (topic name, schema reference)

Always respond with a single valid JSON object matching the ExtractedLayer schema.
Never include markdown fences, explanations, or extra text — only the JSON object.`
}

func (p *ProtoSchemaParser) LayerPrompt(layerName, content string) string {
	schema := `{
  "handlers": [],
  "events": [{"name":"","event_type":"","detail_type":"","bus_name":"","description":""}],
  "models": [{"name":"","table_name":"","dialect":"","fields":[{"name":"","type":"","nullable":true,"primary_key":false,"unique":false}],"associations":[]}],
  "external_apis": [],
  "db_connections": [],
  "config_vars": []
}`

	var instruction string
	if layerName == "asyncapi" {
		instruction = fmt.Sprintf(`Extract from AsyncAPI YAML specs:
- events: each Kafka channel definition (name=channel/topic name, event_type="kafka-schema", bus_name=channel name, detail_type=schema message type referenced, description=channel purpose from description field).

Output schema:
%s

Files:
`, schema)
	} else {
		instruction = fmt.Sprintf(`Extract from these .proto files (domain: %s):
- models: each message type (name=message name, table_name=snake_case of message name, dialect="protobuf", fields=all fields with their scalar/message types, nullable=true for all proto fields).
  Ignore: empty messages, google.protobuf.* wrapper types as standalone models.

Output schema:
%s

Files:
`, layerName, schema)
	}

	return instruction + "\n" + content
}
