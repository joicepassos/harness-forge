package infrastructure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go.yaml.in/yaml/v3"
	"harnessforge/internal/harness/domain"
	"harnessforge/internal/inputlimits"
	"harnessforge/schemas"
	"io"
)

type ManifestLoader struct{}

func (ManifestLoader) Load(path string) (domain.Manifest, error) {
	var manifest domain.Manifest
	data, err := inputlimits.ReadFile(path, inputlimits.HarnessYAMLBytes, "Forge manifest")
	if err != nil {
		return manifest, fmt.Errorf("%s: %w", path, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var value any
	if err := decoder.Decode(&value); err != nil {
		return manifest, fmt.Errorf("%s: %w", path, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return manifest, fmt.Errorf("%s: expected one YAML document", path)
	}
	if err := rejectNull(value, "$"); err != nil {
		return manifest, err
	}
	jsonData, err := json.Marshal(value)
	if err != nil {
		return manifest, fmt.Errorf("%s: YAML must use string keys: %w", path, err)
	}
	strict := json.NewDecoder(bytes.NewReader(jsonData))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&manifest); err != nil {
		return manifest, fmt.Errorf("%s: %w", path, err)
	}
	if err := manifest.Validate(); err != nil {
		return manifest, err
	}
	if err := schemas.Validate("forge-layout-v1.schema.json", jsonData); err != nil {
		return manifest, err
	}
	return manifest, nil
}

type ProjectConfig struct {
	Layout ProjectLayout
	// Harness is a compatibility projection of fields shared with the legacy
	// IR. Forge-only knowledge, targets, and policies remain in Manifest.
	Harness  domain.Harness
	Manifest *domain.Manifest
}

// ProjectLoader adapts the discovered layout to the existing Harness loader
// interface used by application services. A non-empty path is an explicit
// legacy Harness file; an empty path performs layout discovery.
type ProjectLoader struct {
	Root      string
	Selection string
}

func (l ProjectLoader) Load(path string) (domain.Harness, error) {
	if path != "" {
		return (YAMLLoader{}).Load(path)
	}
	project, err := LoadProject(l.Root, l.Selection)
	if err != nil {
		return domain.Harness{}, err
	}
	if project.Manifest != nil {
		return domain.Harness{}, fmt.Errorf("legacy Harness consumers cannot load the Forge manifest; use the Forge-native command")
	}
	return project.Harness, nil
}

// LoadProject resolves either supported layout. For Forge projects, Harness is
// only a compatibility projection of shared fields; Manifest retains the full
// Forge contract and must be used for Forge-specific knowledge and targets.
func LoadProject(root, selection string) (ProjectConfig, error) {
	layout, err := ResolveLayout(root, selection)
	if err != nil {
		return ProjectConfig{}, err
	}
	if layout.Kind == LayoutHarness {
		h, err := (YAMLLoader{}).Load(layout.HarnessPath)
		if err != nil {
			return ProjectConfig{}, err
		}
		return ProjectConfig{Layout: layout, Harness: h}, nil
	}
	manifest, err := (ManifestLoader{}).Load(layout.ManifestPath)
	if err != nil {
		return ProjectConfig{}, err
	}
	h := manifestCompatibilityProjection(manifest)
	return ProjectConfig{Layout: layout, Harness: h, Manifest: &manifest}, nil
}

func manifestCompatibilityProjection(manifest domain.Manifest) domain.Harness {
	h := domain.Harness{
		Version:      manifest.IRVersion,
		Project:      domain.Project{Name: manifest.Project.Name, Languages: append([]string(nil), manifest.Project.Languages...)},
		Architecture: domain.Architecture{Styles: append([]string(nil), manifest.Architecture.Styles...)},
		QualityGates: cloneQualityGates(manifest.QualityGates),
	}
	h.Skills = make([]domain.Skill, 0, len(manifest.References.Skills))
	for _, skill := range manifest.References.Skills {
		h.Skills = append(h.Skills, domain.Skill{
			ID: skill.ID, Description: skill.Description, Path: skill.Path, Status: skill.Status,
			Evidence: append([]domain.Evidence(nil), skill.Evidence...),
		})
	}
	return h
}

func cloneQualityGates(gates []domain.QualityGate) []domain.QualityGate {
	cloned := make([]domain.QualityGate, len(gates))
	for i, gate := range gates {
		cloned[i] = gate
		cloned[i].Workspaces = append([]string(nil), gate.Workspaces...)
		if gate.Env != nil {
			cloned[i].Env = make(map[string]string, len(gate.Env))
			for key, value := range gate.Env {
				cloned[i].Env[key] = value
			}
		}
	}
	return cloned
}
