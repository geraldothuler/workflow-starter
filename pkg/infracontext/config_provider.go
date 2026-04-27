package infracontext

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"text/template"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/credentials"
	"github.com/Cobliteam/workflow-toolkit/pkg/sources"
	"github.com/Cobliteam/workflow-toolkit/pkg/transport"
)

// CommandExecutor abstracts command execution for testing.
type CommandExecutor interface {
	Execute(ctx context.Context, command string, args []string) ([]byte, error)
}

// RealExecutor runs actual OS commands.
type RealExecutor struct{}

// Execute runs a command and returns its stdout.
func (r *RealExecutor) Execute(ctx context.Context, command string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	return cmd.Output()
}

// ConfigProvider is a generic YAML-driven implementation of the Provider interface.
// It interprets the InfraProviderSpec to fetch and map infrastructure data.
type ConfigProvider struct {
	spec         *InfraProviderSpec
	techMapper   *TechMapper
	engine       *MappingEngine
	cache        *Cache
	credResolver *credentials.Resolver
	executor     CommandExecutor

	// HTTP transport (lazy-initialized on first HTTP fetch)
	httpTransport *transport.HTTPTransport
	httpHeaders   map[string]string // headers expanded with credentials
	httpAuthType  string            // cached auth type for per-step transport overrides
	httpAuthValue string            // cached expanded auth value for per-step transport overrides
}

// NewConfigProvider creates a ConfigProvider from a provider spec.
func NewConfigProvider(spec *InfraProviderSpec, techMapper *TechMapper, cache *Cache, credResolver *credentials.Resolver) *ConfigProvider {
	engine := NewMappingEngine(techMapper)
	return &ConfigProvider{
		spec:         spec,
		techMapper:   techMapper,
		engine:       engine,
		cache:        cache,
		credResolver: credResolver,
		executor:     &RealExecutor{},
	}
}

// SetExecutor replaces the command executor (for testing).
func (cp *ConfigProvider) SetExecutor(e CommandExecutor) {
	cp.executor = e
}

// SetHTTPTransport replaces the HTTP transport (for testing).
func (cp *ConfigProvider) SetHTTPTransport(t *transport.HTTPTransport) {
	cp.httpTransport = t
}

// ID returns the provider identifier.
func (cp *ConfigProvider) ID() string {
	return cp.spec.ID
}

// Name returns the provider display name.
func (cp *ConfigProvider) Name() string {
	return cp.spec.Name
}

// Available checks if the provider can run on the current system.
func (cp *ConfigProvider) Available() bool {
	if cp.spec.Transport.Primary == "http" && cp.spec.Transport.HTTP != nil {
		// For HTTP providers, check if credentials can be resolved
		if cp.httpTransport != nil {
			return true // injected transport (test mode)
		}
		if cp.credResolver == nil {
			return false
		}
		// Try to resolve credentials without error to check availability
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := cp.credResolver.ResolveAll(ctx, cp.spec.Auth)
		return err == nil
	}

	if cp.spec.Transport.CLI != nil {
		cli := cp.spec.Transport.CLI
		if len(cli.AvailableCheck) > 0 {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err := cp.executor.Execute(ctx, cli.Command, cli.AvailableCheck)
			return err == nil
		}
		// Just check if the command exists
		_, err := exec.LookPath(cli.Command)
		return err == nil
	}
	return false
}

