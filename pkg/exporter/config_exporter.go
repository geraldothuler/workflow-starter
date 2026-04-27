package exporter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Cobliteam/workflow-toolkit/pkg/credentials"
	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// ConfigExporter implements Exporter from a YAML-driven ExporterSpec.
// One instance per loaded YAML config file.
type ConfigExporter struct {
	config       ExporterSpec
	credResolver *credentials.Resolver
}

// NewConfigExporter creates a ConfigExporter from a validated ExporterSpec.
func NewConfigExporter(spec ExporterSpec, credResolver *credentials.Resolver) (*ConfigExporter, error) {
	return &ConfigExporter{
		config:       spec,
		credResolver: credResolver,
	}, nil
}

// Name returns the exporter identifier.
func (ce *ConfigExporter) Name() string { return ce.config.ID }

// SetupGuide returns the human-readable setup instructions.
func (ce *ConfigExporter) SetupGuide() string {
	return ce.config.Auth.SetupGuide
}

// Push sends the backlog to the external tool using the YAML-driven configuration.
func (ce *ConfigExporter) Push(ctx context.Context, backlog *types.Backlog, opts PushOptions) (*PushResult, error) {
	// Step 1: Resolve credentials
	creds, err := ce.resolveCredentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s auth failed: %w\n\n%s", ce.config.Name, err, ce.SetupGuide())
	}

	// Step 2: Build template context with credentials
	baseCtx := make(map[string]any)
	for name, cred := range creds {
		baseCtx[name] = cred.Value
	}
	baseCtx["project_key"] = opts.ProjectKey

	// Step 3: Expand transport templates
	expandedBaseURL, err := expandTemplate("base_url", ce.config.Transport.BaseURL, baseCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to expand base_url: %w", err)
	}

	expandedAuthValue, err := expandTemplate("auth_value", ce.config.Transport.AuthValue, baseCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to expand auth_value: %w", err)
	}

	// Step 4: Dry-run mode
	if opts.DryRun {
		return ce.dryRun(backlog, opts, baseCtx)
	}

	// Step 5: Create HTTP transport
	transport := NewHTTPTransport(
		expandedBaseURL,
		ce.config.Transport.AuthType,
		expandedAuthValue,
		ce.config.Transport.Headers,
		ce.config.Transport.Timeout,
	)

	// Step 6: Push epics and stories
	return ce.pushAll(ctx, transport, backlog, opts, baseCtx)
}

// pushAll pushes all epics and their stories.
func (ce *ConfigExporter) pushAll(ctx context.Context, transport *HTTPTransport, backlog *types.Backlog, opts PushOptions, baseCtx map[string]any) (*PushResult, error) {
	result := &PushResult{
		Target:     ce.config.ID,
		ProjectKey: opts.ProjectKey,
	}

	for _, epic := range backlog.Epics {
		// Build epic template context
		epicCtx := mergeContext(baseCtx, map[string]any{
			"epic": epic,
		})

		// Push epic
		epicItem, extractedVars, err := ce.pushItem(ctx, transport, "epic", ce.config.Push.Epic, epicCtx, epic.ID, epic.Title)
		if err != nil {
			epicItem.Error = err.Error()
			result.Errors = append(result.Errors, fmt.Sprintf("epic %q: %v", epic.Title, err))
		} else {
			result.EpicsPushed++
		}
		result.Items = append(result.Items, epicItem)

		// Skip stories if epic creation failed
		if epicItem.Error != "" {
			continue
		}

		// Push stories for this epic
		for _, story := range epic.Stories {
			storyCtx := mergeContext(epicCtx, map[string]any{
				"story": story,
			})
			// Add extracted epic fields to story context
			for k, v := range extractedVars {
				storyCtx[k] = v
			}

			storyItem, _, err := ce.pushItem(ctx, transport, "story", ce.config.Push.Story, storyCtx, story.ID, story.Title)
			if err != nil {
				storyItem.Error = err.Error()
				result.Errors = append(result.Errors, fmt.Sprintf("story %q: %v", story.Title, err))
			} else {
				result.StoriesPushed++
			}
			storyItem.ParentKey = epicItem.RemoteKey
			result.Items = append(result.Items, storyItem)
		}
	}

	return result, nil
}

