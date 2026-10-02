package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateCommandWritesV2WithoutOverwritingSource(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "harness.yaml")
	destination := filepath.Join(dir, "harness-v2.yaml")
	original := "version: 1\nproject:\n  name: sample\n"
	if err := os.WriteFile(source, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	command := newRootCommand()
	command.SetArgs([]string{"migrate", source, "--output", destination})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(source)
	if err != nil || string(before) != original {
		t.Fatalf("source changed: %v", err)
	}
	after, err := os.ReadFile(destination)
	if err != nil || !strings.Contains(string(after), "version: 2") {
		t.Fatalf("missing v2 output: %v, %s", err, after)
	}
}

func TestMigrateToForgePreviewApplyAndRollback(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, ".harness", "harness.yaml")
	if err := os.MkdirAll(filepath.Dir(source), 0700); err != nil {
		t.Fatal(err)
	}
	original := "version: 1\nproject: {name: sample}\nrules:\n  - id: boundary\n    description: Keep adapters behind ports\n    origin: human\n    status: approved\n"
	if err := os.WriteFile(source, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	preview := newRootCommand()
	preview.SetArgs([]string{"migrate", "--to-layout", "forge", "--dry-run", "--repository", root, "--target", "codex", "--rule-kind", "convention"})
	previewOutput := new(bytes.Buffer)
	preview.SetOut(previewOutput)
	if err := preview.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(previewOutput.Bytes(), []byte(".forge/forge.yaml")) {
		t.Fatalf("preview omitted target manifest: %s", previewOutput)
	}
	var previewPlan struct {
		PlanSHA256 string `json:"plan_sha256"`
	}
	if err := json.Unmarshal(previewOutput.Bytes(), &previewPlan); err != nil || previewPlan.PlanSHA256 == "" {
		t.Fatalf("preview omitted plan digest: %v, %s", err, previewOutput)
	}
	if _, err := os.Stat(filepath.Join(root, ".forge")); !os.IsNotExist(err) {
		t.Fatal("dry-run wrote target layout")
	}
	apply := newRootCommand()
	apply.SetArgs([]string{"migrate", "--to-layout", "forge", "--apply", "--repository", root, "--target", "codex", "--rule-kind", "convention", "--plan-sha256", previewPlan.PlanSHA256})
	apply.SetOut(new(bytes.Buffer))
	if err := apply.Execute(); err != nil {
		t.Fatal(err)
	}
	rollback := newRootCommand()
	rollback.SetArgs([]string{"migrate", "--rollback", "--repository", root})
	rollback.SetOut(new(bytes.Buffer))
	if err := rollback.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".forge")); !os.IsNotExist(err) {
		t.Fatal("rollback did not remove unchanged generated layout")
	}
	after, err := os.ReadFile(source)
	if err != nil || string(after) != original {
		t.Fatalf("legacy source changed: %v", err)
	}
}

func TestMigrateToForgeApplyRequiresMatchingFreshPlanDigest(t *testing.T) {
	for _, tc := range []struct {
		name         string
		digest       string
		changeSource bool
		want         string
	}{
		{name: "missing", want: "--plan-sha256 is required"},
		{name: "mismatch", digest: strings.Repeat("0", 64), want: "digest mismatch"},
		{name: "source changed", digest: "preview", changeSource: true, want: "digest mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, ".harness", "harness.yaml")
			if err := os.MkdirAll(filepath.Dir(source), 0700); err != nil {
				t.Fatal(err)
			}
			original := "version: 1\nproject: {name: sample}\n"
			if err := os.WriteFile(source, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			preview := newRootCommand()
			preview.SetArgs([]string{"migrate", "--to-layout", "forge", "--dry-run", "--repository", root, "--target", "codex"})
			out := new(bytes.Buffer)
			preview.SetOut(out)
			if err := preview.Execute(); err != nil {
				t.Fatal(err)
			}
			var p struct {
				PlanSHA256 string `json:"plan_sha256"`
			}
			if err := json.Unmarshal(out.Bytes(), &p); err != nil {
				t.Fatal(err)
			}
			if tc.changeSource {
				if err := os.WriteFile(source, []byte(strings.Replace(original, "sample", "changed", 1)), 0600); err != nil {
					t.Fatal(err)
				}
				tc.digest = p.PlanSHA256
			}
			args := []string{"migrate", "--to-layout", "forge", "--apply", "--repository", root, "--target", "codex"}
			if tc.digest != "" {
				args = append(args, "--plan-sha256", tc.digest)
			}
			apply := newRootCommand()
			apply.SetArgs(args)
			err := apply.Execute()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
			if _, err := os.Stat(filepath.Join(root, ".forge")); !os.IsNotExist(err) {
				t.Fatal("rejected apply created .forge")
			}
		})
	}
}
