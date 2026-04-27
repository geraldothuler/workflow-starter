package llm

import (
	"fmt"
	"log"
	"strings"
)

// SecurityMode determines the action when sensitive data is detected in prompts
type SecurityMode int

const (
	// SecurityModeBlock refuses to send the prompt and returns an error.
	// Use in production when zero-tolerance for credential leaks is required.
	SecurityModeBlock SecurityMode = iota

	// SecurityModeRedact replaces sensitive data with [REDACTED] tags and sends.
	// Use when prompts may contain credentials but the workflow must continue.
	SecurityModeRedact

	// SecurityModeWarn logs a warning but sends the original prompt unchanged.
	// Use during migration to identify false positives before switching to Block/Redact.
	SecurityModeWarn
)

// SecurityConfig controls SecurityCheckpoint behavior
type SecurityConfig struct {
	Mode             SecurityMode // Action on detection: Block, Redact, Warn
	ScanPII          bool         // Scan for PII (CPF, CNPJ, email, phone, etc.)
	ScanCredentials  bool         // Scan for API keys, tokens, passwords
	ScanSystemPrompt bool         // Also scan system prompt (not just user prompt)
	AuditEvents      bool         // Log security events
	Verbose          bool         // Verbose logging of scan results
}

// DefaultSecurityConfig returns sensible defaults for security checkpoint.
// Starts with Warn mode for safe rollout — switch to Redact or Block after tuning.
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		Mode:             SecurityModeRedact,
		ScanPII:          true,
		ScanCredentials:  true,
		ScanSystemPrompt: true,
		AuditEvents:      true,
		Verbose:          false,
	}
}

// SecurityCheckpoint is a decorator that intercepts all LLM calls and scans
// prompts for credentials and PII before they reach the LLM API.
//
// Position in decorator chain:
//   EncryptedCacheProvider → SecurityCheckpoint → RetryProvider → Client
//
// This ensures:
// - Cached responses skip the checkpoint (already been through it)
// - Retries re-send the same (already sanitized) prompt
// - Raw API calls always go through the checkpoint
type SecurityCheckpoint struct {
	inner        LLMProvider
	config       SecurityConfig
	sanitizer    *PromptSanitizer
	systemPrompt string

	// Injectable logger for testing (nil-safe)
	logFunc func(format string, args ...interface{})
}

// WithSecurityCheckpoint wraps an LLMProvider with prompt security scanning.
func WithSecurityCheckpoint(provider LLMProvider, config SecurityConfig) *SecurityCheckpoint {
	return &SecurityCheckpoint{
		inner:     provider,
		config:    config,
		sanitizer: NewPromptSanitizer(),
		logFunc:   log.Printf,
	}
}

// ProviderName delegates to inner provider
func (sc *SecurityCheckpoint) ProviderName() string {
	return sc.inner.ProviderName()
}

// ModelID delegates to inner provider
func (sc *SecurityCheckpoint) ModelID() string {
	return sc.inner.ModelID()
}

// SetSystemPrompt stores and scans the system prompt, then delegates
func (sc *SecurityCheckpoint) SetSystemPrompt(system string) {
	if sc.config.ScanSystemPrompt {
		findings := sc.scanText(system)
		if len(findings) > 0 {
			sc.logSecurityEvent("system_prompt", findings)

			switch sc.config.Mode {
			case SecurityModeRedact:
				system = sc.redactText(system)
			case SecurityModeBlock:
				// For system prompts, we redact instead of blocking
				// (blocking would prevent all subsequent calls)
				system = sc.redactText(system)
			}
			// Warn mode: pass through unchanged
		}
	}

	sc.systemPrompt = system
	sc.inner.SetSystemPrompt(system)
}

// Complete implements Completer with security scanning
func (sc *SecurityCheckpoint) Complete(prompt string, maxTokens int) (string, error) {
	response, _, err := sc.CompleteWithUsage(prompt, maxTokens)
	return response, err
}

