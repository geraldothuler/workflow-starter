package llm

import (
	"regexp"
	"strings"

	"github.com/Cobliteam/workflow-toolkit/pkg/logging"
	"github.com/Cobliteam/workflow-toolkit/pkg/privacy"
)

// SecurityFinding represents a sensitive data detection in a prompt
type SecurityFinding struct {
	Type     string // "credential", "pii"
	Category string // "anthropic_key", "aws_key", "cpf", etc.
	Value    string // Masked value for audit
	Position int    // Position in text
}

// PromptSanitizer scans and sanitizes LLM prompts for credentials and PII.
// Extends logging.Sanitizer with additional patterns specific to prompts.
// More aggressive than log sanitization because credentials in prompts are ALWAYS wrong.
type PromptSanitizer struct {
	baseSanitizer      *logging.Sanitizer
	piiDetector        *privacy.Detector
	credentialPatterns map[string]*regexp.Regexp
}

// NewPromptSanitizer creates a new prompt sanitizer with extended patterns
func NewPromptSanitizer() *PromptSanitizer {
	return &PromptSanitizer{
		baseSanitizer: logging.NewSanitizer(true),
		piiDetector:   privacy.NewDetector(),
		credentialPatterns: map[string]*regexp.Regexp{
			// Anthropic API keys
			"anthropic_key": regexp.MustCompile(`sk-ant-[a-zA-Z0-9_-]{20,}`),

			// OpenAI API keys
			"openai_key": regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`),

			// Gemini/Google API keys (AIza prefix)
			"gemini_key": regexp.MustCompile(`AIza[a-zA-Z0-9_-]{35,}`),

			// AWS access key IDs
			"aws_access_key": regexp.MustCompile(`AKIA[A-Z0-9]{16}`),

			// AWS secret access keys (40-char base64-like strings after aws_secret patterns)
			"aws_secret_key": regexp.MustCompile(`(?i)(aws_secret_access_key|secret_access_key)\s*[=:]\s*['"]?([a-zA-Z0-9/+=]{40})['"]?`),

			// GitHub tokens (ghp_, gho_, ghs_, ghr_, github_pat_)
			"github_token": regexp.MustCompile(`(ghp_[a-zA-Z0-9]{36}|gho_[a-zA-Z0-9]{36}|ghs_[a-zA-Z0-9]{36}|ghr_[a-zA-Z0-9]{36}|github_pat_[a-zA-Z0-9_]{22,})`),

			// GitLab tokens
			"gitlab_token": regexp.MustCompile(`glpat-[a-zA-Z0-9_-]{20,}`),

			// Slack tokens
			"slack_token": regexp.MustCompile(`xox[bpors]-[a-zA-Z0-9-]+`),

			// Bearer tokens
			"bearer_token": regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9._-]{20,}`),

			// JWT tokens
			"jwt": regexp.MustCompile(`eyJ[a-zA-Z0-9_-]*\.eyJ[a-zA-Z0-9_-]*\.[a-zA-Z0-9_-]*`),

			// Private keys (PEM format)
			"private_key": regexp.MustCompile(`-----BEGIN\s+(RSA\s+|EC\s+|DSA\s+|OPENSSH\s+)?PRIVATE\s+KEY-----`),

			// Password assignments in config files (excludes [ to avoid matching already-redacted tags)
			"password_assignment": regexp.MustCompile(`(?i)(password|passwd|pwd|secret|token|api_key|apikey|api-key|access_token|auth_token)\s*[=:]\s*['"]?([^\s'"\[]{8,})['"]?`),

			// Database connection strings with passwords
			"db_connection_string": regexp.MustCompile(`(?i)(postgres|mysql|mongodb|redis)://[^:]+:([^@]+)@`),

			// Azure connection strings
			"azure_connection": regexp.MustCompile(`(?i)(AccountKey|SharedAccessKey|AccessKey)\s*=\s*[a-zA-Z0-9+/=]{40,}`),

			// .env file patterns (KEY=value at start of line)
			"env_file_secret": regexp.MustCompile(`(?m)^[A-Z_]*(SECRET|TOKEN|KEY|PASSWORD|CREDENTIAL|AUTH)[A-Z_]*\s*=\s*['"]?([^\s'"]+)['"]?`),

			// Notion API tokens
			"notion_token": regexp.MustCompile(`(ntn_|secret_)[a-zA-Z0-9]{40,}`),

			// Figma tokens
			"figma_token": regexp.MustCompile(`figd_[a-zA-Z0-9_-]{40,}`),

			// Generic base64 encoded secrets (long base64 strings in credential context)
			"base64_secret": regexp.MustCompile(`(?i)(secret|token|key|password|credential)\s*[=:]\s*['"]?([A-Za-z0-9+/]{64,}={0,2})['"]?`),
		},
	}
}

