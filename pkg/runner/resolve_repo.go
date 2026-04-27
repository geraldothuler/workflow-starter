package runner

import (
	repocontext "github.com/Cobliteam/workflow-toolkit/pkg/context"
)

// RepoResolver infers well-known input values from repository markers via pkg/context.
type RepoResolver struct{}

func (r *RepoResolver) Name() string { return "repo" }

func (r *RepoResolver) Resolve(spec InputSpec, inputs RunInputs, ctx ResolveContext) (string, bool) {
	if ctx.RepoPath == "" {
		return "", false
	}

	stack, err := repocontext.DetectStack(ctx.RepoPath)
	if err != nil {
		return "", false
	}

	// Infer well-known inputs from detected stack
	switch spec.Name {
	case "db-name":
		if containsAny(stack.Database, "PostgreSQL", "Postgres") {
			return "postgres", true
		}
	case "db-port":
		if containsAny(stack.Database, "PostgreSQL", "Postgres") {
			return "5432", true
		}
	case "db-user":
		if containsAny(stack.Database, "PostgreSQL", "Postgres") {
			return "postgres", true
		}
	}

	return "", false
}

func containsAny(slice []string, values ...string) bool {
	for _, s := range slice {
		for _, v := range values {
			if s == v {
				return true
			}
		}
	}
	return false
}
