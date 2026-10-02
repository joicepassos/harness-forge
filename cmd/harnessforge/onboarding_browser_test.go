package main

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextBrowserNavigateMarkReviewAndToggle(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "docs"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"README.md", "docs/a.md", "docs/b.md"} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte("Project documentation."), 0644); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	// ReadDir sorts README first, then docs. Mark the directory, review/remove
	// a.md, enter docs, toggle b.md off, then select a.md and go back to root.
	session := setupSession{reader: bufio.NewReader(strings.NewReader("m 2\nr\n1\n2\n2\n1\n..\n..\n\n")), output: &output}
	files, err := session.selectedDocuments(context.Background(), root, nil)
	if err != nil || len(files) != 1 || files[0].Source != "repository-file:docs/a.md" {
		t.Fatalf("selection = %#v, error = %v; output = %s", files, err, output.String())
	}
	if !strings.Contains(output.String(), "[x] b.md") {
		t.Fatalf("selection marker missing: %s", output.String())
	}
}

func TestContextBrowserHidesUnsafePathsAndStaysRooted(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"README.md", ".env", "secret.key", "picture.png"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	entries, err := setupBrowserEntries(root, root)
	if err != nil || len(entries) != 1 || entries[0].Name() != "README.md" {
		t.Fatalf("entries = %#v, error = %v", entries, err)
	}
	if _, err := setupBrowserEntries(root, filepath.Dir(root)); err == nil {
		t.Fatal("browser accepted outside directory")
	}
}

func TestContextBrowserRejectsCredentialContent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("OPENAI_API_KEY=sk-test-secret-value"), 0644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	session := setupSession{reader: bufio.NewReader(strings.NewReader("1\n\n")), output: &output}
	files, err := session.selectedDocuments(context.Background(), root, nil)
	if err != nil || len(files) != 0 || !strings.Contains(output.String(), "credentials") {
		t.Fatalf("files = %#v, error = %v, output = %s", files, err, output.String())
	}
}
