// Package repoindex indexes codebase entities (handlers, models, APIs, events)
// into repos.db via LLM extraction, enabling SQL-based cross-referencing for
// refactor planning.
package repoindex

// Repo represents an indexed repository.
type Repo struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Path               string `json:"path"`
	Lang               string `json:"lang"`
	Framework          string `json:"framework"`
	Owner              string `json:"owner,omitempty"`
	LastIndexedAt      string `json:"last_indexed_at,omitempty"`
	DDServiceName      string `json:"dd_service_name,omitempty"`
	PrimaryHostname    string `json:"primary_hostname,omitempty"`
	SecondaryLang      string `json:"secondary_lang,omitempty"`
	SecondaryFramework string `json:"secondary_framework,omitempty"`
	CIPlatform         string `json:"ci_platform,omitempty"`
}

// Handler represents a Lambda function, HTTP endpoint, or job entry point.
type Handler struct {
	ID            string `json:"id"`
	RepoID        string `json:"repo_id"`
	Name          string `json:"name"`
	HandlerFile   string `json:"handler_file,omitempty"`
	TriggerType   string `json:"trigger_type,omitempty"`  // eventbridge, s3, schedule, http
	TriggerDetail string `json:"trigger_detail,omitempty"` // cron expr, event pattern, prefix
	Timeout       int    `json:"timeout,omitempty"`
	MaxRetry      int    `json:"max_retry,omitempty"`
	Concurrency   int    `json:"concurrency,omitempty"` // 0 = unlimited
	VPC           bool   `json:"vpc,omitempty"`
	Description   string `json:"description,omitempty"`
}

// Event represents an asynchronous event produced or consumed by the repo.
type Event struct {
	ID          string `json:"id"`
	RepoID      string `json:"repo_id"`
	Name        string `json:"name"`
	EventType   string `json:"event_type,omitempty"`  // eventbridge, s3, schedule
	DetailType  string `json:"detail_type,omitempty"` // EventBridge detail-type value
	BusName     string `json:"bus_name,omitempty"`
	Description string `json:"description,omitempty"`
}

// Model represents a database model/entity.
type Model struct {
	ID          string       `json:"id"`
	RepoID      string       `json:"repo_id"`
	Name        string       `json:"name"`
	TableName   string       `json:"table_name,omitempty"`
	Dialect     string       `json:"dialect,omitempty"`
	Fields      []ModelField `json:"fields,omitempty"`
	Associations []ModelAssociation `json:"associations,omitempty"`
}

// ModelField represents a single field in a model.
type ModelField struct {
	ID         string `json:"id"`
	ModelID    string `json:"model_id"`
	Name       string `json:"name"`
	Type       string `json:"type,omitempty"`
	Nullable   bool   `json:"nullable"`
	PrimaryKey bool   `json:"primary_key"`
	Unique     bool   `json:"unique"`
}

// ModelAssociation represents a relationship between models.
type ModelAssociation struct {
	ID          string `json:"id"`
	ModelID     string `json:"model_id"`
	AssocType   string `json:"assoc_type,omitempty"`   // belongsTo, hasMany, belongsToMany
	TargetModel string `json:"target_model,omitempty"`
	ForeignKey  string `json:"foreign_key,omitempty"`
}

// ExternalAPI represents an external HTTP API the repo calls.
type ExternalAPI struct {
	ID          string `json:"id"`
	RepoID      string `json:"repo_id"`
	Name        string `json:"name"`
	URL         string `json:"url,omitempty"`
	Method      string `json:"method,omitempty"`
	AuthType    string `json:"auth_type,omitempty"` // oauth2, basic, bearer, api-key, none
	Description string `json:"description,omitempty"`
}

// DBConnection represents a database connection configuration.
type DBConnection struct {
	ID       string `json:"id"`
	RepoID   string `json:"repo_id"`
	Dialect  string `json:"dialect,omitempty"` // postgres, mysql, sqlite
	HostVar  string `json:"host_var,omitempty"`
	PoolMin  int    `json:"pool_min,omitempty"`
	PoolMax  int    `json:"pool_max,omitempty"`
	PoolIdle int    `json:"pool_idle,omitempty"`
}

// ConfigVar represents an environment variable or SSM parameter used by the repo.
type ConfigVar struct {
	ID          string `json:"id"`
	RepoID      string `json:"repo_id"`
	Key         string `json:"key"`
	Source      string `json:"source,omitempty"`      // ssm, env, hardcoded
	Description string `json:"description,omitempty"`
}

// Note is a free-text annotation attached to any entity.
type Note struct {
	ID         string `json:"id"`
	EntityType string `json:"entity_type"` // handler, model, api, event, repo
	EntityID   string `json:"entity_id"`
	RepoID     string `json:"repo_id"`
	Content    string `json:"content"`
	CreatedAt  string `json:"created_at"`
}

// ServiceMetric is a Datadog metric observed for a repo, categorized for incident triage.
type ServiceMetric struct {
	ID         string `json:"id"`
	RepoID     string `json:"repo_id"`
	MetricName string `json:"metric_name"`
	Category   string `json:"category"` // business, apm, middleware, kafka, flink, jvm
	FetchedAt  string `json:"fetched_at"`
}

