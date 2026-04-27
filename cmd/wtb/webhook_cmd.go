package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/webhook"
	"github.com/spf13/cobra"
)

func newWebhookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhook",
		Short: "Webhook ingestion management (status, setup)",
	}
	cmd.AddCommand(newWebhookStatusCmd(), newWebhookSetupCmd())
	return cmd
}

// ── webhook status ────────────────────────────────────────────────────────────

func newWebhookStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show webhook readiness: Keychain secrets, daemon, ngrok",
		RunE: func(cmd *cobra.Command, args []string) error {
			s := webhook.Check()
			fmt.Print(s.Report())
			return nil
		},
	}
}

// ── webhook setup ─────────────────────────────────────────────────────────────

func newWebhookSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Wizard de setup: Keychain, ngrok, daemon, GitHub/Datadog",
		Long: `Configura o webhook ingestion passo a passo.
Executa tudo que consegue automaticamente — sem perguntas desnecessárias.

  wtb webhook setup         # configura tudo automaticamente
  wtb webhook setup         # rode novamente após restart do ngrok para atualizar URLs`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWebhookSetup()
		},
	}
	return cmd
}

func runWebhookSetup() error {
	s := webhook.Check()

	fmt.Println("=== wtb webhook setup ===")
	fmt.Println()

	// ── 1. ngrok ──────────────────────────────────────────────────────────────
	if !s.NgrokInstalled {
		fmt.Println("── [1/5] ngrok: não instalado ───────────────────────────────")
		fmt.Println()
		fmt.Println("  Instale com Homebrew e crie uma conta gratuita em ngrok.com:")
		fmt.Println()
		fmt.Println("    brew install ngrok/ngrok/ngrok")
		fmt.Println("    ngrok config add-authtoken <seu-token>  # token em: dashboard.ngrok.com")
		fmt.Println()
		fmt.Println("  Após instalar, rode novamente: wtb webhook setup")
		return nil
	}
	fmt.Println("── [1/5] ngrok: ✓ instalado")

	// ── 2. Keychain secrets (auto-gerados) ────────────────────────────────────
	fmt.Println()
	fmt.Println("── [2/5] secrets: Keychain ──────────────────────────────────")
	fmt.Println()

	if !s.SecretGitHub {
		secret, err := randomHex(20)
		if err != nil {
			return fmt.Errorf("gerar secret: %w", err)
		}
		if err := keychainSet("workflow-webhook-secret-github", secret); err != nil {
			return fmt.Errorf("salvar secret GitHub: %w", err)
		}
		fmt.Println("  ✓ workflow-webhook-secret-github gerado e salvo")
	} else {
		fmt.Println("  ✓ workflow-webhook-secret-github já existe")
	}

	if !s.SecretDatadog {
		token, err := randomHex(20)
		if err != nil {
			return fmt.Errorf("gerar token: %w", err)
		}
		if err := keychainSet("workflow-webhook-secret-datadog", token); err != nil {
			return fmt.Errorf("salvar token Datadog: %w", err)
		}
		fmt.Println("  ✓ workflow-webhook-secret-datadog gerado e salvo")
	} else {
		fmt.Println("  ✓ workflow-webhook-secret-datadog já existe")
	}

	// ── 3. wtb daemon ─────────────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("── [3/5] daemon: wtb serve --webhook-port 7655 ──────────────")
	fmt.Println()

	if !s.DaemonRunning {
		fmt.Println("  Iniciando...")
		cmd := exec.Command(os.Args[0], "serve", "--webhook-port", "7655", "--daemon")
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Start(); err != nil {
			fmt.Printf("  ✗ falha: %v\n", err)
			fmt.Println()
			fmt.Println("  Inicie em outro terminal e rode novamente:")
			fmt.Println("    wtb serve --webhook-port 7655")
			return nil
		}
		// Wait for daemon to be ready (max 3s)
		for i := 0; i < 6; i++ {
			time.Sleep(500 * time.Millisecond)
			if webhook.Check().DaemonRunning {
				break
			}
		}
		if !webhook.Check().DaemonRunning {
			fmt.Println("  ✗ daemon não respondeu em 3s.")
			fmt.Println()
			fmt.Println("  Inicie em outro terminal e rode novamente:")
			fmt.Println("    wtb serve --webhook-port 7655")
			return nil
		}
		fmt.Println("  ✓ daemon iniciado")
	} else {
		fmt.Println("  ✓ já rodando")
	}

	// ── 4. ngrok tunnel ───────────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("── [4/5] ngrok: tunnel ──────────────────────────────────────")
	fmt.Println()

	if !s.NgrokRunning {
		fmt.Println("  Iniciando ngrok http 7655...")
		cmd := exec.Command("ngrok", "http", "7655", "--log=stderr")
		if err := cmd.Start(); err != nil {
			fmt.Printf("  ✗ falha: %v\n", err)
			fmt.Println()
			fmt.Println("  Inicie em outro terminal e rode novamente:")
			fmt.Println("    ngrok http 7655")
			return nil
		}
		fmt.Print("  Aguardando URL pública")
		for i := 0; i < 12; i++ {
			time.Sleep(800 * time.Millisecond)
			fmt.Print(".")
			if c := webhook.Check(); c.NgrokRunning {
				s.NgrokURL = c.NgrokURL
				s.NgrokRunning = true
				break
			}
		}
		fmt.Println()
		if !s.NgrokRunning {
			fmt.Println("  ✗ ngrok não respondeu. Inicie manualmente:")
			fmt.Println("    ngrok http 7655")
			fmt.Println("  Depois rode novamente: wtb webhook setup")
			return nil
		}
		fmt.Printf("  ✓ ativo: %s\n", s.NgrokURL)
	} else {
		fmt.Printf("  ✓ já ativo: %s\n", s.NgrokURL)
	}

	// ── 5. Instruções de registro GitHub + Datadog ────────────────────────────
	fmt.Println()
	fmt.Println("── [5/5] registrar webhooks ─────────────────────────────────")

	secret, _ := keychainLookup("workflow-webhook-secret-github")
	ddToken, _ := keychainLookup("workflow-webhook-secret-datadog")
	ddKey, _ := keychainLookup("workflow-dd-api-key")

	githubURL := s.NgrokURL + "/webhooks/github"
	datadogURL := s.NgrokURL + "/webhooks/datadog"

	fmt.Println()
	fmt.Println("  GitHub — cole no terminal (substitua owner/repo):")
	fmt.Println()
	fmt.Printf("    gh api repos/<owner>/<repo>/hooks --method POST \\\n")
	fmt.Printf("      --field \"name=web\" --field \"active=true\" \\\n")
	fmt.Printf("      --field \"events[]=check_run\" \\\n")
	fmt.Printf("      --raw-field \"config[url]=%s\" \\\n", githubURL)
	fmt.Printf("      --raw-field \"config[secret]=%s\" \\\n", secret)
	fmt.Printf("      --raw-field \"config[content_type]=json\"\n")

	fmt.Println()
	fmt.Println("  Datadog — cole no terminal:")
	fmt.Println()
	fmt.Printf("    curl -s -X POST \"https://api.datadoghq.com/api/v1/integration/webhooks/configuration/webhooks\" \\\n")
	fmt.Printf("      -H \"DD-API-KEY: %s\" \\\n", maskSecret(ddKey))
	fmt.Printf("      -H \"Content-Type: application/json\" \\\n")
	fmt.Printf("      -d '{\"name\":\"workflow-toolkit\",\"url\":\"%s\",\"custom_headers\":\"{\\\"X-Webhook-Secret\\\":\\\"%s\\\"}\"}'",
		datadogURL, ddToken)
	fmt.Println()

	fmt.Println()
	fmt.Println("────────────────────────────────────────────────────────────")
	fmt.Printf("  Webhook URLs ativas:\n")
	fmt.Printf("    GitHub  : %s\n", githubURL)
	fmt.Printf("    Datadog : %s\n", datadogURL)
	fmt.Println()
	fmt.Println("  Após o próximo restart do ngrok, rode novamente: wtb webhook setup")
	fmt.Println("  (os secrets são mantidos no Keychain, só a URL muda)")

	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func keychainSet(service, value string) error {
	err := exec.Command("security", "add-generic-password",
		"-s", service, "-a", "geraldothuler", "-w", value).Run()
	if err != nil {
		// Already exists — delete and re-add
		exec.Command("security", "delete-generic-password",
			"-s", service, "-a", "geraldothuler").Run()
		return exec.Command("security", "add-generic-password",
			"-s", service, "-a", "geraldothuler", "-w", value).Run()
	}
	return nil
}

func keychainLookup(service string) (string, error) {
	out, err := exec.Command("security", "find-generic-password",
		"-s", service, "-a", "geraldothuler", "-w").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func maskSecret(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "..." + s[len(s)-4:]
}
