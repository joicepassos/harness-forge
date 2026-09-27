package infrastructure

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
	"harnessforge/internal/harness/domain"
	harnessinfra "harnessforge/internal/harness/infrastructure"
)

type cancelAfterErrContext struct {
	context.Context
	remaining int
}

func (c *cancelAfterErrContext) Err() error {
	if c.remaining > 0 {
		c.remaining--
		return nil
	}
	return context.Canceled
}

func (c *cancelAfterErrContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *cancelAfterErrContext) Done() <-chan struct{}       { return nil }
func (c *cancelAfterErrContext) Value(key any) any           { return c.Context.Value(key) }

func forgeSyncFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	itemDir := filepath.Join(root, ".forge", "knowledge", "items")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}
	item := domain.KnowledgeItem{ID: "rule-a", Kind: domain.KnowledgeConvention, Content: "Use explicit errors.", Origin: "human", Review: domain.KnowledgeApproved, Health: domain.KnowledgeUnknown, Reviewer: "alice", ContentSHA256: domain.HashKnowledgeContent("Use explicit errors."), ReviewDiff: "--- candidate\n+++ reviewed\n+Use explicit errors.\n", EvidenceSHA256: domain.HashKnowledgeEvidence(nil)}
	data, err := yaml.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	itemPath := filepath.Join(itemDir, "rule.md")
	if err := os.WriteFile(itemPath, []byte("---\n"+string(data)+"---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := `layout_version: 1
ir_version: 2
project:
  name: demo
  languages: [Go]
targets: [codex]
references:
  knowledge:
    - id: rule-a
      path: .forge/knowledge/items/rule.md
`
	if err := os.WriteFile(filepath.Join(root, ".forge", "forge.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestSyncForgePreviewApplyCheckAndCloneOwnership(t *testing.T) {
	root := forgeSyncFixture(t)
	preview, err := SyncForge(context.Background(), root, "dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Changed || len(preview.Files) != 1 {
		t.Fatalf("bad preview: %#v", preview)
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatal("dry-run wrote output")
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "check"); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, generatedManifest)
	if _, err := os.Stat(manifest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	firstManifest, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	firstOutput, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	secondManifest, _ := os.ReadFile(manifest)
	secondOutput, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if string(firstManifest) != string(secondManifest) || string(firstOutput) != string(secondOutput) {
		t.Fatal("second apply changed deterministic bytes")
	}
	if _, err := SyncForge(context.Background(), root, "check"); err != nil {
		t.Fatalf("check should be clean immediately after apply: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("edited generated file"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "check"); err == nil {
		t.Fatal("check ignored output hash drift")
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err == nil {
		t.Fatal("apply overwrote an edited generated output")
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), firstOutput, 0644); err != nil {
		t.Fatal(err)
	}
	// The shared manifest makes ownership verifiable in a clone without local state.
	clone := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(clone, ".forge", "knowledge", "items"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".forge/forge.yaml", ".forge/knowledge/items/rule.md", "AGENTS.md", generatedManifest} {
		raw, e := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if e != nil {
			t.Fatal(e)
		}
		if err := os.WriteFile(filepath.Join(clone, filepath.FromSlash(rel)), raw, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := SyncForge(context.Background(), clone, "apply"); err != nil {
		t.Fatalf("clone ownership was not recognized: %v", err)
	}
}

func TestSyncForgePublishesNativeSkillsAndManagedResources(t *testing.T) {
	root := forgeSyncFixture(t)
	skillDir := filepath.Join(root, ".forge", "skills", "review")
	if err := os.MkdirAll(filepath.Join(skillDir, "references"), 0755); err != nil {
		t.Fatal(err)
	}
	skill := "---\nname: review\ndescription: Review code changes and use during code review.\n---\n\nReview each change carefully. See [guide](references/guide.md).\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skill), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "references", "guide.md"), []byte("Inspect tests and error paths.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, ".forge", "forge.yaml")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest = []byte(strings.Replace(string(manifest), "targets: [codex]", "targets: [codex, claude]", 1))
	manifest = append(manifest, []byte("  skills:\n    - id: code-review\n      description: Review code changes and use during code review.\n      path: .forge/skills/review/SKILL.md\n")...)
	if err := os.WriteFile(manifestPath, manifest, 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := CompileForge(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{".agents/skills/review/SKILL.md", ".agents/skills/review/references/guide.md", ".claude/skills/review/SKILL.md", ".claude/skills/review/references/guide.md"} {
		if _, ok := plan.Diff[expected]; !ok {
			t.Fatalf("compiled output omitted %s", expected)
		}
	}
	for _, instructions := range []string{plan.Diff["AGENTS.md"], plan.Diff["CLAUDE.md"]} {
		if strings.Contains(instructions, "Review each change carefully") {
			t.Fatal("skill body was duplicated in agent instructions")
		}
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".agents/skills/review/references/guide.md", ".claude/skills/review/references/guide.md"} {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil || string(content) != "Inspect tests and error paths.\n" {
			t.Fatalf("published resource %s=%q err=%v", rel, content, err)
		}
	}
	if _, err := SyncForge(context.Background(), root, "check"); err != nil {
		t.Fatalf("skill outputs not owned or in sync: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "references", "guide.md"), []byte("Updated reference.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatalf("intact owned skill output did not update: %v", err)
	}
	for _, rel := range []string{".agents/skills/review/references/guide.md", ".claude/skills/review/references/guide.md"} {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil || string(content) != "Updated reference.\n" {
			t.Fatalf("updated resource %s=%q err=%v", rel, content, err)
		}
	}
}

func TestSyncForgeRemovesOnlyIntactOwnedSkillFiles(t *testing.T) {
	root := forgeSyncFixture(t)
	skill := filepath.Join(root, ".forge", "skills", "review", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skill), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, []byte("---\nname: review\ndescription: Review source code.\n---\n\nReview it.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, ".forge", "forge.yaml")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest = append(manifest, []byte("  skills:\n    - id: review\n      description: Review source code.\n      path: .forge/skills/review/SKILL.md\n")...)
	if err := os.WriteFile(manifestPath, manifest, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	ownedSkill := filepath.Join(root, ".agents", "skills", "review", "SKILL.md")
	ownedBefore, err := os.ReadFile(ownedSkill)
	if err != nil {
		t.Fatal(err)
	}
	manifest = []byte(strings.Replace(string(manifest), "  skills:\n    - id: review\n      description: Review source code.\n      path: .forge/skills/review/SKILL.md\n", "", 1))
	if err := os.WriteFile(manifestPath, manifest, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ownedSkill, []byte("manual edit"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err == nil || !strings.Contains(err.Error(), "manually edited") {
		t.Fatalf("edited stale skill was removed: %v", err)
	}
	if err := os.WriteFile(ownedSkill, ownedBefore, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatalf("intact stale skill wasn't removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "review", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("stale Codex skill remained")
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "review", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("stale Claude skill remained")
	}
}

func TestSyncForgeRejectsEvidenceChangedAfterApproval(t *testing.T) {
	root := forgeSyncFixture(t)
	document := filepath.Join(root, ".forge", "knowledge", "items", "rule.md")
	item := domain.KnowledgeItem{ID: "rule-a", Kind: domain.KnowledgeConvention, Content: "Use explicit errors.", Origin: "human", Review: domain.KnowledgeApproved, Health: domain.KnowledgeVerified, Reviewer: "alice", ContentSHA256: domain.HashKnowledgeContent("Use explicit errors."), ReviewDiff: "--- candidate\n+++ reviewed\n+Use explicit errors.\n", Evidence: []domain.KnowledgeEvidence{{Path: "source.go", Quote: "original"}}, EvidenceSHA256: domain.HashKnowledgeEvidence([]string{"source.go:placeholder"})}
	if err := os.WriteFile(filepath.Join(root, "source.go"), []byte("original source"), 0600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := harnessinfra.KnowledgeFingerprint(root, ".forge/knowledge/items/rule.md", "rule-a", item.Evidence)
	if err != nil {
		t.Fatal(err)
	}
	item.EvidenceSHA256 = fingerprint
	encoded, err := yaml.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(document, append(append([]byte("---\n"), encoded...), []byte("---\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileForge(root); err != nil {
		t.Fatalf("valid approved evidence rejected: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "source.go"), []byte("edited source"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileForge(root); err == nil || !strings.Contains(err.Error(), "evidence") {
		t.Fatalf("stale evidence published: %v", err)
	}
}

func TestSyncForgeProtectsUnownedAndEditedFiles(t *testing.T) {
	root := forgeSyncFixture(t)
	target := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(target, []byte("human instructions\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err == nil || !strings.Contains(err.Error(), "unowned") {
		t.Fatalf("expected unowned collision, got %v", err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("manual edit\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err == nil || !strings.Contains(err.Error(), "edited") {
		t.Fatalf("expected edited-file conflict, got %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "manual edit\n" {
		t.Fatal("manual content was overwritten")
	}
}

func TestCompileForgeRejectsCollidingSkillTargets(t *testing.T) {
	root := forgeSyncFixture(t)
	skillDir := filepath.Join(root, ".forge", "skills", "review")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: review\ndescription: Review code changes.\n---\n\nReview changes.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, ".forge", "forge.yaml")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest = append(manifest, []byte("  skills:\n    - id: review-a\n      description: Review code changes.\n      path: .forge/skills/review/SKILL.md\n    - id: review-b\n      description: Review code changes.\n      path: .forge/skills/review/SKILL.md\n")...)
	if err := os.WriteFile(manifestPath, manifest, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileForge(root); err == nil || !strings.Contains(err.Error(), "target collision at .agents/skills/review/SKILL.md") {
		t.Fatalf("expected deterministic skill output collision, got %v", err)
	}
}

func TestCompileForgePreservesGlobScopeTextAndWindowsPathsAreRejected(t *testing.T) {
	root := forgeSyncFixture(t)
	itemPath := filepath.Join(root, ".forge", "knowledge", "items", "rule.md")
	item := domain.KnowledgeItem{ID: "rule-a", Kind: domain.KnowledgeConvention, Scope: domain.Scope{Paths: []string{"services/api/**/*.go", "libs/shared/*_test.go"}}, Content: "Use explicit errors.", Origin: "human", Review: domain.KnowledgeApproved, Health: domain.KnowledgeUnknown, Reviewer: "alice", ContentSHA256: domain.HashKnowledgeContent("Use explicit errors."), ReviewDiff: "--- candidate\n+++ reviewed\n+Use explicit errors.\n", EvidenceSHA256: domain.HashKnowledgeEvidence(nil)}
	encoded, err := yaml.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(itemPath, append(append([]byte("---\n"), encoded...), []byte("---\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := CompileForge(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, glob := range item.Scope.Paths {
		if !strings.Contains(plan.Diff["AGENTS.md"], glob) {
			t.Errorf("rendered output lost glob scope %q: %s", glob, plan.Diff["AGENTS.md"])
		}
	}
	// Manifest/reference paths use POSIX separators on every host. A Windows
	// separator is rejected instead of being interpreted differently by OS.
	manifestPath := filepath.Join(root, ".forge", "forge.yaml")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest = []byte(strings.Replace(string(manifest), ".forge/knowledge/items/rule.md", `.forge\\knowledge\\items\\rule.md`, 1))
	if err := os.WriteFile(manifestPath, manifest, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileForge(root); err == nil || !strings.Contains(err.Error(), "references.knowledge[0].path") {
		t.Fatalf("Windows-style manifest path was not rejected: %v", err)
	}
}

func TestCompileForgeCarriesPolicyCapabilityNotes(t *testing.T) {
	root := forgeSyncFixture(t)
	manifestPath := filepath.Join(root, ".forge", "forge.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := string(data) + "policies:\n  - id: network\n    description: Do not use network\n    capability: advisory\n    executor: text\n"
	if err := os.WriteFile(manifestPath, []byte(updated), 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := CompileForge(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan.Diff["AGENTS.md"], "capability: **advisory**") || !strings.Contains(plan.Diff["AGENTS.md"], "does not enforce system permissions") {
		t.Fatalf("policy limit missing from output: %s", plan.Diff["AGENTS.md"])
	}
}

func TestSyncRemovesOnlyIntactPreviouslyOwnedTarget(t *testing.T) {
	root := forgeSyncFixture(t)
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".forge", "forge.yaml"), []byte(`layout_version: 1
ir_version: 2
project: {name: demo, languages: [Go]}
targets: [claude]
references:
  knowledge:
    - id: rule-a
      path: .forge/knowledge/items/rule.md
`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatal("intact stale generated output wasn't removed")
	}
	if _, err := os.Stat(filepath.Join(root, "CLAUDE.md")); err != nil {
		t.Fatal("current target missing")
	}
	if _, err := SyncForge(context.Background(), root, "check"); err != nil {
		t.Fatalf("updated target ownership not current: %v", err)
	}
}

func TestSyncRefusesToRemoveEditedStaleTarget(t *testing.T) {
	root := forgeSyncFixture(t)
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".forge", "forge.yaml"), []byte(`layout_version: 1
ir_version: 2
project: {name: demo, languages: [Go]}
targets: [claude]
references:
  knowledge:
    - id: rule-a
      path: .forge/knowledge/items/rule.md
`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("human edit"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err == nil || !strings.Contains(err.Error(), "manually edited") {
		t.Fatalf("edited stale target removed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil || string(data) != "human edit" {
		t.Fatalf("manual bytes lost: %q %v", data, err)
	}
}

func TestSyncRestoresStaleTargetWhenApplyIsCanceledMidCommit(t *testing.T) {
	root := forgeSyncFixture(t)
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	stalePath := filepath.Join(root, "AGENTS.md")
	staleBefore, err := os.ReadFile(stalePath)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, generatedManifest)
	manifestBefore, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".forge", "forge.yaml"), []byte(`layout_version: 1
ir_version: 2
project: {name: demo, languages: [Go]}
targets: [claude]
references:
  knowledge:
    - id: rule-a
      path: .forge/knowledge/items/rule.md
`), 0600); err != nil {
		t.Fatal(err)
	}

	// Let the pre-commit and stale-removal checks pass, then cancel before the
	// replacement target is written. Rollback must restore both prior files.
	ctx := &cancelAfterErrContext{Context: context.Background(), remaining: 2}
	if _, err := SyncForge(ctx, root, "apply"); err != context.Canceled {
		t.Fatalf("expected mid-commit cancellation, got %v", err)
	}
	staleAfter, err := os.ReadFile(stalePath)
	if err != nil || string(staleAfter) != string(staleBefore) {
		t.Fatalf("stale generated target was not restored: %q, %v", staleAfter, err)
	}
	manifestAfter, err := os.ReadFile(manifestPath)
	if err != nil || string(manifestAfter) != string(manifestBefore) {
		t.Fatalf("prior generated manifest was not preserved: %q, %v", manifestAfter, err)
	}
	if _, err := os.Stat(filepath.Join(root, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatalf("partial apply left replacement output behind: %v", err)
	}
}

func TestSyncRecoversDurablyAfterInterruptionAtEveryCommitPhase(t *testing.T) {
	for interruptAt := 1; interruptAt <= 3; interruptAt++ {
		t.Run(strconv.Itoa(interruptAt), func(t *testing.T) {
			root := forgeSyncFixture(t)
			if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
				t.Fatal(err)
			}
			human := filepath.Join(root, "notes.txt")
			if err := os.WriteFile(human, []byte("keep this human file\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".forge", "forge.yaml"), []byte(`layout_version: 1
ir_version: 2
project: {name: demo, languages: [Go]}
targets: [claude]
references:
  knowledge:
    - id: rule-a
      path: .forge/knowledge/items/rule.md
`), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := syncForge(context.Background(), root, "apply", func(phase int) error {
				if phase == interruptAt {
					return errSyncInterrupted
				}
				return nil
			})
			if !errors.Is(err, errSyncInterrupted) {
				t.Fatalf("expected simulated process interruption at phase %d, got %v", interruptAt, err)
			}
			if _, err := os.Stat(filepath.Join(root, syncJournal)); err != nil {
				t.Fatalf("interrupted apply did not leave a durable recovery journal: %v", err)
			}
			if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
				t.Fatalf("next apply did not recover and finish: %v", err)
			}
			if _, err := SyncForge(context.Background(), root, "check"); err != nil {
				t.Fatalf("recovered outputs are inconsistent: %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
				t.Fatalf("stale target remains after recovery: %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, "CLAUDE.md")); err != nil {
				t.Fatalf("new target missing after recovery: %v", err)
			}
			if data, err := os.ReadFile(human); err != nil || string(data) != "keep this human file\n" {
				t.Fatalf("human file changed during recovery: %q, %v", data, err)
			}
			if _, err := os.Stat(filepath.Join(root, syncJournal)); !os.IsNotExist(err) {
				t.Fatalf("completed apply retained journal: %v", err)
			}
		})
	}
}

func TestSyncRecoveryPreservesHumanEditMadeAfterInterruption(t *testing.T) {
	root := forgeSyncFixture(t)
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".forge", "forge.yaml"), []byte(`layout_version: 1
ir_version: 2
project: {name: demo, languages: [Go]}
targets: [claude]
references:
  knowledge:
    - id: rule-a
      path: .forge/knowledge/items/rule.md
`), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := syncForge(context.Background(), root, "apply", func(phase int) error {
		if phase == 1 {
			return errSyncInterrupted
		}
		return nil
	})
	if !errors.Is(err, errSyncInterrupted) {
		t.Fatalf("expected simulated interruption, got %v", err)
	}
	human := filepath.Join(root, "CLAUDE.md")
	if err := os.WriteFile(human, []byte("human instructions\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err == nil || !strings.Contains(err.Error(), "post-interruption edit") {
		t.Fatalf("recovery did not report post-interruption edit: %v", err)
	}
	if data, err := os.ReadFile(human); err != nil || string(data) != "human instructions\n" {
		t.Fatalf("recovery overwrote human edit: %q, %v", data, err)
	}
}

func TestSyncForgeRejectsSymlinkTarget(t *testing.T) {
	root := forgeSyncFixture(t)
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "AGENTS.md")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := SyncForge(context.Background(), root, "dry-run"); err == nil {
		t.Fatal("symlink target was accepted")
	}
}

func TestSyncForgeRefusesOwnedSymlinkWithoutChangingItsTarget(t *testing.T) {
	root := forgeSyncFixture(t)
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(target, []byte("external content"), 0600); err != nil {
		t.Fatal(err)
	}
	generated := filepath.Join(root, "AGENTS.md")
	if err := os.Remove(generated); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, generated); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err == nil || !strings.Contains(err.Error(), "unsafe generated file") {
		t.Fatalf("owned symlink was accepted: %v", err)
	}
	content, err := os.ReadFile(target)
	if err != nil || string(content) != "external content" {
		t.Fatalf("symlink target was changed: %q, %v", content, err)
	}
}