// Fetch retrieves infrastructure context using the configured transport.
func (cp *ConfigProvider) Fetch(ctx context.Context, opts FetchOptions) (*InfraContext, error) {
	// Check cache first
	if opts.UseCache && cp.cache != nil {
		cacheKey := cp.cacheKey(opts)
		if cached, ok := cp.cache.Get(cacheKey); ok {
			return cached, nil
		}
	}

	namespace := opts.Namespace
	if namespace == "" {
		namespace = cp.spec.Defaults.Namespace
	}

	ic := &InfraContext{
		Provider:  cp.spec.ID,
		Cluster:   cp.clusterName(opts),
		Namespace: namespace,
		FetchedAt: time.Now(),
		TTL:       cp.spec.Defaults.TTL,
	}

	templateData := map[string]string{
		"namespace": namespace,
	}
	if opts.KubeContext != "" {
		templateData["context"] = opts.KubeContext
	}

	// Initialize HTTP transport lazily if needed
	if cp.spec.Transport.Primary == "http" && cp.spec.Transport.HTTP != nil && cp.httpTransport == nil {
		if err := cp.ensureHTTPTransport(ctx); err != nil {
			return nil, fmt.Errorf("initializing HTTP transport: %w", err)
		}
	}

	// Merge resolved credentials into templateData so steps can reference them
	// (e.g., {{.CONFLUENT_ENVIRONMENT}} in http_path)
	if cp.credResolver != nil && len(cp.spec.Auth.Credentials) > 0 {
		creds, err := cp.credResolver.ResolveAll(ctx, cp.spec.Auth)
		if err == nil {
			for name, cred := range creds {
				if _, exists := templateData[name]; !exists {
					templateData[name] = cred.Value
				}
			}
		}
	}

	// stepVars holds values extracted by provides, consumed by for_each
	stepVars := make(map[string][]any)

	for _, step := range cp.spec.FetchSteps {
		// Skip steps for resource types not requested
		if len(opts.ResourceTypes) > 0 && !contains(opts.ResourceTypes, step.ID) {
			continue
		}

		if step.ForEach != "" {
			// for_each step: iterate over items from a previous provides
			items, ok := stepVars[step.ForEach]
			if !ok || len(items) == 0 {
				if step.Optional {
					continue
				}
				return nil, fmt.Errorf("fetch step %q: for_each %q: no values provided", step.ID, step.ForEach)
			}
			for _, item := range items {
				iterData := cloneTemplateData(templateData)
				applyItemToTemplateData(iterData, item)
				_, err := cp.executeStep(ctx, step, iterData, ic, opts)
				if err != nil {
					if step.Optional {
						continue
					}
					return nil, fmt.Errorf("fetch step %q (for_each): %w", step.ID, err)
				}
			}
		} else {
			rawData, err := cp.executeStep(ctx, step, templateData, ic, opts)
			if err != nil {
				if step.Optional {
					continue // skip optional steps on error
				}
				return nil, fmt.Errorf("fetch step %q: %w", step.ID, err)
			}
			// Extract provides from this step's response
			if step.Provides != nil && rawData != nil {
				for k, v := range extractProvides(rawData, step.Provides) {
					stepVars[k] = v
				}
			}
		}
	}

	// Store in cache
	if opts.UseCache && cp.cache != nil {
		cacheKey := cp.cacheKey(opts)
		cp.cache.Put(cacheKey, ic)
	}

	return ic, nil
}

