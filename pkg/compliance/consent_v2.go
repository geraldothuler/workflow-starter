package compliance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ConsentRecordV2 stores per-provider consent with LGPD Art. 33 compliance.
// Backward compatible with v1: migrates from consent.json to consent_v2.json.
type ConsentRecordV2 struct {
	Version            string                     `json:"version"` // "2.0"
	Consents           map[string]ProviderConsent `json:"consents"`
	DataCategories     []DataCategoryConsent      `json:"data_categories,omitempty"`
	FirstConsentDate   time.Time                  `json:"first_consent_date"`
	LastReviewDate     time.Time                  `json:"last_review_date"`
	ReviewReminderDays int                        `json:"review_reminder_days"` // default: 90
}

// ProviderConsent tracks consent per LLM provider.
type ProviderConsent struct {
	Provider        string     `json:"provider"`          // "claude", "chatgpt", etc
	ConsentGiven    bool       `json:"consent_given"`     // User consented?
	ConsentDate     time.Time  `json:"consent_date"`      // When consented
	RevokedDate     *time.Time `json:"revoked_date,omitempty"`
	DataDestination string     `json:"data_destination"`  // "USA", "EU", "Self-hosted"
	Retention       string     `json:"retention"`         // Provider retention policy
	TrainingOptOut  bool       `json:"training_opt_out"`  // Does provider use data for training?
	DemoMode        bool       `json:"demo_mode"`         // Demo mode consent?
}

// DataCategoryConsent tracks consent per data category.
type DataCategoryConsent struct {
	Category    string `json:"category"`     // "project_specs", "backlogs", "deep_dives"
	Description string `json:"description"`  // Human-readable description
	Consented   bool   `json:"consented"`    // User consented to this category?
}

// ConsentManagerV2 manages per-provider consent with LGPD Art. 33 compliance.
type ConsentManagerV2 struct {
	configDir string
	demoMode  bool

	// Injectable for testing
	nowFunc   func() time.Time
	readFile  func(string) ([]byte, error)
	writeFile func(string, []byte, os.FileMode) error
	mkdirAll  func(string, os.FileMode) error
	stat      func(string) (os.FileInfo, error)
}

// NewConsentManagerV2 creates a new v2 consent manager.
func NewConsentManagerV2(configDir string, demoMode bool) *ConsentManagerV2 {
	return &ConsentManagerV2{
		configDir: configDir,
		demoMode:  demoMode,
		nowFunc:   time.Now,
		readFile:  os.ReadFile,
		writeFile: os.WriteFile,
		mkdirAll:  os.MkdirAll,
		stat:      os.Stat,
	}
}

// consentV2Path returns the path to the v2 consent file.
func (cm *ConsentManagerV2) consentV2Path() string {
	return filepath.Join(cm.configDir, "consent_v2.json")
}

// consentV1Path returns the path to the legacy v1 consent file.
func (cm *ConsentManagerV2) consentV1Path() string {
	return filepath.Join(cm.configDir, "consent.json")
}

// CheckProviderConsent checks if the user has given consent for a specific provider.
// Returns (consented, needsReview, error).
// needsReview is true when the review reminder period has elapsed.
func (cm *ConsentManagerV2) CheckProviderConsent(provider string) (bool, bool, error) {
	if cm.demoMode {
		return true, false, nil
	}

	record, err := cm.loadOrMigrate()
	if err != nil {
		return false, false, err
	}
	if record == nil {
		return false, false, nil // No consent file
	}

	consent, exists := record.Consents[provider]
	if !exists {
		return false, false, nil // No consent for this provider
	}

	if !consent.ConsentGiven {
		return false, false, nil
	}

	if consent.RevokedDate != nil {
		return false, false, nil // Consent was revoked
	}

	// Check if review is needed
	needsReview := false
	if record.ReviewReminderDays > 0 {
		daysSinceReview := cm.nowFunc().Sub(record.LastReviewDate).Hours() / 24
		needsReview = daysSinceReview >= float64(record.ReviewReminderDays)
	}

	return true, needsReview, nil
}

