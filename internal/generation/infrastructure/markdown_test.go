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
	input := domain.Input{Project: "sample", Architecture: []string{"hexagonal", "event-driven"}, Rules: []domain.Rule{{ID: "z", Description: "Last"}, {ID: "a", Description: "First", Paths: []string{"internal/"}}}, Skills: []domain.Skill{{ID: "backend", Description: "Backend help", Path: "skills/backend/SKILL.md"}}, Gates: []domain.QualityGate{{ID: "test-api", Command: "go test ./...", Workspace: "services/api", Workspaces: []string{"services/api", "libs/core"}}}}
	codex, err := (Markdown{Agent: "codex"}).Render(input)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := (Markdown{Agent: "codex"}).Render(input)
	claudeDocs, claudeErr := (ClaudeAdapter{}).RenderDocuments(input)
	if claudeErr != nil {
		t.Fatal(claudeErr)
	}
	if codex.Path != "AGENTS.md" || len(claudeDocs) != 2 || claudeDocs[0].Path != "CLAUDE.md" || !bytes.Equal(codex.Content, again.Content) || bytes.Index(codex.Content, []byte("[a]")) > bytes.Index(codex.Content, []byte("[z]")) {
		t.Fatal("non-deterministic adapter output")
	}
	if _, err := (Markdown{Agent: "other"}).Render(input); err == nil {
		t.Fatal("unsupported adapter accepted")
	}
	for _, expected := range []string{"## Architecture", "- event-driven", "- hexagonal", "skills/backend/SKILL.md", "services/api", "libs/core", "test-api"} {
		if !bytes.Contains(codex.Content, []byte(expected)) {
			t.Fatalf("generated instructions omit %q: %s", expected, codex.Content)
		}
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
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	owned := domain.Document{Path: "AGENTS.md", Content: []byte(marker + "\nold")}
	if err := (FileWriter{}).Write(context.Background(), dir, owned); err != nil {
		t.Fatal(err)
	}
	if err := (FileWriter{}).Write(context.Background(), dir, document); err != nil {
		t.Fatal(err)
	}
	after, _ = os.ReadFile(path)
	if !bytes.Equal(after, document.Content) {
		t.Fatal("owned file not regenerated")
	}
	manualEdit := append(append([]byte(nil), document.Content...), []byte("\nmanual edit")...)
	if err := os.WriteFile(path, manualEdit, 0600); err != nil {
		t.Fatal(err)
	}
	if err := (FileWriter{}).Write(context.Background(), dir, owned); err == nil {
		t.Fatal("manually edited generated file overwritten")
	}
	after, _ = os.ReadFile(path)
	if !bytes.Equal(after, manualEdit) {
		t.Fatal("conflict changed manually edited bytes")
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

func TestWriterRegeneratesAppendedSectionWithoutChangingManualInstructions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "AGENTS.md")
	manual := []byte("# Team instructions\n\nKeep reviews small.")
	old := append(append(append([]byte(nil), manual...), []byte("\n\n")...), []byte(marker+"\nold")...)
	if err := os.WriteFile(path, old, 0644); err != nil {
		t.Fatal(err)
	}
	document := domain.Document{Path: "AGENTS.md", Content: []byte(marker + "\nnew")}
	if err := (FileWriter{}).Write(context.Background(), root, document); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := append(append(append([]byte(nil), manual...), []byte("\n\n")...), document.Content...)
	if !bytes.Equal(after, want) {
		t.Fatalf("manual instructions changed: %q", after)
	}
}
