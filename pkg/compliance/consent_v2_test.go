package compliance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConsentV2_NewManager(t *testing.T) {
	cm := NewConsentManagerV2("/tmp/test", false)
	if cm == nil {
		t.Fatal("expected non-nil manager")
	}
	if cm.demoMode {
		t.Error("expected demoMode=false")
	}
}

func TestConsentV2_DemoModeAlwaysConsented(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConsentManagerV2(tmpDir, true)

	ok, needsReview, err := cm.CheckProviderConsent("claude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected consent=true in demo mode")
	}
	if needsReview {
		t.Error("expected needsReview=false in demo mode")
	}
}

func TestConsentV2_NoFileReturnsNoConsent(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConsentManagerV2(tmpDir, false)

	ok, _, err := cm.CheckProviderConsent("claude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected consent=false when no file exists")
	}
}

func TestConsentV2_GrantAndCheckConsent(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConsentManagerV2(tmpDir, false)

	err := cm.GrantConsent("claude", "USA", "~30 days", true)
	if err != nil {
		t.Fatalf("GrantConsent failed: %v", err)
	}

	ok, _, err := cm.CheckProviderConsent("claude")
	if err != nil {
		t.Fatalf("CheckProviderConsent failed: %v", err)
	}
	if !ok {
		t.Error("expected consent=true after granting")
	}

	// Verify file was created
	v2Path := filepath.Join(tmpDir, "consent_v2.json")
	if _, err := os.Stat(v2Path); os.IsNotExist(err) {
		t.Fatal("consent_v2.json not created")
	}
}

func TestConsentV2_PerProviderConsent(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConsentManagerV2(tmpDir, false)

	// Grant consent for Claude only
	cm.GrantConsent("claude", "USA", "~30 days", true)

	// Claude should have consent
	ok, _, _ := cm.CheckProviderConsent("claude")
	if !ok {
		t.Error("expected consent=true for claude")
	}

	// ChatGPT should NOT have consent
	ok, _, _ = cm.CheckProviderConsent("chatgpt")
	if ok {
		t.Error("expected consent=false for chatgpt")
	}

	// Grant consent for ChatGPT too
	cm.GrantConsent("chatgpt", "USA", "~30 days", false)

	ok, _, _ = cm.CheckProviderConsent("chatgpt")
	if !ok {
		t.Error("expected consent=true for chatgpt after granting")
	}
}

func TestConsentV2_RevokeConsent(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConsentManagerV2(tmpDir, false)

	cm.GrantConsent("claude", "USA", "~30 days", true)

	// Verify consent exists
	ok, _, _ := cm.CheckProviderConsent("claude")
	if !ok {
		t.Fatal("expected consent=true before revocation")
	}

	// Revoke
	err := cm.RevokeConsent("claude")
	if err != nil {
		t.Fatalf("RevokeConsent failed: %v", err)
	}

	// Verify revoked
	ok, _, _ = cm.CheckProviderConsent("claude")
	if ok {
		t.Error("expected consent=false after revocation")
	}

	// Verify revoked_date is set
	record, _ := cm.GetRecord()
	consent := record.Consents["claude"]
	if consent.RevokedDate == nil {
		t.Error("expected revoked_date to be set")
	}
}

func TestConsentV2_RevokeNonexistentProvider(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConsentManagerV2(tmpDir, false)

	// Grant consent for claude
	cm.GrantConsent("claude", "USA", "~30 days", true)

	// Try to revoke a provider that was never granted
	err := cm.RevokeConsent("unknown")
	if err == nil {
		t.Error("expected error when revoking non-existent provider")
	}
}

func TestConsentV2_RevokeNoRecord(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConsentManagerV2(tmpDir, false)

	err := cm.RevokeConsent("claude")
	if err == nil {
		t.Error("expected error when no consent record exists")
	}
}

func TestConsentV2_ReviewReminder(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConsentManagerV2(tmpDir, false)

	// Set time to a known point
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cm.nowFunc = func() time.Time { return now }

	cm.GrantConsent("claude", "USA", "~30 days", true)

	// Check immediately: no review needed
	_, needsReview, _ := cm.CheckProviderConsent("claude")
	if needsReview {
		t.Error("expected needsReview=false immediately after granting")
	}

	// 89 days later: still no review needed
	cm.nowFunc = func() time.Time { return now.Add(89 * 24 * time.Hour) }
	_, needsReview, _ = cm.CheckProviderConsent("claude")
	if needsReview {
		t.Error("expected needsReview=false at 89 days")
	}

	// 91 days later: review needed
	cm.nowFunc = func() time.Time { return now.Add(91 * 24 * time.Hour) }
	_, needsReview, _ = cm.CheckProviderConsent("claude")
	if !needsReview {
		t.Error("expected needsReview=true at 91 days")
	}

	// Mark reviewed
	cm.MarkReviewed()

	// No longer needs review
	_, needsReview, _ = cm.CheckProviderConsent("claude")
	if needsReview {
		t.Error("expected needsReview=false after marking reviewed")
	}
}

