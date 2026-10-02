package onboarding

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildSetupContextUsesSharedPipeline(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal", "auth.go")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package auth\n// authentication convention\nfunc ValidateToken() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildSetupContext(context.Background(), root, SetupContextRequest{Goal: "understand authentication conventions", BudgetTokens: 900, UseBM25: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Included) == 0 {
		t.Fatal("setup context is empty")
	}
}
