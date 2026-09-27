package infrastructure

import (
	"os"
	"path/filepath"
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
