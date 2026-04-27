package llm

import (
	"strings"
	"testing"
)

func TestPromptSanitizer_ScanCredentials(t *testing.T) {
	ps := NewPromptSanitizer()

	tests := []struct {
		name     string
		input    string
		category string
		wantFind bool
	}{
		{
			name:     "Anthropic API key",
			input:    "Use this key: sk-ant-api03-abc123def456ghi789jkl012mno345pqr678stu901vwx234yz567abc890def123ghi456jkl789mno012pqr345stu678vwx901",
			category: "anthropic_key",
			wantFind: true,
		},
		{
			name:     "OpenAI API key",
			input:    "My key is sk-abcdef1234567890abcdef1234567890abcdef1234567890ab",
			category: "openai_key",
			wantFind: true,
		},
		{
			name:     "Gemini API key",
			input:    "Set GEMINI_KEY=AIzaSyA1234567890abcdefGHIJKLMNOPQRSTUVWXYZ",
			category: "gemini_key",
			wantFind: true,
		},
		{
			name:     "AWS access key ID",
			input:    "aws_access_key_id = AKIAIOSFODNN7EXAMPLE",
			category: "aws_access_key",
			wantFind: true,
		},
		{
			name:     "AWS secret access key",
			input:    "aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			category: "aws_secret_key",
			wantFind: true,
		},
		{
			name:     "GitHub personal access token",
			input:    "token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij",
			category: "github_token",
			wantFind: true,
		},
		{
			name:     "GitHub fine-grained PAT",
			input:    "GITHUB_TOKEN=github_pat_11AAAAAA0BBBBBBBCCCCCCCC_DDDDDDDDEEEEEEEEFFFFFFFFGGGGGGGG",
			category: "github_token",
			wantFind: true,
		},
		{
			name:     "GitLab token",
			input:    "Set token: glpat-xYz123AbCdEfGhIjKlMnOp",
			category: "gitlab_token",
			wantFind: true,
		},
		{
			name:     "Slack token",
			input:    "SLACK_TOKEN=xoxb-1234567890-1234567890123-AbCdEfGhIjKl",
			category: "slack_token",
			wantFind: true,
		},
		{
			name:     "Bearer token",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test.sig",
			category: "bearer_token",
			wantFind: true,
		},
		{
			name:     "JWT token",
			input:    "token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
			category: "jwt",
			wantFind: true,
		},
		{
			name:     "Private key header",
			input:    "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQ...",
			category: "private_key",
			wantFind: true,
		},
		{
			name:     "Password assignment",
			input:    "DB_PASSWORD=mySuperSecretPass123!",
			category: "password_assignment",
			wantFind: true,
		},
		{
			name:     "Database connection string",
			input:    "postgres://admin:secretpass@db.example.com:5432/mydb",
			category: "db_connection_string",
			wantFind: true,
		},
		{
			name:     "Notion API token",
			input:    "NOTION_TOKEN=ntn_1234567890abcdef1234567890abcdef1234567890ab",
			category: "notion_token",
			wantFind: true,
		},
		{
			name:     "Figma access token",
			input:    "FIGMA_ACCESS_TOKEN=figd_1234567890abcdef1234567890abcdef1234567890ab",
			category: "figma_token",
			wantFind: true,
		},
		{
			name:     "No credentials - normal technical text",
			input:    "Use Spring Boot 3.2 with PostgreSQL and Redis for caching. The API should handle 1000 requests per second.",
			wantFind: false,
		},
		{
			name:     "No credentials - technology discussion",
			input:    "JWT authentication with refresh tokens. Store sessions in Redis. Use Kubernetes for orchestration.",
			wantFind: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := ps.ScanCredentialsOnly(tt.input)

			if tt.wantFind {
				if len(findings) == 0 {
					t.Errorf("expected to find credential %q but found none", tt.category)
					return
				}
				found := false
				for _, f := range findings {
					if f.Category == tt.category {
						found = true
						break
					}
				}
				if !found {
					categories := []string{}
					for _, f := range findings {
						categories = append(categories, f.Category)
					}
					t.Errorf("expected category %q but found: %v", tt.category, categories)
				}
			} else {
				if len(findings) > 0 {
					t.Errorf("expected no credentials but found %d findings: %v", len(findings), findings)
				}
			}
		})
	}
}

func TestPromptSanitizer_ScanPII(t *testing.T) {
	ps := NewPromptSanitizer()

	tests := []struct {
		name     string
		input    string
		wantPII  bool
		piiType  string
	}{
		{
			name:    "CPF",
			input:   "O CPF do usuário é 123.456.789-09",
			wantPII: true,
			piiType: "CPF",
		},
		{
			name:    "Email",
			input:   "Contact user@example.com for details",
			wantPII: true,
			piiType: "Email",
		},
		{
			name:    "No PII - technical text",
			input:   "Configure the server with 4 CPU cores and 16GB RAM",
			wantPII: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := ps.Scan(tt.input)

			piiFindings := []SecurityFinding{}
			for _, f := range findings {
				if f.Type == "pii" {
					piiFindings = append(piiFindings, f)
				}
			}

			if tt.wantPII && len(piiFindings) == 0 {
				t.Errorf("expected PII type %q but found none", tt.piiType)
			}
			if !tt.wantPII && len(piiFindings) > 0 {
				t.Errorf("expected no PII but found %d findings", len(piiFindings))
			}
		})
	}
}

