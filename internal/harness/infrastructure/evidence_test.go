package infrastructure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"harnessforge/internal/harness/domain"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvidenceAtGitRevision(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init")
	git("config", "user.email", "test@example.invalid")
	git("config", "user.name", "Test")
	path := filepath.Join(root, "source.txt")
	if err := os.WriteFile(path, []byte("OriginalSymbol"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "original")
	revision := git("rev-parse", "HEAD")
	if err := os.WriteFile(path, []byte("ChangedSymbol"), 0600); err != nil {
		t.Fatal(err)
	}
	h := domain.Harness{Rules: []domain.Rule{{Evidence: []domain.Evidence{{File: "source.txt", Revision: revision, Symbol: "OriginalSymbol"}}}}}
	if err := CheckEvidence(context.Background(), root, h); err != nil {
		t.Fatal(err)
	}
	h.Rules[0].Evidence[0].Revision = ""
	if err := CheckEvidence(context.Background(), root, h); err == nil {
		t.Fatal("current content did not match but passed")
	}
}

func TestEvidenceQuoteMustExistInFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("observed convention"), 0600); err != nil {
		t.Fatal(err)
	}
	h := domain.Harness{Rules: []domain.Rule{{Evidence: []domain.Evidence{{File: "source.txt", Quote: "observed convention"}}}}}
	if err := CheckEvidence(context.Background(), root, h); err != nil {
		t.Fatal(err)
	}
	h.Rules[0].Evidence[0].Quote = "missing"
	if err := CheckEvidence(context.Background(), root, h); err == nil {
		t.Fatal("missing quote accepted")
	}
}

func TestEvidenceSHA256MustMatchFile(t *testing.T) {
	root := t.TempDir()
	content := []byte("observed convention")
	if err := os.WriteFile(filepath.Join(root, "source.txt"), content, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	h := domain.Harness{Rules: []domain.Rule{{Evidence: []domain.Evidence{{File: "source.txt", SHA256: hex.EncodeToString(sum[:])}}}}}
	if err := CheckEvidence(context.Background(), root, h); err != nil {
		t.Fatal(err)
	}
	h.Rules[0].Evidence[0].SHA256 = strings.Repeat("0", 64)
	if err := CheckEvidence(context.Background(), root, h); err == nil {
		t.Fatal("mismatched evidence hash accepted")
	}
}
