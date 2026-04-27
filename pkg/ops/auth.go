package ops

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	authScript     = "~/Cobliteam/dev-setup/aws-login.sh"
	authScriptNote = "execute: ~/Cobliteam/dev-setup/aws-login.sh  (requer browser + Google SSO)"
)

type ssoCache struct {
	StartURL  string `json:"startUrl"`
	ExpiresAt string `json:"expiresAt"`
}

// CheckAWSAuth verifies AWS SSO token validity for the given profile.
// It never opens a browser or triggers authentication — purely diagnostic.
func CheckAWSAuth(profile string) OpsResult {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return errorResult(profile, fmt.Sprintf("não foi possível determinar home directory: %v", err))
	}

	configPath := filepath.Join(homeDir, ".aws", "config")
	startURL, err := awsProfileStartURL(configPath, profile)
	if err != nil {
		return errorResult(profile, fmt.Sprintf("perfil '%s' não encontrado em ~/.aws/config: %v", profile, err))
	}

	cacheDir := filepath.Join(homeDir, ".aws", "sso", "cache")
	expiresAt, found := findTokenExpiry(cacheDir, startURL)
	if !found {
		return errorResult(profile, fmt.Sprintf("token SSO não encontrado para '%s' — autenticação necessária", profile))
	}

	now := time.Now()
	remaining := expiresAt.Sub(now)
	remainingMin := int(remaining.Minutes())

	data := map[string]any{
		"profile":            profile,
		"expires_at":         expiresAt.Format(time.RFC3339),
		"expires_in_minutes": remainingMin,
	}

	if remaining <= 0 {
		data["valid"] = false
		return OpsResult{
			Status:  "error",
			Signal:  fmt.Sprintf("token '%s' expirado — autenticação necessária antes de continuar", profile),
			Data:    data,
			Actions: []string{authScriptNote},
			Cost:    "zero-llm",
		}
	}

	data["valid"] = true
	hStatus, hSignal, hActions := EvalHeuristics(data, loadHeuristics("auth"))
	signal := fmt.Sprintf("token '%s' válido, expira em %d min", profile, remainingMin)
	var actions []string
	if hStatus != "ok" {
		signal = hSignal
		actions = append([]string{authScriptNote}, hActions...)
	}
	return OpsResult{
		Status:  hStatus,
		Signal:  signal,
		Data:    data,
		Actions: actions,
		Cost:    "zero-llm",
	}
}

// awsProfileStartURL parses ~/.aws/config and returns the sso_start_url for the profile.
// Handles both direct sso_start_url and indirection via sso_session.
func awsProfileStartURL(configPath, profile string) (string, error) {
	sections, err := parseINI(configPath)
	if err != nil {
		return "", err
	}

	profileKey := "profile " + profile
	if profile == "default" {
		profileKey = "default"
	}

	profileSection, ok := sections[profileKey]
	if !ok {
		return "", fmt.Errorf("seção '[%s]' não encontrada", profileKey)
	}

	// Direct sso_start_url on the profile section
	if url, ok := profileSection["sso_start_url"]; ok {
		return url, nil
	}

	// Indirect via sso_session
	sessionName, ok := profileSection["sso_session"]
	if !ok {
		return "", fmt.Errorf("nem sso_start_url nem sso_session encontrados no perfil '%s'", profile)
	}

	sessionKey := "sso-session " + sessionName
	sessionSection, ok := sections[sessionKey]
	if !ok {
		return "", fmt.Errorf("sso-session '[%s]' não encontrada", sessionKey)
	}

	url, ok := sessionSection["sso_start_url"]
	if !ok {
		return "", fmt.Errorf("sso_start_url não encontrada em '[%s]'", sessionKey)
	}

	return url, nil
}

// findTokenExpiry scans ~/.aws/sso/cache/ for a token matching startURL.
func findTokenExpiry(cacheDir, startURL string) (time.Time, bool) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return time.Time{}, false
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(cacheDir, entry.Name()))
		if err != nil {
			continue
		}

		var cache ssoCache
		if err := json.Unmarshal(data, &cache); err != nil {
			continue
		}

		if cache.StartURL != startURL || cache.ExpiresAt == "" {
			continue
		}

		t, err := parseAWSTime(cache.ExpiresAt)
		if err != nil {
			continue
		}

		return t, true
	}

	return time.Time{}, false
}

// parseAWSTime handles the time formats used in AWS SSO cache files.
func parseAWSTime(s string) (time.Time, error) {
	for _, format := range []string{
		"2006-01-02T15:04:05UTC",
		time.RFC3339,
		"2006-01-02T15:04:05Z",
	} {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("formato não reconhecido: %s", s)
}

// parseINI is a minimal INI parser for ~/.aws/config.
// Returns a map of section name → (key → value).
func parseINI(path string) (map[string]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	result := map[string]map[string]string{}
	var current map[string]string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sectionName := strings.TrimSpace(line[1 : len(line)-1])
			current = map[string]string{}
			result[sectionName] = current
			continue
		}

		if current == nil {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			current[key] = val
		}
	}

	return result, scanner.Err()
}

func errorResult(profile, signal string) OpsResult {
	return OpsResult{
		Status:  "error",
		Signal:  signal,
		Data:    map[string]any{"profile": profile, "valid": false, "expires_in_minutes": 0},
		Actions: []string{authScriptNote},
		Cost:    "zero-llm",
	}
}
