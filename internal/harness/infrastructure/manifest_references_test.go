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
	for _, name := range []string{".forge/knowledge/architecture.md"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	skill := filepath.Join(root, ".forge", "skills", "review", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skill), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, []byte("---\nname: review\ndescription: Review skill\n---\n\nReview the change.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	manifest := domain.Manifest{
		References: domain.ManifestReferences{
			Knowledge: []domain.KnowledgeReference{{ID: "architecture", Path: ".forge/knowledge/architecture.md"}},
			Skills:    []domain.SkillReference{{ID: "review", Description: "Review skill", Path: ".forge/skills/review/SKILL.md"}},
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

func TestValidateManifestReferencesRejectsSymlinkedAncestorComponents(t *testing.T) {
	for _, tc := range []struct {
		name       string
		workspace  bool
		linkTarget func(root string) string
	}{
		{name: "knowledge ancestor to outside", linkTarget: func(root string) string { return filepath.Join(filepath.Dir(root), "outside-knowledge") }},
		{name: "knowledge ancestor to internal directory", linkTarget: func(root string) string { return filepath.Join(root, "real-knowledge") }},
		{name: "workspace ancestor to outside", workspace: true, linkTarget: func(root string) string { return filepath.Join(filepath.Dir(root), "outside-workspace") }},
		{name: "workspace ancestor to internal directory", workspace: true, linkTarget: func(root string) string { return filepath.Join(root, "real-workspace") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			layout := ProjectLayout{Root: root, Kind: LayoutForge}
			outside := tc.linkTarget(root)
			if err := os.MkdirAll(outside, 0700); err != nil {
				t.Fatal(err)
			}
			if !tc.workspace {
				if err := os.WriteFile(filepath.Join(outside, "item.md"), []byte("content"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(filepath.Join(outside, "go.mod"), []byte("module example\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			link := filepath.Join(root, "apps")
			if err := os.Symlink(outside, link); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			manifest := domain.Manifest{}
			if tc.workspace {
				manifest.QualityGates = []domain.QualityGate{{ID: "test", Workspaces: []string{"apps"}}}
			} else {
				manifest.References.Knowledge = []domain.KnowledgeReference{{ID: "linked", Path: "apps/item.md"}}
			}
			if err := ValidateManifestReferences(layout, manifest); err == nil || !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("ancestor symlink accepted: %v", err)
			}
		})
	}
}