func TestConsentV2_MigrateV1(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a v1 consent file
	v1 := ConsentRecord{
		Version:          "1.0",
		Provider:         "claude",
		ConsentGiven:     true,
		ConsentDate:      time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
		DemoMode:         false,
		UnderstandsLGPD:  true,
		NoSensitiveData:  true,
		HasAuthorization: true,
	}
	v1Data, _ := json.MarshalIndent(v1, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, "consent.json"), v1Data, 0600)

	// Create v2 manager
	cm := NewConsentManagerV2(tmpDir, false)

	// Should auto-migrate and find consent
	ok, _, err := cm.CheckProviderConsent("claude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected consent=true after v1→v2 migration")
	}

	// Verify v2 file was created
	v2Path := filepath.Join(tmpDir, "consent_v2.json")
	if _, err := os.Stat(v2Path); os.IsNotExist(err) {
		t.Error("expected consent_v2.json to be created during migration")
	}

	// Verify migrated record has correct data
	record, err := cm.GetRecord()
	if err != nil {
		t.Fatalf("GetRecord failed: %v", err)
	}
	if record.Version != "2.0" {
		t.Errorf("expected version 2.0, got %q", record.Version)
	}
	consent := record.Consents["claude"]
	if consent.DataDestination != "USA" {
		t.Errorf("expected destination 'USA', got %q", consent.DataDestination)
	}
	if !consent.TrainingOptOut {
		t.Error("expected training_opt_out=true for Claude")
	}
}

func TestConsentV2_MigrateV1DemoMode(t *testing.T) {
	tmpDir := t.TempDir()

	v1 := ConsentRecord{
		Version:      "1.0",
		Provider:     "chatgpt",
		ConsentGiven: true,
		ConsentDate:  time.Now(),
		DemoMode:     true,
	}
	v1Data, _ := json.MarshalIndent(v1, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, "consent.json"), v1Data, 0600)

	cm := NewConsentManagerV2(tmpDir, false)
	record, _ := cm.GetRecord()

	consent := record.Consents["chatgpt"]
	if !consent.DemoMode {
		t.Error("expected demoMode=true preserved during migration")
	}
}

func TestConsentV2_InternationalTransferSummary(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConsentManagerV2(tmpDir, false)

	// Grant consent for two providers
	cm.GrantConsent("claude", "USA", "~30 days", true)
	cm.GrantConsent("chatgpt", "USA", "~30 days", false)

	entries, err := cm.InternationalTransferSummary()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 transfer entries, got %d", len(entries))
	}

	// Find Claude entry
	var claudeEntry *TransferSummaryEntry
	for i := range entries {
		if entries[i].Provider == "claude" {
			claudeEntry = &entries[i]
			break
		}
	}
	if claudeEntry == nil {
		t.Fatal("claude not found in transfer summary")
	}
	if claudeEntry.Destination != "USA" {
		t.Errorf("expected destination 'USA', got %q", claudeEntry.Destination)
	}
	if claudeEntry.TrainingUse {
		t.Error("expected training_use=false for Claude (opt-out=true)")
	}

	// Revoke chatgpt → should not appear in summary
	cm.RevokeConsent("chatgpt")
	entries, _ = cm.InternationalTransferSummary()
	if len(entries) != 1 {
		t.Errorf("expected 1 transfer entry after revocation, got %d", len(entries))
	}
}

func TestConsentV2_ListConsents(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConsentManagerV2(tmpDir, false)

	// Initially empty
	consents, err := cm.ListConsents()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(consents) != 0 {
		t.Errorf("expected 0 consents initially, got %d", len(consents))
	}

	// Add two
	cm.GrantConsent("claude", "USA", "~30 days", true)
	cm.GrantConsent("ollama", "Self-hosted", "Local only", true)

	consents, _ = cm.ListConsents()
	if len(consents) != 2 {
		t.Errorf("expected 2 consents, got %d", len(consents))
	}
}

func TestConsentV2_ListConsentsDemoMode(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConsentManagerV2(tmpDir, true)

	consents, err := cm.ListConsents()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(consents) != 1 {
		t.Errorf("expected 1 demo consent, got %d", len(consents))
	}
	if !consents["demo"].DemoMode {
		t.Error("expected demo mode consent")
	}
}

