package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"harnessforge/internal/harness/domain"
)

func TestValidateManifestReferencesChecksFilesAndWorkspacesWithoutRunningCommands(t *testing.T) {
	root := t.TempDir()
	layout := ProjectLayout{Root: root, Kind: LayoutForge}
	for _, name := range []string{".forge/knowledge/architecture.md", ".forge/skills/review/SKILL.md"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	manifest := domain.Manifest{
		References: domain.ManifestReferences{
			Knowledge: []domain.KnowledgeReference{{ID: "architecture", Path: ".forge/knowledge/architecture.md"}},
			Skills:    []string{".forge/skills/review/SKILL.md"},
		},
		QualityGates: []domain.QualityGate{{ID: "test", Command: "this command must not run", Workspace: ".", Workspaces: []string{"."}}},
	}
	if err := ValidateManifestReferences(layout, manifest); err != nil {
		t.Fatal(err)
	}
}

func TestValidateManifestReferencesReportsMissingAndRejectsSymlinks(t *testing.T) {
	root := t.TempDir()
	layout := ProjectLayout{Root: root, Kind: LayoutForge}
	manifest := domain.Manifest{References: domain.ManifestReferences{Knowledge: []domain.KnowledgeReference{{ID: "missing", Path: ".forge/knowledge/missing.md"}}}}
	if err := ValidateManifestReferences(layout, manifest); err == nil || !strings.Contains(err.Error(), "references.knowledge[missing].path") {
		t.Fatalf("missing reference diagnostic: %v", err)
	}
	target := filepath.Join(root, "outside.md")
	if err := os.WriteFile(target, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(root, ".forge", "knowledge", "linked.md")
	if err := os.MkdirAll(filepath.Dir(symlink), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, symlink); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	manifest.References.Knowledge[0].Path = ".forge/knowledge/linked.md"
	if err := ValidateManifestReferences(layout, manifest); err == nil || !strings.Contains(err.Error(), "regular non-symlink") {
		t.Fatalf("symlink reference accepted: %v", err)
	}
}
