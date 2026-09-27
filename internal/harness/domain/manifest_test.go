package domain

import (
	"strings"
	"testing"
)

func validManifest() Manifest {
	return Manifest{
		LayoutVersion: 1, IRVersion: 2,
		Project: Project{Name: "example", Languages: []string{"Go"}},
		Targets: []string{"codex", "claude"},
		References: ManifestReferences{
			Knowledge: []KnowledgeReference{{ID: "architecture", Path: "knowledge/architecture.md"}},
			Skills:    []string{"skills/review/SKILL.md"},
		},
		QualityGates: []QualityGate{{ID: "unit", Command: "go test ./...", Workspace: "services/api", Workspaces: []string{"services/api"}}},
	}
}

func TestManifestVersionsAreIndependent(t *testing.T) {
	for _, irVersion := range []int{1, 2} {
		m := validManifest()
		m.IRVersion = irVersion
		if err := m.Validate(); err != nil {
			t.Fatalf("IR version %d rejected: %v", irVersion, err)
		}
	}
	m := validManifest()
	m.LayoutVersion = 2
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "layout_version") {
		t.Fatalf("expected layout version diagnostic, got %v", err)
	}
}

func TestManifestRejectsInvalidRelativePaths(t *testing.T) {
	badPaths := []string{"", "/absolute/path", `C:\work\file`, `..\outside`, "knowledge/../../outside"}
	for _, bad := range badPaths {
		m := validManifest()
		m.References.Knowledge[0].Path = bad
		if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "references.knowledge[0].path") {
			t.Errorf("expected path diagnostic for %q, got %v", bad, err)
		}
	}
}

func TestManifestReportsClearFieldErrors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Manifest)
		want   string
	}{
		{"unsupported target", func(m *Manifest) { m.Targets = []string{"cursor"} }, "targets[0]"},
		{"duplicate knowledge id", func(m *Manifest) { m.References.Knowledge = append(m.References.Knowledge, m.References.Knowledge[0]) }, "references.knowledge[1].id"},
		{"gate workspace outside workspaces", func(m *Manifest) { m.QualityGates[0].Workspaces = nil }, "quality_gates[0]"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifest()
			tc.mutate(&m)
			if err := m.Validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q diagnostic, got %v", tc.want, err)
			}
		})
	}
}
