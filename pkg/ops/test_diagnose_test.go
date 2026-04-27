package ops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckTestDiagnose_MissingPath(t *testing.T) {
	r := CheckTestDiagnose(TestDiagnoseConfig{})
	if r.Status != "error" {
		t.Errorf("expected error for missing path, got %q", r.Status)
	}
}

func TestCheckTestDiagnose_NoXMLFiles(t *testing.T) {
	dir := t.TempDir()
	r := CheckTestDiagnose(TestDiagnoseConfig{Path: dir, Module: "empty-module"})
	if r.Status != "warn" {
		t.Errorf("expected warn (no XML files), got %q: %s", r.Status, r.Signal)
	}
}

func TestCheckTestDiagnose_InfraFailuresOnly(t *testing.T) {
	dir := t.TempDir()
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="com.example.RepositoryTest">
  <testcase classname="com.example.RepositoryTest" name="testFindById">
    <failure message="AllNodesFailedException: no nodes available to execute query"
             type="com.datastax.driver.core.exceptions.AllNodesFailedException">
      AllNodesFailedException: no nodes available
        at com.example.RepositoryTest.testFindById(RepositoryTest.kt:45)
    </failure>
  </testcase>
</testsuite>`
	if err := os.WriteFile(filepath.Join(dir, "TEST-RepositoryTest.xml"), []byte(xmlContent), 0600); err != nil {
		t.Fatal(err)
	}
	r := CheckTestDiagnose(TestDiagnoseConfig{Path: dir, Module: "sherlock"})
	if r.Status != "warn" {
		t.Errorf("expected warn (infra failure only), got %q: %s", r.Status, r.Signal)
	}
	if r.Data["infra_fails"] != 1 {
		t.Errorf("expected infra_fails=1, got %v", r.Data["infra_fails"])
	}
	if r.Data["code_fails"] != 0 {
		t.Errorf("expected code_fails=0, got %v", r.Data["code_fails"])
	}
}

func TestCheckTestDiagnose_CodeFailures(t *testing.T) {
	dir := t.TempDir()
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="com.example.ServiceTest">
  <testcase classname="com.example.ServiceTest" name="testSyncCount">
    <failure message="expected: 100 but was: 99"
             type="org.junit.ComparisonFailure">
      expected: 100 but was: 99
        at com.example.ServiceTest.testSyncCount(ServiceTest.kt:72)
    </failure>
  </testcase>
</testsuite>`
	if err := os.WriteFile(filepath.Join(dir, "TEST-ServiceTest.xml"), []byte(xmlContent), 0600); err != nil {
		t.Fatal(err)
	}
	r := CheckTestDiagnose(TestDiagnoseConfig{Path: dir, Module: "webhook-sender"})
	if r.Status != "critical" {
		t.Errorf("expected critical (code failure), got %q: %s", r.Status, r.Signal)
	}
	if r.Data["code_fails"] != 1 {
		t.Errorf("expected code_fails=1, got %v", r.Data["code_fails"])
	}
}
