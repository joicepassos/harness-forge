package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validForgeManifest = `layout_version: 1
ir_version: 2
project:
  name: sample
  languages: [Go]
architecture:
  styles: [hexagonal]
targets: [codex, claude]
references:
  knowledge:
    - id: architecture
      path: .forge/knowledge/architecture.md
  skills:
    - id: review
      description: Review skill
      path: .forge/skills/review/SKILL.md
      status: approved
      evidence:
        - file: docs/architecture.md
          quote: ports and adapters
quality_gates:
  - id: tests
    command: go test ./...
    workspace: .
    workspaces: [.]
    env: {GOFLAGS: -count=1}
policies:
  - id: no-network
    description: Do not access external networks
    capability: advisory
    executor: instruction-text
`

func TestManifestLoaderValidatesOfflineAndPreservesRelativeReferences(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".forge", "forge.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(validForgeManifest), 0600); err != nil {
		t.Fatal(err)
	}
	manifest, err := (ManifestLoader{}).Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.LayoutVersion != 1 || manifest.IRVersion != 2 || manifest.References.Knowledge[0].Path != ".forge/knowledge/architecture.md" {
		t.Fatalf("manifest fields were not preserved: %#v", manifest)
	}
	layout, err := ResolveLayout(root, "forge")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := layout.ResolveReference(manifest.References.Knowledge[0].Path)
	if err != nil || resolved != filepath.Join(root, ".forge", "knowledge", "architecture.md") {
		t.Fatalf("relative reference resolved to %q: %v", resolved, err)
	}
	project, err := LoadProject(root, "forge")
	if err != nil || project.Manifest == nil || project.Harness.Version != 2 || project.Harness.Project.Name != "sample" {
		t.Fatalf("project load=%#v err=%v", project, err)
	}
	if len(project.Harness.Architecture.Styles) != 1 || project.Harness.Architecture.Styles[0] != "hexagonal" || len(project.Harness.Skills) != 1 || project.Harness.Skills[0].Status != "approved" || project.Harness.Skills[0].Evidence[0].Quote != "ports and adapters" || project.Harness.QualityGates[0].Env["GOFLAGS"] != "-count=1" {
		t.Fatalf("compatibility projection lost shared fields: %#v", project.Harness)
	}
	if project.Manifest.LayoutVersion != 1 || len(project.Manifest.Targets) != 2 || len(project.Manifest.Policies) != 1 || project.Manifest.Policies[0].ID != "no-network" || len(project.Manifest.References.Knowledge) != 1 {
		t.Fatalf("Forge-only fields were not retained in the manifest: %#v", project.Manifest)
	}
	project.Harness.Architecture.Styles[0] = "mutated"
	project.Harness.Skills[0].Evidence[0].Quote = "mutated"
	project.Harness.QualityGates[0].Workspaces[0] = "mutated"
	project.Harness.QualityGates[0].Env["GOFLAGS"] = "mutated"
	if project.Manifest.Architecture.Styles[0] != "hexagonal" || project.Manifest.References.Skills[0].Evidence[0].Quote != "ports and adapters" || project.Manifest.QualityGates[0].Workspaces[0] != "." || project.Manifest.QualityGates[0].Env["GOFLAGS"] != "-count=1" {
		t.Fatal("compatibility projection shares mutable fields with the Forge manifest")
	}
}

func TestManifestLoaderRejectsUnknownFieldsNullAndMultipleDocuments(t *testing.T) {
	tests := []struct{ name, content, want string }{
		{"unknown", validForgeManifest + "secret: value\n", "unknown field"},
		{"null", strings.Replace(validForgeManifest, "targets: [codex, claude]", "targets: null", 1), "null is not allowed"},
		{"multiple", validForgeManifest + "---\n" + validForgeManifest, "one YAML document"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "forge.yaml")
			if err := os.WriteFile(path, []byte(tc.content), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := (ManifestLoader{}).Load(path); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestLoadProjectRejectsCoexistingLayoutsWithoutSelection(t *testing.T) {
	root := t.TempDir()
	harness := filepath.Join(root, ".harness", "harness.yaml")
	forge := filepath.Join(root, ".forge", "forge.yaml")
	for path, content := range map[string]string{
		harness: "version: 1\nproject: {name: sample}\n",
		forge:   validForgeManifest,
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := LoadProject(root, ""); err == nil || !strings.Contains(err.Error(), "select --layout") {
		t.Fatalf("coexisting layouts did not require selection: %v", err)
	}
}

func TestYAMLLoaderLoadsQualityGateEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "harness.yaml")
	content := "version: 2\nproject: {name: sample}\nquality_gates:\n  - id: tests\n    command: go test ./...\n    env:\n      GOFLAGS: -count=1\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	h, err := (YAMLLoader{}).Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := h.QualityGates[0].Env["GOFLAGS"]; got != "-count=1" {
		t.Fatalf("gate env = %q", got)
	}
}
