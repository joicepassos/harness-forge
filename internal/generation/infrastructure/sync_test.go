package infrastructure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	item.ReviewMetadataSHA256 = domain.HashKnowledgeReviewMetadata(item)
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

func addForgeSyncKnowledge(t *testing.T, root, id, content string, paths []string) {
	t.Helper()
	item := domain.KnowledgeItem{
		ID: id, Kind: domain.KnowledgeConvention, Scope: domain.Scope{Paths: append([]string(nil), paths...)},
		Content: content, Origin: "human", Review: domain.KnowledgeApproved, Health: domain.KnowledgeUnknown,
		Reviewer: "alice", ContentSHA256: domain.HashKnowledgeContent(content),
		ReviewDiff: "--- candidate\n+++ reviewed\n+" + content + "\n", EvidenceSHA256: domain.HashKnowledgeEvidence(nil),
	}
	item.ReviewMetadataSHA256 = domain.HashKnowledgeReviewMetadata(item)
	encoded, err := yaml.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	itemPath := filepath.Join(root, ".forge", "knowledge", "items", id+".md")
	if err := os.WriteFile(itemPath, append(append([]byte("---\n"), encoded...), []byte("---\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, ".forge", "forge.yaml")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest = append(manifest, []byte("    - id: "+id+"\n      path: .forge/knowledge/items/"+id+".md\n")...)
	if err := os.WriteFile(manifestPath, manifest, 0600); err != nil {
		t.Fatal(err)
	}
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
	if _, err := os.Stat(filepath.Join(root, ".forge", ".sync.lock")); !os.IsNotExist(err) {
		t.Fatal("dry-run wrote lock state into the project")
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

func TestCompileAndSyncForgePublishNestedCodexScopesWithOwnership(t *testing.T) {
	root := forgeSyncFixture(t)
	addForgeSyncKnowledge(t, root, "service-rule", "Use service conventions.", []string{"services/**"})
	addForgeSyncKnowledge(t, root, "api-rule", "Use API conventions.", []string{"services/api/**"})
	addForgeSyncKnowledge(t, root, "go-rule", "Keep Go handlers explicit.", []string{"services/api/**/*.go"})

	plan, err := CompileForge(root)
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := []string{"AGENTS.md", "services/AGENTS.md", "services/api/AGENTS.md"}
	if len(plan.Files) != len(wantPaths) {
		t.Fatalf("compiled files = %#v, want %v", plan.Files, wantPaths)
	}
	for i, path := range wantPaths {
		if plan.Files[i].Path != path {
			t.Errorf("files[%d].Path = %q, want %q", i, plan.Files[i].Path, path)
		}
	}
	rootDoc := plan.Diff["AGENTS.md"]
	if !strings.Contains(rootDoc, "[rule-a] Use explicit errors. (global)") || !strings.Contains(rootDoc, "[go-rule] Keep Go handlers explicit. (advisory; applies only to paths matching: `services/api/**/*.go`)") {
		t.Fatalf("root output lost global or advisory file-glob rules:\n%s", rootDoc)
	}
	if strings.Contains(rootDoc, "[service-rule]") || strings.Contains(rootDoc, "[api-rule]") {
		t.Fatalf("native nested rules leaked into the root output:\n%s", rootDoc)
	}
	if !strings.Contains(plan.Diff["services/AGENTS.md"], "[service-rule] Use service conventions. (native Codex directory scope: `services/**`)") {
		t.Fatalf("service scope was not compiled natively:\n%s", plan.Diff["services/AGENTS.md"])
	}
	if !strings.Contains(plan.Diff["services/api/AGENTS.md"], "[api-rule] Use API conventions. (native Codex directory scope: `services/api/**`)") {
		t.Fatalf("API scope was not compiled natively:\n%s", plan.Diff["services/api/AGENTS.md"])
	}
	for _, file := range plan.Files {
		if file.Target != "codex" || file.AdapterVersion != "2" {
			t.Errorf("nested-capable Codex adapter metadata = %#v", file)
		}
		if strings.Contains(file.Path, "/") && file.Capabilities["scope"] != "native-directory-cwd" {
			t.Errorf("nested capability = %q, want native-directory-cwd", file.Capabilities["scope"])
		}
	}

	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "check"); err != nil {
		t.Fatalf("generated nested files are not owned/in sync: %v", err)
	}
	manifestData, err := os.ReadFile(filepath.Join(root, generatedManifest))
	if err != nil {
		t.Fatal(err)
	}
	var manifest GeneratedManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, path := range wantPaths {
		if !containsGeneratedPath(manifest.Files, path) {
			t.Errorf("manifest does not own %s: %#v", path, manifest.Files)
		}
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatalf("second nested apply failed: %v", err)
	}
}

func TestSyncForgeProtectsNestedCodexOwnershipAndOverrides(t *testing.T) {
	t.Run("unowned nested file", func(t *testing.T) {
		root := forgeSyncFixture(t)
		addForgeSyncKnowledge(t, root, "service-rule", "Use service conventions.", []string{"services/**"})
		nested := filepath.Join(root, "services", "AGENTS.md")
		if err := os.MkdirAll(filepath.Dir(nested), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(nested, []byte("Human instructions.\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := SyncForge(context.Background(), root, "apply"); err == nil || !strings.Contains(err.Error(), "unowned") {
			t.Fatalf("unowned nested AGENTS.md was overwritten: %v", err)
		}
		content, err := os.ReadFile(nested)
		if err != nil || string(content) != "Human instructions.\n" {
			t.Fatalf("human instructions changed: %q, err=%v", content, err)
		}
	})

	t.Run("override shadows generated file", func(t *testing.T) {
		root := forgeSyncFixture(t)
		addForgeSyncKnowledge(t, root, "service-rule", "Use service conventions.", []string{"services/**"})
		dir := filepath.Join(root, "services")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "AGENTS.override.md"), []byte("Local override.\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := SyncForge(context.Background(), root, "dry-run"); err == nil || !strings.Contains(err.Error(), "shadowed by services/AGENTS.override.md") {
			t.Fatalf("shadowed generated instructions were accepted: %v", err)
		}
	})

	t.Run("edited stale nested output is preserved", func(t *testing.T) {
		root := forgeSyncFixture(t)
		addForgeSyncKnowledge(t, root, "service-rule", "Use service conventions.", []string{"services/**"})
		if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
			t.Fatal(err)
		}
		nested := filepath.Join(root, "services", "AGENTS.md")
		if err := os.WriteFile(nested, []byte("human edit\n"), 0600); err != nil {
			t.Fatal(err)
		}
		manifestPath := filepath.Join(root, ".forge", "forge.yaml")
		manifest, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatal(err)
		}
		manifest = []byte(strings.Replace(string(manifest), "    - id: service-rule\n      path: .forge/knowledge/items/service-rule.md\n", "", 1))
		if err := os.WriteFile(manifestPath, manifest, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := SyncForge(context.Background(), root, "apply"); err == nil || !strings.Contains(err.Error(), "manually edited generated file services/AGENTS.md") {
			t.Fatalf("edited stale nested file was removed: %v", err)
		}
		content, err := os.ReadFile(nested)
		if err != nil || string(content) != "human edit\n" {
			t.Fatalf("edited stale output changed: %q, err=%v", content, err)
		}
	})
}

func TestNestedCodexInstructionPathsAreWhitelistedAndRejectSymlinkParents(t *testing.T) {
	for _, path := range []string{"services/AGENTS.md", "services/api/AGENTS.md"} {
		if !isSupportedGeneratedPath(path) {
			t.Errorf("nested instruction path %q should be supported", path)
		}
	}
	for _, path := range []string{
		"../AGENTS.md", "services/../AGENTS.md", ".forge/AGENTS.md", ".agents/AGENTS.md", ".git/AGENTS.md",
		"services\\AGENTS.md", "services/CLAUDE.md", "services/AGENTS.override.md", "services:stream/AGENTS.md",
	} {
		if isSupportedGeneratedPath(path) {
			t.Errorf("unsafe or non-generated path %q was supported", path)
		}
	}

	root := forgeSyncFixture(t)
	addForgeSyncKnowledge(t, root, "service-rule", "Use service conventions.", []string{"services/**"})
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "services")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := SyncForge(context.Background(), root, "dry-run"); err == nil || !strings.Contains(err.Error(), "generated path parent") {
		t.Fatalf("nested instruction path followed a symlink parent: %v", err)
	}
}

func TestSyncForgePreservesEditMadeAfterPreflightBeforePublish(t *testing.T) {
	root := forgeSyncFixture(t)
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	originalOutput, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, ".forge", "forge.yaml")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest = []byte(strings.Replace(string(manifest), "targets: [codex]", "targets: [codex, claude]", 1))
	if err := os.WriteFile(manifestPath, manifest, 0644); err != nil {
		t.Fatal(err)
	}
	preview, err := SyncForge(context.Background(), root, "dry-run")
	if err != nil || len(preview.Files) != 2 {
		t.Fatalf("expected two outputs in preview, files=%#v err=%v", preview.Files, err)
	}
	firstOutput := filepath.Join(root, filepath.FromSlash(preview.Files[0].Path))
	concurrentOutput := filepath.Join(root, filepath.FromSlash(preview.Files[1].Path))
	humanEdit := []byte("created by a concurrent editor after sync preflight\n")
	_, syncErr := syncForge(context.Background(), root, "apply", func(mutation int) error {
		if mutation == 1 {
			if err := os.WriteFile(concurrentOutput, humanEdit, 0644); err != nil {
				return err
			}
		}
		return nil
	})
	if syncErr == nil || !strings.Contains(syncErr.Error(), "appeared during sync") {
		t.Fatalf("concurrent output creation was not reported: %v", syncErr)
	}
	if got, err := os.ReadFile(concurrentOutput); err != nil || string(got) != string(humanEdit) {
		t.Fatalf("concurrent edit was not preserved: content=%q err=%v", got, err)
	}
	if got, err := os.ReadFile(firstOutput); err != nil || string(got) != string(originalOutput) {
		t.Fatalf("earlier output %s was not restored: content matches original=%v err=%v (sync err=%v, concurrent=%s)", firstOutput, string(got) == string(originalOutput), err, syncErr, concurrentOutput)
	}
}

func TestConcurrentSyncAppliesSerializeAndLeaveCheckableOutput(t *testing.T) {
	root := forgeSyncFixture(t)
	enteredMutation := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	var pauseOnce sync.Once
	go func() {
		_, err := syncForge(context.Background(), root, "apply", func(int) error {
			pauseOnce.Do(func() {
				close(enteredMutation)
				<-releaseFirst
			})
			return nil
		})
		firstDone <- err
	}()
	<-enteredMutation

	secondStarted := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		close(secondStarted)
		_, err := SyncForge(context.Background(), root, "apply")
		secondDone <- err
	}()
	<-secondStarted
	select {
	case err := <-secondDone:
		t.Fatalf("concurrent apply completed while the first transaction was paused: %v", err)
	case <-time.After(40 * time.Millisecond):
	}
	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("first apply failed: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second apply failed: %v", err)
	}
	if _, err := SyncForge(context.Background(), root, "check"); err != nil {
		t.Fatalf("serialized applies left output out of sync: %v", err)
	}
}

