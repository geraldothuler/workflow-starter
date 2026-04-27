package i18n

import (
	"embed"
	"encoding/json"
	"os"
	"strings"
	"sync"
)

//go:embed translations/*.json
var translationFiles embed.FS

// Locale representa um locale
type Locale string

const (
	LocalePTBR Locale = "pt-BR"
	LocaleENUS Locale = "en-US"
)

var (
	translations map[Locale]map[string]string
	currentLocale Locale
	once         sync.Once
	mu           sync.RWMutex
)

// loadTranslations carrega traducoes dos arquivos JSON embarcados
func loadTranslations() {
	translations = make(map[Locale]map[string]string)

	locales := []Locale{LocalePTBR, LocaleENUS}
	for _, loc := range locales {
		filename := "translations/" + string(loc) + ".json"
		data, err := translationFiles.ReadFile(filename)
		if err != nil {
			// Fallback para hardcoded se arquivo nao encontrado
			translations[loc] = hardcodedTranslations(loc)
			continue
		}

		var msgs map[string]string
		if err := json.Unmarshal(data, &msgs); err != nil {
			translations[loc] = hardcodedTranslations(loc)
			continue
		}
		translations[loc] = msgs
	}

	// Detectar locale padrao
	currentLocale = DetectLocale()
}

// hardcodedTranslations retorna traducoes fallback
func hardcodedTranslations(loc Locale) map[string]string {
	switch loc {
	case LocalePTBR:
		return map[string]string{
			"validate.title": "Valida especificação",
			"generating":     "Gerando backlog...",
			"done":           "Concluído!",
		}
	case LocaleENUS:
		return map[string]string{
			"validate.title": "Validate specification",
			"generating":     "Generating backlog...",
			"done":           "Done!",
		}
	}
	return nil
}

// T traduz uma mensagem
func T(key string, locale ...Locale) string {
	once.Do(loadTranslations)

	mu.RLock()
	loc := currentLocale
	mu.RUnlock()

	if len(locale) > 0 {
		loc = locale[0]
	}

	if msgs, ok := translations[loc]; ok {
		if msg, ok := msgs[key]; ok {
			return msg
		}
	}

	return key
}

// SetLocale define o locale ativo
func SetLocale(locale Locale) {
	once.Do(loadTranslations)
	mu.Lock()
	currentLocale = locale
	mu.Unlock()
}

// GetLocale retorna o locale ativo
func GetLocale() Locale {
	once.Do(loadTranslations)
	mu.RLock()
	defer mu.RUnlock()
	return currentLocale
}

// DetectLocale detecta o locale do sistema via variáveis de ambiente
func DetectLocale() Locale {
	for _, envVar := range []string{"WTB_LANG", "LANG", "LC_ALL", "LC_MESSAGES"} {
		val := os.Getenv(envVar)
		if val == "" {
			continue
		}
		lower := strings.ToLower(val)
		if strings.Contains(lower, "pt") {
			return LocalePTBR
		}
		if strings.Contains(lower, "en") {
			return LocaleENUS
		}
	}
	return LocalePTBR // default
}

// AvailableLocales retorna os locales disponíveis
func AvailableLocales() []Locale {
	once.Do(loadTranslations)
	locales := make([]Locale, 0, len(translations))
	for loc := range translations {
		locales = append(locales, loc)
	}
	return locales
}

// Keys retorna todas as chaves disponíveis para um locale
func Keys(locale ...Locale) []string {
	once.Do(loadTranslations)

	loc := LocalePTBR
	if len(locale) > 0 {
		loc = locale[0]
	}

	msgs, ok := translations[loc]
	if !ok {
		return nil
	}

	keys := make([]string, 0, len(msgs))
	for k := range msgs {
		keys = append(keys, k)
	}
	return keys
}