// pushItem pushes a single epic or story.
func (ce *ConfigExporter) pushItem(ctx context.Context, transport *HTTPTransport, itemType string, spec APICallSpec, tmplCtx map[string]any, localID, title string) (PushedItem, map[string]string, error) {
	item := PushedItem{
		Type:    itemType,
		LocalID: localID,
		Title:   title,
	}

	// Expand path template
	expandedPath, err := expandTemplate(itemType+"_path", spec.Path, tmplCtx)
	if err != nil {
		return item, nil, fmt.Errorf("path template error: %w", err)
	}

	// Expand body template
	var expandedBody string
	if spec.Body != "" {
		expandedBody, err = expandTemplate(itemType+"_body", spec.Body, tmplCtx)
		if err != nil {
			return item, nil, fmt.Errorf("body template error: %w", err)
		}
	}

	// Execute HTTP request
	respBody, _, err := transport.Execute(ctx, spec.Method, expandedPath, expandedBody, spec.Headers)
	if err != nil {
		return item, nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	// Parse response
	var respData any
	if err := json.Unmarshal(respBody, &respData); err != nil {
		return item, nil, fmt.Errorf("response JSON parse failed: %w", err)
	}

	// Extract fields from response
	extractedVars := make(map[string]string)
	for varName, jsonPath := range spec.Extract {
		extracted := extractField(respData, jsonPath)
		if extracted != "" {
			extractedVars[varName] = extracted
		}
	}

	// Set remote key from first extracted value (convention: first extract is the key)
	for _, v := range extractedVars {
		if item.RemoteKey == "" {
			item.RemoteKey = v
		}
		break
	}

	return item, extractedVars, nil
}

// dryRun generates a preview of what would be pushed.
func (ce *ConfigExporter) dryRun(backlog *types.Backlog, opts PushOptions, baseCtx map[string]any) (*PushResult, error) {
	result := &PushResult{
		Target:     ce.config.ID,
		ProjectKey: opts.ProjectKey,
		DryRun:     true,
	}

	for _, epic := range backlog.Epics {
		epicCtx := mergeContext(baseCtx, map[string]any{
			"epic": epic,
		})

		item := PushedItem{
			Type:    "epic",
			LocalID: epic.ID,
			Title:   epic.Title,
		}
		result.Items = append(result.Items, item)
		result.EpicsPushed++

		// Use placeholder epic key for stories
		storyBaseCtx := mergeContext(epicCtx, map[string]any{
			"epic_key": "<EPIC_KEY>",
			"epic_id":  "<EPIC_ID>",
			"epic_url": "<EPIC_URL>",
		})

		for _, story := range epic.Stories {
			storyCtx := mergeContext(storyBaseCtx, map[string]any{
				"story": story,
			})
			_ = storyCtx // suppress unused warning in dry-run

			storyItem := PushedItem{
				Type:      "story",
				LocalID:   story.ID,
				Title:     story.Title,
				ParentKey: fmt.Sprintf("<parent of %s>", epic.Title),
			}
			result.Items = append(result.Items, storyItem)
			result.StoriesPushed++
		}
	}

	return result, nil
}

// resolveCredentials resolves all required credentials for this exporter.
func (ce *ConfigExporter) resolveCredentials(ctx context.Context) (map[string]*credentials.Credential, error) {
	if ce.credResolver == nil {
		return nil, fmt.Errorf("no credential resolver configured")
	}
	return ce.credResolver.ResolveAll(ctx, ce.config.Auth)
}

// mergeContext creates a new map with all entries from base and overlay.
// Overlay values take precedence.
func mergeContext(base, overlay map[string]any) map[string]any {
	result := make(map[string]any, len(base)+len(overlay))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range overlay {
		result[k] = v
	}
	return result
}
