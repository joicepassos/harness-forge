package analyzer

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestPrintJSONIncludesStructuredFindings(t *testing.T) {
	analysis := &Analysis{
		Project: "example",
		Languages: []Finding{
			{
				Value:      "Go",
				Confidence: 1.0,
				Evidence:   []string{"main.go"},
			},
		},
		Files: 1,
	}

	var output bytes.Buffer
	if err := PrintJSON(&output, analysis); err != nil {
		t.Fatalf("PrintJSON() error = %v", err)
	}

	var decoded Analysis
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Project != "example" {
		t.Fatalf("decoded.Project = %q, want %q", decoded.Project, "example")
	}

	assertFinding(t, decoded.Languages, "Go")
}

func TestFindingKeepsEvidenceSampleAndCount(t *testing.T) {
	result := finding(
		"Java",
		"file-01.java",
		"file-02.java",
		"file-03.java",
		"file-04.java",
		"file-05.java",
		"file-06.java",
		"file-07.java",
		"file-08.java",
		"file-09.java",
		"file-10.java",
		"file-11.java",
	)

	if result.EvidenceCount != 11 {
		t.Fatalf("EvidenceCount = %d, want %d", result.EvidenceCount, 11)
	}

	if len(result.Evidence) != 10 {
		t.Fatalf("len(Evidence) = %d, want %d", len(result.Evidence), 10)
	}
}