func TestConsentV2_SetDataCategories(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConsentManagerV2(tmpDir, false)

	categories := []DataCategoryConsent{
		{Category: "project_specs", Description: "Project specifications", Consented: true},
		{Category: "backlogs", Description: "Generated backlogs", Consented: true},
		{Category: "deep_dives", Description: "Technical deep dives", Consented: false},
	}

	err := cm.SetDataCategories(categories)
	if err != nil {
		t.Fatalf("SetDataCategories failed: %v", err)
	}

	record, _ := cm.GetRecord()
	if len(record.DataCategories) != 3 {
		t.Errorf("expected 3 data categories, got %d", len(record.DataCategories))
	}
}

func TestConsentV2_GrantDemoConsent(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConsentManagerV2(tmpDir, true)

	err := cm.GrantDemoConsent("claude")
	if err != nil {
		t.Fatalf("GrantDemoConsent failed: %v", err)
	}

	record, _ := cm.GetRecord()
	consent := record.Consents["claude"]
	if !consent.DemoMode {
		t.Error("expected demoMode=true")
	}
	if !consent.ConsentGiven {
		t.Error("expected consentGiven=true")
	}
	if consent.DataDestination != "USA" {
		t.Errorf("expected destination 'USA', got %q", consent.DataDestination)
	}
}

func TestConsentV2_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()

	var writtenPerm os.FileMode
	var dirPerm os.FileMode

	cm := NewConsentManagerV2(tmpDir, false)
	cm.writeFile = func(name string, data []byte, perm os.FileMode) error {
		writtenPerm = perm
		return os.WriteFile(name, data, perm)
	}
	cm.mkdirAll = func(path string, perm os.FileMode) error {
		dirPerm = perm
		return os.MkdirAll(path, perm)
	}

	cm.GrantConsent("claude", "USA", "~30 days", true)

	if writtenPerm != 0600 {
		t.Errorf("expected file perm 0600, got %04o", writtenPerm)
	}
	if dirPerm != 0700 {
		t.Errorf("expected dir perm 0700, got %04o", dirPerm)
	}
}

func TestConsentV2_JSONRoundtrip(t *testing.T) {
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	revoked := now.Add(24 * time.Hour)

	record := &ConsentRecordV2{
		Version: "2.0",
		Consents: map[string]ProviderConsent{
			"claude": {
				Provider:        "claude",
				ConsentGiven:    true,
				ConsentDate:     now,
				DataDestination: "USA",
				Retention:       "~30 days",
				TrainingOptOut:  true,
			},
			"chatgpt": {
				Provider:        "chatgpt",
				ConsentGiven:    false,
				ConsentDate:     now,
				RevokedDate:     &revoked,
				DataDestination: "USA",
				Retention:       "~30 days",
				TrainingOptOut:  false,
			},
		},
		FirstConsentDate:   now,
		LastReviewDate:     now,
		ReviewReminderDays: 90,
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded ConsentRecordV2
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Version != "2.0" {
		t.Errorf("expected version '2.0', got %q", decoded.Version)
	}
	if len(decoded.Consents) != 2 {
		t.Errorf("expected 2 consents, got %d", len(decoded.Consents))
	}
	if decoded.Consents["chatgpt"].RevokedDate == nil {
		t.Error("expected revoked_date to be preserved")
	}
	if decoded.ReviewReminderDays != 90 {
		t.Errorf("expected review_reminder_days=90, got %d", decoded.ReviewReminderDays)
	}
}

func TestConsentV2_GetProviderInfo2(t *testing.T) {
	tests := []struct {
		provider    string
		destination string
	}{
		{"claude", "USA"},
		{"chatgpt", "USA"},
		{"gemini", "USA"},
		{"ollama", "Self-hosted"},
		{"azure", "Configured region"},
		{"unknown", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			info := getProviderInfo2(tt.provider)
			if info.Destination != tt.destination {
				t.Errorf("expected destination %q, got %q", tt.destination, info.Destination)
			}
		})
	}
}

func TestConsentV2_FirstConsentDatePreserved(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConsentManagerV2(tmpDir, false)

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cm.nowFunc = func() time.Time { return now }

	// First grant
	cm.GrantConsent("claude", "USA", "~30 days", true)

	// Second grant 30 days later
	cm.nowFunc = func() time.Time { return now.Add(30 * 24 * time.Hour) }
	cm.GrantConsent("chatgpt", "USA", "~30 days", false)

	record, _ := cm.GetRecord()
	if !record.FirstConsentDate.Equal(now) {
		t.Errorf("first_consent_date changed: expected %v, got %v", now, record.FirstConsentDate)
	}
}
