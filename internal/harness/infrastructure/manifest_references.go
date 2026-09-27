package infrastructure

import (
	"fmt"
	"harnessforge/internal/harness/domain"
	"os"
	"path/filepath"
)

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
	for index, reference := range manifest.References.Skills {
		path, err := layout.ResolveReference(reference.Path)
		if err != nil {
			return fmt.Errorf("references.skills[%d]: %w", index, err)
		}
		if err := requireRegularFile(path); err != nil {
			return fmt.Errorf("references.skills[%d]: %w", index, err)
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
