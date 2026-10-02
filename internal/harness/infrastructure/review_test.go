package infrastructure

import (
	"bytes"
	"context"
	"harnessforge/internal/harness/application"
	"os"
	"path/filepath"
	"testing"
)

func TestReviewPreservesBytesAndControlsTransitions(t *testing.T) {
	original := []byte("# cabeçalho\r\nversion: 1\r\nproject: {name: sample-project}\r\nrules:\r\n  - id: sample\r\n    description: 'Manually written' # keep\r\n    origin: human\r\n    status: 'candidate' # decision\r\n")
	path := filepath.Join(t.TempDir(), "harness.yaml")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	review := application.NewReview(YAMLRuleStore{})
	if err := review.Execute(path, "sample", "approved"); err != nil {
		t.Fatal(err)
	}
	actual, _ := os.ReadFile(path)
	want := bytes.Replace(original, []byte("'candidate'"), []byte("'approved'"), 1)
	if !bytes.HasPrefix(actual, want) || !bytes.Contains(actual, []byte("review:\n      content_sha256: ")) || !bytes.Contains(actual, []byte("      evidence_sha256: ")) {
		t.Fatalf("unexpected formatting: %q", actual)
	}
	if err := review.Execute(path, "sample", "rejected"); err == nil {
		t.Fatal("reversal accepted without reopening")
	}
	if err := review.Execute(path, "missing", "approved"); err == nil {
		t.Fatal("missing rule accepted")
	}
	unchanged, _ := os.ReadFile(path)
	if !bytes.Equal(unchanged, actual) {
		t.Fatal("failed review changed file")
	}
	if err := review.Execute(path, "sample", "candidate"); err != nil {
		t.Fatal(err)
	}
	if err := review.Execute(path, "sample", "rejected"); err != nil {
		t.Fatal(err)
	}
}

func TestApprovedRuleRequiresReReviewWhenContentOrEvidenceChanges(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".harness", "harness.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "source.go")
	if err := os.WriteFile(source, []byte("func Execute() { first() }"), 0600); err != nil {
		t.Fatal(err)
	}
	doc := []byte("version: 1\nproject: {name: sample}\nrules:\n  - id: sample\n    description: call Execute\n    origin: ai\n    status: candidate\n    evidence:\n      - file: source.go\n        symbol: Execute\n")
	if err := os.WriteFile(path, doc, 0600); err != nil {
		t.Fatal(err)
	}
	review := application.NewReview(YAMLRuleStore{}, EvidenceRevalidator{Root: root})
	if err := review.Execute(path, "sample", "approved"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("func Execute() { second() }"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := review.Execute(path, "sample", "approved"); err == nil {
		t.Fatal("changed evidence retained approval")
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated = bytes.Replace(updated, []byte("description: call Execute"), []byte("description: changed Execute"), 1)
	if err := os.WriteFile(path, updated, 0600); err != nil {
		t.Fatal(err)
	}
	if err := review.Execute(path, "sample", "approved"); err == nil {
		t.Fatal("changed rule content retained approval")
	}
}

func TestReviewRequiresCheckerForRulesWithEvidence(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "harness.yaml")
	doc := []byte("version: 1\nproject: {name: sample}\nrules:\n  - id: sample\n    description: verified\n    origin: human\n    status: candidate\n    evidence:\n      - file: source.go\n")
	if err := os.WriteFile(path, doc, 0600); err != nil {
		t.Fatal(err)
	}
	if err := application.NewReview(YAMLRuleStore{}).Execute(path, "sample", "approved"); err == nil {
		t.Fatal("rule with unvalidated evidence was approved")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, doc) {
		t.Fatal("failed review changed file")
	}
}
func TestEvidenceChecksFilesAndSymbols(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "harness.yaml")
	document := []byte("version: 1\nproject: {name: test}\nrules:\n  - id: sample\n    description: test\n    origin: ai\n    status: candidate\n    evidence:\n      - file: source.go\n        symbol: Execute\n")
	if err := os.WriteFile(path, document, 0600); err != nil {
		t.Fatal(err)
	}
	h, err := (YAMLLoader{}).Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckEvidence(context.Background(), root, h); err == nil {
		t.Fatal("missing file accepted")
	}
	if err := os.WriteFile(filepath.Join(root, "source.go"), []byte("func Execute() {}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := CheckEvidence(context.Background(), root, h); err != nil {
		t.Fatal(err)
	}
	h.Rules[0].Evidence[0].Symbol = "Missing"
	if err := CheckEvidence(context.Background(), root, h); err == nil {
		t.Fatal("missing symbol accepted")
	}
	h.Rules[0].Evidence[0].File = "../outside"
	if err := CheckEvidence(context.Background(), root, h); err == nil {
		t.Fatal("traversal accepted")
	}
}

func TestReviewRejectsAnchoredStatus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "harness.yaml")
	original := []byte("version: 1\nproject: {name: sample}\nrules:\n  - id: rule\n    description: test\n    origin: human\n    status: &shared candidate\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := application.NewReview(YAMLRuleStore{}).Execute(path, "rule", "approved"); err == nil {
		t.Fatal("anchored value edited")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(original, after) {
		t.Fatal("file changed")
	}
}
