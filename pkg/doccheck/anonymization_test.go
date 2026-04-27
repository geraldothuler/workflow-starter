package doccheck

import (
	"path/filepath"
	"testing"
)

func TestCheckAnonymizationInDocs_clean(t *testing.T) {
	root := t.TempDir()
	createFile(t, filepath.Join(root, "docs", "overview.html"),
		`<html><body><p>Slot de replicação configurado. Use hostname genérico.</p></body></html>`)

	result := CheckAnonymizationInDocs(root)
	if !result.Passed {
		t.Errorf("expected pass for clean doc, got: %s", result.Detail)
	}
}

func TestCheckAnonymizationInDocs_privateIP(t *testing.T) {
	root := t.TempDir()
	createFile(t, filepath.Join(root, "docs", "guide.html"),
		`<html><body>Connect to 10.0.1.45 for database access.</body></html>`)

	result := CheckAnonymizationInDocs(root)
	if result.Passed {
		t.Error("expected failure for private IP in docs")
	}
	if result.Check != "anonymization-in-docs" {
		t.Errorf("wrong check name: %s", result.Check)
	}
}

func TestCheckAnonymizationInDocs_internalHostname(t *testing.T) {
	root := t.TempDir()
	createFile(t, filepath.Join(root, "docs", "guide.md"),
		`Connect to fusca-db.cobli.co on port 5432.`)

	result := CheckAnonymizationInDocs(root)
	if result.Passed {
		t.Error("expected failure for internal hostname")
	}
}

func TestCheckAnonymizationInDocs_personalEmail(t *testing.T) {
	root := t.TempDir()
	createFile(t, filepath.Join(root, "docs", "guide.md"),
		`Contact john.doe@cobli.co for access.`)

	result := CheckAnonymizationInDocs(root)
	if result.Passed {
		t.Error("expected failure for personal email")
	}
}

func TestCheckAnonymizationInDocs_slackID(t *testing.T) {
	root := t.TempDir()
	createFile(t, filepath.Join(root, "docs", "guide.md"),
		`owner: U02QV90JS1M`)

	result := CheckAnonymizationInDocs(root)
	if result.Passed {
		t.Error("expected failure for Slack user ID")
	}
}

func TestCheckAnonymizationInDocs_replicationSlot(t *testing.T) {
	root := t.TempDir()
	createFile(t, filepath.Join(root, "docs", "guide.html"),
		`<code>slot airbyte_slot invalidado por WAL overflow</code>`)

	result := CheckAnonymizationInDocs(root)
	if result.Passed {
		t.Error("expected failure for replication slot name")
	}
}

func TestCheckAnonymizationInDocs_noguardOverride(t *testing.T) {
	root := t.TempDir()
	createFile(t, filepath.Join(root, "docs", "guide.html"),
		`<!-- wtb-noguard: anonymization-in-docs — case study com dados reais aprovados -->
<html><body>Connect to 10.0.1.45 using slot airbyte_slot.</body></html>`)

	result := CheckAnonymizationInDocs(root)
	if !result.Passed {
		t.Errorf("expected pass when noguard override present, got: %s", result.Detail)
	}
}

func TestCheckAnonymizationInDocs_noDocsDir(t *testing.T) {
	root := t.TempDir()
	result := CheckAnonymizationInDocs(root)
	if !result.Passed {
		t.Errorf("expected pass when docs/ absent, got: %s", result.Detail)
	}
}

func TestCheckAnonymizationInDocs_multipleViolations(t *testing.T) {
	root := t.TempDir()
	createFile(t, filepath.Join(root, "docs", "guide.html"),
		"<p>DB at 10.0.1.45 (db-host.internal) slot airbyte_slot</p>")

	result := CheckAnonymizationInDocs(root)
	if result.Passed {
		t.Error("expected failure for multiple violations")
	}
}
