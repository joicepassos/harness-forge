package infrastructure

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These clones exercise the files a downstream agent receives after export.
// They intentionally contain no Forge config or generated ownership state.
func TestWithoutForgeClonesPreserveExportedContract(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "clones", "without-forge")
	forgeSource := filepath.Join("..", "..", "..", "testdata", "clones", "payments-demo-forge")
	sourceCopy := filepath.Join(t.TempDir(), "payments-demo-forge")
	copyTree(t, forgeSource, sourceCopy)

	compiled, err := CompileForge(sourceCopy)
	if err != nil {
		t.Fatalf("compile canonical Forge fixture: %v", err)
	}
	if _, err := SyncForge(context.Background(), sourceCopy, "apply"); err != nil {
		t.Fatalf("sync canonical Forge fixture: %v", err)
	}
	if len(compiled.Files) == 0 {
		t.Fatal("canonical Forge fixture compiled no native files")
	}
	expectedNative := map[string]map[string]bool{"codex": {}, "claude": {}}
	for _, generated := range compiled.Files {
		var cloneName string
		switch generated.Target {
		case "codex":
			cloneName = "codex"
		case "claude":
			cloneName = "claude"
		default:
			t.Fatalf("unexpected fixture target %q for %s", generated.Target, generated.Path)
		}
		expectedNative[cloneName][filepath.ToSlash(generated.Path)] = true
		wantPath := filepath.Join(root, cloneName, filepath.FromSlash(generated.Path))
		gotPath := filepath.Join(sourceCopy, filepath.FromSlash(generated.Path))
		want, err := os.ReadFile(wantPath)
		if err != nil {
			t.Fatalf("read %s clone export %s: %v", cloneName, generated.Path, err)
		}
		got, err := os.ReadFile(gotPath)
		if err != nil {
			t.Fatalf("read generated Forge export %s: %v", generated.Path, err)
		}
		compiledBytes, ok := compiled.Diff[generated.Path]
		if !ok {
			t.Fatalf("CompileForge omitted native output %s", generated.Path)
		}
		if !bytes.Equal([]byte(compiledBytes), got) {
			t.Errorf("SyncForge output %s differs from CompileForge output", generated.Path)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s clone export %s differs from canonical Forge output", cloneName, generated.Path)
		}
	}
	for _, cloneName := range []string{"codex", "claude"} {
		clone := filepath.Join(root, cloneName)
		for _, forbidden := range []string{
			filepath.Join(clone, ".forge"), filepath.Join(clone, ".harness"),
			filepath.Join(clone, ".forge", "generated-manifest.json"),
		} {
			if _, err := os.Lstat(forbidden); !os.IsNotExist(err) {
				t.Errorf("%s consumer clone contains Forge state %s: %v", cloneName, forbidden, err)
			}
		}
		for _, ownership := range []string{
			filepath.Join(clone, ".harnessforge-generated-hashes"),
			filepath.Join(clone, ".forge", "generated-manifest.json"),
			filepath.Join(clone, ".harness", "generated-manifest.json"),
		} {
			if _, err := os.Lstat(ownership); !os.IsNotExist(err) {
				t.Errorf("%s consumer clone contains generated ownership state %s: %v", cloneName, ownership, err)
			}
		}
		actualNative, err := listNativeExports(clone)
		if err != nil {
			t.Fatalf("list %s native exports: %v", cloneName, err)
		}
		for rel := range expectedNative[cloneName] {
			if !actualNative[rel] {
				t.Errorf("%s clone is missing native export %s", cloneName, rel)
			}
		}
		for rel := range actualNative {
			if !expectedNative[cloneName][rel] {
				t.Errorf("%s clone has stale or extra native export %s", cloneName, rel)
			}
		}
	}

	for _, agent := range []struct {
		name, instruction string
	}{
		{name: "codex", instruction: "AGENTS.md"},
		{name: "claude", instruction: "CLAUDE.md"},
	} {
		t.Run(agent.name, func(t *testing.T) {
			clone := filepath.Join(root, agent.name)
			for _, forbidden := range []string{
				filepath.Join(clone, ".forge"), filepath.Join(clone, ".harness"),
				filepath.Join(clone, ".forge", "generated-manifest.json"),
			} {
				if _, err := os.Lstat(forbidden); !os.IsNotExist(err) {
					t.Fatalf("consumer clone contains Forge state %s: %v", forbidden, err)
				}
			}
			instruction, err := os.ReadFile(filepath.Join(clone, agent.instruction))
			if err != nil {
				t.Fatal(err)
			}
			for _, required := range []string{
				"PAY-001", "idempotent", "PAY-002", "typed error", "PAY-003",
				"PaymentProvider", "SKILL-001",
				"GATE-001", "go test ./...",
			} {
				if !strings.Contains(string(instruction), required) {
					t.Errorf("%s export is missing %q", agent.instruction, required)
				}
			}
			skillDir := filepath.Join(clone, ".agents", "skills", "payment-provider-change")
			if agent.name == "claude" {
				skillDir = filepath.Join(clone, ".claude", "skills", "payment-provider-change")
			}
			skill, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(skill), "idempotency") || !strings.Contains(string(skill), "amount and currency") {
				t.Errorf("%s clone skill is missing its payment procedure", agent.name)
			}
			if _, err := os.Stat(filepath.Join(clone, "go.mod")); err != nil {
				t.Errorf("%s clone lacks its independent project: %v", agent.name, err)
			}
		})
	}
}

func listNativeExports(root string) (map[string]bool, error) {
	files := map[string]bool{}
	for _, rel := range []string{"AGENTS.md", "CLAUDE.md"} {
		if info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel))); err == nil && info.Mode().IsRegular() {
			files[rel] = true
		} else if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	for _, dir := range []string{".agents/skills", ".claude/skills", ".claude/rules"} {
		base := filepath.Join(root, filepath.FromSlash(dir))
		if _, err := os.Lstat(base); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}
		var addDirectory func(string) error
		addDirectory = func(path string) error {
			entries, err := os.ReadDir(path)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				child := filepath.Join(path, entry.Name())
				if entry.IsDir() {
					if err := addDirectory(child); err != nil {
						return err
					}
					continue
				}
				rel, err := filepath.Rel(root, child)
				if err != nil {
					return err
				}
				files[filepath.ToSlash(rel)] = true
			}
			return nil
		}
		if err := addDirectory(base); err != nil {
			return nil, err
		}
	}
	return files, nil
}

func copyTree(t *testing.T, source, destination string) {
	t.Helper()
	if err := os.MkdirAll(destination, 0755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		from := filepath.Join(source, entry.Name())
		to := filepath.Join(destination, entry.Name())
		if entry.IsDir() {
			copyTree(t, from, to)
			continue
		}
		data, err := os.ReadFile(from)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(to, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}
