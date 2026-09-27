package infrastructure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
	"harnessforge/internal/contextpack/application"
	contextdomain "harnessforge/internal/contextpack/domain"
	harnessdomain "harnessforge/internal/harness/domain"
)

func TestForgeKnowledgeCandidatesIncludeOnlyApprovedHashBoundItems(t *testing.T) {
	root := t.TempDir()
	writeKnowledgeFixture(t, root, "approved", harnessdomain.KnowledgeApproved, "alice", "Auth tokens must expire.", nil)
	writeKnowledgeFixture(t, root, "candidate", harnessdomain.KnowledgeCandidate, "", "Candidate note about auth tokens.", nil)
	writeKnowledgeFixture(t, root, "rejected", harnessdomain.KnowledgeRejected, "alice", "Rejected auth note.", nil)
	writeForgeManifest(t, root, `
  - id: approved
    path: .forge/knowledge/approved.md
  - id: candidate
    path: .forge/knowledge/candidate.md
  - id: rejected
    path: .forge/knowledge/rejected.md`)

	items, err := forgeKnowledgeCandidates(root, "", "auth token expiry", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected only approved item, got %#v", items)
	}
	item := items[0]
	if item.KnowledgeID != "approved" || item.Source != "forge-knowledge:approved" || item.Path != ".forge/knowledge/approved.md" {
		t.Fatalf("unexpected knowledge source identity/provenance: %#v", item)
	}
	if item.Text != "Auth tokens must expire." || item.Status != "" {
		t.Fatalf("unexpected knowledge payload: %#v", item)
	}
	if len(item.Origins) != 2 || item.Origins[0] != "forge-knowledge:approved" || item.Origins[1] != "forge-document:.forge/knowledge/approved.md" {
		t.Fatalf("missing explicit provenance: %#v", item.Origins)
	}
}

func TestBuildAddsApprovedForgeKnowledgeToSelectedSources(t *testing.T) {
	root := t.TempDir()
	writeKnowledgeFixture(t, root, "auth-expiration", harnessdomain.KnowledgeApproved, "alice", "Authentication tokens expire after fifteen minutes.", nil)
	writeForgeManifest(t, root, `
  - id: auth-expiration
    path: .forge/knowledge/auth-expiration.md`)
	plan, err := Build(context.Background(), root, "How long until authentication tokens expire?", "", contextdomain.Options{BudgetTokens: 12000})
	if err != nil {
		t.Fatal(err)
	}
	for _, excerpt := range plan.Included {
		if excerpt.KnowledgeID == "auth-expiration" && strings.Contains(excerpt.Text, "fifteen minutes") {
			sources := application.SourceList("How long until authentication tokens expire?", plan)
			for _, source := range sources {
				if source.KnowledgeID == "auth-expiration" {
					if len(source.Origins) < 2 || source.Origins[0] != "forge-knowledge:auth-expiration" {
						t.Fatalf("downstream source lost knowledge provenance: %#v", source)
					}
					return
				}
			}
			t.Fatalf("lossless source envelope omitted approved knowledge provenance: %#v", sources)
		}
	}
	t.Fatalf("approved Forge knowledge was not selected: included=%#v excluded=%#v", plan.Included, plan.Excluded)
}

func TestForgeKnowledgeCandidatesSelectByTaskPathGlobAndKeyword(t *testing.T) {
	root := t.TempDir()
	writeScopedKnowledgeFixture(t, root, "backend-webhooks", "services/backend/**", "webhook", "Backend webhook delivery uses the queue.")
	writeScopedKnowledgeFixture(t, root, "frontend-webhooks", "apps/web/**", "webhook", "Frontend webhook status is shown in the dashboard.")
	writeForgeManifest(t, root, `
  - id: backend-webhooks
    path: .forge/knowledge/backend-webhooks.md
  - id: frontend-webhooks
    path: .forge/knowledge/frontend-webhooks.md`)

	for _, test := range []struct {
		name     string
		taskPath string
		prompt   string
		want     string
	}{
		{name: "backend", taskPath: "services/backend/api/handler.go", prompt: "Update webhook delivery", want: "backend-webhooks"},
		{name: "frontend", taskPath: "apps/web/src/WebhookPanel.tsx", prompt: "Update webhook status", want: "frontend-webhooks"},
		{name: "keyword required", taskPath: "services/backend/api/handler.go", prompt: "Update audit logging", want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			items, err := forgeKnowledgeCandidates(root, "forge", test.prompt, []string{test.taskPath})
			if err != nil {
				t.Fatal(err)
			}
			if test.want == "" {
				for _, item := range items {
					if item.Status != "excluded" {
						t.Fatalf("expected no eligible scoped knowledge without keyword match, got %#v", items)
					}
				}
				return
			}
			var selected []string
			for _, item := range items {
				if item.Status != "excluded" {
					selected = append(selected, item.KnowledgeID)
				}
			}
			if len(selected) != 1 || selected[0] != test.want {
				t.Fatalf("task path %q selected %#v; want only %q", test.taskPath, items, test.want)
			}
		})
	}
}

