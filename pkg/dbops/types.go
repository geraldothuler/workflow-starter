package dbops

// ColumnMeta describes a column in a query result.
type ColumnMeta struct {
	Name string `json:"name" yaml:"name"`
	Desc string `json:"desc,omitempty" yaml:"desc,omitempty"`
}

// ParamDef describes a named parameter in a query template.
type ParamDef struct {
	Name    string `yaml:"name"`
	Default string `yaml:"default,omitempty"`
	Desc    string `yaml:"desc,omitempty"`
}

// NamedQuery is a single query entry in a repo YAML file.
type NamedQuery struct {
	Name    string       `yaml:"name"`
	Desc    string       `yaml:"desc"`
	Driver  string       `yaml:"driver"` // postgres | cassandra | snowflake
	SQL     string       `yaml:"sql"`
	Params  []ParamDef   `yaml:"params,omitempty"`
	Columns []ColumnMeta `yaml:"columns,omitempty"`
}

// RepoQueries is the top-level structure of a db-queries/<repo>.yml file.
type RepoQueries struct {
	Repo      string       `yaml:"repo"`
	Namespace string       `yaml:"namespace"`
	Context   string       `yaml:"context,omitempty"`
	Queries   []NamedQuery `yaml:"queries"`
}

// QueryResult is the envelope returned by every query execution.
// Designed to be LLM-friendly: includes full context alongside the data.
type QueryResult struct {
	Repo      string           `json:"repo"`
	Query     string           `json:"query"`
	Driver    string           `json:"driver"`
	Columns   []ColumnMeta     `json:"columns"`
	Rows      []map[string]any `json:"rows"`
	Count     int              `json:"count"`
	ElapsedMs int64            `json:"elapsed_ms"`
}

// DBCredentials holds resolved connection credentials for a single driver.
type DBCredentials struct {
	// PostgreSQL
	Host     string
	Port     string
	Database string
	User     string
	Password string
	// Cassandra/Scylla
	ContactPoints string
	Keyspace      string
	Datacenter    string
}
