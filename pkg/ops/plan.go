package ops

import (
	"embed"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed config/plans/*.yml
var planTemplatesFS embed.FS

// Plan is the top-level YAML structure for an ops plan file.
type Plan struct {
	Plan PlanBody `yaml:"plan"`
}

// PlanBody holds the full plan definition.
type PlanBody struct {
	ID               string     `yaml:"id"`
	Scenario         string     `yaml:"scenario"`
	Context          string     `yaml:"context,omitempty"`
	Risk             string     `yaml:"risk"`             // low | medium | high
	CreatedAt        string     `yaml:"created_at"`
	Steps            []PlanStep `yaml:"steps"`
	ApprovalRequired []int      `yaml:"approval_required,omitempty"`
}

// PlanStep is one step inside a plan.
type PlanStep struct {
	ID        int    `yaml:"id"`
	Tool      string `yaml:"tool"`
	Purpose   string `yaml:"purpose"`
	Owner     string `yaml:"owner"`              // auto | human
	Rollback  string `yaml:"rollback,omitempty"` // command to undo this step
	ProceedIf string `yaml:"proceed_if,omitempty"`
	AbortIf   string `yaml:"abort_if,omitempty"`
}

// PlanTemplate is a pre-defined plan for a known scenario.
type PlanTemplate struct {
	ID          string
	Name        string
	Description string
	Risk        string
	Builder     func(vars map[string]string) PlanBody
}

// ── Built-in templates ────────────────────────────────────────────────────────

var builtinPlanTemplates = []PlanTemplate{
	{
		ID:          "health-check",
		Name:        "Full Health Check",
		Description: "Read-only check: auth → DB → k8s → Kafka. Zero mutations.",
		Risk:        "low",
		Builder:     buildHealthCheckTemplate,
	},
	{
		ID:          "lock-contention",
		Name:        "PostgreSQL Lock Contention Recovery",
		Description: "Identify and terminate root blocker. Requires human approval before kill.",
		Risk:        "high",
		Builder:     buildLockContentionTemplate,
	},
	{
		ID:          "kafka-recovery",
		Name:        "Kafka Epoch Conflict Recovery",
		Description: "Diagnose InvalidProducerEpochException and guide remediation.",
		Risk:        "high",
		Builder:     buildKafkaRecoveryTemplate,
	},
	{
		ID:          "deploy-rollback",
		Name:        "Kubernetes Deployment Rollback",
		Description: "Verify bad deploy and rollback to previous revision.",
		Risk:        "high",
		Builder:     buildDeployRollbackTemplate,
	},
}

func buildHealthCheckTemplate(v map[string]string) PlanBody {
	p := v["aws-profile"]
	if p == "" {
		p = "cobli-tech"
	}
	return PlanBody{
		Risk: "low",
		Steps: []PlanStep{
			{
				ID:      1,
				Tool:    "wtb ops auth --profile " + p,
				Purpose: "verificar token AWS SSO antes de iniciar",
				Owner:   "auto",
			},
			{
				ID:      2,
				Tool:    fmt.Sprintf("wtb ops db health --context %s --namespace %s --db-host %s --db-user %s --db-password-ssm %s", v["kubectl-context"], v["namespace"], v["db-host"], v["db-user"], v["ssm-path"]),
				Purpose: "estado do PostgreSQL: locks, dead tuples, WAL lag",
				Owner:   "auto",
			},
			{
				ID:      3,
				Tool:    fmt.Sprintf("wtb ops k8s status --context %s --namespace %s --deployment %s", v["kubectl-context"], v["namespace"], v["deployment"]),
				Purpose: "estado dos pods: readiness, restarts, hash",
				Owner:   "auto",
			},
			{
				ID:      4,
				Tool:    fmt.Sprintf("wtb ops kafka status --context %s --namespace %s --deployment %s --topic %s --window 30m", v["kubectl-context"], v["namespace"], v["consumer-deployment"], v["kafka-topic"]),
				Purpose: "Kafka consumer: erros epoch, lag, rebalances",
				Owner:   "auto",
			},
		},
	}
}

func buildLockContentionTemplate(v map[string]string) PlanBody {
	p := v["aws-profile"]
	if p == "" {
		p = "cobli-tech"
	}
	dbArgs := fmt.Sprintf("--context %s --namespace %s --db-host %s --db-user %s --db-password-ssm %s",
		v["kubectl-context"], v["namespace"], v["db-host"], v["db-user"], v["ssm-path"])
	return PlanBody{
		Risk: "high",
		Steps: []PlanStep{
			{
				ID:      1,
				Tool:    "wtb ops auth --profile " + p,
				Purpose: "verificar token AWS SSO",
				Owner:   "auto",
			},
			{
				ID:        2,
				Tool:      "wtb ops db health " + dbArgs,
				Purpose:   "confirmar locks e dimensão do problema",
				Owner:     "auto",
				ProceedIf: "data.locks > 0",
			},
			{
				ID:      3,
				Tool:    fmt.Sprintf("wtb ops k8s status --context %s --namespace %s --deployment %s", v["kubectl-context"], v["namespace"], v["deployment"]),
				Purpose: "verificar pods em crash loop — pode ser root cause dos locks",
				Owner:   "auto",
			},
			{
				ID:      4,
				Tool:    "# SELECT pid, query, wait_event, now()-query_start AS duration FROM pg_stat_activity WHERE wait_event_type='Lock' ORDER BY query_start LIMIT 20",
				Purpose: "identificar root blocker (rodar via psql/DBeaver, anotar <root_pid>)",
				Owner:   "human",
			},
			{
				ID:       5,
				Tool:     "# SELECT pg_terminate_backend(<root_pid>)",
				Purpose:  "terminar root blocker após aprovação — ação IRREVERSÍVEL",
				Owner:    "human",
				Rollback: "# não reversível — sessão já foi terminada",
			},
			{
				ID:      6,
				Tool:    "wtb ops db health " + dbArgs,
				Purpose: "confirmar que locks foram resolvidos",
				Owner:   "auto",
			},
		},
		ApprovalRequired: []int{5},
	}
}

func buildKafkaRecoveryTemplate(v map[string]string) PlanBody {
	p := v["aws-profile"]
	if p == "" {
		p = "cobli-tech"
	}
	kafkaArgs := fmt.Sprintf("--context %s --namespace %s --deployment %s --topic %s",
		v["kubectl-context"], v["namespace"], v["consumer-deployment"], v["kafka-topic"])
	return PlanBody{
		Risk: "high",
		Steps: []PlanStep{
			{
				ID:      1,
				Tool:    "wtb ops auth --profile " + p,
				Purpose: "verificar token AWS SSO",
				Owner:   "auto",
			},
			{
				ID:      2,
				Tool:    "wtb ops kafka status " + kafkaArgs + " --window 2h",
				Purpose: "verificar erros epoch e timeline nas últimas 2h",
				Owner:   "auto",
			},
			{
				ID:      3,
				Tool:    fmt.Sprintf("wtb ops k8s status --context %s --namespace %s --deployment %s", v["kubectl-context"], v["namespace"], v["deployment"]),
				Purpose: "verificar se restarts recentes do producer/consumer coincide com o spike",
				Owner:   "auto",
			},
			{
				ID:      4,
				Tool:    "# revert do deploy do producer (git revert + push) OU reiniciar consumer group via CLI Kafka",
				Purpose: "resolver epoch conflict: revert ou reset de transactional.id",
				Owner:   "human",
			},
			{
				ID:      5,
				Tool:    "wtb ops kafka status " + kafkaArgs + " --window 10m",
				Purpose: "confirmar zero erros e lag estável após intervenção",
				Owner:   "auto",
			},
		},
		ApprovalRequired: []int{4},
	}
}

func buildDeployRollbackTemplate(v map[string]string) PlanBody {
	p := v["aws-profile"]
	if p == "" {
		p = "cobli-tech"
	}
	k8sArgs := fmt.Sprintf("--context %s --namespace %s --deployment %s",
		v["kubectl-context"], v["namespace"], v["deployment"])
	return PlanBody{
		Risk: "high",
		Steps: []PlanStep{
			{
				ID:      1,
				Tool:    "wtb ops auth --profile " + p,
				Purpose: "verificar token AWS SSO",
				Owner:   "auto",
			},
			{
				ID:      2,
				Tool:    "wtb ops k8s status " + k8sArgs,
				Purpose: "confirmar estado atual e hash do deploy problemático",
				Owner:   "auto",
			},
			{
				ID:      3,
				Tool:    fmt.Sprintf("# kubectl rollout undo deployment/%s -n %s --context %s", v["deployment"], v["namespace"], v["kubectl-context"]),
				Purpose: "rollback para revisão anterior — ação IRREVERSÍVEL sem novo deploy",
				Owner:   "human",
				Rollback: fmt.Sprintf("# kubectl rollout undo deployment/%s -n %s --context %s  # reverte o rollback", v["deployment"], v["namespace"], v["kubectl-context"]),
			},
			{
				ID:      4,
				Tool:    "wtb ops k8s status " + k8sArgs,
				Purpose: "confirmar pods saudáveis após rollback",
				Owner:   "auto",
			},
		},
		ApprovalRequired: []int{3},
	}
}

// ── Plan construction ─────────────────────────────────────────────────────────

// NewPlanFromTemplate builds a Plan from a known template ID with variable substitution.
// vars maps placeholder names (e.g. "kubectl-context") to concrete values.
func NewPlanFromTemplate(templateID, contextNote string, vars map[string]string) (Plan, bool) {
	tmpl, ok := findTemplate(templateID)
	if !ok {
		return Plan{}, false
	}
	body := tmpl.Builder(vars)
	body.ID = planID()
	body.Scenario = tmpl.Name
	body.Context = contextNote
	body.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	if body.Risk == "" {
		body.Risk = tmpl.Risk
	}
	return Plan{Plan: body}, true
}

// NewBlankPlan creates an empty plan skeleton for a custom scenario.
func NewBlankPlan(scenario, contextNote, risk string) Plan {
	return Plan{
		Plan: PlanBody{
			ID:        planID(),
			Scenario:  scenario,
			Context:   contextNote,
			Risk:      risk,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
			Steps: []PlanStep{
				{
					ID:      1,
					Tool:    "wtb ops auth --profile cobli-tech",
					Purpose: "verificar token AWS SSO antes de iniciar",
					Owner:   "auto",
				},
				{
					ID:      2,
					Tool:    "# adicionar próximos passos",
					Purpose: "descrever propósito",
					Owner:   "human",
				},
			},
		},
	}
}

// MarshalPlan serialises a Plan to YAML bytes.
func MarshalPlan(p Plan) ([]byte, error) {
	return yaml.Marshal(p)
}

// UnmarshalPlan deserialises YAML bytes into a Plan.
func UnmarshalPlan(data []byte) (Plan, error) {
	var p Plan
	if err := yaml.Unmarshal(data, &p); err != nil {
		return Plan{}, err
	}
	return p, nil
}

// ListTemplates returns all built-in plan templates.
func ListTemplates() []PlanTemplate {
	return builtinPlanTemplates
}

// SubstituteVars replaces <placeholder> tokens in a plan's tool strings.
func SubstituteVars(p Plan, vars map[string]string) Plan {
	for i, step := range p.Plan.Steps {
		for k, v := range vars {
			step.Tool = strings.ReplaceAll(step.Tool, "<"+k+">", v)
			step.Rollback = strings.ReplaceAll(step.Rollback, "<"+k+">", v)
		}
		p.Plan.Steps[i] = step
	}
	return p
}

// ── YAML-driven plan templates ────────────────────────────────────────────────

// planStepYAML mirrors PlanStep for YAML unmarshalling with template variables.
type planStepYAML struct {
	ID        int    `yaml:"id"`
	Tool      string `yaml:"tool"`
	Purpose   string `yaml:"purpose"`
	Owner     string `yaml:"owner"`
	Rollback  string `yaml:"rollback,omitempty"`
	ProceedIf string `yaml:"proceed_if,omitempty"`
	AbortIf   string `yaml:"abort_if,omitempty"`
}

// planTemplateYAML is the schema for config/plans/*.yml files.
// Vars defines default values keyed with underscores (e.g. aws_profile).
// Caller vars use hyphens (e.g. aws-profile) and are normalised on load.
type planTemplateYAML struct {
	ID               string            `yaml:"id"`
	Name             string            `yaml:"name"`
	Description      string            `yaml:"description"`
	Risk             string            `yaml:"risk"`
	ApprovalRequired []int             `yaml:"approval_required"`
	Vars             map[string]string `yaml:"vars"`
	Steps            []planStepYAML    `yaml:"steps"`
}

func init() {
	builtinPlanTemplates = append(builtinPlanTemplates, loadYAMLPlanTemplates()...)
}

func loadYAMLPlanTemplates() []PlanTemplate {
	entries, err := planTemplatesFS.ReadDir("config/plans")
	if err != nil {
		return nil
	}
	var out []PlanTemplate
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}
		data, err := planTemplatesFS.ReadFile("config/plans/" + entry.Name())
		if err != nil {
			continue
		}
		var tmpl planTemplateYAML
		if err := yaml.Unmarshal(data, &tmpl); err != nil {
			continue
		}
		t := tmpl // capture for closure
		out = append(out, PlanTemplate{
			ID:          t.ID,
			Name:        t.Name,
			Description: t.Description,
			Risk:        t.Risk,
			Builder: func(vars map[string]string) PlanBody {
				return buildFromYAMLTemplate(t, vars)
			},
		})
	}
	return out
}

// buildFromYAMLTemplate applies caller vars (hyphen keys) over YAML defaults
// (underscore keys) and interpolates {{.key}} tokens in each step string.
func buildFromYAMLTemplate(tmpl planTemplateYAML, vars map[string]string) PlanBody {
	merged := make(map[string]string, len(tmpl.Vars)+len(vars))
	for k, v := range tmpl.Vars {
		merged[k] = v
	}
	for k, v := range vars {
		merged[strings.ReplaceAll(k, "-", "_")] = v
	}
	steps := make([]PlanStep, len(tmpl.Steps))
	for i, s := range tmpl.Steps {
		steps[i] = PlanStep{
			ID:        s.ID,
			Tool:      interpolateVars(s.Tool, merged),
			Purpose:   interpolateVars(s.Purpose, merged),
			Owner:     s.Owner,
			Rollback:  interpolateVars(s.Rollback, merged),
			ProceedIf: interpolateVars(s.ProceedIf, merged),
			AbortIf:   interpolateVars(s.AbortIf, merged),
		}
	}
	return PlanBody{
		Risk:             tmpl.Risk,
		Steps:            steps,
		ApprovalRequired: tmpl.ApprovalRequired,
	}
}

// interpolateVars replaces {{.key}} tokens with values from vars.
func interpolateVars(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{{."+k+"}}", v)
	}
	return s
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func findTemplate(id string) (PlanTemplate, bool) {
	for _, t := range builtinPlanTemplates {
		if t.ID == id {
			return t, true
		}
	}
	return PlanTemplate{}, false
}

func planID() string {
	return "plan-" + time.Now().UTC().Format("20060102-150405")
}

// PlanSummary returns a human-readable text representation of a plan.
func PlanSummary(p Plan) string {
	b := p.Plan
	var sb strings.Builder

	riskIcon := map[string]string{"low": "🟢", "medium": "🟡", "high": "🔴"}[b.Risk]
	if riskIcon == "" {
		riskIcon = "⚪"
	}

	sb.WriteString(fmt.Sprintf("Plan: %s  %s risk:%s\n", b.ID, riskIcon, b.Risk))
	sb.WriteString(fmt.Sprintf("Scenario: %s\n", b.Scenario))
	if b.Context != "" {
		sb.WriteString(fmt.Sprintf("Context: %s\n", b.Context))
	}
	sb.WriteString(fmt.Sprintf("Created: %s\n", b.CreatedAt))
	if len(b.ApprovalRequired) > 0 {
		sb.WriteString(fmt.Sprintf("Approval required for steps: %v\n", b.ApprovalRequired))
	}
	sb.WriteString(fmt.Sprintf("\nSteps (%d):\n", len(b.Steps)))

	for _, step := range b.Steps {
		ownerIcon := "🤖"
		if step.Owner == "human" {
			ownerIcon = "👤"
		}
		sb.WriteString(fmt.Sprintf("\n  Step %d %s [%s]\n", step.ID, ownerIcon, step.Owner))
		sb.WriteString(fmt.Sprintf("  Purpose: %s\n", step.Purpose))
		sb.WriteString(fmt.Sprintf("  Tool:    %s\n", step.Tool))
		if step.ProceedIf != "" {
			sb.WriteString(fmt.Sprintf("  proceed_if: %s\n", step.ProceedIf))
		}
		if step.AbortIf != "" {
			sb.WriteString(fmt.Sprintf("  abort_if:   %s\n", step.AbortIf))
		}
		if step.Rollback != "" {
			sb.WriteString(fmt.Sprintf("  rollback:   %s\n", step.Rollback))
		}
	}
	return sb.String()
}
