package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/credentials"
)

// ConfigSource implements Source from a YAML-driven SourceSpec.
// One instance per loaded YAML config file. Stateless after construction.
type ConfigSource struct {
	config       SourceSpec
	urlRegexps   []*regexp.Regexp     // compiled from config.URLPatterns
	parseRegex   *regexp.Regexp       // compiled from config.URLParser.Regex
	mcpFactory   MCPClientFactory     // injected for testability
	credResolver *credentials.Resolver // optional; nil = legacy os.Getenv fallback
}

// NewConfigSource creates a ConfigSource from a validated SourceSpec.
// Optional credResolver enables pluggable credential resolution; nil falls back to os.Getenv().
func NewConfigSource(spec SourceSpec, factory MCPClientFactory, credResolver ...*credentials.Resolver) (*ConfigSource, error) {
	// Compile URL patterns
	urlRegexps := make([]*regexp.Regexp, 0, len(spec.URLPatterns))
	for _, pattern := range spec.URLPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid url_pattern %q: %w", pattern, err)
		}
		urlRegexps = append(urlRegexps, re)
	}

	// Compile URL parser regex
	parseRegex, err := regexp.Compile(spec.URLParser.Regex)
	if err != nil {
		return nil, fmt.Errorf("invalid url_parser.regex %q: %w", spec.URLParser.Regex, err)
	}

	if factory == nil {
		factory = NewMCPProcessFactory()
	}

	var resolver *credentials.Resolver
	if len(credResolver) > 0 && credResolver[0] != nil {
		resolver = credResolver[0]
	}

	return &ConfigSource{
		config:       spec,
		urlRegexps:   urlRegexps,
		parseRegex:   parseRegex,
		mcpFactory:   factory,
		credResolver: resolver,
	}, nil
}

// Name returns the source identifier.
func (cs *ConfigSource) Name() string { return cs.config.ID }

// CanHandle returns true if the URL matches any of the configured patterns.
func (cs *ConfigSource) CanHandle(url string) bool {
	lowered := strings.ToLower(url)
	for _, re := range cs.urlRegexps {
		if re.MatchString(lowered) {
			return true
		}
	}
	return false
}

// SetupGuide returns the human-readable setup instructions.
func (cs *ConfigSource) SetupGuide() string {
	return cs.config.Auth.SetupGuide
}

// Fetch retrieves content from the URL using the MCP fetch pipeline.
func (cs *ConfigSource) Fetch(url string) (*FetchResult, error) {
	// Step A: Check auth — prefer credential resolver, fallback to os.Getenv
	token, err := cs.resolveAuthToken()
	if err != nil || token == "" {
		return nil, fmt.Errorf("%s token não configurado.\n\n%s", cs.config.Name, cs.SetupGuide())
	}

	// Step B: Parse URL — extract named captures
	vars := cs.parseURL(url)
	if vars == nil {
		return nil, fmt.Errorf("não foi possível extrair parâmetros de: %s", url)
	}

	// Step C: Connect MCP session
	ctx := context.Background()
	session, err := cs.mcpFactory.Connect(ctx, cs.config.Transport)
	if err != nil {
		return nil, fmt.Errorf("falha ao conectar MCP server para %s: %w", cs.config.Name, err)
	}
	defer session.Close()

	// Step D: Execute fetch_steps
	storedResults := make(map[string]any)
	var lastResult any

	for i, step := range cs.config.FetchSteps {
		// Template-expand args
		expandedArgs, err := expandTemplateArgs(step.Args, vars)
		if err != nil {
			return nil, fmt.Errorf("fetch_step[%d] arg expansion failed: %w", i, err)
		}

		// Call MCP tool
		rawResult, err := session.CallTool(ctx, step.Tool, expandedArgs)
		if err != nil {
			return nil, fmt.Errorf("fetch_step[%d] tool %q failed: %w", i, step.Tool, err)
		}

		// Parse JSON result
		var result any
		if err := json.Unmarshal(rawResult, &result); err != nil {
			return nil, fmt.Errorf("fetch_step[%d] JSON parse failed: %w", i, err)
		}
		lastResult = result

		// Extract fields into vars
		for fieldName, path := range step.Extract {
			extracted := ExtractString(result, path)
			if extracted != "" || path == "." {
				vars[fieldName] = extracted
			}
		}

		// Store full result if requested
		if step.StoreAs != "" {
			storedResults[step.StoreAs] = result
		}
	}

	// Step E: Convert to markdown
	markdown, err := cs.renderMarkdown(vars, storedResults, lastResult)
	if err != nil {
		return nil, fmt.Errorf("markdown conversion failed: %w", err)
	}

	// Add title as H1
	title := vars["title"]
	if title != "" {
		markdown = fmt.Sprintf("# %s\n\n%s", title, markdown)
	}

	return &FetchResult{
		Content:    markdown,
		Title:      title,
		URL:        url,
		Source:     cs.config.ID,
		BlockCount: countStructure(lastResult),
		Metadata: map[string]string{
			"source_type": cs.config.Transport.Type,
		},
	}, nil
}

