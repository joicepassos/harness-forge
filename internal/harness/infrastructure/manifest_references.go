package infrastructure

import (
	"fmt"
	"harnessforge/internal/agentskills"
	"harnessforge/internal/harness/domain"
	"harnessforge/internal/inputlimits"
	"os"
	"path/filepath"
)

// loadManifestKnowledge reads and validates only the referenced knowledge
// documents. Evidence paths in those documents remain inert metadata here.
func loadManifestKnowledge(layout ProjectLayout, manifest domain.Manifest) ([]domain.KnowledgeItem, error) {
	if len(manifest.References.Knowledge) == 0 {
		return nil, nil
	}
	root, err := os.OpenRoot(layout.Root)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	items := make([]domain.KnowledgeItem, 0, len(manifest.References.Knowledge))
	for _, reference := range manifest.References.Knowledge {
		data, err := ReadProjectFile(root, reference.Path, inputlimits.HarnessYAMLBytes)
		if err != nil {
			return nil, fmt.Errorf("references.knowledge[%s].path: %w", reference.ID, err)
		}
		item, err := parseKnowledgeFrontMatter(data)
		if err != nil {
			return nil, fmt.Errorf("references.knowledge[%s]: %w", reference.ID, err)
		}
		if item.ID != reference.ID {
			return nil, fmt.Errorf("knowledge reference %q points to item %q", reference.ID, item.ID)
		}
		items = append(items, item)
	}
	return items, nil
}

// ValidateManifestReferences checks that referenced documents, skills, and
// gate workspaces exist under the selected project root. It does not parse or
// execute referenced content or quality-gate commands.
func ValidateManifestReferences(layout ProjectLayout, manifest domain.Manifest) error {
	for _, item := range manifest.References.Knowledge {
		path, err := layout.ResolveReference(item.Path)
		if err != nil {
			return fmt.Errorf("references.knowledge[%s].path: %w", item.ID, err)
		}
		if err := requireRegularFile(path); err != nil {
			return fmt.Errorf("references.knowledge[%s].path: %w", item.ID, err)
		}
	}
	root, err := os.OpenRoot(layout.Root)
	if err != nil {
		return err
	}
	defer root.Close()
	for index, reference := range manifest.References.Skills {
		bundle, err := agentskills.Read(root, reference.Path)
		if err != nil {
			return fmt.Errorf("references.skills[%d]: %w", index, err)
		}
		if bundle.Description != reference.Description {
			return fmt.Errorf("references.skills[%d]: manifest description does not match SKILL.md", index)
		}
	}
	for index, gate := range manifest.QualityGates {
		workspaces := gate.Workspaces
		if len(workspaces) == 0 && gate.Workspace != "" {
			workspaces = []string{gate.Workspace}
		}
		for workspaceIndex, workspace := range workspaces {
			path, err := layout.ResolveReference(workspace)
			if err != nil {
				return fmt.Errorf("quality_gates[%d].workspaces[%d]: %w", index, workspaceIndex, err)
			}
			info, err := os.Lstat(path)
			if err != nil {
				return fmt.Errorf("quality_gates[%d].workspaces[%d]: %w", index, workspaceIndex, err)
			}
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return fmt.Errorf("quality_gates[%d].workspaces[%d]: expected a regular directory", index, workspaceIndex)
			}
		}
	}
	return nil
}

func requireRegularFile(path string) error {
	info, err := os.Lstat(filepath.Clean(path))
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("expected a regular non-symlink file")
	}
	return nil
}