func TestBuildSelectsKnowledgeByTaskPathAndReportsOutOfScopeExclusion(t *testing.T) {
	root := t.TempDir()
	writeScopedKnowledgeFixture(t, root, "backend-webhooks", "services/backend/**", "webhook", "Backend webhook delivery retries through the durable queue.")
	writeScopedKnowledgeFixture(t, root, "frontend-webhooks", "apps/web/**", "webhook", "Frontend webhook status appears in the dashboard.")
	writeForgeManifest(t, root, `
  - id: backend-webhooks
    path: .forge/knowledge/backend-webhooks.md
  - id: frontend-webhooks
    path: .forge/knowledge/frontend-webhooks.md`)

	for _, test := range []struct {
		name, taskPath, wantID, excludedID string
	}{
		{name: "backend", taskPath: "services/backend/api/handler.go", wantID: "backend-webhooks", excludedID: "frontend-webhooks"},
		{name: "frontend", taskPath: "apps/web/src/WebhookPanel.tsx", wantID: "frontend-webhooks", excludedID: "backend-webhooks"},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan, err := Build(context.Background(), root, "Update webhook delivery status", "", contextdomain.Options{
				Layout: "forge", TaskPaths: []string{test.taskPath}, BudgetTokens: 12000,
			})
			if err != nil {
				t.Fatal(err)
			}
			if !hasKnowledgeExcerpt(plan.Included, test.wantID) {
				t.Fatalf("Build did not include task-scoped knowledge %q: included=%#v excluded=%#v", test.wantID, plan.Included, plan.Excluded)
			}
			if !hasKnowledgeExcerpt(plan.Excluded, test.excludedID) {
				t.Fatalf("Build did not report out-of-scope knowledge %q as excluded: included=%#v excluded=%#v", test.excludedID, plan.Included, plan.Excluded)
			}
			for _, excerpt := range plan.Excluded {
				if excerpt.KnowledgeID == test.excludedID && excerpt.Reason != "knowledge scope does not match the task paths" {
					t.Fatalf("out-of-scope knowledge has unexpected exclusion reason: %#v", excerpt)
				}
			}
		})
	}
}

func hasKnowledgeExcerpt(excerpts []contextdomain.Excerpt, id string) bool {
	for _, excerpt := range excerpts {
		if excerpt.KnowledgeID == id {
			return true
		}
	}
	return false
}

func TestKnowledgePathMatchSupportsRecursiveAndSegmentGlobs(t *testing.T) {
	for _, test := range []struct {
		pattern, candidate string
		want               bool
	}{
		{"services/backend/**", "services/backend/api/handler.go", true},
		{"services/backend/**", "services/frontend/api/handler.go", false},
		{"apps/*/src/**", "apps/web/src/App.tsx", true},
		{"apps/*/src/**", "apps/web/nested/src/App.tsx", false},
	} {
		if got := knowledgePathMatch(test.pattern, test.candidate); got != test.want {
			t.Errorf("knowledgePathMatch(%q, %q) = %t, want %t", test.pattern, test.candidate, got, test.want)
		}
	}
}