// parseURL extracts named captures from the URL using the configured regex.
func (cs *ConfigSource) parseURL(url string) map[string]string {
	matches := cs.parseRegex.FindStringSubmatch(url)
	if matches == nil {
		return nil
	}

	vars := make(map[string]string)
	for name, groupIndex := range cs.config.URLParser.Captures {
		if groupIndex < len(matches) {
			vars[name] = matches[groupIndex]
		}
	}
	return vars
}

// renderMarkdown converts the fetched data to markdown using the configured mode.
func (cs *ConfigSource) renderMarkdown(vars map[string]string, stored map[string]any, lastResult any) (string, error) {
	mode := strings.ToLower(cs.config.Markdown.Mode)

	switch mode {
	case "walker":
		return cs.renderWalker(vars, stored, lastResult)
	case "template":
		return cs.renderTemplate(vars, stored, lastResult)
	default:
		return "", fmt.Errorf("unknown markdown mode: %q", mode)
	}
}

// renderWalker uses the JSON walker to convert data to markdown.
func (cs *ConfigSource) renderWalker(vars map[string]string, stored map[string]any, lastResult any) (string, error) {
	spec := cs.config.Markdown.Walker
	walker := NewJSONWalker(spec)

	// Determine which data to walk
	var data any
	if spec != nil && spec.SourceKey != "" {
		data = stored[spec.SourceKey]
		if data == nil {
			return "", fmt.Errorf("walker source_key %q not found in stored results", spec.SourceKey)
		}
	} else {
		data = lastResult
	}

	return walker.Walk(data), nil
}

// renderTemplate uses Go text/template to convert data to markdown.
func (cs *ConfigSource) renderTemplate(vars map[string]string, stored map[string]any, lastResult any) (string, error) {
	// Build template data: merge vars + stored results
	data := make(map[string]any)
	for k, v := range vars {
		data[k] = v
	}
	for k, v := range stored {
		data[k] = v
	}
	if lastResult != nil {
		data["_result"] = lastResult
	}

	return RenderTemplate(cs.config.Markdown.Template, data)
}

// resolveAuthToken resolves the auth token using the credential resolver chain.
// Falls back to os.Getenv for backward compatibility.
func (cs *ConfigSource) resolveAuthToken() (string, error) {
	envVar := cs.config.Auth.EnvVar
	if envVar == "" {
		return "", fmt.Errorf("no auth.env_var configured")
	}

	// Prefer credential resolver
	if cs.credResolver != nil {
		cred, err := cs.credResolver.Resolve(context.Background(), envVar, nil)
		if err == nil {
			return cred.Value, nil
		}
		// Fall through to legacy
	}

	// Legacy fallback
	return os.Getenv(envVar), nil
}

// countStructure recursively counts elements in the JSON structure.
func countStructure(data any) int {
	switch v := data.(type) {
	case map[string]any:
		count := 1
		for _, val := range v {
			count += countStructure(val)
		}
		return count
	case []any:
		count := 0
		for _, item := range v {
			count += countStructure(item)
		}
		return count
	default:
		return 0
	}
}
