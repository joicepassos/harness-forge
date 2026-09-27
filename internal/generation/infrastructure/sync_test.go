package infrastructure

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
	"harnessforge/internal/harness/domain"
)

func forgeSyncFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	itemDir := filepath.Join(root, ".forge", "knowledge", "items")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}
	item := domain.KnowledgeItem{ID: "rule-a", Kind: domain.KnowledgeConvention, Content: "Use explicit errors.", Origin: "human", Review: domain.KnowledgeApproved, Health: domain.KnowledgeUnknown, Reviewer: "alice", ContentSHA256: domain.HashKnowledgeContent("Use explicit errors.")}
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
