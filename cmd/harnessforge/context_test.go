package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"
	"harnessforge/internal/harness/domain"
)

func TestContextExplainExposesSelectionFlags(t *testing.T) {
	command := newContextCommand()
	explain, _, err := command.Find([]string{"explain"})
	if err != nil {
		t.Fatal(err)
	}
	flag := explain.Flags().Lookup("bm25")
	if flag == nil {
		t.Fatal("--bm25 flag is not exposed")
	}
	if flag.DefValue != "false" {
		t.Fatalf("default bm25 = %q", flag.DefValue)
	}
	if taskPath := explain.Flags().Lookup("task-path"); taskPath == nil {
		t.Fatal("--task-path flag is not exposed")
	}
	if compare := explain.Flags().Lookup("compare-knowledge"); compare == nil {
		t.Fatal("--compare-knowledge flag is not exposed")
	}
}

func TestContextExplainComparesSamePlanWithAndWithoutApprovedKnowledge(t *testing.T) {
	root := t.TempDir()
	forgeDir := filepath.Join(root, ".forge")
	if err := os.MkdirAll(filepath.Join(forgeDir, "knowledge"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("Authentication tokens are used by the API.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	item := domain.KnowledgeItem{
		ID: "auth-expiration", Kind: domain.KnowledgeConvention,
		Content: "Authentication tokens expire after fifteen minutes.", Origin: "reviewed project convention",
		Review: domain.KnowledgeApproved, Health: domain.KnowledgeVerified, Reviewer: "alice",
		ReviewDiff: "+Authentication tokens expire after fifteen minutes.\n",
	}
	item.ContentSHA256 = domain.HashKnowledgeContent(item.Content)
	item.EvidenceSHA256 = domain.HashKnowledgeEvidence(nil)
	item.ReviewMetadataSHA256 = domain.HashKnowledgeReviewMetadata(item)
	knowledgeYAML, err := yaml.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	knowledgeDoc := append(append([]byte("---\n"), knowledgeYAML...), []byte("---\n")...)
	if err := os.WriteFile(filepath.Join(forgeDir, "knowledge", "auth.md"), knowledgeDoc, 0600); err != nil {
		t.Fatal(err)
	}
	manifest := "layout_version: 1\nir_version: 2\nproject:\n  name: auth-service\n  languages: [Go]\ntargets: [codex]\nreferences:\n  knowledge:\n    - id: auth-expiration\n      path: .forge/knowledge/auth.md\n"
	if err := os.WriteFile(filepath.Join(forgeDir, "forge.yaml"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}

	command := newContextCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"explain", root, "How long do authentication tokens last?", "--compare-knowledge", "--budget", "4000", "--model", "gpt-4o-mini"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	var report contextComparisonReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("invalid comparison JSON: %v\n%s", err, output.String())
	}
	if report.MetricNature != "paired_context_selection_proxy_not_task_quality" || !report.SameEstimator {
		t.Fatalf("comparison omitted proxy or estimator metadata: %#v", report)
	}
	for _, excerpt := range report.Baseline.Included {
		if excerpt.KnowledgeID != "" {
			t.Fatalf("baseline unexpectedly included Forge knowledge: %#v", excerpt)
		}
	}
	found := false
	for _, id := range report.SelectedKnowledgeIDs {
		if id == "auth-expiration" {
			found = true
		}
	}
	if !found || report.EstimatedTokenDelta != report.WithApprovedKnowledge.EstimatedTokens-report.Baseline.EstimatedTokens {
		t.Fatalf("knowledge comparison did not report selected item and matching token delta: %#v", report)
	}
}
