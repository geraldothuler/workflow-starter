package repoindex

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ParseChartSnapshots finds all deploy/helm/**/values.yaml files in a repo and parses them.
// Returns one ChartSnapshot per chart file (multi-deployment repos like fusca yield multiple).
func ParseChartSnapshots(repoName, repoPath string) ([]ChartSnapshot, error) {
	var paths []string

	// Pattern 1: <repoPath>/deploy/helm/chart/values.yaml  (single-deployment)
	single := filepath.Join(repoPath, "deploy", "helm", "chart", "values.yaml")
	if _, err := os.Stat(single); err == nil {
		paths = append(paths, single)
	}

	// Pattern 2: <repoPath>/<sub>/deploy/helm/chart/values.yaml  (multi-deployment, e.g. fusca)
	subDirs, _ := filepath.Glob(filepath.Join(repoPath, "*", "deploy", "helm", "chart", "values.yaml"))
	paths = append(paths, subDirs...)

	// Pattern 3: <repoPath>/deploy/helm/<sub>/chart/values.yaml  (herbie-api)
	subDirs2, _ := filepath.Glob(filepath.Join(repoPath, "deploy", "helm", "*", "chart", "values.yaml"))
	paths = append(paths, subDirs2...)

	if len(paths) == 0 {
		return nil, fmt.Errorf("no deploy/helm/**/values.yaml found for %q", repoName)
	}

	var snaps []ChartSnapshot
	for _, p := range paths {
		snap, err := parseChartFile(repoName, repoPath, p)
		if err != nil {
			continue
		}
		snaps = append(snaps, snap)
	}
	return snaps, nil
}

