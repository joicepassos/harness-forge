package infrastructure

import (
	"bytes"
	"context"
	"harnessforge/internal/generation/domain"
	"os"
	"path/filepath"
	"testing"
)

func TestAdaptersAreDeterministicAndAgentSpecific(t *testing.T) {
	input := domain.Input{Project: "sample", Rules: []domain.Rule{{ID: "z", Description: "Last"}, {ID: "a", Description: "First", Paths: []string{"internal/"}}}, Commands: []string{"go test ./..."}}
	codex, err := (Markdown{Agent: "codex"}).Render(input)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := (Markdown{Agent: "codex"}).Render(input)
	claude, _ := (Markdown{Agent: "claude"}).Render(input)
	if codex.Path != "AGENTS.md" || claude.Path != "CLAUDE.md" || !bytes.Equal(codex.Content, again.Content) || bytes.Index(codex.Content, []byte("[a]")) > bytes.Index(codex.Content, []byte("[z]")) {
		t.Fatal("non-deterministic adapter output")
	}
	if _, err := (Markdown{Agent: "other"}).Render(input); err == nil {
		t.Fatal("unsupported adapter accepted")
	}
}
func TestWriterProtectsManualFilesAndAllowsOwnedRegeneration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")
	original := []byte("# Manual\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	document := domain.Document{Path: "AGENTS.md", Content: []byte(marker + "\nfirst")}
	if err := (FileWriter{}).Write(context.Background(), dir, document); err == nil {
		t.Fatal("manual file overwritten")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(after, original) {
		t.Fatal("manual bytes changed")
	}
	if err := os.WriteFile(path, []byte(marker+"\nold"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := (FileWriter{}).Write(context.Background(), dir, document); err != nil {
		t.Fatal(err)
	}
	after, _ = os.ReadFile(path)
	if !bytes.Equal(after, document.Content) {
		t.Fatal("owned file not regenerated")
	}
	if err := (FileWriter{}).Write(context.Background(), dir, domain.Document{Path: "../escape", Content: document.Content}); err == nil {
		t.Fatal("unsafe output accepted")
	}
}
func TestWriterRejectsSymlink(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	target := filepath.Join(outside, "manual")
	if err := os.WriteFile(target, []byte(marker), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Skip(err)
	}
	if err := (FileWriter{}).Write(context.Background(), dir, domain.Document{Path: "AGENTS.md", Content: []byte(marker)}); err == nil {
		t.Fatal("symlink accepted")
	}
}
