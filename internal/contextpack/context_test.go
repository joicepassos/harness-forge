package contextpack

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildExplainsRankingDedupCompressionAndBudget(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "internal/webhook/handler.go", strings.Repeat("webhook authentication persists event monitoring alert ", 80))
	writeFile(t, root, "internal/webhook/copy.go", strings.Repeat("webhook authentication persists event monitoring alert ", 80))
	writeFile(t, root, "docs/irrelevant.txt", "color palette vacation schedule")

	plan, err := Build(context.Background(), root, "How are webhooks authenticated persisted and monitored?", "", Options{BudgetTokens: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if plan.EstimatedTokens > plan.BudgetTokens {
		t.Fatalf("estimated tokens %d exceed budget %d", plan.EstimatedTokens, plan.BudgetTokens)
	}
	if len(plan.Included) == 0 {
		t.Fatal("expected included excerpts")
	}
	foundWebhookFile := false
	foundDuplicate := false
	foundIrrelevant := false
	foundCompressed := false
	for _, excerpt := range append(plan.Included, plan.Excluded...) {
		if excerpt.Path == "internal/webhook/copy.go" || excerpt.Path == "internal/webhook/handler.go" {
			foundWebhookFile = true
		}
		if strings.Contains(excerpt.Reason, "duplicate") {
			foundDuplicate = true
		}
		if strings.Contains(excerpt.Reason, "not relevant") {
			foundIrrelevant = true
		}
		if excerpt.CompressedFrom != "" {
			foundCompressed = true
		}
	}
	if !foundDuplicate {
		t.Fatal("duplicate was not explained")
	}
	if !foundWebhookFile {
		t.Fatal("webhook file was not considered")
	}
	if !foundIrrelevant {
		t.Fatal("irrelevant document was not explained")
	}
	if !foundCompressed {
		t.Fatal("oversized excerpt was not compressed with provenance")
	}
	if plan.Comparison.UnfilteredCandidateEstimatedTokens <= plan.Comparison.SelectedEstimatedTokens {
		t.Fatalf("expected selected context to cost less than baseline: %+v", plan.Comparison)
	}
}

func TestBuildSkipsSecretsSymlinksAndUnsafePaths(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "safe.txt", "webhook authentication")
	writeFile(t, root, ".env", "TOKEN=secret")
	writeFile(t, root, "service.token", "secret")
	writeFile(t, root, "password-template.yml", "webhook password authentication")
	writeFile(t, root, ".runtime/cache.txt", "webhook authentication")
	if runtime.GOOS != "windows" {
		if err := os.Symlink(filepath.Join(root, ".env"), filepath.Join(root, "linked.txt")); err != nil {
			t.Fatal(err)
		}
	}

	plan, err := Build(context.Background(), root, "webhook authentication", "", Options{BudgetTokens: 300})
	if err != nil {
		t.Fatal(err)
	}
	all := append(plan.Included, plan.Excluded...)
	for _, excerpt := range all {
		if strings.Contains(excerpt.Text, "TOKEN=") || strings.Contains(excerpt.Text, "secret") {
			t.Fatalf("unsafe excerpt leaked: %+v", excerpt)
		}
		if strings.Contains(excerpt.Path, ".runtime") {
			t.Fatalf("sensitive or dot-runtime path was considered: %+v", excerpt)
		}
		if strings.Contains(excerpt.Path, "password-template") && excerpt.Text != "" {
			t.Fatalf("sensitive path leaked content: %+v", excerpt)
		}
	}
}

func TestBuildHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Build(ctx, t.TempDir(), "prompt", "", Options{}); err != context.Canceled {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestBuildRejectsBudgetBelowPromptEnvelope(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "safe.txt", "webhook authentication")
	if _, err := Build(context.Background(), root, "webhook authentication", "", Options{BudgetTokens: 10}); err == nil {
		t.Fatal("tiny context budget accepted")
	}
}

func TestBuildFiltersNaturalLanguageStopwordsAndAgentInstructions(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/webhook/WebhookAuthentication.java", "class WebhookAuthentication { void authenticateWebhook() { persist(); monitor(); } }")
	writeFile(t, root, "src/operation/OperationPostgresTest.java", "class OperationPostgresTest { void howAreAndThe() {} }")
	writeFile(t, root, "CLAUDE.md", "webhook authentication persistence monitoring")

	plan, err := Build(context.Background(), root, "How are inbound webhooks received authenticated persisted and monitored?", "", Options{BudgetTokens: 1600})
	if err != nil {
		t.Fatal(err)
	}
	foundAuthSource := false
	for _, excerpt := range plan.Included {
		if excerpt.Path == "src/webhook/WebhookAuthentication.java" {
			foundAuthSource = true
		}
		if excerpt.Path == "src/operation/OperationPostgresTest.java" || excerpt.Path == "CLAUDE.md" {
			t.Fatalf("irrelevant or instruction file included: %+v", excerpt)
		}
	}
	if !foundAuthSource {
		t.Fatalf("legitimate authentication source not included: %+v", plan)
	}
	foundInstructionExclusion := false
	for _, excerpt := range plan.Excluded {
		if excerpt.Path == "CLAUDE.md" && excerpt.Reason == "agent instruction file skipped" && excerpt.Text == "" {
			foundInstructionExclusion = true
		}
	}
	if !foundInstructionExclusion {
		t.Fatal("agent instruction file was not safely excluded")
	}
}

func writeFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
