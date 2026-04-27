package runner

import "testing"

// ── ResolveMappingTemplate ─────────────────────────────────────────────────

func TestResolveMappingTemplate_SimpleKey(t *testing.T) {
	inputs := RunInputs{"namespace": "production"}
	got := ResolveMappingTemplate("${namespace}", inputs)
	if got != "production" {
		t.Errorf("expected 'production', got %q", got)
	}
}

func TestResolveMappingTemplate_LiteralDefault(t *testing.T) {
	got := ResolveMappingTemplate("${profile:-cobli-tech}", RunInputs{})
	if got != "cobli-tech" {
		t.Errorf("expected 'cobli-tech', got %q", got)
	}
}

func TestResolveMappingTemplate_CallerWinsOverDefault(t *testing.T) {
	inputs := RunInputs{"profile": "my-profile"}
	got := ResolveMappingTemplate("${profile:-cobli-tech}", inputs)
	if got != "my-profile" {
		t.Errorf("expected 'my-profile', got %q", got)
	}
}

func TestResolveMappingTemplate_NestedDefault(t *testing.T) {
	// ${namespace:-${environment}} → falls back to environment value
	inputs := RunInputs{"environment": "staging"}
	got := ResolveMappingTemplate("${namespace:-${environment}}", inputs)
	if got != "staging" {
		t.Errorf("expected 'staging', got %q", got)
	}
}

func TestResolveMappingTemplate_NestedDefault_CallerWins(t *testing.T) {
	inputs := RunInputs{"namespace": "prod-ns", "environment": "production"}
	got := ResolveMappingTemplate("${namespace:-${environment}}", inputs)
	if got != "prod-ns" {
		t.Errorf("expected 'prod-ns', got %q", got)
	}
}

func TestResolveMappingTemplate_MissingKey_ReturnsEmpty(t *testing.T) {
	got := ResolveMappingTemplate("${nonexistent}", RunInputs{})
	if got != "" {
		t.Errorf("expected empty string for missing key, got %q", got)
	}
}

func TestResolveMappingTemplate_PlainString_Unchanged(t *testing.T) {
	got := ResolveMappingTemplate("no-template-here", RunInputs{})
	if got != "no-template-here" {
		t.Errorf("expected plain string unchanged, got %q", got)
	}
}

// ── ResolveInputs ─────────────────────────────────────────────────────────

func TestResolveInputs_NoMapping_ReturnsSameMap(t *testing.T) {
	inputs := RunInputs{"k": "v"}
	got := ResolveInputs(nil, inputs)
	if got["k"] != "v" {
		t.Errorf("expected original inputs preserved, got %v", got)
	}
}

func TestResolveInputs_AppliesDefault(t *testing.T) {
	mapping := map[string]string{"profile": "${profile:-cobli-tech}"}
	got := ResolveInputs(mapping, RunInputs{})
	if got["profile"] != "cobli-tech" {
		t.Errorf("expected 'cobli-tech', got %q", got["profile"])
	}
}

func TestResolveInputs_CallerWins(t *testing.T) {
	mapping := map[string]string{"profile": "${profile:-cobli-tech}"}
	inputs := RunInputs{"profile": "my-profile"}
	got := ResolveInputs(mapping, inputs)
	if got["profile"] != "my-profile" {
		t.Errorf("caller value should win over mapping, got %q", got["profile"])
	}
}

func TestResolveInputs_NestedAlias(t *testing.T) {
	mapping := map[string]string{"namespace": "${namespace:-${environment}}"}
	inputs := RunInputs{"environment": "production"}
	got := ResolveInputs(mapping, inputs)
	if got["namespace"] != "production" {
		t.Errorf("expected 'production' from alias, got %q", got["namespace"])
	}
}

func TestResolveInputs_PreservesExistingKeys(t *testing.T) {
	mapping := map[string]string{"extra": "default-value"}
	inputs := RunInputs{"foo": "bar"}
	got := ResolveInputs(mapping, inputs)
	if got["foo"] != "bar" {
		t.Errorf("existing keys should be preserved, got %v", got)
	}
	if got["extra"] != "default-value" {
		t.Errorf("expected default-value for extra, got %q", got["extra"])
	}
}
