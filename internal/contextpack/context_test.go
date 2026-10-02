package contextpack

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type buildTokenCounter struct {
	model string
}

func (c *buildTokenCounter) Name() string { return "build-test-counter" }
func (c *buildTokenCounter) Count(_ context.Context, model string, payload []byte) (int, error) {
	c.model = model
	return len(payload) / 4, nil
}

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

func TestBuildCanOptIntoBM25Ranking(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "docs/unrelated.md", "deployment history")
	writeFile(t, root, "internal/auth/validator.go", "JWT authentication validates token")
	plan, err := Build(context.Background(), root, "where is JWT authentication validated?", "", Options{BudgetTokens: 900, UseBM25: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Included) == 0 || plan.Included[0].Path != "internal/auth/validator.go" {
		t.Fatalf("BM25 ranking = %+v", plan.Included)
	}
}

func TestBuildPassesModelAndCounterToSelection(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example\n")
	counter := &buildTokenCounter{}
	plan, err := Build(context.Background(), root, "where is the Go module configured?", "provider-model", Options{BudgetTokens: 900, Counter: counter})
	if err != nil {
		t.Fatal(err)
	}
	if counter.model != "provider-model" || plan.Estimator != "build-test-counter" {
		t.Fatalf("counter/model not propagated: model=%q plan=%+v", counter.model, plan)
	}
}

func TestBuildCarriesStructuredAnalyzerEvidence(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "packages/auth/go.mod", "module example/auth\n")
	writeFile(t, root, "packages/auth/main.go", "package auth\n")
	plan, err := Build(context.Background(), root, "where is the Go module configured?", "", Options{BudgetTokens: 900})
	if err != nil {
		t.Fatal(err)
	}
	for _, excerpt := range plan.Included {
		if strings.Contains(excerpt.Text, "go module: example/auth") && excerpt.Workspace != "packages/auth" {
			t.Fatalf("go module workspace = %q", excerpt.Workspace)
		}
		if strings.Contains(excerpt.Text, "structured evidence: packages/auth/go.mod") && strings.Contains(excerpt.Text, "workspace=packages/auth") && strings.Contains(excerpt.Text, "sha256=") {
			if excerpt.Workspace != "packages/auth" {
				t.Fatalf("excerpt workspace = %q", excerpt.Workspace)
			}
			return
		}
	}
	t.Fatalf("structured analyzer evidence was not carried into context: %+v", plan)
}

func TestBuildCarriesAllAnalyzerWorkspaces(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "packages/auth/main.go", "package auth\n")
	writeFile(t, root, "apps/web/main.go", "package web\n")
	plan, err := Build(context.Background(), root, "where are the Go sources?", "", Options{BudgetTokens: 900})
	if err != nil {
		t.Fatal(err)
	}
	for _, excerpt := range plan.Included {
		if strings.Contains(excerpt.Text, "workspaces: apps/web, packages/auth") {
			return
		}
	}
	t.Fatalf("all workspaces were not carried into context: %+v", plan)
}

func TestBuildCarriesQualityGateWorkspaceStructurally(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "services/auth/package.json", "{\"scripts\":{\"test\":\"go test\"}}\n")
	plan, err := Build(context.Background(), root, "which test command validates auth?", "", Options{BudgetTokens: 900})
	if err != nil {
		t.Fatal(err)
	}
	for _, excerpt := range plan.Included {
		if strings.Contains(excerpt.Text, "quality gate: npm test") {
			if excerpt.Workspace != "services/auth" {
				t.Fatalf("quality gate workspace = %q", excerpt.Workspace)
			}
			return
		}
	}
	t.Fatalf("quality gate excerpt not selected: %+v", plan)
}

func TestBuildFindsRelevantFileBeyondDeepReadLimit(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 250; i++ {
		writeFile(t, root, filepath.Join("aaa", fmt.Sprintf("file-%03d.txt", i)), "unrelated content")
	}
	writeFile(t, root, "zzz/security/authentication.go", "func ValidateJWT(token string) bool { return token != \"\" }")

	plan, err := Build(context.Background(), root, "where is JWT authentication validated?", "", Options{BudgetTokens: 900})
	if err != nil {
		t.Fatal(err)
	}
	for _, excerpt := range plan.Included {
		if excerpt.Path == "zzz/security/authentication.go" {
			return
		}
	}
	t.Fatalf("relevant file after the first 200 paths was not selected: %+v", plan)
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

func TestBuildSkipsIgnoredAndSensitiveContent(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".gitignore", "private/\n")
	writeFile(t, root, "private/notes.txt", "webhook authentication")
	writeFile(t, root, "README.md", "api_key=fictional-credential-value webhook authentication")
	writeFile(t, root, "safe.txt", "webhook authentication")
	plan, err := Build(context.Background(), root, "webhook authentication", "", Options{BudgetTokens: 300})
	if err != nil {
		t.Fatal(err)
	}
	for _, excerpt := range append(plan.Included, plan.Excluded...) {
		if excerpt.Path == "private/notes.txt" || (excerpt.Path == "README.md" && excerpt.Text != "") {
			t.Fatalf("unsafe content considered: %+v", excerpt)
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

func TestBuildReportsBudgetOverflowBelowPromptEnvelope(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "safe.txt", "webhook authentication")
	plan, err := Build(context.Background(), root, "webhook authentication", "", Options{BudgetTokens: 10})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.BudgetOverflow || plan.OverflowTokens <= 0 {
		t.Fatalf("tiny context budget did not report explicit overflow: %+v", plan)
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
