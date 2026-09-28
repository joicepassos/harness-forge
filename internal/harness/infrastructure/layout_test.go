package infrastructure

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveLayoutSelectsSingleSourceAndRequiresChoiceForCoexistence(t *testing.T) {
	root := t.TempDir()
	harness := filepath.Join(root, ".harness", "harness.yaml")
	forge := filepath.Join(root, ".forge", "forge.yaml")
	if err := os.MkdirAll(filepath.Dir(harness), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(harness, []byte("version: 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	layout, err := ResolveLayout(root, "")
	if err != nil || layout.Kind != LayoutHarness || layout.HarnessPath != harness {
		t.Fatalf("layout=%#v err=%v", layout, err)
	}
	if err := os.MkdirAll(filepath.Dir(forge), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(forge, []byte("layout_version: 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveLayout(root, ""); err == nil || !strings.Contains(err.Error(), "select --layout") {
		t.Fatalf("coexisting layouts must conflict: %v", err)
	}
	layout, err = ResolveLayout(root, "forge")
	if err != nil || layout.Kind != LayoutForge || layout.ManifestPath != forge {
		t.Fatalf("explicit forge selection failed: %#v %v", layout, err)
	}
	layout, err = ResolveLayout(root, "harness")
	if err != nil || layout.Kind != LayoutHarness {
		t.Fatalf("explicit harness selection failed: %#v %v", layout, err)
	}
}

func TestResolveLayoutRejectsSymlinkAndInvalidSelection(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "outside.yaml")
	if err := os.WriteFile(target, []byte("layout_version: 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	forge := filepath.Join(root, ".forge", "forge.yaml")
	if err := os.MkdirAll(filepath.Dir(forge), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, forge); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := ResolveLayout(root, "forge"); err == nil {
		t.Fatal("symlink config accepted")
	}
	if _, err := ResolveLayout(root, "cursor"); err == nil {
		t.Fatal("unknown layout accepted")
	}
}

func TestResolveReferenceRejectsTraversalAndPreservesProjectRelativePaths(t *testing.T) {
	layout := ProjectLayout{Root: filepath.Clean("C:/repo")}
	rootPath, err := layout.ResolveReference(".")
	if err != nil || rootPath != layout.Root {
		t.Fatalf("root workspace resolved to %q: %v", rootPath, err)
	}
	resolved, err := layout.ResolveReference(".forge/knowledge/architecture.md")
	if err != nil || resolved != filepath.Join(layout.Root, ".forge", "knowledge", "architecture.md") {
		t.Fatalf("resolved=%q err=%v", resolved, err)
	}
	for _, invalid := range []string{"../outside.md", "C:/outside.md", "\\\\host\\share", ""} {
		if _, err := layout.ResolveReference(invalid); err == nil {
			t.Errorf("accepted unsafe reference %q", invalid)
		}
	}
}

func TestWindowsWorkspaceFixtureLoadsPortableReferences(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(source), "..", "..", "..", "testdata", "fixtures", "windows-workspaces")
	project, err := LoadProject(root, "forge")
	if err != nil {
		t.Fatal(err)
	}
	if project.Manifest == nil || project.Manifest.Project.Name != "windows-workspaces" {
		t.Fatalf("fixture did not load as a Forge project: %#v", project)
	}
	if len(project.Manifest.QualityGates) != 1 {
		t.Fatalf("fixture quality gates = %#v", project.Manifest.QualityGates)
	}
	gate := project.Manifest.QualityGates[0]
	if gate.Workspace != "apps/windows-service" || len(gate.Workspaces) != 2 || gate.Workspaces[1] != "libs/shared" {
		t.Fatalf("fixture lost workspace roots: %#v", gate)
	}
	references := []string{".forge/knowledge/windows-paths.md", ".forge/skills/windows-build/SKILL.md"}
	references = append(references, gate.Workspaces...)
	for _, reference := range references {
		resolved, err := project.Layout.ResolveReference(reference)
		if err != nil {
			t.Fatalf("resolve portable reference %q: %v", reference, err)
		}
		if _, err := os.Stat(resolved); err != nil {
			t.Fatalf("fixture reference %q does not exist at %q: %v", reference, resolved, err)
		}
	}
	for _, windowsPath := range []string{`C:/repo/src/main.go`, `C:\repo\src\main.go`, `\\server\share\main.go`} {
		if _, err := project.Layout.ResolveReference(windowsPath); err == nil {
			t.Errorf("accepted non-portable Windows path %q", windowsPath)
		}
	}
}
