package domain

import (
	"fmt"
	"path"
	"strings"
)

// Manifest is the versioned project configuration stored at .forge/forge.yaml.
// LayoutVersion describes this document's layout; IRVersion independently
// selects the canonical Harness representation used by its contents.
type Manifest struct {
	LayoutVersion int                `json:"layout_version" yaml:"layout_version"`
	IRVersion     int                `json:"ir_version" yaml:"ir_version"`
	Project       Project            `json:"project" yaml:"project"`
	Targets       []string           `json:"targets" yaml:"targets"`
	References    ManifestReferences `json:"references" yaml:"references"`
	QualityGates  []QualityGate      `json:"quality_gates,omitempty" yaml:"quality_gates,omitempty"`
}

type ManifestReferences struct {
	Knowledge []KnowledgeReference `json:"knowledge,omitempty" yaml:"knowledge,omitempty"`
	Skills    []string             `json:"skills,omitempty" yaml:"skills,omitempty"`
}

type KnowledgeReference struct {
	ID   string `json:"id" yaml:"id"`
	Path string `json:"path" yaml:"path"`
}

// Validate checks the offline manifest contract. It never resolves or reads
// referenced paths; paths are repository-relative POSIX paths in all hosts.
func (m Manifest) Validate() error {
	if m.LayoutVersion != 1 {
		return fmt.Errorf("layout_version: expected 1")
	}
	if m.IRVersion != 1 && m.IRVersion != 2 {
		return fmt.Errorf("ir_version: expected 1 or 2")
	}
	if strings.TrimSpace(m.Project.Name) == "" {
		return fmt.Errorf("project.name: must not be empty")
	}
	if err := nonemptyList("project.languages", m.Project.Languages); err != nil {
		return err
	}
	if len(m.Targets) == 0 {
		return fmt.Errorf("targets: must contain at least one target")
	}
	seenTargets := map[string]bool{}
	for i, target := range m.Targets {
		if target != "codex" && target != "claude" {
			return fmt.Errorf("targets[%d]: expected codex or claude", i)
		}
		if seenTargets[target] {
			return fmt.Errorf("targets[%d]: duplicate %q", i, target)
		}
		seenTargets[target] = true
	}
	seen := map[string]bool{}
	for i, ref := range m.References.Knowledge {
		field := fmt.Sprintf("references.knowledge[%d]", i)
		if strings.TrimSpace(ref.ID) == "" {
			return fmt.Errorf("%s.id: must not be empty", field)
		}
		if seen[ref.ID] {
			return fmt.Errorf("%s.id: duplicate %q", field, ref.ID)
		}
		seen[ref.ID] = true
		if err := validateRelativePath(field+".path", ref.Path); err != nil {
			return err
		}
	}
	for i, skill := range m.References.Skills {
		if err := validateRelativePath(fmt.Sprintf("references.skills[%d]", i), skill); err != nil {
			return err
		}
	}
	ids := map[string]bool{}
	for i, gate := range m.QualityGates {
		field := fmt.Sprintf("quality_gates[%d]", i)
		if err := uniqueID(field, gate.ID, ids); err != nil {
			return err
		}
		if strings.TrimSpace(gate.Command) == "" {
			return fmt.Errorf("%s.command: must not be empty", field)
		}
		if gate.Workspace != "" {
			if err := validateRelativePath(field+".workspace", gate.Workspace); err != nil {
				return err
			}
		}
		for j, workspace := range gate.Workspaces {
			if err := validateRelativePath(fmt.Sprintf("%s.workspaces[%d]", field, j), workspace); err != nil {
				return err
			}
		}
		if err := nonemptyList(field+".workspaces", gate.Workspaces); err != nil {
			return err
		}
		if gate.Workspace != "" && (len(gate.Workspaces) == 0 || gate.Workspaces[0] != gate.Workspace) {
			return fmt.Errorf("%s: workspace must be represented in workspaces", field)
		}
	}
	return nil
}

func validateRelativePath(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s: must not be empty", field)
	}
	// Reject Windows absolute paths and separators too, even when validating on Unix.
	if strings.Contains(value, `\`) || strings.HasPrefix(value, "/") || path.IsAbs(value) || (len(value) >= 2 && value[1] == ':') {
		return fmt.Errorf("%s: expected a relative path", field)
	}
	for _, part := range strings.Split(value, "/") {
		if part == ".." {
			return fmt.Errorf("%s: path must not escape the project root", field)
		}
	}
	return nil
}