// GrantConsent records consent for a specific provider.
func (cm *ConsentManagerV2) GrantConsent(provider string, destination string, retention string, trainingOptOut bool) error {
	record, err := cm.loadOrMigrate()
	if err != nil {
		return err
	}
	if record == nil {
		record = cm.newEmptyRecord()
	}

	now := cm.nowFunc()

	record.Consents[provider] = ProviderConsent{
		Provider:        provider,
		ConsentGiven:    true,
		ConsentDate:     now,
		RevokedDate:     nil,
		DataDestination: destination,
		Retention:       retention,
		TrainingOptOut:  trainingOptOut,
		DemoMode:        cm.demoMode,
	}

	if record.FirstConsentDate.IsZero() {
		record.FirstConsentDate = now
	}
	record.LastReviewDate = now

	return cm.save(record)
}

// RevokeConsent revokes consent for a specific provider.
func (cm *ConsentManagerV2) RevokeConsent(provider string) error {
	record, err := cm.loadOrMigrate()
	if err != nil {
		return err
	}
	if record == nil {
		return fmt.Errorf("no consent record found")
	}

	consent, exists := record.Consents[provider]
	if !exists {
		return fmt.Errorf("no consent found for provider %q", provider)
	}

	now := cm.nowFunc()
	consent.ConsentGiven = false
	consent.RevokedDate = &now
	record.Consents[provider] = consent

	return cm.save(record)
}

// MarkReviewed updates the last review date.
func (cm *ConsentManagerV2) MarkReviewed() error {
	record, err := cm.loadOrMigrate()
	if err != nil {
		return err
	}
	if record == nil {
		return fmt.Errorf("no consent record found")
	}

	record.LastReviewDate = cm.nowFunc()
	return cm.save(record)
}

// ListConsents returns all provider consents.
func (cm *ConsentManagerV2) ListConsents() (map[string]ProviderConsent, error) {
	if cm.demoMode {
		return map[string]ProviderConsent{
			"demo": {
				Provider:     "demo",
				ConsentGiven: true,
				ConsentDate:  cm.nowFunc(),
				DemoMode:     true,
			},
		}, nil
	}

	record, err := cm.loadOrMigrate()
	if err != nil {
		return nil, err
	}
	if record == nil {
		return make(map[string]ProviderConsent), nil
	}

	return record.Consents, nil
}

// GetRecord returns the full consent record.
func (cm *ConsentManagerV2) GetRecord() (*ConsentRecordV2, error) {
	return cm.loadOrMigrate()
}

// SetDataCategories updates the data category consents.
func (cm *ConsentManagerV2) SetDataCategories(categories []DataCategoryConsent) error {
	record, err := cm.loadOrMigrate()
	if err != nil {
		return err
	}
	if record == nil {
		record = cm.newEmptyRecord()
	}

	record.DataCategories = categories
	return cm.save(record)
}

// InternationalTransferSummary returns a summary of international data transfers.
// Implements LGPD Art. 33 requirement for transparency on international transfers.
func (cm *ConsentManagerV2) InternationalTransferSummary() ([]TransferSummaryEntry, error) {
	record, err := cm.loadOrMigrate()
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, nil
	}

	var entries []TransferSummaryEntry
	for _, consent := range record.Consents {
		if consent.ConsentGiven && consent.RevokedDate == nil {
			entries = append(entries, TransferSummaryEntry{
				Provider:    consent.Provider,
				Destination: consent.DataDestination,
				Retention:   consent.Retention,
				TrainingUse: !consent.TrainingOptOut,
				ConsentDate: consent.ConsentDate,
			})
		}
	}
	return entries, nil
}

// TransferSummaryEntry is a single entry in the international transfer summary.
type TransferSummaryEntry struct {
	Provider    string    `json:"provider"`
	Destination string    `json:"destination"`
	Retention   string    `json:"retention"`
	TrainingUse bool      `json:"training_use"`
	ConsentDate time.Time `json:"consent_date"`
}

// --- Internal helpers ---

func (cm *ConsentManagerV2) newEmptyRecord() *ConsentRecordV2 {
	return &ConsentRecordV2{
		Version:            "2.0",
		Consents:           make(map[string]ProviderConsent),
		ReviewReminderDays: 90,
	}
}