// Scan scans text for both credentials and PII, returning all findings
func (ps *PromptSanitizer) Scan(text string) []SecurityFinding {
	findings := []SecurityFinding{}

	// Scan for credentials
	for category, pattern := range ps.credentialPatterns {
		matches := pattern.FindAllStringIndex(text, -1)
		for _, match := range matches {
			value := text[match[0]:match[1]]
			findings = append(findings, SecurityFinding{
				Type:     "credential",
				Category: category,
				Value:    maskForAudit(value),
				Position: match[0],
			})
		}
	}

	// Scan for PII using existing detector
	piiDetections := ps.piiDetector.Scan(text)
	for _, detection := range piiDetections {
		findings = append(findings, SecurityFinding{
			Type:     "pii",
			Category: string(detection.Type),
			Value:    detection.Value,
			Position: detection.Position,
		})
	}

	return findings
}

// ScanCredentialsOnly scans only for credentials (faster, for system prompt check)
func (ps *PromptSanitizer) ScanCredentialsOnly(text string) []SecurityFinding {
	findings := []SecurityFinding{}

	for category, pattern := range ps.credentialPatterns {
		matches := pattern.FindAllStringIndex(text, -1)
		for _, match := range matches {
			value := text[match[0]:match[1]]
			findings = append(findings, SecurityFinding{
				Type:     "credential",
				Category: category,
				Value:    maskForAudit(value),
				Position: match[0],
			})
		}
	}

	return findings
}

// credentialRedactOrder defines the order in which patterns are applied during redaction.
// Specific patterns MUST come before generic ones to avoid partial matches.
// E.g., "github_token" must be applied before "password_assignment" which
// would otherwise match "token: ghp_..." as a generic password assignment.
var credentialRedactOrder = []string{
	// Most specific first (known token prefixes)
	"anthropic_key",
	"openai_key",
	"gemini_key",
	"aws_access_key",
	"aws_secret_key",
	"github_token",
	"gitlab_token",
	"slack_token",
	"notion_token",
	"figma_token",
	"jwt",
	"private_key",
	"bearer_token",
	"db_connection_string",
	"azure_connection",
	// Generic patterns last (catch-all for remaining secrets)
	"env_file_secret",
	"base64_secret",
	"password_assignment",
}

// Redact replaces all findings in text with [REDACTED] placeholders.
// Patterns are applied in a specific order: specific patterns first, generic last.
func (ps *PromptSanitizer) Redact(text string) string {
	result := text

	// Redact credentials in deterministic order (specific before generic)
	for _, category := range credentialRedactOrder {
		if pattern, ok := ps.credentialPatterns[category]; ok {
			tag := credentialRedactTag(category)
			result = pattern.ReplaceAllString(result, tag)
		}
	}

	// Redact PII using existing anonymizer
	result = ps.piiDetector.Anonymize(result)

	return result
}

// RedactCredentialsOnly redacts only credentials (preserves PII for consent-based flow)
func (ps *PromptSanitizer) RedactCredentialsOnly(text string) string {
	result := text

	for _, category := range credentialRedactOrder {
		if pattern, ok := ps.credentialPatterns[category]; ok {
			tag := credentialRedactTag(category)
			result = pattern.ReplaceAllString(result, tag)
		}
	}

	return result
}

// HasCredentials quickly checks if text contains any credential patterns
func (ps *PromptSanitizer) HasCredentials(text string) bool {
	for _, pattern := range ps.credentialPatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

// maskForAudit masks a value for audit logging (shows type hint but not the secret)
func maskForAudit(value string) string {
	if len(value) <= 8 {
		return strings.Repeat("*", len(value))
	}
	// Show first 4 chars for identification, mask the rest
	return value[:4] + strings.Repeat("*", min(len(value)-4, 12)) + "..."
}

// credentialRedactTag returns the appropriate redaction tag for a category
func credentialRedactTag(category string) string {
	switch category {
	case "anthropic_key":
		return "[ANTHROPIC-KEY-REDACTED]"
	case "openai_key":
		return "[OPENAI-KEY-REDACTED]"
	case "gemini_key":
		return "[GEMINI-KEY-REDACTED]"
	case "aws_access_key":
		return "[AWS-ACCESS-KEY-REDACTED]"
	case "aws_secret_key":
		return "$1=[AWS-SECRET-REDACTED]"
	case "github_token":
		return "[GITHUB-TOKEN-REDACTED]"
	case "gitlab_token":
		return "[GITLAB-TOKEN-REDACTED]"
	case "slack_token":
		return "[SLACK-TOKEN-REDACTED]"
	case "bearer_token":
		return "Bearer [TOKEN-REDACTED]"
	case "jwt":
		return "[JWT-REDACTED]"
	case "private_key":
		return "[PRIVATE-KEY-REDACTED]"
	case "password_assignment":
		return "$1=[SECRET-REDACTED]"
	case "db_connection_string":
		return "$1://[CREDENTIALS-REDACTED]@"
	case "azure_connection":
		return "$1=[AZURE-KEY-REDACTED]"
	case "env_file_secret":
		return "$1=[ENV-SECRET-REDACTED]"
	case "notion_token":
		return "[NOTION-TOKEN-REDACTED]"
	case "figma_token":
		return "[FIGMA-TOKEN-REDACTED]"
	case "base64_secret":
		return "$1=[BASE64-SECRET-REDACTED]"
	default:
		return "[REDACTED]"
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