// ImportChartSnapshots persists chart snapshots for a repo, replacing prior data.
func ImportChartSnapshots(db *DB, repoName string, snaps []ChartSnapshot) error {
	repoID := slugID(repoName)
	now := time.Now().Format(time.RFC3339)

	// Delete existing snapshot data for this repo
	db.sql.Exec(`DELETE FROM chart_env_vars    WHERE repo_id=?`, repoID)
	db.sql.Exec(`DELETE FROM chart_resources   WHERE repo_id=?`, repoID)
	db.sql.Exec(`DELETE FROM chart_sidecars    WHERE repo_id=?`, repoID)
	db.sql.Exec(`DELETE FROM chart_snapshots   WHERE repo_id=?`, repoID)

	for _, snap := range snaps {
		snapID := slugID(fmt.Sprintf("%s-%s-%s", repoID, snap.Env, snap.AppVersion))
		_, err := db.sql.Exec(
			`INSERT INTO chart_snapshots(id,repo_id,env,image_tag,app_version,captured_at,kube_context,namespace) VALUES(?,?,?,?,?,?,?,?)`,
			snapID, repoID, snap.Env, snap.ImageTag, snap.AppVersion, now, snap.KubeContext, snap.Namespace)
		if err != nil {
			return fmt.Errorf("insert chart_snapshot %q: %w", snap.AppVersion, err)
		}

		// Resources
		for _, r := range snap.Resources {
			rid := slugID(fmt.Sprintf("%s-%s", snapID, r.Container))
			db.sql.Exec(
				`INSERT INTO chart_resources(id,snapshot_id,repo_id,container,cpu_request,cpu_limit,mem_request,mem_limit,heap_size,replicas_min,replicas_max) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
				rid, snapID, repoID, r.Container, r.CPURequest, r.CPULimit, r.MemRequest, r.MemLimit, r.HeapSize, r.ReplicasMin, r.ReplicasMax)
		}

		// Env vars (non-secret)
		for i, e := range snap.EnvVars {
			eid := slugID(fmt.Sprintf("%s-env-%d-%s", snapID, i, e.Key))
			db.sql.Exec(
				`INSERT INTO chart_env_vars(id,snapshot_id,repo_id,key,value) VALUES(?,?,?,?,?)`,
				eid, snapID, repoID, e.Key, e.Value)
		}

		// Sidecars
		for _, s := range snap.Sidecars {
			sid := slugID(fmt.Sprintf("%s-sidecar-%s", snapID, s.Name))
			db.sql.Exec(
				`INSERT INTO chart_sidecars(id,snapshot_id,repo_id,name,image) VALUES(?,?,?,?,?)`,
				sid, snapID, repoID, s.Name, s.Image)
		}
	}
	return nil
}

// parseChartFile reads values.yaml, merges prod.yaml override (if present), and builds a ChartSnapshot.
func parseChartFile(repoName, repoPath, filePath string) (ChartSnapshot, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ChartSnapshot{}, err
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return ChartSnapshot{}, fmt.Errorf("parse %s: %w", filePath, err)
	}

	// Merge prod.yaml override: lives at sibling of chart/ directory
	prodPath := filepath.Join(filepath.Dir(filepath.Dir(filePath)), "prod.yaml")
	if prodData, err := os.ReadFile(prodPath); err == nil {
		var prodRaw map[string]interface{}
		if yaml.Unmarshal(prodData, &prodRaw) == nil {
			raw = deepMergeMaps(raw, prodRaw)
		}
	}

	snap := ChartSnapshot{Env: "prod"}
	snap.KubeContext, snap.Namespace = parseMakefileHelmContext(repoPath, prodPath)

	// Detect chart type and extract from the right top-level key
	if herbie, ok := raw["herbie-api-chart"].(map[string]interface{}); ok {
		parseHerbieChart(herbie, &snap)
	} else if alex, ok := raw["alex-job-chart"].(map[string]interface{}); ok {
		parseAlexJobChart(alex, &snap)
	} else {
		// Fallback: try to extract from any top-level map that has applicationName/jobName
		for _, v := range raw {
			if m, ok := v.(map[string]interface{}); ok {
				if _, hasApp := m["applicationName"]; hasApp {
					parseHerbieChart(m, &snap)
					break
				}
				if _, hasJob := m["jobName"]; hasJob {
					parseAlexJobChart(m, &snap)
					break
				}
			}
		}
	}

	// If app version still empty, derive from directory name
	if snap.AppVersion == "" {
		// e.g. .../fusca-api/deploy/helm/chart/values.yaml → fusca-api
		parts := strings.Split(filepath.ToSlash(filePath), "/")
		for i, p := range parts {
			if p == "deploy" && i > 0 {
				snap.AppVersion = parts[i-1]
				break
			}
		}
	}

	return snap, nil
}

// parseHerbieChart extracts fields from a herbie-api-chart values map.
func parseHerbieChart(m map[string]interface{}, snap *ChartSnapshot) {
	snap.AppVersion = stringField(m, "applicationName")
	snap.ImageTag = stringField(m, "appImageVersion")

	res := ChartResources{Container: snap.AppVersion}
	res.CPURequest = stringField(m, "appCpuRequest")
	res.CPULimit = stringField(m, "appCpuLimit")
	res.MemRequest = stringField(m, "appMemoryRequest")
	res.MemLimit = stringField(m, "appMemoryLimit")
	res.ReplicasMin = intField(m, "minReplicas")
	res.ReplicasMax = intField(m, "maxReplicas")

	// Extract env vars
	envVars := extractEnvVars(m, "appEnvironmentVariables")
	res.HeapSize = extractHeapSize(envVars)

	// Datadog config as env var
	if dd, ok := m["datadog"].(map[string]interface{}); ok {
		if trace, ok := dd["trace"].(map[string]interface{}); ok {
			envVars = append(envVars, ChartEnvVar{Key: "DD_TRACE_ENABLED", Value: fmt.Sprintf("%v", trace["enabled"])})
		}
		if prof, ok := dd["profiling"].(map[string]interface{}); ok {
			envVars = append(envVars, ChartEnvVar{Key: "DD_PROFILING_ENABLED", Value: fmt.Sprintf("%v", prof["enabled"])})
		}
	}

	if res.CPURequest != "" || res.CPULimit != "" || res.MemRequest != "" || res.MemLimit != "" {
		snap.Resources = append(snap.Resources, res)
	}
	snap.EnvVars = append(snap.EnvVars, envVars...)
}

// parseAlexJobChart extracts fields from an alex-job-chart values map (Flink jobs).
func parseAlexJobChart(m map[string]interface{}, snap *ChartSnapshot) {
	snap.AppVersion = stringField(m, "jobName")
	snap.ImageTag = stringField(m, "jobImageVersion")

	envVars := extractEnvVars(m, "jobEnvironmentVariables")
	snap.EnvVars = append(snap.EnvVars, envVars...)

	// Flink parallelism as env var
	if fc, ok := m["flinkConf"].(map[string]interface{}); ok {
		if par, ok := fc["parallelism"].(map[string]interface{}); ok {
			snap.EnvVars = append(snap.EnvVars, ChartEnvVar{Key: "FLINK_PARALLELISM", Value: fmt.Sprintf("%v", par["default"])})
		}
	}

	// Jobmanager resources
	if jm, ok := m["jobmanager"].(map[string]interface{}); ok {
		if res, ok := jm["resources"].(map[string]interface{}); ok {
			snap.Resources = append(snap.Resources, ChartResources{
				Container:  snap.AppVersion + "-jm",
				CPURequest: fmt.Sprintf("%v", res["cpu"]),
				CPULimit:   fmt.Sprintf("%v", res["cpuLimit"]),
				MemLimit:   fmt.Sprintf("%v", res["memory"]),
			})
		}
	}

	// Taskmanager resources
	if tm, ok := m["taskmanager"].(map[string]interface{}); ok {
		if res, ok := tm["resources"].(map[string]interface{}); ok {
			snap.Resources = append(snap.Resources, ChartResources{
				Container:  snap.AppVersion + "-tm",
				CPURequest: fmt.Sprintf("%v", res["cpu"]),
				CPULimit:   fmt.Sprintf("%v", res["cpuLimit"]),
				MemLimit:   fmt.Sprintf("%v", res["memory"]),
			})
		}
	}
}

// extractEnvVars reads appEnvironmentVariables/jobEnvironmentVariables, filters secrets.
func extractEnvVars(m map[string]interface{}, key string) []ChartEnvVar {
	raw, ok := m[key].(map[string]interface{})
	if !ok {
		return nil
	}
	var vars []ChartEnvVar
	for k, v := range raw {
		if isSecretKey(k) {
			continue
		}
		val := fmt.Sprintf("%v", v)
		// Skip SSM paths and empty
		if val == "" || strings.HasPrefix(val, "/cobli/") || strings.HasPrefix(val, "$(") {
			continue
		}
		vars = append(vars, ChartEnvVar{Key: k, Value: val})
	}
	return vars
}

// isSecretKey returns true if an env var key looks like a credential.
var secretPatterns = regexp.MustCompile(`(?i)(password|secret|token|apikey|api_key|credentials|passwd|private_key|client_secret|auth_key|access_key)`)

func isSecretKey(key string) bool {
	return secretPatterns.MatchString(key)
}

// extractHeapSize finds JVM/Node heap size from env var values.
var reXmx = regexp.MustCompile(`(?i)-Xmx(\S+)`)
var reMaxHeap = regexp.MustCompile(`(?i)-XX:MaxHeapSize=(\S+)`)
var reNodeHeap = regexp.MustCompile(`(?i)--max-old-space-size[= ](\d+)`)

func extractHeapSize(vars []ChartEnvVar) string {
	for _, v := range vars {
		if v.Key == "JAVA_OPTS" || v.Key == "JVM_OPTS" || v.Key == "JAVA_TOOL_OPTIONS" {
			if m := reXmx.FindStringSubmatch(v.Value); m != nil {
				return m[1]
			}
			if m := reMaxHeap.FindStringSubmatch(v.Value); m != nil {
				return m[1]
			}
		}
		if v.Key == "NODE_OPTIONS" {
			if m := reNodeHeap.FindStringSubmatch(v.Value); m != nil {
				return m[1] + "m"
			}
		}
	}
	return ""
}

// deepMergeMaps merges override into base recursively; override wins on all leaf conflicts.
func deepMergeMaps(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(base))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		if baseVal, ok := result[k]; ok {
			if baseMap, ok := baseVal.(map[string]interface{}); ok {
				if overMap, ok := v.(map[string]interface{}); ok {
					result[k] = deepMergeMaps(baseMap, overMap)
					continue
				}
			}
		}
		result[k] = v
	}
	return result
}

var reMakeAssign = regexp.MustCompile(`^([A-Z_][A-Z0-9_]*)\s*:?=\s*(.+)`)
var reMakeRef = regexp.MustCompile(`\$\(([A-Z_][A-Z0-9_]*)\)`)

// parseMakeVars extracts simple VAR := value assignments from a Makefile.
func parseMakeVars(content string) map[string]string {
	vars := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		if m := reMakeAssign.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			vars[m[1]] = strings.TrimSpace(m[2])
		}
	}
	return vars
}

// resolveMakeRef substitutes $(VAR) references using the provided vars map.
func resolveMakeRef(s string, vars map[string]string) string {
	return reMakeRef.ReplaceAllStringFunc(s, func(match string) string {
		name := match[2 : len(match)-1]
		if v, ok := vars[name]; ok {
			return v
		}
		return match
	})
}

// parseMakefileHelmContext reads the Makefile in repoPath and extracts --kube-context and
// --namespace from the helm upgrade target that references prodValuesPath.
func parseMakefileHelmContext(repoPath, prodValuesPath string) (kubeContext, namespace string) {
	// Candidate Makefile locations in priority order; pick the first one that
	// contains a reference to prodValuesPath (handles both single-deployment
	// repos and monorepos where each module has its own Makefile).
	moduleDir := filepath.Dir(filepath.Dir(filepath.Dir(prodValuesPath)))
	candidates := []string{
		filepath.Join(repoPath, "Makefile"),
		filepath.Join(moduleDir, "Makefile"),
		filepath.Join(filepath.Dir(repoPath), "Makefile"),
	}

	reKube := regexp.MustCompile(`--kube-context\s+(\S+)`)
	reNS := regexp.MustCompile(`--namespace\s+(\S+)`)

	for _, makePath := range candidates {
		data, err := os.ReadFile(makePath)
		if err != nil {
			continue
		}
		content := string(data)
		vars := parseMakeVars(content)

		relProd, _ := filepath.Rel(filepath.Dir(makePath), prodValuesPath)
		lines := strings.Split(content, "\n")

		anchor := -1
		for i, line := range lines {
			if strings.Contains(line, relProd) || strings.Contains(line, filepath.Base(prodValuesPath)) {
				anchor = i
				break
			}
		}
		if anchor < 0 {
			continue
		}

		lo, hi := anchor-20, anchor+20
		if lo < 0 {
			lo = 0
		}
		if hi > len(lines) {
			hi = len(lines)
		}
		for _, line := range lines[lo:hi] {
			if kubeContext == "" {
				if m := reKube.FindStringSubmatch(line); m != nil {
					kubeContext = resolveMakeRef(m[1], vars)
				}
			}
			if namespace == "" {
				if m := reNS.FindStringSubmatch(line); m != nil {
					namespace = resolveMakeRef(m[1], vars)
				}
			}
			if kubeContext != "" && namespace != "" {
				break
			}
		}
		if kubeContext != "" || namespace != "" {
			return
		}
	}
	return
}

func stringField(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func intField(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		var i int
		fmt.Sscanf(fmt.Sprintf("%v", v), "%d", &i)
		return i
	}
	return 0
}