// loadOrMigrate loads the v2 consent file, or migrates from v1 if needed.
func (cm *ConsentManagerV2) loadOrMigrate() (*ConsentRecordV2, error) {
	// Try v2 file first
	v2Path := cm.consentV2Path()
	data, err := cm.readFile(v2Path)
	if err == nil {
		var record ConsentRecordV2
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("invalid consent_v2.json: %w", err)
		}
		// Ensure map is initialized
		if record.Consents == nil {
			record.Consents = make(map[string]ProviderConsent)
		}
		return &record, nil
	}

	// Try to migrate from v1
	v1Path := cm.consentV1Path()
	data, err = cm.readFile(v1Path)
	if err != nil {
		return nil, nil // No consent file at all
	}

	var v1 ConsentRecord
	if err := json.Unmarshal(data, &v1); err != nil {
		return nil, nil // Invalid v1 file, treat as no consent
	}

	// Migrate v1 → v2
	record := cm.migrateV1(v1)

	// Save the migrated record as v2
	if saveErr := cm.save(record); saveErr != nil {
		// Non-fatal: return the migrated record even if save fails
		return record, nil
	}

	return record, nil
}

// migrateV1 converts a v1 consent record to v2 format.
func (cm *ConsentManagerV2) migrateV1(v1 ConsentRecord) *ConsentRecordV2 {
	providerInfo := getProviderInfo2(v1.Provider)

	record := cm.newEmptyRecord()
	record.FirstConsentDate = v1.ConsentDate
	record.LastReviewDate = v1.ConsentDate

	record.Consents[v1.Provider] = ProviderConsent{
		Provider:        v1.Provider,
		ConsentGiven:    v1.ConsentGiven,
		ConsentDate:     v1.ConsentDate,
		DataDestination: providerInfo.Destination,
		Retention:       providerInfo.Retention,
		TrainingOptOut:  providerInfo.TrainingOptOut,
		DemoMode:        v1.DemoMode,
	}

	return record
}

// save writes the consent record to disk.
func (cm *ConsentManagerV2) save(record *ConsentRecordV2) error {
	if err := cm.mkdirAll(cm.configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal consent: %w", err)
	}

	return cm.writeFile(cm.consentV2Path(), data, 0600)
}

// providerInfoV2 stores known provider metadata for consent.
type providerInfoV2 struct {
	Destination    string
	Retention      string
	TrainingOptOut bool
}

// getProviderInfo2 returns known metadata for a provider.
func getProviderInfo2(provider string) providerInfoV2 {
	switch provider {
	case "claude":
		return providerInfoV2{
			Destination:    "USA",
			Retention:      "~30 days in cache",
			TrainingOptOut: true, // Anthropic does not train on API data
		}
	case "chatgpt":
		return providerInfoV2{
			Destination:    "USA",
			Retention:      "~30 days in cache",
			TrainingOptOut: false, // OpenAI may use API data (opt-out available)
		}
	case "gemini":
		return providerInfoV2{
			Destination:    "USA",
			Retention:      "~18 months",
			TrainingOptOut: false, // Google may use data per AI Terms
		}
	case "ollama":
		return providerInfoV2{
			Destination:    "Self-hosted",
			Retention:      "Local only",
			TrainingOptOut: true, // Self-hosted = no training
		}
	case "azure":
		return providerInfoV2{
			Destination:    "Configured region",
			Retention:      "Per Azure policy",
			TrainingOptOut: true, // Azure OpenAI does not train on customer data
		}
	default:
		return providerInfoV2{
			Destination:    "Unknown",
			Retention:      "See provider ToS",
			TrainingOptOut: false,
		}
	}
}

// GrantDemoConsent creates demo mode consent for a provider.
func (cm *ConsentManagerV2) GrantDemoConsent(provider string) error {
	record, err := cm.loadOrMigrate()
	if err != nil {
		return err
	}
	if record == nil {
		record = cm.newEmptyRecord()
	}

	now := cm.nowFunc()
	info := getProviderInfo2(provider)

	record.Consents[provider] = ProviderConsent{
		Provider:        provider,
		ConsentGiven:    true,
		ConsentDate:     now,
		DataDestination: info.Destination,
		Retention:       info.Retention,
		TrainingOptOut:  info.TrainingOptOut,
		DemoMode:        true,
	}

	if record.FirstConsentDate.IsZero() {
		record.FirstConsentDate = now
	}
	record.LastReviewDate = now

	return cm.save(record)
}