// CompleteWithUsage implements Completer with security scanning.
// This is the main entry point where all prompts are scanned.
func (sc *SecurityCheckpoint) CompleteWithUsage(prompt string, maxTokens int) (string, *Usage, error) {
	findings := sc.scanText(prompt)

	if len(findings) > 0 {
		sc.logSecurityEvent("user_prompt", findings)

		switch sc.config.Mode {
		case SecurityModeBlock:
			return "", nil, sc.buildBlockError(findings)

		case SecurityModeRedact:
			prompt = sc.redactText(prompt)
			// Continue with redacted prompt

		case SecurityModeWarn:
			// Continue with original prompt (just logged the warning)
		}
	}

	return sc.inner.CompleteWithUsage(prompt, maxTokens)
}

// scanText scans text for sensitive data based on config
func (sc *SecurityCheckpoint) scanText(text string) []SecurityFinding {
	if !sc.config.ScanCredentials && !sc.config.ScanPII {
		return nil
	}

	if sc.config.ScanCredentials && sc.config.ScanPII {
		return sc.sanitizer.Scan(text)
	}

	if sc.config.ScanCredentials {
		return sc.sanitizer.ScanCredentialsOnly(text)
	}

	// ScanPII only
	findings := []SecurityFinding{}
	piiDetections := sc.sanitizer.piiDetector.Scan(text)
	for _, d := range piiDetections {
		findings = append(findings, SecurityFinding{
			Type:     "pii",
			Category: string(d.Type),
			Value:    d.Value,
			Position: d.Position,
		})
	}
	return findings
}

// redactText applies redaction to text based on config
func (sc *SecurityCheckpoint) redactText(text string) string {
	if sc.config.ScanCredentials && sc.config.ScanPII {
		return sc.sanitizer.Redact(text)
	}
	if sc.config.ScanCredentials {
		return sc.sanitizer.RedactCredentialsOnly(text)
	}
	// PII only
	return sc.sanitizer.piiDetector.Anonymize(text)
}

// buildBlockError creates an informative error when blocking a prompt
func (sc *SecurityCheckpoint) buildBlockError(findings []SecurityFinding) error {
	credCount := 0
	piiCount := 0
	categories := []string{}

	for _, f := range findings {
		switch f.Type {
		case "credential":
			credCount++
		case "pii":
			piiCount++
		}
		categories = append(categories, f.Category)
	}

	parts := []string{}
	if credCount > 0 {
		parts = append(parts, fmt.Sprintf("%d credential(s)", credCount))
	}
	if piiCount > 0 {
		parts = append(parts, fmt.Sprintf("%d PII item(s)", piiCount))
	}

	return fmt.Errorf(
		"security checkpoint: prompt blocked — detected %s [categories: %s]. "+
			"Remove sensitive data from input or change security mode to 'redact'",
		strings.Join(parts, " and "),
		strings.Join(uniqueStrings(categories), ", "),
	)
}

// logSecurityEvent logs a security finding for audit
func (sc *SecurityCheckpoint) logSecurityEvent(source string, findings []SecurityFinding) {
	if !sc.config.AuditEvents && !sc.config.Verbose {
		return
	}

	credCount := 0
	piiCount := 0
	for _, f := range findings {
		switch f.Type {
		case "credential":
			credCount++
		case "pii":
			piiCount++
		}
	}

	if sc.logFunc != nil {
		sc.logFunc("[security] %s scan: %d credential(s), %d PII item(s) detected (mode=%s, provider=%s)",
			source, credCount, piiCount, sc.modeString(), sc.inner.ProviderName())
	}

	if sc.config.Verbose && sc.logFunc != nil {
		for _, f := range findings {
			sc.logFunc("[security]   → %s/%s at position %d: %s", f.Type, f.Category, f.Position, f.Value)
		}
	}
}

// modeString returns human-readable mode name
func (sc *SecurityCheckpoint) modeString() string {
	switch sc.config.Mode {
	case SecurityModeBlock:
		return "block"
	case SecurityModeRedact:
		return "redact"
	case SecurityModeWarn:
		return "warn"
	default:
		return "unknown"
	}
}

// uniqueStrings returns unique strings preserving order
func uniqueStrings(ss []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
