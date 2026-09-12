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
	original := []byte("# cabeçalho\r\nversion: 1\r\nproject: {name: mili}\r\nrules:\r\n  - id: sample\r\n    description: 'Manually written' # keep\r\n    origin: human\r\n    status: 'candidate' # decision\r\n")
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
	if !bytes.Equal(actual, want) {
		t.Fatalf("unexpected formatting: %q", actual)
	}
	if err := review.Execute(path, "sample", "rejected"); err == nil {
		t.Fatal("reversal accepted without reopening")
	}
	if err := review.Execute(path, "missing", "approved"); err == nil {
		t.Fatal("missing rule accepted")
	}
	unchanged, _ := os.ReadFile(path)
	if !bytes.Equal(unchanged, want) {
		t.Fatal("failed review changed file")
	}
	if err := review.Execute(path, "sample", "candidate"); err != nil {
		t.Fatal(err)
	}
	if err := review.Execute(path, "sample", "rejected"); err != nil {
		t.Fatal(err)
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
