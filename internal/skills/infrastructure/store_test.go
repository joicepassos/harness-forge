package infrastructure

import (
	"harnessforge/internal/skills/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreGeneratesOnceAndPreservesManualSkills(t *testing.T) {
	root := t.TempDir()
	harness := filepath.Join(root, ".harness", "harness.yaml")
	if err := os.MkdirAll(filepath.Dir(harness), 0755); err != nil {
		t.Fatal(err)
	}
	original := "# manual header\nversion: 1\nproject: {name: sample}\n"
	if err := os.WriteFile(harness, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	proposal := domain.Proposal{ID: "add-backed-feature", Description: "Add a feature.", Examples: []domain.Evidence{{File: "src/A.java", Symbol: "A"}}, Limitations: []string{"Review it."}}
	if err := (Store{}).Generate(root, proposal); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(harness)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), original) || !strings.Contains(string(data), "status: approved") {
		t.Fatalf("manual YAML was not preserved: %s", data)
	}
	if _, err := os.Stat(filepath.Join(root, ".harness", "skills", proposal.ID, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if err := (Store{}).Generate(root, proposal); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(harness)
	if string(data2) != string(data) {
		t.Fatal("repeated generation duplicated a skill")
	}

	manualRoot := t.TempDir()
	manualHarness := filepath.Join(manualRoot, ".harness", "harness.yaml")
	if err := os.MkdirAll(filepath.Dir(manualHarness), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manualHarness, []byte(original+"skills:\n  - id: manual\n    description: keep\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := (Store{}).Generate(manualRoot, proposal); err == nil {
		t.Fatal("manual skills section was overwritten")
	}
}