func TestCompileForgeExportsArchitectureToCodexAndClaude(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, ".forge", "forge.yaml")
	if err := os.MkdirAll(filepath.Dir(config), 0700); err != nil {
		t.Fatal(err)
	}
	manifest := "layout_version: 1\nir_version: 2\nproject: {name: sample}\narchitecture:\n  styles: [hexagonal, event-driven]\ntargets: [codex, claude]\nreferences: {}\n"
	if err := os.WriteFile(config, []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := CompileForge(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"AGENTS.md", "CLAUDE.md"} {
		content, ok := result.Diff[path]
		if !ok {
			t.Fatalf("missing target export %s: %#v", path, result.Files)
		}
		if !strings.Contains(content, "## Architecture") || !strings.Contains(content, "- event-driven") || !strings.Contains(content, "- hexagonal") {
			t.Errorf("%s omitted architecture styles: %s", path, content)
		}
	}
}

func TestCompileAndSyncClaudeNativeScopedRulesWithOwnership(t *testing.T) {
	root := forgeSyncFixture(t)
	addForgeSyncKnowledge(t, root, "scoped", "Use service conventions.", []string{"services/**", "services/api/**/*.go"})
	manifestPath := filepath.Join(root, ".forge", "forge.yaml")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest = []byte(strings.Replace(string(manifest), "targets: [codex]", "targets: [claude]", 1))
	if err := os.WriteFile(manifestPath, manifest, 0600); err != nil {
		t.Fatal(err)
	}

	plan, err := CompileForge(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Files) != 2 || !containsGeneratedPath(plan.Files, "CLAUDE.md") {
		t.Fatalf("compiled Claude outputs = %#v", plan.Files)
	}
	var scoped GeneratedFile
	for _, file := range plan.Files {
		if strings.HasPrefix(file.Path, ".claude/rules/") {
			scoped = file
		}
	}
	if scoped.Path == "" {
		t.Fatalf("compiled scoped Claude output missing: %#v", plan.Files)
	}
	if strings.Contains(plan.Diff["CLAUDE.md"], "Use service conventions") {
		t.Fatal("scoped rule leaked into global CLAUDE.md")
	}
	if scoped.AdapterVersion != "2" || scoped.Capabilities["scope"] != "native-path-frontmatter;glob-parity-unverified" {
		t.Fatalf("scoped Claude adapter metadata = %#v", scoped)
	}
	for _, file := range plan.Files {
		if file.Path == "CLAUDE.md" && file.Capabilities["scope"] != "global-only" {
			t.Fatalf("CLAUDE.md capability should be global-only: %#v", file)
		}
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "check"); err != nil {
		t.Fatalf("applied Claude rules failed check: %v", err)
	}

	var owned GeneratedManifest
	data, err := os.ReadFile(filepath.Join(root, generatedManifest))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &owned); err != nil {
		t.Fatal(err)
	}
	if !containsGeneratedPath(owned.Files, scoped.Path) {
		t.Fatalf("manifest doesn't own scoped rule: %#v", owned.Files)
	}

	// Removing the rule makes its generated file stale. A human edit must stop
	// apply and preserve bytes rather than silently deleting the file.
	rulePath := filepath.Join(root, filepath.FromSlash(scoped.Path))
	generatedRule, err := os.ReadFile(rulePath)
	if err != nil {
		t.Fatal(err)
	}
	manual := []byte("human rule edit\n")
	if err := os.WriteFile(rulePath, manual, 0600); err != nil {
		t.Fatal(err)
	}
	manifest, err = os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest = []byte(strings.Replace(string(manifest), "    - id: scoped\n      path: .forge/knowledge/items/scoped.md\n", "", 1))
	if err := os.WriteFile(manifestPath, manifest, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err == nil || !strings.Contains(err.Error(), "manually edited generated file "+scoped.Path) {
		t.Fatalf("edited stale Claude rule was removed: %v", err)
	}
	if got, err := os.ReadFile(rulePath); err != nil || !bytes.Equal(got, manual) {
		t.Fatalf("human rule edit changed: %q err=%v", got, err)
	}
	if err := os.WriteFile(rulePath, generatedRule, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatalf("intact stale Claude rule was not cleaned up: %v", err)
	}
	if _, err := os.Stat(rulePath); !os.IsNotExist(err) {
		t.Fatalf("intact stale Claude rule remains after apply: %v", err)
	}
	if _, err := SyncForge(context.Background(), root, "check"); err != nil {
		t.Fatalf("stale Claude rule cleanup left output out of sync: %v", err)
	}
}

