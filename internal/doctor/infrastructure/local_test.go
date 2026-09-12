package infrastructure

import (
	"context"
	"harnessforge/internal/doctor/application"
	"harnessforge/internal/doctor/domain"
	"os"
	"path/filepath"
	"testing"
)

func TestDoctorReportsActionableFailuresWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "harness.yaml")
	original := "version: 1\nproject: {name: test}\nrules:\n  - id: first\n    description: Same instruction\n    origin: human\n    status: candidate\n    scope: {paths: [../unsafe]}\n  - id: second\n    description: same   instruction\n    origin: ai\n    status: candidate\n    evidence: [{file: missing.go}]\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := application.NewDiagnose(LocalSource{}).Execute(context.Background(), path, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Diagnostics) < 3 {
		t.Fatalf("diagnostics = %#v", report.Diagnostics)
	}
	if got, _ := os.ReadFile(path); string(got) != original {
		t.Fatal("doctor modified harness")
	}
	for _, d := range report.Diagnostics {
		if d.Suggestion == "" {
			t.Fatalf("missing suggestion: %#v", d)
		}
	}
}
func TestDoctorReportsInvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "harness.yaml")
	os.WriteFile(path, []byte("bad: ["), 0600)
	report, err := application.NewDiagnose(LocalSource{}).Execute(context.Background(), path, "")
	if err != nil || len(report.Diagnostics) != 1 || report.Diagnostics[0].Severity != domain.SeverityError {
		t.Fatalf("%#v %v", report, err)
	}
}

func TestDoctorReportsMissingSkillAndTests(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "harness.yaml")
	document := "version: 1\nproject: {name: test}\nskills:\n  - id: missing\n    description: Missing skill\n    path: .harness/skills/missing/SKILL.md\n"
	if err := os.WriteFile(path, []byte(document), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := application.NewDiagnose(LocalSource{}).Execute(context.Background(), path, dir)
	if err != nil {
		t.Fatal(err)
	}
	codes := map[string]bool{}
	for _, diagnostic := range report.Diagnostics {
		codes[diagnostic.Code] = true
	}
	if !codes["skill.missing"] || !codes["tests.missing"] || !codes["rules.missing"] {
		t.Fatalf("diagnostics = %#v", report.Diagnostics)
	}
}
