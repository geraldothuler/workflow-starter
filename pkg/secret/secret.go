// Package secret provides a cross-platform credential lookup/store helper.
//
// Resolution chain (Get):
//  1. macOS Keychain (security) with current user's login account
//  2. Linux Secret Service (secret-tool / GNOME Keyring / KDE Wallet)
//  3. GNU pass (wtb/<key>)
//  4. Environment variable WORKFLOW_SECRET_<KEY_UPPER> or <KEY_UPPER>
//
// Store chain (Set):
//  1. macOS Keychain
//  2. Linux Secret Service
//  3. GNU pass
//
// This package is intentionally low-level — no config, no YAML, no resolver.
// Use pkg/credentials for the full resolver (YAML providers, encrypted-file, etc.).
// Use this package when you need a single-function call with the standard fallback chain.
package secret

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Get resolves a credential by service key, trying multiple backends in order.
// Returns ("", ErrNotFound) if no backend has the key.
func Get(key string) (string, error) {
	account := currentUser()

	// 1. macOS Keychain
	if runtime.GOOS == "darwin" {
		if val, err := keychainGet(key, account); err == nil {
			return val, nil
		}
	}

	// 2. Linux Secret Service (GNOME Keyring / KDE Wallet)
	if runtime.GOOS == "linux" {
		if val, err := secretToolGet(key); err == nil {
			return val, nil
		}
	}

	// 3. GNU pass (cross-platform)
	if val, err := passGet(key); err == nil {
		return val, nil
	}

	// 4. Environment variable
	if val := envGet(key); val != "" {
		return val, nil
	}

	return "", fmt.Errorf("secret %q not found (tried keychain/secret-tool/pass/env)", key)
}

// Set stores a credential using the first available backend.
func Set(key, value string) error {
	account := currentUser()

	if runtime.GOOS == "darwin" {
		if err := keychainSet(key, account, value); err == nil {
			return nil
		}
	}

	if runtime.GOOS == "linux" {
		if err := secretToolSet(key, value); err == nil {
			return nil
		}
	}

	if err := passSet(key, value); err == nil {
		return nil
	}

	return fmt.Errorf("secret %q: no writable backend available (keychain/secret-tool/pass)", key)
}

func currentUser() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	if u := os.Getenv("LOGNAME"); u != "" {
		return u
	}
	return "workflow"
}

func keychainGet(key, account string) (string, error) {
	if _, err := exec.LookPath("security"); err != nil {
		return "", err
	}
	out, err := exec.Command("security", "find-generic-password", "-s", key, "-a", account, "-w").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func keychainSet(key, account, value string) error {
	if _, err := exec.LookPath("security"); err != nil {
		return err
	}
	err := exec.Command("security", "add-generic-password", "-U", "-s", key, "-a", account, "-w", value).Run()
	return err
}

func secretToolGet(key string) (string, error) {
	if _, err := exec.LookPath("secret-tool"); err != nil {
		return "", err
	}
	out, err := exec.Command("secret-tool", "lookup", "service", key, "account", "workflow").Output()
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(string(out))
	if v == "" {
		return "", fmt.Errorf("empty")
	}
	return v, nil
}

func secretToolSet(key, value string) error {
	if _, err := exec.LookPath("secret-tool"); err != nil {
		return err
	}
	cmd := exec.Command("secret-tool", "store", "--label", key, "service", key, "account", "workflow")
	cmd.Stdin = strings.NewReader(value)
	return cmd.Run()
}

func passGet(key string) (string, error) {
	if _, err := exec.LookPath("pass"); err != nil {
		return "", err
	}
	passPath := "wtb/" + strings.TrimPrefix(key, "workflow-")
	out, err := exec.Command("pass", "show", passPath).Output()
	if err != nil {
		return "", err
	}
	// pass outputs one line: the secret value (possibly followed by metadata lines)
	lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
	if len(lines) == 0 || lines[0] == "" {
		return "", fmt.Errorf("empty")
	}
	return lines[0], nil
}

func passSet(key, value string) error {
	if _, err := exec.LookPath("pass"); err != nil {
		return err
	}
	passPath := "wtb/" + strings.TrimPrefix(key, "workflow-")
	cmd := exec.Command("pass", "insert", "-f", passPath)
	cmd.Stdin = strings.NewReader(value + "\n" + value + "\n")
	return cmd.Run()
}

func envGet(key string) string {
	upper := strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
	if v := os.Getenv("WORKFLOW_SECRET_" + upper); v != "" {
		return v
	}
	return os.Getenv(upper)
}
