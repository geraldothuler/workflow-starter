package infracontext

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTechMapper_ExtractTechFromImage(t *testing.T) {
	mapper := NewTechMapperFromMap(map[string]string{
		"postgres":      "PostgreSQL",
		"redis":         "Redis",
		"mongo":         "MongoDB",
		"mysql":         "MySQL",
		"elasticsearch": "Elasticsearch",
		"kafka":         "Kafka",
		"rabbitmq":      "RabbitMQ",
		"nginx":         "Nginx",
		"envoy":         "Envoy",
		"grafana":       "Grafana",
		"prometheus":    "Prometheus",
	})

	tests := []struct {
		image string
		want  string
	}{
		{"postgres:15.4", "PostgreSQL"},
		{"docker.io/library/postgres:15", "PostgreSQL"},
		{"redis:7.2", "Redis"},
		{"bitnami/redis:7.2", "Redis"},
		{"mongo:6.0", "MongoDB"},
		{"mysql:8.0", "MySQL"},
		{"nginx:1.25", "Nginx"},
		{"elasticsearch:8.11", "Elasticsearch"},
		{"confluentinc/kafka:7.5", "Kafka"},
		{"myregistry/myapp:v1.0", ""},
		{"gcr.io/project/custom-service:latest", ""},
		{"", ""},
		{"grafana:10.0", "Grafana"},
		{"prom/prometheus:v2.48", "Prometheus"},
	}

	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			got := mapper.ExtractTechFromImage(tt.image)
			if got != tt.want {
				t.Errorf("ExtractTechFromImage(%q) = %q, want %q", tt.image, got, tt.want)
			}
		})
	}
}

func TestTechMapper_PrefixMatch(t *testing.T) {
	mapper := NewTechMapperFromMap(map[string]string{
		"postgres": "PostgreSQL",
	})

	got := mapper.ExtractTechFromImage("postgresql:15")
	if got != "PostgreSQL" {
		t.Errorf("ExtractTechFromImage(postgresql:15) = %q, want PostgreSQL", got)
	}
}

func TestTechMapper_Mapping(t *testing.T) {
	original := map[string]string{
		"postgres": "PostgreSQL",
		"redis":    "Redis",
	}
	mapper := NewTechMapperFromMap(original)

	mapping := mapper.Mapping()
	if len(mapping) != 2 {
		t.Errorf("expected 2 entries, got %d", len(mapping))
	}
	if mapping["postgres"] != "PostgreSQL" {
		t.Errorf("postgres = %q, want PostgreSQL", mapping["postgres"])
	}

	// Verify it's a copy
	mapping["postgres"] = "PG"
	if mapper.Mapping()["postgres"] != "PostgreSQL" {
		t.Error("Mapping() should return a copy")
	}
}

func TestNewTechMapper_Embedded(t *testing.T) {
	mapper, err := NewTechMapper("")
	if err != nil {
		t.Fatalf("failed to create tech mapper: %v", err)
	}

	if tech := mapper.ExtractTechFromImage("postgres:15"); tech != "PostgreSQL" {
		t.Errorf("postgres -> %q, want PostgreSQL", tech)
	}
	if tech := mapper.ExtractTechFromImage("redis:7"); tech != "Redis" {
		t.Errorf("redis -> %q, want Redis", tech)
	}
	if tech := mapper.ExtractTechFromImage("nginx:1.25"); tech != "Nginx" {
		t.Errorf("nginx -> %q, want Nginx", tech)
	}
}

func TestNewTechMapper_WithOverride(t *testing.T) {
	dir := t.TempDir()
	overrideDir := filepath.Join(dir, ".workflow", "infra-providers")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatal(err)
	}
	overrideContent := []byte(`tech_mapping:
  custom-db: "CustomDB"
  postgres: "PG-Override"
`)
	if err := os.WriteFile(filepath.Join(overrideDir, "tech_mapping.yml"), overrideContent, 0644); err != nil {
		t.Fatal(err)
	}

	mapper, err := NewTechMapper(dir)
	if err != nil {
		t.Fatalf("failed to create tech mapper: %v", err)
	}

	// Override should replace
	if tech := mapper.ExtractTechFromImage("postgres:15"); tech != "PG-Override" {
		t.Errorf("postgres -> %q, want PG-Override", tech)
	}
	// New entry should be added
	if tech := mapper.ExtractTechFromImage("custom-db:1.0"); tech != "CustomDB" {
		t.Errorf("custom-db -> %q, want CustomDB", tech)
	}
	// Embedded entries should still exist
	if tech := mapper.ExtractTechFromImage("redis:7"); tech != "Redis" {
		t.Errorf("redis -> %q, want Redis", tech)
	}
}
