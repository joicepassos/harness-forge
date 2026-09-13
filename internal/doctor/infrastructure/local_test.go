package infrastructure

import (
	"context"
	"harnessforge/internal/doctor/application"
	"harnessforge/internal/doctor/domain"
	"os"
	"path/filepath"
	"strings"
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

func TestDoctorReportsEveryInvalidEvidenceAndInvalidRepository(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "harness.yaml")
	document := "version: 1\nproject: {name: sample}\nrules:\n  - id: first\n    description: Use a boundary\n    origin: ai\n    status: candidate\n    evidence: [{file: missing.go}, {file: other.go}]\nskills:\n  - id: task\n    description: Build a feature\n    evidence: [{file: skill.go}]\n"
	if err := os.WriteFile(path, []byte(document), 0600); err != nil {
		t.Fatal(err)
	}
	diagnostics, err := (LocalSource{}).Diagnose(context.Background(), path, dir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "evidence.invalid" {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("wanted three evidence failures, got %d", count)
	}
	diagnostics, err = (LocalSource{}).Diagnose(context.Background(), path, filepath.Join(dir, "absent"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "repository.invalid" {
			found = true
		}
	}
	if !found {
		t.Fatal("invalid repository was ignored")
	}
}

func TestDoctorCancellationAndOversizedInput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (LocalSource{}).Diagnose(ctx, "unused", "unused"); err != context.Canceled {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "harness.yaml")
	if err := os.WriteFile(path, []byte("version: 1\nproject: {name: sample}\n#"+strings.Repeat("x", 65536)), 0600); err != nil {
		t.Fatal(err)
	}
	diagnostics, err := (LocalSource{}).Diagnose(context.Background(), path, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "context.oversized" {
			return
		}
	}
	t.Fatal("oversized context was ignored")
}

func TestDoctorRejectsSkillSymlinkEscape(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	target := filepath.Join(outside, "SKILL.md")
	if err := os.WriteFile(target, []byte("# External fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "SKILL.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	path := filepath.Join(dir, "harness.yaml")
	if err := os.WriteFile(path, []byte("version: 1\nproject: {name: sample}\nskills: [{id: task, description: Task, path: SKILL.md}]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	diagnostics, err := (LocalSource{}).Diagnose(context.Background(), path, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "path.unsafe" {
			return
		}
	}
	t.Fatal("skill symlink escape was accepted")
}
