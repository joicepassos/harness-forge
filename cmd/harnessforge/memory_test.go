package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
	harnessdomain "harnessforge/internal/harness/domain"
	"harnessforge/internal/memory"
)

func TestMemoryObservationRequiresTwoExplicitReviewStepsBeforeSync(t *testing.T) {
	state := t.TempDir()
	t.Setenv("APPDATA", state)
	t.Setenv("XDG_CONFIG_HOME", state)
	root := t.TempDir()
	manifest := filepath.Join(root, ".forge", "forge.yaml")
	if err := os.MkdirAll(filepath.Dir(manifest), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte("layout_version: 1\nir_version: 2\nproject: {name: sample, languages: [Go]}\ntargets: [codex]\nreferences: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCommand()
	cmd.SetArgs([]string{"memory", "capture", "Auth policy is explicit.", "--source", "manual", "--repository", root})
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var observation memory.Observation
	if err := json.Unmarshal(out.Bytes(), &observation); err != nil {
		t.Fatal(err)
	}
	if observation.State != memory.Candidate {
		t.Fatal("capture was not a candidate")
	}
	cmd = newRootCommand()
	cmd.SetArgs([]string{"memory", "publish", observation.ID, "--kind", "convention", "--repository", root})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err == nil {
		t.Fatal("unreviewed observation was published")
	}
	cmd = newRootCommand()
	cmd.SetArgs([]string{"memory", "review", observation.ID, "approved", "--reviewer", "alice", "--repository", root})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	cmd = newRootCommand()
	cmd.SetArgs([]string{"memory", "publish", observation.ID, "--kind", "convention", "--repository", root})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(root, ".forge", "knowledge", "items", "*.md"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("published knowledge files=%v err=%v", paths, err)
	}
	published, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	frontMatter := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(string(published), "---"), "---"))
	var knowledge harnessdomain.KnowledgeItem
	if err := yaml.Unmarshal([]byte(frontMatter), &knowledge); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"reviewer:alice", "repository:", "checkout:", "revision:", "reviewed_at:"} {
		if !strings.Contains(knowledge.Origin, value) {
			t.Fatalf("publication origin omits %q: %s", value, knowledge.Origin)
		}
	}
	if !strings.Contains(knowledge.ReviewDiff, "Auth policy is explicit.") || knowledge.ContentSHA256 == "" {
		t.Fatalf("publication lost review evidence: %#v", knowledge)
	}
	cmd = newRootCommand()
	cmd.SetArgs([]string{"review", "observation-" + observation.ID, "approved", "--reviewer", "bob", "--layout", "forge", "--repository", root})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	cmd = newRootCommand()
	cmd.SetArgs([]string{"sync", "--apply", "--repository", root})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	generated, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(generated, []byte("Auth policy is explicit.")) {
		t.Fatalf("reviewed observation wasn't compiled: %s", generated)
	}
}

func TestMemoryCaptureRejectsMissingOrForgedEvidence(t *testing.T) {
	state := t.TempDir()
	t.Setenv("APPDATA", state)
	t.Setenv("XDG_CONFIG_HOME", state)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "evidence.txt"), []byte("verified source"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCommand()
	cmd.SetArgs([]string{"memory", "capture", "observation", "--source", "manual", "--evidence", "../outside", "--repository", root})
	if err := cmd.Execute(); err == nil {
		t.Fatal("escaping evidence path accepted")
	}
}
