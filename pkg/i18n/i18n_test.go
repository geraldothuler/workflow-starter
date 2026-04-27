package i18n

import (
	"sync"
	"testing"
)

// resetState resets the package state for isolated tests
func resetState() {
	once = sync.Once{}
	translations = nil
	currentLocale = ""
}

func TestT_PortugueseLookup(t *testing.T) {
	resetState()
	SetLocale(LocalePTBR)
	result := T("generating")
	if result != "Gerando backlog..." {
		t.Errorf("expected Portuguese translation, got %q", result)
	}
}

func TestT_EnglishLookup(t *testing.T) {
	resetState()
	result := T("generating", LocaleENUS)
	if result != "Generating backlog..." {
		t.Errorf("expected English translation, got %q", result)
	}
}

func TestT_MissingKey(t *testing.T) {
	resetState()
	result := T("nonexistent_key")
	if result != "nonexistent_key" {
		t.Errorf("expected key returned as-is, got %q", result)
	}
}

func TestT_UnknownLocale(t *testing.T) {
	resetState()
	result := T("generating", Locale("fr-FR"))
	if result != "generating" {
		t.Errorf("expected key returned for unknown locale, got %q", result)
	}
}

func TestT_AllOriginalKeys(t *testing.T) {
	resetState()
	keys := []string{"validate.title", "generating", "done"}
	for _, key := range keys {
		result := T(key)
		if result == key {
			t.Errorf("expected translation for %q, got key back", key)
		}
	}
}

func TestLocaleConstants(t *testing.T) {
	if LocalePTBR != "pt-BR" {
		t.Errorf("expected pt-BR, got %s", LocalePTBR)
	}
	if LocaleENUS != "en-US" {
		t.Errorf("expected en-US, got %s", LocaleENUS)
	}
}

// --- New tests ---

func TestT_NewKeys_Portuguese(t *testing.T) {
	resetState()
	SetLocale(LocalePTBR)

	tests := map[string]string{
		"backlog.generating": "Gerando backlog...",
		"backlog.done":       "Backlog gerado com sucesso!",
		"backlog.error":      "Erro ao gerar backlog",
		"extract.extracting": "Extraindo especificação...",
		"extract.done":       "Extração concluída!",
		"spec.validating":    "Validando spec...",
		"spec.generating":    "Gerando spec...",
		"spec.report":        "Relatório de qualidade",
		"export.exporting":   "Exportando...",
		"export.done":        "Exportação concluída!",
		"render.applying":    "Aplicando render...",
		"render.done":        "Render aplicado com sucesso!",
		"common.error":       "Erro",
		"common.success":     "Sucesso",
		"common.verbose":     "Modo verbose ativado",
	}

	for key, expected := range tests {
		result := T(key)
		if result != expected {
			t.Errorf("T(%q) = %q, want %q", key, result, expected)
		}
	}
}

func TestT_NewKeys_English(t *testing.T) {
	resetState()

	tests := map[string]string{
		"backlog.generating": "Generating backlog...",
		"backlog.done":       "Backlog generated successfully!",
		"export.done":        "Export complete!",
		"common.error":       "Error",
	}

	for key, expected := range tests {
		result := T(key, LocaleENUS)
		if result != expected {
			t.Errorf("T(%q, en-US) = %q, want %q", key, result, expected)
		}
	}
}

func TestSetLocale_ChangesDefault(t *testing.T) {
	resetState()
	SetLocale(LocaleENUS)
	result := T("done")
	if result != "Done!" {
		t.Errorf("expected English after SetLocale, got %q", result)
	}

	SetLocale(LocalePTBR)
	result = T("done")
	if result != "Concluído!" {
		t.Errorf("expected Portuguese after SetLocale, got %q", result)
	}
}

func TestGetLocale(t *testing.T) {
	resetState()
	SetLocale(LocaleENUS)
	if GetLocale() != LocaleENUS {
		t.Errorf("expected en-US, got %s", GetLocale())
	}
}

func TestDetectLocale_FromEnv(t *testing.T) {
	// Test WTB_LANG takes priority
	t.Setenv("WTB_LANG", "en-US")
	result := DetectLocale()
	if result != LocaleENUS {
		t.Errorf("expected en-US from WTB_LANG, got %s", result)
	}

	// Test pt detection
	t.Setenv("WTB_LANG", "")
	t.Setenv("LANG", "pt_BR.UTF-8")
	result = DetectLocale()
	if result != LocalePTBR {
		t.Errorf("expected pt-BR from LANG, got %s", result)
	}

	// Test default when nothing set
	t.Setenv("WTB_LANG", "")
	t.Setenv("LANG", "")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	result = DetectLocale()
	if result != LocalePTBR {
		t.Errorf("expected pt-BR as default, got %s", result)
	}
}

func TestAvailableLocales(t *testing.T) {
	resetState()
	locales := AvailableLocales()
	if len(locales) < 2 {
		t.Errorf("expected at least 2 locales, got %d", len(locales))
	}
}

func TestKeys(t *testing.T) {
	resetState()
	keys := Keys(LocalePTBR)
	if len(keys) < 15 {
		t.Errorf("expected at least 15 keys, got %d", len(keys))
	}

	// All keys should have translations
	for _, key := range keys {
		result := T(key)
		if result == key {
			t.Errorf("key %q has no translation", key)
		}
	}
}

func TestKeys_BothLocalesHaveSameKeys(t *testing.T) {
	resetState()
	ptKeys := Keys(LocalePTBR)
	enKeys := Keys(LocaleENUS)

	if len(ptKeys) != len(enKeys) {
		t.Errorf("pt-BR has %d keys, en-US has %d keys", len(ptKeys), len(enKeys))
	}

	ptMap := make(map[string]bool)
	for _, k := range ptKeys {
		ptMap[k] = true
	}

	for _, k := range enKeys {
		if !ptMap[k] {
			t.Errorf("key %q exists in en-US but not in pt-BR", k)
		}
	}
}

func TestLoadTranslations_FromEmbed(t *testing.T) {
	resetState()
	// Force loading
	once.Do(loadTranslations)

	if translations == nil {
		t.Fatal("translations should be loaded")
	}
	if len(translations) < 2 {
		t.Errorf("expected at least 2 locales loaded, got %d", len(translations))
	}
	// Check a key from JSON file (not in hardcoded fallback)
	if msg, ok := translations[LocalePTBR]["backlog.done"]; !ok || msg == "" {
		t.Error("expected backlog.done from JSON file")
	}
}
