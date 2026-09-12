package analyzer

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitForTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git: %v %s", err, out)
	}
}
func TestGitMetadataAndFailures(t *testing.T) {
	root := t.TempDir()
	gitForTest(t, root, "init", "-b", "main")
	empty, err := AnalyzeWithOptions(context.Background(), root, Options{IncludeGit: true})
	if err != nil {
		t.Fatal(err)
	}
	assertFinding(t, empty.Git, "Commits: 0")
	gitForTest(t, root, "config", "user.email", "test@example.invalid")
	gitForTest(t, root, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "sample.txt"), []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	gitForTest(t, root, "add", ".")
	gitForTest(t, root, "commit", "-m", "first-message")
	if err := os.WriteFile(filepath.Join(root, "sample.txt"), []byte("second"), 0600); err != nil {
		t.Fatal(err)
	}
	gitForTest(t, root, "commit", "-am", "second-message")
	gitForTest(t, root, "branch", "feature")
	if err := os.WriteFile(filepath.Join(root, "pending.txt"), []byte("pending"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := AnalyzeWithOptions(context.Background(), root, Options{IncludeGit: true})
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, f := range result.Git {
		joined += f.Value + "\n"
	}
	for _, want := range []string{"feature", "first-message", "second-message", "pending.txt", "sample.txt"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %s: %s", want, joined)
		}
	}
	if _, err := AnalyzeWithOptions(context.Background(), t.TempDir(), Options{IncludeGit: true}); err == nil {
		t.Fatal("non-git error ignored")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := AnalyzeWithOptions(ctx, root, Options{IncludeGit: true}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}