func TestClaudeRuleAllowlistRejectsUnsafeNamesAndSymlinkParents(t *testing.T) {
	valid := ".claude/rules/" + strings.Repeat("a", 64) + ".md"
	if !isSupportedGeneratedPath(valid) {
		t.Fatalf("expected supported Claude rule path: %s", valid)
	}
	for _, path := range []string{".claude/rules/x.md", ".claude/rules/" + strings.Repeat("A", 64) + ".md", ".claude/rules/" + strings.Repeat("a", 64) + ".md/child", ".claude/rules/../CLAUDE.md", ".claude/rules\\" + strings.Repeat("a", 64) + ".md"} {
		if isSupportedGeneratedPath(path) {
			t.Errorf("unsafe Claude rule path supported: %q", path)
		}
	}
	root := forgeSyncFixture(t)
	addForgeSyncKnowledge(t, root, "scoped", "Use service conventions.", []string{"services/**"})
	manifestPath := filepath.Join(root, ".forge", "forge.yaml")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest = []byte(strings.Replace(string(manifest), "targets: [codex]", "targets: [claude]", 1))
	if err := os.WriteFile(manifestPath, manifest, 0600); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.Symlink(outside, claudeDir); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := SyncForge(context.Background(), root, "dry-run"); err == nil || !strings.Contains(err.Error(), "generated path parent") {
		t.Fatalf("Claude rules followed a symlink parent: %v", err)
	}
}

