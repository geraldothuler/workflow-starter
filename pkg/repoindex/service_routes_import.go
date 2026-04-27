package repoindex

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// ServiceRoute represents a single route prefix declared in a Helm values.yaml.
type ServiceRoute struct {
	Prefix  string
	Method  string   // optional — only set when chart specifies a method constraint
	Aliases []string // external/public paths that map to this internal prefix
}

// ParseServiceRoutes reads all deploy/helm/**/values.yaml files for a repo and
// extracts routePrefix declarations (plain strings and {prefix, aliases, method} objects).
func ParseServiceRoutes(repoPath string) []ServiceRoute {
	var paths []string

	// Same discovery patterns as ParseChartSnapshots
	single := filepath.Join(repoPath, "deploy", "helm", "chart", "values.yaml")
	if _, err := os.Stat(single); err == nil {
		paths = append(paths, single)
	}
	subDirs, _ := filepath.Glob(filepath.Join(repoPath, "*", "deploy", "helm", "chart", "values.yaml"))
	paths = append(paths, subDirs...)
	subDirs2, _ := filepath.Glob(filepath.Join(repoPath, "deploy", "helm", "*", "chart", "values.yaml"))
	paths = append(paths, subDirs2...)

	var all []ServiceRoute
	seen := map[string]bool{}
	for _, p := range paths {
		routes := parseRoutePrefixFromFile(p)
		for _, r := range routes {
			if !seen[r.Prefix] {
				seen[r.Prefix] = true
				all = append(all, r)
			}
		}
	}
	return all
}

// parseRoutePrefixFromFile reads one values.yaml and extracts all routePrefix entries.
func parseRoutePrefixFromFile(filePath string) []ServiceRoute {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil
	}

	// routePrefix lives inside any top-level chart map (herbie-api-chart, alex-job-chart, etc.)
	for _, v := range raw {
		m, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		rawPrefixes, ok := m["routePrefix"]
		if !ok {
			continue
		}
		prefixes, ok := rawPrefixes.([]interface{})
		if !ok {
			continue
		}
		return extractRoutePrefixes(prefixes)
	}
	return nil
}

// extractRoutePrefixes converts raw YAML list items into ServiceRoute values.
// Each item is either a plain string ("/activities") or a map
// {prefix: "/routes/v2", method: "GET", aliases: ["/public/v2/routes"]}.
func extractRoutePrefixes(raw []interface{}) []ServiceRoute {
	var routes []ServiceRoute
	for _, item := range raw {
		switch v := item.(type) {
		case string:
			if v != "" {
				routes = append(routes, ServiceRoute{Prefix: v})
			}
		case map[string]interface{}:
			prefix, _ := v["prefix"].(string)
			if prefix == "" {
				continue
			}
			method, _ := v["method"].(string)
			var aliases []string
			if rawAliases, ok := v["aliases"].([]interface{}); ok {
				for _, a := range rawAliases {
					if s, ok := a.(string); ok && s != "" {
						aliases = append(aliases, s)
					}
				}
			}
			routes = append(routes, ServiceRoute{Prefix: prefix, Method: method, Aliases: aliases})
		}
	}
	return routes
}

// ImportServiceRoutes persists service route declarations for a repo, replacing prior data.
func ImportServiceRoutes(db *DB, repoName string, routes []ServiceRoute) error {
	repoID := slugID(repoName)
	now := time.Now().Format(time.RFC3339)

	// Replace existing data for this repo
	db.sql.Exec(`DELETE FROM service_route_aliases WHERE route_id IN (SELECT id FROM service_routes WHERE repo_id=?)`, repoID)
	db.sql.Exec(`DELETE FROM service_routes WHERE repo_id=?`, repoID)

	for i, r := range routes {
		routeID := slugID(fmt.Sprintf("%s-route-%d-%s", repoID, i, r.Prefix))
		if _, err := db.sql.Exec(
			`INSERT INTO service_routes(id,repo_id,prefix,method,captured_at) VALUES(?,?,?,?,?)`,
			routeID, repoID, r.Prefix, r.Method, now,
		); err != nil {
			return fmt.Errorf("insert service_route %q: %w", r.Prefix, err)
		}
		for j, alias := range r.Aliases {
			aliasID := slugID(fmt.Sprintf("%s-alias-%d-%s", routeID, j, alias))
			if _, err := db.sql.Exec(
				`INSERT INTO service_route_aliases(id,route_id,alias) VALUES(?,?,?)`,
				aliasID, routeID, alias,
			); err != nil {
				return fmt.Errorf("insert alias %q for route %q: %w", alias, r.Prefix, err)
			}
		}
	}
	return nil
}

// QueryServiceRoutes returns all route prefixes (with aliases) for a repo.
func QueryServiceRoutes(db *DB, repoName string) ([]ServiceRoute, error) {
	repoID := slugID(repoName)
	rows, err := db.sql.Query(
		`SELECT sr.id, sr.prefix, sr.method FROM service_routes sr WHERE sr.repo_id=? ORDER BY sr.prefix`,
		repoID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	routeMap := map[string]*ServiceRoute{}
	var order []string
	for rows.Next() {
		var id, prefix, method string
		if err := rows.Scan(&id, &prefix, &method); err != nil {
			continue
		}
		routeMap[id] = &ServiceRoute{Prefix: prefix, Method: method}
		order = append(order, id)
	}

	// Fetch aliases
	for id := range routeMap {
		arows, err := db.sql.Query(`SELECT alias FROM service_route_aliases WHERE route_id=?`, id)
		if err != nil {
			continue
		}
		for arows.Next() {
			var alias string
			if err := arows.Scan(&alias); err == nil {
				routeMap[id].Aliases = append(routeMap[id].Aliases, alias)
			}
		}
		arows.Close()
	}

	result := make([]ServiceRoute, 0, len(order))
	for _, id := range order {
		result = append(result, *routeMap[id])
	}
	return result, nil
}