func (cp *ConfigProvider) executeStep(ctx context.Context, step InfraFetchStep, templateData map[string]string, ic *InfraContext, opts FetchOptions) (any, error) {
	var rawData any
	var rawText string
	isText := step.ParseMode == "text"

	if cp.spec.Transport.Primary == "cli" && cp.spec.Transport.CLI != nil {
		// CLI transport
		var args []string
		if len(step.CLIArgs) > 0 {
			// Explicit args list: each arg is template-expanded independently (preserves spaces)
			for _, arg := range step.CLIArgs {
				expanded, err := expandAnyTemplate(arg, templateData)
				if err != nil {
					return nil, fmt.Errorf("expanding cli_arg: %w", err)
				}
				args = append(args, expanded)
			}
		} else {
			cmdStr, err := expandTemplate(step.CLICommand, templateData)
			if err != nil {
				return nil, fmt.Errorf("expanding cli_command template: %w", err)
			}
			args = strings.Fields(cmdStr)
		}
		if opts.KubeContext != "" {
			args = append([]string{"--context", opts.KubeContext}, args...)
		}

		timeout := cp.spec.Transport.CLI.Timeout
		execCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		output, err := cp.executor.Execute(execCtx, cp.spec.Transport.CLI.Command, args)
		if err != nil {
			return nil, fmt.Errorf("executing %s (step %s): %w", cp.spec.Transport.CLI.Command, step.ID, err)
		}

		if isText {
			rawText = string(output)
		} else {
			if err := json.Unmarshal(output, &rawData); err != nil {
				return nil, fmt.Errorf("parsing JSON output: %w", err)
			}
		}
	} else if cp.spec.Transport.Primary == "http" && cp.httpTransport != nil {
		// HTTP transport — determine which transport to use (default or per-step override)
		targetTransport := cp.httpTransport
		if step.HTTPBaseURL != "" {
			expandedBase, err := expandAnyTemplate(step.HTTPBaseURL, templateData)
			if err != nil {
				return nil, fmt.Errorf("expanding http_base_url: %w", err)
			}
			targetTransport = transport.NewHTTPTransport(expandedBase, cp.httpAuthType, cp.httpAuthValue, cp.httpHeaders, cp.spec.Transport.HTTP.Timeout)
		}

		expandedPath, err := expandAnyTemplate(step.HTTPPath, templateData)
		if err != nil {
			return nil, fmt.Errorf("expanding http_path template: %w", err)
		}

		// Expand query params
		expandedParams := make(map[string]string)
		for k, v := range step.HTTPParams {
			expanded, err := expandAnyTemplate(v, templateData)
			if err != nil {
				return nil, fmt.Errorf("expanding http_param %q: %w", k, err)
			}
			expandedParams[k] = expanded
		}

		method := step.HTTPMethod
		if method == "" {
			method = "GET"
		}

		if step.Pagination != nil {
			// Paginated request — accumulate all pages
			pCfg := transport.PaginationConfig{
				Style:         step.Pagination.Style,
				CursorParam:   step.Pagination.CursorParam,
				CursorPath:    step.Pagination.CursorPath,
				PageParam:     step.Pagination.PageParam,
				PageSizeParam: step.Pagination.PageSizeParam,
				PageSize:      step.Pagination.PageSize,
				OffsetParam:   step.Pagination.OffsetParam,
				LimitParam:    step.Pagination.LimitParam,
				Limit:         step.Pagination.Limit,
				TotalPath:     step.Pagination.TotalPath,
				ResultsPath:   step.Pagination.ResultsPath,
				MaxPages:      step.Pagination.MaxPages,
			}

			items, err := transport.ExecuteAllPages(ctx, targetTransport, method, expandedPath, expandedParams, step.HTTPHeaders, pCfg)
			if err != nil {
				return nil, fmt.Errorf("paginated HTTP request: %w", err)
			}
			// Wrap items into a structure with the results path key for mapping
			rawData = wrapForMapping(items, step.Pagination.ResultsPath)
		} else {
			// Single request
			resp, _, err := targetTransport.Execute(ctx, method, expandedPath, step.HTTPBody, step.HTTPHeaders)
			if err != nil {
				return nil, fmt.Errorf("HTTP request: %w", err)
			}
			if err := json.Unmarshal(resp, &rawData); err != nil {
				return nil, fmt.Errorf("parsing HTTP JSON response: %w", err)
			}
		}
	} else {
		return nil, fmt.Errorf("transport %q not implemented", cp.spec.Transport.Primary)
	}

	// Apply mappings
	if step.Mapping.Topology != nil && !isText {
		nodes, err := cp.engine.MapTopology(rawData, step.Mapping.Topology)
		if err != nil {
			return rawData, fmt.Errorf("mapping topology: %w", err)
		}
		ic.Topology = append(ic.Topology, nodes...)
	}

	if step.Mapping.Health != nil && !isText {
		checks, err := cp.engine.MapHealth(rawData, step.Mapping.Health)
		if err != nil {
			return rawData, fmt.Errorf("mapping health: %w", err)
		}
		ic.Health = append(ic.Health, checks...)
	}

	if step.Mapping.Metrics != nil {
		if isText {
			metrics, err := cp.engine.MapTextMetrics(rawText, step.Mapping.Metrics)
			if err != nil {
				return rawData, fmt.Errorf("mapping text metrics: %w", err)
			}
			ic.Metrics = append(ic.Metrics, metrics...)
		} else {
			metrics, err := cp.engine.MapMetrics(rawData, step.Mapping.Metrics)
			if err != nil {
				return rawData, fmt.Errorf("mapping metrics: %w", err)
			}
			ic.Metrics = append(ic.Metrics, metrics...)
		}
	}

	if step.Mapping.Alerts != nil && !isText {
		alerts, err := cp.engine.MapAlerts(rawData, step.Mapping.Alerts)
		if err != nil {
			return rawData, fmt.Errorf("mapping alerts: %w", err)
		}
		ic.Alerts = append(ic.Alerts, alerts...)
	}

	return rawData, nil
}

// ensureHTTPTransport lazily initializes the HTTP transport with resolved credentials.
func (cp *ConfigProvider) ensureHTTPTransport(ctx context.Context) error {
	if cp.httpTransport != nil {
		return nil
	}

	httpSpec := cp.spec.Transport.HTTP
	if httpSpec == nil {
		return fmt.Errorf("no HTTP spec configured")
	}

	// Resolve credentials
	var credValues map[string]string
	if cp.credResolver != nil && len(cp.spec.Auth.Credentials) > 0 {
		creds, err := cp.credResolver.ResolveAll(ctx, cp.spec.Auth)
		if err != nil {
			return fmt.Errorf("resolving credentials: %w", err)
		}
		credValues = make(map[string]string, len(creds))
		for name, cred := range creds {
			credValues[name] = cred.Value
		}
	} else {
		credValues = make(map[string]string)
	}

	// Expand base_url template with credentials
	expandedBaseURL, err := expandAnyTemplate(httpSpec.BaseURL, credValues)
	if err != nil {
		return fmt.Errorf("expanding base_url: %w", err)
	}

	// Expand auth_value template with credentials
	expandedAuthValue := ""
	if httpSpec.AuthValue != "" {
		expandedAuthValue, err = expandAnyTemplate(httpSpec.AuthValue, credValues)
		if err != nil {
			return fmt.Errorf("expanding auth_value: %w", err)
		}
	}

	// Expand header templates with credentials
	expandedHeaders := make(map[string]string, len(httpSpec.Headers))
	for k, v := range httpSpec.Headers {
		expanded, err := expandAnyTemplate(v, credValues)
		if err != nil {
			return fmt.Errorf("expanding header %q: %w", k, err)
		}
		expandedHeaders[k] = expanded
	}

	cp.httpTransport = transport.NewHTTPTransport(expandedBaseURL, httpSpec.AuthType, expandedAuthValue, expandedHeaders, httpSpec.Timeout)
	cp.httpHeaders = expandedHeaders
	cp.httpAuthType = httpSpec.AuthType
	cp.httpAuthValue = expandedAuthValue

	return nil
}

