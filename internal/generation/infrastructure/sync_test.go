package infrastructure

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
	"harnessforge/internal/harness/domain"
	harnessinfra "harnessforge/internal/harness/infrastructure"
)

type cancelAfterErrContext struct {
	context.Context
	remaining int
}

func (c *cancelAfterErrContext) Err() error {
	if c.remaining > 0 {
		c.remaining--
		return nil
	}
	return context.Canceled
}

func (c *cancelAfterErrContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *cancelAfterErrContext) Done() <-chan struct{}       { return nil }
func (c *cancelAfterErrContext) Value(key any) any           { return c.Context.Value(key) }

func forgeSyncFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	itemDir := filepath.Join(root, ".forge", "knowledge", "items")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}
	item := domain.KnowledgeItem{ID: "rule-a", Kind: domain.KnowledgeConvention, Content: "Use explicit errors.", Origin: "human", Review: domain.KnowledgeApproved, Health: domain.KnowledgeUnknown, Reviewer: "alice", ContentSHA256: domain.HashKnowledgeContent("Use explicit errors."), ReviewDiff: "--- candidate\n+++ reviewed\n+Use explicit errors.\n", EvidenceSHA256: domain.HashKnowledgeEvidence(nil)}
	data, err := yaml.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	itemPath := filepath.Join(itemDir, "rule.md")
	if err := os.WriteFile(itemPath, []byte("---\n"+string(data)+"---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := `layout_version: 1
ir_version: 2
project:
  name: demo
  languages: [Go]
targets: [codex]
references:
  knowledge:
    - id: rule-a
      path: .forge/knowledge/items/rule.md
`
	if err := os.WriteFile(filepath.Join(root, ".forge", "forge.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestSyncForgePreviewApplyCheckAndCloneOwnership(t *testing.T) {
	root := forgeSyncFixture(t)
	preview, err := SyncForge(context.Background(), root, "dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Changed || len(preview.Files) != 1 {
		t.Fatalf("bad preview: %#v", preview)
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatal("dry-run wrote output")
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "check"); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, generatedManifest)
	if _, err := os.Stat(manifest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	firstManifest, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	firstOutput, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	secondManifest, _ := os.ReadFile(manifest)
	secondOutput, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if string(firstManifest) != string(secondManifest) || string(firstOutput) != string(secondOutput) {
		t.Fatal("second apply changed deterministic bytes")
	}
	if _, err := SyncForge(context.Background(), root, "check"); err != nil {
		t.Fatalf("check should be clean immediately after apply: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("edited generated file"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "check"); err == nil {
		t.Fatal("check ignored output hash drift")
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err == nil {
		t.Fatal("apply overwrote an edited generated output")
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), firstOutput, 0644); err != nil {
		t.Fatal(err)
	}
	// The shared manifest makes ownership verifiable in a clone without local state.
	clone := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(clone, ".forge", "knowledge", "items"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".forge/forge.yaml", ".forge/knowledge/items/rule.md", "AGENTS.md", generatedManifest} {
		raw, e := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if e != nil {
			t.Fatal(e)
		}
		if err := os.WriteFile(filepath.Join(clone, filepath.FromSlash(rel)), raw, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := SyncForge(context.Background(), clone, "apply"); err != nil {
		t.Fatalf("clone ownership was not recognized: %v", err)
	}
}

func TestSyncForgeRejectsEvidenceChangedAfterApproval(t *testing.T) {
	root := forgeSyncFixture(t)
	document := filepath.Join(root, ".forge", "knowledge", "items", "rule.md")
	item := domain.KnowledgeItem{ID: "rule-a", Kind: domain.KnowledgeConvention, Content: "Use explicit errors.", Origin: "human", Review: domain.KnowledgeApproved, Health: domain.KnowledgeVerified, Reviewer: "alice", ContentSHA256: domain.HashKnowledgeContent("Use explicit errors."), ReviewDiff: "--- candidate\n+++ reviewed\n+Use explicit errors.\n", Evidence: []domain.KnowledgeEvidence{{Path: "source.go", Quote: "original"}}, EvidenceSHA256: domain.HashKnowledgeEvidence([]string{"source.go:placeholder"})}
	if err := os.WriteFile(filepath.Join(root, "source.go"), []byte("original source"), 0600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := harnessinfra.KnowledgeFingerprint(root, ".forge/knowledge/items/rule.md", "rule-a", item.Evidence)
	if err != nil {
		t.Fatal(err)
	}
	item.EvidenceSHA256 = fingerprint
	encoded, err := yaml.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(document, append(append([]byte("---\n"), encoded...), []byte("---\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileForge(root); err != nil {
		t.Fatalf("valid approved evidence rejected: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "source.go"), []byte("edited source"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileForge(root); err == nil || !strings.Contains(err.Error(), "evidence") {
		t.Fatalf("stale evidence published: %v", err)
	}
}

func TestSyncForgeProtectsUnownedAndEditedFiles(t *testing.T) {
	root := forgeSyncFixture(t)
	target := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(target, []byte("human instructions\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err == nil || !strings.Contains(err.Error(), "unowned") {
		t.Fatalf("expected unowned collision, got %v", err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("manual edit\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err == nil || !strings.Contains(err.Error(), "edited") {
		t.Fatalf("expected edited-file conflict, got %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "manual edit\n" {
		t.Fatal("manual content was overwritten")
	}
}

func TestCompileForgeCarriesPolicyCapabilityNotes(t *testing.T) {
	root := forgeSyncFixture(t)
	manifestPath := filepath.Join(root, ".forge", "forge.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := string(data) + "policies:\n  - id: network\n    description: Do not use network\n    capability: advisory\n    executor: text\n"
	if err := os.WriteFile(manifestPath, []byte(updated), 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := CompileForge(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan.Diff["AGENTS.md"], "capability: **advisory**") || !strings.Contains(plan.Diff["AGENTS.md"], "does not enforce system permissions") {
		t.Fatalf("policy limit missing from output: %s", plan.Diff["AGENTS.md"])
	}
}

func TestSyncRemovesOnlyIntactPreviouslyOwnedTarget(t *testing.T) {
	root := forgeSyncFixture(t)
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".forge", "forge.yaml"), []byte(`layout_version: 1
ir_version: 2
project: {name: demo, languages: [Go]}
targets: [claude]
references:
  knowledge:
    - id: rule-a
      path: .forge/knowledge/items/rule.md
`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatal("intact stale generated output wasn't removed")
	}
	if _, err := os.Stat(filepath.Join(root, "CLAUDE.md")); err != nil {
		t.Fatal("current target missing")
	}
	if _, err := SyncForge(context.Background(), root, "check"); err != nil {
		t.Fatalf("updated target ownership not current: %v", err)
	}
}

func TestSyncRefusesToRemoveEditedStaleTarget(t *testing.T) {
	root := forgeSyncFixture(t)
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".forge", "forge.yaml"), []byte(`layout_version: 1
ir_version: 2
project: {name: demo, languages: [Go]}
targets: [claude]
references:
  knowledge:
    - id: rule-a
      path: .forge/knowledge/items/rule.md
`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("human edit"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err == nil || !strings.Contains(err.Error(), "manually edited") {
		t.Fatalf("edited stale target removed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil || string(data) != "human edit" {
		t.Fatalf("manual bytes lost: %q %v", data, err)
	}
}

func TestSyncRestoresStaleTargetWhenApplyIsCanceledMidCommit(t *testing.T) {
	root := forgeSyncFixture(t)
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	stalePath := filepath.Join(root, "AGENTS.md")
	staleBefore, err := os.ReadFile(stalePath)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, generatedManifest)
	manifestBefore, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".forge", "forge.yaml"), []byte(`layout_version: 1
ir_version: 2
project: {name: demo, languages: [Go]}
targets: [claude]
references:
  knowledge:
    - id: rule-a
      path: .forge/knowledge/items/rule.md
`), 0600); err != nil {
		t.Fatal(err)
	}

	// Let the pre-commit and stale-removal checks pass, then cancel before the
	// replacement target is written. Rollback must restore both prior files.
	ctx := &cancelAfterErrContext{Context: context.Background(), remaining: 2}
	if _, err := SyncForge(ctx, root, "apply"); err != context.Canceled {
		t.Fatalf("expected mid-commit cancellation, got %v", err)
	}
	staleAfter, err := os.ReadFile(stalePath)
	if err != nil || string(staleAfter) != string(staleBefore) {
		t.Fatalf("stale generated target was not restored: %q, %v", staleAfter, err)
	}
	manifestAfter, err := os.ReadFile(manifestPath)
	if err != nil || string(manifestAfter) != string(manifestBefore) {
		t.Fatalf("prior generated manifest was not preserved: %q, %v", manifestAfter, err)
	}
	if _, err := os.Stat(filepath.Join(root, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatalf("partial apply left replacement output behind: %v", err)
	}
}

func TestSyncForgeRejectsSymlinkTarget(t *testing.T) {
	root := forgeSyncFixture(t)
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "AGENTS.md")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := SyncForge(context.Background(), root, "dry-run"); err == nil {
		t.Fatal("symlink target was accepted")
	}
}