func TestPromptSanitizer_Redact(t *testing.T) {
	ps := NewPromptSanitizer()

	tests := []struct {
		name       string
		input      string
		wantAbsent string // This string should NOT be in the output
		wantTag    string // This tag should be in the output
	}{
		{
			name:       "Redacts Anthropic key",
			input:      "Use sk-ant-api03-abc123def456ghi789jkl012mno345pqr678stu901vwx234yz567abc890def123ghi456jkl789mno012pqr345stu678vwx901",
			wantAbsent: "sk-ant-api03",
			wantTag:    "[ANTHROPIC-KEY-REDACTED]",
		},
		{
			name:       "Redacts GitHub token",
			input:      "token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij",
			wantAbsent: "ghp_ABCDEF",
			wantTag:    "[GITHUB-TOKEN-REDACTED]",
		},
		{
			name:       "Redacts private key header",
			input:      "-----BEGIN RSA PRIVATE KEY-----\nMIIBogIBAAJBAI+",
			wantAbsent: "BEGIN RSA PRIVATE KEY",
			wantTag:    "[PRIVATE-KEY-REDACTED]",
		},
		{
			name:       "Preserves normal text",
			input:      "Use Spring Boot 3.2 with PostgreSQL",
			wantAbsent: "",
			wantTag:    "Spring Boot",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ps.Redact(tt.input)

			if tt.wantAbsent != "" && strings.Contains(result, tt.wantAbsent) {
				t.Errorf("redacted text still contains %q: %s", tt.wantAbsent, result)
			}

			if !strings.Contains(result, tt.wantTag) {
				t.Errorf("redacted text missing tag %q: %s", tt.wantTag, result)
			}
		})
	}
}

func TestPromptSanitizer_HasCredentials(t *testing.T) {
	ps := NewPromptSanitizer()

	tests := []struct {
		name string
		text string
		want bool
	}{
		{
			name: "has Anthropic key",
			text: "sk-ant-api03-abc123def456ghi789jkl012mno",
			want: true,
		},
		{
			name: "has GitHub token",
			text: "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij",
			want: true,
		},
		{
			name: "no credentials",
			text: "Just a normal text about Spring Boot and PostgreSQL",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ps.HasCredentials(tt.text)
			if got != tt.want {
				t.Errorf("HasCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPromptSanitizer_FalsePositiveCorpus(t *testing.T) {
	ps := NewPromptSanitizer()

	// Corpus of legitimate technical prompts that should NOT trigger credential detection
	legitimatePrompts := []string{
		// Technology discussions
		"Use JWT for authentication with refresh token rotation",
		"Configure Kubernetes pods with 4 replicas",
		"Spring Boot 3.2 with Spring Security and OAuth2",
		"React Native with TypeScript and Expo",
		"PostgreSQL with pgx driver for connection pooling",

		// Architecture patterns
		"Implement CQRS with event sourcing pattern",
		"Use Redis for distributed caching with TTL of 24h",
		"Deploy to AWS ECS with Fargate launch type",
		"Configure GitHub Actions for CI/CD pipeline",

		// Code snippets (without actual secrets)
		"func main() { fmt.Println(\"Hello World\") }",
		"const API_URL = \"https://api.example.com/v1\"",
		"type Config struct { Port int; Host string }",

		// Technical requirements
		"The system should handle 10000 concurrent connections",
		"Response time should be under 200ms at P99",
		"Database should support 1TB of data with 99.9% uptime",

		// UUIDs and hashes (should not be flagged as keys)
		"Trace ID: 550e8400-e29b-41d4-a716-446655440000",
		"SHA256: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",

		// Version numbers and IDs
		"Version 3.14.159 released on 2026-01-15",
		"Issue #12345: Fix memory leak in connection pool",
	}

	for _, prompt := range legitimatePrompts {
		t.Run(prompt[:min(len(prompt), 50)], func(t *testing.T) {
			findings := ps.ScanCredentialsOnly(prompt)
			if len(findings) > 0 {
				t.Errorf("FALSE POSITIVE: legitimate prompt triggered %d finding(s): %v\nPrompt: %s",
					len(findings), findings, prompt)
			}
		})
	}
}

func TestPromptSanitizer_EnvFilePatterns(t *testing.T) {
	ps := NewPromptSanitizer()

	envContent := `
DATABASE_URL=postgres://user:pass@localhost/db
ANTHROPIC_API_KEY=sk-ant-api03-abc123def456ghi789jkl012mno345pqr678stu901vwx234
SECRET_TOKEN=mysecrettoken12345678
WTB_DEBUG=true
LOG_LEVEL=info
`
	findings := ps.ScanCredentialsOnly(envContent)
	if len(findings) == 0 {
		t.Error("expected to find credentials in .env content but found none")
	}

	// The Anthropic key should be detected
	foundAnthropicKey := false
	for _, f := range findings {
		if f.Category == "anthropic_key" {
			foundAnthropicKey = true
			break
		}
	}
	if !foundAnthropicKey {
		t.Error("expected to find anthropic_key in .env content")
	}
}

func TestMaskForAudit(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"short", "*****"},
		{"sk-ant-api03-long-key-here", "sk-a************..."},
	}

	for _, tt := range tests {
		t.Run(tt.input[:min(len(tt.input), 10)], func(t *testing.T) {
			got := maskForAudit(tt.input)
			if got != tt.want {
				t.Errorf("maskForAudit(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