func TestForgeKnowledgeCandidatesRejectChangedContentOrEvidenceHashes(t *testing.T) {
	for _, test := range []struct {
		name      string
		before    string
		after     string
		evidence  string
		wantError string
	}{
		{name: "content edited after approval", before: "Original auth policy.", after: "Edited auth policy.", wantError: "content hash mismatch"},
		{name: "scope edited after approval", before: "Original auth policy.", after: "Original auth policy.", wantError: "review metadata hash mismatch"},
		{name: "keywords edited after approval", before: "Original auth policy.", after: "Original auth policy.", wantError: "review metadata hash mismatch"},
		{name: "evidence changed", before: "Approved auth policy.", evidence: "evidence hash mismatch", wantError: "evidence"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			evidence := []harnessdomain.KnowledgeEvidence(nil)
			if test.evidence != "" {
				_ = os.MkdirAll(filepath.Join(root, "docs"), 0o755)
				if err := os.WriteFile(filepath.Join(root, "docs", "policy.md"), []byte("current source"), 0o600); err != nil {
					t.Fatal(err)
				}
				evidence = []harnessdomain.KnowledgeEvidence{{Path: "docs/policy.md", SHA256: strings.Repeat("0", 64)}}
			}
			writeKnowledgeFixture(t, root, "rule", harnessdomain.KnowledgeApproved, "reviewer", test.before, evidence)
			if strings.Contains(test.name, "scope edited") || strings.Contains(test.name, "keywords edited") {
				path := filepath.Join(root, ".forge", "knowledge", "rule.md")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				item, err := parseKnowledgeDocument(data)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(test.name, "scope edited") {
					item.Scope.Paths = []string{"services/private/**"}
				} else {
					item.Keywords = []string{"secret"}
				}
				encoded, err := yaml.Marshal(item)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, append(append([]byte("---\n"), encoded...), []byte("---\n")...), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if test.after != "" {
				path := filepath.Join(root, ".forge", "knowledge", "rule.md")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(strings.Replace(string(data), test.before, test.after, 1)), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			writeForgeManifest(t, root, `
  - id: rule
    path: .forge/knowledge/rule.md`)
			_, err := forgeKnowledgeCandidates(root, "", "auth policy", nil)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("expected %q error, got %v", test.wantError, err)
			}
		})
	}
}

func TestParseKnowledgeDocumentPreservesReviewDiffDelimitersAndHashes(t *testing.T) {
	root := t.TempDir()
	writeKnowledgeFixture(t, root, "reviewed", harnessdomain.KnowledgeApproved, "reviewer", "Approved knowledge.", nil)
	data, err := os.ReadFile(filepath.Join(root, ".forge", "knowledge", "reviewed.md"))
	if err != nil {
		t.Fatal(err)
	}
	item, err := parseKnowledgeDocument(data)
	if err != nil {
		t.Fatal(err)
	}
	if item.ContentSHA256 != harnessdomain.HashKnowledgeContent(item.Content) || item.EvidenceSHA256 != harnessdomain.HashKnowledgeEvidence(nil) {
		t.Fatalf("review hashes were lost while parsing: %+v", item)
	}
	if !strings.Contains(item.ReviewDiff, "--- candidate") {
		t.Fatalf("review diff was truncated: %q", item.ReviewDiff)
	}
}

func TestParseKnowledgeDocumentRequiresReviewHashes(t *testing.T) {
	for _, test := range []struct {
		name      string
		content   string
		evidence  string
		wantError string
	}{
		{name: "content hash", evidence: harnessdomain.HashKnowledgeEvidence(nil), wantError: "content_sha256: required"},
		{name: "evidence hash", content: harnessdomain.HashKnowledgeContent("Reviewed content."), wantError: "evidence_sha256: required"},
	} {
		t.Run(test.name, func(t *testing.T) {
			item := harnessdomain.KnowledgeItem{
				ID: "reviewed", Kind: harnessdomain.KnowledgeConvention, Content: "Reviewed content.",
				Origin: "human", Review: harnessdomain.KnowledgeApproved, Health: harnessdomain.KnowledgeVerified,
				Reviewer: "reviewer", ContentSHA256: test.content, EvidenceSHA256: test.evidence,
			}
			data, err := yaml.Marshal(item)
			if err != nil {
				t.Fatal(err)
			}
			document := append(append([]byte("---\n"), data...), []byte("---\n")...)
			if _, err := parseKnowledgeDocument(document); err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("expected %q error, got %v", test.wantError, err)
			}
		})
	}
}