func (cp *ConfigProvider) clusterName(opts FetchOptions) string {
	if opts.KubeContext != "" {
		return opts.KubeContext
	}

	// For HTTP providers, use the provider ID as cluster name
	if cp.spec.Transport.Primary == "http" {
		return cp.spec.ID
	}

	// Try to get current context name (for CLI providers)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := cp.executor.Execute(ctx, "kubectl", []string{"config", "current-context"})
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

func (cp *ConfigProvider) cacheKey(opts FetchOptions) string {
	key := cp.spec.ID + ":" + opts.Namespace
	if opts.KubeContext != "" {
		key += ":" + opts.KubeContext
	}
	return key
}

func expandTemplate(tmplStr string, data map[string]string) (string, error) {
	tmpl, err := template.New("cmd").Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// expandAnyTemplate expands a Go template string with any string map as data.
// Uses "or" function for defaults: {{or .DD_SITE "api.datadoghq.com"}}
func expandAnyTemplate(tmplStr string, data map[string]string) (string, error) {
	funcMap := template.FuncMap{
		"or": func(values ...any) string {
			for _, v := range values {
				if v == nil {
					continue
				}
				s := fmt.Sprintf("%v", v)
				if s != "" && s != "<nil>" && s != "<no value>" {
					return s
				}
			}
			return ""
		},
	}
	tmpl, err := template.New("tmpl").Funcs(funcMap).Option("missingkey=zero").Parse(tmplStr)
	if err != nil {
		return "", err
	}

	// Convert to map[string]any for template execution
	dataMap := make(map[string]any, len(data))
	for k, v := range data {
		dataMap[k] = v
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, dataMap); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// wrapForMapping wraps paginated items into a structure that the mapping engine expects.
// If resultsPath is "items", returns {"items": items}. If empty or ".", returns items directly.
func wrapForMapping(items []any, resultsPath string) any {
	if resultsPath == "" || resultsPath == "." {
		return items
	}
	// Create a nested map matching the results path
	parts := strings.Split(resultsPath, ".")
	var result any = items
	for i := len(parts) - 1; i >= 0; i-- {
		result = map[string]any{parts[i]: result}
	}
	return result
}

// extractProvides extracts values from a step's raw response for use in for_each.
func extractProvides(rawData any, provides map[string]*InfraProvidesSpec) map[string][]any {
	result := make(map[string][]any)
	for name, spec := range provides {
		items := sources.ExtractSlice(rawData, spec.SourcePath)
		if spec.Field != "" {
			// Single-field: extract []string values
			for _, item := range items {
				val := sources.ExtractString(item, spec.Field)
				if val != "" {
					result[name] = append(result[name], val)
				}
			}
		} else if len(spec.Fields) > 0 {
			// Multi-field: extract []map[string]string
			for _, item := range items {
				fields := make(map[string]string)
				for alias, path := range spec.Fields {
					fields[alias] = sources.ExtractString(item, path)
				}
				result[name] = append(result[name], fields)
			}
		}
	}
	return result
}

// cloneTemplateData creates a shallow copy of template data for per-iteration isolation.
func cloneTemplateData(data map[string]string) map[string]string {
	clone := make(map[string]string, len(data))
	for k, v := range data {
		clone[k] = v
	}
	return clone
}

// applyItemToTemplateData injects for_each item values into template data.
// For single-field provides: sets {{.item}}.
// For multi-field provides: sets {{.item_<field>}} for each field.
func applyItemToTemplateData(data map[string]string, item any) {
	switch v := item.(type) {
	case string:
		data["item"] = v
	case map[string]string:
		for k, val := range v {
			data["item_"+k] = val
		}
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
