package repoindex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// PublicEndpoint represents a single path/operation documented in the OpenAPI spec.
type PublicEndpoint struct {
	Path        string
	Method      string
	OperationID string
	Summary     string
	AuthType    string // "api-key" | "none" | "legacy"
}

// ImportPublicEndpoints parses an api-docs repo (spec/spec.yaml + referenced path files)
// and persists the results into public_endpoints, replacing prior data.
func ImportPublicEndpoints(db *DB, repoPath string) (int, error) {
	specPath := filepath.Join(repoPath, "spec", "spec.yaml")
	endpoints, err := ParseOpenAPISpec(specPath)
	if err != nil {
		return 0, fmt.Errorf("parse spec: %w", err)
	}

	now := time.Now().Format(time.RFC3339)
	db.sql.Exec(`DELETE FROM public_endpoints`)

	for i, ep := range endpoints {
		id := slugID(fmt.Sprintf("openapi-%d-%s-%s", i, ep.Method, ep.Path))
		if _, err := db.sql.Exec(
			`INSERT INTO public_endpoints(id,path,method,operation_id,summary,auth_type,captured_at) VALUES(?,?,?,?,?,?,?)`,
			id, ep.Path, ep.Method, ep.OperationID, ep.Summary, ep.AuthType, now,
		); err != nil {
			return 0, fmt.Errorf("insert endpoint %s %s: %w", ep.Method, ep.Path, err)
		}
	}
	return len(endpoints), nil
}

// ParseOpenAPISpec reads spec.yaml and resolves each $ref to extract endpoint details.
// Only local $ref are resolved (no HTTP fetching).
func ParseOpenAPISpec(specPath string) ([]PublicEndpoint, error) {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil, err
	}

	var spec map[string]interface{}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("unmarshal spec.yaml: %w", err)
	}

	rawPaths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no 'paths' section in spec.yaml")
	}

	specDir := filepath.Dir(specPath)
	var endpoints []PublicEndpoint

	for path, rawVal := range rawPaths {
		pathMap, ok := rawVal.(map[string]interface{})
		if !ok {
			continue
		}

		// Resolve $ref to local file (optionally with #/fragment)
		ref, hasRef := pathMap["$ref"].(string)
		var ops map[string]interface{}

		if hasRef {
			ops, err = resolveRef(specDir, ref)
			if err != nil {
				// Skip unresolvable refs — don't fail the whole import
				continue
			}
		} else {
			ops = pathMap
		}

		// Extract one endpoint per HTTP method in the operation object
		for _, method := range []string{"get", "post", "put", "patch", "delete", "head", "options"} {
			opRaw, ok := ops[method].(map[string]interface{})
			if !ok {
				continue
			}
			ep := PublicEndpoint{
				Path:        path,
				Method:      strings.ToUpper(method),
				OperationID: stringField(opRaw, "operationId"),
				Summary:     stringField(opRaw, "summary"),
				AuthType:    resolveAuthType(path, opRaw),
			}
			endpoints = append(endpoints, ep)
		}
	}

	return endpoints, nil
}

// resolveRef loads a local $ref path (e.g. "paths/v1/vehicles/list.yaml" or
// "paths/maintenance/maintenance.yaml#/v1") and returns the operation map.
func resolveRef(baseDir, ref string) (map[string]interface{}, error) {
	filePath := ref
	fragment := ""
	if idx := strings.Index(ref, "#/"); idx >= 0 {
		filePath = ref[:idx]
		fragment = ref[idx+2:]
	}

	fullPath := filepath.Join(baseDir, filePath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read ref %s: %w", filePath, err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse ref %s: %w", filePath, err)
	}

	if fragment == "" {
		return raw, nil
	}

	// Navigate into fragment (e.g. "v1" → raw["v1"])
	node, ok := raw[fragment].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("fragment %q not found in %s", fragment, filePath)
	}
	return node, nil
}

// resolveAuthType infers authentication type from path prefix and security field.
func resolveAuthType(path string, op map[string]interface{}) string {
	if strings.HasPrefix(path, "/herbie-1.1/") {
		return "legacy"
	}
	security, hasSec := op["security"].([]interface{})
	if !hasSec || len(security) == 0 {
		return "none"
	}
	for _, s := range security {
		if sm, ok := s.(map[string]interface{}); ok {
			if _, hasAPIKey := sm["APIKey"]; hasAPIKey {
				return "api-key"
			}
		}
	}
	return "none"
}

// QueryPublicEndpoints returns all documented public endpoints, optionally filtered by path prefix.
func QueryPublicEndpoints(db *DB, pathFilter string) ([]PublicEndpoint, error) {
	var rows interface{ Next() bool; Scan(...interface{}) error; Close() error }
	var err error

	if pathFilter != "" {
		rows, err = db.sql.Query(
			`SELECT path,method,operation_id,summary,auth_type FROM public_endpoints WHERE path LIKE ? ORDER BY path,method`,
			pathFilter+"%",
		)
	} else {
		rows, err = db.sql.Query(
			`SELECT path,method,operation_id,summary,auth_type FROM public_endpoints ORDER BY path,method`,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var eps []PublicEndpoint
	for rows.Next() {
		var ep PublicEndpoint
		if err := rows.Scan(&ep.Path, &ep.Method, &ep.OperationID, &ep.Summary, &ep.AuthType); err != nil {
			continue
		}
		eps = append(eps, ep)
	}
	return eps, nil
}

// QueryPublicEndpointsForRepo returns public endpoints whose path matches any
// service_route_alias for the given repo, plus gap (handlers with no public match).
func QueryPublicEndpointsForRepo(db *DB, repoName string) (documented []PublicEndpoint, err error) {
	repoID := slugID(repoName)
	rows, err := db.sql.Query(`
		SELECT DISTINCT pe.path, pe.method, pe.operation_id, pe.summary, pe.auth_type
		FROM public_endpoints pe
		JOIN service_route_aliases sra ON pe.path = sra.alias
		JOIN service_routes sr ON sr.id = sra.route_id
		WHERE sr.repo_id = ?
		ORDER BY pe.path, pe.method
	`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ep PublicEndpoint
		if err := rows.Scan(&ep.Path, &ep.Method, &ep.OperationID, &ep.Summary, &ep.AuthType); err != nil {
			continue
		}
		documented = append(documented, ep)
	}
	return documented, nil
}

// QueryHandlerGaps returns HTTP handler paths for a repo that have no matching public_endpoint.
// Only handlers with trigger_type = "http" (or empty, assuming HTTP) are considered.
func QueryHandlerGaps(db *DB, repoName string) ([]string, error) {
	repoID := slugID(repoName)
	rows, err := db.sql.Query(`
		SELECT DISTINCT h.trigger_detail
		FROM handlers h
		WHERE h.repo_id = ?
		  AND h.trigger_detail != ''
		  AND h.trigger_detail NOT IN (
		      SELECT sra.alias
		      FROM service_route_aliases sra
		      JOIN service_routes sr ON sr.id = sra.route_id
		      WHERE sr.repo_id = ?
		  )
		ORDER BY h.trigger_detail
	`, repoID, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gaps []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			continue
		}
		gaps = append(gaps, path)
	}
	return gaps, nil
}
