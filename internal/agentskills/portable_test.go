package agentskills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadValidatesPortableSkillAndPreservesResources(t *testing.T) {
	rootPath := t.TempDir()
	writeSkillFile(t, rootPath, "skills/review/SKILL.md", "---\nname: review\ndescription: Review changes and use when reviewing a pull request.\n---\n\nFollow this process. See [guide](references/guide.md).\n")
	writeSkillFile(t, rootPath, "skills/review/references/guide.md", "Reference details.\n")
	writeSkillFile(t, rootPath, "skills/review/scripts/check.sh", "#!/bin/sh\necho ok\n")
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	bundle, err := Read(root, "skills/review")
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Name != "review" || !strings.Contains(bundle.Description, "Review changes") {
		t.Fatalf("frontmatter not parsed: %#v", bundle)
	}
	if string(bundle.Files["references/guide.md"]) != "Reference details.\n" || len(bundle.SortedPaths()) != 3 {
		t.Fatalf("portable resources not preserved: %#v", bundle.Files)
	}
}

func TestReadRejectsInvalidNameMismatchResourcesAndSymlinks(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		files map[string]string
		want  string
	}{
		{"invalid-name", "---\nname: Review\ndescription: valid description\n---\n\nbody\n", nil, "name must"},
		{"name-mismatch", "---\nname: other\ndescription: valid description\n---\n\nbody\n", nil, "must match"},
		{"unsupported-resource", "---\nname: review\ndescription: valid description\n---\n\nbody\n", map[string]string{"private/data.txt": "no"}, "unsupported skill resource"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rootPath := t.TempDir()
			writeSkillFile(t, rootPath, "skills/review/SKILL.md", tc.body)
			for name, content := range tc.files {
				writeSkillFile(t, rootPath, filepath.ToSlash(filepath.Join("skills/review", name)), content)
			}
			root, err := os.OpenRoot(rootPath)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			if _, err := Read(root, "skills/review/SKILL.md"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}

	rootPath := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(rootPath, "skills", "review", "references"), 0700); err != nil {
		t.Fatal(err)
	}
	writeSkillFile(t, rootPath, "skills/review/SKILL.md", "---\nname: review\ndescription: valid description\n---\n\nbody\n")
	if err := os.Symlink(outside, filepath.Join(rootPath, "skills", "review", "references", "outside.md")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err := Read(root, "skills/review"); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink resource accepted: %v", err)
	}
}

func writeSkillFile(t *testing.T, root, name, content string) {
	t.Helper()
	file := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
