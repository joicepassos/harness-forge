package main

import (
	"bytes"
	"encoding/json"
	"go.yaml.in/yaml/v3"
	"os"
	"path/filepath"
	"strings"
	"testing"

	driftdomain "harnessforge/internal/drift/domain"
	harnessdomain "harnessforge/internal/harness/domain"
	harnessinfra "harnessforge/internal/harness/infrastructure"
)

func TestDriftCommandDiscoversForgeKnowledgeAndDoesNotClaimConformance(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "evidence.go"), []byte("package example\nconst TokenLifetime = 15\n"), 0600); err != nil {
		t.Fatal(err)
	}
	writeDriftKnowledgeFixture(t, root)

	cmd := newRootCommand()
	cmd.SetArgs([]string{"drift", "--repository", root})
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var report driftdomain.Report
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("invalid report %q: %v", out, err)
	}
	if len(report.Occurrences) != 1 {
		t.Fatalf("want one approved knowledge occurrence, got %#v", report.Occurrences)
	}
	occurrence := report.Occurrences[0]
	if occurrence.SubjectType != "knowledge" || occurrence.RuleID != "knowledge:auth-policy" || occurrence.Status != driftdomain.StatusAligned || occurrence.EvidenceStatus != driftdomain.EvidencePresent || occurrence.Conformance != driftdomain.StatusNotEvaluated {
		t.Fatalf("unexpected unchanged knowledge result: %#v", occurrence)
	}
	if !strings.Contains(strings.Join(occurrence.Explanations, " "), "not proof of rule conformance") {
		t.Fatalf("missing conformance limitation: %#v", occurrence.Explanations)
	}

	if err := os.WriteFile(filepath.Join(root, "evidence.go"), []byte("package example\nconst TokenTTL = 15\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd = newRootCommand()
	cmd.SetArgs([]string{"drift", "--repository", root, "--layout", "forge"})
	out = new(bytes.Buffer)
	cmd.SetOut(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("invalid report %q: %v", out, err)
	}
	if len(report.Occurrences) != 1 || report.Occurrences[0].Status != driftdomain.StatusDifference || report.Occurrences[0].EvidenceStatus != driftdomain.EvidenceChanged || report.Occurrences[0].Conformance != driftdomain.StatusNotEvaluated {
		t.Fatalf("changed evidence was not reported honestly: %#v", report.Occurrences)
	}
	if err := os.Remove(filepath.Join(root, "evidence.go")); err != nil {
		t.Fatal(err)
	}
	cmd = newRootCommand()
	cmd.SetArgs([]string{"drift", "--repository", root, "--layout", "forge"})
	out = new(bytes.Buffer)
	cmd.SetOut(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("invalid report %q: %v", out, err)
	}
	if len(report.Occurrences) != 1 || report.Occurrences[0].Status != driftdomain.StatusDifference || report.Occurrences[0].EvidenceStatus != driftdomain.EvidenceMissing {
		t.Fatalf("missing evidence was not reported: %#v", report.Occurrences)
	}
}

func writeDriftKnowledgeFixture(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".forge", "knowledge"), 0700); err != nil {
		t.Fatal(err)
	}
	evidence := []harnessdomain.KnowledgeEvidence{{Path: "evidence.go", Quote: "TokenLifetime"}}
	fingerprint, err := harnessinfra.KnowledgeFingerprint(root, ".forge/knowledge/auth-policy.md", "auth-policy", evidence)
	if err != nil {
		t.Fatal(err)
	}
	item := harnessdomain.KnowledgeItem{ID: "auth-policy", Kind: harnessdomain.KnowledgeFact, Content: "Token lifetime is fifteen minutes.", Origin: "test", Review: harnessdomain.KnowledgeApproved, Health: harnessdomain.KnowledgeVerified, Evidence: evidence, Reviewer: "reviewed", ReviewDiff: "--- candidate\n+++ reviewed\n+Token lifetime is fifteen minutes.\n", ContentSHA256: harnessdomain.HashKnowledgeContent("Token lifetime is fifteen minutes."), EvidenceSHA256: fingerprint}
	encoded, err := yaml.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	doc := append(append([]byte("---\n"), encoded...), []byte("---\n")...)
	if err := os.WriteFile(filepath.Join(root, ".forge", "knowledge", "auth-policy.md"), doc, 0600); err != nil {
		t.Fatal(err)
	}
	manifest := "layout_version: 1\nir_version: 2\nproject: {name: sample}\ntargets: [codex]\nreferences:\n  knowledge:\n    - id: auth-policy\n      path: .forge/knowledge/auth-policy.md\n"
	if err := os.WriteFile(filepath.Join(root, ".forge", "forge.yaml"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
}
