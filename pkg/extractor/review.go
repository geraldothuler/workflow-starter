package extractor

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// InteractiveReviewer permite revisão interativa antes de aprovar
type InteractiveReviewer struct {
	result *ExtractionResult
	reader *bufio.Reader
}

// NewInteractiveReviewer cria novo reviewer
func NewInteractiveReviewer(result *ExtractionResult) *InteractiveReviewer {
	return &InteractiveReviewer{
		result: result,
		reader: bufio.NewReader(os.Stdin),
	}
}

// Review inicia processo de revisão interativa
func (ir *InteractiveReviewer) Review() (bool, error) {
	ir.clearScreen()
	ir.showHeader()

	// 1. Resumo geral
	if !ir.reviewSummary() {
		return false, nil
	}

	// 2. Revisar seções com baixa confiança
	if !ir.reviewLowConfidenceSections() {
		return false, nil
	}

	// 3. Revisar itens inferidos
	if !ir.reviewInferredItems() {
		return false, nil
	}

	// 4. Revisar sugestões de Golden Paths
	if !ir.reviewInferences() {
		return false, nil
	}

	// 5. Confirmação final
	return ir.finalConfirmation(), nil
}

// showHeader mostra cabeçalho
func (ir *InteractiveReviewer) showHeader() {
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║        📋 INTERACTIVE EXTRACTION REVIEW                      ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()
}

