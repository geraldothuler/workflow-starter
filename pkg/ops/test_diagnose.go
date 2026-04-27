package ops

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TestDiagnoseConfig holds parameters for test failure diagnosis.
type TestDiagnoseConfig struct {
	Path   string // directory containing JUnit XML reports
	Module string // display name for the module
}

// TestFailure describes a single classified test failure.
type TestFailure struct {
	Class string `json:"class"`
	Test  string `json:"test"`
	Kind  string `json:"kind"` // "infra" | "code"
	Hint  string `json:"hint"` // excerpt of failure message
}

// junitTestSuite maps the <testsuite> XML element.
type junitTestSuite struct {
	XMLName   xml.Name        `xml:"testsuite"`
	Name      string          `xml:"name,attr"`
	TestCases []junitTestCase `xml:"testcase"`
}

type junitTestCase struct {
	Classname string        `xml:"classname,attr"`
	Name      string        `xml:"name,attr"`
	Failure   *junitFailure `xml:"failure"`
	Error     *junitFailure `xml:"error"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Text    string `xml:",chardata"`
}

// infraSignatures are patterns in failure text that indicate infra unavailability
// rather than a code defect. Order matters: more specific patterns first.
var infraSignatures = []string{
	"AllNodesFailedException",
	"NoRouteToHostException",
	"UnknownHostException",
	"org.testcontainers",
	"testcontainers",
	"container startup",
	"ConnectException",
	"Connection refused",
	"Connection timed out",
	"Cannot connect",
	"Could not connect",
}

// CheckTestDiagnose scans JUnit XML reports and classifies each failure as
// "infra" (environment unavailable) or "code" (actual test defect).
func CheckTestDiagnose(cfg TestDiagnoseConfig) OpsResult {
	if cfg.Path == "" {
		return OpsResult{
			Status:  "error",
			Signal:  "test-diagnose: missing --path to JUnit XML reports",
			Actions: []string{"wtb ops test-diagnose --path module/build/test-results/test --module <name>"},
			Cost:    "zero-llm",
		}
	}

	xmlFiles, err := filepath.Glob(filepath.Join(cfg.Path, "*.xml"))
	if err != nil || len(xmlFiles) == 0 {
		return OpsResult{
			Status:  "warn",
			Signal:  fmt.Sprintf("test-diagnose: no XML reports found in %s", cfg.Path),
			Actions: []string{"verify --path and that tests have run"},
			Cost:    "zero-llm",
		}
	}

	module := cfg.Module
	if module == "" {
		module = filepath.Base(cfg.Path)
	}

	var infraFails, codeFails []TestFailure

	for _, f := range xmlFiles {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var suite junitTestSuite
		if err := xml.Unmarshal(raw, &suite); err != nil {
			continue
		}
		for _, tc := range suite.TestCases {
			var fl *junitFailure
			switch {
			case tc.Failure != nil:
				fl = tc.Failure
			case tc.Error != nil:
				fl = tc.Error
			}
			if fl == nil {
				continue
			}

			combined := strings.ToLower(fl.Message + " " + fl.Text)
			isInfra := false
			for _, sig := range infraSignatures {
				if strings.Contains(combined, strings.ToLower(sig)) {
					isInfra = true
					break
				}
			}

			hint := strings.TrimSpace(fl.Message)
			if len(hint) > 120 {
				hint = hint[:120] + "..."
			}

			class := tc.Classname
			if class == "" {
				class = suite.Name
			}

			tf := TestFailure{Class: class, Test: tc.Name, Hint: hint}
			if isInfra {
				tf.Kind = "infra"
				infraFails = append(infraFails, tf)
			} else {
				tf.Kind = "code"
				codeFails = append(codeFails, tf)
			}
		}
	}

	totalFails := len(infraFails) + len(codeFails)

	data := map[string]any{
		"module":      module,
		"xml_files":   len(xmlFiles),
		"total_fails": totalFails,
		"infra_fails": len(infraFails),
		"code_fails":  len(codeFails),
		"failures":    append(infraFails, codeFails...),
	}

	base := fmt.Sprintf("test-diagnose %s: %d failures (%d infra, %d code) across %d suites",
		module, totalFails, len(infraFails), len(codeFails), len(xmlFiles))

	if totalFails == 0 {
		return OpsResult{
			Status: "ok",
			Signal: fmt.Sprintf("test-diagnose %s: no failures in %d suites", module, len(xmlFiles)),
			Data:   data,
			Cost:   "zero-llm",
		}
	}

	hStatus, hSignal, hActions := EvalHeuristics(data, loadHeuristics("test-diagnose"))
	signal := base
	if hSignal != "" {
		signal += " | " + hSignal
	}

	return OpsResult{
		Status:  hStatus,
		Signal:  signal,
		Data:    data,
		Actions: hActions,
		Cost:    "zero-llm",
	}
}
