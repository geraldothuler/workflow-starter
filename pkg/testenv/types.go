package testenv

// TestEnvResult is the output contract for the testenv runner.
type TestEnvResult struct {
	Status     string          // "ok" | "error" | "timeout" | "skipped"
	Signal     string          // human-readable summary
	Runtime    string          // "docker-desktop" | "colima" | ""
	ComposeFile string         // resolved docker-compose path
	Services   []ServiceHealth // per-service health
	TestOutput string          // captured test stdout+stderr
	ExitCode   int             // test command exit code
	Duration   string          // total elapsed time
	Actions    []string        // suggested next steps on failure
}

// ServiceHealth captures health state for one docker-compose service.
type ServiceHealth struct {
	Name    string
	Port    int
	Status  string // "healthy" | "unhealthy" | "timeout"
	Elapsed string
}

// TestEnvConfig is the global embedded config (test_env.yml).
type TestEnvConfig struct {
	Runtimes []RuntimeConfig `yaml:"runtimes"`
	Compose  ComposeConfig   `yaml:"compose"`
	Health   HealthConfig    `yaml:"health"`
	Port     PortConfig      `yaml:"port_conflict"`
}

type RuntimeConfig struct {
	Name         string `yaml:"name"`
	CheckCmd     string `yaml:"check_cmd"`
	CheckPattern string `yaml:"check_pattern"`
	InstallURL   string `yaml:"install_url"`
}

type ComposeConfig struct {
	Discovery []string `yaml:"discovery"`
}

type HealthConfig struct {
	Retries         int `yaml:"retries"`
	IntervalSeconds int `yaml:"interval_seconds"`
	TimeoutSeconds  int `yaml:"timeout_seconds"`
}

type PortConfig struct {
	AutoStop bool `yaml:"auto_stop"`
}

// RepoTestEnvConfig is the per-repo config (.workflow/testenv.yml).
type RepoTestEnvConfig struct {
	Services []ServiceConfig `yaml:"services"`
	Compose  RepoCompose     `yaml:"compose"`
	Tests    TestsConfig     `yaml:"tests"`
}

type ServiceConfig struct {
	Name        string `yaml:"name"`
	Port        int    `yaml:"port"`
	HealthCheck string `yaml:"health_check"` // "tcp" (default)
}

type RepoCompose struct {
	File string `yaml:"file"` // override; empty = auto-discover
}

type TestsConfig struct {
	Command        string            `yaml:"command"`
	TimeoutSeconds int               `yaml:"timeout_seconds"`
	Env            map[string]string `yaml:"env"`
}
