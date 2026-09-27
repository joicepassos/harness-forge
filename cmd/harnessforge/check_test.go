package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"go.yaml.in/yaml/v3"
	generation "harnessforge/internal/generation/infrastructure"
	harnessdomain "harnessforge/internal/harness/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckCommandValidatesForgeAndRequiresGeneratedFilesInSync(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, ".forge", "forge.yaml")
	if err := os.MkdirAll(filepath.Dir(manifest), 0700); err != nil {
		t.Fatal(err)
	}
	content := "layout_version: 1\nir_version: 2\nproject: {name: sample, languages: [Go]}\ntargets: [codex]\nreferences: {}\n"
	if err := os.WriteFile(manifest, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCommand()
	cmd.SetArgs([]string{"check", "--repository", root, "--format", "json"})
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "checks failed") {
		t.Fatalf("check should require generated exports, got %v", err)
	}
	if !strings.Contains(out.String(), "generated.drift") || !strings.Contains(out.String(), `"version": 1`) {
		t.Fatalf("missing versioned drift diagnostic: %s", out)
	}
	if _, err := generation.SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	cmd = newRootCommand()
	cmd.SetArgs([]string{"check", "--repository", root, "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("check failed after syncing generated outputs: %v", err)
	}

	// Check is read-only: it reports missing generated files and does not create them.
}

func TestCheckCommandRejectsAmbiguousLayoutsAndUnsupportedFormat(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{".forge/forge.yaml", ".harness/harness.yaml"} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		content := "layout_version: 1\nir_version: 2\nproject: {name: sample, languages: [Go]}\ntargets: [codex]\nreferences: {}\n"
		if strings.Contains(path, "harness.yaml") {
			content = "version: 1\nproject: {name: sample}\n"
		}
		if err := os.WriteFile(full, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := newRootCommand()
	cmd.SetArgs([]string{"check", "--repository", root, "--format", "json"})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err == nil {
		t.Fatal("ambiguous project layout accepted")
	}
	cmd = newRootCommand()
	cmd.SetArgs([]string{"check", "--repository", root, "--layout", "forge", "--format", "xml"})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("unsupported format accepted: %v", err)
	}
}

func TestCheckCommandChecksManagedLegacyOutputOnly(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, ".harness", "harness.yaml")
	if err := os.MkdirAll(filepath.Dir(config), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("version: 1\nproject: {name: sample}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCommand()
	cmd.SetArgs([]string{"check", "--repository", root, "--format", "json"})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("manual Harness project with no generated ownership should validate: %v", err)
	}
	// A generated hash record makes the static output a required, checked artifact.
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("stale"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".harnessforge-generated-hashes"), []byte(strings.Repeat("0", 64)+" AGENTS.md\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd = newRootCommand()
	cmd.SetArgs([]string{"check", "--repository", root, "--format", "json"})
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	if err := cmd.Execute(); err == nil || !strings.Contains(out.String(), "generated.drift") {
		t.Fatalf("managed legacy output drift wasn't reported: %v %s", err, out)
	}
}

func TestCheckRunsGatesOnlyWhenExplicitlyRequested(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, ".harness", "harness.yaml")
	if err := os.MkdirAll(filepath.Dir(config), 0700); err != nil {
		t.Fatal(err)
	}
	text := "version: 2\nproject: {name: sample}\nquality_gates:\n  - id: explicit\n    command: exit 9\n    workspace: .\n    workspaces: [.]\n"
	if err := os.WriteFile(config, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCommand()
	cmd.SetArgs([]string{"check", "--repository", root, "--format", "json"})
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate ran without explicit flag: %v %s", err, out)
	}
	cmd = newRootCommand()
	cmd.SetArgs([]string{"check", "--repository", root, "--run-gates", "--format", "json"})
	out = new(bytes.Buffer)
	cmd.SetOut(out)
	if err := cmd.Execute(); err == nil || !strings.Contains(out.String(), "gate.failed") {
		t.Fatalf("failing gate wasn't reported: %v %s", err, out)
	}
}

func TestCheckDetectsApprovedKnowledgeChanges(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "evidence.go"), []byte("package example\nconst TokenLifetime = 15\n"), 0600); err != nil {
		t.Fatal(err)
	}
	createForgeKnowledgeFixture(t, root, "auth-policy", "approved", "reviewed", "Token lifetime is fifteen minutes.", "evidence.go")
	// The fixture is deliberately valid before a post-review edit.
	cmd := newRootCommand()
	cmd.SetArgs([]string{"check", "--repository", root, "--format", "json"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected missing exports to be reported")
	}
	path := filepath.Join(root, ".forge", "knowledge", "auth-policy.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte("Token lifetime is fifteen minutes."), []byte("Token lifetime is thirty minutes."), 1)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	cmd = newRootCommand()
	cmd.SetArgs([]string{"check", "--repository", root, "--format", "json"})
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	if err := cmd.Execute(); err == nil || !strings.Contains(out.String(), "project.knowledge_invalid") {
		t.Fatalf("reviewed edit wasn't diagnosed: %v %s", err, out)
	}
}

func createForgeKnowledgeFixture(t *testing.T, root, id, state, reviewer, content, evidence string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".forge", "knowledge"), 0700); err != nil {
		t.Fatal(err)
	}
	item := harnessdomain.KnowledgeItem{ID: id, Kind: harnessdomain.KnowledgeFact, Content: content, Origin: "test", Review: harnessdomain.KnowledgeCandidate, Health: harnessdomain.KnowledgeUnknown}
	if evidence != "" {
		item.Evidence = []harnessdomain.KnowledgeEvidence{{Path: evidence, Quote: "TokenLifetime"}}
	}
	if state == "approved" {
		item.Review = harnessdomain.KnowledgeApproved
		item.Reviewer = reviewer
		item.Health = harnessdomain.KnowledgeVerified
		item.ContentSHA256 = harnessdomain.HashKnowledgeContent(content)
		values := []string{}
		if evidence != "" {
			body, err := os.ReadFile(filepath.Join(root, evidence))
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(body)
			values = append(values, evidence+":"+hex.EncodeToString(sum[:]))
		}
		item.EvidenceSHA256 = harnessdomain.HashKnowledgeEvidence(values)
	}
	encoded, err := yaml.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	file := append(append([]byte("---\n"), encoded...), []byte("---\n")...)
	if err := os.WriteFile(filepath.Join(root, ".forge", "knowledge", "auth-policy.md"), file, 0600); err != nil {
		t.Fatal(err)
	}
	manifest := "layout_version: 1\nir_version: 2\nproject: {name: sample}\ntargets: [codex]\nreferences:\n  knowledge:\n    - id: " + id + "\n      path: .forge/knowledge/auth-policy.md\n"
	if err := os.WriteFile(filepath.Join(root, ".forge", "forge.yaml"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
}