func TestForgeKnowledgeCandidatesRejectAmbiguousLayout(t *testing.T) {
	root := t.TempDir()
	writeKnowledgeFixture(t, root, "rule", harnessdomain.KnowledgeApproved, "reviewer", "Use auth middleware.", nil)
	writeForgeManifest(t, root, `
  - id: rule
    path: .forge/knowledge/rule.md`)
	if err := os.MkdirAll(filepath.Join(root, ".harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".harness", "harness.yaml"), []byte("version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := forgeKnowledgeCandidates(root, "", "auth middleware", nil); err == nil || !strings.Contains(err.Error(), "both .harness") {
		t.Fatalf("expected explicit layout selection error, got %v", err)
	}
	items, err := forgeKnowledgeCandidates(root, "forge", "auth middleware", nil)
	if err != nil || len(items) != 1 {
		t.Fatalf("explicit Forge layout selection failed: items=%#v err=%v", items, err)
	}
}

func TestForgeKnowledgeCandidatesRejectSymlinkedKnowledgePath(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "item.md"), []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".forge")); err != nil {
		t.Skipf("symlink creation is unavailable: %v", err)
	}
	if _, err := forgeKnowledgeCandidates(root, "forge", "auth", nil); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlinked layout to be rejected, got %v", err)
	}
}

func writeForgeManifest(t *testing.T, root, knowledge string) {
	t.Helper()
	manifest := "layout_version: 1\nir_version: 1\nproject:\n  name: fixture\n  languages: [go]\ntargets: [codex]\nreferences:\n  knowledge:" + knowledge + "\n"
	if err := os.MkdirAll(filepath.Join(root, ".forge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".forge", "forge.yaml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeKnowledgeFixture(t *testing.T, root, id string, review harnessdomain.KnowledgeReviewState, reviewer, content string, evidence []harnessdomain.KnowledgeEvidence) {
	t.Helper()
	writeKnowledgeFixtureWithMetadata(t, root, id, review, reviewer, content, evidence, nil, nil)
}

func writeScopedKnowledgeFixture(t *testing.T, root, id, scope, keyword, content string) {
	t.Helper()
	writeKnowledgeFixtureWithMetadata(t, root, id, harnessdomain.KnowledgeApproved, "alice", content, nil, []string{scope}, []string{keyword})
}

func writeKnowledgeFixtureWithMetadata(t *testing.T, root, id string, review harnessdomain.KnowledgeReviewState, reviewer, content string, evidence []harnessdomain.KnowledgeEvidence, scopes, keywords []string) {
	t.Helper()
	item := harnessdomain.KnowledgeItem{ID: id, Kind: harnessdomain.KnowledgeConvention, Content: content, Origin: "human", Review: review, Health: harnessdomain.KnowledgeVerified, Reviewer: reviewer, Evidence: evidence, Scope: harnessdomain.Scope{Paths: scopes}, Keywords: keywords}
	if review != harnessdomain.KnowledgeCandidate {
		item.ContentSHA256 = harnessdomain.HashKnowledgeContent(content)
		values := make([]string, 0, len(evidence))
		for _, e := range evidence {
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(e.Path)))
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(data)
			values = append(values, e.Path+":"+hex.EncodeToString(sum[:]))
		}
		item.EvidenceSHA256 = harnessdomain.HashKnowledgeEvidence(values)
		item.ReviewMetadataSHA256 = harnessdomain.HashKnowledgeReviewMetadata(item)
		item.ReviewDiff = "--- candidate\n+++ reviewed\n+" + content + "\n"
	}
	data, err := yamlMarshalFixture(item)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".forge", "knowledge", id+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	document := append(append([]byte("---\n"), data...), []byte("---\n\n# Knowledge\n")...)
	if err := os.WriteFile(path, document, 0o600); err != nil {
		t.Fatal(err)
	}
}

func yamlMarshalFixture(item harnessdomain.KnowledgeItem) ([]byte, error) {
	return yaml.Marshal(item)
}