// RepoSnapshot is the full indexed state of a repo — returned by Show.
type RepoSnapshot struct {
	Repo              Repo              `json:"repo"`
	Handlers          []Handler         `json:"handlers"`
	Events            []Event           `json:"events"`
	Models            []Model           `json:"models"`
	ExternalAPIs      []ExternalAPI     `json:"external_apis"`
	DBConnections     []DBConnection    `json:"db_connections"`
	ConfigVars        []ConfigVar       `json:"config_vars"`
	Notes             []Note            `json:"notes,omitempty"`
	DeploymentUnits   []DeploymentUnit  `json:"deployment_units,omitempty"`
	TopicEnrichments  []TopicEnrichment `json:"topic_enrichments,omitempty"`
	DDMonitors        []DDMonitor       `json:"dd_monitors,omitempty"`
	ChartSnapshots    []ChartSnapshot   `json:"chart_snapshots,omitempty"`
	ServiceMetrics    []ServiceMetric   `json:"service_metrics,omitempty"`
}

// ExtractedLayer is what the LLM returns for a single layer of a repo.
type ExtractedLayer struct {
	Handlers      []Handler      `json:"handlers,omitempty"`
	Events        []Event        `json:"events,omitempty"`
	Models        []Model        `json:"models,omitempty"`
	ExternalAPIs  []ExternalAPI  `json:"external_apis,omitempty"`
	DBConnections []DBConnection `json:"db_connections,omitempty"`
	ConfigVars    []ConfigVar    `json:"config_vars,omitempty"`
}

// Layer is a named group of files fed to the LLM in a single call.
type Layer struct {
	Name  string   // infra, models, handlers, apis
	Files []string // absolute paths
}

// DeploymentUnit represents a named deployment inside a repo (e.g. fusca-api, fusca-identification-token).
type DeploymentUnit struct {
	ID            string `json:"id"`
	RepoID        string `json:"repo_id"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
	ReplicasMin   int    `json:"replicas_min,omitempty"`
	ReplicasMax   int    `json:"replicas_max,omitempty"`
	ConsumerGroup string `json:"consumer_group,omitempty"`
	Deprecated    bool   `json:"deprecated,omitempty"`
}

// TopicEnrichment holds architecture-derived metadata for a Kafka topic used by a repo.
type TopicEnrichment struct {
	ID             string `json:"id"`
	RepoID         string `json:"repo_id"`
	DeploymentUnit string `json:"deployment_unit,omitempty"`
	Topic          string `json:"topic"`
	Direction      string `json:"direction"`               // "consumes" | "produces"
	Serialization  string `json:"serialization,omitempty"` // "json" | "protobuf" | "avro" | ""
	ConsumerGroup  string `json:"consumer_group,omitempty"`
	KeyDescription string `json:"key_description,omitempty"`
}

// DDMonitor holds a cached Datadog monitor reference for a repo.
type DDMonitor struct {
	ID        string `json:"id"`
	RepoID    string `json:"repo_id"`
	MonitorID string `json:"monitor_id"`
	Name      string `json:"name,omitempty"`
	Type      string `json:"type,omitempty"`
	Status    string `json:"status,omitempty"`
	URL       string `json:"url,omitempty"`
	FetchedAt string `json:"fetched_at,omitempty"`
}

// ChartSnapshot holds the Helm chart state for a repo at a point in time.
type ChartSnapshot struct {
	ID          string `json:"id"`
	RepoID      string `json:"repo_id"`
	Env         string `json:"env"`
	ImageTag    string `json:"image_tag,omitempty"`
	AppVersion  string `json:"app_version,omitempty"`
	CapturedAt  string `json:"captured_at,omitempty"`
	KubeContext string `json:"kube_context,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	Sidecars    []ChartSidecar   `json:"sidecars,omitempty"`
	Resources   []ChartResources `json:"resources,omitempty"`
	EnvVars     []ChartEnvVar    `json:"env_vars,omitempty"`
}

// ChartSidecar is a sidecar container found in a Helm chart.
type ChartSidecar struct {
	ID         string `json:"id"`
	SnapshotID string `json:"snapshot_id"`
	RepoID     string `json:"repo_id"`
	Name       string `json:"name"`
	Image      string `json:"image,omitempty"`
}

// ChartResources holds container resource limits from a Helm chart.
type ChartResources struct {
	ID          string `json:"id"`
	SnapshotID  string `json:"snapshot_id"`
	RepoID      string `json:"repo_id"`
	Container   string `json:"container,omitempty"`
	CPURequest  string `json:"cpu_request,omitempty"`
	CPULimit    string `json:"cpu_limit,omitempty"`
	MemRequest  string `json:"mem_request,omitempty"`
	MemLimit    string `json:"mem_limit,omitempty"`
	HeapSize    string `json:"heap_size,omitempty"`
	ReplicasMin int    `json:"replicas_min,omitempty"`
	ReplicasMax int    `json:"replicas_max,omitempty"`
}

// ChartEnvVar is a non-secret environment variable from a Helm chart.
type ChartEnvVar struct {
	ID         string `json:"id"`
	SnapshotID string `json:"snapshot_id"`
	RepoID     string `json:"repo_id"`
	Key        string `json:"key"`
	Value      string `json:"value,omitempty"`
}

// ArchSummary holds the enriched architecture data parsed from a summary.md file.
type ArchSummary struct {
	RepoName        string
	DDServiceName   string
	PrimaryHostname string
	Units           []DeploymentUnit
	Topics          []TopicEnrichment
}