func TestSyncForgeRejectsMalformedOwnershipManifestBeforeApply(t *testing.T) {
	for name, manifest := range map[string]GeneratedManifest{
		"duplicate path": {Version: 1, Files: []GeneratedFile{
			{Path: "AGENTS.md", SHA256: strings.Repeat("a", sha256.Size*2)},
			{Path: "AGENTS.md", SHA256: strings.Repeat("b", sha256.Size*2)},
		}},
		"invalid hash":      {Version: 1, Files: []GeneratedFile{{Path: "AGENTS.md", SHA256: "not-a-hash"}}},
		"wrong hash length": {Version: 1, Files: []GeneratedFile{{Path: "AGENTS.md", SHA256: "abcd"}}},
	} {
		t.Run(name, func(t *testing.T) {
			root := forgeSyncFixture(t)
			path := filepath.Join(root, filepath.FromSlash(generatedManifest))
			data, err := json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := SyncForge(context.Background(), root, "apply"); err == nil || !strings.Contains(err.Error(), "invalid generated manifest") {
				t.Fatalf("malformed ownership manifest was accepted: %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
				t.Fatal("apply wrote generated output after rejecting manifest")
			}
		})
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
	item.ReviewMetadataSHA256 = domain.HashKnowledgeReviewMetadata(item)
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

func TestCompileForgeRejectsSemanticMetadataChangedAfterApproval(t *testing.T) {
	for _, mutate := range []struct {
		name  string
		apply func(*domain.KnowledgeItem)
	}{
		{name: "scope", apply: func(item *domain.KnowledgeItem) { item.Scope.Paths = []string{"private/**"} }},
		{name: "keywords", apply: func(item *domain.KnowledgeItem) { item.Keywords = []string{"secret"} }},
		{name: "kind", apply: func(item *domain.KnowledgeItem) { item.Kind = domain.KnowledgeConstraint }},
		{name: "origin", apply: func(item *domain.KnowledgeItem) { item.Origin = "different-source" }},
		{name: "evidence reference", apply: func(item *domain.KnowledgeItem) {
			item.Evidence = []domain.KnowledgeEvidence{{Path: "docs/new-source.md", Quote: "new assertion"}}
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			root := forgeSyncFixture(t)
			path := filepath.Join(root, ".forge", "knowledge", "items", "rule.md")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			text := string(data)
			end := strings.Index(text, "\n---\n")
			if end < 0 {
				t.Fatal("fixture front matter is unterminated")
			}
			var item domain.KnowledgeItem
			if err := yaml.Unmarshal([]byte(text[len("---\n"):end]), &item); err != nil {
				t.Fatal(err)
			}
			mutate.apply(&item)
			encoded, err := yaml.Marshal(item)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append(append([]byte("---\n"), encoded...), []byte("---\n")...), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := CompileForge(root); err == nil || !strings.Contains(err.Error(), "metadata changed after review") {
				t.Fatalf("changed %s was published: %v", mutate.name, err)
			}
		})
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

func TestSyncDryRunReportsOwnershipConflictsWithoutWriting(t *testing.T) {
	t.Run("unmanaged collision", func(t *testing.T) {
		root := forgeSyncFixture(t)
		target := filepath.Join(root, "AGENTS.md")
		if err := os.WriteFile(target, []byte("human instructions\n"), 0644); err != nil {
			t.Fatal(err)
		}
		result, err := SyncForge(context.Background(), root, "dry-run")
		if err == nil || !strings.Contains(err.Error(), "unmanaged output") {
			t.Fatalf("missing collision diagnostic: %v", err)
		}
		if len(result.Conflicts) != 1 || !strings.Contains(result.Conflicts[0], "AGENTS.md") {
			t.Fatalf("unexpected conflicts: %#v", result.Conflicts)
		}
		if data, e := os.ReadFile(target); e != nil || string(data) != "human instructions\n" {
			t.Fatalf("dry-run changed target: %q %v", data, e)
		}
		if _, e := os.Stat(filepath.Join(root, generatedManifest)); !os.IsNotExist(e) {
			t.Fatalf("dry-run wrote ownership manifest: %v", e)
		}
	})
	t.Run("manual edit", func(t *testing.T) {
		root := forgeSyncFixture(t)
		if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(root, "AGENTS.md")
		if err := os.WriteFile(target, []byte("manual edit\n"), 0644); err != nil {
			t.Fatal(err)
		}
		result, err := SyncForge(context.Background(), root, "dry-run")
		if err == nil || !strings.Contains(err.Error(), "manually edited managed output") {
			t.Fatalf("missing ownership diagnostic: %v", err)
		}
		if len(result.Conflicts) != 1 || !strings.Contains(result.Conflicts[0], "AGENTS.md") {
			t.Fatalf("unexpected conflicts: %#v", result.Conflicts)
		}
		if data, e := os.ReadFile(target); e != nil || string(data) != "manual edit\n" {
			t.Fatalf("dry-run changed target: %q %v", data, e)
		}
	})
}

func TestSyncCheckListsMissingCurrentTarget(t *testing.T) {
	root := forgeSyncFixture(t)
	if _, err := SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, ".forge", "forge.yaml")
	if err := os.WriteFile(manifest, []byte(`layout_version: 1
ir_version: 2
project: {name: demo, languages: [Go]}
targets: [codex, claude]
references:
  knowledge:
    - id: rule-a
      path: .forge/knowledge/items/rule.md
`), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := SyncForge(context.Background(), root, "check")
	if err == nil || !strings.Contains(err.Error(), "CLAUDE.md") {
		t.Fatalf("missing target path in diagnostic: %v", err)
	}
	if !result.Changed {
		t.Fatal("check did not mark missing current target as drift")
	}
	if _, statErr := os.Stat(filepath.Join(root, "CLAUDE.md")); !os.IsNotExist(statErr) {
		t.Fatalf("check wrote missing target: %v", statErr)
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
	item.ReviewMetadataSHA256 = domain.HashKnowledgeReviewMetadata(item)
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
