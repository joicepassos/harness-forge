package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	harnessinfra "harnessforge/internal/harness/infrastructure"
)

func TestOnboardImportCreatesReviewedCandidate(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, ".forge", "forge.yaml")
	if err := os.MkdirAll(filepath.Dir(manifest), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte("layout_version: 1\nir_version: 2\nproject: {name: sample}\ntargets: [codex]\nreferences: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := []byte("Keep authentication on the server.\n")
	if err := os.WriteFile(filepath.Join(root, "SECURITY.md"), source, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCommand()
	cmd.SetArgs([]string{"onboard", "import", "SECURITY.md", "--kind", "convention", "--repository", root})
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "as candidate import-security-") || !strings.Contains(out.String(), "run harnessforge review") {
		t.Fatalf("import did not explain the candidate review step: %s", out)
	}
	project, err := harnessinfra.LoadProject(root, "forge")
	if err != nil {
		t.Fatal(err)
	}
	if len(project.Manifest.References.Knowledge) != 1 {
		t.Fatalf("references = %#v", project.Manifest.References.Knowledge)
	}
	itemPath, err := project.Layout.ResolveReference(project.Manifest.References.Knowledge[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	item, err := os.ReadFile(itemPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(item, []byte("review: candidate")) || !bytes.Contains(item, []byte("Keep authentication on the server.")) {
		t.Fatalf("imported item was not a candidate: %s", item)
	}
	unchanged, err := os.ReadFile(filepath.Join(root, "SECURITY.md"))
	if err != nil || !bytes.Equal(unchanged, source) {
		t.Fatalf("source file changed: %q, %v", unchanged, err)
	}
}