// reviewSummary mostra resumo geral
func (ir *InteractiveReviewer) reviewSummary() bool {
	fmt.Println("📊 RESUMO DA EXTRAÇÃO")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// Overall confidence
	confidenceEmoji := ir.getConfidenceEmoji(ir.result.Metadata.OverallConfidence)
	fmt.Printf("Confiança Geral: %.0f%% %s\n", 
		ir.result.Metadata.OverallConfidence*100, confidenceEmoji)
	fmt.Println()

	// Section breakdown
	fmt.Println("Confiança por Seção:")
	for section, conf := range ir.result.Metadata.SectionConfidence {
		emoji := ir.getConfidenceEmoji(conf)
		bar := ir.makeBar(conf, 20)
		fmt.Printf("  %-12s %s %.0f%% %s\n", section+":", bar, conf*100, emoji)
	}
	fmt.Println()

	// Speakers
	if len(ir.result.Metadata.SpeakersDetected) > 0 {
		fmt.Printf("Participantes: %s\n", strings.Join(ir.result.Metadata.SpeakersDetected, ", "))
		fmt.Println()
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	return ir.askContinue("Continuar com revisão detalhada?")
}

// reviewLowConfidenceSections revisa seções com baixa confiança
func (ir *InteractiveReviewer) reviewLowConfidenceSections() bool {
	lowConfSections := []string{}

	for section, conf := range ir.result.Metadata.SectionConfidence {
		if conf < 0.7 {
			lowConfSections = append(lowConfSections, section)
		}
	}

	if len(lowConfSections) == 0 {
		return true
	}

	fmt.Println()
	fmt.Println("⚠️  SEÇÕES COM BAIXA CONFIANÇA")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	for _, section := range lowConfSections {
		conf := ir.result.Metadata.SectionConfidence[section]
		
		fmt.Printf("📋 Seção: %s (%.0f%% confiança)\n", section, conf*100)
		
		// Mostrar issues se disponível
		if ir.result.Scoring != nil {
			if sectionScore, ok := ir.result.Scoring.SectionScores[section]; ok {
				if len(sectionScore.Issues) > 0 {
					fmt.Println("\nProblemas detectados:")
					for _, issue := range sectionScore.Issues {
						fmt.Printf("  ✗ %s\n", issue)
					}
				}
			}
		}

		fmt.Println()
		
		// Opções
		fmt.Println("Opções:")
		fmt.Println("  [e] Editar manualmente")
		fmt.Println("  [a] Aceitar mesmo assim")
		fmt.Println("  [s] Pular (deixar como está)")
		fmt.Println("  [q] Cancelar extração")
		fmt.Println()

		choice := ir.askChoice("Escolha", []string{"e", "a", "s", "q"})

		switch choice {
		case "e":
			if !ir.editSection(section) {
				return false
			}
		case "q":
			return false
		// "a" e "s" continuam
		}

		fmt.Println()
	}

	return true
}

// reviewInferredItems revisa itens inferidos
func (ir *InteractiveReviewer) reviewInferredItems() bool {
	if len(ir.result.Metadata.InferredItems) == 0 {
		return true
	}

	fmt.Println()
	fmt.Println("🔍 ITENS INFERIDOS (Validação Necessária)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	for i, item := range ir.result.Metadata.InferredItems {
		emoji := ir.getConfidenceEmoji(item.Confidence)
		
		fmt.Printf("%d. %s %s (%.0f%%)\n", i+1, item.Type, item.Value, item.Confidence*100)
		fmt.Printf("   %s\n", emoji)
		if item.Rationale != "" {
			fmt.Printf("   Razão: %s\n", item.Rationale)
		}
		fmt.Println()
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	return ir.askContinue("Aceitar itens inferidos?")
}

// reviewInferences revisa inferências de Golden Paths
func (ir *InteractiveReviewer) reviewInferences() bool {
	if ir.result.Inference == nil {
		return true
	}

	hasInferences := len(ir.result.Inference.InferredTechnologies) > 0 ||
		len(ir.result.Inference.InferredPatterns) > 0 ||
		len(ir.result.Inference.Suggestions) > 0

	if !hasInferences {
		return true
	}

	fmt.Println()
	fmt.Println("🎯 INFERÊNCIAS DE GOLDEN PATHS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// Tecnologias inferidas
	if len(ir.result.Inference.InferredTechnologies) > 0 {
		fmt.Println("Tecnologias Sugeridas:")
		for _, tech := range ir.result.Inference.InferredTechnologies {
			emoji := ir.getConfidenceEmoji(tech.Confidence)
			required := ""
			if tech.Required {
				required = " ⚠️  NECESSÁRIA"
			}
			
			fmt.Printf("  • %s: %.0f%%%s %s\n", tech.Name, tech.Confidence*100, required, emoji)
			fmt.Printf("    Pattern: %s\n", tech.Pattern)
			fmt.Printf("    Razão: %s\n", tech.Rationale)
		}
		fmt.Println()
	}

	// Patterns aplicáveis
	if len(ir.result.Inference.InferredPatterns) > 0 {
		fmt.Println("Patterns Aplicáveis:")
		for _, pattern := range ir.result.Inference.InferredPatterns {
			fmt.Printf("  • %s - %s (%.0f%%)\n", 
				pattern.PatternID, pattern.PatternName, pattern.Confidence*100)
		}
		fmt.Println()
	}

	// Sugestões
	if len(ir.result.Inference.Suggestions) > 0 {
		fmt.Println("Sugestões:")
		for _, suggestion := range ir.result.Inference.Suggestions {
			fmt.Printf("  💡 %s\n", suggestion.Description)
		}
		fmt.Println()
	}

	// Gap analysis
	if len(ir.result.Inference.GapAnalysis.CriticalGaps) > 0 {
		fmt.Println("⚠️  Gaps Críticos:")
		for _, gap := range ir.result.Inference.GapAnalysis.CriticalGaps {
			fmt.Printf("  ✗ %s\n", gap)
		}
		fmt.Println()
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	return ir.askContinue("Aceitar sugestões de Golden Paths?")
}

// finalConfirmation confirmação final
func (ir *InteractiveReviewer) finalConfirmation() bool {
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║              ✅ CONFIRMAÇÃO FINAL                             ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Printf("Confiança Geral: %.0f%% %s\n", 
		ir.result.Metadata.OverallConfidence*100,
		ir.getConfidenceEmoji(ir.result.Metadata.OverallConfidence))
	
	if ir.result.Inference != nil {
		fmt.Printf("Completude do Stack: %.0f%%\n", 
			ir.result.Inference.GapAnalysis.Completeness*100)
	}
	
	fmt.Println()
	fmt.Println("Próximos passos:")
	fmt.Println("  1. project-definition.md será salvo")
	fmt.Println("  2. Você poderá gerar o backlog com:")
	fmt.Println("     wtb run backlog project-definition.md")
	fmt.Println()

	return ir.askYesNo("Confirmar e salvar?")
}

// editSection abre editor para editar seção
func (ir *InteractiveReviewer) editSection(section string) bool {
	fmt.Printf("\n🖊️  Editando seção: %s\n", section)
	fmt.Println("(Abrirá editor de texto...)")
	fmt.Println()

	// TODO: Abrir editor (vim, nano, code) com conteúdo da seção
	// Por ora, placeholder

	return ir.askYesNo("Edição concluída?")
}

// Helper functions

func (ir *InteractiveReviewer) askContinue(prompt string) bool {
	fmt.Printf("\n%s [y/n]: ", prompt)
	return ir.readYesNo()
}

func (ir *InteractiveReviewer) askYesNo(prompt string) bool {
	fmt.Printf("%s [y/n]: ", prompt)
	return ir.readYesNo()
}

func (ir *InteractiveReviewer) askChoice(prompt string, options []string) string {
	for {
		fmt.Printf("%s [%s]: ", prompt, strings.Join(options, "/"))
		input, _ := ir.reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		for _, opt := range options {
			if input == opt {
				return input
			}
		}

		fmt.Printf("Opção inválida. Escolha entre: %s\n", strings.Join(options, ", "))
	}
}

func (ir *InteractiveReviewer) readYesNo() bool {
	input, _ := ir.reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes" || input == "s" || input == "sim"
}

func (ir *InteractiveReviewer) getConfidenceEmoji(conf float64) string {
	if conf >= 0.8 {
		return "✅ Alta"
	} else if conf >= 0.6 {
		return "⚠️  Média"
	}
	return "❌ Baixa"
}

func (ir *InteractiveReviewer) makeBar(value float64, width int) string {
	filled := int(value * float64(width))
	empty := width - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return fmt.Sprintf("[%s]", bar)
}

func (ir *InteractiveReviewer) clearScreen() {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux", "darwin":
		cmd = exec.Command("clear")
	case "windows":
		cmd = exec.Command("cmd", "/c", "cls")
	default:
		// Fallback: print newlines
		fmt.Print("\n\n\n\n\n\n\n\n\n\n")
		return
	}

	cmd.Stdout = os.Stdout
	cmd.Run()
}

// EditInExternalEditor abre editor externo
func (ir *InteractiveReviewer) EditInExternalEditor(filepath string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		// Fallbacks
		if runtime.GOOS == "windows" {
			editor = "notepad"
		} else {
			editor = "nano"
		}
	}

	cmd := exec.Command(editor, filepath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
